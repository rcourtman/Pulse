package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSSOPresentationComparison(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(kind), func(t *testing.T) {
			original := unrepresentedSSOFixture(kind)
			original.Enabled = true
			if kind == config.SSOProviderTypeOIDC {
				original.SAML = nil
			} else {
				original.OIDC = nil
			}
			clone := func() config.SSOProvider {
				return (&config.SSOConfig{Providers: []config.SSOProvider{original}}).Clone().Providers[0]
			}
			// Enumerate all fields, including future additions. Only the four
			// presentation fields may avoid initialization; protocol fields,
			// credentials, trust, claims and policies must stay significant.
			var visit func(reflect.Value, string)
			visit = func(v reflect.Value, prefix string) {
				for i := 0; i < v.NumField(); i++ {
					field := v.Type().Field(i)
					name := prefix + field.Name
					if v.Field(i).Kind() == reflect.Pointer && !v.Field(i).IsNil() {
						visit(v.Field(i).Elem(), name+".")
						continue
					}
					if name == "OIDC.ClientSecretSet" {
						continue // presence marker, never a credential
					}
					t.Run(name, func(t *testing.T) {
						updated := clone()
						target := reflect.ValueOf(&updated).Elem()
						for _, part := range strings.Split(name, ".") {
							if target.Kind() == reflect.Pointer {
								target = target.Elem()
							}
							target = target.FieldByName(part)
						}
						switch target.Kind() {
						case reflect.String:
							target.SetString(target.String() + "changed")
						case reflect.Bool:
							target.SetBool(!target.Bool())
						case reflect.Int:
							target.SetInt(target.Int() + 1)
						case reflect.Slice:
							target.Set(reflect.ValueOf([]string{"changed"}))
						case reflect.Map:
							if target.Type().Elem().Kind() == reflect.Bool {
								target.Set(reflect.ValueOf(map[string]bool{"changed": true}))
							} else {
								target.Set(reflect.ValueOf(map[string]string{"changed": "viewer"}))
							}
						case reflect.Pointer:
							target.Set(reflect.New(target.Type().Elem()))
						default:
							t.Fatalf("add a boundary control for new field %s", name)
						}
						want := name == "Name" || name == "DisplayName" || name == "IconURL" || name == "Priority"
						require.Equal(t, want, ssoPresentationOnlyUpdate(original, updated))
					})
				}
			}
			visit(reflect.ValueOf(original), "")
			before := clone()
			updated := clone()
			original.AllowedEmails, updated.AllowedEmails = nil, []string{}
			original.GroupRoleMappings, updated.GroupRoleMappings = nil, map[string]string{}
			if original.OIDC != nil {
				original.OIDC.Scopes, updated.OIDC.Scopes = nil, []string{}
				updated.OIDC.ClientSecretSet = false
			}
			require.True(t, ssoPresentationOnlyUpdate(original, updated), "API empty collections must not look like a policy change")
			require.Empty(t, original.AllowedEmails)
			require.Equal(t, before.SAML, original.SAML)
			updated.Enabled = false
			require.False(t, ssoPresentationOnlyUpdate(original, updated))
			original.Enabled = false
			require.False(t, ssoPresentationOnlyUpdate(original, updated))
		})
	}
}

type presentationFixture struct {
	router   *Router
	provider config.SSOProvider
	requests atomic.Int32
	offline  atomic.Bool
}

// Both protocols use real service initialization and a guest-local metadata
// endpoint. All credentials, certificates and identities are synthetic.
func newPresentationFixture(t *testing.T, kind config.SSOProviderType) *presentationFixture {
	t.Helper()
	r, _ := setupTestRouter(t)
	r.config.PublicURL = "https://pulse.example.test"
	require.NoError(t, r.syncSAMLPublicURL())
	f := &presentationFixture{router: r}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f.requests.Add(1)
		if f.offline.Load() {
			http.Error(w, "synthetic metadata outage", http.StatusServiceUnavailable)
			return
		}
		if kind == config.SSOProviderTypeSAML {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://identity.example.test"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"><SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://identity.example.test/sso"/><SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://identity.example.test/slo"/></IDPSSODescriptor></EntityDescriptor>`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		base := "http://" + req.Host
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{
			"issuer": base, "authorization_endpoint": base + "/authorize",
			"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
		}))
	}))
	t.Cleanup(server.Close)
	p := config.SSOProvider{ID: "presentation", Name: "Original " + strings.ToUpper(string(kind)), Type: kind, Enabled: true,
		AllowedGroups: []string{"Operators"}, AllowedDomains: []string{"example.test"},
		GroupsClaim: "groups", GroupRoleMappings: map[string]string{"Operators": "viewer"}}
	if kind == config.SSOProviderTypeSAML {
		cert, key, _ := generateTestCert(t)
		p.SAML = &config.SAMLProviderConfig{IDPMetadataURL: server.URL + "/metadata", IDPSSOURL: "https://identity.example.test/sso", IDPEntityID: "https://identity.example.test",
			SPEntityID: "pulse", SPCertificate: string(cert), SPPrivateKey: string(key), SignRequests: true,
			UsernameAttr: "uid", EmailAttr: "mail", GroupsAttr: "groups", WantAssertionsSigned: true}
		require.NoError(t, r.samlManager.InitializeProvider(t.Context(), p.ID, p.SAML))
	} else {
		p.OIDC = &config.OIDCProviderConfig{IssuerURL: server.URL, ClientID: "pulse", ClientSecret: "synthetic-presentation-secret", ClientSecretSet: true, Scopes: []string{"openid", "email"}}
		require.NoError(t, r.oidcManager.InitializeProvider(t.Context(), p.ID, &p, ""))
	}
	t.Cleanup(func() { r.oidcManager.RemoveService(p.ID) })
	require.NoError(t, r.ssoConfig.AddProvider(p))
	r.ssoConfig.DefaultProviderID = p.ID
	require.NoError(t, r.saveSSOConfig())
	f.provider = p
	return f
}

