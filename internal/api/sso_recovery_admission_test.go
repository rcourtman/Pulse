package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
	"github.com/stretchr/testify/require"
)

func bindSSORecoveryProbe(t *testing.T, bound bool) **ssoLoadEndpointProbe {
	t.Helper()
	var probe *ssoLoadEndpointProbe
	if bound {
		SetSSOAdminEndpointsBinder(func(defaults extensions.SSOAdminEndpoints, _ extensions.SSOAdminRuntime) extensions.SSOAdminEndpoints {
			probe = &ssoLoadEndpointProbe{SSOAdminEndpoints: defaults}
			return probe
		})
		t.Cleanup(func() { SetSSOAdminEndpointsBinder(nil) })
	}
	return &probe
}

func TestSSOLoadFailureDoesNotMigrateLegacyOnRecovery(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "core", true: "bound"}[bound], func(t *testing.T) {
			fixture, file, saved := ssoLoadFixture(t)
			const token = "sso-legacy-recovery-synthetic-token.12345678"
			fixture.config.APITokens = []config.APITokenRecord{newTokenRecord(t, token, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)}
			legacyFile := filepath.Join(filepath.Dir(file), "oidc.enc")
			legacy := []byte(`{"enabled":true,"issuerUrl":"https://obsolete.example.test","clientId":"obsolete","clientSecret":"synthetic-obsolete-secret","allowedGroups":["obsolete-admins"]}`)
			require.NoError(t, os.WriteFile(legacyFile, legacy, 0600))
			require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
			probe := bindSSORecoveryProbe(t, bound)
			r := newTestRouter(t, fixture.config, nil, nil, nil, nil, "test")
			require.True(t, r.ssoAuthenticationLoadFailed())
			serve := func(method, path string) *httptest.ResponseRecorder {
				body := &ssoLoadObservedBody{}
				req := httptest.NewRequest(method, path, body)
				req.RemoteAddr = ssoTestIP()
				req.Header.Set("X-API-Token", token)
				w := httptest.NewRecorder()
				r.Handler().ServeHTTP(w, req)
				require.Zero(t, body.reads, "no refused operation may consume its body")
				return w
			}
			requireSSOLoadUnavailable(t, serve(http.MethodGet, "/api/security/sso/providers"))
			require.NoError(t, os.Remove(file))
			// Missing modern state must not revive or persist older authority.
			for _, op := range []struct{ method, path string }{
				{http.MethodGet, "/api/security/sso/providers"},
				{http.MethodGet, "/api/security/sso/providers/saved"},
				{http.MethodPost, "/api/security/sso/providers"},
				{http.MethodPut, "/api/security/sso/providers/saved"},
				{http.MethodDelete, "/api/security/sso/providers/saved"},
			} {
				requireSSOLoadUnavailable(t, serve(op.method, op.path))
			}
			require.True(t, r.ssoAuthenticationLoadFailed())
			_, err := os.Stat(file)
			require.True(t, os.IsNotExist(err), "legacy migration recreated missing saved authority")
			if *probe != nil {
				require.Zero(t, (*probe).collections)
				require.Zero(t, (*probe).items)
			}
			require.NoError(t, os.WriteFile(file, saved, 0600))
			require.Equal(t, http.StatusOK, serve(http.MethodGet, "/api/security/sso/providers").Code)
			require.False(t, r.ssoAuthenticationLoadFailed())
			current := r.currentSSOConfig()
			require.Nil(t, current.GetProvider(config.LegacyOIDCProviderID))
			provider := current.GetProvider("saved")
			require.NotNil(t, provider)
			require.Equal(t, "synthetic-retained-secret", provider.OIDC.ClientSecret)
			require.Equal(t, []string{"ops"}, provider.AllowedGroups)
			require.Equal(t, map[string]string{"ops": "viewer"}, provider.GroupRoleMappings)
			after, err := os.ReadFile(file)
			require.NoError(t, err)
			require.Equal(t, saved, after, "recovery changed restored encrypted bytes")
			afterLegacy, err := os.ReadFile(legacyFile)
			require.NoError(t, err)
			require.Equal(t, legacy, afterLegacy)
		})
	}
}

type ssoRecoveryPausedRequestKey struct{}

type ssoRecoveryPauseAuthorizer struct {
	paused, resume chan struct{}
}

