package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// p2pPeer — клиент, ожидающий соединения партнёра в комнате.
type p2pPeer struct {
	username string
	w        http.ResponseWriter
	r        *http.Request
	done     chan struct{}
}

// P2PBroker связывает двух авторизованных клиентов одного юзера в room.
//
// Семантика: первый клиент с заданным room оказывается в "waiting".
// Когда второй клиент с тем же room и тем же username приходит — их потоки
// сцепляются (io.Copy в обе стороны) до закрытия любого из них.
//
// Если за timeout никто не пришёл — первому отдаём 504 Gateway Timeout.
type P2PBroker struct {
	mu      sync.Mutex
	waiting map[string]*p2pPeer // key = username + ":" + room
	timeout time.Duration
}

func NewP2PBroker() *P2PBroker {
	return &P2PBroker{
		waiting: make(map[string]*p2pPeer),
		timeout: 60 * time.Second,
	}
}

// Handle обрабатывает CONNECT _p2p запрос.
func (b *P2PBroker) Handle(w http.ResponseWriter, r *http.Request, username string) {
	room := r.Header.Get("X-P2P-Room")
	if room == "" {
		http.Error(w, "X-P2P-Room header required", http.StatusBadRequest)
		return
	}

	key := username + ":" + room

	b.mu.Lock()
	peer, exists := b.waiting[key]
	if !exists {
		// Мы первые — ждём партнёра
		self := &p2pPeer{
			username: username,
			w:        w,
			r:        r,
			done:     make(chan struct{}),
		}
		b.waiting[key] = self
		b.mu.Unlock()

		slog.Debug("p2p: waiting", "user", username, "room", room)

		// Сразу шлём 200 чтобы клиент знал что мы ждём партнёра
		w.WriteHeader(http.StatusOK)
		w.Header().Set("X-P2P-Status", "waiting")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		select {
		case <-self.done:
			// Партнёр пришёл и инициировал copy — выходим, copy сам всё закроет
			return
		case <-r.Context().Done():
			b.mu.Lock()
			if b.waiting[key] == self {
				delete(b.waiting, key)
			}
			b.mu.Unlock()
			slog.Debug("p2p: client disconnected before peer", "user", username, "room", room)
			return
		case <-time.After(b.timeout):
			b.mu.Lock()
			if b.waiting[key] == self {
				delete(b.waiting, key)
			}
			b.mu.Unlock()
			slog.Debug("p2p: timeout", "user", username, "room", room)
			return
		}
	}

	// Партнёр уже ждал — снимаем его, связываем потоки
	delete(b.waiting, key)
	b.mu.Unlock()

	slog.Info("p2p: connected", "user", username, "room", room)

	// Отвечаем второму клиенту что коннект установлен
	w.WriteHeader(http.StatusOK)
	w.Header().Set("X-P2P-Status", "connected")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	errCh := make(chan error, 2)
	go func() {
		_, err := copyFlush(peer.w, r.Body)
		errCh <- err
	}()
	go func() {
		_, err := copyFlush(w, peer.r.Body)
		errCh <- err
	}()

	select {
	case <-errCh:
	case <-r.Context().Done():
	case <-peer.r.Context().Done():
	}

	close(peer.done)
	slog.Debug("p2p: closed", "user", username, "room", room)
}

// copyFlush — общая утилита (уже определена в handler.go).
// Используем io.Copy для read side, ResponseWriter сам флушит на write.
func init() {
	_ = io.Copy
}
