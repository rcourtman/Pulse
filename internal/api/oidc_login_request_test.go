package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type oidcLoginObservedBody struct {
	reader io.Reader
	bytes  int
	reads  int
}

func (b *oidcLoginObservedBody) Read(p []byte) (int, error) {
	b.reads++
	n, err := b.reader.Read(p)
	b.bytes += n
	return n, err
}

type oidcLoginFailedReader struct{}

func (oidcLoginFailedReader) Read([]byte) (int, error) {
	return 0, errors.New("synthetic-private-reader-error")
}

// Exercise the registered public route with real lazy discovery against a
// guest-local IdP. No production credentials, network, sessions or grants.
func newOIDCLoginRequestFixture(t *testing.T, warm bool) (*Router, *atomic.Int32, string) {
	t.Helper()
	var discoveries atomic.Int32
	idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		discoveries.Add(1)
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": base, "authorization_endpoint": base + "/authorize",
			"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
		})
	}))
	t.Cleanup(idp.Close)
	provider := config.SSOProvider{
		ID: "bounded-login", Type: config.SSOProviderTypeOIDC, Enabled: true,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL: idp.URL, ClientID: "synthetic-bounded-client",
			RedirectURL: "https://pulse.example.test/api/oidc/bounded-login/callback",
		},
	}
	r := &Router{
		config:      &config.Config{PublicURL: "https://pulse.example.test"},
		ssoConfig:   &config.SSOConfig{Providers: []config.SSOProvider{provider}},
		oidcManager: NewOIDCServiceManager(), mux: http.NewServeMux(),
	}
	r.registerAuthSecurityInstallRoutes()
	t.Cleanup(func() { r.oidcManager.RemoveService(provider.ID) })
	pending := ""
	if warm {
		if err := r.oidcManager.EnsureProvider(t.Context(), provider.ID, &provider, provider.OIDC.RedirectURL); err != nil {
			t.Fatal(err)
		}
		var err error
		pending, _, err = r.oidcManager.GetService(provider.ID).newStateEntry(provider.ID, "/existing-login")
		if err != nil {
			t.Fatal(err)
		}
	}
	return r, &discoveries, pending
}

func oidcLoginRequest(t *testing.T, r *Router, method, path string, body io.Reader) (*httptest.ResponseRecorder, *oidcLoginObservedBody) {
	t.Helper()
	observed := &oidcLoginObservedBody{reader: body}
	req := httptest.NewRequest(method, path, observed)
	req.RemoteAddr = "192.0.2.86:1234"
	// Do not rely on a trustworthy Content-Length (chunked clients have none).
	req.ContentLength = -1
	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, req)
	return w, observed
}

