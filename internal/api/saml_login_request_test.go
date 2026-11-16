package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type samlLoginCountingBody struct {
	io.Reader
	bytes int
}

func (b *samlLoginCountingBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.bytes += n
	return n, err
}

func (*samlLoginCountingBody) Close() error { return nil }

type samlLoginFailedBody struct{}

func (samlLoginFailedBody) Read([]byte) (int, error) { return 0, errors.New("synthetic body failure") }
func (samlLoginFailedBody) Close() error             { return nil }

func samlLoginRequests(service *SAMLService) map[string]samlRequestRecord {
	if service == nil {
		return nil
	}
	service.authnRequests.mu.Lock()
	defer service.authnRequests.mu.Unlock()
	entries := make(map[string]samlRequestRecord, len(service.authnRequests.entries))
	for id, entry := range service.authnRequests.entries {
		entries[id] = entry
	}
	return entries
}

func assertSAMLLoginRequest(t *testing.T, rec *httptest.ResponseRecorder, service *SAMLService, wantReturnTo string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("login POST: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		AuthorizationURL string `json:"authorizationUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(response.AuthorizationURL)
	if err != nil || target.Scheme != "https" || target.Host != "idp.example.com" || target.Query().Get("SAMLRequest") == "" {
		t.Fatalf("invalid native authorization target: %q (%v)", response.AuthorizationURL, err)
	}
	returnTo, err := url.QueryUnescape(target.Query().Get("RelayState"))
	if err != nil || returnTo != wantReturnTo {
		t.Errorf("RelayState = %q (%v), want %q", returnTo, err, wantReturnTo)
	}
	var loginToken string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == cookieNameSAMLLoginSecure && cookie.Secure && cookie.HttpOnly && cookie.SameSite == http.SameSiteLaxMode && cookie.Path == "/" && cookie.MaxAge == int(samlRequestTTL/time.Second) {
			loginToken = cookie.Value
		}
	}
	if loginToken == "" {
		t.Fatal("login did not preserve the secure browser-binding cookie")
	}
	matched := 0
	for _, entry := range samlLoginRequests(service) {
		if entry.bindingKey == sessionHash(loginToken) && entry.returnTo == wantReturnTo && time.Now().Before(entry.expiresAt) {
			matched++
		}
	}
	if matched != 1 {
		t.Errorf("got %d matching browser-bound requests, want one", matched)
	}
}

func TestSAMLLoginRequestAcceptedInput(t *testing.T) {
	for _, tc := range []struct{ name, query, body, want string }{
		{"empty", "", "", "/"},
		{"whitespace", "", " \n\t", "/"},
		{"null", "", "null", "/"},
		{"empty-object", "", "{}", "/"},
		{"unknown-field", "", `{"extra":"compatible","returnTo":"/alerts?range=7d&tab=history#recent"}`, "/alerts?range=7d&tab=history#recent"},
		{"query-priority", "?returnTo=/infrastructure", `{"returnTo":"/alerts"}`, "/infrastructure"},
		{"unsafe-query-local-body", "?returnTo=https://outside.example/", `{"returnTo":"/alerts"}`, "/alerts"},
		{"unsafe-body", "", `{"returnTo":"//outside.example/"}`, "/"},
		{"exact-limit", "", "{}" + strings.Repeat(" ", int(maxSecurityRequestBodyBytes)-2), "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSAMLRouter(t, testSAMLProvider("okta", true))
			req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login"+tc.query, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			r.handleSAMLLogin(rec, req)
			assertSAMLLoginRequest(t, rec, r.samlManager.GetService("okta"), tc.want)
		})
	}
}

func TestSAMLLoginRequestPreservesPendingAndGET(t *testing.T) {
	r := newSAMLRouter(t, testSAMLProvider("okta", true))
	req := httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login?returnTo=/infrastructure", nil)
	// GET remains a navigation handoff and must not start reading a body.
	req.Body = samlLoginFailedBody{}
	first := httptest.NewRecorder()
	r.handleSAMLLogin(first, req)
	if first.Code != http.StatusFound {
		t.Fatalf("GET handoff: %d %s", first.Code, first.Body.String())
	}
	service := r.samlManager.GetService("okta")
	before := samlLoginRequests(service)
	if len(before) != 1 {
		t.Fatalf("GET created %d pending requests, want one", len(before))
	}
	req = httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login", strings.NewReader(`{"returnTo":"/alerts"}`))
	for _, cookie := range first.Result().Cookies() {
		req.AddCookie(cookie)
	}
	second := httptest.NewRecorder()
	r.handleSAMLLogin(second, req)
	assertSAMLLoginRequest(t, second, service, "/alerts")
	after := samlLoginRequests(service)
	if r.samlManager.GetService("okta") != service || len(after) != 2 {
		t.Fatal("valid POST replaced the service or lost a pending login")
	}
	for id, entry := range before {
		if !reflect.DeepEqual(after[id], entry) {
			t.Error("valid POST changed the existing login's browser, path or expiry")
		}
	}
}

