package api

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	contextKeyAPIToken contextKey = "apiTokenRecord"
)

// Global session store instance
var (
	sessionStore     *SessionStore
	sessionOnce      sync.Once
	adminBypassState struct {
		once     sync.Once
		enabled  bool
		declined bool
	}
)

// InitSessionStore initializes the persistent session store
func InitSessionStore(dataPath string) {
	sessionOnce.Do(func() {
		sessionStore = NewSessionStore(dataPath)
	})
}

// GetSessionStore returns the global session store instance
func GetSessionStore() *SessionStore {
	if sessionStore == nil {
		// Initialize with default path if not already initialized
		InitSessionStore("/etc/pulse")
	}
	return sessionStore
}

// detectProxy checks if the request is coming through a reverse proxy
func detectProxy(r *http.Request) bool {
	// Check multiple headers that proxies commonly set
	return r.Header.Get("X-Forwarded-For") != "" ||
		r.Header.Get("X-Real-IP") != "" ||
		r.Header.Get("X-Forwarded-Proto") != "" ||
		r.Header.Get("X-Forwarded-Host") != "" ||
		r.Header.Get("Forwarded") != "" || // RFC 7239
		r.Header.Get("CF-Ray") != "" || // Cloudflare
		r.Header.Get("CF-Connecting-IP") != "" || // Cloudflare
		r.Header.Get("X-Forwarded-Server") != "" || // Some proxies
		r.Header.Get("X-Forwarded-Port") != "" // Some proxies
}

// isConnectionSecure checks if the connection is over HTTPS
func isConnectionSecure(r *http.Request) bool {
	return r.TLS != nil ||
		r.Header.Get("X-Forwarded-Proto") == "https" ||
		strings.Contains(r.Header.Get("Forwarded"), "proto=https")
}

// getCookieSettings returns the appropriate cookie settings based on proxy detection
func getCookieSettings(r *http.Request) (secure bool, sameSite http.SameSite) {
	isProxied := detectProxy(r)
	isSecure := isConnectionSecure(r)

	// Debug logging for Cloudflare tunnel issues
	if isProxied {
		log.Debug().
			Bool("proxied", isProxied).
			Bool("secure", isSecure).
			Str("cf_ray", r.Header.Get("CF-Ray")).
			Str("cf_connecting_ip", r.Header.Get("CF-Connecting-IP")).
			Str("x_forwarded_for", r.Header.Get("X-Forwarded-For")).
			Str("x_forwarded_proto", r.Header.Get("X-Forwarded-Proto")).
			Msg("Proxy/tunnel detected - adjusting cookie settings")
	}

	// Default to Lax for better compatibility
	sameSitePolicy := http.SameSiteLaxMode

	if isProxied {
		// For proxied connections, we need to be more permissive
		// But only use None if connection is secure (required by browsers)
		if isSecure {
			sameSitePolicy = http.SameSiteNoneMode
		} else {
			// For HTTP proxies, stay with Lax for compatibility
			sameSitePolicy = http.SameSiteLaxMode
		}
	}

	return isSecure, sameSitePolicy
}

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
	return GetSessionStore().ValidateSession(token)
}

// ValidateAndExtendSession validates a session and extends its expiration (sliding window)
func ValidateAndExtendSession(token string) bool {
	return GetSessionStore().ValidateAndExtendSession(token)
}

// CheckProxyAuth validates proxy authentication headers
func CheckProxyAuth(cfg *config.Config, r *http.Request) (bool, string, bool) {
	// Check if proxy auth is configured
	if cfg.ProxyAuthSecret == "" {
		return false, "", false
	}

	// Validate proxy secret header
	proxySecret := r.Header.Get("X-Proxy-Secret")
	if proxySecret != cfg.ProxyAuthSecret {
		log.Debug().
			Int("provided_secret_length", len(proxySecret)).
			Msg("Invalid proxy secret")
		return false, "", false
	}

	// Get username from header if configured
	username := ""
	if cfg.ProxyAuthUserHeader != "" {
		username = r.Header.Get(cfg.ProxyAuthUserHeader)
		if username == "" {
			log.Debug().Str("header", cfg.ProxyAuthUserHeader).Msg("Proxy auth user header not found")
			return false, "", false
		}
	}

	// Check admin role if configured
	isAdmin := true // Default to admin if no role checking configured
	if cfg.ProxyAuthRoleHeader != "" && cfg.ProxyAuthAdminRole != "" {
		roles := r.Header.Get(cfg.ProxyAuthRoleHeader)
		if roles != "" {
			// Split roles by separator
			separator := cfg.ProxyAuthRoleSeparator
			if separator == "" {
				separator = "|"
			}
			roleList := strings.Split(roles, separator)
			isAdmin = false
			for _, role := range roleList {
				if strings.TrimSpace(role) == cfg.ProxyAuthAdminRole {
					isAdmin = true
					break
				}
			}
			log.Debug().
				Str("roles", roles).
				Bool("is_admin", isAdmin).
				Msg("Proxy auth roles checked")
		}
	}

	log.Debug().
		Str("user", username).
		Bool("is_admin", isAdmin).
		Msg("Proxy authentication successful")

	return true, username, isAdmin
}