func presentationPut(t *testing.T, r *Router, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	r.handleSSOProvider(rec, httptest.NewRequest("PUT", "/api/security/sso/providers/presentation", bytes.NewReader(data)))
	return rec
}

func TestSSOPresentationSavePreservesBindings(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeSAML, config.SSOProviderTypeOIDC} {
		for _, offline := range []bool{false, true} {
			for _, failSave := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/offline=%t/failSave=%t", kind, offline, failSave), func(t *testing.T) {
					f := newPresentationFixture(t, kind)
					r, p := f.router, f.provider
					priorSnapshot := r.ensureSSOConfig()
					before := priorSnapshot.Clone()
					file := filepath.Join(r.config.DataPath, "sso.enc")
					diskBefore, err := os.ReadFile(file)
					require.NoError(t, err)
					var samlService *SAMLService
					var oidcService *OIDCService
					var loginID, logoutID, state string
					if kind == config.SSOProviderTypeSAML {
						samlService = r.samlManager.GetService(p.ID)
						_, err := samlService.MakeAuthRequest("/history", "original-browser")
						require.NoError(t, err)
						ids, bound := samlService.authnRequests.outstanding("original-browser", time.Now())
						require.True(t, bound)
						require.Len(t, ids, 1)
						loginID = ids[0]
						_, err = samlService.MakeLogoutRequest("synthetic-subject", "synthetic-index", "original-session")
						require.NoError(t, err)
						ids, bound = samlService.logoutRequests.outstanding("original-session", time.Now())
						require.True(t, bound)
						require.Len(t, ids, 1)
						logoutID = ids[0]
					} else {
						oidcService = r.oidcManager.GetService(p.ID)
						state, _, err = oidcService.newStateEntry(p.ID, "/history")
						require.NoError(t, err)
					}
					// Use the non-secret editor DTO: blank OIDC secrets and omitted
					// advanced SAML settings must merge before runtime comparison.
					detail := providerToDetailResponse(&p, r.config.PublicURL, httptest.NewRequest("GET", "/", nil))
					payload := detail
					payload.Name = "Renamed " + strings.ToUpper(string(kind))
					f.offline.Store(offline)
					if failSave {
						// Fail only the write through the existing filesystem seam;
						// constructing an invalid persistence root is fatal by design.
						r.persistence.SetFileSystem(failingWriteFileSystem{})
					}
					rec := presentationPut(t, r, payload)
					wantStatus := http.StatusOK
					if failSave {
						wantStatus = http.StatusInternalServerError
					}
					require.Equal(t, wantStatus, rec.Code, rec.Body.String())
					require.Equal(t, int32(1), f.requests.Load(), "presentation save fetched IdP metadata")
					require.Equal(t, before, priorSnapshot, "save mutated the published old snapshot")
					if samlService != nil {
						require.Same(t, samlService, r.samlManager.GetService(p.ID), "label edit discarded SAML request stores")
						_, err := samlService.authnRequests.spend(loginID, "different-browser", time.Now())
						require.ErrorIs(t, err, errSAMLResponseUnbound)
						binding, err := samlService.authnRequests.spend(loginID, "original-browser", time.Now())
						require.NoError(t, err)
						require.Equal(t, "/history", binding.returnTo)
						_, err = samlService.authnRequests.spend(loginID, "original-browser", time.Now())
						require.ErrorIs(t, err, errSAMLResponseUnbound)
						require.NoError(t, samlService.logoutRequests.consume(logoutID, "original-session", time.Now()))
						require.ErrorIs(t, samlService.logoutRequests.consume(logoutID, "original-session", time.Now()), errSAMLLogoutResponseUnbound)
					} else {
						require.Same(t, oidcService, r.oidcManager.GetService(p.ID))
						binding, ok := oidcService.consumeState(state)
						require.True(t, ok, "label edit discarded OIDC nonce/PKCE state")
						require.Equal(t, "/history", binding.ReturnTo)
						require.NotEmpty(t, binding.Nonce)
						require.NotEmpty(t, binding.CodeVerifier)
						_, ok = oidcService.consumeState(state)
						require.False(t, ok)
					}
					// Read the actual encrypted store through a fresh persistence
					// reader, independent of the failed writer when applicable.
					fresh := config.NewConfigPersistence(r.config.DataPath)
					saved, err := fresh.LoadSSOConfig()
					require.NoError(t, err)
					expected := before.Clone()
					if !failSave {
						expected.Providers[0].Name = payload.Name
						// Detail DTO renders an empty displayName as the name.
						expected.Providers[0].DisplayName = payload.DisplayName
					}
					require.Equal(t, expected, saved)
					wantJSON, err := json.Marshal(expected)
					require.NoError(t, err)
					liveJSON, err := json.Marshal(r.ensureSSOConfig())
					require.NoError(t, err)
					require.JSONEq(t, string(wantJSON), string(liveJSON), "persisted nil and live empty collections have identical JSON semantics")
					diskAfter, err := os.ReadFile(file)
					require.NoError(t, err)
					require.False(t, json.Valid(diskAfter), "SSO config was not encrypted")
					if failSave {
						require.Equal(t, diskBefore, diskAfter)
					}
					for _, secret := range []string{"synthetic-presentation-secret", "PRIVATE KEY"} {
						require.NotContains(t, rec.Body.String(), secret)
						require.NotContains(t, string(diskAfter), secret)
					}
				})
			}
		}
	}
}

