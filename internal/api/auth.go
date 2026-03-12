package api

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
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

type ssoOIDCProviderAuthSnapshot struct {
	ProviderID    string
	IssuerURL     string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	UsernameClaim string
	EmailClaim    string
	CABundle      string
}

type ssoAuthSnapshot struct {
	HasEnabledProviders bool
	OIDCProviders       []ssoOIDCProviderAuthSnapshot
}

var authSSOState = struct {
	mu         sync.RWMutex
	byConfigID map[string]ssoAuthSnapshot
}{
	byConfigID: make(map[string]ssoAuthSnapshot),
}

func authConfigID(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if id := strings.TrimSpace(cfg.DataPath); id != "" {
		return id
	}
	return strings.TrimSpace(cfg.ConfigPath)
}

func buildSSOAuthSnapshot(ssoCfg *config.SSOConfig) ssoAuthSnapshot {
	if ssoCfg == nil {
		return ssoAuthSnapshot{}
	}

	enabledProviders := ssoCfg.GetEnabledProviders()
	snapshot := ssoAuthSnapshot{HasEnabledProviders: len(enabledProviders) > 0}
	for _, provider := range enabledProviders {
		if provider.Type != config.SSOProviderTypeOIDC || provider.OIDC == nil {
			continue
		}
		scopes := append([]string{}, provider.OIDC.Scopes...)
		if len(scopes) == 0 {
			scopes = []string{"openid", "profile", "email"}
		}
		snapshot.OIDCProviders = append(snapshot.OIDCProviders, ssoOIDCProviderAuthSnapshot{
			ProviderID:    provider.ID,
			IssuerURL:     provider.OIDC.IssuerURL,
			ClientID:      provider.OIDC.ClientID,
			ClientSecret:  provider.OIDC.ClientSecret,
			RedirectURL:   provider.OIDC.RedirectURL,
			Scopes:        scopes,
			UsernameClaim: provider.OIDC.UsernameClaim,
			EmailClaim:    provider.OIDC.EmailClaim,
			CABundle:      provider.OIDC.CABundle,
		})
	}

	return snapshot
}

func setSSOAuthSnapshot(cfg *config.Config, ssoCfg *config.SSOConfig) {
	configID := authConfigID(cfg)
	if configID == "" {
		return
	}

	authSSOState.mu.Lock()
	authSSOState.byConfigID[configID] = buildSSOAuthSnapshot(ssoCfg)
	authSSOState.mu.Unlock()
}

func getSSOAuthSnapshot(cfg *config.Config) ssoAuthSnapshot {
	configID := authConfigID(cfg)
	if configID == "" {
		return ssoAuthSnapshot{}
	}

	authSSOState.mu.RLock()
	snapshot := authSSOState.byConfigID[configID]
	authSSOState.mu.RUnlock()
	return snapshot
}

func hasEnabledSSOProvidersForAuth(cfg *config.Config) bool {
	return getSSOAuthSnapshot(cfg).HasEnabledProviders
}

func resolveOIDCRefreshConfig(cfg *config.Config, session *SessionData) (*config.OIDCConfig, string) {
	if session == nil {
		return nil, ""
	}

	issuer := strings.TrimSpace(session.OIDCIssuer)
	if issuer == "" {
		return nil, ""
	}

	sessionClientID := strings.TrimSpace(session.OIDCClientID)
	snapshot := getSSOAuthSnapshot(cfg)
	if !snapshot.HasEnabledProviders {
		return nil, ""
	}

	for _, provider := range snapshot.OIDCProviders {
		if strings.TrimSpace(provider.IssuerURL) != issuer {
			continue
		}
		if sessionClientID != "" && strings.TrimSpace(provider.ClientID) != sessionClientID {
			continue
		}
		return &config.OIDCConfig{
			Enabled:       true,
			IssuerURL:     provider.IssuerURL,
			ClientID:      provider.ClientID,
			ClientSecret:  provider.ClientSecret,
			RedirectURL:   provider.RedirectURL,
			Scopes:        append([]string{}, provider.Scopes...),
			UsernameClaim: provider.UsernameClaim,
			EmailClaim:    provider.EmailClaim,
			CABundle:      provider.CABundle,
		}, provider.ProviderID
	}

	return nil, ""
}