// CheckAuth checks both basic auth and API token
func CheckAuth(cfg *config.Config, w http.ResponseWriter, r *http.Request) bool {
	config.Mu.RLock()
	defer config.Mu.RUnlock()

	// Check proxy auth first if configured
	if cfg.ProxyAuthSecret != "" {
		if valid, username, _ := CheckProxyAuth(cfg, r); valid {
			// Set username in response header for frontend
			if username != "" {
				w.Header().Set("X-Authenticated-User", username)
			}
			w.Header().Set("X-Auth-Method", "proxy")
			return true
		}
	}

	// Check for OIDC session cookie
	if cfg.OIDC != nil && cfg.OIDC.Enabled {
		if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
			if ValidateSession(cookie.Value) {
				// Check if this is an OIDC session
				if username := GetSessionUsername(cookie.Value); username != "" {
					w.Header().Set("X-Authenticated-User", username)
					w.Header().Set("X-Auth-Method", "oidc")
					return true
				}
			}
		}
	}

	// If no auth is configured at all, allow access unless OIDC is enabled
	if cfg.AuthUser == "" && cfg.AuthPass == "" && !cfg.HasAPITokens() && cfg.ProxyAuthSecret == "" {
		if cfg.OIDC != nil && cfg.OIDC.Enabled {
			log.Debug().Msg("OIDC enabled without local credentials, authentication required")
		} else {
			log.Debug().Msg("No auth configured, allowing access")
			return true
		}
	}

	// API-only mode: when only API token is configured (no password auth)
	if cfg.AuthUser == "" && cfg.AuthPass == "" && cfg.HasAPITokens() {
		// Check if an API token was provided
		providedToken := r.Header.Get("X-API-Token")

		// If a token was provided, validate it
		if providedToken != "" {
			if record, ok := cfg.ValidateAPIToken(providedToken); ok {
				attachAPITokenRecord(r, record)
				return true
			}
			// Invalid token provided
			if w != nil {
				http.Error(w, "Invalid API token", http.StatusUnauthorized)
			}
			return false
		}

		// Require a valid token for all requests in API-only mode
		if w != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="API token required; supply via Authorization header or X-API-Token header"`)
			http.Error(w, "API token required via Authorization header or X-API-Token header", http.StatusUnauthorized)
		}
		return false
	}

	log.Debug().
		Str("configured_user", cfg.AuthUser).
		Bool("has_pass", cfg.AuthPass != "").
		Bool("has_token", cfg.HasAPITokens()).
		Str("url", r.URL.Path).
		Msg("Checking authentication")

	validateToken := func(token string) bool {
		if token == "" {
			return false
		}
		if record, ok := cfg.ValidateAPIToken(token); ok {
			attachAPITokenRecord(r, record)
			return true
		}
		return false
	}

	// Check API tokens (header, bearer, query) before other auth methods
	if cfg.HasAPITokens() {
		if validateToken(r.Header.Get("X-API-Token")) {
			return true
		}
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				if validateToken(strings.TrimSpace(authHeader[7:])) {
					return true
				}
			}
		}
		// Check query parameter (for WebSocket connections that can't send headers)
		if queryToken := r.URL.Query().Get("token"); queryToken != "" {
			if validateToken(queryToken) {
				return true
			}
		}
	}

	// Check session cookie (for WebSocket and UI)
	if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		// Use ValidateAndExtendSession for sliding expiration
		if ValidateAndExtendSession(cookie.Value) {
			return true
		}
		// Debug logging for failed session validation
		log.Debug().
			Str("session_token", safePrefixForLog(cookie.Value, 8)+"...").
			Str("path", r.URL.Path).
			Msg("Session validation failed - token not found or expired")
	} else if err != nil {
		// Debug logging when no session cookie found
		log.Debug().
			Err(err).
			Str("path", r.URL.Path).
			Bool("has_cf_headers", r.Header.Get("CF-Ray") != "").
			Msg("No session cookie found")
	}

	// Check basic auth
	if cfg.AuthUser != "" && cfg.AuthPass != "" {
		auth := r.Header.Get("Authorization")
		authScheme := "none"
		if auth != "" {
			if idx := strings.IndexByte(auth, ' '); idx != -1 {
				authScheme = strings.ToLower(auth[:idx])
			} else {
				authScheme = strings.ToLower(auth)
			}
		}
		log.Debug().Str("auth_scheme", authScheme).Str("url", r.URL.Path).Msg("Checking Authorization header")
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
						_, userLockedUntil, userLocked := GetLockoutInfo(parts[0])
						_, ipLockedUntil, ipLocked := GetLockoutInfo(clientIP)

						if userLocked || ipLocked {
							lockedUntil := userLockedUntil
							if ipLocked && ipLockedUntil.After(lockedUntil) {
								lockedUntil = ipLockedUntil
							}

							remainingMinutes := int(time.Until(lockedUntil).Minutes())
							if remainingMinutes < 1 {
								remainingMinutes = 1
							}

							log.Warn().Str("user", parts[0]).Str("ip", clientIP).Msg("Account locked out")
							LogAuditEvent("login", parts[0], clientIP, r.URL.Path, false, "Account locked")
							if w != nil {
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(http.StatusForbidden)
								w.Write([]byte(fmt.Sprintf(`{"error":"Account temporarily locked","message":"Too many failed attempts. Please try again in %d minutes.","lockedUntil":"%s"}`,
									remainingMinutes, lockedUntil.Format(time.RFC3339))))
							}
							return false
						}
						// Check username
						userMatch := parts[0] == cfg.AuthUser

						// Check password - support both hashed and plain text for migration
						// Config always has hashed password now (auto-hashed on load)
						passMatch := internalauth.CheckPasswordHash(parts[1], cfg.AuthPass)

						log.Debug().
							Str("provided_user", parts[0]).
							Str("expected_user", cfg.AuthUser).
							Bool("user_match", userMatch).
							Bool("pass_match", passMatch).
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

								// Store session persistently
								userAgent := r.Header.Get("User-Agent")
								clientIP := GetClientIP(r)
								GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP)

								// Track session for user
								TrackUserSession(parts[0], token)

								// Generate CSRF token
								csrfToken := generateCSRFToken(token)

								// Get appropriate cookie settings based on proxy detection
								isSecure, sameSitePolicy := getCookieSettings(r)

								// Debug logging for Cloudflare tunnel issues
								sameSiteName := "Default"
								switch sameSitePolicy {
								case http.SameSiteNoneMode:
									sameSiteName = "None"
								case http.SameSiteLaxMode:
									sameSiteName = "Lax"
								case http.SameSiteStrictMode:
									sameSiteName = "Strict"
								}

								log.Debug().
									Bool("secure", isSecure).
									Str("same_site", sameSiteName).
									Str("token", safePrefixForLog(token, 8)+"...").
									Str("remote_addr", r.RemoteAddr).
									Msg("Setting session cookie after successful login")

								// Set session cookie
								http.SetCookie(w, &http.Cookie{
									Name:     "pulse_session",
									Value:    token,
									Path:     "/",
									HttpOnly: true,
									Secure:   isSecure,
									SameSite: sameSitePolicy,
									MaxAge:   86400, // 24 hours
								})

								// Set CSRF cookie (not HttpOnly so JS can read it)
								http.SetCookie(w, &http.Cookie{
									Name:     "pulse_csrf",
									Value:    csrfToken,
									Path:     "/",
									Secure:   isSecure,
									SameSite: sameSitePolicy,
									MaxAge:   86400, // 24 hours
								})

								// Audit log successful login
								LogAuditEvent("login", parts[0], GetClientIP(r), r.URL.Path, true, "Basic auth login")
							}
							return true
						} else {
							// Failed login
							RecordFailedLogin(parts[0])
							RecordFailedLogin(clientIP)
							LogAuditEvent("login", parts[0], clientIP, r.URL.Path, false, "Invalid credentials")

							// Get updated attempt counts
							newUserAttempts, _, _ := GetLockoutInfo(parts[0])
							newIPAttempts, _, _ := GetLockoutInfo(clientIP)

							// Use the higher count for warning
							attempts := newUserAttempts
							if newIPAttempts > attempts {
								attempts = newIPAttempts
							}

							if r.URL.Path == "/api/login" && w != nil {
								// For login endpoint, provide detailed error response
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(http.StatusUnauthorized)
								remaining := maxFailedAttempts - attempts
								if remaining > 0 {
									w.Write([]byte(fmt.Sprintf(`{"error":"Invalid credentials","attempts":%d,"remaining":%d,"maxAttempts":%d}`,
										attempts, remaining, maxFailedAttempts)))
								} else {
									w.Write([]byte(`{"error":"Invalid credentials","locked":true,"message":"Account locked for 15 minutes"}`))
								}
								return false
							}
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
		// Dev mode bypass for all auth (disabled by default)
		if adminBypassEnabled() {
			log.Debug().
				Str("path", r.URL.Path).
				Msg("Auth bypass enabled for dev mode")
			handler(w, r)
			return
		}

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

// RequireAdmin middleware checks for authentication and admin privileges
// For proxy auth users, it ensures they have the admin role
// For other auth methods, all authenticated users are considered admins
func RequireAdmin(cfg *config.Config, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Dev mode bypass for admin endpoints (disabled by default)
		if adminBypassEnabled() {
			log.Debug().
				Str("path", r.URL.Path).
				Msg("Admin bypass enabled for dev mode")
			handler(w, r)
			return
		}

		// First check if user is authenticated
		if !CheckAuth(cfg, w, r) {
			// Log the failed attempt
			log.Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("method", r.Method).
				Msg("Unauthorized access attempt")

			// Return authentication error
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"Authentication required"}`))
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Check if using proxy auth and if so, verify admin status
		if cfg.ProxyAuthSecret != "" {
			if valid, username, isAdmin := CheckProxyAuth(cfg, r); valid {
				if !isAdmin {
					// User is authenticated but not an admin
					log.Warn().
						Str("ip", r.RemoteAddr).
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Str("username", username).
						Msg("Non-admin user attempted to access admin endpoint")

					// Return forbidden error
					if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						w.Write([]byte(`{"error":"Admin privileges required"}`))
					} else {
						http.Error(w, "Admin privileges required", http.StatusForbidden)
					}
					return
				}
			}
		}

		// User is authenticated and has admin privileges (or not using proxy auth)
		handler(w, r)
	}
}

