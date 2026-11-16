package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const samlInitializationMetadata = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com/metadata"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/></IDPSSODescriptor></EntityDescriptor>`

// Only the first HTTP metadata read waits. Management and another browser must
// finish while it is held, without timing sleeps or a production test hook.
func heldSAMLInitializationMetadata(t *testing.T, metadata string) (string, <-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			select {
			case <-release:
			case <-req.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, metadata)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(unblock)
	return server.URL, entered, unblock
}

func TestSAMLInitializationManagementOwnership(t *testing.T) {
	for _, warm := range []bool{false, true} {
		for _, action := range []string{"remove", "replace", "same-settings-reset", "failed-reset", "remove-reenable", "other-provider"} {
			name := "cold/"
			if warm {
				name = "warm/"
			}
			t.Run(name+action, func(t *testing.T) {
				metadataURL, entered, unblock := heldSAMLInitializationMetadata(t, samlInitializationMetadata)
				p := testSAMLProvider("ownership", true)
				manager := NewSAMLServiceManager("https://pulse.example.com")
				if warm {
					if err := manager.InitializeProvider(t.Context(), p.ID, p.SAML); err != nil {
						t.Fatal(err)
					}
				}
				remote := *p.SAML
				remote.IDPMetadataURL = metadataURL
				done := make(chan struct{})
				var staleErr error
				go func() { defer close(done); staleErr = manager.InitializeProvider(t.Context(), p.ID, &remote) }()
				awaitOIDCInitialization(t, entered)
				switch action {
				case "remove":
					manager.RemoveProvider(p.ID)
				case "replace":
					changed := *p.SAML
					changed.UsernameAttr = "current-name"
					if err := manager.InitializeProvider(t.Context(), p.ID, &changed); err != nil {
						t.Fatal(err)
					}
				case "same-settings-reset":
					if err := manager.InitializeProvider(t.Context(), p.ID, &remote); err != nil {
						t.Fatal(err)
					}
				case "failed-reset":
					invalid := config.SAMLProviderConfig{IDPMetadataXML: "<invalid"}
					if err := manager.InitializeProvider(t.Context(), p.ID, &invalid); err == nil {
						t.Fatal("invalid replacement succeeded")
					}
				case "remove-reenable":
					manager.RemoveProvider(p.ID)
					if err := manager.InitializeProvider(t.Context(), p.ID, &remote); err != nil {
						t.Fatal(err)
					}
				case "other-provider":
					manager.RemoveProvider("independent")
					if err := manager.InitializeProvider(t.Context(), "independent", p.SAML); err != nil {
						t.Fatal(err)
					}
				}
				current := manager.GetService(p.ID)
				var pending map[string]samlRequestRecord
				if current != nil {
					if _, err := current.makeAuthRequest(httptest.NewRequest("GET", "https://pulse.example.com/", nil), "/alerts", "synthetic-current-browser"); err != nil {
						t.Fatal(err)
					}
					pending = samlLoginRequests(current)
				}
				unblock()
				awaitOIDCInitialization(t, done)
				if action == "other-provider" {
					if staleErr != nil || manager.GetService(p.ID) == nil {
						t.Fatalf("independent provider invalidated valid initialization: %v", staleErr)
					}
					return
				}
				if staleErr == nil {
					t.Error("superseded metadata completion reported success")
				}
				if manager.GetService(p.ID) != current {
					t.Fatal("superseded completion replaced current service")
				}
				if current != nil && !reflect.DeepEqual(samlLoginRequests(current), pending) {
					t.Error("superseded completion changed current pending login")
				}
			})
		}
	}
}

func TestSAMLInitializationPublicURLChanged(t *testing.T) {
	metadataURL, entered, unblock := heldSAMLInitializationMetadata(t, samlInitializationMetadata)
	manager := NewSAMLServiceManager("https://before.example.com")
	p := testSAMLProvider("ownership", true)
	p.SAML.IDPMetadataURL = metadataURL
	done := make(chan struct{})
	var staleErr error
	go func() { defer close(done); staleErr = manager.InitializeProvider(t.Context(), p.ID, p.SAML) }()
	awaitOIDCInitialization(t, entered)
	if err := manager.SetPublicURL("https://current.example.com/pulse/"); err != nil {
		t.Fatal(err)
	}
	unblock()
	awaitOIDCInitialization(t, done)
	if staleErr == nil || manager.GetService(p.ID) != nil {
		t.Error("initialization published the old public base")
	}
	if err := manager.InitializeProvider(t.Context(), p.ID, p.SAML); err != nil {
		t.Fatal(err)
	}
	metadata, err := manager.GetService(p.ID).GetMetadata()
	if err != nil || !strings.Contains(string(metadata), `Location="https://current.example.com/pulse/api/saml/ownership/acs"`) {
		t.Fatalf("fresh initialization did not use current public base: %s %v", metadata, err)
	}
}

