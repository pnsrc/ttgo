package server

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pnsrc/ttgo/internal/auth"
	"github.com/pnsrc/ttgo/internal/config"
)

// AdminUserStore — узкий интерфейс store'а для управления юзерами через admin API.
// Реализуется в cmd/endpoint/main.go (там доступен полный admin.UserStore).
type AdminUserStore interface {
	List() ([]AdminUser, error)
	Add(username, password string) error
	Delete(username string) error
	ChangePassword(username, password string) error
	SetMaxDevices(username string, n int) error
}

// AdminLifecycle — опциональный интерфейс для квот/срока действия/enable.
type AdminLifecycle interface {
	SetEnabled(username string, enabled bool) error
	SetExpiresAt(username string, expiresUnix int64) error
	SetTrafficLimit(username string, bytes uint64) error
	ResetTraffic(username string) error
}

// AdminUser — представление пользователя для admin API.
type AdminUser struct {
	Username        string `json:"username"`
	MaxDevices      int    `json:"max_devices"`
	Enabled         bool   `json:"enabled"`
	ExpiresAt       int64  `json:"expires_at"`        // unix, 0 = never
	TrafficLimit    uint64 `json:"traffic_limit"`     // bytes, 0 = unlimited
	TrafficUsed     uint64 `json:"traffic_used"`      // bytes (lifetime, can be reset)
}

var serverStartedAt = time.Now()

// patchConfigFn — установить из main.go (там доступен admin пакет).
// Принимает путь vpn.toml и raw JSON с изменениями (EditableConfig).
var patchConfigFn = func(path string, raw []byte) error {
	return fmt.Errorf("config edit not configured")
}

// SetPatchConfigFn устанавливает функцию-мост для PATCH /api/config.
func SetPatchConfigFn(f func(path string, raw []byte) error) {
	patchConfigFn = f
}

// ReadCertInfo читает PEM cert и возвращает срок и issuer.
type CertSummary struct {
	NotAfter time.Time
	Issuer   string
}

func ReadCertInfo(path string) (*CertSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, os.ErrInvalid
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &CertSummary{NotAfter: cert.NotAfter, Issuer: cert.Issuer.CommonName}, nil
}

// FullConfig — для /api/info, чтобы фронт мог показать настройки эндпоинта.
type FullConfig struct {
	ListenAddress    string `json:"listen_address"`
	StoreType        string `json:"store_type"`
	StoreDSN         string `json:"store_dsn"`
	CacheTTLSecs     int    `json:"cache_ttl_secs"`
	AllowPrivateNet  bool   `json:"allow_private_net"`
	IPv6Available    bool   `json:"ipv6_available"`
	TCPIdleTimeout   int    `json:"tcp_idle_timeout_secs"`
	UDPIdleTimeout   int    `json:"udp_idle_timeout_secs"`
	ConnectTimeout   int    `json:"connect_timeout_secs"`
	TLSHandshake     int    `json:"tls_handshake_timeout_secs"`
	HTTP3Enabled     bool   `json:"http3_enabled"`
	ICMPEnabled      bool   `json:"icmp_enabled"`
	Hostnames        []HostInfo `json:"hostnames"`
}

// HostInfo — сертификат + срок его действия.
type HostInfo struct {
	Hostname  string    `json:"hostname"`
	NotAfter  time.Time `json:"not_after"`
	Issuer    string    `json:"issuer"`
}

// RulesEngine — узкий интерфейс engine для admin API.
type RulesEngine interface {
	Snapshot() []RuleView
	Set(views []RuleView) error
	Path() string
}

type RuleView struct {
	CIDR               string `json:"cidr"`
	ClientRandomPrefix string `json:"client_random_prefix"`
	Action             string `json:"action"`
}