func (p *ssoRecoveryPauseAuthorizer) Authorize(ctx context.Context, _, _ string) (bool, error) {
	if pause, _ := ctx.Value(ssoRecoveryPausedRequestKey{}).(bool); pause {
		close(p.paused) // RequirePermission's CheckAuth has already admitted it.
		<-p.resume
	}
	return true, nil
}

func TestSSOLoadFailureAnonymousAdmissionCannotSurviveRecovery(t *testing.T) {
	for _, bound := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, op := range []struct{ name, method, path string }{
				{"list", http.MethodGet, "/api/security/sso/providers"},
				{"detail", http.MethodGet, "/api/security/sso/providers/saved"},
				{"create", http.MethodPost, "/api/security/sso/providers"},
				{"update", http.MethodPut, "/api/security/sso/providers/saved"},
				{"delete", http.MethodDelete, "/api/security/sso/providers/saved"},
			} {
				name := map[bool]string{false: "core", true: "bound"}[bound] + "/" + map[bool]string{false: "disabled", true: "enabled"}[enabled] + "/" + op.name
				t.Run(name, func(t *testing.T) {
					fixture, file, _ := ssoLoadFixture(t)
					savedConfig, err := fixture.persistence.LoadSSOConfig()
					require.NoError(t, err)
					savedConfig.GetProvider("saved").Enabled = enabled
					require.NoError(t, fixture.persistence.SaveSSOConfig(savedConfig))
					saved, err := os.ReadFile(file)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(file, []byte("invalid"), 0600))
					probe := bindSSORecoveryProbe(t, bound)
					pause := &ssoRecoveryPauseAuthorizer{paused: make(chan struct{}), resume: make(chan struct{})}
					original := auth.GetAuthorizer()
					auth.SetAuthorizer(pause)
					t.Cleanup(func() { auth.SetAuthorizer(original) })
					r := newTestRouter(t, fixture.config, nil, nil, nil, nil, "test")
					require.True(t, r.ssoAuthenticationLoadFailed())
					// An existing actual admin session, without any local password or
					// token inventory. No fake SSO credential or wider grant is added.
					const raw = "synthetic-previously-admitted-sso-admin-session"
					const user = "sso:oidc:saved:recovery-admin"
					require.NotNil(t, auth.GetManager())
					require.NoError(t, auth.GetManager().UpdateUserRoles(user, []string{auth.RoleAdmin}))
					r.sessionStore.CreateSession(raw, time.Hour, "fixture", "127.0.0.1", user)
					body := &ssoLoadObservedBody{}
					req := httptest.NewRequest(op.method, op.path, body)
					req.RemoteAddr = "127.0.0.1:1234"
					req.Host = "localhost"
					// A mutable identity-looking header is deliberately not admission.
					w := httptest.NewRecorder()
					w.Header().Set("X-Authenticated-User", user)
					w.Header().Set("X-Auth-Method", "session")
					req = req.WithContext(context.WithValue(req.Context(), ssoRecoveryPausedRequestKey{}, true))
					done := make(chan struct{})
					var resumeOnce sync.Once
					resume := func() { resumeOnce.Do(func() { close(pause.resume) }) }
					t.Cleanup(func() { resume(); <-done })
					go func() { defer close(done); r.Handler().ServeHTTP(w, req) }()
					select {
					case <-pause.paused:
					case <-time.After(10 * time.Second):
						t.Fatal("request did not reach post-auth pause")
					}
					require.Equal(t, "none", w.Header().Get("X-Auth-Method"))
					require.True(t, r.ssoAuthenticationLoadFailed())
					require.NoError(t, os.WriteFile(file, saved, 0600))
					admin := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
					admin.RemoteAddr = "127.0.0.1:4321"
					admin.Host = "localhost"
					admin.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
					recovered := httptest.NewRecorder()
					r.Handler().ServeHTTP(recovered, admin)
					require.Equal(t, http.StatusOK, recovered.Code, recovered.Body.String())
					require.Equal(t, "session", recovered.Header().Get("X-Auth-Method"))
					require.False(t, r.ssoAuthenticationLoadFailed())
					require.Equal(t, enabled, hasEnabledSSOProvidersForAuth(r.config))
					require.Contains(t, recovered.Body.String(), `"id":"saved"`)
					require.NotContains(t, recovered.Body.String(), "synthetic-retained-secret")
					// Middleware cannot turn fallback admission into identity by
					// replacing the response headers while this request is paused.
					w.Header().Set("X-Authenticated-User", user)
					w.Header().Set("X-Auth-Method", "session")
					resume()
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						t.Fatal("paused request did not finish")
					}
					require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
					require.Zero(t, body.reads, "stale anonymous mutation read its body")
					require.NotContains(t, w.Body.String(), "saved-client")
					if *probe != nil {
						require.Equal(t, 1, (*probe).collections, "only the admin recovery read may reach the bound endpoint")
						require.Zero(t, (*probe).items)
					}
					after, err := os.ReadFile(file)
					require.NoError(t, err)
					require.Equal(t, saved, after)
					require.Equal(t, savedConfig, r.currentSSOConfig(), "anonymous operation changed recovered authority")
				})
			}
		}
	}
}

