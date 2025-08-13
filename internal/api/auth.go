package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
	
	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// Simple session store - in production you'd use Redis or similar
var (
	sessions = make(map[string]time.Time)
	sessionMu sync.RWMutex
)

// generateSessionToken creates a simple random session token
func generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// validateSession checks if a session token is valid
func validateSession(token string) bool {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	
	expiry, exists := sessions[token]
	if !exists {
		return false
	}
	
	// Check if expired
	if time.Now().After(expiry) {
		// Clean up expired session
		sessionMu.RUnlock()
		sessionMu.Lock()
		delete(sessions, token)
		sessionMu.Unlock()
		sessionMu.RLock()
		return false
	}
	
	return true
}

// CheckAuth checks both basic auth and API token
func CheckAuth(cfg *config.Config, w http.ResponseWriter, r *http.Request) bool {
	// If no auth is configured at all, allow access
	if cfg.AuthUser == "" && cfg.AuthPass == "" && cfg.APIToken == "" {
		log.Debug().Msg("No auth configured, allowing access")
		return true
	}
	
	log.Debug().
		Str("configured_user", cfg.AuthUser).
		Bool("has_pass", cfg.AuthPass != "").
		Bool("has_token", cfg.APIToken != "").
		Str("url", r.URL.Path).
		Msg("Checking authentication")
	
	// Check API token first (for backward compatibility)
	if cfg.APIToken != "" {
		// Check header
		if token := r.Header.Get("X-API-Token"); token == cfg.APIToken {
			return true
		}
		// Check query parameter (for export/import)
		if token := r.URL.Query().Get("token"); token == cfg.APIToken {
			return true
		}
	}
	
	// Check session cookie (for WebSocket and UI)
	if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if validateSession(cookie.Value) {
			return true
		}
	}
	
	// Check basic auth
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		auth := r.Header.Get("Authorization")
		log.Debug().Str("auth_header", auth).Str("url", r.URL.Path).Msg("Checking auth")
		if auth != "" {
			const prefix = "Basic "
			if strings.HasPrefix(auth, prefix) {
				decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
				if err == nil {
					parts := strings.SplitN(string(decoded), ":", 2)
					if len(parts) == 2 {
						// Check username
						userMatch := parts[0] == cfg.AuthUser
						
						// Check password - support both hashed and plain text for migration
						var passMatch bool
						if strings.HasPrefix(cfg.AuthPass, "$2") && len(cfg.AuthPass) == 60 {
							// Config has hashed password, check against hash
							passMatch = internalauth.CheckPasswordHash(parts[1], cfg.AuthPass)
						} else {
							// Config has plain text password (legacy), do direct comparison
							passMatch = parts[1] == cfg.AuthPass
							if passMatch {
								log.Warn().Msg("Using plain text password comparison - please update to hashed password")
							}
						}
						
						log.Debug().
							Str("provided_user", parts[0]).
							Str("expected_user", cfg.AuthUser).
							Bool("user_match", userMatch).
							Bool("pass_match", passMatch).
							Bool("password_is_hashed", strings.HasPrefix(cfg.AuthPass, "$2") && len(cfg.AuthPass) == 60).
							Msg("Auth check")
						
						if userMatch && passMatch {
							// Valid credentials - create session
							if w != nil {
								token := generateSessionToken()
								
								// Store session
								sessionMu.Lock()
								sessions[token] = time.Now().Add(24 * time.Hour)
								sessionMu.Unlock()
								
								// Set cookie
								http.SetCookie(w, &http.Cookie{
									Name:     "pulse_session",
									Value:    token,
									Path:     "/",
									HttpOnly: true,
									Secure:   r.TLS != nil,
									SameSite: http.SameSiteLaxMode, // Lax for cross-origin navigation
									MaxAge:   86400, // 24 hours
								})
							}
							return true
						}
					}
				}
			}
		}
	}
	
	return false
}

// RequireAuth middleware checks for authentication
func RequireAuth(cfg *config.Config, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CheckAuth(cfg, w, r) {
			handler(w, r)
			return
		}
		
		// Log the failed attempt
		log.Warn().
			Str("ip", r.RemoteAddr).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Msg("Unauthorized access attempt")
		
		// Only send WWW-Authenticate header for non-API/non-AJAX requests
		// This prevents the browser popup for API calls from the frontend
		isAPIRequest := strings.HasPrefix(r.URL.Path, "/api/") ||
			r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
			strings.Contains(r.Header.Get("Accept"), "application/json")
		
		if cfg.AuthUser != "" && cfg.AuthPass != "" && !isAPIRequest {
			w.Header().Set("WWW-Authenticate", `Basic realm="Pulse"`)
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}