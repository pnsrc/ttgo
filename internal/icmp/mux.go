package icmp

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/pnsrc/ttgo/internal/config"
)

// Mux — глобальный raw socket для ICMP, фабрика per-session handler'ов.
// Один сокет на весь процесс, ответы роутятся по id+sessionID.
type Mux struct {
	conn    *icmp.PacketConn
	timeout time.Duration

	mu      sync.RWMutex
	pending map[pendingKey]chan replyMsg
}

type pendingKey struct {
	sessionID uint64
	id        uint16
}

type replyMsg struct {
	srcIP net.IP
	typ   uint8
	code  uint8
	seq   uint16
}

var globalSessionID atomic.Uint64

func NewMux(cfg *config.ICMPConfig) (*Mux, error) {
	timeout := 3 * time.Second
	if cfg != nil && cfg.RequestTimeoutSecs > 0 {
		timeout = time.Duration(cfg.RequestTimeoutSecs) * time.Second
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}

	m := &Mux{
		conn:    conn,
		timeout: timeout,
		pending: make(map[pendingKey]chan replyMsg),
	}
	go m.readLoop()
	return m, nil
}

// Session — per-connection ICMP handler.
type Session struct {
	mux       *Mux
	sessionID uint64
}

func (m *Mux) NewSession() *Session {
	return &Session{
		mux:       m,
		sessionID: globalSessionID.Add(1),
	}
}

func (s *Session) Close() {} // ресурсы освобождаются автоматически через pending timeout

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	for {
		id, dstIP, seq, ttl, err := readEchoRequest(r.Body)
		if err != nil {
			if err != io.EOF {
				slog.Warn("icmp: read", "err", err)
			}
			return
		}

		key := pendingKey{s.sessionID, id}
		ch := make(chan replyMsg, 1)

		s.mux.mu.Lock()
		s.mux.pending[key] = ch
		s.mux.mu.Unlock()

		go func() {
			defer func() {
				s.mux.mu.Lock()
				delete(s.mux.pending, key)
				s.mux.mu.Unlock()
			}()

			if err := s.mux.send(dstIP, id, seq, ttl); err != nil {
				slog.Warn("icmp: send", "err", err)
				return
			}

			select {
			case reply := <-ch:
				if err := writeEchoReply(w, id, reply); err != nil {
					return
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			case <-time.After(s.mux.timeout):
			case <-r.Context().Done():
			}
		}()
	}
}

func (m *Mux) send(dst net.IP, id, seq uint16, ttl uint8) error {
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: int(id), Seq: int(seq), Data: make([]byte, 56)},
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return err
	}
	m.conn.IPv4PacketConn().SetTTL(int(ttl))
	_, err = m.conn.WriteTo(b, &net.IPAddr{IP: dst})
	return err
}

func (m *Mux) readLoop() {
	buf := make([]byte, 1500)
	for {
		n, peer, err := m.conn.ReadFrom(buf)
		if err != nil {
			if !isClosedErr(err) {
				slog.Warn("icmp: readloop", "err", err)
			}
			return
		}
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}
		echo, ok := msg.Body.(*icmp.Echo)
		if !ok {
			continue
		}

		id := uint16(echo.ID)
		seq := uint16(echo.Seq)
		typ := uint8(msg.Type.(ipv4.ICMPType))
		code := uint8(msg.Code)
		srcIP := peer.(*net.IPAddr).IP.To16()

		reply := replyMsg{srcIP: srcIP, typ: typ, code: code, seq: seq}

		// Рассылаем всем сессиям у которых есть pending с этим id.
		// (разные клиенты могут использовать одинаковый ICMP id)
		m.mu.RLock()
		for k, ch := range m.pending {
			if k.id == id {
				select {
				case ch <- reply:
				default:
				}
			}
		}
		m.mu.RUnlock()
	}
}

// readEchoRequest: [id:2][dstAddr:16][seq:2][ttl:1][dataSize:2]
func readEchoRequest(r io.Reader) (id uint16, dstIP net.IP, seq uint16, ttl uint8, err error) {
	buf := make([]byte, 2+16+2+1+2)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}
	id = binary.BigEndian.Uint16(buf[0:2])
	dstIP = net.IP(buf[2:18]).To16()
	seq = binary.BigEndian.Uint16(buf[18:20])
	ttl = buf[20]
	return
}

// writeEchoReply: [id:2][srcAddr:16][type:1][code:1][seq:2]
func writeEchoReply(w io.Writer, id uint16, r replyMsg) error {
	buf := make([]byte, 2+16+1+1+2)
	binary.BigEndian.PutUint16(buf[0:2], id)
	copy(buf[2:18], r.srcIP)
	buf[18] = r.typ
	buf[19] = r.code
	binary.BigEndian.PutUint16(buf[20:22], r.seq)
	_, err := w.Write(buf)
	return err
}

func isClosedErr(err error) bool {
	return err == os.ErrClosed || (err != nil && err.Error() == "use of closed network connection")
}