type authContextKey string

const (
	adminBypassContextKey authContextKey = "admin_bypass"
)

// InitSessionStore initializes the persistent session store
func InitSessionStore(dataPath string) {
	sessionOnce.Do(func() {
		sessionStore = NewSessionStore(dataPath)
	})
}

// GetSessionStore returns the global session store instance
func GetSessionStore() *SessionStore {
	// Always route through sync.Once to avoid unsynchronized reads on sessionStore.
	InitSessionStore("/etc/pulse")
	return sessionStore
}

// detectProxy checks if the request is coming through a reverse proxy.
// Only trusts proxy-set headers when the direct peer is a known trusted proxy
// to prevent attackers from injecting these headers on direct connections.
func detectProxy(r *http.Request) bool {
	peerIP := extractRemoteIP(r.RemoteAddr)
	if !isTrustedProxyIP(peerIP) {
		return false
	}
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

// isConnectionSecure checks if the connection is over HTTPS.
// Forwarded-proto headers are only trusted when the direct peer is a known
// trusted proxy, preventing attackers from injecting X-Forwarded-Proto: https
// on plain HTTP connections to influence cookie security attributes.
func isConnectionSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	peerIP := extractRemoteIP(r.RemoteAddr)
	if !isTrustedProxyIP(peerIP) {
		return false
	}
	return r.Header.Get("X-Forwarded-Proto") == "https" ||
		strings.Contains(r.Header.Get("Forwarded"), "proto=https")
}

// isWebSocketUpgrade reports whether the request is a WebSocket upgrade handshake.
// Query-string tokens are only accepted for WebSocket connections because those
// can't set custom headers during the upgrade. Accepting tokens in the URL for
// regular HTTP requests would expose them in logs, referrers, and browser history.
func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
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

// Cookie name constants. The session cookie uses the __Host- prefix when served
// over HTTPS, which instructs browsers to reject the cookie unless Secure is set,
// Path is "/", and no Domain attribute is present — preventing cookie injection via
// related subdomains. The CSRF and org cookies do not use the prefix: the CSRF
// cookie must be JS-readable for AJAX headers, and the org cookie must be
// JS-readable for WebSocket org context synchronization.
const (
	cookieNameSession       = "pulse_session"
	cookieNameSessionSecure = "__Host-pulse_session"
	CookieNameCSRF          = "pulse_csrf"
	CookieNameOrgID         = "pulse_org_id"
)

// sessionCookieName returns the appropriate session cookie name based on whether
// the connection is secure. When secure, the __Host- prefix is used.
func sessionCookieName(secure bool) string {
	if secure {
		return cookieNameSessionSecure
	}
	return cookieNameSession
}

// readSessionCookie reads the session cookie from the request, checking for the
// __Host- prefixed name first (HTTPS) then falling back to the unprefixed name
// (HTTP or upgrade transition). This ensures sessions survive an HTTP→HTTPS migration.
func readSessionCookie(r *http.Request) (*http.Cookie, error) {
	if c, err := r.Cookie(cookieNameSessionSecure); err == nil {
		return c, nil
	}
	return r.Cookie(cookieNameSession)
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

func explicitAPITokenFromRequest(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}

	if values := r.Header.Values("X-API-Token"); len(values) > 0 {
		return strings.TrimSpace(values[0]), true
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" && strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:]), true
	}

	if isWebSocketUpgrade(r) {
		if values, ok := r.URL.Query()["token"]; ok && len(values) > 0 {
			return strings.TrimSpace(values[0]), true
		}
	}

	return "", false
}

func validateGlobalAPITokenLocked(cfg *config.Config, token string) (*config.APITokenRecord, bool) {
	if cfg == nil || token == "" || !cfg.IsValidAPIToken(token) {
		return nil, false
	}

	config.Mu.RUnlock()
	config.Mu.Lock()
	record, ok := cfg.ValidateAPIToken(token)
	config.Mu.Unlock()
	config.Mu.RLock()

	if !ok {
		return nil, false
	}
	return record, true
}