// RequireScope ensures that token-authenticated requests include the specified scope.
// Session-based (browser) requests bypass the scope check.
func RequireScope(scope string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if scope == "" {
			handler(w, r)
			return
		}

		record := getAPITokenRecordFromRequest(r)
		if record == nil {
			// Session-authenticated request
			handler(w, r)
			return
		}

		if record.HasScope(scope) {
			handler(w, r)
			return
		}

		log.Warn().
			Str("token_id", record.ID).
			Str("required_scope", scope).
			Msg("API token missing required scope")

		respondMissingScope(w, scope)
	}
}

func respondMissingScope(w http.ResponseWriter, scope string) {
	if w == nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":         "missing_scope",
		"requiredScope": scope,
	})
}

// ensureScope enforces that the request either originates from a session or a token
// possessing the specified scope. Returns true when access should continue.
func ensureScope(w http.ResponseWriter, r *http.Request, scope string) bool {
	if scope == "" {
		return true
	}
	record := getAPITokenRecordFromRequest(r)
	if record == nil || record.HasScope(scope) {
		return true
	}
	respondMissingScope(w, scope)
	return false
}

func attachAPITokenRecord(r *http.Request, record *config.APITokenRecord) {
	if record == nil {
		return
	}
	clone := record.Clone()
	ctx := context.WithValue(r.Context(), contextKeyAPIToken, clone)
	*r = *r.WithContext(ctx)
}

func getAPITokenRecordFromRequest(r *http.Request) *config.APITokenRecord {
	value := r.Context().Value(contextKeyAPIToken)
	if value == nil {
		return nil
	}
	record, ok := value.(config.APITokenRecord)
	if !ok {
		return nil
	}
	clone := record.Clone()
	return &clone
}
func adminBypassEnabled() bool {
	adminBypassState.once.Do(func() {
		if os.Getenv("ALLOW_ADMIN_BYPASS") != "1" {
			return
		}

		if os.Getenv("PULSE_DEV") == "true" || strings.EqualFold(os.Getenv("NODE_ENV"), "development") {
			log.Warn().Msg("Admin authentication bypass ENABLED (development mode)")
			adminBypassState.enabled = true
			return
		}

		log.Warn().Msg("Ignoring ALLOW_ADMIN_BYPASS outside development mode")
		adminBypassState.declined = true
	})
	return adminBypassState.enabled
}
