package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

// All credentials and identity-provider endpoints in these tests are synthetic.
func environmentBoundaryRouter(t *testing.T) *Router {
	t.Helper()
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://environment.example.test")
	t.Setenv("OIDC_CLIENT_ID", "environment-client")
	t.Setenv("OIDC_CLIENT_SECRET", "synthetic-environment-secret")
	r, _ := setupTestRouter(t)
	r.ssoConfig.Providers = []config.SSOProvider{{
		ID: "saved", Name: "Saved recovery", Type: config.SSOProviderTypeOIDC,
		AllowedGroups: []string{"operators"}, GroupRoleMappings: map[string]string{"operators": "admin"},
		OIDC: &config.OIDCProviderConfig{IssuerURL: "https://saved.example.test", ClientID: "saved-client", ClientSecret: "synthetic-saved-secret", ClientSecretSet: true, Scopes: []string{"openid", "email"}},
	}}
	r.ensureSSOConfig()
	if err := r.saveSSOConfig(); err != nil {
		t.Fatal(err)
	}
	return r
}

func assertEnvironmentNotPersisted(t *testing.T, r *Router) {
	t.Helper()
	live := r.currentSSOConfig().GetProvider(config.LegacyOIDCProviderID)
	if live == nil || !live.RuntimeManaged || live.OIDC == nil || live.OIDC.ClientSecret != "synthetic-environment-secret" || !live.OIDC.EnvOverrides["clientSecret"] {
		t.Error("live environment authority or credential was lost")
	}
	disk, err := r.persistence.LoadSSOConfig()
	if err != nil {
		t.Fatal(err)
	}
	if disk.GetProvider(config.LegacyOIDCProviderID) != nil {
		t.Error("environment provider was persisted")
	}
	data, err := os.ReadFile(filepath.Join(r.config.DataPath, "sso.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("synthetic-environment-secret")) {
		t.Error("environment credential reached disk")
	}
}

func TestSSOEnvironmentBoundaryRoundTrip(t *testing.T) {
	r := environmentBoundaryRouter(t)
	core := r.ensureSSOConfig()
	roundTrip := toCoreSSOConfig(toExtensionSSOConfigSnapshot(core))
	if p := roundTrip.GetProvider(config.LegacyOIDCProviderID); p == nil || !p.RuntimeManaged {
		t.Error("extension conversion discarded environment authority")
	}
	encoded, err := json.Marshal(toExtensionSSOConfigSnapshot(core))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(encoded), []byte("runtimemanaged")) {
		t.Error("internal authority marker is JSON-writable")
	}
}

func TestSSOEnvironmentBoundaryUnrelatedSave(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "marker-preserved", true: "legacy-marker-omitted"}[legacy], func(t *testing.T) {
			r := environmentBoundaryRouter(t)
			hooks := newSSOAdminRuntime(r)
			snapshot := hooks.GetSSOConfigSnapshot()
			if legacy {
				encoded, err := json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				snapshot = extensions.SSOConfigSnapshot{}
				if err := json.Unmarshal(encoded, &snapshot); err != nil {
					t.Fatal(err)
				}
			}
			for i := range snapshot.Providers {
				if snapshot.Providers[i].ID == "saved" {
					snapshot.Providers[i].Name = "Edited recovery"
				}
			}
			if err := hooks.SaveSSOConfigSnapshot(snapshot); err != nil {
				t.Fatal(err)
			}
			assertEnvironmentNotPersisted(t, r)
			if r.currentSSOConfig().GetProvider("saved").Name != "Edited recovery" {
				t.Error("unrelated saved-provider edit was lost")
			}
			if r.currentSSOConfig().GetProvider("saved").OIDC.ClientSecret != "synthetic-saved-secret" {
				t.Error("saved-provider secret was lost")
			}
		})
	}
}