func validateAPITokenAgainstConfigsLocked(globalCfg, targetCfg *config.Config, token string) (*config.APITokenRecord, bool) {
	if token == "" {
		return nil, false
	}

	if targetCfg != nil && targetCfg != globalCfg {
		if record, ok := targetCfg.ValidateAPIToken(token); ok {
			return record, true
		}
	}

	return validateGlobalAPITokenLocked(globalCfg, token)
}

// CheckProxyAuth validates proxy authentication headers
func CheckProxyAuth(cfg *config.Config, r *http.Request) (bool, string, bool) {
	// Check if proxy auth is configured
	if cfg.ProxyAuthSecret == "" {
		return false, "", false
	}

	// Validate proxy secret header
	proxySecret := r.Header.Get("X-Proxy-Secret")
	if subtle.ConstantTimeCompare([]byte(proxySecret), []byte(cfg.ProxyAuthSecret)) != 1 {
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
	// Dev mode bypass for all auth (disabled by default)
	if adminBypassEnabled() {
		if w != nil {
			// Set headers for standard admin user
			w.Header().Set("X-Authenticated-User", "admin")
			w.Header().Set("X-Auth-Method", "bypass")
		}
		return true
	}

	if cfg == nil {
		path := ""
		if r != nil && r.URL != nil {
			path = r.URL.Path
		}
		log.Error().
			Str("path", path).
			Msg("CheckAuth called without configuration")
		if w != nil {
			http.Error(w, "Authentication unavailable", http.StatusServiceUnavailable)
		}
		return false
	}

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

	// If no auth is configured at all, allow access unless SSO is enabled.
	if cfg.AuthUser == "" && cfg.AuthPass == "" && !cfg.HasAPITokens() && cfg.ProxyAuthSecret == "" {
		if hasEnabledSSOProvidersForAuth(cfg) {
			log.Debug().Msg("SSO enabled without local credentials, authentication required")
		} else {
			log.Debug().Msg("No auth configured, allowing access as 'anonymous'")
			if w != nil {
				w.Header().Set("X-Authenticated-User", "anonymous")
				w.Header().Set("X-Auth-Method", "none")
			}
			return true
		}
	}

	// API-only mode: when only API token is configured (no password auth)
	if cfg.AuthUser == "" && cfg.AuthPass == "" && cfg.HasAPITokens() {
		if providedToken, provided := explicitAPITokenFromRequest(r); provided {
			if record, ok := validateGlobalAPITokenLocked(cfg, providedToken); ok {
				attachAPITokenRecord(r, record)
				if authenticatedUser := apiTokenAuthenticatedUser(record); authenticatedUser != "" {
					w.Header().Set("X-Authenticated-User", authenticatedUser)
				}
				w.Header().Set("X-Auth-Method", "api_token")
				return true
			}
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

	authenticateToken := func(token string) bool {
		if record, ok := validateGlobalAPITokenLocked(cfg, token); ok {
			attachAPITokenRecord(r, record)
			if authenticatedUser := apiTokenAuthenticatedUser(record); authenticatedUser != "" {
				w.Header().Set("X-Authenticated-User", authenticatedUser)
			}
			w.Header().Set("X-Auth-Method", "api_token")
			return true
		}
		return false
	}

	// Explicit token credentials always take precedence over session/basic auth.
	if cfg.HasAPITokens() {
		if providedToken, provided := explicitAPITokenFromRequest(r); provided {
			if authenticateToken(providedToken) {
				return true
			}
			if w != nil {
				http.Error(w, "Invalid API token", http.StatusUnauthorized)
			}
			return false
		}
	}

	// Check session cookie (for WebSocket and UI)
	if cookie, err := readSessionCookie(r); err == nil && cookie.Value != "" {
		// Use ValidateAndExtendSession for sliding expiration
		if ValidateAndExtendSession(cookie.Value) {
			username := GetSessionUsername(cookie.Value)
			session := GetSessionStore().GetSession(cookie.Value)
			if session != nil && session.OIDCRefreshToken != "" && hasEnabledSSOProvidersForAuth(cfg) {
				// Check if access token is expired or about to expire (5 min buffer)
				if time.Now().Add(5 * time.Minute).After(session.OIDCAccessTokenExp) {
					go refreshOIDCSessionTokens(cfg, cookie.Value, session)
				}
			}
			if username != "" {
				w.Header().Set("X-Authenticated-User", username)
			}
			if session != nil && strings.TrimSpace(session.OIDCIssuer) != "" {
				w.Header().Set("X-Auth-Method", "oidc")
			} else {
				w.Header().Set("X-Auth-Method", "session")
			}
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
								LogAuditEventForTenant(GetOrgID(r.Context()), "login", parts[0], clientIP, r.URL.Path, false, "Rate limited")
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
							LogAuditEventForTenant(GetOrgID(r.Context()), "login", parts[0], clientIP, r.URL.Path, false, "Account locked")
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
								// Invalidate any pre-existing session to prevent session fixation attacks.
								InvalidateOldSessionFromRequest(r)

								token := generateSessionToken()
								if token == "" {
									return false
								}

								// Store session persistently (including username for restart survival)
								userAgent := r.Header.Get("User-Agent")
								clientIP := GetClientIP(r)
								GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP, parts[0])

								// Track session for user (in-memory for fast lookups)
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
									Name:     sessionCookieName(isSecure),
									Value:    token,
									Path:     "/",
									HttpOnly: true,
									Secure:   isSecure,
									SameSite: sameSitePolicy,
									MaxAge:   86400, // 24 hours
								})

								// Set CSRF cookie (not HttpOnly so JS can read it)
								http.SetCookie(w, &http.Cookie{
									Name:     CookieNameCSRF,
									Value:    csrfToken,
									Path:     "/",
									Secure:   isSecure,
									SameSite: sameSitePolicy,
									MaxAge:   86400, // 24 hours
								})

								// Audit log successful login
								LogAuditEventForTenant(GetOrgID(r.Context()), "login", parts[0], GetClientIP(r), r.URL.Path, true, "Basic auth login")
							}
							w.Header().Set("X-Authenticated-User", parts[0])
							w.Header().Set("X-Auth-Method", "basic")
							return true
						} else {
							// Failed login
							RecordFailedLogin(parts[0])
							RecordFailedLogin(clientIP)
							LogAuditEventForTenant(GetOrgID(r.Context()), "login", parts[0], clientIP, r.URL.Path, false, "Invalid credentials")

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

// RequireAdmin middleware checks for authentication and admin privileges.
// Proxy-auth users must have the configured admin role. Session/OIDC users
// must match the configured admin identity.
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

		// Enforce configured admin identity for session-based auth.
		if !ensureAdminSession(cfg, w, r) {
			return
		}

		// User is authenticated and has admin privileges.
		handler(w, r)
	}
}

