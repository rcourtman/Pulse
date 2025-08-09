package api

import (
	"net/http"
	
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// RequireAuth middleware checks for API token authentication
func RequireAuth(cfg *config.Config, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no API token is configured, allow access
		if cfg.APIToken == "" {
			handler(w, r)
			return
		}
		
		// Check for API token in header
		apiToken := r.Header.Get("X-API-Token")
		if apiToken == "" || apiToken != cfg.APIToken {
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Msg("Unauthorized API access attempt")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		handler(w, r)
	}
}