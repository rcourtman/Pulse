package api

import (
	cryptorand "crypto/rand"
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

// generateSessionToken creates a cryptographically secure session token
func generateSessionToken() string {
	b := make([]byte, 32)
	if _, err := cryptorand.Read(b); err != nil {
		log.Error().Err(err).Msg("Failed to generate secure session token")
		// Fallback - should never happen
		return ""
	}
	return hex.EncodeToString(b)
}

// ValidateSession checks if a session token is valid
func ValidateSession(token string) bool {
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
	
	// API-only mode: when only API token is configured (no password auth)
	// Allow read-only endpoints for the UI to work
	if cfg.AuthUser == "" && cfg.AuthPass == "" && cfg.APIToken != "" {
		// Check if an API token was provided
		providedToken := r.Header.Get("X-API-Token")
		if providedToken == "" {
			providedToken = r.URL.Query().Get("token")
		}
		
		// If a token was provided, validate it
		if providedToken != "" {
			if providedToken == cfg.APIToken {
				return true
			}
			// Invalid token provided
			if w != nil {
				http.Error(w, "Invalid API token", http.StatusUnauthorized)
			}
			return false
		}
		
		// No token provided - allow read-only endpoints for UI
		if r.Method == "GET" || r.URL.Path == "/ws" {
			// Allow these endpoints without auth for UI to function
			allowedPaths := []string{
				"/api/state",
				"/api/config/nodes",
				"/api/config/system",
				"/api/settings",
				"/api/discover",
				"/api/security/status",
				"/api/version",
				"/api/health",
				"/api/updates/check",
				"/api/system/diagnostics",
				"/api/guests/metadata",
				"/ws", // WebSocket for real-time updates
			}
			for _, path := range allowedPaths {
				if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
					log.Debug().Str("path", r.URL.Path).Msg("Allowing read-only access in API-only mode")
					return true
				}
			}
		}
		
		// Require token for everything else
		if w != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="API Token Required"`)
			http.Error(w, "API token required", http.StatusUnauthorized)
		}
		return false
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
		if token := r.Header.Get("X-API-Token"); token != "" {
			// Check if stored token is hashed or plain text
			isHashed := internalauth.IsAPITokenHashed(cfg.APIToken)
			log.Debug().
				Bool("is_hashed", isHashed).
				Msg("Comparing API tokens")
			
			if isHashed {
				// Compare against hash
				if internalauth.CompareAPIToken(token, cfg.APIToken) {
					return true
				}
			} else {
				// Legacy plain text comparison (should migrate)
				if token == cfg.APIToken {
					log.Warn().Msg("Using plain text API token - please regenerate for security")
					return true
				}
			}
		}
		// Check query parameter (for export/import)
		if token := r.URL.Query().Get("token"); token != "" {
			if internalauth.IsAPITokenHashed(cfg.APIToken) {
				if internalauth.CompareAPIToken(token, cfg.APIToken) {
					return true
				}
			} else {
				if token == cfg.APIToken {
					log.Warn().Msg("Using plain text API token - please regenerate for security")
					return true
				}
			}
		}
	}
	
	// Check session cookie (for WebSocket and UI)
	if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
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
						clientIP := GetClientIP(r)
						
						// Only apply rate limiting for actual login attempts, not regular auth checks
						// Login attempts come to /api/login endpoint
						if r.URL.Path == "/api/login" {
							// Check rate limiting for auth attempts
							if !authLimiter.Allow(clientIP) {
								log.Warn().Str("ip", clientIP).Msg("Rate limit exceeded for auth")
								LogAuditEvent("login", parts[0], clientIP, r.URL.Path, false, "Rate limited")
								if w != nil {
									http.Error(w, "Too many authentication attempts", http.StatusTooManyRequests)
								}
								return false
							}
						}
						
						// Check if account is locked out
						if IsLockedOut(parts[0]) || IsLockedOut(clientIP) {
							log.Warn().Str("user", parts[0]).Str("ip", clientIP).Msg("Account locked out")
							LogAuditEvent("login", parts[0], clientIP, r.URL.Path, false, "Account locked")
							if w != nil {
								http.Error(w, "Account temporarily locked due to failed attempts", http.StatusForbidden)
							}
							return false
						}
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
							// Clear failed login attempts
							ClearFailedLogins(parts[0])
							ClearFailedLogins(GetClientIP(r))
							
							// Valid credentials - create session
							if w != nil {
								token := generateSessionToken()
								if token == "" {
									return false
								}
								
								// Store session
								sessionMu.Lock()
								sessions[token] = time.Now().Add(24 * time.Hour)
								sessionMu.Unlock()
								
								// Track session for user
								TrackUserSession(parts[0], token)
								
								// Generate CSRF token
								csrfToken := generateCSRFToken(token)
								
								// Set session cookie
								http.SetCookie(w, &http.Cookie{
									Name:     "pulse_session",
									Value:    token,
									Path:     "/",
									HttpOnly: true,
									Secure:   r.TLS != nil,
									SameSite: http.SameSiteLaxMode,
									MaxAge:   86400, // 24 hours
								})
								
								// Set CSRF cookie (not HttpOnly so JS can read it)
								http.SetCookie(w, &http.Cookie{
									Name:     "pulse_csrf",
									Value:    csrfToken,
									Path:     "/",
									Secure:   r.TLS != nil,
									SameSite: http.SameSiteStrictMode,
									MaxAge:   86400, // 24 hours
								})
								
								// Audit log successful login
								LogAuditEvent("login", parts[0], GetClientIP(r), r.URL.Path, true, "Basic auth login")
							}
							return true
						} else {
							// Failed login
							RecordFailedLogin(parts[0])
							RecordFailedLogin(GetClientIP(r))
							LogAuditEvent("login", parts[0], GetClientIP(r), r.URL.Path, false, "Invalid credentials")
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
		
		// Never send WWW-Authenticate header - we want to use our custom login page
		// The frontend will detect 401 responses and show the login component
		// Return JSON error for API requests, plain text for others
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Authentication required"}`))
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
	}
}