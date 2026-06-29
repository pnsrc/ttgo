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

// DeviceLimitStore — опциональный интерфейс. Реализуется store'ами которые
// умеют возвращать лимит устройств. Если store не реализует — лимит = 0 (без ограничений).
type DeviceLimitStore interface {
	GetMaxDevices(ctx context.Context, username string) (int, error)
}

// LifecycleStore — расширенные поля для квот и сроков действия аккаунта.
// Опциональный интерфейс, реализуется только sqlite/postgres.
type LifecycleStore interface {
	// GetLifecycle возвращает enabled, expiresAt (Unix, 0 = бессрочно),
	// trafficLimitBytes (0 = безлимит), trafficUsedBytes.
	GetLifecycle(ctx context.Context, username string) (enabled bool, expiresAt int64, trafficLimit, trafficUsed uint64, err error)
}

type cacheEntry struct {
	password         string
	maxDevices       int
	enabled          bool
	expiresAt        int64
	trafficLimit     uint64
	trafficUsed      uint64
	hasLifecycle     bool
	cacheExpiresAt   time.Time
	notFound         bool // негативный кэш
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

// DenyReason — причина отказа в auth.
type DenyReason string

const (
	DenyNone          DenyReason = ""
	DenyBadCreds      DenyReason = "Invalid credentials"
	DenyDisabled      DenyReason = "Account disabled"
	DenyExpired       DenyReason = "Account expired"
	DenyTrafficLimit  DenyReason = "Traffic limit reached"
)

// CheckResult — расширенный результат auth.
type CheckResult struct {
	Username    string
	MaxDevices  int    // 0 = unlimited
	Deny        DenyReason
}

func (a *Authenticator) Check(r *http.Request) string {
	res := a.CheckExt(r)
	return res.Username
}

// CheckExt возвращает username, device limit и причину отказа (если есть).
func (a *Authenticator) CheckExt(r *http.Request) CheckResult {
	hdr := r.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(hdr, "Basic ") {
		return CheckResult{Deny: DenyBadCreds}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(hdr, "Basic "))
	if err != nil {
		return CheckResult{Deny: DenyBadCreds}
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return CheckResult{Deny: DenyBadCreds}
	}
	username, password := parts[0], parts[1]

	entry, ok := a.lookupEntry(r.Context(), username)
	if !ok {
		return CheckResult{Deny: DenyBadCreds}
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(entry.password)) != 1 {
		return CheckResult{Deny: DenyBadCreds}
	}
	// Lifecycle checks
	if entry.hasLifecycle {
		if !entry.enabled {
			return CheckResult{Username: username, Deny: DenyDisabled}
		}
		if entry.expiresAt > 0 && time.Now().Unix() >= entry.expiresAt {
			return CheckResult{Username: username, Deny: DenyExpired}
		}
		if entry.trafficLimit > 0 && entry.trafficUsed >= entry.trafficLimit {
			return CheckResult{Username: username, Deny: DenyTrafficLimit}
		}
	}
	return CheckResult{Username: username, MaxDevices: entry.maxDevices}
}

func (a *Authenticator) lookupEntry(ctx context.Context, username string) (cacheEntry, bool) {
	a.mu.RLock()
	entry, hit := a.cache[username]
	a.mu.RUnlock()

	if hit && time.Now().Before(entry.cacheExpiresAt) {
		if entry.notFound {
			return cacheEntry{}, false
		}
		return entry, true
	}

	pw, err := a.store.GetPassword(ctx, username)
	if err != nil {
		slog.Warn("auth: store error", "user", username, "err", err)
		if hit && !entry.notFound {
			return entry, true
		}
		return cacheEntry{}, false
	}

	if pw == "" {
		exp := time.Now().Add(a.ttl)
		a.mu.Lock()
		a.cache[username] = cacheEntry{notFound: true, cacheExpiresAt: exp}
		a.mu.Unlock()
		return cacheEntry{}, false
	}

	out := cacheEntry{password: pw}
	if dls, ok := a.store.(DeviceLimitStore); ok {
		if md, err := dls.GetMaxDevices(ctx, username); err == nil {
			out.maxDevices = md
		}
	}
	if lcs, ok := a.store.(LifecycleStore); ok {
		if enabled, expAt, limit, used, err := lcs.GetLifecycle(ctx, username); err == nil {
			out.enabled = enabled
			out.expiresAt = expAt
			out.trafficLimit = limit
			out.trafficUsed = used
			out.hasLifecycle = true
		}
	}
	out.cacheExpiresAt = time.Now().Add(a.ttl)
	a.mu.Lock()
	a.cache[username] = out
	a.mu.Unlock()
	return out, true
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
			if now.After(v.cacheExpiresAt) {
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
