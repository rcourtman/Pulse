package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type countingConfigTransferBody struct {
	reader io.Reader
	reads  int
}

func (b *countingConfigTransferBody) Read(p []byte) (int, error) {
	b.reads++
	return b.reader.Read(p)
}

func newConfigTransferTestRouter(t *testing.T, hosted bool, sso *config.SSOConfig) *Router {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	if hosted {
		t.Setenv("PULSE_HOSTED_MODE", "true")
	} else {
		t.Setenv("PULSE_HOSTED_MODE", "false")
	}

	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir}
	if sso != nil {
		if err := config.NewConfigPersistence(dataDir).SaveSSOConfig(sso); err != nil {
			t.Fatalf("save synthetic SSO config: %v", err)
		}
	}
	return NewRouter(cfg, nil, nil, nil, nil, "test")
}

func enabledConfigTransferSSO(providerType config.SSOProviderType) *config.SSOConfig {
	provider := config.SSOProvider{ID: "synthetic", Name: "Synthetic", Type: providerType, Enabled: true}
	if providerType == config.SSOProviderTypeOIDC {
		provider.OIDC = &config.OIDCProviderConfig{IssuerURL: "https://idp.invalid", ClientID: "synthetic"}
	} else {
		provider.SAML = &config.SAMLProviderConfig{IDPEntityID: "https://idp.invalid"}
	}
	return &config.SSOConfig{Providers: []config.SSOProvider{provider}}
}

func configTransferRequest(t *testing.T, router *Router, path, remoteAddr, body string) (*httptest.ResponseRecorder, *countingConfigTransferBody) {
	t.Helper()
	counted := &countingConfigTransferBody{reader: strings.NewReader(body)}
	req := httptest.NewRequest(http.MethodPost, path, counted)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	return rec, counted
}

func TestConfigTransferAnonymousAuthenticatedModesDenyBeforeBodyRead(t *testing.T) {
	tests := []struct {
		name   string
		hosted bool
		sso    *config.SSOConfig
	}{
		{name: "persisted OIDC", sso: enabledConfigTransferSSO(config.SSOProviderTypeOIDC)},
		{name: "persisted SAML", sso: enabledConfigTransferSSO(config.SSOProviderTypeSAML)},
		{name: "hosted", hosted: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := newConfigTransferTestRouter(t, tc.hosted, tc.sso)
			requests := []struct {
				path string
				body string
			}{
				{path: "/api/config/export", body: `{not-json`},
				{path: "/api/config/export", body: `{"passphrase":"synthetic-passphrase"}`},
				{path: "/api/config/import", body: `{not-json`},
				{path: "/api/config/import", body: `{"passphrase":"synthetic-passphrase","data":"synthetic-archive"}`},
			}
			for i, request := range requests {
				remoteAddr := "127.0.0." + string(rune('1'+i)) + ":1234"
				rec, body := configTransferRequest(t, router, request.path, remoteAddr, request.body)
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("%s status = %d, want 401: %s", request.path, rec.Code, rec.Body.String())
				}
				if body.reads != 0 {
					t.Errorf("%s read denied request body %d times", request.path, body.reads)
				}
			}
		})
	}
}

