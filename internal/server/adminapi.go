package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pnsrc/ttgo/internal/auth"
	"github.com/pnsrc/ttgo/internal/config"
)

// StartAdminAPI запускает HTTP admin API на localhost.
// Endpoints:
//
//	POST /users/kick?username=X   — GOAWAY + инвалидация кэша
//	GET  /users/active            — список активных соединений
//	POST /tls/reload              — SIGHUP (горячая перезагрузка TLS)
func StartAdminAPI(cfg *config.AdminConfig, authn *auth.Authenticator) {
	if cfg == nil || cfg.Token == "" || cfg.Address == "" {
		return
	}

	mux := http.NewServeMux()

	auth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token != cfg.Token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}

	// POST /users/kick?username=X
	mux.HandleFunc("/users/kick", auth(func(w http.ResponseWriter, r *http.Request) {
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
		// Kick with custom reason (mark revoked → 407 on next request → GOAWAY in 2s)
		GlobalConnTracker.KickUser(username, reason)
		// Invalidate auth cache so re-auth also fails
		authn.Invalidate(username)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "kicked", "username": username})
	}))

	// GET /users/active
	mux.HandleFunc("/users/active", auth(func(w http.ResponseWriter, r *http.Request) {
		active := GlobalConnTracker.ActiveUsers()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(active)
	}))

	// GET /sessions — все активные TLS-соединения с трафиком
	mux.HandleFunc("/sessions", auth(func(w http.ResponseWriter, r *http.Request) {
		sessions := GlobalConnTracker.Sessions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}))

	// POST /sessions/kick?username=X&remote_addr=Y&reason=Z — кикнуть конкретную сессию
	mux.HandleFunc("/sessions/kick", auth(func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"status": "not_found"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"status":      "kicked",
			"username":    username,
			"remote_addr": remote,
		})
	}))

	// GET /stats — агрегированная статистика по юзерам (lifetime + active)
	mux.HandleFunc("/stats", auth(func(w http.ResponseWriter, r *http.Request) {
		stats := GlobalConnTracker.UserStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}))

	go http.ListenAndServe(cfg.Address, mux)
}
