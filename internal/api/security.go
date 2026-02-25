package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rs/zerolog/log"
)

// Security improvements for Pulse

// generateCSRFToken creates a new CSRF token for a session
func generateCSRFToken(sessionID string) string {
	return GetCSRFStore().GenerateCSRFToken(sessionID)
}

// validateCSRFToken checks if a CSRF token is valid for a session
func validateCSRFToken(sessionID, token string) bool {
	return GetCSRFStore().ValidateCSRFToken(sessionID, token)
}

// CheckCSRF validates CSRF token for state-changing requests
func CheckCSRF(w http.ResponseWriter, r *http.Request) bool {
	// Skip CSRF check for safe methods
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return true
	}

	// Skip CSRF for API token auth (API clients don't have sessions)
	if r.Header.Get("X-API-Token") != "" {
		log.Debug().Str("path", r.URL.Path).Msg("CSRF check skipped: API token auth")
		return true
	}

	// Skip CSRF only for explicit non-session auth schemes.
	if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader != "" {
		lower := strings.ToLower(authHeader)
		if strings.HasPrefix(lower, "basic ") {
			log.Debug().Str("path", r.URL.Path).Msg("CSRF check skipped: Basic auth header present")
			return true
		}
		if strings.HasPrefix(lower, "bearer ") {
			log.Debug().Str("path", r.URL.Path).Msg("CSRF check skipped: Bearer auth header present")
			return true
		}
	}

	// Get session from cookie
	cookie, err := r.Cookie("pulse_session")
	if err != nil {
		// No session cookie means no CSRF check needed
		// (either no auth configured or using basic auth which doesn't use sessions)
		log.Debug().Str("path", r.URL.Path).Msg("CSRF check skipped: no session cookie")
		return true
	}

	// Get CSRF token from header or form
	csrfToken := r.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		csrfToken = r.FormValue("csrf_token")
	}

	// Log CSRF validation attempt for debugging
	log.Debug().
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Str("session", safePrefixForLog(cookie.Value, 8)+"...").
		Bool("has_csrf_token", csrfToken != "").
		Msg("CSRF validation attempt")

	// No CSRF token means request is not eligible for mutation
	if csrfToken == "" {
		log.Warn().
			Str("path", r.URL.Path).
			Str("session", safePrefixForLog(cookie.Value, 8)+"...").
			Msg("Missing CSRF token")
		clearCSRFCookie(w, r)
		if newToken := issueNewCSRFCookie(w, r, cookie.Value); newToken != "" {
			w.Header().Set("X-CSRF-Token", newToken)
			log.Debug().Str("new_token", safePrefixForLog(newToken, 8)+"...").Msg("Issued new CSRF token after missing")
		}
		return false
	}

	// Check if the CSRF token validates
	if !validateCSRFToken(cookie.Value, csrfToken) {
		log.Warn().
			Str("path", r.URL.Path).
			Str("session", safePrefixForLog(cookie.Value, 8)+"...").
			Str("provided_token", safePrefixForLog(csrfToken, 8)+"...").
			Msg("Invalid CSRF token")
		clearCSRFCookie(w, r)
		if newToken := issueNewCSRFCookie(w, r, cookie.Value); newToken != "" {
			w.Header().Set("X-CSRF-Token", newToken)
			log.Debug().Str("new_token", safePrefixForLog(newToken, 8)+"...").Msg("Issued new CSRF token after invalid")
		}
		return false
	}

	log.Debug().
		Str("path", r.URL.Path).
		Str("session", safePrefixForLog(cookie.Value, 8)+"...").
		Msg("CSRF validation successful")
	return true
}

func clearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if w == nil {
		return
	}
	var secure bool
	var sameSite http.SameSite
	if r != nil {
		secure, sameSite = getCookieSettings(r)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_csrf",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func issueNewCSRFCookie(w http.ResponseWriter, r *http.Request, sessionID string) string {
	if w == nil || r == nil {
		return ""
	}
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}

	newToken := generateCSRFToken(sessionID)
	secure, sameSite := getCookieSettings(r)

	http.SetCookie(w, &http.Cookie{
		Name:     "pulse_csrf",
		Value:    newToken,
		Path:     "/",
		Secure:   secure,
		SameSite: sameSite,
		MaxAge:   86400,
	})
	return newToken
}