// StartAdminAPI запускает HTTP admin API на localhost.
// Если webRoot не пустой — раздаёт статику фронта.
// fullCfg — для /api/info (можно nil, тогда конфиг не отдаётся).
// vpnPath — путь к vpn.toml для редактирования через UI.
// restart — функция graceful exit для перезапуска через systemd.
func StartAdminAPI(cfg *config.AdminConfig, authn *auth.Authenticator, store AdminUserStore, webRoot string, fullCfg *FullConfig, tlsReload func() error, rulesEng RulesEngine, vpnPath string, restart func()) {
	if cfg == nil || cfg.Token == "" || cfg.Address == "" {
		return
	}

	mux := http.NewServeMux()

	authMW := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// CORS preflight
			if r.Method == http.MethodOptions {
				setCORS(w, cfg)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			setCORS(w, cfg)
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				token = r.URL.Query().Get("token") // для EventSource / debug
			}
			if token != cfg.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}

	// ── auth status ──────────────────────────────────────────────────────────

	mux.HandleFunc("/api/whoami", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok":            true,
			"version":       "ttgo",
			"has_userstore": store != nil,
		})
	}))

	// ── server info ──────────────────────────────────────────────────────────

	mux.HandleFunc("/api/info", authMW(func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(serverStartedAt).Round(time.Second)
		resp := map[string]any{
			"version":         "ttgo",
			"uptime_seconds":  int(uptime.Seconds()),
			"started_at":      serverStartedAt.Format(time.RFC3339),
			"has_userstore":   store != nil,
			"has_tls_reload":  tlsReload != nil,
			"config":          fullCfg,
		}
		writeJSON(w, resp)
	}))

	// ── TLS hot reload ───────────────────────────────────────────────────────

	mux.HandleFunc("/api/tls/reload", authMW(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if tlsReload == nil {
			http.Error(w, "tls reload not configured", http.StatusServiceUnavailable)
			return
		}
		if err := tlsReload(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "reloaded"})
	}))

	// ── system info ──────────────────────────────────────────────────────────

	mux.HandleFunc("/api/system", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, gatherSystem())
	}))

	// ── edit vpn.toml ────────────────────────────────────────────────────────

	mux.HandleFunc("/api/config", authMW(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if vpnPath == "" {
			http.Error(w, "config path unknown", http.StatusServiceUnavailable)
			return
		}
		// Декодируем raw JSON и пробрасываем в admin.PatchVPNConfig через адаптер.
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := patchConfigFn(vpnPath, []byte(raw)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "saved", "path": vpnPath})
	}))

	// ── graceful restart ─────────────────────────────────────────────────────

	mux.HandleFunc("/api/restart", authMW(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if restart == nil {
			http.Error(w, "restart not configured", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]string{"status": "restarting"})
		// Даём время отправить response клиенту, потом просим main завершиться.
		go func() {
			time.Sleep(500 * time.Millisecond)
			restart()
		}()
	}))

	// ── routing rules ────────────────────────────────────────────────────────

	mux.HandleFunc("/api/rules", authMW(func(w http.ResponseWriter, r *http.Request) {
		if rulesEng == nil {
			http.Error(w, "rules engine unavailable", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{
				"path":  rulesEng.Path(),
				"rules": rulesEng.Snapshot(),
			})
		case http.MethodPut:
			var req struct {
				Rules []RuleView `json:"rules"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			// Адаптер: server.RuleView → rules.RuleView
			out := make([]RuleView, len(req.Rules))
			for i, r := range req.Rules {
				if r.Action != "allow" && r.Action != "deny" {
					http.Error(w, fmt.Sprintf("rule %d: action must be allow or deny", i), http.StatusBadRequest)
					return
				}
				out[i] = r
			}
			if err := rulesEng.Set(out); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]string{"status": "saved"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// ── sessions ─────────────────────────────────────────────────────────────

	mux.HandleFunc("/api/sessions", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, GlobalConnTracker.Sessions())
	}))

	mux.HandleFunc("/api/sessions/kick", authMW(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := r.URL.Query().Get("username")
		remote := r.URL.Query().Get("remote_addr")
		reason := r.URL.Query().Get("reason")
		if username == "" || remote == "" {
			http.Error(w, "username and remote_addr required", http.StatusBadRequest)
			return
		}
		if reason == "" {
			reason = "Session terminated by administrator"
		}
		ok := GlobalConnTracker.KickSession(username, remote, reason)
		if !ok {
			writeStatus(w, http.StatusNotFound, map[string]string{"status": "not_found"})
			return
		}
		writeJSON(w, map[string]string{"status": "kicked", "remote_addr": remote})
	}))

	// ── users ────────────────────────────────────────────────────────────────

	mux.HandleFunc("/api/users", authMW(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			http.Error(w, "user store unavailable", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			users, err := store.List()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Обогащаем активной статистикой
			stats := GlobalConnTracker.UserStats()
			statsByName := make(map[string]UserStats, len(stats))
			for _, s := range stats {
				statsByName[s.Username] = s
			}
			active := GlobalConnTracker.ActiveUsers()

			type enriched struct {
				AdminUser
				ActiveConns   int    `json:"active_conns"`
				BytesIn       uint64 `json:"bytes_in"`
				BytesOut      uint64 `json:"bytes_out"`
				TotalTunnels  uint64 `json:"total_tunnels"`
			}
			out := make([]enriched, 0, len(users))
			for _, u := range users {
				e := enriched{AdminUser: u, ActiveConns: active[u.Username]}
				if s, ok := statsByName[u.Username]; ok {
					e.BytesIn = s.BytesIn
					e.BytesOut = s.BytesOut
					e.TotalTunnels = s.TotalTunnels
				}
				out = append(out, e)
			}
			writeJSON(w, out)

		case http.MethodPost:
			var req struct {
				Username     string `json:"username"`
				Password     string `json:"password"`
				MaxDevices   int    `json:"max_devices"`
				ExpiresAt    int64  `json:"expires_at"`
				TrafficLimit uint64 `json:"traffic_limit"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Username == "" || req.Password == "" {
				http.Error(w, "username and password required", http.StatusBadRequest)
				return
			}
			if err := store.Add(req.Username, req.Password); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			if req.MaxDevices > 0 {
				_ = store.SetMaxDevices(req.Username, req.MaxDevices)
			}
			if lc, ok := store.(AdminLifecycle); ok {
				if req.ExpiresAt > 0 {
					_ = lc.SetExpiresAt(req.Username, req.ExpiresAt)
				}
				if req.TrafficLimit > 0 {
					_ = lc.SetTrafficLimit(req.Username, req.TrafficLimit)
				}
			}
			writeJSON(w, map[string]string{"status": "created"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/users/", authMW(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			http.Error(w, "user store unavailable", http.StatusServiceUnavailable)
			return
		}
		// /api/users/{username}[/...]
		path := strings.TrimPrefix(r.URL.Path, "/api/users/")
		parts := strings.Split(path, "/")
		username := parts[0]
		if username == "" {
			http.Error(w, "username required", http.StatusBadRequest)
			return
		}

		// /api/users/{u}/kick
		if len(parts) == 2 && parts[1] == "kick" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			reason := r.URL.Query().Get("reason")
			if reason == "" {
				reason = "Account suspended by administrator"
			}
			GlobalConnTracker.KickUser(username, reason)
			authn.Invalidate(username)
			writeJSON(w, map[string]string{"status": "kicked"})
			return
		}

		switch r.Method {
		case http.MethodDelete:
			if err := store.Delete(username); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			GlobalConnTracker.KickUser(username, "Account deleted")
			authn.Invalidate(username)
			writeJSON(w, map[string]string{"status": "deleted"})

		case http.MethodPatch:
			var req struct {
				Password     *string `json:"password,omitempty"`
				MaxDevices   *int    `json:"max_devices,omitempty"`
				Enabled      *bool   `json:"enabled,omitempty"`
				ExpiresAt    *int64  `json:"expires_at,omitempty"`
				TrafficLimit *uint64 `json:"traffic_limit,omitempty"`
				ResetTraffic *bool   `json:"reset_traffic,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			lc, _ := store.(AdminLifecycle)

			if req.Password != nil {
				if err := store.ChangePassword(username, *req.Password); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
			}
			if req.MaxDevices != nil {
				if err := store.SetMaxDevices(username, *req.MaxDevices); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
			}
			if req.Enabled != nil && lc != nil {
				if err := lc.SetEnabled(username, *req.Enabled); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !*req.Enabled {
					GlobalConnTracker.KickUser(username, "Account disabled")
				}
			}
			if req.ExpiresAt != nil && lc != nil {
				if err := lc.SetExpiresAt(username, *req.ExpiresAt); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			if req.TrafficLimit != nil && lc != nil {
				if err := lc.SetTrafficLimit(username, *req.TrafficLimit); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			if req.ResetTraffic != nil && *req.ResetTraffic && lc != nil {
				if err := lc.ResetTraffic(username); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			authn.Invalidate(username)
			writeJSON(w, map[string]string{"status": "updated"})

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// ── obsolete v1 endpoints kept for ttadmin/CLI compatibility ─────────────

	mux.HandleFunc("/users/kick", authMW(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username := r.URL.Query().Get("username")
		reason := r.URL.Query().Get("reason")
		if username == "" {
			http.Error(w, "username required", http.StatusBadRequest)
			return
		}
		if reason == "" {
			reason = "Account suspended by administrator"
		}
		GlobalConnTracker.KickUser(username, reason)
		authn.Invalidate(username)
		writeJSON(w, map[string]string{"status": "kicked", "username": username})
	}))
	mux.HandleFunc("/users/active", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, GlobalConnTracker.ActiveUsers())
	}))
	mux.HandleFunc("/sessions", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, GlobalConnTracker.Sessions())
	}))
	mux.HandleFunc("/sessions/kick", authMW(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/sessions/kick?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
	}))
	mux.HandleFunc("/stats", authMW(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, GlobalConnTracker.UserStats())
	}))

	// ── static webui ─────────────────────────────────────────────────────────

	if webRoot != "" {
		if _, err := os.Stat(webRoot); err == nil {
			fileServer := http.FileServer(spaFS(http.Dir(webRoot)))
			mux.Handle("/", fileServer)
		}
	}

	go http.ListenAndServe(cfg.Address, mux)
	_ = context.Background
	_ = strconv.Atoi
}

// ── helpers ───────────────────────────────────────────────────────────────────

func setCORS(w http.ResponseWriter, cfg *config.AdminConfig) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	_ = cfg
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// spaFS — оборачивает http.FileSystem чтобы 404 (отсутствующий файл) отдавал index.html.
// Нужно для SPA-роутинга.
type spaFileSystem struct{ root http.FileSystem }

func spaFS(root http.FileSystem) http.FileSystem { return &spaFileSystem{root} }

func (s *spaFileSystem) Open(name string) (http.File, error) {
	f, err := s.root.Open(name)
	if err == nil {
		return f, nil
	}
	if os.IsNotExist(err) {
		return s.root.Open("/index.html")
	}
	return nil, err
}

var _ fs.FS // keep io/fs import for future use
