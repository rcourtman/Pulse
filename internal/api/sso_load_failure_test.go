package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/stretchr/testify/require"
)

// Only synthetic local encrypted SSO files. No real IdP or external service.
func ssoLoadFixture(t *testing.T) (*Router, string, []byte) {
	t.Helper()
	t.Setenv("OIDC_ENABLED", "false")
	r, dir := setupTestRouter(t)
	cfg := config.NewSSOConfig()
	require.NoError(t, cfg.AddProvider(config.SSOProvider{ID: "saved", Name: "Saved recovery", Type: config.SSOProviderTypeOIDC, AllowedGroups: []string{"ops"}, GroupRoleMappings: map[string]string{"ops": "viewer"}, OIDC: &config.OIDCProviderConfig{IssuerURL: "https://saved.example.test", ClientID: "saved-client", ClientSecret: "synthetic-retained-secret", ClientSecretSet: true, Scopes: []string{"openid", "email"}}}))
	require.NoError(t, r.persistence.SaveSSOConfig(cfg))
	file := filepath.Join(dir, "sso.enc")
	b, err := os.ReadFile(file)
	require.NoError(t, err)
	require.NotContains(t, string(b), "synthetic-retained-secret")
	r.ssoConfig = nil
	return r, file, b
}

func ssoLoadRequest(r *Router, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(`{"id":"new","name":"New provider","type":"oidc","enabled":false,"oidc":{"clientId":"new-client"}}`))
	if path == "/api/security/sso/providers" {
		r.handleSSOProviders(w, req)
	} else {
		r.handleSSOProvider(w, req)
	}
	return w
}

