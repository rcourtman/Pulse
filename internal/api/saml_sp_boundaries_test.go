package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The same saved path must be safe before and after PublicURL is captured.
// In particular a missing leading slash must never become host/userinfo/port.
func TestSAMLSPPathsStayOnPublicAuthority(t *testing.T) {
	for _, base := range []string{"", "https://pulse.example.test", "https://pulse.example.test/base", "https://[2001:db8::1]:8443/pulse"} {
		for _, field := range []string{"acs", "metadata"} {
			for _, tc := range []struct {
				name, path string
				valid      bool
			}{
				{"default", "", true}, {"rooted", "/api/saml/setup/acs/", true}, {"query", "/custom/acs?tenant=ops%2fwest#section", true}, {"encoded", "/custom/%2fcallback", true}, {"double slash", "//custom/acs", true},
				{"userinfo", "@other.example.test/acs", false}, {"host suffix", ".other.example.test/acs", false}, {"port", ":9999/acs", false}, {"relative", "api/acs", false}, {"absolute", "https://other.example.test/acs", false}, {"query only", "?acs=/", false}, {"fragment only", "#acs/", false}, {"whitespace", " /acs", false}, {"backslash prefix", `\other/acs`, false}, {"bad escape", "/acs%zz", false}, {"control", "/acs\n", false},
			} {
				t.Run(fmt.Sprintf("%s/%s/%s", base, field, tc.name), func(t *testing.T) {
					p := testSAMLProvider("setup", true)
					p.SAML.SPEntityID = "urn:pulse:operations"
					if field == "acs" {
						p.SAML.SPACSPath = tc.path
					} else {
						p.SAML.SPMetadataPath = tc.path
					}
					endpoints, err := buildSAMLSPURLs(p.ID, p.SAML, base)
					if !tc.valid {
						if err == nil {
							t.Fatalf("unsafe %s path accepted: %+v", field, endpoints)
						}
						if endpoints != (samlSPURLs{}) {
							t.Fatal("failed construction leaked partial endpoints")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if endpoints.entityID != p.SAML.SPEntityID {
						t.Fatal("explicit entity changed")
					}
					for name, got := range map[string]url.URL{"acs": endpoints.acs, "metadata": endpoints.metadata, "slo": endpoints.slo} {
						wantPath := "/api/saml/setup/" + name
						if name == field && tc.path != "" {
							wantPath = tc.path
						}
						if base != "" {
							want, err := url.Parse(base + wantPath)
							if err != nil {
								t.Fatal(err)
							}
							if got.String() != want.String() || got.User != nil || got.Host != want.Host || got.Scheme != want.Scheme {
								t.Fatalf("%s changed target: %s vs %s", name, got.String(), want.String())
							}
						} else if got.Scheme != "" || got.Host != "" || got.User != nil || got.String() != wantPath {
							t.Fatalf("unconfigured %s acquired authority or lost path: %+v", name, got)
						}
					}
				})
			}
		}
	}
}

func TestSAMLSPInvalidSavedPathsCannotInitializeOrAdvertise(t *testing.T) {
	for _, base := range []string{"", "https://pulse.example.test", "https://pulse.example.test/base"} {
		for _, field := range []string{"acs", "metadata"} {
			t.Run(base+"/"+field, func(t *testing.T) {
				p := testSAMLProvider("setup", true)
				p.SAML.SPEntityID = "urn:pulse:operations"
				if field == "acs" {
					p.SAML.SPACSPath = "@other.example.test/acs"
				} else {
					p.SAML.SPMetadataPath = ".other.example.test/metadata"
				}
				if svc, err := NewSAMLService(context.Background(), p.ID, p.SAML, base); err == nil || svc != nil {
					t.Fatal("initialized unsafe saved provider")
				}
				r := &Router{config: &config.Config{PublicURL: base}, ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{p}}, samlManager: NewSAMLServiceManager("")}
				req := httptest.NewRequest("GET", "https://pulse.lan/api/saml/setup/metadata", nil)
				got := providerToResponse(&p, base, req)
				if got.SAMLACSURL != "" || got.SAMLMetadataURL != "" || got.SAMLSPEntityID != p.SAML.SPEntityID {
					t.Fatalf("unsafe/partial transport target advertised or explicit entity lost: %+v", got)
				}
				for _, run := range []struct {
					path   string
					handle func(http.ResponseWriter, *http.Request)
				}{{"metadata", r.handleSAMLMetadata}, {"login", r.handleSAMLLogin}} {
					rr := httptest.NewRecorder()
					run.handle(rr, httptest.NewRequest("GET", "https://pulse.lan/api/saml/setup/"+run.path, nil))
					if rr.Code == http.StatusOK || rr.Code == http.StatusFound || rr.Header().Get("Location") != "" {
						t.Fatalf("unsafe provider served a transport target: %d %s", rr.Code, rr.Body.String())
					}
				}
				if r.samlManager.GetService(p.ID) != nil {
					t.Fatal("failed init published a service")
				}
				if r.ssoPublicURL() != base || r.getSSOProvider(p.ID).SAML.SPEntityID != p.SAML.SPEntityID {
					t.Fatal("rejection changed saved authority")
				}
			})
		}
	}
}

func TestSAMLSPFailedReplacementKeepsServiceAndPendingBindings(t *testing.T) {
	p := testSAMLProvider("setup", true)
	m := NewSAMLServiceManager("https://pulse.example.test")
	if err := m.InitializeProvider(context.Background(), p.ID, p.SAML); err != nil {
		t.Fatal(err)
	}
	old := m.GetService(p.ID)
	now := time.Now()
	old.authnRequests.put("pending", samlRequestRecord{bindingKey: "browser", returnTo: "/alerts"}, now)
	bad := *p.SAML
	bad.SPACSPath = ":9999/acs"
	if err := m.InitializeProvider(context.Background(), p.ID, &bad); err == nil {
		t.Fatal("unsafe replacement accepted")
	}
	if m.GetService(p.ID) != old || old.config != p.SAML {
		t.Fatal("failed replacement erased existing service/config")
	}
	entry, err := old.authnRequests.spend("pending", "browser", now)
	if err != nil || entry.returnTo != "/alerts" {
		t.Fatal("failed replacement discarded browser binding")
	}
}

func TestSAMLSPPathChangesAreTransactional(t *testing.T) {
	for _, via := range []string{"create", "update", "private snapshot"} {
		for _, field := range []string{"acs", "metadata"} {
			t.Run(via+"/"+field, func(t *testing.T) {
				p := testSAMLProvider("setup", false)
				p.SAML.SPPrivateKey = "synthetic-key-canary"
				p.SAML.SPEntityID = "urn:pulse:operations"
				r := newSAMLRouter(t, p)
				r.persistence = config.NewConfigPersistence(t.TempDir())
				if err := r.saveSSOConfig(); err != nil {
					t.Fatal(err)
				}
				before := r.ensureSSOConfig().Clone()
				beforeBytes, err := os.ReadFile(filepath.Join(r.persistence.GetConfigDir(), "sso.enc"))
				if err != nil {
					t.Fatal(err)
				}
				changed := *p.SAML
				if field == "acs" {
					changed.SPACSPath = "api/acs"
				} else {
					changed.SPMetadataPath = "@other.example.test/metadata"
				}
				if via == "private snapshot" {
					runtime := newSSOAdminRuntime(r)
					snap := runtime.GetSSOConfigSnapshot()
					if field == "acs" {
						snap.Providers[0].SAML.SPACSPath = changed.SPACSPath
					} else {
						snap.Providers[0].SAML.SPMetadataPath = changed.SPMetadataPath
					}
					if err := runtime.SaveSSOConfigSnapshot(snap); err == nil {
						t.Fatal("private binding persisted unsafe path")
					}
				} else {
					body := map[string]interface{}{"id": p.ID, "name": p.Name, "type": "saml", "enabled": false, "saml": changed}
					rr := httptest.NewRecorder()
					raw, _ := json.Marshal(body)
					req := httptest.NewRequest("POST", "/api/security/sso/providers", strings.NewReader(string(raw)))
					if via == "create" {
						body["id"] = "new-provider"
						raw, _ = json.Marshal(body)
						req = httptest.NewRequest("POST", "/api/security/sso/providers", strings.NewReader(string(raw)))
						r.handleCreateSSOProvider(rr, req)
					} else {
						req.Method = "PUT"
						r.handleUpdateSSOProvider(rr, req, p.ID)
					}
					if rr.Code != http.StatusBadRequest {
						t.Fatalf("invalid write=%d %s", rr.Code, rr.Body.String())
					}
				}
				if !reflect.DeepEqual(r.ensureSSOConfig(), before) {
					t.Fatal("invalid write published replacement config")
				}
				after, err := os.ReadFile(filepath.Join(r.persistence.GetConfigDir(), "sso.enc"))
				if err != nil || string(beforeBytes) != string(after) {
					t.Fatal("invalid write changed saved bytes")
				}

				// The corresponding valid save must still work, including paths
				// that need the uncaptured-base handling and saved private fields.
				safe := *p.SAML
				safe.SPACSPath = "//api/saml/setup/acs?tenant=ops%2fwest"
				safe.SPMetadataPath = "/api/saml/setup/metadata/"
				id := p.ID
				if via == "private snapshot" {
					runtime := newSSOAdminRuntime(r)
					snapshot := runtime.GetSSOConfigSnapshot()
					snapshot.Providers[0].SAML.SPACSPath = safe.SPACSPath
					snapshot.Providers[0].SAML.SPMetadataPath = safe.SPMetadataPath
					if err := runtime.SaveSSOConfigSnapshot(snapshot); err != nil {
						t.Fatal(err)
					}
				} else {
					if via == "create" {
						id = "new-provider"
					}
					body, err := json.Marshal(map[string]interface{}{"id": id, "name": p.Name, "type": "saml", "enabled": false, "saml": safe})
					if err != nil {
						t.Fatal(err)
					}
					rr := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/api/security/sso/providers", strings.NewReader(string(body)))
					want := http.StatusCreated
					if via == "create" {
						r.handleCreateSSOProvider(rr, req)
					} else {
						want = http.StatusOK
						req.Method = http.MethodPut
						r.handleUpdateSSOProvider(rr, req, id)
					}
					if rr.Code != want {
						t.Fatalf("valid custom path save=%d %s", rr.Code, rr.Body.String())
					}
				}
				loaded, err := r.persistence.LoadPersistedSSOConfig()
				if err != nil || loaded == nil || loaded.GetProvider(id) == nil {
					t.Fatalf("saved readback: %v", err)
				}
				if !reflect.DeepEqual(loaded.GetProvider(id).SAML, &safe) || !reflect.DeepEqual(r.getSSOProvider(id).SAML, &safe) {
					t.Fatal("valid custom path save changed paths, entity or saved private fields")
				}
			})
		}
	}
}

// A legacy malformed entry must remain repairable/disableable/deletable and
// cannot block an unrelated provider's save. Validation is not a migration.
func TestSAMLSPExistingMalformedPathsRemainRecoverable(t *testing.T) {
	p := testSAMLProvider("setup", true)
	p.SAML.SPACSPath = "@other.example.test/acs"
	p.SAML.SPPrivateKey = "synthetic-key-canary"
	r := newSAMLRouter(t, p)
	apply, save := r.commitSSOConfig(func(c *config.SSOConfig) error { c.GetProvider(p.ID).Enabled = false; return nil })
	if apply != nil || save != nil {
		t.Fatalf("disable invalid legacy provider: %v %v", apply, save)
	}
	apply, save = r.commitSSOConfig(func(c *config.SSOConfig) error { return c.AddProvider(newRecoveryTestProvider("other", false, "")) })
	if apply != nil || save != nil {
		t.Fatalf("unrelated update: %v %v", apply, save)
	}
	apply, _ = r.commitSSOConfig(func(c *config.SSOConfig) error { c.GetProvider(p.ID).Enabled = true; return nil })
	if apply == nil {
		t.Fatal("unsafe disabled provider re-enabled")
	}
	apply, save = r.commitSSOConfig(func(c *config.SSOConfig) error {
		c.GetProvider(p.ID).SAML.SPACSPath = "/api/saml/setup/acs/"
		return nil
	})
	if apply != nil || save != nil {
		t.Fatalf("repair legacy path: %v %v", apply, save)
	}
	if got := r.getSSOProvider(p.ID).SAML.SPPrivateKey; got != p.SAML.SPPrivateKey {
		t.Fatal("repair erased saved private key")
	}
	apply, save = r.commitSSOConfig(func(c *config.SSOConfig) error { return c.RemoveProvider(p.ID) })
	if apply != nil || save != nil {
		t.Fatalf("remove legacy provider: %v %v", apply, save)
	}
}

func TestSSOAdminPublicURLUsesCaptureMutex(t *testing.T) {
	if newSSOAdminRuntime(nil).GetPublicURL != nil {
		t.Fatal("nil router unexpectedly has a binding")
	}
	for _, r := range []*Router{{}, {config: &config.Config{PublicURL: "https://pulse.example.test"}}} {
		get := newSSOAdminRuntime(r).GetPublicURL
		if r == nil || r.config == nil {
			if get() != "" {
				t.Fatal("nil config invented a base")
			}
			continue
		}
		r.publicURLMu.Lock()
		started := make(chan struct{})
		result := make(chan string, 1)
		go func() { close(started); result <- get() }()
		<-started
		var early bool
		select {
		case <-result:
			early = true
		case <-time.After(50 * time.Millisecond):
		}
		r.publicURLMu.Unlock()
		if early {
			t.Fatal("runtime reader bypassed public-URL capture mutex")
		}
		select {
		case got := <-result:
			if got != r.ssoPublicURL() {
				t.Fatal("runtime changed configured base")
			}
		case <-time.After(time.Second):
			t.Fatal("runtime reader did not resume after capture")
		}
	}
}

func TestSSOAdminPublicURLConcurrentCapture(t *testing.T) {
	const token = "synthetic-capture-token.12345678"
	for i := 0; i < 20; i++ {
		record, err := config.NewAPITokenRecord(token, "capture", []string{config.ScopeSettingsWrite})
		if err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{APITokens: []config.APITokenRecord{*record}}
		r := &Router{config: cfg, samlManager: NewSAMLServiceManager("")}
		runtime := newSSOAdminRuntime(r)
		req := httptest.NewRequest("GET", "https://pulse.example.test/api/health", nil)
		req.Header.Set("X-API-Token", token)
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; r.capturePublicURLFromRequest(req) }()
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 100; j++ {
				if got := runtime.GetPublicURL(); got != "" && got != "https://pulse.example.test" {
					t.Errorf("torn/foreign base: %q", got)
				}
			}
		}()
		close(start)
		wg.Wait()
		if got := runtime.GetPublicURL(); got != "https://pulse.example.test" {
			t.Fatalf("authenticated capture not visible: %q", got)
		}
	}
}