func TestSSOEnvironmentBoundaryRejectsSnapshotMutation(t *testing.T) {
	for _, mutation := range []string{"name", "secret", "disable", "delete", "duplicate", "new-runtime", "saved-runtime"} {
		t.Run(mutation, func(t *testing.T) {
			r := environmentBoundaryRouter(t)
			hooks := newSSOAdminRuntime(r)
			snapshot := hooks.GetSSOConfigSnapshot()
			before := r.currentSSOConfig().Clone()
			beforeDisk, err := os.ReadFile(filepath.Join(r.config.DataPath, "sso.enc"))
			if err != nil {
				t.Fatal(err)
			}
			for i := range snapshot.Providers {
				p := &snapshot.Providers[i]
				if p.ID != config.LegacyOIDCProviderID {
					continue
				}
				switch mutation {
				case "name":
					p.Name = "changed"
				case "secret":
					p.OIDC.ClientSecret = "changed"
				case "disable":
					p.Enabled = false
				case "delete":
					snapshot.Providers = append(snapshot.Providers[:i], snapshot.Providers[i+1:]...)
				case "duplicate":
					snapshot.Providers = append(snapshot.Providers, *p)
				case "new-runtime", "saved-runtime":
					id := "new"
					if mutation == "saved-runtime" {
						id = "saved"
					}
					forged := toExtensionSSOProvider(config.SSOProvider{ID: id, Name: "Forged", RuntimeManaged: true})
					if id == "saved" {
						snapshot.Providers[0] = forged
					} else {
						snapshot.Providers = append(snapshot.Providers, forged)
					}
				}
				break
			}
			if err := hooks.SaveSSOConfigSnapshot(snapshot); err == nil {
				t.Error("snapshot environment mutation was accepted")
			}
			if !reflect.DeepEqual(before, r.currentSSOConfig()) {
				t.Error("refused save changed the published snapshot")
			}
			afterDisk, err := os.ReadFile(filepath.Join(r.config.DataPath, "sso.enc"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(beforeDisk, afterDisk) {
				t.Error("refused save changed persisted bytes")
			}
			assertEnvironmentNotPersisted(t, r)
		})
	}
}

func TestSSOEnvironmentBoundarySnapshotOwnership(t *testing.T) {
	for _, stage := range []string{"read", "save"} {
		t.Run(stage, func(t *testing.T) {
			r := environmentBoundaryRouter(t)
			hooks := newSSOAdminRuntime(r)
			snapshot := hooks.GetSSOConfigSnapshot()
			if stage == "save" {
				if err := hooks.SaveSSOConfigSnapshot(snapshot); err != nil {
					t.Fatal(err)
				}
			}
			before := r.currentSSOConfig().Clone()
			snapshot.Providers[0].AllowedGroups[0] = "changed"
			snapshot.Providers[0].GroupRoleMappings["operators"] = "viewer"
			snapshot.Providers[0].OIDC.Scopes[0] = "changed"
			if !reflect.DeepEqual(before, r.currentSSOConfig()) {
				t.Error("caller-owned snapshot mutates published authorization")
			}
		})
	}
}

func TestSSOEnvironmentBoundaryInitialization(t *testing.T) {
	r := environmentBoundaryRouter(t)
	hooks := newSSOAdminRuntime(r)
	provider := toExtensionSSOProvider(*r.currentSSOConfig().GetProvider(config.LegacyOIDCProviderID))
	// Nil managers must not turn a prohibited operation into apparent success.
	r.oidcManager = nil
	r.samlManager = nil
	if err := hooks.InitializeOIDCProvider(context.Background(), provider.ID, &provider); err == nil {
		t.Error("environment OIDC initialization was accepted")
	}
	if err := hooks.InitializeSAMLProvider(context.Background(), provider.ID, &extensions.SAMLProviderConfig{}); err == nil {
		t.Error("environment type change was accepted")
	}
	if err := hooks.InitializeOIDCProvider(context.Background(), "saved", &provider); err != nil {
		t.Error("saved-provider initialization was blocked")
	}
}

func TestSSOEnvironmentBoundaryRemoval(t *testing.T) {
	r := environmentBoundaryRouter(t)
	id := config.LegacyOIDCProviderID
	oidc := &OIDCService{stateStore: newOIDCStateStore()}
	t.Cleanup(oidc.stateStore.Stop)
	saml := &SAMLService{}
	r.oidcManager.services[id] = oidc
	r.oidcManager.services["saved"] = oidc
	r.samlManager.services[id] = saml
	r.samlManager.services["saved"] = saml
	hooks := newSSOAdminRuntime(r)
	hooks.RemoveOIDCProvider(id)
	hooks.RemoveSAMLProvider(id)
	if r.oidcManager.GetService(id) != oidc {
		t.Error("extension removed environment OIDC service")
	}
	if r.samlManager.GetService(id) != saml {
		t.Error("extension removed environment SAML service")
	}
	hooks.RemoveOIDCProvider("saved")
	hooks.RemoveSAMLProvider("saved")
	if r.oidcManager.GetService("saved") != nil || r.samlManager.GetService("saved") != nil {
		t.Error("saved-provider removal was blocked")
	}
}

type environmentBoundaryEndpointProbe struct {
	extensions.SSOAdminEndpoints
	writes     int
	permissive bool
}

func (p *environmentBoundaryEndpointProbe) HandleProviderItem(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPut || req.Method == http.MethodDelete {
		p.writes++
		if p.permissive {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	p.SSOAdminEndpoints.HandleProviderItem(w, req)
}

func TestSSOEnvironmentBoundaryBeforeExtensionDispatch(t *testing.T) {
	fixture := environmentBoundaryRouter(t)
	const token = "sso-environment-boundary-synthetic-token.12345678"
	record := newTokenRecord(t, token, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	var probe *environmentBoundaryEndpointProbe
	SetSSOAdminEndpointsBinder(func(defaults extensions.SSOAdminEndpoints, _ extensions.SSOAdminRuntime) extensions.SSOAdminEndpoints {
		probe = &environmentBoundaryEndpointProbe{SSOAdminEndpoints: defaults, permissive: true}
		return probe
	})
	t.Cleanup(func() { SetSSOAdminEndpointsBinder(nil) })
	r := NewRouter(cfg, nil, nil, nil, nil, "test")
	r.ssoConfig = fixture.currentSSOConfig().Clone()
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		for _, suffix := range []string{"", "/"} {
			req := httptest.NewRequest(method, "/api/security/sso/providers/"+config.LegacyOIDCProviderID+suffix, strings.NewReader(`{"name":"changed","type":"oidc"}`))
			req.RemoteAddr = ssoTestIP()
			req.Header.Set("X-API-Token", token)
			w := httptest.NewRecorder()
			r.Handler().ServeHTTP(w, req)
			if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "provider_managed_by_environment") {
				t.Errorf("%s environment provider: got %d, want 409", method, w.Code)
			}
		}
	}
	if probe.writes != 0 {
		t.Errorf("environment writes reached extension %d times", probe.writes)
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/security/sso/providers/saved", strings.NewReader(`{}`))
		req.RemoteAddr = ssoTestIP()
		req.Header.Set("X-API-Token", token)
		w := httptest.NewRecorder()
		r.Handler().ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("saved provider: got %d, want extension 204", w.Code)
		}
	}
	if probe.writes != 2 {
		t.Error("saved-provider dispatch was blocked")
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/security/sso/providers/"+config.LegacyOIDCProviderID, nil)
	req.RemoteAddr = ssoTestIP()
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("authentication must precede environment detail, got %d", w.Code)
	}
}
