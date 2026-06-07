package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// UserStore — реализуй под любую БД.
type UserStore interface {
	// GetPassword возвращает хэш/пароль для username, "" если не найден.
	GetPassword(ctx context.Context, username string) (password string, err error)
}

type cacheEntry struct {
	password  string
	expiresAt time.Time
	notFound  bool // негативный кэш
}

type Authenticator struct {
	store    UserStore
	ttl      time.Duration

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func New(store UserStore, cacheTTL time.Duration) *Authenticator {
	if cacheTTL == 0 {
		cacheTTL = 30 * time.Second
	}
	a := &Authenticator{
		store: store,
		ttl:   cacheTTL,
		cache: make(map[string]cacheEntry),
	}
	go a.evictLoop()
	return a
}

func (a *Authenticator) Check(r *http.Request) string {
	hdr := r.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(hdr, "Basic ") {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(hdr, "Basic "))
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return ""
	}
	username, password := parts[0], parts[1]

	stored, ok := a.lookup(r.Context(), username)
	if !ok {
		return ""
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(stored)) != 1 {
		return ""
	}
	return username
}

func (a *Authenticator) lookup(ctx context.Context, username string) (string, bool) {
	// Читаем из кэша
	a.mu.RLock()
	entry, hit := a.cache[username]
	a.mu.RUnlock()

	if hit && time.Now().Before(entry.expiresAt) {
		if entry.notFound {
			return "", false
		}
		return entry.password, true
	}

	// Промах — идём в store
	pw, err := a.store.GetPassword(ctx, username)
	if err != nil {
		slog.Warn("auth: store error", "user", username, "err", err)
		// При ошибке БД используем протухший кэш если есть
		if hit && !entry.notFound {
			return entry.password, true
		}
		return "", false
	}

	exp := time.Now().Add(a.ttl)
	a.mu.Lock()
	if pw == "" {
		a.cache[username] = cacheEntry{notFound: true, expiresAt: exp}
	} else {
		a.cache[username] = cacheEntry{password: pw, expiresAt: exp}
	}
	a.mu.Unlock()

	return pw, pw != ""
}

// Invalidate сбрасывает кэш для конкретного пользователя.
// Вызывай после изменения пароля чтобы не ждать TTL.
func (a *Authenticator) Invalidate(username string) {
	a.mu.Lock()
	delete(a.cache, username)
	a.mu.Unlock()
}

// InvalidateAll сбрасывает весь кэш.
func (a *Authenticator) InvalidateAll() {
	a.mu.Lock()
	a.cache = make(map[string]cacheEntry)
	a.mu.Unlock()
}

func (a *Authenticator) evictLoop() {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		a.mu.Lock()
		for k, v := range a.cache {
			if now.After(v.expiresAt) {
				delete(a.cache, k)
			}
		}
		a.mu.Unlock()
	}
}

// OnInvalidate — опциональный callback вызывается когда credentials
// для username инвалидируются. Используй для принудительной отправки
// GOAWAY активным соединениям.
var OnInvalidate func(username string)

// InvalidateWithNotify сбрасывает кэш и вызывает OnInvalidate если установлен.
func (a *Authenticator) InvalidateWithNotify(username string) {
	a.Invalidate(username)
	if OnInvalidate != nil {
		OnInvalidate(username)
	}
}
