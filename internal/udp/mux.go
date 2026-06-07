package udp

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// Mux — фабрика per-session UDP tracker'ов.
type Mux struct {
	idleTimeout time.Duration
}

func NewMux(idleTimeoutSecs int) *Mux {
	d := time.Duration(idleTimeoutSecs) * time.Second
	if d == 0 {
		d = 300 * time.Second
	}
	return &Mux{idleTimeout: d}
}

// Session — изолированный UDP tracker для одного TLS соединения.
// Каждый клиент получает свой, коллизии connKey между клиентами исключены.
type Session struct {
	mu          sync.Mutex
	conns       map[connKey]*udpConn
	idleTimeout time.Duration
	closed      bool
}

type connKey struct {
	srcAddr string
	srcPort uint16
	dstAddr string
	dstPort uint16
}

type udpConn struct {
	conn     *net.UDPConn
	lastSeen time.Time
}

func NewSession(idleTimeout time.Duration) *Session {
	s := &Session{
		conns:       make(map[connKey]*udpConn),
		idleTimeout: idleTimeout,
	}
	go s.reaper()
	return s
}

// NewSession реализует server.SessionMux — каждый вызов даёт новый Session.
func (m *Mux) NewSession() *Session {
	return NewSession(m.idleTimeout)
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for _, c := range s.conns {
		c.conn.Close()
	}
	s.conns = nil
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	defer s.Close()

	for {
		payload, srcIP, dstIP, srcPort, dstPort, err := readOutgoing(r.Body)
		if err != nil {
			if err != io.EOF {
				slog.Warn("udp: read", "err", err)
			}
			return
		}

		key := connKey{srcIP.String(), srcPort, dstIP.String(), dstPort}
		uc, err := s.getOrDial(key, dstIP, dstPort, w)
		if err != nil {
			slog.Warn("udp: dial", "err", err)
			continue
		}

		s.mu.Lock()
		uc.lastSeen = time.Now()
		s.mu.Unlock()

		if _, err := uc.conn.Write(payload); err != nil {
			slog.Warn("udp: write", "err", err)
			s.remove(key)
		}
	}
}

func (s *Session) getOrDial(key connKey, dstIP net.IP, dstPort uint16, w http.ResponseWriter) (*udpConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("session closed")
	}
	if uc, ok := s.conns[key]; ok {
		return uc, nil
	}

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: dstIP, Port: int(dstPort)})
	if err != nil {
		return nil, err
	}

	uc := &udpConn{conn: conn, lastSeen: time.Now()}
	s.conns[key] = uc

	// Читаем ответы от target и шлём клиенту
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			srcIP16 := addr.IP.To16()
			dstIP16 := net.ParseIP(key.srcAddr).To16()
			if err := writeIncoming(w, srcIP16, uint16(addr.Port), dstIP16, key.srcPort, buf[:n]); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}()

	return uc, nil
}

func (s *Session) remove(key connKey) {
	s.mu.Lock()
	if uc, ok := s.conns[key]; ok {
		uc.conn.Close()
		delete(s.conns, key)
	}
	s.mu.Unlock()
}

func (s *Session) reaper() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		now := time.Now()
		for k, uc := range s.conns {
			if now.Sub(uc.lastSeen) > s.idleTimeout {
				uc.conn.Close()
				delete(s.conns, k)
			}
		}
		s.mu.Unlock()
	}
}

// readOutgoing: [length:4][srcAddr:16][srcPort:2][dstAddr:16][dstPort:2][appNameLen:1][appName:L][payload:N]
func readOutgoing(r io.Reader) (payload []byte, srcIP, dstIP net.IP, srcPort, dstPort uint16, err error) {
	var hdr [4]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return
	}
	body := make([]byte, binary.BigEndian.Uint32(hdr[:]))
	if _, err = io.ReadFull(r, body); err != nil {
		return
	}
	if len(body) < 37 {
		err = fmt.Errorf("udp: packet too short")
		return
	}
	srcIP = net.IP(body[0:16]).To16()
	srcPort = binary.BigEndian.Uint16(body[16:18])
	dstIP = net.IP(body[18:34]).To16()
	dstPort = binary.BigEndian.Uint16(body[34:36])
	appNameLen := int(body[36])
	start := 37 + appNameLen
	if len(body) < start {
		err = fmt.Errorf("udp: truncated")
		return
	}
	payload = body[start:]
	return
}

// writeIncoming: [length:4][srcAddr:16][srcPort:2][dstAddr:16][dstPort:2][payload:N]
func writeIncoming(w io.Writer, srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, payload []byte) error {
	inner := 16 + 2 + 16 + 2 + len(payload)
	buf := make([]byte, 4+inner)
	binary.BigEndian.PutUint32(buf[0:4], uint32(inner))
	copy(buf[4:20], srcIP.To16())
	binary.BigEndian.PutUint16(buf[20:22], srcPort)
	copy(buf[22:38], dstIP.To16())
	binary.BigEndian.PutUint16(buf[38:40], dstPort)
	copy(buf[40:], payload)
	_, err := w.Write(buf)
	return err
}
