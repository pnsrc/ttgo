package server

import (
	"context"
	"log/slog"
	"sync"

	icmpmux "github.com/pnsrc/ttgo/internal/icmp"
	udpmux "github.com/pnsrc/ttgo/internal/udp"
)

// Session — per-TLS-connection state.
// Создаётся в ConnContext, живёт пока живёт соединение.
// Cleanup вешается на ctx.Done() соединения.
type Session struct {
	mu   sync.Mutex
	udp  *udpmux.Session
	icmp *icmpmux.Session
}

func newSession(ctx context.Context) *Session {
	s := &Session{}
	// Когда соединение закрывается — освобождаем все UDP сокеты
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.udp != nil {
			s.udp.Close()
			slog.Debug("session: UDP closed")
		}
	}()
	return s
}

// UDP возвращает per-session UDP tracker, создаёт при первом вызове.
func (s *Session) UDP(factory *udpmux.Mux) *udpmux.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udp == nil {
		s.udp = factory.NewSession()
	}
	return s.udp
}

// ICMP возвращает per-session ICMP handler, создаёт при первом вызове.
func (s *Session) ICMP(factory *icmpmux.Mux) *icmpmux.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.icmp == nil {
		s.icmp = factory.NewSession()
	}
	return s.icmp
}