func TestSAMLSPCustomPathsMatchNativeTargets(t *testing.T) {
	for _, base := range []string{"", "https://pulse.example.test/base/?source=setup#section"} {
		t.Run(base, func(t *testing.T) {
			p := testSAMLProvider("setup", true)
			p.SAML.SPEntityID = "urn:pulse:operations"
			p.SAML.SPACSPath = "//api/saml/setup/acs?tenant=ops%2fwest"
			p.SAML.SPMetadataPath = "/api/saml/setup/metadata/"
			r := &Router{config: &config.Config{PublicURL: base}, ssoConfig: &config.SSOConfig{Providers: []config.SSOProvider{p}}, samlManager: NewSAMLServiceManager("")}
			req := httptest.NewRequest("GET", "https://pulse.lan/api/saml/setup/metadata", nil)
			display := providerToResponse(&p, base, req)
			wantBase := "https://pulse.lan"
			if base != "" {
				wantBase = "https://pulse.example.test/base"
			}
			if display.SAMLACSURL != wantBase+p.SAML.SPACSPath || display.SAMLMetadataURL != wantBase+p.SAML.SPMetadataPath || display.SAMLSPEntityID != p.SAML.SPEntityID {
				t.Fatalf("saved custom targets changed: %+v", display)
			}
			meta := httptest.NewRecorder()
			r.handleSAMLMetadata(meta, req)
			if meta.Code != 200 || !strings.Contains(meta.Body.String(), `Location="`+display.SAMLACSURL+`"`) {
				t.Fatalf("native metadata disagrees: %d %s", meta.Code, meta.Body.String())
			}
			login := httptest.NewRecorder()
			r.handleSAMLLogin(login, httptest.NewRequest("GET", "https://pulse.lan/api/saml/setup/login", nil))
			if login.Code != 302 {
				t.Fatalf("native login=%d %s", login.Code, login.Body.String())
			}
			authn := registrationAuthnRequest(t, login.Header().Get("Location"))
			if authn.AssertionConsumerServiceURL != display.SAMLACSURL || authn.Issuer == nil || authn.Issuer.Value != p.SAML.SPEntityID {
				t.Fatal("native AuthnRequest changed custom targets")
			}
			if base == "" && r.ssoPublicURL() != "" {
				t.Fatal("request-local target changed shared base")
			}
		})
	}
}

func TestSAMLSPRejectsPathBeforeFetchingIDPMetadata(t *testing.T) {
	var calls atomic.Int32
	srv := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { calls.Add(1); http.Error(w, "not expected", 500) }))
	defer srv.Close()
	p := testSAMLProvider("setup", true)
	p.SAML.IDPMetadataURL = srv.URL
	p.SAML.SPACSPath = "api/acs"
	if _, err := NewSAMLService(context.Background(), p.ID, p.SAML, "https://pulse.example.test"); err == nil {
		t.Fatal("unsafe path accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("unsafe path reached IdP metadata fetch")
	}
}
