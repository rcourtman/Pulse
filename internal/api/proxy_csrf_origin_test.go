package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const (
	proxyCSRFTestSecret = "synthetic-proxy-secret"
	proxyCSRFTestTarget = "https://pulse.example.test/api/security/tokens"
	proxyCSRFBlocked    = "CSRF origin validation failed"
)

func newProxyCSRFTestRouter(t *testing.T) *Router {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	cfg := &config.Config{
		DataPath: dataDir, ConfigPath: dataDir,
		ProxyAuthSecret: proxyCSRFTestSecret, ProxyAuthUserHeader: "X-Proxy-User",
	}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	return router
}

type proxyCSRFCase struct {
	name    string
	method  string
	target  string
	headers map[string]string
	reached bool
}

// serveProxyCSRFCase sends a request carrying the identity a forward-auth
// proxy attaches to every request with its sign-in cookie. A reached write
// gets the token handler's malformed-body answer, so no token is created.
func serveProxyCSRFCase(t *testing.T, router *Router, tc proxyCSRFCase) {
	t.Helper()
	method := tc.method
	if method == "" {
		method = http.MethodPost
	}
	target := tc.target
	if target == "" {
		target = proxyCSRFTestTarget
	}
	body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Proxy-Secret", proxyCSRFTestSecret)
	req.Header.Set("X-Proxy-User", "proxy-admin")
	for key, value := range tc.headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	blocked := rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), proxyCSRFBlocked)
	if tc.reached {
		if blocked {
			t.Fatalf("legitimate request was blocked: %d %s", rec.Code, rec.Body.String())
		}
		if method == http.MethodPost && (rec.Code != http.StatusBadRequest || body.reads == 0) {
			t.Fatalf("legitimate write did not reach the handler: status %d, reads %d: %s", rec.Code, body.reads, rec.Body.String())
		}
		return
	}
	if !blocked {
		t.Fatalf("other-origin browser request was admitted: %d %s", rec.Code, rec.Body.String())
	}
	if body.reads != 0 {
		t.Fatalf("blocked request body was read %d times", body.reads)
	}
}

func TestProxyAuthBrowserWritesRequireExactOrigin(t *testing.T) {
	router := newProxyCSRFTestRouter(t)

	cases := []proxyCSRFCase{
		{name: "cross-site", headers: map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://attacker.invalid"}},
		{name: "same-site sibling origin", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://sibling.example.test"}},
		{name: "same-site same host other port", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://pulse.example.test:8443"}},
		{name: "same-site same host other scheme", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://pulse.example.test"}},
		{name: "same-site null origin", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "null"}},
		{name: "same-site sibling referer", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Referer": "https://sibling.example.test/page"}},
		{name: "same-site without origin metadata", headers: map[string]string{"Sec-Fetch-Site": "same-site"}},
		{name: "legacy browser foreign origin", headers: map[string]string{"Origin": "https://sibling.example.test"}},
		{name: "legacy browser null origin", headers: map[string]string{"Origin": "null"}},
		{name: "legacy browser foreign referer", headers: map[string]string{"Referer": "https://pulse.example.test:8443/"}},

		{name: "same-site matching origin", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://pulse.example.test"}, reached: true},
		{name: "same-site matching origin with default port", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://pulse.example.test:443"}, reached: true},
		{name: "same-site matching referer", headers: map[string]string{"Sec-Fetch-Site": "same-site", "Referer": "https://pulse.example.test/settings"}, reached: true},
		{name: "same-origin", headers: map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://pulse.example.test"}, reached: true},
		{name: "user-initiated", headers: map[string]string{"Sec-Fetch-Site": "none"}, reached: true},
		{name: "legacy browser matching origin", headers: map[string]string{"Origin": "https://pulse.example.test"}, reached: true},
		{name: "non-browser client without origin metadata", reached: true},
		{name: "same-site read", method: http.MethodGet, headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://sibling.example.test"}, reached: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serveProxyCSRFCase(t, router, tc)
		})
	}
}

func TestProxyAuthOriginComparisonUsesOnlyTrustedForwardedScheme(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "192.0.2.1/32")
	ResetTrustedProxyConfigForTests()
	t.Cleanup(ResetTrustedProxyConfigForTests)
	router := newProxyCSRFTestRouter(t)
	const plainTarget = "http://pulse.example.test/api/security/tokens"

	t.Run("trusted proxy terminated TLS", func(t *testing.T) {
		serveProxyCSRFCase(t, router, proxyCSRFCase{
			target:  plainTarget,
			headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://pulse.example.test", "X-Forwarded-Proto": "https"},
			reached: true,
		})
	})
	t.Run("trusted proxy plain origin is another scheme", func(t *testing.T) {
		serveProxyCSRFCase(t, router, proxyCSRFCase{
			target:  plainTarget,
			headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "http://pulse.example.test", "X-Forwarded-Proto": "https"},
		})
	})

	ResetTrustedProxyConfigForTests()
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "198.51.100.1/32")
	t.Run("untrusted peer cannot claim the secure scheme", func(t *testing.T) {
		serveProxyCSRFCase(t, router, proxyCSRFCase{
			target:  plainTarget,
			headers: map[string]string{"Sec-Fetch-Site": "same-site", "Origin": "https://pulse.example.test", "X-Forwarded-Proto": "https"},
		})
	})
}

// The proxy-auth origin boundary is additional to, and never a substitute
// for, the session CSRF token.
func TestSessionCSRFTokenStillRequiredForSameSiteWrites(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, AuthUser: "synthetic-admin", AuthPass: hash}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	sessionToken := "synthetic-proxy-csrf-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "192.0.2.1", "synthetic-admin")

	send := func(csrf string) *httptest.ResponseRecorder {
		body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
		req := httptest.NewRequest(http.MethodPost, proxyCSRFTestTarget, body)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Origin", "https://pulse.example.test")
		req.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: sessionToken})
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec
	}

	if rec := send(""); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "CSRF token validation failed") {
		t.Fatalf("session write without CSRF token = %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(generateCSRFToken(sessionToken)); rec.Code != http.StatusBadRequest {
		t.Fatalf("session write with CSRF token = %d %s", rec.Code, rec.Body.String())
	}
}
