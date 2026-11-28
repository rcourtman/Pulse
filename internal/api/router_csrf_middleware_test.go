package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newRouterWithSession(t *testing.T) (*Router, string) {
	t.Helper()
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)
	InitRecoveryTokenStore(dir)

	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dir,
		ConfigPath: dir,
	}

	router := &Router{
		mux:    http.NewServeMux(),
		config: cfg,
	}

	token := generateSessionToken()
	GetSessionStore().CreateSession(token, time.Hour, "test-agent", "127.0.0.1", "admin")

	return router, token
}

func newRouterWithProxyAuth(t *testing.T) *Router {
	t.Helper()
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dir := t.TempDir()
	InitSessionStore(dir)
	InitCSRFStore(dir)

	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		DataPath:            dir,
		ConfigPath:          dir,
	}

	return &Router{
		mux:    http.NewServeMux(),
		config: cfg,
	}
}

func TestRouterCSRFEnforcedForSessionRequests(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFAllowsValidToken(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	csrfToken := generateCSRFToken(sessionToken)
	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.Header.Set("X-CSRF-Token", csrfToken)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFRetainsConcurrentReplacementTokens(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	replacementTokens := make([]string, 0, 2)
	for _, staleToken := range []string{"stale-token-a", "stale-token-b"} {
		req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
		req.Header.Set("X-CSRF-Token", staleToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status %d for stale token, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
		}
		replacementToken := rec.Header().Get("X-CSRF-Token")
		if replacementToken == "" {
			t.Fatal("expected replacement CSRF token header after stale token")
		}
		replacementTokens = append(replacementTokens, replacementToken)
	}

	for i, replacementToken := range replacementTokens {
		req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
		req.Header.Set("X-CSRF-Token", replacementToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("replacement token %d should remain valid, got status %d (%s)", i, rec.Code, rec.Body.String())
		}
	}
}

func TestRouterCSRFBlocksCrossSiteProxyAuthMutation(t *testing.T) {
	router := newRouterWithProxyAuth(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.Host = "pulse.example.com"
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "alice")
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFAcceptsSameOriginProxyAuthMutation(t *testing.T) {
	router := newRouterWithProxyAuth(t)

	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	req.Host = "pulse.example.com"
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "alice")
	req.Header.Set("Origin", "http://pulse.example.com")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFSpecialCaseSkips(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	skipPaths := []string{
		"/api/login",
		"/api/security/validate-bootstrap-token",
		"/api/setup-script-url",
	}

	for _, path := range skipPaths {
		router.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("path %s: expected status %d, got %d (%s)", path, http.StatusOK, rec.Code, rec.Body.String())
		}
	}
}

func TestRouterCSRFEnforcedForSecurityMutationRoutesWhenAuthConfigured(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	protectedPaths := []string{
		"/api/security/apply-restart",
		"/api/security/quick-setup",
	}

	for _, path := range protectedPaths {
		router.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("path %s: expected status %d, got %d (%s)", path, http.StatusForbidden, rec.Code, rec.Body.String())
		}
	}
}

func TestRouterCSRFNotBypassedForQuickSetupWithInvalidRecoveryToken(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/security/quick-setup", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.Header.Set("X-Recovery-Token", "invalid-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusForbidden, rec.Code, rec.Body.String())
	}
}

func TestRouterCSRFBypassedForQuickSetupWithValidRecoveryToken(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/security/quick-setup", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5*time.Minute, "127.0.0.1")
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.Header.Set("X-Recovery-Token", token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRouterIssuesCSRFCookieForGetWithSession(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var csrfCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_csrf" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatalf("expected pulse_csrf cookie to be set")
	}
	if csrfCookie.Value == "" {
		t.Fatalf("expected pulse_csrf cookie to have a value")
	}
}

func TestRouterDoesNotIssueCSRFCookieWithoutSession(t *testing.T) {
	router, _ := newRouterWithSession(t)

	router.mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	for _, c := range rec.Result().Cookies() {
		if c.Name == "pulse_csrf" {
			t.Fatalf("did not expect pulse_csrf cookie to be set")
		}
	}
}

// Public paths pass the global auth gate, so the cookie alone must not be
// enough for the router to add a token to the persisted CSRF store.
func TestRouterDoesNotIssueCSRFCookieForUnknownSession(t *testing.T) {
	router, _ := newRouterWithSession(t)

	router.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/api/health", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: "made-up-session"})
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		wantStatus := http.StatusOK
		if method == http.MethodPost {
			wantStatus = http.StatusForbidden
		}
		if rec.Code != wantStatus {
			t.Fatalf("%s: expected status %d, got %d (%s)", method, wantStatus, rec.Code, rec.Body.String())
		}
		if token := rec.Header().Get("X-CSRF-Token"); token != "" {
			t.Fatalf("%s: did not expect a replacement CSRF token header, got %q", method, token)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == "pulse_csrf" && c.Value != "" {
				t.Fatalf("%s: did not expect pulse_csrf cookie to be set", method)
			}
		}
	}

	if sessions := csrfSessionCountForTest(); sessions != 0 {
		t.Fatalf("expected no CSRF records for unknown sessions, got %d", sessions)
	}
}