func TestSSOPresentationRuntimeLifecycle(t *testing.T) {
	for _, kind := range []config.SSOProviderType{config.SSOProviderTypeSAML, config.SSOProviderTypeOIDC} {
		for _, change := range []string{"policy", "protocol", "missing service", "disable", "enable", "change type"} {
			t.Run(string(kind)+"/"+change, func(t *testing.T) {
				f := newPresentationFixture(t, kind)
				r, p := f.router, f.provider
				oldSAML, oldOIDC := r.samlManager.GetService(p.ID), r.oidcManager.GetService(p.ID)
				switch change {
				case "policy":
					p.AllowedGroups = []string{"different-group"}
				case "protocol":
					if p.SAML != nil {
						copy := *p.SAML
						copy.NameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
						p.SAML = &copy
					} else {
						copy := *p.OIDC
						copy.ClientSecret = "synthetic-replacement-secret"
						p.OIDC = &copy
					}
				case "missing service":
					r.samlManager.RemoveProvider(p.ID)
					r.oidcManager.RemoveService(p.ID)
				case "disable":
					p.Enabled = false
				case "enable":
					_, err := r.commitSSOConfig(func(cfg *config.SSOConfig) error { cfg.GetProvider(p.ID).Enabled = false; return nil })
					require.NoError(t, err)
				case "change type":
					other := newPresentationFixture(t, map[config.SSOProviderType]config.SSOProviderType{config.SSOProviderTypeSAML: config.SSOProviderTypeOIDC, config.SSOProviderTypeOIDC: config.SSOProviderTypeSAML}[kind])
					p.Type, p.OIDC, p.SAML = other.provider.Type, other.provider.OIDC, other.provider.SAML
				}
				p.Name = "Renamed provider"
				rec := presentationPut(t, r, p)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				if change == "disable" || change == "change type" {
					if kind == config.SSOProviderTypeSAML {
						require.Nil(t, r.samlManager.GetService(p.ID))
					} else {
						require.Nil(t, r.oidcManager.GetService(p.ID))
					}
				}
				if p.Enabled {
					if p.Type == config.SSOProviderTypeSAML {
						require.NotNil(t, r.samlManager.GetService(p.ID))
						require.NotSame(t, oldSAML, r.samlManager.GetService(p.ID))
					} else {
						require.NotNil(t, r.oidcManager.GetService(p.ID))
						require.NotSame(t, oldOIDC, r.oidcManager.GetService(p.ID))
					}
				}
				if change != "disable" && change != "change type" {
					require.Equal(t, int32(2), f.requests.Load(), "changed runtime or missing service did not initialize")
				}
			})
		}
	}
}