func TestSAMLLoginRequestRegisteredRoute(t *testing.T) {
	r := newSAMLRouter(t, testSAMLProvider("okta", true))
	r.config.DataPath = t.TempDir()
	r.mux = http.NewServeMux()
	r.licenseHandlers = NewLicenseHandlers(config.NewMultiTenantPersistence(r.config.DataPath), false, r.config)
	r.registerAuthSecurityInstallRoutes()
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"bounded", `{"returnTo":"/alerts"}`, 200},
		{"malformed-query-body", "{", 400},
		{"oversized-query-body", strings.Repeat(" ", int(maxSecurityRequestBodyBytes)+1), 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login?returnTo=/infrastructure", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			r.mux.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("registered public route: %d %s, want %d", rec.Code, rec.Body.String(), tc.status)
			}
		})
	}
}

func TestSAMLLoginRequestRejectsInvalidInputBeforeProviderWork(t *testing.T) {
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name, query, body string
				status            int
				failedRead        bool
			}{
				{name: "oversized-value", body: `{"returnTo":"/` + strings.Repeat("a", int(maxSecurityRequestBodyBytes)) + `"}`, status: 413},
				{name: "oversized-tail", body: "{}" + strings.Repeat(" ", int(maxSecurityRequestBodyBytes)), status: 413},
				{name: "oversized-whitespace", body: strings.Repeat(" ", int(maxSecurityRequestBodyBytes)+1), status: 413},
				{name: "query-oversized-body", query: "?returnTo=/alerts", body: `{"extra":"` + strings.Repeat("a", int(maxSecurityRequestBodyBytes)) + `"}`, status: 413},
				{name: "trailing-object", body: `{} {"returnTo":"/alerts"}`, status: 400},
				{name: "trailing-null", body: "{} null", status: 400},
				{name: "trailing-garbage", body: "{} invalid", status: 400},
				{name: "malformed", body: "{", status: 400},
				{name: "query-malformed-body", query: "?returnTo=/alerts", body: "{", status: 400},
				{name: "array", body: "[]", status: 400},
				{name: "wrong-field-type", body: `{"returnTo":true}`, status: 400},
				{name: "reader-failure", failedRead: true, status: 400},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var metadataReads atomic.Int32
					idp := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						metadataReads.Add(1)
						w.Header().Set("Content-Type", "application/xml")
						io.WriteString(w, `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com/metadata"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/></IDPSSODescriptor></EntityDescriptor>`)
					}))
					t.Cleanup(idp.Close)
					provider := testSAMLProvider("okta", true)
					provider.SAML.IDPMetadataURL = idp.URL
					r := newSAMLRouter(t, provider)
					// A rejected POST must not even rebind existing SP URLs.
					r.samlManager = NewSAMLServiceManager("https://previous.example.com")
					if warm {
						if err := r.samlManager.InitializeProvider(t.Context(), provider.ID, provider.SAML); err != nil {
							t.Fatal(err)
						}
						s := r.samlManager.GetService(provider.ID)
						if _, err := s.MakeAuthRequest("/alerts", "existing-browser"); err != nil {
							t.Fatal(err)
						}
					}
					beforeService := r.samlManager.GetService(provider.ID)
					beforeRequests := samlLoginRequests(beforeService)
					beforeReads := metadataReads.Load()
					body := &samlLoginCountingBody{Reader: strings.NewReader(tc.body)}
					req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login"+tc.query, body)
					if tc.failedRead {
						req.Body = samlLoginFailedBody{}
					}
					rec := httptest.NewRecorder()
					r.handleSAMLLogin(rec, req)
					if rec.Code != tc.status || !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
						t.Errorf("status/body = %d %s, want %d invalid_request", rec.Code, rec.Body.String(), tc.status)
					}
					if body.bytes > int(maxSecurityRequestBodyBytes)+1 {
						t.Errorf("read %d bytes, exceeds bound plus detection byte", body.bytes)
					}
					if metadataReads.Load() != beforeReads || r.samlManager.GetPublicURL() != "https://previous.example.com" {
						t.Error("invalid input caused metadata work or SP URL rebinding")
					}
					current := r.samlManager.GetService(provider.ID)
					if current != beforeService || !reflect.DeepEqual(beforeRequests, samlLoginRequests(current)) {
						t.Error("invalid input created/replaced a service or changed pending login requests")
					}
					if len(rec.Result().Cookies()) != 0 || rec.Header().Get("Location") != "" {
						t.Error("invalid input wrote a cookie or redirect")
					}
				})
			}
		})
	}
}