func TestConfigTransferEnvironmentOIDCAndSSOLoadFailureFailClosed(t *testing.T) {
	t.Run("environment OIDC", func(t *testing.T) {
		t.Setenv("OIDC_ENABLED", "true")
		t.Setenv("OIDC_ISSUER_URL", "https://idp.invalid")
		t.Setenv("OIDC_CLIENT_ID", "synthetic")
		router := newConfigTransferTestRouter(t, false, nil)
		for _, path := range []string{"/api/config/export", "/api/config/import"} {
			rec, body := configTransferRequest(t, router, path, "127.0.0.1:1234", `{not-json`)
			if rec.Code != http.StatusUnauthorized || body.reads != 0 {
				t.Errorf("%s environment OIDC denial = status %d, reads %d", path, rec.Code, body.reads)
			}
		}
	})

	t.Run("unreadable persisted SSO", func(t *testing.T) {
		dataDir := t.TempDir()
		t.Setenv("PULSE_DATA_DIR", dataDir)
		t.Setenv("PULSE_HOSTED_MODE", "false")
		// Establish the synthetic encryption key before introducing a corrupt
		// encrypted payload; production persistence is never consulted.
		config.NewConfigPersistence(dataDir)
		if err := os.WriteFile(filepath.Join(dataDir, "sso.enc"), []byte("synthetic-corruption"), 0o600); err != nil {
			t.Fatalf("write corrupt synthetic SSO: %v", err)
		}
		cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir}
		router := NewRouter(cfg, nil, nil, nil, nil, "test")
		if !router.ssoAuthenticationLoadFailed() {
			t.Fatal("precondition: corrupt persisted SSO was not recorded as a load failure")
		}
		rec, body := configTransferRequest(t, router, "/api/config/export", "127.0.0.1:1234", `{not-json`)
		if rec.Code != http.StatusUnauthorized || body.reads != 0 {
			t.Fatalf("SSO load failure denial = status %d, reads %d", rec.Code, body.reads)
		}
	})
}

func TestConfigTransferNoAuthUsesDirectLoopbackPolicy(t *testing.T) {
	t.Setenv("ALLOW_UNPROTECTED_EXPORT", "false")
	router := newConfigTransferTestRouter(t, false, nil)

	for _, path := range []string{"/api/config/export", "/api/config/import"} {
		rec, body := configTransferRequest(t, router, path, "192.168.1.50:1234", `{not-json`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s private-peer status = %d, want 403: %s", path, rec.Code, rec.Body.String())
		}
		if body.reads != 0 {
			t.Errorf("%s read private-peer denied body %d times", path, body.reads)
		}
	}

	for _, path := range []string{"/api/config/export", "/api/config/import"} {
		rec, body := configTransferRequest(t, router, path, "127.0.0.1:1234", `{not-json`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s loopback status = %d, want handler 400: %s", path, rec.Code, rec.Body.String())
		}
		if body.reads == 0 {
			t.Errorf("%s loopback request did not reach handler", path)
		}
	}
}

func TestAllowUnprotectedExportIsExportOnly(t *testing.T) {
	t.Setenv("ALLOW_UNPROTECTED_EXPORT", "true")
	router := newConfigTransferTestRouter(t, false, nil)

	exportRec, exportBody := configTransferRequest(t, router, "/api/config/export", "203.0.113.10:1234", `{not-json`)
	if exportRec.Code != http.StatusBadRequest || exportBody.reads == 0 {
		t.Fatalf("unprotected export did not reach handler: status=%d reads=%d body=%s", exportRec.Code, exportBody.reads, exportRec.Body.String())
	}

	importRec, importBody := configTransferRequest(t, router, "/api/config/import", "203.0.113.10:1234", `{not-json`)
	if importRec.Code != http.StatusForbidden {
		t.Fatalf("import status = %d, want 403: %s", importRec.Code, importRec.Body.String())
	}
	if importBody.reads != 0 {
		t.Fatalf("import override denial read body %d times", importBody.reads)
	}
}

func TestAllowUnprotectedExportCannotOverrideAuthentication(t *testing.T) {
	t.Setenv("ALLOW_UNPROTECTED_EXPORT", "true")
	router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
	for _, path := range []string{"/api/config/export", "/api/config/import"} {
		rec, body := configTransferRequest(t, router, path, "203.0.113.12:1234", `{not-json`)
		if rec.Code != http.StatusUnauthorized || body.reads != 0 {
			t.Errorf("%s authenticated override denial = status %d, reads %d", path, rec.Code, body.reads)
		}
	}
}

func TestConfigTransferNoAuthRejectsForwardedLoopback(t *testing.T) {
	t.Setenv("ALLOW_UNPROTECTED_EXPORT", "false")
	router := newConfigTransferTestRouter(t, false, nil)
	for _, header := range []string{"X-Forwarded-For", "Forwarded", "X-Real-IP"} {
		body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", body)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set(header, "127.0.0.1")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || body.reads != 0 {
			t.Errorf("%s forwarded loopback = status %d, reads %d", header, rec.Code, body.reads)
		}
	}
}

