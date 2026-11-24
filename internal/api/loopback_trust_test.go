package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rs/zerolog"
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

// The relay client serves a remote app's request in process, from a loopback
// peer address and with a loopback Host. That request was not written on the
// Pulse host, so it gets no recovery token and no recovery session, whichever
// API token opened the relay channel.
func TestRelayedRequestGetsNoLoopbackTrust(t *testing.T) {
	router := newRecoveryRouter(t)
	rawToken := "relay-mobile-123.12345678"
	record, err := config.NewAPITokenRecord(rawToken, "phone", []string{config.ScopeRelayMobileAccess})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	router.config.APITokens = []config.APITokenRecord{*record}

	var sawRelayedRequest, trusted bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sawRelayedRequest = true
		trusted = trusted || isDirectLoopbackRequest(req)
		router.Handler().ServeHTTP(w, req)
	})
	proxy := relay.NewHTTPProxyWithLocalHandler("127.0.0.1:7655", handler, zerolog.Nop())

	for _, action := range []string{"generate_token", "disable_auth"} {
		t.Run(action, func(t *testing.T) {
			payload, err := json.Marshal(relay.ProxyRequest{
				ID:      action,
				Method:  http.MethodPost,
				Path:    "/api/security/recovery",
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    base64.StdEncoding.EncodeToString([]byte(`{"action":"` + action + `"}`)),
			})
			if err != nil {
				t.Fatalf("marshal relay request: %v", err)
			}
			raw, err := proxy.HandleRequest(payload, rawToken)
			if err != nil {
				t.Fatalf("HandleRequest: %v", err)
			}
			var resp relay.ProxyResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatalf("unmarshal relay response: %v", err)
			}
			body, _ := base64.StdEncoding.DecodeString(resp.Body)
			if resp.Status != http.StatusForbidden {
				t.Fatalf("status = %d, want %d (%s)", resp.Status, http.StatusForbidden, body)
			}
			if strings.Contains(string(body), `"token"`) {
				t.Fatalf("recovery token issued through the relay: %s", body)
			}
		})
	}
	if !sawRelayedRequest {
		t.Fatal("relayed requests never reached the router")
	}
	if trusted {
		t.Fatal("relayed request passed isDirectLoopbackRequest")
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

// A hosted runtime has no operator on its own host: its users arrive through
// the control plane handoff and it never goes through first-run setup. What
// reaches its loopback port are the requests it sends for its tenants (webhook
// and Apprise tests), so a runtime without local credentials must not serve
// them as the anonymous administrator or hand them recovery.
func TestHostedRuntimeGivesLoopbackNoTrust(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	adminRequests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list API tokens", method: http.MethodGet, path: "/api/security/tokens"},
		{name: "change system settings", method: http.MethodPost, path: "/api/system/settings/update", body: `{"webhookAllowedPrivateCIDRs":"127.0.0.0/8"}`},
		{name: "register an SSO provider", method: http.MethodPost, path: "/api/security/sso/providers", body: `{"id":"x","name":"x","type":"oidc","enabled":true}`},
	}

	t.Run("hosted", func(t *testing.T) {
		router := newConfigTransferTestRouter(t, true, nil)

		if CheckAuth(router.config, httptest.NewRecorder(), newLoopbackTrustRequest(http.MethodGet, "/api/state", "localhost:7655", nil, "")) {
			t.Fatal("hosted runtime admitted a loopback request without credentials")
		}
		for _, tc := range adminRequests {
			req := newLoopbackTrustRequest(tc.method, tc.path, "127.0.0.1:7655", nil, tc.body)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: status = %d, want %d (%s)", tc.name, rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
		}

		for _, action := range []string{"generate_token", "disable_auth"} {
			req := newLoopbackTrustRequest(http.MethodPost, "/api/security/recovery", "127.0.0.1:7655", nil, `{"action":"`+action+`"}`)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("recovery %s: status = %d, want %d (%s)", action, rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"token"`) {
				t.Errorf("recovery %s: token issued: %s", action, rec.Body.String())
			}
			for _, cookie := range rec.Result().Cookies() {
				if cookie.Name == cookieNameSession || cookie.Name == cookieNameSessionSecure {
					t.Errorf("recovery %s: session cookie issued", action)
				}
			}
		}
	})

	t.Run("self-hosted before first-run setup", func(t *testing.T) {
		router := newConfigTransferTestRouter(t, false, nil)

		req := newLoopbackTrustRequest(http.MethodGet, "/api/security/tokens", "127.0.0.1:7655", nil, "")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want the console lane to stay open (%s)", rec.Code, rec.Body.String())
		}
	})
}