func findCSRFCookie(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == CookieNameCSRF {
			return c
		}
	}
	return nil
}

func TestRouterRefreshesExpiredOrInvalidCSRFCookieOnGet(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/security/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.mux.HandleFunc("/api/secure", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Seed an expired token for this session to simulate a cookie that outlived the 4h store TTL.
	expiredToken := generateCSRFToken(sessionToken)
	store := GetCSRFStore()
	store.mu.Lock()
	store.tokens[csrfSessionKey(sessionToken)][0].Expires = time.Now().Add(-1 * time.Hour)
	store.mu.Unlock()

	// GET /api/security/status (a public route) with the valid session and expired CSRF cookie must refresh it.
	getReq := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	getReq.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	getReq.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: expiredToken})
	getRec := httptest.NewRecorder()

	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected GET status %d, got %d (%s)", http.StatusOK, getRec.Code, getRec.Body.String())
	}

	refreshed := findCSRFCookie(getRec.Result().Cookies())
	if refreshed == nil || refreshed.Value == "" {
		t.Fatalf("expected refreshed pulse_csrf cookie after expired token")
	}
	if refreshed.Value == expiredToken {
		t.Fatalf("expected new pulse_csrf token, got expired token back")
	}
	if refreshed.MaxAge != csrfCookieMaxAgeSeconds {
		t.Fatalf("refreshed pulse_csrf MaxAge = %d, want %d", refreshed.MaxAge, csrfCookieMaxAgeSeconds)
	}

	// Subsequent non-replayed POST with the refreshed cookie must succeed on the first attempt.
	postReq := httptest.NewRequest(http.MethodPost, "/api/secure", nil)
	postReq.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	postReq.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: refreshed.Value})
	postReq.Header.Set("X-CSRF-Token", refreshed.Value)
	postRec := httptest.NewRecorder()

	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected POST status %d with refreshed CSRF token, got %d (%s)", http.StatusOK, postRec.Code, postRec.Body.String())
	}

	// A follow-up GET with the fresh token must not needlessly rotate the cookie.
	freshGetReq := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	freshGetReq.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	freshGetReq.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: refreshed.Value})
	freshGetRec := httptest.NewRecorder()

	router.ServeHTTP(freshGetRec, freshGetReq)

	if c := findCSRFCookie(freshGetRec.Result().Cookies()); c != nil {
		t.Fatalf("did not expect fresh pulse_csrf cookie to rotate, got %q", c.Value)
	}
}

func TestRouterProactivelyRefreshesNearExpiryCSRFCookieOnce(t *testing.T) {
	router, sessionToken := newRouterWithSession(t)

	router.mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	nearExpiryToken := generateCSRFToken(sessionToken)
	store := GetCSRFStore()
	store.mu.Lock()
	store.tokens[csrfSessionKey(sessionToken)][0].Expires = time.Now().Add(10 * time.Minute)
	store.mu.Unlock()

	// First GET sees a valid token with <30m left and rotates once.
	req1 := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req1.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req1.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: nearExpiryToken})
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	rotated := findCSRFCookie(rec1.Result().Cookies())
	if rotated == nil || rotated.Value == "" || rotated.Value == nearExpiryToken {
		t.Fatalf("expected proactive rotation of near-expiry pulse_csrf cookie")
	}

	// A concurrent GET carrying the same near-expiry cookie must coalesce instead of minting a second token.
	req2 := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	req2.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req2.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: nearExpiryToken})
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if duplicate := findCSRFCookie(rec2.Result().Cookies()); duplicate != nil {
		t.Fatalf("expected concurrent near-expiry GET to coalesce, but issued another cookie %q", duplicate.Value)
	}

	// The old near-expiry token must remain valid so in-flight mutations still succeed.
	if !validateCSRFToken(sessionToken, nearExpiryToken) {
		t.Fatalf("expected old near-expiry token to stay valid during proactive rotation")
	}
}

func TestRouterDoesNotRefreshCSRFCookieForInvalidSessionWithStaleCookie(t *testing.T) {
	router, _ := newRouterWithSession(t)

	router.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	bogusSession := "not-a-valid-session-token"
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: bogusSession})
	req.AddCookie(&http.Cookie{Name: CookieNameCSRF, Value: "stale-csrf"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected public GET status %d, got %d", http.StatusOK, rec.Code)
	}
	if c := findCSRFCookie(rec.Result().Cookies()); c != nil {
		t.Fatalf("did not expect pulse_csrf cookie for invalid session, got %q", c.Value)
	}
	if sessions := csrfSessionCountForTest(); sessions != 0 {
		t.Fatalf("invalid session on public GET must not write to CSRFTokenStore, got %d sessions", sessions)
	}
}