func TestConfigTransferAuthorizedInstanceModesReachHandler(t *testing.T) {
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	t.Run("basic admin", func(t *testing.T) {
		dataDir := t.TempDir()
		t.Setenv("PULSE_DATA_DIR", dataDir)
		cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, AuthUser: "admin", AuthPass: hash}
		router := NewRouter(cfg, nil, nil, nil, nil, "test")
		for _, path := range []string{"/api/config/export", "/api/config/import"} {
			body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
			req := httptest.NewRequest(http.MethodPost, path, body)
			req.SetBasicAuth("admin", "SyntheticPassword!1")
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || body.reads == 0 {
				t.Errorf("%s basic admin = status %d, reads %d", path, rec.Code, body.reads)
			}
		}
	})

	for _, providerType := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run("SSO session "+string(providerType), func(t *testing.T) {
			router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(providerType))
			manager := installTestRBACManager(t)
			const ssoAdmin = "sso:owner@example.invalid"
			if err := manager.UpdateUserRoles(ssoAdmin, []string{internalauth.RoleAdmin}); err != nil {
				t.Fatalf("assign SSO administrator role: %v", err)
			}
			sessionToken := "synthetic-sso-session-" + string(providerType)
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", ssoAdmin)
			csrf := generateCSRFToken(sessionToken)
			for _, path := range []string{"/api/config/export", "/api/config/import"} {
				body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
				req := httptest.NewRequest(http.MethodPost, path, body)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				req.Header.Set("X-CSRF-Token", csrf)
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest || body.reads == 0 {
					t.Errorf("%s SSO admin = status %d, reads %d", path, rec.Code, body.reads)
				}
			}
		})
	}

	t.Run("proxy admin", func(t *testing.T) {
		dataDir := t.TempDir()
		t.Setenv("PULSE_DATA_DIR", dataDir)
		cfg := &config.Config{
			DataPath: dataDir, ConfigPath: dataDir,
			ProxyAuthSecret: "synthetic-proxy-secret", ProxyAuthUserHeader: "X-Proxy-User",
			ProxyAuthRoleHeader: "X-Proxy-Roles", ProxyAuthAdminRole: "admin",
		}
		router := NewRouter(cfg, nil, nil, nil, nil, "test")
		for _, path := range []string{"/api/config/export", "/api/config/import"} {
			body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
			req := httptest.NewRequest(http.MethodPost, path, body)
			req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
			req.Header.Set("X-Proxy-User", "proxy-admin")
			req.Header.Set("X-Proxy-Roles", "viewer|admin")
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || body.reads == 0 {
				t.Errorf("%s proxy admin = status %d, reads %d", path, rec.Code, body.reads)
			}
		}
	})
}

func TestConfigTransferTokenScopesAndOrganizationBinding(t *testing.T) {
	readRaw := "synthetic-read-token-123.12345678"
	writeRaw := "synthetic-write-token-123.12345678"
	readRecord := newTokenRecord(t, readRaw, []string{config.ScopeSettingsRead}, nil)
	writeRecord := newTokenRecord(t, writeRaw, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, readRecord, writeRecord)
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")

	tests := []struct {
		name   string
		path   string
		token  string
		bearer bool
		status int
		reads  bool
	}{
		{name: "read scope exports", path: "/api/config/export", token: readRaw, status: http.StatusBadRequest, reads: true},
		{name: "read scope cannot import", path: "/api/config/import", token: readRaw, status: http.StatusForbidden},
		{name: "write scope imports with bearer", path: "/api/config/import", token: writeRaw, bearer: true, status: http.StatusBadRequest, reads: true},
		{name: "write scope cannot export", path: "/api/config/export", token: writeRaw, status: http.StatusForbidden},
		{name: "invalid token", path: "/api/config/export", token: "invalid-synthetic-token", status: http.StatusUnauthorized},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
			req := httptest.NewRequest(http.MethodPost, tc.path, body)
			req.RemoteAddr = "127.0.1." + string(rune('1'+i)) + ":1234"
			if tc.bearer {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			} else {
				req.Header.Set("X-API-Token", tc.token)
			}
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if (body.reads > 0) != tc.reads {
				t.Fatalf("body reads = %d, want reached=%v", body.reads, tc.reads)
			}
		})
	}

	boundRaw := "synthetic-bound-token-123.12345678"
	boundRecord := newTokenRecord(t, boundRaw, []string{config.ScopeSettingsRead}, nil)
	boundRecord.OrgID = "tenant-a"
	boundCfg := newTestConfigWithTokens(t, boundRecord)
	t.Setenv("PULSE_DATA_DIR", boundCfg.DataPath)
	boundRouter := NewRouter(boundCfg, nil, nil, nil, nil, "test")
	body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
	req := httptest.NewRequest(http.MethodPost, "/api/config/export", body)
	req.Header.Set("X-API-Token", boundRaw)
	req.Header.Set("X-Pulse-Org-ID", "default")
	rec := httptest.NewRecorder()
	boundRouter.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || body.reads != 0 {
		t.Fatalf("cross-organization token denial = status %d, reads %d", rec.Code, body.reads)
	}
}

