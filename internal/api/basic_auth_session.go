package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// requestIsPulseWebClient reports whether Pulse's own web client sent this
// request: frontend-modern/src/utils/apiClient.ts marks every API call it makes.
// Scripts and dashboards do not send the header, and a page on another origin
// cannot without Pulse's CORS policy allowing it, which
// TestPulseCORSNeverAllowsTheWebClientMarker holds. Issuing a session on it is a
// hint, not a boundary: the caller has already proved the credentials, and a
// session grants nothing those credentials do not. isCrossSiteBrowserRequest does
// rely on it, where the browser sent no Sec-Fetch-Site.
func requestIsPulseWebClient(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Requested-With")), "XMLHttpRequest")
}

// issueBasicBrowserSession gives Pulse's web client a browser session for
// verified Basic credentials. A browser behind a proxy that forwards Basic
// credentials, such as an Authentik proxy provider, signs in this way. API clients
// never reach it: each of their calls would take one of the user's
// maxSessionsPerUser sessions and eventually sign the user's browser out.
func issueBasicBrowserSession(w http.ResponseWriter, r *http.Request, username string) bool {
	token := generateSessionToken()
	if token == "" {
		return false
	}

	// Store session persistently (including username for restart survival)
	userAgent := r.Header.Get("User-Agent")
	clientIP := GetClientIP(r)
	GetSessionStore().CreateSession(token, 24*time.Hour, userAgent, clientIP, username)

	// Track session for user (in-memory for fast lookups)
	TrackUserSession(username, token)

	// Generate CSRF token
	csrfToken := generateCSRFToken(token)

	// Get appropriate cookie settings based on proxy detection
	cookiePolicy := getBrowserCookiePolicy(r)

	// Debug logging for Cloudflare tunnel issues
	sameSiteName := "Default"
	switch cookiePolicy.sameSite {
	case http.SameSiteNoneMode:
		sameSiteName = "None"
	case http.SameSiteLaxMode:
		sameSiteName = "Lax"
	case http.SameSiteStrictMode:
		sameSiteName = "Strict"
	}

	log.Debug().
		Bool("secure", cookiePolicy.secure).
		Str("same_site", sameSiteName).
		Str("token", safePrefixForLog(token, 8)+"...").
		Str("remote_addr", r.RemoteAddr).
		Msg("Setting session cookie after successful login")

	// Set session cookie
	cookiePolicy.setHTTPOnly(w, &http.Cookie{
		Name:     sessionCookieName(cookiePolicy.secure),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400, // 24 hours
	})

	// Set CSRF cookie (not HttpOnly so JS can read it)
	cookiePolicy.setClientReadable(w, &http.Cookie{
		Name:   CookieNameCSRF,
		Value:  csrfToken,
		Path:   "/",
		MaxAge: 86400, // 24 hours
	})

	// Audit log successful login
	LogAuditEventForTenant(GetOrgID(r.Context()), "login", username, clientIP, r.URL.Path, true, "Basic auth login")
	return true
}

// basicAuthAuditInterval re-records continued Basic access from one address at
// most this often, so the audit trail still shows it.
const basicAuthAuditInterval = 24 * time.Hour

// maxBasicAuthAudits bounds remembered addresses. Only a caller holding the
// password adds one, and at capacity an unremembered address is audited on every
// call until a slot expires: repeating a record beats losing one.
const maxBasicAuthAudits = 1024

// basicAuthAuditTracker decides which successful API-client Basic calls are audit
// events. A password used only over Basic must leave a trace, but recording every
// poll buries the logins and failures the log exists to show. Failed attempts are
// always audited and never pass through here.
type basicAuthAuditTracker struct {
	mu   sync.Mutex
	last map[basicAuthAuditKey]time.Time
}

type basicAuthAuditKey struct {
	orgID    string
	username string
	clientIP string
}

func newBasicAuthAuditTracker() *basicAuthAuditTracker {
	return &basicAuthAuditTracker{last: make(map[basicAuthAuditKey]time.Time)}
}

var basicAuthAudits = newBasicAuthAuditTracker()

// due reports whether this call is new audit information: the first from this
// organization, user and address since startup, or the first in
// basicAuthAuditInterval.
func (t *basicAuthAuditTracker) due(orgID, username, clientIP string, now time.Time) bool {
	key := basicAuthAuditKey{orgID: orgID, username: username, clientIP: clientIP}

	t.mu.Lock()
	defer t.mu.Unlock()
	if last, seen := t.last[key]; seen {
		if now.Sub(last) < basicAuthAuditInterval {
			return false
		}
		t.last[key] = now
		return true
	}
	if len(t.last) >= maxBasicAuthAudits {
		for candidate, last := range t.last {
			if now.Sub(last) >= basicAuthAuditInterval {
				delete(t.last, candidate)
			}
		}
		if len(t.last) >= maxBasicAuthAudits {
			return true
		}
	}
	t.last[key] = now
	return true
}
