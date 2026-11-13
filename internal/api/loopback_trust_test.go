package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Loopback trust (no-auth anonymous admin, recovery tokens and sessions,
// unauthenticated config transfer) belongs to a client on the Pulse host
// talking straight to Pulse. A loopback peer alone does not prove that: a
// reverse proxy on the same host connects from loopback, and a browser on the
// Pulse host connects from loopback when a page re-resolves its own name to
// 127.0.0.1 or posts across sites to 127.0.0.1.

func newLoopbackTrustRequest(method, target, host string, headers map[string]string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:43210"
	req.Host = host
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req
}

func TestIsDirectLoopbackRequestRequiresLocalHostAndSameOrigin(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	cases := []struct {
		name    string
		host    string
		method  string
		headers map[string]string
		want    bool
	}{
		{name: "localhost with port", host: "localhost:7655", want: true},
		{name: "localhost upper case", host: "LOCALHOST:7655", want: true},
		{name: "localhost subdomain", host: "pulse.localhost:7655", want: true},
		{name: "IPv4 loopback", host: "127.0.0.1:7655", want: true},
		{name: "other IPv4 loopback", host: "127.0.1.1", want: true},
		{name: "IPv6 loopback", host: "[::1]:7655", want: true},
		{name: "same-origin browser", host: "localhost:7655", headers: map[string]string{
			"Sec-Fetch-Site": "same-origin",
			"Origin":         "http://localhost:7655",
		}, method: http.MethodPost, want: true},
		{name: "typed into address bar", host: "127.0.0.1:7655", headers: map[string]string{
			"Sec-Fetch-Site": "none",
		}, want: true},

		{name: "rebound public name", host: "rebind.attacker.test:7655", want: false},
		{name: "proxy forwarding public host", host: "pulse.example.com", want: false},
		{name: "machine hostname", host: "pulse-host:7655", want: false},
		{name: "unspecified address", host: "0.0.0.0:7655", want: false},
		{name: "missing host", host: "", want: false},
		{name: "localhost lookalike", host: "localhost.attacker.test", want: false},
		{name: "cross-site browser", host: "127.0.0.1:7655", headers: map[string]string{
			"Sec-Fetch-Site": "cross-site",
		}, want: false},
		{name: "cross-site browser without fetch metadata", host: "127.0.0.1:7655", method: http.MethodPost, headers: map[string]string{
			"Origin": "http://attacker.test",
		}, want: false},
		{name: "opaque origin", host: "127.0.0.1:7655", method: http.MethodPost, headers: map[string]string{
			"Origin": "null",
		}, want: false},
		{name: "proxy forwarded host header", host: "127.0.0.1:7655", headers: map[string]string{
			"X-Forwarded-Host": "pulse.example.com",
		}, want: false},
		{name: "proxy forwarded proto header", host: "127.0.0.1:7655", headers: map[string]string{
			"X-Forwarded-Proto": "https",
		}, want: false},
		{name: "proxy via header", host: "127.0.0.1:7655", headers: map[string]string{
			"Via": "1.1 proxy",
		}, want: false},
		{name: "empty forwarded port header", host: "localhost:7655", headers: map[string]string{
			"X-Forwarded-Port": "",
		}, want: false},
		{name: "forwarded scheme header", host: "localhost:7655", headers: map[string]string{
			"X-Forwarded-Scheme": "https",
		}, want: false},
		{name: "same-site read from another local port", host: "127.0.0.1:7655", headers: map[string]string{
			"Sec-Fetch-Site": "same-site",
			"Origin":         "http://127.0.0.1:9000",
		}, want: false},
		{name: "same-site read without origin", host: "127.0.0.1:7655", headers: map[string]string{
			"Sec-Fetch-Site": "same-site",
		}, want: false},
		{name: "websocket from another local port without fetch metadata", host: "127.0.0.1:7655", headers: map[string]string{
			"Origin":  "http://127.0.0.1:9000",
			"Upgrade": "websocket",
		}, want: false},
		{name: "read from another local page by referer", host: "localhost:7655", headers: map[string]string{
			"Referer": "http://localhost:9000/page",
		}, want: false},
		{name: "same-origin read without fetch metadata", host: "localhost:7655", headers: map[string]string{
			"Origin": "http://localhost:7655",
		}, want: true},
		{name: "trailing-dot localhost", host: "localhost.:7655", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req := newLoopbackTrustRequest(method, "/api/security/recovery", tc.host, tc.headers, "")
			if got := isDirectLoopbackRequest(req); got != tc.want {
				t.Fatalf("isDirectLoopbackRequest(Host=%q, headers=%v) = %v, want %v", tc.host, tc.headers, got, tc.want)
			}
		})
	}
}

