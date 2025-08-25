package api

import (
	"net/http"
	"strings"
	"sync"
	"time"
	
	"github.com/rs/zerolog/log"
)

// Security improvements for Pulse

// CSRF Protection
type CSRFToken struct {
	Token   string
	Expires time.Time
}

// CSRF tokens are now managed by the persistent CSRFTokenStore

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
		return true
	}
	
	// Skip CSRF for Basic Auth (doesn't use sessions, not vulnerable to CSRF)
	if r.Header.Get("Authorization") != "" {
		return true
	}
	
	// Get session from cookie
	cookie, err := r.Cookie("pulse_session")
	if err != nil {
		// No session cookie means no CSRF check needed
		// (either no auth configured or using basic auth which doesn't use sessions)
		return true
	}
	
	// Get CSRF token from header or form
	csrfToken := r.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		csrfToken = r.FormValue("csrf_token")
	}
	
	// If no CSRF token is provided, check if this is a valid session
	// This handles the case where the server restarted and lost CSRF tokens
	if csrfToken == "" {
		// No CSRF token provided - this is definitely invalid
		log.Warn().
			Str("path", r.URL.Path).
			Str("session", cookie.Value[:8]+"...").
			Msg("Missing CSRF token")
		return false
	}
	
	// Check if the CSRF token validates
	if !validateCSRFToken(cookie.Value, csrfToken) {
		// CSRF validation failed, but check if session is still valid
		// If session is valid but CSRF token doesn't match, it might be due to server restart
		if ValidateSession(cookie.Value) {
			// Valid session but mismatched CSRF - likely server restart
			// Generate a new CSRF token for this session
			newToken := generateCSRFToken(cookie.Value)
			
			// Detect if we're behind a proxy/tunnel
			isProxied := r.Header.Get("X-Forwarded-For") != "" || 
				r.Header.Get("X-Real-IP") != "" ||
				r.Header.Get("CF-Ray") != "" ||
				r.Header.Get("X-Forwarded-Proto") != ""
			
			sameSitePolicy := http.SameSiteStrictMode
			if isProxied {
				sameSitePolicy = http.SameSiteNoneMode
			}
			
			isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
			
			// Set the new CSRF token as a cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "pulse_csrf",
				Value:    newToken,
				Path:     "/",
				Secure:   isSecure,
				SameSite: sameSitePolicy,
				MaxAge:   86400, // 24 hours
			})
			// For this request, we'll be lenient and allow it through
			log.Debug().
				Str("path", r.URL.Path).
				Str("session", cookie.Value[:8]+"...").
				Msg("Regenerated CSRF token after server restart")
			return true
		}
		
		log.Warn().
			Str("path", r.URL.Path).
			Str("session", cookie.Value[:8]+"...").
			Str("provided_token", csrfToken[:8]+"...").
			Msg("Invalid CSRF token")
		return false
	}
	
	return true
}

// Rate Limiting - using existing RateLimiter from ratelimit.go
var (
	// Auth endpoints: 10 attempts per minute
	authLimiter = NewRateLimiter(10, 1*time.Minute)
	
	// General API: 500 requests per minute (increased for metadata endpoints)
	apiLimiter = NewRateLimiter(500, 1*time.Minute)
)

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	
	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// Failed Login Tracking
type FailedLogin struct {
	Count      int
	LastAttempt time.Time
	LockedUntil time.Time
}

var (
	failedLogins = make(map[string]*FailedLogin)
	failedMu     sync.RWMutex
	
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

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

// IsLockedOut checks if an account is locked out
func IsLockedOut(identifier string) bool {
	failedMu.RLock()
	defer failedMu.RUnlock()
	
	failed, exists := failedLogins[identifier]
	if !exists {
		return false
	}
	
	if time.Now().After(failed.LockedUntil) {
		// Lockout expired
		return false
	}
	
	return failed.Count >= maxFailedAttempts
}

// Security Headers Middleware
func SecurityHeaders(next http.Handler) http.Handler {
	return SecurityHeadersWithConfig(next, false, "")
}

// SecurityHeadersWithConfig applies security headers with embedding configuration
func SecurityHeadersWithConfig(next http.Handler, allowEmbedding bool, allowedOrigins string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Configure clickjacking protection based on embedding settings
		if allowEmbedding {
			if allowedOrigins != "" {
				// Use ALLOW-FROM for specific origins (legacy browsers)
				// Note: Most modern browsers ignore this in favor of CSP frame-ancestors
				w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			} else {
				// Allow same-origin embedding
				w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			}
		} else {
			// Deny all embedding
			w.Header().Set("X-Frame-Options", "DENY")
		}
		
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Build Content Security Policy
		cspDirectives := []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'", // Needed for React
			"style-src 'self' 'unsafe-inline'", // Needed for inline styles
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
				// Allow same-origin embedding
				cspDirectives = append(cspDirectives, "frame-ancestors 'self'")
			}
		} else {
			// Deny all embedding
			cspDirectives = append(cspDirectives, "frame-ancestors 'none'")
		}
		
		w.Header().Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))
		
		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy (formerly Feature Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		next.ServeHTTP(w, r)
	})
}

// Audit Logging
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	User      string    `json:"user,omitempty"`
	IP        string    `json:"ip"`
	Path      string    `json:"path,omitempty"`
	Success   bool      `json:"success"`
	Details   string    `json:"details,omitempty"`
}

// LogAuditEvent logs security-relevant events
func LogAuditEvent(event string, user string, ip string, path string, success bool, details string) {
	if success {
		log.Info().
			Str("event", event).
			Str("user", user).
			Str("ip", ip).
			Str("path", path).
			Str("details", details).
			Time("timestamp", time.Now()).
			Msg("Security audit event")
	} else {
		log.Warn().
			Str("event", event).
			Str("user", user).
			Str("ip", ip).
			Str("path", path).
			Str("details", details).
			Time("timestamp", time.Now()).
			Msg("Security audit event - FAILED")
	}
}

// Session Management Improvements
var (
	allSessions = make(map[string][]string) // user -> []sessionIDs
	sessionsMu  sync.RWMutex
)

// TrackUserSession tracks which sessions belong to which user
func TrackUserSession(user, sessionID string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	
	if allSessions[user] == nil {
		allSessions[user] = []string{}
	}
	allSessions[user] = append(allSessions[user], sessionID)
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