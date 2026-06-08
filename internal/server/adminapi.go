package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

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

// AdminUser — представление пользователя для admin API.
type AdminUser struct {
	Username   string `json:"username"`
	MaxDevices int    `json:"max_devices"`
}

// StartAdminAPI запускает HTTP admin API на localhost.
// Если webRoot не пустой — раздаёт статику фронта.
func StartAdminAPI(cfg *config.AdminConfig, authn *auth.Authenticator, store AdminUserStore, webRoot string) {
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
				ActiveConns  int    `json:"active_conns"`
				BytesIn      uint64 `json:"bytes_in"`
				BytesOut     uint64 `json:"bytes_out"`
				TotalTunnels uint64 `json:"total_tunnels"`
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
				Username   string `json:"username"`
				Password   string `json:"password"`
				MaxDevices int    `json:"max_devices"`
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
				Password   *string `json:"password,omitempty"`
				MaxDevices *int    `json:"max_devices,omitempty"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Password != nil {
				if err := store.ChangePassword(username, *req.Password); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				authn.Invalidate(username)
			}
			if req.MaxDevices != nil {
				if err := store.SetMaxDevices(username, *req.MaxDevices); err != nil {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				authn.Invalidate(username)
			}
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