func TestSSOOIDCLoginRequestRejected(t *testing.T) {
	const valid = `{"returnTo":"/alerts"}`
	limit := int(maxSecurityRequestBodyBytes)
	tests := []struct {
		name   string
		body   string
		reader func() io.Reader
		status int
	}{
		{name: "oversized-return-path", body: `{"returnTo":"/` + strings.Repeat("a", limit*4) + `"}`, status: http.StatusRequestEntityTooLarge},
		{name: "oversized-unknown-field", body: `{"returnTo":"/alerts","ignored":"` + strings.Repeat("a", limit*4) + `"}`, status: http.StatusRequestEntityTooLarge},
		{name: "trailing-whitespace", body: valid + strings.Repeat(" ", limit), status: http.StatusRequestEntityTooLarge},
		{name: "oversized-empty-body", body: strings.Repeat(" ", limit+1), status: http.StatusRequestEntityTooLarge},
		{name: "one-byte-over-limit", body: valid + strings.Repeat(" ", limit+1-len(valid)), status: http.StatusRequestEntityTooLarge},
		{name: "concatenated-object", body: valid + `{"returnTo":"/settings"}`, status: http.StatusBadRequest},
		{name: "trailing-null", body: valid + `null`, status: http.StatusBadRequest},
		{name: "trailing-garbage", body: valid + `not-json`, status: http.StatusBadRequest},
		{name: "malformed", body: `{"returnTo":`, status: http.StatusBadRequest},
		{name: "wrong-type", body: `{"returnTo":42}`, status: http.StatusBadRequest},
		{name: "read-error", reader: func() io.Reader { return io.MultiReader(strings.NewReader("{"), oidcLoginFailedReader{}) }, status: http.StatusBadRequest},
	}
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					r, discoveries, pending := newOIDCLoginRequestFixture(t, warm)
					previous := r.oidcManager.GetService("bounded-login")
					before := discoveries.Load()
					var reader io.Reader = strings.NewReader(tc.body)
					if tc.reader != nil {
						reader = tc.reader()
					}
					w, read := oidcLoginRequest(t, r, http.MethodPost, "/api/oidc/bounded-login/login", reader)
					if w.Code != tc.status {
						t.Errorf("status = %d, want %d", w.Code, tc.status)
					}
					var response struct {
						Code    string `json:"code"`
						Message string `json:"error"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Code != "invalid_request" || response.Message != "Invalid request payload" {
						t.Errorf("expected fixed invalid_request response, got %s (decode %v)", w.Body.String(), err)
					}
					if strings.Contains(w.Body.String(), "synthetic-private-reader-error") || w.Header().Get("Location") != "" || len(w.Result().Cookies()) != 0 {
						t.Error("rejected input leaked reader text, redirected or issued cookies")
					}
					if read.bytes > limit+1 {
						t.Errorf("read %d bytes, bound is %d", read.bytes, limit+1)
					}
					if discoveries.Load() != before || r.oidcManager.GetService("bounded-login") != previous {
						t.Error("rejected input discovered or replaced an OIDC service")
					}
					if service := r.oidcManager.GetService("bounded-login"); service != nil {
						service.stateStore.mu.RLock()
						count := len(service.stateStore.entries)
						service.stateStore.mu.RUnlock()
						if count != 1 {
							t.Errorf("pending state count = %d, want the one existing login only", count)
						}
						if entry, ok := service.peekState(pending); !ok || entry.ReturnTo != "/existing-login" {
							t.Error("rejected input removed or changed an existing login")
						}
					}
				})
			}
		})
	}
}

func TestSSOOIDCLoginRequestCompatibility(t *testing.T) {
	const exactStart = `{"returnTo":"/alerts","ignored":"`
	const exactEnd = `"}`
	exact := exactStart + strings.Repeat("a", int(maxSecurityRequestBodyBytes)-len(exactStart)-len(exactEnd)) + exactEnd
	tests := []struct{ name, method, body, query, returnTo string }{
		{name: "empty", method: http.MethodPost},
		{name: "whitespace", method: http.MethodPost, body: " \t\r\n "},
		{name: "empty-object", method: http.MethodPost, body: `{}`},
		{name: "null", method: http.MethodPost, body: `null`},
		{name: "local-path", method: http.MethodPost, body: `{"returnTo":" /alerts?state=active#recent "}`, returnTo: "/alerts?state=active#recent"},
		{name: "unknown-field", method: http.MethodPost, body: `{"returnTo":"/alerts","ignored":true}`, returnTo: "/alerts"},
		{name: "exact-limit", method: http.MethodPost, body: exact, returnTo: "/alerts"},
		{name: "exact-limit-whitespace", method: http.MethodPost, body: `{}` + strings.Repeat(" ", int(maxSecurityRequestBodyBytes)-2)},
		{name: "external-return", method: http.MethodPost, body: `{"returnTo":"https://outside.example.test/"}`},
		{name: "network-path-return", method: http.MethodPost, body: `{"returnTo":"//outside.example.test/"}`},
		{name: "get", method: http.MethodGet, query: "?returnTo=" + url.QueryEscape("/alerts?state=active#recent"), returnTo: "/alerts?state=active#recent"},
		{name: "get-ignores-body", method: http.MethodGet, body: strings.Repeat("a", int(maxSecurityRequestBodyBytes)*4)},
	}
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					r, discoveries, pending := newOIDCLoginRequestFixture(t, warm)
					w, read := oidcLoginRequest(t, r, tc.method, "/api/oidc/bounded-login/login"+tc.query, strings.NewReader(tc.body))
					if discoveries.Load() != 1 {
						t.Errorf("discovery requests = %d, want one per client lifecycle", discoveries.Load())
					}
					authorizationURL := w.Header().Get("Location")
					if tc.method == http.MethodPost {
						var payload struct {
							URL string `json:"authorizationUrl"`
						}
						if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &payload) != nil {
							t.Fatalf("POST failed: %d %s", w.Code, w.Body.String())
						}
						authorizationURL = payload.URL
					} else if w.Code != http.StatusFound || read.reads != 0 {
						t.Fatalf("GET status = %d, body read = %d", w.Code, read.reads)
					}
					u, err := url.Parse(authorizationURL)
					if err != nil {
						t.Fatal(err)
					}
					q := u.Query()
					service := r.oidcManager.GetService("bounded-login")
					entry, ok := service.consumeState(q.Get("state"))
					if !ok || entry.ReturnTo != tc.returnTo || entry.ProviderID != "bounded-login" {
						t.Fatalf("incorrect pending login: %+v, exists %v", entry, ok)
					}
					if q.Get("client_id") != "synthetic-bounded-client" || q.Get("redirect_uri") != "https://pulse.example.test/api/oidc/bounded-login/callback" || q.Get("response_type") != "code" || q.Get("nonce") == "" || q.Get("nonce") != entry.Nonce || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("code_challenge") != entry.CodeChallenge || entry.CodeVerifier == "" {
						t.Error("login lost client, callback, nonce or PKCE binding")
					}
					if _, replay := service.consumeState(q.Get("state")); replay {
						t.Error("pending login was not single-use")
					}
					if warm {
						if _, ok := service.peekState(pending); !ok {
							t.Error("new login discarded the existing pending login")
						}
					}
				})
			}
		})
	}
}