func TestConfigTransferSSOViewerDeniedBeforeBodyRead(t *testing.T) {
	for _, providerType := range []config.SSOProviderType{config.SSOProviderTypeOIDC, config.SSOProviderTypeSAML} {
		t.Run(string(providerType), func(t *testing.T) {
			router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(providerType))
			installTestRBACManager(t)
			sessionToken := "synthetic-sso-viewer-session-" + string(providerType)
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", "sso:viewer@example.invalid")
			csrf := generateCSRFToken(sessionToken)

			for _, path := range []string{"/api/config/export", "/api/config/import"} {
				body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
				req := httptest.NewRequest(http.MethodPost, path, body)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				req.Header.Set("X-CSRF-Token", csrf)
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s SSO viewer = status %d, want 403: %s", path, rec.Code, rec.Body.String())
				}
				if body.reads != 0 {
					t.Errorf("%s SSO viewer denial read body %d times", path, body.reads)
				}
			}
		})
	}
}

func TestConfigTransferTenantSessionsRequireManagement(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	tenantTokenRaw := "synthetic-tenant-token-123.12345678"
	tenantToken := newTokenRecord(t, tenantTokenRaw, []string{config.ScopeSettingsRead}, nil)
	tenantToken.OrgID = "tenant-a"
	cfg := &config.Config{
		DataPath: dataDir, ConfigPath: dataDir, AuthUser: "instance-admin", AuthPass: hash,
		APITokens: []config.APITokenRecord{tenantToken},
	}
	org := &models.Organization{
		ID: "tenant-a", DisplayName: "Tenant A", OwnerUserID: "owner",
		Members: []models.OrganizationMember{
			{UserID: "owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
			{UserID: "manager", Role: models.OrgRoleAdmin, AddedAt: time.Now()},
			{UserID: "viewer", Role: models.OrgRoleViewer, AddedAt: time.Now()},
		},
	}
	mtp := config.NewMultiTenantPersistence(dataDir)
	if err := mtp.SaveOrganization(org); err != nil {
		t.Fatalf("save synthetic organization: %v", err)
	}
	if err := mtp.SaveOrganization(&models.Organization{ID: "tenant-b", DisplayName: "Tenant B", OwnerUserID: "other-owner"}); err != nil {
		t.Fatalf("save cross-organization fixture: %v", err)
	}
	mtm := monitoring.NewMultiTenantMonitor(cfg, mtp, nil)
	t.Cleanup(mtm.Stop)
	router := NewRouter(cfg, nil, mtm, nil, nil, "test")

	tests := []struct {
		user    string
		status  int
		reached bool
	}{
		{user: "viewer", status: http.StatusForbidden},
		{user: "manager", status: http.StatusBadRequest, reached: true},
		{user: "owner", status: http.StatusBadRequest, reached: true},
	}
	for i, tc := range tests {
		t.Run(tc.user, func(t *testing.T) {
			sessionToken := "tenant-transfer-session-" + tc.user
			GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", tc.user)
			body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
			req := httptest.NewRequest(http.MethodPost, "/api/config/import", body)
			req.RemoteAddr = "127.0.2." + string(rune('1'+i)) + ":1234"
			req.Header.Set("X-Pulse-Org-ID", "tenant-a")
			req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if (body.reads > 0) != tc.reached {
				t.Fatalf("body reads = %d, want reached=%v", body.reads, tc.reached)
			}
		})
	}

	t.Run("organization-bound token selects and reaches its tenant", func(t *testing.T) {
		body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
		req := httptest.NewRequest(http.MethodPost, "/api/config/export", body)
		req.Header.Set("X-API-Token", tenantTokenRaw)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || body.reads == 0 {
			t.Fatalf("bound tenant token = status %d, reads %d: %s", rec.Code, body.reads, rec.Body.String())
		}
	})

	t.Run("organization-bound token cannot cross tenants", func(t *testing.T) {
		body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
		req := httptest.NewRequest(http.MethodPost, "/api/config/export", body)
		req.Header.Set("X-API-Token", tenantTokenRaw)
		req.Header.Set("X-Pulse-Org-ID", "tenant-b")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || body.reads != 0 {
			t.Fatalf("cross-tenant token = status %d, reads %d: %s", rec.Code, body.reads, rec.Body.String())
		}
	})
}

func TestSecurityStatusMatchesConfigTransferPolicy(t *testing.T) {
	t.Run("hosted requires authentication", func(t *testing.T) {
		router := newConfigTransferTestRouter(t, true, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		var status map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode hosted security status: %v", err)
		}
		if status["requiresAuth"] != true || status["hasAuthentication"] != true {
			t.Fatalf("hosted security status did not report required auth: %#v", status)
		}
	})

	t.Run("export override is ineffective with SSO", func(t *testing.T) {
		t.Setenv("ALLOW_UNPROTECTED_EXPORT", "true")
		router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
		manager := installTestRBACManager(t)
		const ssoAdmin = "sso:owner@example.invalid"
		if err := manager.UpdateUserRoles(ssoAdmin, []string{internalauth.RoleAdmin}); err != nil {
			t.Fatalf("assign SSO administrator role: %v", err)
		}
		sessionToken := "synthetic-security-status-session"
		GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", ssoAdmin)
		req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		var status map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode SSO security status: %v", err)
		}
		if status["detailLevel"] != securityStatusDetailPrivileged {
			t.Fatalf("security status detail = %v, want privileged: %#v", status["detailLevel"], status)
		}
		if status["exportProtected"] != true || status["unprotectedExportAllowed"] != false {
			t.Fatalf("security status misreported authenticated export override: %#v", status)
		}
	})
}