// RequirePermission middleware checks for authentication and specific RBAC permissions
func RequirePermission(cfg *config.Config, authorizer auth.Authorizer, action, resource string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// First check if user is authenticated (using RequireAdmin logic as base)
		if !CheckAuth(cfg, w, r) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"authentication_required","message":"Authentication required"}`))
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}

		// Check if using proxy auth and if so, verify admin status.
		// When a real RBAC authorizer is active (non-DefaultAuthorizer), non-admin
		// proxy users are allowed through to the RBAC check below, which may grant
		// access based on their role assignments. Without RBAC, non-admin proxy
		// users are hard-rejected since there's no other authorization mechanism.
		if cfg.ProxyAuthSecret != "" {
			if valid, username, isAdmin := CheckProxyAuth(cfg, r); valid {
				if !isAdmin {
					// Check if a real RBAC authorizer is active
					_, isDefaultAuth := authorizer.(*internalauth.DefaultAuthorizer)
					if isDefaultAuth {
						// No RBAC: non-admin proxy users are rejected
						log.Warn().
							Str("ip", r.RemoteAddr).
							Str("path", r.URL.Path).
							Str("action", action).
							Str("resource", resource).
							Str("username", username).
							Msg("Non-admin proxy user attempted to access permissioned endpoint (no RBAC active)")

						if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusForbidden)
							w.Write([]byte(`{"error":"Admin privileges required"}`))
						} else {
							http.Error(w, "Admin privileges required", http.StatusForbidden)
						}
						return
					}
					// RBAC active: defer to authorizer check below
					log.Debug().
						Str("username", username).
						Str("action", action).
						Str("resource", resource).
						Msg("Non-admin proxy user deferred to RBAC authorizer")
				}
			}
		}

		// Extract user from header (set by CheckAuth) and inject into context
		username := w.Header().Get("X-Authenticated-User")
		ctx := r.Context()
		if username != "" {
			ctx = internalauth.WithUser(ctx, username)
		}

		// Check permission via authorizer
		allowed, err := authorizer.Authorize(ctx, action, resource)
		if err != nil {
			log.Error().Err(err).Str("user", username).Str("action", action).Str("resource", resource).Msg("RBAC authorization failed due to system error")
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal_error","message":"Failed to verify permissions"}`))
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		if !allowed {
			log.Warn().
				Str("user", username).
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Str("action", action).
				Str("resource", resource).
				Msg("Forbidden access attempt (RBAC)")

			if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.Header.Get("Accept"), "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":    "forbidden",
					"message":  "You do not have permission to perform this action",
					"action":   action,
					"resource": resource,
				})
			} else {
				http.Error(w, "Forbidden", http.StatusForbidden)
			}
			return
		}

		next(w, r.WithContext(ctx))
	}
}