func TestSSOOIDCLoginRequestChecksProviderBeforeBody(t *testing.T) {
	r, discoveries, _ := newOIDCLoginRequestFixture(t, false)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPut, "/api/oidc/bounded-login/login", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/oidc/missing/login", http.StatusNotFound},
		{http.MethodPost, "/api/oidc/invalid!id/login", http.StatusBadRequest},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w, read := oidcLoginRequest(t, r, tc.method, tc.path, oidcLoginFailedReader{})
			if w.Code != tc.status || read.reads != 0 || discoveries.Load() != 0 {
				t.Fatalf("provider/method admission status %d, read %d, discoveries %d", w.Code, read.reads, discoveries.Load())
			}
		})
	}
}

func TestSSOOIDCLoginRequestLegacyAlias(t *testing.T) {
	r, discoveries, _ := newOIDCLoginRequestFixture(t, false)
	r.ssoConfig.Providers[0].ID = config.LegacyOIDCProviderID
	t.Cleanup(func() { r.oidcManager.RemoveService(config.LegacyOIDCProviderID) })
	w, _ := oidcLoginRequest(t, r, http.MethodPost, "/api/oidc/login", strings.NewReader(`{} {}`))
	if w.Code != http.StatusBadRequest || discoveries.Load() != 0 {
		t.Fatalf("legacy alias bypassed admission: status %d, discoveries %d", w.Code, discoveries.Load())
	}
	w, _ = oidcLoginRequest(t, r, http.MethodPost, "/api/oidc/login", strings.NewReader(`{}`))
	if w.Code != http.StatusOK || discoveries.Load() != 1 {
		t.Fatalf("valid legacy alias failed: status %d, discoveries %d", w.Code, discoveries.Load())
	}
}
