package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func logoutProviderFixture() config.SSOProvider {
	return config.SSOProvider{ID: "logout-fixture", Name: "Identity provider", Type: config.SSOProviderTypeOIDC,
		AllowedGroups: []string{"operators"}, GroupRoleMappings: map[string]string{"operators": "admin"},
		OIDC: &config.OIDCProviderConfig{IssuerURL: "https://identity.example.test", ClientID: "fixture", ClientSecret: "synthetic-preserved-secret", ClientSecretSet: true, LogoutURL: "https://identity.example.test/logout"}}
}

func logoutWrite(t *testing.T, router *Router, method string, enabled bool, value any) *httptest.ResponseRecorder {
	t.Helper()
	p := logoutProviderFixture()
	id, path := p.ID, "/api/security/sso/providers/"+p.ID
	if method == http.MethodPost {
		id = "new-logout-fixture"
		path = "/api/security/sso/providers"
	}
	body, err := json.Marshal(map[string]any{"id": id, "name": p.Name, "type": "oidc", "enabled": enabled, "allowedGroups": p.AllowedGroups, "groupRoleMappings": p.GroupRoleMappings, "oidc": map[string]any{"issuerUrl": p.OIDC.IssuerURL, "clientId": p.OIDC.ClientID, "logoutUrl": value}})
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	// Accepted writes must not depend on an external IdP being reachable.
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	if method == http.MethodPost {
		router.handleSSOProviders(rec, req)
	} else {
		router.handleSSOProvider(rec, req)
	}
	return rec
}

func TestSSOLogoutURLRejectsUnsafeWrites(t *testing.T) {
	for _, tt := range []struct{ name, value string }{
		{"script", "javascript:void(0)"}, {"mixed script", "JaVaScRiPt:void(0)"},
		{"data", "data:text/plain,inert"}, {"relative", "/logout"}, {"protocol relative", "//identity.example.test/logout"},
		{"userinfo", "https://fixture:synthetic-url-secret@identity.example.test/logout"},
		{"opaque", "https:logout"}, {"no host", "https:///logout"},
		{"blank", " \t "}, {"leading space", " https://identity.example.test/logout"},
		{"control", "https://identity.example.test/\nlogout"}, {"damaged escape", "https://identity.example.test/%zz"},
		{"overlong", "https://identity.example.test/" + strings.Repeat("x", 2048)},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/enabled=%t", tt.name, method, enabled), func(t *testing.T) {
					router, dir := setupTestRouter(t)
					require.NoError(t, router.ssoConfig.AddProvider(logoutProviderFixture()))
					require.NoError(t, router.saveSSOConfig())
					before := router.ensureSSOConfig().Clone()
					disk, err := os.ReadFile(filepath.Join(dir, "sso.enc"))
					require.NoError(t, err)
					rec := logoutWrite(t, router, method, enabled, tt.value)
					require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
					var failure APIError
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &failure))
					require.Equal(t, "validation_error", failure.Code)
					require.Equal(t, "Invalid OIDC logout URL", failure.ErrorMessage)
					require.NotContains(t, rec.Body.String(), "synthetic-url-secret")
					require.NotContains(t, rec.Body.String(), "javascript:")
					require.Equal(t, before, router.ensureSSOConfig(), "rejection changed published provider or credentials")
					after, err := os.ReadFile(filepath.Join(dir, "sso.enc"))
					require.NoError(t, err)
					require.Equal(t, disk, after, "rejection changed encrypted settings")
				})
			}
		}
	}
}

func TestSSOLogoutURLPreservesSupportedWrites(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
	}{
		{"https", "https://identity.example.test/logout?return=app#done"},
		{"http compatibility", "http://identity.example.test/logout"},
		{"loopback", "http://127.0.0.1:8080/logout"},
		{"ipv6", "https://[::1]:8443/logout"},
		{"uppercase", "HTTPS://identity.example.test/logout"},
		{"clear", ""}, {"null clear", nil},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/enabled=%t", tt.name, method, enabled), func(t *testing.T) {
					router, _ := setupTestRouter(t)
					require.NoError(t, router.ssoConfig.AddProvider(logoutProviderFixture()))
					require.NoError(t, router.saveSSOConfig())
					rec := logoutWrite(t, router, method, enabled, tt.value)
					status, id := http.StatusOK, "logout-fixture"
					if method == http.MethodPost {
						status, id = http.StatusCreated, "new-logout-fixture"
					}
					require.Equal(t, status, rec.Code, rec.Body.String())
					p := router.ensureSSOConfig().GetProvider(id)
					require.NotNil(t, p)
					want, _ := tt.value.(string)
					require.Equal(t, want, p.OIDC.LogoutURL)
					require.Equal(t, enabled, p.Enabled)
					require.Equal(t, []string{"operators"}, p.AllowedGroups)
					require.Equal(t, map[string]string{"operators": "admin"}, p.GroupRoleMappings)
					if method == http.MethodPut {
						require.Equal(t, "synthetic-preserved-secret", p.OIDC.ClientSecret)
					}
					require.NotContains(t, rec.Body.String(), "synthetic-preserved-secret")
					loaded, err := router.persistence.LoadSSOConfig()
					require.NoError(t, err)
					require.Equal(t, p, loaded.GetProvider(id))
				})
			}
		}
	}
}

