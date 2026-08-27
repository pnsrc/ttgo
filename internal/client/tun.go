package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.zx2c4.com/wireguard/tun"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	defaultNICID = 1
	defaultMTU   = 1500

	// Виртуальный адрес который TUN присваивает себе. Любой адрес из неиспользуемого
	// диапазона — система направит трафик к "internet" через нас по default route.
	tunIPv4 = "10.99.0.2"
	tunIPv6 = "fdfd:1234::2"
)

// TunDevice — поднятый TUN-интерфейс + gvisor netstack, который раскидывает
// TCP flow в HTTP/2 CONNECT через Dialer.
type TunDevice struct {
	dev    tun.Device
	name   string
	stack  *stack.Stack
	link   *channel.Endpoint
	dialer *Dialer
	stats  *Stats
	ctx    context.Context
	cancel context.CancelFunc

	dnsExclusions []string
	onDNSBypass   func(ips []net.IP)
	adBlocker     *AdBlocker
	upstreamDNS   string

	wg sync.WaitGroup
}

// OpenTUN creates a TUN device, brings it up, sets up gvisor netstack,
// и регистрирует TCP forwarder который для каждого нового flow вызовет
// dialer.DialContext.
func OpenTUN(ctx context.Context, dialer *Dialer, stats *Stats) (*TunDevice, error) {
	dev, err := tun.CreateTUN(tunDeviceName, defaultMTU)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	name, err := dev.Name()
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("get tun name: %w", err)
	}
	slog.Info("tun created", "name", name)

	link := channel.New(1024, defaultMTU, "")

	s := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol, ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6,
		},
		HandleLocal: false,
	})

	if tcpErr := s.CreateNIC(defaultNICID, link); tcpErr != nil {
		dev.Close()
		return nil, fmt.Errorf("create nic: %s", tcpErr)
	}

	// Назначаем виртуальные IP интерфейсу netstack'а
	if err := assignAddress(s, defaultNICID, ipv4.ProtocolNumber, tunIPv4); err != nil {
		dev.Close()
		return nil, err
	}
	if err := assignAddress(s, defaultNICID, ipv6.ProtocolNumber, tunIPv6); err != nil {
		dev.Close()
		return nil, err
	}

	// Принимаем все пакеты ("promiscuous"-режим netstack):
	// gvisor видит destination и решает что с ним делать.
	s.SetPromiscuousMode(defaultNICID, true)
	s.SetSpoofing(defaultNICID, true)

	// Default route: всё уходит на наш NIC, dst gateway не важен.
	s.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: defaultNICID},
		{Destination: header.IPv6EmptySubnet, NIC: defaultNICID},
	})

	td := &TunDevice{
		dev:    dev,
		name:   name,
		stack:  s,
		link:   link,
		dialer: dialer,
		stats:  stats,
	}
	td.ctx, td.cancel = context.WithCancel(ctx)

	// TCP forwarder: для каждого нового TCP flow gvisor вызовет нашу функцию.
	tcpFwd := tcp.NewForwarder(s, 65536, 4096, td.handleTCP)
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	// UDP forwarder (минимально — отвечаем на DNS, остальное через CONNECT _udp2).
	udpFwd := udp.NewForwarder(s, td.handleUDP)
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)

	// Запускаем два насоса: TUN→netstack и netstack→TUN.
	td.wg.Add(2)
	go td.pumpTUNToStack()
	go td.pumpStackToTUN()

	return td, nil
}

// Name returns the TUN interface name (e.g. "utun5" on macOS).
func (t *TunDevice) Name() string { return t.name }

// Close brings the device down and stops forwarders.
func (t *TunDevice) Close() error {
	t.cancel()
	t.stack.Close()
	err := t.dev.Close()
	t.wg.Wait()
	return err
}

func (t *TunDevice) pumpTUNToStack() {
	defer t.wg.Done()
	bufs := make([][]byte, 1)
	sizes := make([]int, 1)
	bufs[0] = make([]byte, defaultMTU+tunHeaderLen+4)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}
		n, err := t.dev.Read(bufs, sizes, tunHeaderLen)
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			slog.Warn("tun read", "err", err)
			return
		}
		if n == 0 {
			continue
		}
		pktData := bufs[0][tunHeaderLen : tunHeaderLen+sizes[0]]

		var proto tcpip.NetworkProtocolNumber
		switch pktData[0] >> 4 {
		case 4:
			proto = ipv4.ProtocolNumber
		case 6:
			proto = ipv6.ProtocolNumber
		default:
			continue
		}

		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(pktData),
		})
		t.link.InjectInbound(proto, pkt)
		pkt.DecRef()
	}
}