func TestSSOManagementRecordsActualAuthentication(t *testing.T) {
	t.Setenv("PULSE_TEST_BCRYPT_COST", "4")
	InitSessionStore(t.TempDir())
	const token = "synthetic-sso-admission-token.12345678"
	const session = "synthetic-sso-admission-session"
	GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "operator")
	hash, err := auth.HashPassword("synthetic-admission-password")
	require.NoError(t, err)
	for _, mode := range []string{"token", "bearer", "session", "proxy", "basic", "anonymous", "invalid-explicit-token"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &config.Config{}
			req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
			req.RemoteAddr = "127.0.0.1:1234"
			req.Host = "localhost"
			switch mode {
			case "token", "bearer":
				cfg.APITokens = []config.APITokenRecord{newTokenRecord(t, token, []string{config.ScopeSettingsRead}, nil)}
				if mode == "token" {
					req.Header.Set("X-API-Token", token)
				} else {
					req.Header.Set("Authorization", "Bearer "+token)
				}
			case "session", "invalid-explicit-token":
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: session})
				if mode == "invalid-explicit-token" {
					req.Header.Set("X-API-Token", "invalid")
				}
			case "proxy":
				cfg.ProxyAuthSecret = "synthetic-proxy-secret"
				cfg.ProxyAuthUserHeader = "X-Remote-User"
				req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
				req.Header.Set("X-Remote-User", "operator")
			case "basic":
				cfg.AuthUser, cfg.AuthPass = "operator", hash
				req.SetBasicAuth("operator", "synthetic-admission-password")
			}
			admission := &ssoManagementAdmission{unknown: true, authenticated: true}
			req = req.WithContext(context.WithValue(req.Context(), ssoManagementAdmissionKey{}, admission))
			w := httptest.NewRecorder()
			w.Header().Set("X-Auth-Method", "session")
			require.Equal(t, mode != "invalid-explicit-token", CheckAuth(cfg, w, req))
			require.Equal(t, mode != "anonymous" && mode != "invalid-explicit-token", admission.authenticated)
			// A second failed authentication cannot retain the earlier admission.
			req.Header.Set("X-API-Token", "invalid")
			req.Header.Del("X-Proxy-Secret")
			require.False(t, CheckAuth(cfg, httptest.NewRecorder(), req))
			require.False(t, admission.authenticated)
		})
	}
}

func TestSSOLoadFailureKnownFirstRunKeepsLoopbackManagement(t *testing.T) {
	fixture, file, _ := ssoLoadFixture(t)
	require.NoError(t, os.Remove(file))
	r := newTestRouter(t, fixture.config, nil, nil, nil, nil, "test")
	for _, op := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/api/security/sso/providers", "", http.StatusOK},
		{http.MethodPost, "/api/security/sso/providers", `{"id":"new","name":"New","type":"oidc","enabled":false,"oidc":{"clientId":"new"}}`, http.StatusCreated},
		{http.MethodGet, "/api/security/sso/providers/new", "", http.StatusOK},
		{http.MethodPut, "/api/security/sso/providers/new", `{"name":"Renamed","enabled":false}`, http.StatusOK},
		{http.MethodDelete, "/api/security/sso/providers/new", "", http.StatusNoContent},
		{http.MethodGet, "/api/security/sso/providers/new", "", http.StatusNotFound},
	} {
		req := httptest.NewRequest(op.method, op.path, strings.NewReader(op.body))
		req.RemoteAddr = "127.0.0.1:1234"
		req.Host = "localhost"
		w := httptest.NewRecorder()
		r.Handler().ServeHTTP(w, req)
		require.Equal(t, op.status, w.Code, w.Body.String())
		require.False(t, r.ssoAuthenticationLoadFailed())
	}
	require.Empty(t, r.currentSSOConfig().Providers)
}