type samlInitializationHandoverResult struct {
	ManagementStatus       int    `json:"managementStatus"`
	StaleLoginStatus       int    `json:"staleLoginStatus"`
	StaleCookieWrites      int    `json:"staleCookieWrites"`
	CurrentLoginStatus     int    `json:"currentLoginStatus"`
	CurrentServiceRetained bool   `json:"currentServiceRetained"`
	CallbackTarget         string `json:"callbackTarget"`
	ReplayTarget           string `json:"replayTarget"`
	SessionVerified        bool   `json:"sessionVerified"`
}

// The signed current browser completes after older native metadata is released.
// This proves actual ACS/session/replay behavior, not just map pointer identity.
func runSAMLInitializationManagementHandover(t *testing.T) samlInitializationHandoverResult {
	t.Helper()
	idp := newSAMLManualCertTestIdP(t)
	metadataURL, entered, unblock := heldSAMLInitializationMetadata(t, idp.metadataXML())
	useSAMLTestAuthStores(t)
	r, _ := setupTestRouter(t)
	r.config.PublicURL = "https://pulse.example.com"
	p := testSAMLProvider("okta", true)
	p.SAML.IDPMetadataURL = metadataURL
	if err := r.ssoConfig.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := r.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	stale, done := httptest.NewRecorder(), make(chan struct{})
	go func() {
		defer close(done)
		r.handleSAMLLogin(stale, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login?returnTo=/alerts", nil))
	}()
	awaitOIDCInitialization(t, entered)
	updated := (&config.SSOConfig{Providers: []config.SSOProvider{p}}).Clone().Providers[0]
	updated.SAML.UsernameAttr = "current-name"
	payload, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	management := httptest.NewRecorder()
	r.handleSSOProvider(management, httptest.NewRequest("PUT", "/api/security/sso/providers/okta", bytes.NewReader(payload)))
	if management.Code != 200 {
		t.Fatalf("management save: %d %s", management.Code, management.Body.String())
	}
	current := httptest.NewRecorder()
	r.handleSAMLLogin(current, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login?returnTo=/alerts", nil))
	if current.Code != 302 {
		t.Fatalf("current login: %d %s", current.Code, current.Body.String())
	}
	service := r.samlManager.GetService(p.ID)
	requestID := samlTestAuthnRequestID(t, current.Header().Get("Location"))
	response := idp.signedResponseXML(t, service, requestID, "synthetic-current-subject", nil)
	unblock()
	awaitOIDCInitialization(t, done)
	result := samlInitializationHandoverResult{ManagementStatus: management.Code, StaleLoginStatus: stale.Code, StaleCookieWrites: len(stale.Result().Cookies()), CurrentLoginStatus: current.Code, CurrentServiceRetained: r.samlManager.GetService(p.ID) == service}
	form := url.Values{"SAMLResponse": {base64.StdEncoding.EncodeToString(response)}, "RelayState": {"/settings"}}
	callbackRequest := func() *http.Request {
		req := httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/acs", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range current.Result().Cookies() {
			req.AddCookie(cookie)
		}
		return req
	}
	callback := httptest.NewRecorder()
	r.handleSAMLACS(callback, callbackRequest())
	result.CallbackTarget = callback.Header().Get("Location")
	principal, err := stableSSOPrincipal(config.SSOProviderTypeSAML, p.ID, "synthetic-current-subject")
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range callback.Result().Cookies() {
		if cookie.Name == sessionCookieName(true) && cookie.Value != "" {
			session := GetSessionStore().GetSession(cookie.Value)
			result.SessionVerified = cookie.Secure && cookie.HttpOnly && session != nil && session.Username == principal && GetSessionStore().ValidateSession(cookie.Value)
		}
	}
	replay := httptest.NewRecorder()
	r.handleSAMLACS(replay, callbackRequest())
	result.ReplayTarget = replay.Header().Get("Location")
	return result
}

func TestSAMLInitializationManagementHandover(t *testing.T) {
	got := runSAMLInitializationManagementHandover(t)
	if got.ManagementStatus != 200 || got.StaleLoginStatus != 500 || got.StaleCookieWrites != 0 || got.CurrentLoginStatus != 302 || !got.CurrentServiceRetained || got.CallbackTarget != "/alerts?saml=success" || got.ReplayTarget != "/settings?saml=error&saml_error=saml_validation_failed" || !got.SessionVerified {
		t.Errorf("management handover lost current native login: %+v", got)
	}
}

func TestSAMLInitializationConcurrentNativeLogins(t *testing.T) {
	for _, name := range []string{"login-first", "metadata-first", "management-first"} {
		metadataFirst := name == "metadata-first"
		managementFirst := name == "management-first"
		t.Run(name, func(t *testing.T) {
			metadataURL, entered, unblock := heldSAMLInitializationMetadata(t, samlInitializationMetadata)
			p := testSAMLProvider("okta", true)
			p.SAML.IDPMetadataURL = metadataURL
			r := newSAMLRouter(t, p)
			first, done := httptest.NewRecorder(), make(chan struct{})
			go func() {
				defer close(done)
				if managementFirst {
					if err := r.samlManager.InitializeProvider(t.Context(), p.ID, p.SAML); err != nil {
						t.Error(err)
					}
					first.WriteHeader(200)
				} else if metadataFirst {
					r.handleSAMLMetadata(first, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/metadata", nil))
				} else {
					r.handleSAMLLogin(first, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login?returnTo=/infrastructure", nil))
				}
			}()
			awaitOIDCInitialization(t, entered)
			second := httptest.NewRecorder()
			r.handleSAMLLogin(second, httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login", strings.NewReader(`{"returnTo":"/alerts"}`)))
			service := r.samlManager.GetService(p.ID)
			assertSAMLLoginRequest(t, second, service, "/alerts")
			pending := samlLoginRequests(service)
			unblock()
			awaitOIDCInitialization(t, done)
			expectedStatus, expectedRequests := 302, 2
			if metadataFirst || managementFirst {
				expectedStatus, expectedRequests = 200, 1
			}
			if first.Code != expectedStatus || r.samlManager.GetService(p.ID) != service || len(samlLoginRequests(service)) != expectedRequests {
				t.Fatalf("native completion displaced current login: status=%d retained=%t requests=%d", first.Code, r.samlManager.GetService(p.ID) == service, len(samlLoginRequests(service)))
			}
			for key, record := range pending {
				if !reflect.DeepEqual(samlLoginRequests(service)[key], record) {
					t.Error("native completion changed another browser's pending request")
				}
			}
		})
	}
}

func TestSAMLInitializationNativeRepairsMismatchedCache(t *testing.T) {
	p := testSAMLProvider("okta", true)
	r := newSAMLRouter(t, p)
	old := *p.SAML
	old.IDPSSOURL = "https://old-idp.example.com/sso"
	for _, metadata := range []bool{true, false} {
		if err := r.samlManager.InitializeProvider(context.Background(), p.ID, &old); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		if metadata {
			r.handleSAMLMetadata(response, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/metadata", nil))
		} else {
			r.handleSAMLLogin(response, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login", nil))
		}
		service := r.samlManager.GetService(p.ID)
		if !service.matchesConfig(p.SAML) {
			t.Error("native request reused stale saved trust/configuration")
		}
		if !metadata && !strings.HasPrefix(response.Header().Get("Location"), "https://idp.example.com/sso?") {
			t.Error("native login used previous IdP")
		}
	}
}

// The same metadata URL need not return the same trust snapshot twice. Neither
// native discovery nor management may silently call that a matching owner.
func TestSAMLInitializationDifferentCapturedTrust(t *testing.T) {
	for _, managementFirst := range []bool{false, true} {
		name := "native-first"
		if managementFirst {
			name = "management-first"
		}
		t.Run(name, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var reads atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body := samlInitializationMetadata
				if reads.Add(1) == 1 {
					close(entered)
					select {
					case <-release:
					case <-req.Context().Done():
						return
					}
				} else {
					body = strings.Replace(body, `entityID="https://idp.example.com/metadata"`, `entityID="https://current-idp.example.com/metadata"`, 1)
				}
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(server.Close)
			t.Cleanup(unblock)
			p := testSAMLProvider("okta", true)
			p.SAML.IDPMetadataURL = server.URL
			r := newSAMLRouter(t, p)
			first, done := httptest.NewRecorder(), make(chan struct{})
			go func() {
				defer close(done)
				if managementFirst {
					if err := r.samlManager.InitializeProvider(t.Context(), p.ID, p.SAML); err != nil {
						first.WriteHeader(500)
					} else {
						first.WriteHeader(200)
					}
				} else {
					r.handleSAMLLogin(first, httptest.NewRequest("GET", "https://pulse.example.com/api/saml/okta/login", nil))
				}
			}()
			awaitOIDCInitialization(t, entered)
			second := httptest.NewRecorder()
			r.handleSAMLLogin(second, httptest.NewRequest("POST", "https://pulse.example.com/api/saml/okta/login", strings.NewReader(`{"returnTo":"/alerts"}`)))
			current := r.samlManager.GetService(p.ID)
			assertSAMLLoginRequest(t, second, current, "/alerts")
			pending := samlLoginRequests(current)
			unblock()
			awaitOIDCInitialization(t, done)
			if first.Code != 500 || len(first.Result().Cookies()) != 0 || r.samlManager.GetService(p.ID) != current || !reflect.DeepEqual(samlLoginRequests(current), pending) {
				t.Fatal("different captured trust replaced or reused current browser authority")
			}
		})
	}
}