func (t *TunDevice) pumpStackToTUN() {
	defer t.wg.Done()
	for {
		pkt := t.link.ReadContext(t.ctx)
		if pkt == nil {
			return
		}
		var raw []byte
		for _, v := range pkt.AsSlices() {
			raw = append(raw, v...)
		}
		buf := tunWritePacket(raw, pkt.NetworkProtocolNumber)
		pkt.DecRef()
		if _, err := t.dev.Write([][]byte{buf}, tunHeaderLen); err != nil {
			if t.ctx.Err() != nil {
				return
			}
			slog.Warn("tun write", "err", err)
		}
	}
}

// handleTCP — gvisor вызывает на каждый новый TCP flow с TUN.
func (t *TunDevice) handleTCP(r *tcp.ForwarderRequest) {
	id := r.ID()
	target := net.JoinHostPort(id.LocalAddress.String(), fmt.Sprintf("%d", id.LocalPort))
	slog.Debug("tcp flow accepted", "target", target)

	var wq waiter.Queue
	ep, tcpErr := r.CreateEndpoint(&wq)
	if tcpErr != nil {
		slog.Debug("tcp endpoint", "target", target, "err", tcpErr.String())
		r.Complete(true)
		return
	}
	r.Complete(false)

	localConn := gonet.NewTCPConn(&wq, ep)
	go t.relayTCP(localConn, target)
}

// relayTCP: открываем CONNECT-туннель и проксируем bidirectional.
func (t *TunDevice) relayTCP(localConn net.Conn, target string) {
	defer localConn.Close()
	startedAt := time.Now()

	// Нельзя использовать короткий ctx для CONNECT-стрима: его отмена рвёт downlink.
	remote, err := t.dialer.DialContext(t.ctx, target)
	if err != nil {
		slog.Debug("dial endpoint failed", "target", target, "err", err)
		return
	}
	defer remote.Close()
	slog.Debug("tunnel open", "target", target)

	copyDone := make(chan copyResult, 2)
	go func() {
		n, err := io.Copy(remote, localConn)
		closeWrite(remote)
		copyDone <- copyResult{dir: "uplink", bytes: n, err: err}
	}()
	go func() {
		n, err := io.Copy(localConn, remote)
		closeWrite(localConn)
		copyDone <- copyResult{dir: "downlink", bytes: n, err: err}
	}()

	// Ждём завершение обоих направлений, иначе можно обрубить download.
	r1 := <-copyDone
	r2 := <-copyDone
	slog.Debug("tunnel closed",
		"target", target,
		"duration", time.Since(startedAt).String(),
		"uplink_bytes", pickBytes("uplink", r1, r2),
		"downlink_bytes", pickBytes("downlink", r1, r2),
		"uplink_err", pickErr("uplink", r1, r2),
		"downlink_err", pickErr("downlink", r1, r2),
	)
}

type writeCloser interface {
	CloseWrite() error
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(writeCloser); ok {
		_ = cw.CloseWrite()
	}
}

func (t *TunDevice) handleUDP(r *udp.ForwarderRequest) bool {
	id := r.ID()
	if id.LocalPort != 53 {
		slog.Debug("udp dropped", "dst", net.JoinHostPort(id.LocalAddress.String(), fmt.Sprintf("%d", id.LocalPort)))
		return false
	}

	var wq waiter.Queue
	ep, udpErr := r.CreateEndpoint(&wq)
	if udpErr != nil {
		return true
	}

	localUDP := gonet.NewUDPConn(&wq, ep)
	go t.relayDNSUDP(localUDP)
	return true
}

func (t *TunDevice) relayDNSUDP(localUDP net.Conn) {
	defer localUDP.Close()

	upstream := t.upstreamDNS
	if upstream == "" {
		upstream = "1.1.1.1"
	}

	buf := make([]byte, 4096)
	for {
		localUDP.SetReadDeadline(time.Now().Add(90 * time.Second))
		n, err := localUDP.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		t.stats.BytesOut.Add(uint64(n))

		if t.adBlocker != nil {
			res, domain, parseErr := createFakeDNSResponse(buf[:n])
			if parseErr == nil && t.adBlocker.IsBlocked(domain) {
				slog.Debug("adblock intercepted", "domain", domain)
				localUDP.Write(res)
				continue
			}
		}

		resp, fwdErr := t.forwardDNSQuery(buf[:n], upstream)
		if fwdErr != nil {
			slog.Debug("dns forward failed", "upstream", upstream, "err", fwdErr)
			continue
		}
		t.stats.BytesIn.Add(uint64(len(resp)))

		if t.onDNSBypass != nil {
			if ips := parseDNSResponse(resp, t.dnsExclusions); len(ips) > 0 {
				t.onDNSBypass(ips)
			}
		}

		localUDP.Write(resp)
	}
}