type samlLoginHeldBody struct {
	io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (b *samlLoginHeldBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.Reader.Read(p)
}
func (*samlLoginHeldBody) Close() error { return nil }

func awaitSAMLLoginBody(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(10 * time.Second):
		t.Fatal("SAML login did not reach its bounded body barrier")
	}
}

func TestSAMLLoginRequestUsesProviderAfterBody(t *testing.T) {
	for _, warm := range []bool{false, true} {
		mode := "cold"
		if warm {
			mode = "warm"
		}
		t.Run(mode, func(t *testing.T) {
			for _, action := range []string{"unchanged", "disable", "remove", "switch-protocol", "replace"} {
				t.Run(action, func(t *testing.T) {
					provider := testSAMLProvider("okta", true)
					r := newSAMLRouter(t, provider)
					if warm {
						if err := r.samlManager.InitializeProvider(t.Context(), provider.ID, provider.SAML); err != nil {
							t.Fatal(err)
						}
					}
					original := r.samlManager.GetService(provider.ID)
					body := &samlLoginHeldBody{Reader: strings.NewReader(`{"returnTo":"/alerts"}`), entered: make(chan struct{}), release: make(chan struct{})}
					var once sync.Once
					unblock := func() { once.Do(func() { close(body.release) }) }
					t.Cleanup(unblock)
					rec, done := httptest.NewRecorder(), make(chan struct{})
					req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login", body)
					go func() { defer close(done); r.handleSAMLLogin(rec, req) }()
					awaitSAMLLoginBody(t, body.entered)
					updated := (&config.SSOConfig{Providers: []config.SSOProvider{provider}}).Clone().Providers[0]
					switch action {
					case "disable":
						updated.Enabled = false
					case "switch-protocol":
						updated.Type, updated.SAML = config.SSOProviderTypeOIDC, nil
						updated.OIDC = &config.OIDCProviderConfig{IssuerURL: "https://idp.example.test", ClientID: "synthetic"}
					case "replace":
						updated.SAML.IDPSSOURL = "https://replacement.example.com/sso"
					}
					if action != "unchanged" {
						apply, save := r.commitSSOConfig(func(cfg *config.SSOConfig) error {
							if action == "remove" {
								return cfg.RemoveProvider(provider.ID)
							}
							return cfg.UpdateProvider(updated)
						})
						if apply != nil || save != nil {
							t.Fatalf("snapshot publication failed: %v %v", apply, save)
						}
						// Complete the same service lifecycle as management before
						// releasing input. No configuration lock spans the body read.
						if action == "replace" {
							if err := r.samlManager.InitializeProvider(t.Context(), provider.ID, updated.SAML); err != nil {
								t.Fatal(err)
							}
						} else {
							r.samlManager.RemoveProvider(provider.ID)
						}
					}
					unblock()
					awaitSAMLLoginBody(t, done)
					switch action {
					case "unchanged":
						assertSAMLLoginRequest(t, rec, r.samlManager.GetService(provider.ID), "/alerts")
						if warm && r.samlManager.GetService(provider.ID) != original {
							t.Error("unchanged POST replaced the existing service")
						}
					case "replace":
						if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://replacement.example.com/sso") {
							t.Errorf("POST did not use the completed replacement: %d %s", rec.Code, rec.Body.String())
						}
					default:
						if rec.Code != http.StatusNotFound || len(rec.Result().Cookies()) != 0 || r.samlManager.GetService(provider.ID) != nil {
							t.Errorf("POST used withdrawn provider: %d %s", rec.Code, rec.Body.String())
						}
					}
					if action != "unchanged" && len(samlLoginRequests(original)) != 0 {
						t.Error("POST issued a challenge on the withdrawn service")
					}
				})
			}
		})
	}
}