// Rate Limiting - using existing RateLimiter from ratelimit.go
var (
	// Auth endpoints: 10 attempts per minute
	authLimiter = NewRateLimiter(10, 1*time.Minute)
)

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	rawRemoteIP := extractRemoteIP(r.RemoteAddr)
	if rawRemoteIP == "" {
		return ""
	}

	// Only trust proxy headers when the immediate peer is trusted.
	if isTrustedProxyIP(rawRemoteIP) {
		if forwarded := firstValidForwardedIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			return forwarded
		}

		if realIP := strings.TrimSpace(strings.Trim(r.Header.Get("X-Real-IP"), "[]")); realIP != "" && net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	return rawRemoteIP
}

// Failed Login Tracking
type FailedLogin struct {
	Count       int
	LastAttempt time.Time
	LockedUntil time.Time
}

var (
	failedLogins = make(map[string]*FailedLogin)
	failedMu     sync.RWMutex

	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute

	trustedProxyOnce  sync.Once
	trustedProxyCIDRs []*net.IPNet
)

func loadTrustedProxyCIDRs() {
	raw := utils.GetenvTrim("PULSE_TRUSTED_PROXY_CIDRS")
	if raw == "" {
		return
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, network, parseErr := net.ParseCIDR(entry)
			if parseErr == nil {
				network.IP = network.IP.Mask(network.Mask)
				trustedProxyCIDRs = append(trustedProxyCIDRs, network)
				continue
			}
			log.Warn().
				Str("cidr", entry).
				Err(parseErr).
				Msg("Ignoring invalid CIDR in PULSE_TRUSTED_PROXY_CIDRS")
			continue
		}

		ip := net.ParseIP(entry)
		if ip == nil {
			log.Warn().
				Str("value", entry).
				Msg("Ignoring invalid IP in PULSE_TRUSTED_PROXY_CIDRS")
			continue
		}

		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		mask := net.CIDRMask(bits, bits)
		network := &net.IPNet{IP: ip.Mask(mask), Mask: mask}
		trustedProxyCIDRs = append(trustedProxyCIDRs, network)
	}
}

func extractRemoteIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(remoteAddr, "[]")
}

func firstValidForwardedIP(header string) string {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(strings.Trim(part, "[]"))
		if part == "" {
			continue
		}

		if net.ParseIP(part) != nil {
			return part
		}
	}
	return ""
}