func (t *TunDevice) forwardDNSQuery(query []byte, upstream string) ([]byte, error) {
	conn, err := net.DialTimeout("udp", upstream+":53", 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	resp := make([]byte, 4096)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	return resp[:n], nil
}

type copyResult struct {
	dir   string
	bytes int64
	err   error
}

func pickBytes(dir string, a, b copyResult) int64 {
	if a.dir == dir {
		return a.bytes
	}
	if b.dir == dir {
		return b.bytes
	}
	return 0
}

func pickErr(dir string, a, b copyResult) string {
	var err error
	if a.dir == dir {
		err = a.err
	} else if b.dir == dir {
		err = b.err
	}
	if err == nil {
		return ""
	}
	if err == io.EOF {
		return "EOF"
	}
	return err.Error()
}

// assignAddress — присвоить IP виртуальному интерфейсу netstack.
func assignAddress(s *stack.Stack, nic tcpip.NICID, proto tcpip.NetworkProtocolNumber, ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("parse %s: %w", ip, err)
	}
	prefix := addr.BitLen()
	if proto == ipv4.ProtocolNumber {
		prefix = 32
	} else {
		prefix = 128
	}
	addrWithPrefix := tcpip.AddressWithPrefix{
		Address:   tcpip.AddrFromSlice(addr.AsSlice()),
		PrefixLen: prefix,
	}
	protoAddr := tcpip.ProtocolAddress{Protocol: proto, AddressWithPrefix: addrWithPrefix}
	if tcpErr := s.AddProtocolAddress(nic, protoAddr, stack.AddressProperties{}); tcpErr != nil {
		return fmt.Errorf("add address %s: %s", ip, tcpErr)
	}
	return nil
}

func createFakeDNSResponse(buf []byte) ([]byte, string, error) {
	var p dnsmessage.Parser
	header, err := p.Start(buf)
	if err != nil {
		return nil, "", err
	}
	q, err := p.Question()
	if err != nil {
		return nil, "", err
	}
	
	domain := strings.TrimSuffix(q.Name.String(), ".")
	
	header.Response = true
	header.Authoritative = true
	
	b := dnsmessage.NewBuilder(nil, header)
	b.StartQuestions()
	b.Question(q)
	b.StartAnswers()
	if q.Type == dnsmessage.TypeA {
		b.AResource(
			dnsmessage.ResourceHeader{
				Name:  q.Name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   60,
			},
			dnsmessage.AResource{A: [4]byte{0, 0, 0, 0}},
		)
	} else if q.Type == dnsmessage.TypeAAAA {
		b.AAAAResource(
			dnsmessage.ResourceHeader{
				Name:  q.Name,
				Type:  dnsmessage.TypeAAAA,
				Class: dnsmessage.ClassINET,
				TTL:   60,
			},
			dnsmessage.AAAAResource{AAAA: [16]byte{0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0}},
		)
	}
	res, err := b.Finish()
	return res, domain, err
}

// SetDNSBypass configures DNS interception for wildcard/domain exclusions.
func (t *TunDevice) SetDNSBypass(exclusions []string, callback func(ips []net.IP)) {
	t.dnsExclusions = exclusions
	t.onDNSBypass = callback
}


func matchDomain(domain string, exclusions []string) bool {
	domain = strings.TrimSuffix(domain, ".")
	for _, exc := range exclusions {
		exc = strings.TrimSpace(exc)
		if exc == "" { continue }
		if strings.HasPrefix(exc, "*.") {
			suffix := strings.TrimPrefix(exc, "*")
			if strings.HasSuffix(domain, suffix) || domain == exc[2:] {
				return true
			}
		} else if domain == exc {
			return true
		}
	}
	return false
}

func parseDNSResponse(buf []byte, exclusions []string) []net.IP {
	if len(exclusions) == 0 {
		return nil
	}
	var p dnsmessage.Parser
	header, err := p.Start(buf)
	if err != nil || !header.Response {
		return nil
	}
	q, err := p.Question()
	if err != nil {
		return nil
	}
	domain := q.Name.String()
	if !matchDomain(domain, exclusions) {
		return nil
	}
	
	p.SkipAllQuestions()
	var ips []net.IP
	for {
		h, err := p.AnswerHeader()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			break
		}
		if h.Type == dnsmessage.TypeA {
			res, err := p.AResource()
			if err == nil {
				ips = append(ips, net.IP(res.A[:]))
			}
		} else if h.Type == dnsmessage.TypeAAAA {
			res, err := p.AAAAResource()
			if err == nil {
				ips = append(ips, net.IP(res.AAAA[:]))
			}
		} else {
			p.SkipAnswer()
		}
	}
	return ips
}

// SetUpstreamDNS sets the upstream DNS server for the relay.
func (t *TunDevice) SetUpstreamDNS(dns string) {
	t.upstreamDNS = dns
}

// SetAdBlocker configures the DNS-level ad blocker.
func (t *TunDevice) SetAdBlocker(a *AdBlocker) {
	t.adBlocker = a
}