func TestRecoveryTokenRequiresLocalHostAndSameOrigin(t *testing.T) {
	router := newRecoveryRouter(t)
	body := `{"action":"generate_token"}`

	refused := []struct {
		name    string
		host    string
		headers map[string]string
	}{
		{name: "rebound public name", host: "rebind.attacker.test:7655"},
		{name: "local proxy without forwarded-for", host: "pulse.example.com"},
		{name: "cross-site page", host: "127.0.0.1:7655", headers: map[string]string{
			"Sec-Fetch-Site": "cross-site",
			"Origin":         "https://attacker.test",
		}},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			req := newLoopbackTrustRequest(http.MethodPost, "/api/security/recovery", tc.host, tc.headers, body)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"token"`) {
				t.Fatalf("recovery token issued: %s", rec.Body.String())
			}
		})
	}

	allowed := []struct {
		name    string
		host    string
		headers map[string]string
	}{
		{name: "local shell", host: "127.0.0.1:7655"},
		{name: "local browser", host: "localhost:7655", headers: map[string]string{
			"Sec-Fetch-Site": "same-origin",
			"Origin":         "http://localhost:7655",
		}},
	}
	for _, tc := range allowed {
		t.Run("allows "+tc.name, func(t *testing.T) {
			req := newLoopbackTrustRequest(http.MethodPost, "/api/security/recovery", tc.host, tc.headers, body)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"token"`) {
				t.Fatalf("status = %d, want token (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRecoverySessionRefusesRebindingHost(t *testing.T) {
	router := newRecoveryRouter(t)

	req := newLoopbackTrustRequest(http.MethodPost, "/api/security/recovery", "rebind.attacker.test:7655", map[string]string{
		"Sec-Fetch-Site": "same-origin",
		"Origin":         "http://rebind.attacker.test:7655",
	}, `{"action":"disable_auth"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (%s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSession || cookie.Name == cookieNameSessionSecure {
			t.Fatalf("recovery session cookie issued to a rebound host")
		}
	}
}

func TestNoAuthLoopbackAccessRequiresLocalHostAndSameOrigin(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()
	cfg := &config.Config{}

	local := newLoopbackTrustRequest(http.MethodGet, "/api/state", "localhost:7655", nil, "")
	if !CheckAuth(cfg, httptest.NewRecorder(), local) {
		t.Fatal("expected direct local access to stay open before auth is configured")
	}

	for name, req := range map[string]*http.Request{
		"rebound public name": newLoopbackTrustRequest(http.MethodGet, "/api/state", "rebind.attacker.test:7655", nil, ""),
		"local proxy":         newLoopbackTrustRequest(http.MethodGet, "/api/state", "pulse.example.com", nil, ""),
		"cross-site page": newLoopbackTrustRequest(http.MethodPost, "/api/state", "127.0.0.1:7655", map[string]string{
			"Sec-Fetch-Site": "cross-site",
		}, `{}`),
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if CheckAuth(cfg, rec, req) {
				t.Fatal("expected anonymous loopback access to be refused")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}
