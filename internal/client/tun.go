package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

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
	dev         tun.Device
	name        string
	stack       *stack.Stack
	link        *channel.Endpoint
	dialer      *Dialer
	stats       *Stats
	ctx         context.Context
	cancel      context.CancelFunc

	wg sync.WaitGroup
}

// OpenTUN creates a TUN device, brings it up, sets up gvisor netstack,
// и регистрирует TCP forwarder который для каждого нового flow вызовет
// dialer.DialContext.
func OpenTUN(ctx context.Context, dialer *Dialer, stats *Stats) (*TunDevice, error) {
	// "utun" на macOS = система сама выбирает номер (utun5, utun6, ...)
	dev, err := tun.CreateTUN("utun", defaultMTU)
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

// pumpTUNToStack читает пакеты из TUN и инжектит их в netstack.
func (t *TunDevice) pumpTUNToStack() {
	defer t.wg.Done()
	// wireguard/tun.Device.Read принимает batch буферов.
	bufs := make([][]byte, 1)
	sizes := make([]int, 1)
	bufs[0] = make([]byte, defaultMTU+16) // +16 для возможных tun-overhead байтов на macOS

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}
		n, err := t.dev.Read(bufs, sizes, 4) // 4 = offset (macOS utun header)
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
		pktData := bufs[0][4 : 4+sizes[0]] // снимаем utun префикс

		// Определяем version (IPv4 vs IPv6) по первым 4 битам.
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

// pumpStackToTUN читает пакеты из netstack и пишет в TUN.
func (t *TunDevice) pumpStackToTUN() {
	defer t.wg.Done()
	for {
		pkt := t.link.ReadContext(t.ctx)
		if pkt == nil {
			return
		}
		// Собираем slices из packet buffer'a в один []byte с префиксом utun (4 байта).
		buf := make([]byte, 4, 4+pkt.Size())
		// utun header: AF_INET (2) или AF_INET6 (30) в network byte order.
		switch pkt.NetworkProtocolNumber {
		case ipv4.ProtocolNumber:
			buf[3] = 2 // syscall.AF_INET
		case ipv6.ProtocolNumber:
			buf[3] = 30 // syscall.AF_INET6
		}
		for _, v := range pkt.AsSlices() {
			buf = append(buf, v...)
		}
		pkt.DecRef()
		if _, err := t.dev.Write([][]byte{buf}, 4); err != nil {
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

	ctx, cancel := context.WithTimeout(t.ctx, 15*time.Second)
	remote, err := t.dialer.DialContext(ctx, target)
	cancel()
	if err != nil {
		slog.Debug("dial endpoint failed", "target", target, "err", err)
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(remote, localConn); done <- struct{}{} }()
	go func() { io.Copy(localConn, remote); done <- struct{}{} }()
	<-done
}

// handleUDP — заглушка: пока что роняем UDP (включая DNS).
// Полноценная имплементация требует _udp2 псевдо-хост туннелирование.
// Возвращаем false (не обработано) — netstack отвечает ICMP unreachable.
func (t *TunDevice) handleUDP(r *udp.ForwarderRequest) bool {
	id := r.ID()
	slog.Debug("udp dropped", "dst", net.JoinHostPort(id.LocalAddress.String(), fmt.Sprintf("%d", id.LocalPort)))
	return false
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
