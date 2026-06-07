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
		if username == "" {
			http.Error(w, "username required", http.StatusBadRequest)
			return
		}
		authn.InvalidateWithNotify(username) // инвалидирует кэш + кикает через OnInvalidate
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "kicked", "username": username})
	}))

	// GET /users/active
	mux.HandleFunc("/users/active", auth(func(w http.ResponseWriter, r *http.Request) {
		active := GlobalConnTracker.ActiveUsers()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(active)
	}))

	go http.ListenAndServe(cfg.Address, mux)
}