func isTrustedProxyIP(ipStr string) bool {
	ipStr = strings.TrimSpace(strings.Trim(ipStr, "[]"))
	if ipStr == "" {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	trustedProxyOnce.Do(loadTrustedProxyCIDRs)
	if len(trustedProxyCIDRs) == 0 {
		return false
	}
	for _, network := range trustedProxyCIDRs {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func isPrivateIP(ip string) bool {
	host := extractRemoteIP(ip)
	if host == "" {
		return false
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	if parsedIP.IsLoopback() ||
		parsedIP.IsLinkLocalUnicast() ||
		parsedIP.IsLinkLocalMulticast() {
		return true
	}

	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

func isTrustedNetwork(ip string, trustedNetworks []string) bool {
	if len(trustedNetworks) == 0 {
		return isPrivateIP(ip)
	}

	host := extractRemoteIP(ip)
	if host == "" {
		return false
	}

	parsedIP := net.ParseIP(host)
	if parsedIP == nil {
		return false
	}

	for _, cidr := range trustedNetworks {
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// RecordFailedLogin tracks failed login attempts
func RecordFailedLogin(identifier string) {
	failedMu.Lock()
	defer failedMu.Unlock()

	failed, exists := failedLogins[identifier]
	if !exists {
		failed = &FailedLogin{}
		failedLogins[identifier] = failed
	}

	failed.Count++
	failed.LastAttempt = time.Now()

	if failed.Count >= maxFailedAttempts {
		failed.LockedUntil = time.Now().Add(lockoutDuration)
		log.Warn().
			Str("identifier", identifier).
			Int("attempts", failed.Count).
			Time("locked_until", failed.LockedUntil).
			Msg("Account locked due to failed login attempts")
	}
}

// ClearFailedLogins resets failed login counter on successful login
func ClearFailedLogins(identifier string) {
	failedMu.Lock()
	defer failedMu.Unlock()
	delete(failedLogins, identifier)
}

// GetLockoutInfo returns lockout information for an identifier
func GetLockoutInfo(identifier string) (attempts int, lockedUntil time.Time, isLocked bool) {
	failedMu.RLock()
	defer failedMu.RUnlock()

	failed, exists := failedLogins[identifier]
	if !exists {
		return 0, time.Time{}, false
	}

	// Check if lockout has expired
	if time.Now().After(failed.LockedUntil) && failed.Count >= maxFailedAttempts {
		// Lockout expired, treat as no attempts
		return 0, time.Time{}, false
	}

	isLocked = failed.Count >= maxFailedAttempts && time.Now().Before(failed.LockedUntil)
	return failed.Count, failed.LockedUntil, isLocked
}

// ResetLockout manually resets lockout for an identifier (admin function)
func ResetLockout(identifier string) {
	failedMu.Lock()
	defer failedMu.Unlock()
	delete(failedLogins, identifier)

	log.Info().
		Str("identifier", identifier).
		Msg("Lockout manually reset")
}

// SecurityHeadersWithConfig applies security headers with embedding configuration
func SecurityHeadersWithConfig(next http.Handler, allowEmbedding bool, allowedOrigins string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Configure clickjacking protection based on embedding settings
		if allowEmbedding {
			// When embedding is allowed, don't set X-Frame-Options header
			// frame-ancestors CSP directive controls allowed embed origins below
			// Security note: User explicitly enabled this for iframe embedding
		} else {
			// Deny all embedding when not explicitly allowed
			w.Header().Set("X-Frame-Options", "DENY")
		}

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Disable legacy XSS auditor — it is removed from modern browsers and
		// can introduce vulnerabilities in older ones.  CSP provides XSS protection.
		w.Header().Set("X-XSS-Protection", "0")

		// Build Content Security Policy
		cspDirectives := []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'", // TODO: migrate to nonce-based CSP; currently needed for SolidJS bundled output
			"style-src 'self' 'unsafe-inline'",                // Needed for inline styles in SolidJS components
			"img-src 'self' data: blob:",
			"connect-src 'self' ws: wss:", // WebSocket support
			"font-src 'self' data:",
		}

		// Add frame-ancestors based on embedding settings
		if allowEmbedding {
			if allowedOrigins != "" {
				// Parse comma-separated origins and add them to frame-ancestors
				origins := strings.Split(allowedOrigins, ",")
				frameAncestors := "frame-ancestors 'self'"
				for _, origin := range origins {
					origin = strings.TrimSpace(origin)
					if origin != "" {
						frameAncestors += " " + origin
					}
				}
				cspDirectives = append(cspDirectives, frameAncestors)
			} else {
				// Default to self-only when embedding is enabled but no specific origins configured.
				// This prevents clickjacking while still allowing same-origin iframes.
				cspDirectives = append(cspDirectives, "frame-ancestors 'self'")
			}
		} else {
			// Deny all embedding
			cspDirectives = append(cspDirectives, "frame-ancestors 'none'")
		}

		// Upgrade HTTP sub-resource requests to HTTPS when the page is served over HTTPS
		if shouldSetHSTS(r) {
			cspDirectives = append(cspDirectives, "upgrade-insecure-requests")
		}

		w.Header().Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()")

		// Enable HSTS only for requests known to be HTTPS.
		// Forwarded proto is trusted only when the direct peer is a trusted proxy.
		if shouldSetHSTS(r) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

func shouldSetHSTS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}

	peerIP := extractRemoteIP(r.RemoteAddr)
	if !isTrustedProxyIP(peerIP) {
		return false
	}

	proto := strings.ToLower(firstForwardedValue(r.Header.Get("X-Forwarded-Proto")))
	if proto == "" {
		proto = strings.ToLower(firstForwardedValue(r.Header.Get("X-Forwarded-Scheme")))
	}
	return proto == "https"
}

// LogAuditEvent logs security-relevant events using the audit package.
// This function delegates to the configured audit.Logger, allowing enterprise
// versions to provide persistent storage and signing.
// For tenant-aware logging, use LogAuditEventForTenant instead.
func LogAuditEvent(event string, user string, ip string, path string, success bool, details string) {
	audit.Log(event, user, ip, path, success, details)
}

// LogAuditEventForTenant logs security-relevant events to a tenant-specific audit log.
// Uses the TenantLoggerManager to route events to the correct tenant's audit database.
func LogAuditEventForTenant(orgID, event, user, ip, path string, success bool, details string) {
	manager := GetTenantAuditManager()
	if manager == nil {
		// Fall back to global logger
		audit.Log(event, user, ip, path, success, details)
		return
	}
	if err := manager.Log(orgID, event, user, ip, path, success, details); err != nil {
		// If tenant logging fails, fall back to global logger
		audit.Log(event, user, ip, path, success, details)
	}
}

// global tenant audit manager
var (
	tenantAuditManager   *audit.TenantLoggerManager
	tenantAuditManagerMu sync.RWMutex
)

// SetTenantAuditManager sets the global tenant audit manager.
func SetTenantAuditManager(manager *audit.TenantLoggerManager) {
	tenantAuditManagerMu.Lock()
	defer tenantAuditManagerMu.Unlock()
	tenantAuditManager = manager
}

// GetTenantAuditManager returns the global tenant audit manager.
func GetTenantAuditManager() *audit.TenantLoggerManager {
	tenantAuditManagerMu.RLock()
	defer tenantAuditManagerMu.RUnlock()
	return tenantAuditManager
}

// Session Management Improvements
var (
	allSessions = make(map[string][]string) // user -> []sessionIDs
	sessionsMu  sync.RWMutex
)

// maxSessionsPerUser limits concurrent sessions to prevent session accumulation.
const maxSessionsPerUser = 10

// TrackUserSession tracks which sessions belong to which user.
// When the limit is exceeded, the oldest sessions are evicted.
func TrackUserSession(user, sessionID string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	if allSessions[user] == nil {
		allSessions[user] = []string{}
	}

	// Add the new session
	allSessions[user] = append(allSessions[user], sessionID)

	// If near the limit, prune stale session IDs (already deleted via logout/
	// rotation/expiry) before evicting valid sessions.
	if len(allSessions[user]) > maxSessionsPerUser {
		store := GetSessionStore()
		alive := make([]string, 0, len(allSessions[user]))
		for _, sid := range allSessions[user] {
			if store.ValidateSession(sid) {
				alive = append(alive, sid)
			}
		}
		allSessions[user] = alive

		// After pruning stale entries, evict oldest if still over the limit
		if len(allSessions[user]) > maxSessionsPerUser {
			excess := allSessions[user][:len(allSessions[user])-maxSessionsPerUser]
			for _, oldSID := range excess {
				store.DeleteSession(oldSID)
				GetCSRFStore().DeleteCSRFToken(oldSID)
			}
			allSessions[user] = allSessions[user][len(allSessions[user])-maxSessionsPerUser:]
		}
	}
}

// GetSessionUsername returns the username associated with a session ID
func GetSessionUsername(sessionID string) string {
	// First check in-memory map
	sessionsMu.RLock()
	for user, sessions := range allSessions {
		for _, sid := range sessions {
			if sid == sessionID {
				sessionsMu.RUnlock()
				return user
			}
		}
	}
	sessionsMu.RUnlock()

	// Fall back to persisted username in session store (survives restarts)
	if session := GetSessionStore().GetSession(sessionID); session != nil && session.Username != "" {
		// Re-populate in-memory map for faster future lookups
		TrackUserSession(session.Username, sessionID)
		return session.Username
	}

	return ""
}

// InvalidateUserSessions invalidates all sessions for a user (e.g., on password change)
func InvalidateUserSessions(user string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	sessionIDs := allSessions[user]
	for _, sid := range sessionIDs {
		// Delete from persistent session store
		GetSessionStore().DeleteSession(sid)

		// Delete CSRF tokens
		GetCSRFStore().DeleteCSRFToken(sid)
	}

	delete(allSessions, user)

	log.Info().
		Str("user", user).
		Int("sessions_invalidated", len(sessionIDs)).
		Msg("Invalidated all user sessions")
}

// UntrackUserSession removes all occurrences of a session from a user's session list
// (used for single session logout, not password change which clears all)
func UntrackUserSession(user, sessionID string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	sessions := allSessions[user]
	filtered := sessions[:0]
	for _, sid := range sessions {
		if sid != sessionID {
			filtered = append(filtered, sid)
		}
	}
	allSessions[user] = filtered
}

// InvalidateOldSessionFromRequest destroys any pre-existing session cookie to
// prevent session fixation attacks. Call this before creating a new session.
// It deletes the session from the persistent store, its CSRF token, and
// removes it from the in-memory user session tracking map.
func InvalidateOldSessionFromRequest(r *http.Request) {
	cookie, err := r.Cookie("pulse_session")
	if err != nil || cookie.Value == "" {
		return
	}
	oldToken := cookie.Value

	// Remove from persistent store
	GetSessionStore().DeleteSession(oldToken)
	GetCSRFStore().DeleteCSRFToken(oldToken)

	// Remove from in-memory tracking so GetSessionUsername won't resolve it
	user := GetSessionUsername(oldToken)
	if user != "" {
		UntrackUserSession(user, oldToken)
	}
}
