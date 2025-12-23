package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// DemoModeMiddleware blocks all modification requests in demo mode
func DemoModeMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cfg.DemoMode {
			next.ServeHTTP(w, r)
			return
		}

		// Add header so frontend knows we're in demo mode
		w.Header().Set("X-Demo-Mode", "true")

		// Allow GET and HEAD requests (read-only)
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Allow WebSocket upgrades
		if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow authentication endpoints (login is read-only - verifies credentials)
		authPaths := []string{
			"/api/login",
			"/api/oidc/login",
			"/api/oidc/callback",
			"/api/logout",
			// Allow AI chat interaction (mocked in backend if key missing)
			"/api/ai/execute",
		}
		for _, path := range authPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Block all modification requests (POST, PUT, DELETE, PATCH)
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote", r.RemoteAddr).
			Msg("Demo mode: blocked modification request")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Demo mode enabled",
			"message": "This is a read-only demo instance. Modifications are disabled.",
		})
	})
}