// RequireScope ensures that token-authenticated requests include the specified scope.
// Session-based (browser) requests bypass the scope check.
func RequireScope(scope string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ensureScope(w, r, scope) {
			return
		}
		handler(w, r)
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
	ctx := internalauth.WithAPIToken(r.Context(), &clone)
	*r = *r.WithContext(ctx)
}

// attachUserContext stores the authenticated username in the request context.
func attachUserContext(r *http.Request, username string) *http.Request {
	if username == "" {
		return r
	}
	ctx := internalauth.WithUser(r.Context(), username)
	return r.WithContext(ctx)
}

func attachAdminBypassContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), adminBypassContextKey, true)
	return r.WithContext(ctx)
}

func isAdminBypassRequest(ctx context.Context) bool {
	bypass, ok := ctx.Value(adminBypassContextKey).(bool)
	return ok && bypass
}

// AuthContextMiddleware creates a middleware that extracts auth info and stores it in context.
// This should run early in the middleware chain so subsequent middleware can access auth context.
// Note: This middleware does NOT enforce authentication - it only populates context.
// Use RequireAuth for enforcement.
// AuthContextMiddleware creates a middleware that extracts auth info and stores it in context.
// This should run early in the middleware chain so subsequent middleware can access auth context.
// Note: This middleware does NOT enforce authentication - it only populates context.
// Use RequireAuth for enforcement.
func AuthContextMiddleware(cfg *config.Config, mtm *monitoring.MultiTenantMonitor, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to extract auth info and store in context WITHOUT enforcing auth
		// This allows tenant middleware to check authorization later
		r = extractAndStoreAuthContext(cfg, mtm, r)
		next.ServeHTTP(w, r)
	})
}

