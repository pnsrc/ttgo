package server

import (
	"encoding/binary"
	"net"
	"sync"
)

// ConnTracker tracks active HTTP/2 connections by username.
// Used to send GOAWAY when a user is removed/invalidated.
type ConnTracker struct {
	mu    sync.RWMutex
	conns map[string][]net.Conn // username → list of raw connections
}

var GlobalConnTracker = &ConnTracker{
	conns: make(map[string][]net.Conn),
}

// Register associates a connection with a username.
// Call after successful auth.
func (t *ConnTracker) Register(username string, conn net.Conn) {
	t.mu.Lock()
	t.conns[username] = append(t.conns[username], conn)
	t.mu.Unlock()
}

// Unregister removes a connection from tracking (on close).
func (t *ConnTracker) Unregister(username string, conn net.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	list := t.conns[username]
	for i, c := range list {
		if c == conn {
			t.conns[username] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(t.conns[username]) == 0 {
		delete(t.conns, username)
	}
}

// KickUser sends GOAWAY to all connections for username and closes them.
func (t *ConnTracker) KickUser(username string) {
	t.mu.Lock()
	list := t.conns[username]
	delete(t.conns, username)
	t.mu.Unlock()

	frame := goawayFrame(0, 0x1F)
	for _, conn := range list {
		conn.Write(frame)
		conn.Close()
	}
}

// goawayFrame builds an HTTP/2 GOAWAY frame.
func goawayFrame(lastStreamID, errorCode uint32) []byte {
	f := make([]byte, 17)
	f[0], f[1], f[2] = 0, 0, 8 // length = 8
	f[3] = 0x7                  // type GOAWAY
	f[4] = 0                    // flags
	binary.BigEndian.PutUint32(f[5:9], 0)
	binary.BigEndian.PutUint32(f[9:13], lastStreamID&0x7FFFFFFF)
	binary.BigEndian.PutUint32(f[13:17], errorCode)
	return f
}

// ActiveUsers returns a snapshot of currently connected usernames.
func (t *ConnTracker) ActiveUsers() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]int, len(t.conns))
	for u, conns := range t.conns {
		out[u] = len(conns)
	}
	return out
}
