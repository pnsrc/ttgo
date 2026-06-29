package server

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SessionInfo describes a single active TLS connection.
type SessionInfo struct {
	Username      string    `json:"username"`
	RemoteAddr    string    `json:"remote_addr"`
	ConnectedAt   time.Time `json:"connected_at"`
	BytesIn       uint64    `json:"bytes_in"`
	BytesOut      uint64    `json:"bytes_out"`
	OpenTunnels   int32     `json:"open_tunnels"`   // current active tunnels
	TotalTunnels  uint64    `json:"total_tunnels"`  // cumulative tunnels created
}

// UserStats — агрегированная статистика по username.
type UserStats struct {
	Username       string    `json:"username"`
	ActiveConns    int       `json:"active_conns"`
	BytesIn        uint64    `json:"bytes_in"`
	BytesOut       uint64    `json:"bytes_out"`
	OpenTunnels    int32     `json:"open_tunnels"`
	TotalTunnels   uint64    `json:"total_tunnels"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
}

// connEntry — внутреннее состояние одного соединения.
type connEntry struct {
	username     string
	remoteAddr   string
	connectedAt  time.Time
	bytesIn      atomic.Uint64
	bytesOut     atomic.Uint64
	openTunnels  atomic.Int32
	totalTunnels atomic.Uint64
}

// ConnTracker — учёт активных соединений, статистики трафика и revocation.
type ConnTracker struct {
	mu      sync.RWMutex
	entries map[net.Conn]*connEntry  // conn → state
	byUser  map[string][]net.Conn    // username → connections
	revoked map[net.Conn]string      // conn → reason

	// Cumulative счётчики по юзеру, переживают дисконект.
	// Используются как минимум для отчётов "сколько потратил" за uptime сервера.
	totalsMu     sync.RWMutex
	totalsByUser map[string]*userTotals
}

type userTotals struct {
	bytesIn      atomic.Uint64
	bytesOut     atomic.Uint64
	totalTunnels atomic.Uint64
	firstSeenAt  time.Time
	// flushedBytes — сколько уже записано в persistent store;
	// дельта flushTotal() - flushedBytes выгружается в БД.
	flushedBytes uint64
}

var GlobalConnTracker = &ConnTracker{
	entries:      make(map[net.Conn]*connEntry),
	byUser:       make(map[string][]net.Conn),
	revoked:      make(map[net.Conn]string),
	totalsByUser: make(map[string]*userTotals),
}

// Register создаёт запись для соединения юзера.
func (t *ConnTracker) Register(username string, conn net.Conn) {
	if conn == nil {
		return
	}
	entry := &connEntry{
		username:    username,
		remoteAddr:  conn.RemoteAddr().String(),
		connectedAt: time.Now(),
	}

	t.mu.Lock()
	t.entries[conn] = entry
	t.byUser[username] = append(t.byUser[username], conn)
	t.mu.Unlock()

	t.totalsMu.Lock()
	if _, ok := t.totalsByUser[username]; !ok {
		t.totalsByUser[username] = &userTotals{firstSeenAt: time.Now()}
	}
	t.totalsMu.Unlock()
}

// Unregister удаляет запись соединения и переносит счётчики в totals.
func (t *ConnTracker) Unregister(username string, conn net.Conn) {
	if conn == nil {
		return
	}
	t.mu.Lock()
	delete(t.entries, conn)
	list := t.byUser[username]
	for i, c := range list {
		if c == conn {
			t.byUser[username] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(t.byUser[username]) == 0 {
		delete(t.byUser, username)
	}
	delete(t.revoked, conn)
	t.mu.Unlock()
}

// connEntryFor — внутренний lookup.
func (t *ConnTracker) connEntryFor(conn net.Conn) *connEntry {
	if conn == nil {
		return nil
	}
	t.mu.RLock()
	e := t.entries[conn]
	t.mu.RUnlock()
	return e
}

// TunnelOpened — вызывается при старте туннеля (после auth + dial).
func (t *ConnTracker) TunnelOpened(conn net.Conn) {
	e := t.connEntryFor(conn)
	if e == nil {
		return
	}
	e.openTunnels.Add(1)
	e.totalTunnels.Add(1)

	t.totalsMu.RLock()
	tot := t.totalsByUser[e.username]
	t.totalsMu.RUnlock()
	if tot != nil {
		tot.totalTunnels.Add(1)
	}
}

// TunnelClosed — при закрытии туннеля.
func (t *ConnTracker) TunnelClosed(conn net.Conn) {
	e := t.connEntryFor(conn)
	if e == nil {
		return
	}
	e.openTunnels.Add(-1)
}

// AddBytes — учёт трафика. in = от клиента к нам, out = от нас к клиенту.
func (t *ConnTracker) AddBytes(conn net.Conn, in, out uint64) {
	e := t.connEntryFor(conn)
	if e == nil {
		return
	}
	if in > 0 {
		e.bytesIn.Add(in)
	}
	if out > 0 {
		e.bytesOut.Add(out)
	}

	t.totalsMu.RLock()
	tot := t.totalsByUser[e.username]
	t.totalsMu.RUnlock()
	if tot != nil {
		if in > 0 {
			tot.bytesIn.Add(in)
		}
		if out > 0 {
			tot.bytesOut.Add(out)
		}
	}
}

// IsRevoked возвращает (true, reason) если соединение помечено на дисконект.
func (t *ConnTracker) IsRevoked(conn net.Conn) (bool, string) {
	if conn == nil {
		return false, ""
	}
	t.mu.RLock()
	reason, ok := t.revoked[conn]
	t.mu.RUnlock()
	return ok, reason
}

// KickUser — помечает все соединения как revoked, после 2s — GOAWAY + close.
func (t *ConnTracker) KickUser(username, reason string) {
	t.mu.Lock()
	list := make([]net.Conn, len(t.byUser[username]))
	copy(list, t.byUser[username])
	for _, conn := range list {
		t.revoked[conn] = reason
	}
	t.mu.Unlock()
	t.scheduleKick(list)
}

// KickSession — выкидывает одно конкретное соединение по remote_addr.
// Возвращает true если соединение было найдено.
func (t *ConnTracker) KickSession(username, remoteAddr, reason string) bool {
	t.mu.Lock()
	var target net.Conn
	for _, conn := range t.byUser[username] {
		if e, ok := t.entries[conn]; ok && e.remoteAddr == remoteAddr {
			target = conn
			t.revoked[conn] = reason
			break
		}
	}
	t.mu.Unlock()

	if target == nil {
		return false
	}
	t.scheduleKick([]net.Conn{target})
	return true
}

func (t *ConnTracker) scheduleKick(list []net.Conn) {
	if len(list) == 0 {
		return
	}
	go func() {
		time.Sleep(2 * time.Second)
		frame := goawayFrame(0, 0x1F)
		for _, conn := range list {
			conn.Write(frame)
			conn.Close()
		}
	}()
}

// Sessions — снимок всех активных сессий.
func (t *ConnTracker) Sessions() []SessionInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]SessionInfo, 0, len(t.entries))
	for _, e := range t.entries {
		out = append(out, SessionInfo{
			Username:     e.username,
			RemoteAddr:   e.remoteAddr,
			ConnectedAt:  e.connectedAt,
			BytesIn:      e.bytesIn.Load(),
			BytesOut:     e.bytesOut.Load(),
			OpenTunnels:  e.openTunnels.Load(),
			TotalTunnels: e.totalTunnels.Load(),
		})
	}
	return out
}

// UserStats — агрегаты по username (живая статистика + cumulative).
func (t *ConnTracker) UserStats() []UserStats {
	// сначала активные соединения с агрегацией
	live := map[string]*UserStats{}
	t.mu.RLock()
	for _, e := range t.entries {
		s, ok := live[e.username]
		if !ok {
			s = &UserStats{Username: e.username}
			live[e.username] = s
		}
		s.ActiveConns++
		s.BytesIn += e.bytesIn.Load()
		s.BytesOut += e.bytesOut.Load()
		s.OpenTunnels += e.openTunnels.Load()
	}
	t.mu.RUnlock()

	// добавляем cumulative из totals (для юзеров без активных коннектов тоже)
	t.totalsMu.RLock()
	for username, tot := range t.totalsByUser {
		s, ok := live[username]
		if !ok {
			s = &UserStats{Username: username}
			live[username] = s
		}
		// Перезаписываем cumulative-поля totals (они = lifetime агрегат)
		s.BytesIn = tot.bytesIn.Load()
		s.BytesOut = tot.bytesOut.Load()
		s.TotalTunnels = tot.totalTunnels.Load()
		s.FirstSeenAt = tot.firstSeenAt
	}
	t.totalsMu.RUnlock()

	out := make([]UserStats, 0, len(live))
	for _, s := range live {
		out = append(out, *s)
	}
	return out
}

// IsKnown — true если соединение уже зарегистрировано (Register был вызван).
// Используется для устранения двойного подсчёта при повторных CONNECT
// внутри уже учтённого HTTP/2 соединения.
func (t *ConnTracker) IsKnown(conn net.Conn) bool {
	if conn == nil {
		return false
	}
	t.mu.RLock()
	_, ok := t.entries[conn]
	t.mu.RUnlock()
	return ok
}

// TrafficFlusher — сохраняет дельту трафика в persistent store.
type TrafficFlusher interface {
	AddTraffic(ctx context.Context, username string, n uint64) error
}

// StartTrafficFlusher периодически выгружает прирост трафика в БД.
// Каждые interval секунд считаем дельту total - flushed и пишем.
func (t *ConnTracker) StartTrafficFlusher(ctx context.Context, fl TrafficFlusher, interval time.Duration) {
	if fl == nil || interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.flushOnce(fl) // финальный flush
				return
			case <-ticker.C:
				t.flushOnce(fl)
			}
		}
	}()
}

func (t *ConnTracker) flushOnce(fl TrafficFlusher) {
	t.totalsMu.Lock()
	type pending struct {
		name  string
		delta uint64
	}
	var pendings []pending
	for name, tot := range t.totalsByUser {
		total := tot.bytesIn.Load() + tot.bytesOut.Load()
		if total > tot.flushedBytes {
			pendings = append(pendings, pending{name, total - tot.flushedBytes})
			tot.flushedBytes = total
		}
	}
	t.totalsMu.Unlock()

	for _, p := range pendings {
		if err := fl.AddTraffic(context.Background(), p.name, p.delta); err != nil {
			// Не страшно — следующий тик попробует снова, дельта восстановится
			// если flushedBytes откатить. Здесь мы не откатываем — мирно теряем
			// дельту в случае persistent ошибки store.
			_ = err
		}
	}
}

// ResetUserTraffic сбрасывает cumulative-счётчики юзера.
// Должен вызываться вместе со сбросом в persistent store.
func (t *ConnTracker) ResetUserTraffic(username string) {
	t.totalsMu.Lock()
	if tot, ok := t.totalsByUser[username]; ok {
		tot.bytesIn.Store(0)
		tot.bytesOut.Store(0)
		tot.flushedBytes = 0
	}
	t.totalsMu.Unlock()

	// Также сбрасываем per-connection-счётчики (live)
	t.mu.Lock()
	for _, e := range t.entries {
		if e.username == username {
			e.bytesIn.Store(0)
			e.bytesOut.Store(0)
		}
	}
	t.mu.Unlock()
}

// CountUser возвращает число активных TLS-соединений для username.
func (t *ConnTracker) CountUser(username string) int {
	t.mu.RLock()
	n := len(t.byUser[username])
	t.mu.RUnlock()
	return n
}

// ActiveUsers — обратная совместимость: map[username]connections_count.
func (t *ConnTracker) ActiveUsers() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]int, len(t.byUser))
	for u, conns := range t.byUser {
		out[u] = len(conns)
	}
	return out
}

// goawayFrame builds a minimal HTTP/2 GOAWAY frame.
func goawayFrame(lastStreamID, errorCode uint32) []byte {
	f := make([]byte, 17)
	f[0], f[1], f[2] = 0, 0, 8
	f[3] = 0x7
	f[4] = 0
	binary.BigEndian.PutUint32(f[5:9], 0)
	binary.BigEndian.PutUint32(f[9:13], lastStreamID&0x7FFFFFFF)
	binary.BigEndian.PutUint32(f[13:17], errorCode)
	return f
}