// extractAndStoreAuthContext extracts user/token info from the request and stores in context.
// Returns the request with updated context. Does not enforce auth.
func extractAndStoreAuthContext(cfg *config.Config, mtm *monitoring.MultiTenantMonitor, r *http.Request) *http.Request {
	// Use RLock for common case, upgrade to Lock only if we need to update token stats
	config.Mu.RLock()
	defer config.Mu.RUnlock()

	// Dev mode bypass
	if adminBypassEnabled() {
		return attachAdminBypassContext(attachUserContext(r, "admin"))
	}

	// Check proxy auth
	if cfg.ProxyAuthSecret != "" {
		if valid, username, _ := CheckProxyAuth(cfg, r); valid && username != "" {
			return attachUserContext(r, username)
		}
	}

	// Check API tokens
	// Check API tokens
	// We need to check if EITHER the global config has tokens OR if we have a tenant monitor (which might have tokens)
	if cfg.HasAPITokens() || mtm != nil {
		// Determine which config to use for validation (Global vs Tenant)
		targetConfig := cfg

		if mtm != nil {
			// Check for Tenant ID in header or cookie
			orgID := "default"
			if headerOrgID := r.Header.Get("X-Pulse-Org-ID"); headerOrgID != "" {
				orgID = headerOrgID
			} else if cookie, err := r.Cookie(CookieNameOrgID); err == nil && cookie.Value != "" {
				orgID = cookie.Value
			}

			// If targeting a specific tenant, try to load that tenant's config
			if orgID != "default" {
				// Prevent DoS: Check if org exists before loading (which triggers directory creation)
				if mtm.OrgExists(orgID) {
					if m, err := mtm.GetMonitor(orgID); err == nil && m != nil {
						targetConfig = m.GetConfig()
					}
				}
			}
		}

		validateToken := func(token string) (*http.Request, bool) {
			if record, ok := validateAPITokenAgainstConfigsLocked(cfg, targetConfig, token); ok {
				attachAPITokenRecord(r, record)
				return attachUserContext(r, apiTokenAuthenticatedUser(record)), true
			}
			return nil, false
		}

		if providedToken, provided := explicitAPITokenFromRequest(r); provided {
			if req, ok := validateToken(providedToken); ok {
				return req
			}
			return r
		}
	}

	// Check session cookie
	if cookie, err := readSessionCookie(r); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
			if username := GetSessionUsername(cookie.Value); username != "" {
				return attachUserContext(r, username)
			}
		}
	}

	return r
}

func getAPITokenRecordFromRequest(r *http.Request) *config.APITokenRecord {
	val := internalauth.GetAPIToken(r.Context())
	if val == nil {
		return nil
	}
	record, ok := val.(*config.APITokenRecord)
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

// oidcRefreshMutex prevents concurrent refresh attempts for the same session
var oidcRefreshMutex sync.Map

// refreshOIDCSessionTokens refreshes OIDC tokens for a session in the background
// If refresh fails, the session is invalidated and the user will need to re-login
func refreshOIDCSessionTokens(cfg *config.Config, sessionToken string, session *SessionData) {
	// Prevent concurrent refresh attempts for the same session
	if _, loaded := oidcRefreshMutex.LoadOrStore(sessionToken, true); loaded {
		return // Another goroutine is already refreshing this session
	}
	defer oidcRefreshMutex.Delete(sessionToken)

	// Mark session as refreshing to prevent duplicate attempts
	GetSessionStore().SetTokenRefreshing(sessionToken, true)
	defer GetSessionStore().SetTokenRefreshing(sessionToken, false)

	log.Debug().
		Str("issuer", session.OIDCIssuer).
		Time("token_expiry", session.OIDCAccessTokenExp).
		Msg("Attempting OIDC token refresh")

	// Create a context with timeout for the refresh operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Resolve OIDC provider config from v6 enabled SSO providers.
	oidcCfg, providerID := resolveOIDCRefreshConfig(cfg, session)
	if oidcCfg == nil {
		// Session may belong to a disabled/removed provider. Skip refresh silently;
		// the session continues until natural expiry.
		log.Debug().Msg("No matching enabled SSO OIDC provider for session refresh")
		return
	}

	// Create a temporary OIDC service for refreshing
	service, err := NewOIDCService(ctx, oidcCfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create OIDC service for token refresh")
		return
	}

	// Attempt to refresh the token
	result, err := service.RefreshToken(ctx, session.OIDCRefreshToken)
	if err != nil {
		log.Warn().
			Err(err).
			Str("issuer", session.OIDCIssuer).
			Str("provider_id", providerID).
			Msg("OIDC token refresh failed - invalidating session")

		// Token refresh failed - this usually means the refresh token was revoked
		// or expired. Invalidate the session to force re-login.
		GetSessionStore().InvalidateSession(sessionToken)
		LogAuditEvent("oidc_token_refresh", "", "", "", false, "Token refresh failed: "+err.Error())
		return
	}

	// Update the session with new tokens
	GetSessionStore().UpdateOIDCTokens(sessionToken, result.RefreshToken, result.Expiry)

	log.Info().
		Time("new_expiry", result.Expiry).
		Str("provider_id", providerID).
		Msg("OIDC token refresh successful - session extended")

	LogAuditEvent("oidc_token_refresh", "", "", "", true, "Token refreshed successfully")
}