func TestSSOLogoutURLRejectsInheritedUnsafeTargetButAllowsCorrection(t *testing.T) {
	for _, nested := range []string{"", `,"oidc":null`, `,"OIDC":null`} {
		t.Run(nested, func(t *testing.T) {
			router, _ := setupTestRouter(t)
			p := logoutProviderFixture()
			p.OIDC.LogoutURL = "javascript:void(0)"
			require.NoError(t, router.ssoConfig.AddProvider(p))
			require.NoError(t, router.saveSSOConfig())
			rec := httptest.NewRecorder()
			body := `{"name":"Identity provider","type":"oidc","enabled":false` + nested + `}`
			router.handleSSOProvider(rec, httptest.NewRequest(http.MethodPut, "/api/security/sso/providers/logout-fixture", strings.NewReader(body)))
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, p, *router.ensureSSOConfig().GetProvider(p.ID))
			require.Equal(t, http.StatusOK, logoutWrite(t, router, http.MethodPut, false, "").Code)
			require.Empty(t, router.ensureSSOConfig().GetProvider(p.ID).OIDC.LogoutURL)
			require.Equal(t, p.OIDC.ClientSecret, router.ensureSSOConfig().GetProvider(p.ID).OIDC.ClientSecret)
		})
	}
}

// Use a genuine SSO session and the registered status producer. Older saved
// settings and environment authority must not become arbitrary navigation.
func logoutStatusFixture(t *testing.T, raw string, environment bool) (*Router, string) {
	t.Helper()
	resetSessionStoreForTests()
	resetSessionTracking()
	t.Cleanup(resetSessionStoreForTests)
	t.Cleanup(resetSessionTracking)
	cfg := newTestConfigWithTokens(t)
	p := logoutProviderFixture()
	p.Enabled = true
	p.OIDC.LogoutURL = raw
	if environment {
		t.Setenv("OIDC_ENABLED", "true")
		t.Setenv("OIDC_ISSUER_URL", p.OIDC.IssuerURL)
		t.Setenv("OIDC_CLIENT_ID", p.OIDC.ClientID)
		t.Setenv("OIDC_CLIENT_SECRET", p.OIDC.ClientSecret)
		t.Setenv("OIDC_LOGOUT_URL", raw)
	} else {
		sso := config.NewSSOConfig()
		require.NoError(t, sso.AddProvider(p))
		require.NoError(t, config.NewConfigPersistence(cfg.DataPath).SaveSSOConfig(sso))
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	token := "synthetic-logout-status-session"
	GetSessionStore().CreateOIDCSessionWithDisplayName(token, time.Hour, "fixture", "127.0.0.1", "sso:oidc:logout-fixture:subject", "operator@example.test", &OIDCTokenInfo{Issuer: p.OIDC.IssuerURL, ClientID: p.OIDC.ClientID})
	return router, token
}

func TestSSOLogoutURLStatusWithholdsUnsafeLegacyAndEnvironmentTargets(t *testing.T) {
	for _, environment := range []bool{false, true} {
		for _, raw := range []string{"javascript:void(0)", "data:text/plain,inert", "/logout", "https://fixture:synthetic-url-secret@identity.example.test/logout", "https://identity.example.test/logout", "http://127.0.0.1:8080/logout", ""} {
			t.Run(fmt.Sprintf("environment=%t/%s", environment, raw), func(t *testing.T) {
				router, token := logoutStatusFixture(t, raw, environment)
				for _, authenticated := range []bool{false, true} {
					req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
					if authenticated {
						req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: token})
					}
					rec := httptest.NewRecorder()
					router.Handler().ServeHTTP(rec, req)
					require.Equal(t, http.StatusOK, rec.Code)
					var payload map[string]any
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
					want := authenticated && (raw == "https://identity.example.test/logout" || raw == "http://127.0.0.1:8080/logout")
					_, hasTarget := payload["ssoLogoutURL"]
					require.Equal(t, want, hasTarget)
					if want {
						require.Equal(t, raw, payload["ssoLogoutURL"])
					}
					if authenticated {
						require.Equal(t, "operator@example.test", payload["ssoSessionDisplayName"])
					}
					require.NotContains(t, rec.Body.String(), "synthetic-url-secret")
				}
				require.True(t, ValidateSession(token), "status filtering invalidated administrator access")
				id := "logout-fixture"
				if environment {
					id = config.LegacyOIDCProviderID
				}
				require.Equal(t, raw, router.ensureSSOConfig().GetProvider(id).OIDC.LogoutURL, "status rewrote configuration authority")
				if environment {
					_, err := os.Stat(filepath.Join(router.config.DataPath, "sso.enc"))
					require.True(t, os.IsNotExist(err), "environment provider reached disk")
				}
			})
		}
	}
}