func TestDeniedConfigTransferDoesNotMutateOrReload(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{
		DataPath: dataDir, ConfigPath: dataDir,
		AuthUser: "synthetic-admin", AuthPass: hash, PublicURL: "https://before.invalid",
	}
	reloadCalls := 0
	router := NewRouter(cfg, nil, nil, nil, func() error {
		reloadCalls++
		return nil
	}, "test")

	requests := []struct {
		path string
		body string
	}{
		{path: "/api/config/export", body: `{"passphrase":"synthetic-passphrase"}`},
		{path: "/api/config/import", body: `{"passphrase":"synthetic-passphrase","data":"synthetic-archive"}`},
		{path: "/api/config/import", body: `{not-json`},
	}
	for i, request := range requests {
		body := &countingConfigTransferBody{reader: strings.NewReader(request.body)}
		req := httptest.NewRequest(http.MethodPost, request.path, body)
		req.RemoteAddr = "203.0.113." + string(rune('1'+i)) + ":1234"
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s denied status = %d: %s", request.path, rec.Code, rec.Body.String())
		}
		if body.reads != 0 {
			t.Fatalf("%s denial read request body %d times", request.path, body.reads)
		}
	}

	if reloadCalls != 0 {
		t.Fatalf("denied import reloaded runtime %d times", reloadCalls)
	}
	if cfg.PublicURL != "https://before.invalid" || cfg.AuthUser != "synthetic-admin" || cfg.AuthPass != hash {
		t.Fatalf("denied import mutated live config: %+v", cfg)
	}
}