func requireSSOLoadUnavailable(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Contains(t, []int{http.StatusServiceUnavailable, http.StatusConflict}, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"sso_config_unavailable"`)
	require.NotContains(t, w.Body.String(), "synthetic")
	require.NotContains(t, w.Body.String(), "sso.enc")
	require.NotContains(t, w.Body.String(), `"providers"`)
}

func TestSSOLoadFailureReadAndRecovery(t *testing.T) {
	for _, mode := range []string{"unreadable", "malformed", "decrypt"} {
		t.Run(mode, func(t *testing.T) {
			r, file, saved := ssoLoadFixture(t)
			switch mode {
			case "unreadable":
				require.NoError(t, os.Remove(file))
				require.NoError(t, os.Mkdir(file, 0700))
			case "malformed":
				require.NoError(t, os.WriteFile(file, []byte(`{"providers":`), 0600))
			case "decrypt":
				require.NoError(t, os.WriteFile(file, []byte("invalid-encrypted-configuration"), 0600))
			}
			requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers"))
			requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers/saved"))
			requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers/not-saved"))
			require.True(t, r.ssoAuthenticationLoadFailed())
			fallback := r.currentSSOConfig()
			require.Empty(t, fallback.Providers)
			if mode == "unreadable" {
				require.NoError(t, os.Remove(file))
			}
			require.NoError(t, os.WriteFile(file, saved, 0600))
			// A write must not silently retry the load or replace intact saved authority.
			for _, op := range []struct{ method, path string }{{http.MethodPost, "/api/security/sso/providers"}, {http.MethodPut, "/api/security/sso/providers/saved"}, {http.MethodDelete, "/api/security/sso/providers/saved"}} {
				requireSSOLoadUnavailable(t, ssoLoadRequest(r, op.method, op.path))
			}
			before, _ := os.ReadFile(file)
			require.Equal(t, saved, before)
			require.Same(t, fallback, r.currentSSOConfig())
			require.True(t, r.ssoAuthenticationLoadFailed())
			// Only an explicit management read recovers from a valid persisted config.
			w := ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), `"id":"saved"`)
			require.NotContains(t, w.Body.String(), "synthetic-retained-secret")
			require.False(t, r.ssoAuthenticationLoadFailed())
			require.Empty(t, fallback.Providers, "already published fallback mutated")
			require.Equal(t, "synthetic-retained-secret", r.currentSSOConfig().GetProvider("saved").OIDC.ClientSecret)
			require.Equal(t, http.StatusOK, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers/saved").Code)
			// A subsequent independent add preserves saved provider, secret and policy.
			require.Equal(t, http.StatusCreated, ssoLoadRequest(r, http.MethodPost, "/api/security/sso/providers").Code)
			disk, err := config.NewConfigPersistence(filepath.Dir(file)).LoadSSOConfig()
			require.NoError(t, err)
			require.Equal(t, r.currentSSOConfig(), disk)
			require.NotNil(t, disk.GetProvider("saved"))
			require.NotNil(t, disk.GetProvider("new"))
			require.Equal(t, "synthetic-retained-secret", disk.GetProvider("saved").OIDC.ClientSecret)
			require.Equal(t, []string{"ops"}, disk.GetProvider("saved").AllowedGroups)
			require.Equal(t, map[string]string{"ops": "viewer"}, disk.GetProvider("saved").GroupRoleMappings)
		})
	}
}

func TestSSOLoadFailureMissingIsNotRecovery(t *testing.T) {
	r, file, _ := ssoLoadFixture(t)
	require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
	requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers"))
	require.NoError(t, os.Remove(file))
	requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers"))
	requireSSOLoadUnavailable(t, ssoLoadRequest(r, http.MethodPost, "/api/security/sso/providers"))
	_, err := os.Stat(file)
	require.True(t, os.IsNotExist(err), "write recreated lost state")
	// An explicitly persisted, readable empty config differs from absent bytes.
	require.NoError(t, r.persistence.SaveSSOConfig(config.NewSSOConfig()))
	require.Equal(t, http.StatusOK, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers").Code)
	require.False(t, r.ssoAuthenticationLoadFailed())
}

func TestSSOLoadFailureFirstRunAndValidControls(t *testing.T) {
	for _, mode := range []string{"missing", "valid", "valid-empty"} {
		t.Run(mode, func(t *testing.T) {
			r, file, _ := ssoLoadFixture(t)
			if mode == "missing" {
				require.NoError(t, os.Remove(file))
			}
			if mode == "valid-empty" {
				require.NoError(t, r.persistence.SaveSSOConfig(config.NewSSOConfig()))
			}
			w := ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers")
			require.Equal(t, http.StatusOK, w.Code)
			require.False(t, r.ssoAuthenticationLoadFailed())
			var data SSOProvidersListResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &data))
			if mode == "valid" {
				require.Len(t, data.Providers, 1)
			} else {
				require.NotNil(t, data.Providers)
				require.Empty(t, data.Providers)
			}
			require.Equal(t, http.StatusCreated, ssoLoadRequest(r, http.MethodPost, "/api/security/sso/providers").Code)
		})
	}
}

func TestSSOLoadFailureAllPersistenceEntrypoints(t *testing.T) {
	for _, op := range []string{"commit", "save", "persist", "extension"} {
		t.Run(op, func(t *testing.T) {
			r, file, saved := ssoLoadFixture(t)
			require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
			r.ensureSSOConfig()
			require.True(t, r.ssoAuthenticationLoadFailed())
			require.NoError(t, os.WriteFile(file, saved, 0600))
			mutated := false
			var err error
			switch op {
			case "commit":
				var applyErr error
				applyErr, err = r.commitSSOConfig(func(*config.SSOConfig) error { mutated = true; return nil })
				require.NoError(t, applyErr)
			case "save":
				err = r.saveSSOConfig()
			case "persist":
				r.ssoMu.Lock()
				err = r.persistSSOConfigLocked(config.NewSSOConfig())
				r.ssoMu.Unlock()
			case "extension":
				err = newSSOAdminRuntime(r).SaveSSOConfigSnapshot(extensions.SSOConfigSnapshot{})
			}
			require.ErrorIs(t, err, errSSOConfigUnavailable)
			require.False(t, mutated)
			after, e := os.ReadFile(file)
			require.NoError(t, e)
			require.True(t, bytes.Equal(saved, after))
			require.True(t, r.ssoAuthenticationLoadFailed())
		})
	}
}

func TestSSOLoadFailureConcurrentReadAndWrite(t *testing.T) {
	r, file, saved := ssoLoadFixture(t)
	require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
	r.ensureSSOConfig()
	require.NoError(t, os.WriteFile(file, saved, 0600))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				w := ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers")
				if w.Code != http.StatusOK {
					t.Errorf("read status %d", w.Code)
				}
			} else {
				applyErr, err := r.commitSSOConfig(func(cfg *config.SSOConfig) error {
					if cfg.GetProvider("saved") == nil {
						return errors.New("saved authority absent at mutation")
					}
					return nil
				})
				if applyErr != nil || err != nil && !errors.Is(err, errSSOConfigUnavailable) {
					t.Errorf("commit: %v %v", applyErr, err)
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
	disk, err := r.persistence.LoadSSOConfig()
	require.NoError(t, err)
	require.NotNil(t, disk.GetProvider("saved"))
	require.Equal(t, "synthetic-retained-secret", disk.GetProvider("saved").OIDC.ClientSecret)
}

// A bound endpoint may initialize live services before it saves. It must not
// even be called while saved authority is unknown, not just fail at persistence.
type ssoLoadEndpointProbe struct {
	extensions.SSOAdminEndpoints
	collections, items int
}

func (p *ssoLoadEndpointProbe) HandleProvidersCollection(w http.ResponseWriter, req *http.Request) {
	p.collections++
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p.SSOAdminEndpoints.HandleProvidersCollection(w, req)
}

func (p *ssoLoadEndpointProbe) HandleProviderItem(w http.ResponseWriter, req *http.Request) {
	p.items++
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	p.SSOAdminEndpoints.HandleProviderItem(w, req)
}

type ssoLoadObservedBody struct{ reads int }

func (b *ssoLoadObservedBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("refused request body must not be read")
}

func TestSSOLoadFailureRegisteredBoundary(t *testing.T) {
	for _, binding := range []bool{false, true} {
		for _, transport := range []string{"X-API-Token", "Authorization"} {
			t.Run(map[bool]string{false: "core", true: "bound"}[binding]+"/"+transport, func(t *testing.T) {
				fixture, file, saved := ssoLoadFixture(t)
				const full = "sso-load-full-synthetic-token.12345678"
				const read = "sso-load-read-synthetic-token.12345678"
				const monitor = "sso-load-monitor-synthetic-token.12345678"
				cfg := fixture.config
				cfg.APITokens = []config.APITokenRecord{
					newTokenRecord(t, full, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil),
					newTokenRecord(t, read, []string{config.ScopeSettingsRead}, nil),
					newTokenRecord(t, monitor, []string{config.ScopeMonitoringRead}, nil),
				}
				var probe *ssoLoadEndpointProbe
				if binding {
					SetSSOAdminEndpointsBinder(func(defaults extensions.SSOAdminEndpoints, _ extensions.SSOAdminRuntime) extensions.SSOAdminEndpoints {
						probe = &ssoLoadEndpointProbe{SSOAdminEndpoints: defaults}
						return probe
					})
					t.Cleanup(func() { SetSSOAdminEndpointsBinder(nil) })
				}
				require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
				r := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
				require.True(t, r.ssoAuthenticationLoadFailed())
				serve := func(method, path, token string) *httptest.ResponseRecorder {
					body := &ssoLoadObservedBody{}
					req := httptest.NewRequest(method, path, body)
					req.RemoteAddr = ssoTestIP()
					if token != "" {
						if transport == "Authorization" {
							req.Header.Set(transport, "Bearer "+token)
						} else {
							req.Header.Set(transport, token)
						}
					}
					w := httptest.NewRecorder()
					r.Handler().ServeHTTP(w, req)
					require.Zero(t, body.reads)
					return w
				}
				require.Equal(t, http.StatusUnauthorized, serve(http.MethodGet, "/api/security/sso/providers", "").Code)
				require.Equal(t, http.StatusForbidden, serve(http.MethodGet, "/api/security/sso/providers", monitor).Code)
				requireSSOLoadUnavailable(t, serve(http.MethodGet, "/api/security/sso/providers", full))
				requireSSOLoadUnavailable(t, serve(http.MethodGet, "/api/security/sso/providers/saved", full))
				require.NoError(t, os.WriteFile(file, saved, 0600))
				// Neither public discovery nor rejected callers can trigger recovery.
				require.Equal(t, http.StatusOK, serve(http.MethodGet, "/api/security/status", "").Code)
				require.True(t, r.ssoAuthenticationLoadFailed())
				require.Equal(t, http.StatusForbidden, serve(http.MethodGet, "/api/security/sso/providers", monitor).Code)
				require.Equal(t, http.StatusForbidden, serve(http.MethodPost, "/api/security/sso/providers", read).Code)
				for _, op := range []struct{ method, path string }{
					{http.MethodPost, "/api/security/sso/providers"},
					{http.MethodPut, "/api/security/sso/providers/saved"},
					{http.MethodDelete, "/api/security/sso/providers/saved"},
				} {
					w := serve(op.method, op.path, full)
					requireSSOLoadUnavailable(t, w)
					require.Equal(t, http.StatusConflict, w.Code, "definite refusal, not uncertain save")
				}
				if probe != nil {
					require.Zero(t, probe.collections)
					require.Zero(t, probe.items)
				}
				after, err := os.ReadFile(file)
				require.NoError(t, err)
				require.Equal(t, saved, after)
				require.True(t, r.ssoAuthenticationLoadFailed())
				w := serve(http.MethodGet, "/api/security/sso/providers", read)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Contains(t, w.Body.String(), `"id":"saved"`)
				require.NotContains(t, w.Body.String(), "synthetic-retained-secret")
				require.False(t, r.ssoAuthenticationLoadFailed())
				if probe != nil {
					require.Equal(t, 1, probe.collections)
					require.Zero(t, probe.items)
				}
				require.Equal(t, http.StatusOK, serve(http.MethodGet, "/api/security/sso/providers/saved", read).Code)
				// Permission is not broadened by a successful read.
				require.Equal(t, http.StatusForbidden, serve(http.MethodDelete, "/api/security/sso/providers/saved", read).Code)
			})
		}
	}
}

func TestSSOLoadFailureEnvironmentSurvivesRecovery(t *testing.T) {
	r, file, saved := ssoLoadFixture(t)
	t.Setenv("OIDC_ENABLED", "true")
	t.Setenv("OIDC_ISSUER_URL", "https://environment.example.test")
	t.Setenv("OIDC_CLIENT_ID", "environment-client")
	t.Setenv("OIDC_CLIENT_SECRET", "synthetic-environment-secret")
	require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
	r.ensureSSOConfig()
	fallback := r.currentSSOConfig()
	require.NotNil(t, fallback.GetProvider(config.LegacyOIDCProviderID))
	require.True(t, r.ssoAuthenticationLoadFailed())
	require.NoError(t, os.WriteFile(file, saved, 0600))
	require.Equal(t, http.StatusOK, ssoLoadRequest(r, http.MethodGet, "/api/security/sso/providers").Code)
	require.False(t, r.ssoAuthenticationLoadFailed())
	require.NotNil(t, r.currentSSOConfig().GetProvider("saved"))
	env := r.currentSSOConfig().GetProvider(config.LegacyOIDCProviderID)
	require.NotNil(t, env)
	require.True(t, env.RuntimeManaged)
	require.Equal(t, "synthetic-environment-secret", env.OIDC.ClientSecret)
	require.True(t, hasEnabledSSOProvidersForAuth(r.config))
	require.True(t, r.configTransferAuthenticationConfigured())
	after, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, saved, after)
	require.Nil(t, fallback.GetProvider("saved"), "reload mutated old fallback")
}

func TestSSOLoadFailureDoesNotAdmitAnonymousRecovery(t *testing.T) {
	fixture, file, saved := ssoLoadFixture(t)
	require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
	r := newTestRouter(t, fixture.config, nil, nil, nil, nil, "test")
	require.True(t, r.ssoAuthenticationLoadFailed())
	// Unknown saved authentication is not a first-run bootstrap invitation.
	require.Empty(t, r.bootstrapTokenHash)
	require.NoError(t, os.WriteFile(file, saved, 0600))
	for _, path := range []string{"/api/security/sso/providers", "/api/security/sso/providers/saved"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		r.Handler().ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
		require.True(t, r.ssoAuthenticationLoadFailed(), "loopback fallback triggered a reload")
	}
	after, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, saved, after)
}
