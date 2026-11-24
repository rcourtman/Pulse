package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	return router
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
	req.Host = "localhost"
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
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
		cleanupTestRouter(t, router)
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
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
		cleanupTestRouter(t, router)
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
		router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
		cleanupTestRouter(t, router)
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

// Export and import test the administrator password outside CheckAuth. They
// must share sign-in's lockout, or they answer password guesses without limit
// to a caller with no credential at all.
func TestConfigTransferBasicPasswordGuessesMeetSignInLockout(t *testing.T) {
	const password = "SyntheticPassword!1"
	hash, err := internalauth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir, AuthUser: "admin", AuthPass: hash}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	resetFailedLogins()
	t.Cleanup(resetFailedLogins)

	attempt := func(path, address, guess string) (int, int) {
		body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.RemoteAddr = address + ":54321"
		req.SetBasicAuth("admin", guess)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec.Code, body.reads
	}

	for i := 0; i < maxFailedAttempts; i++ {
		path := []string{"/api/config/export", "/api/config/import"}[i%2]
		if code, reads := attempt(path, "203.0.113."+strconv.Itoa(10+i), "wrong-guess"); code != http.StatusUnauthorized || reads != 0 {
			t.Fatalf("wrong guess %d on %s = status %d, reads %d, want 401 before any read", i+1, path, code, reads)
		}
	}
	if _, _, locked := GetLockoutInfo("admin"); !locked {
		t.Fatalf("%d wrong passwords from different addresses did not lock the account", maxFailedAttempts)
	}
	for _, path := range []string{"/api/config/export", "/api/config/import"} {
		if code, reads := attempt(path, "203.0.113.200", password); code != http.StatusForbidden || reads != 0 {
			t.Errorf("%s on a locked account = status %d, reads %d, want 403 before any read", path, code, reads)
		}
	}

	resetFailedLogins()
	if code, _ := attempt("/api/config/export", "203.0.113.201", "wrong-guess"); code != http.StatusUnauthorized {
		t.Fatalf("wrong guess = %d, want 401", code)
	}
	if code, reads := attempt("/api/config/export", "203.0.113.201", password); code != http.StatusBadRequest || reads == 0 {
		t.Fatalf("right password after one wrong guess = status %d, reads %d, want the handler's 400", code, reads)
	}
	if attempts, _, _ := GetLockoutInfo("admin"); attempts != 0 {
		t.Fatalf("successful Basic authentication left %d failed attempts on the account", attempts)
	}
}

func TestConfigTransferTokenScopesAndOrganizationBinding(t *testing.T) {
	readRaw := "synthetic-read-token-123.12345678"
	writeRaw := "synthetic-write-token-123.12345678"
	readRecord := newTokenRecord(t, readRaw, []string{config.ScopeSettingsRead}, nil)
	writeRecord := newTokenRecord(t, writeRaw, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, readRecord, writeRecord)
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)

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
	boundRouter := newTestRouter(t, boundCfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, boundRouter)
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
	router := newTestRouter(t, cfg, nil, mtm, nil, nil, "test")
	cleanupTestRouter(t, router)

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
	router := newTestRouter(t, cfg, nil, nil, nil, func() error {
		reloadCalls++
		return nil
	}, "test")
	cleanupTestRouter(t, router)

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

func TestConfigTransferFixtureOwnsRouterLifecycle(t *testing.T) {
	var router *Router
	workerDone := make(chan struct{})
	t.Run("fixture", func(t *testing.T) {
		router = newConfigTransferTestRouter(t, false, nil)
		router.startLifecycleWorker(func() {
			<-router.lifecycleCtx.Done()
			close(workerDone)
		})
	})
	// Retain cleanup even if the helper regresses.
	defer router.shutdownBackgroundWorkers()
	defer router.ShutdownResourceStores()
	defer router.ShutdownRBAC()
	select {
	case <-router.lifecycleCtx.Done():
	default:
		t.Fatal("config-transfer helper left router lifecycle active")
	}
	select {
	case <-workerDone:
	default:
		t.Fatal("config-transfer helper did not join its router worker")
	}
}

// TestConfigTransferVerifiedUsersRequireTenantManagement applies the transfer
// management rule to the configured local administrator under both verified
// credential types. Instance administration plus tenant membership is not
// tenant management, so a viewer membership is refused before the handler
// reads the archive; a tenant the administrator manages, and the default
// organization, reach the handler.
func TestConfigTransferVerifiedUsersRequireTenantManagement(t *testing.T) {
	f := newBasicTenantFixture(t)
	sessionToken := "synthetic-transfer-admin-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "198.51.100.1", basicTenantAdmin)

	credentials := map[string]func(*http.Request){
		"basic": func(req *http.Request) {
			req.SetBasicAuth(basicTenantAdmin, basicTenantPassword)
		},
		"session": func(req *http.Request) {
			req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
			req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
		},
	}
	tests := []struct {
		orgID   string
		reached bool
	}{
		{orgID: "", reached: true},
		{orgID: "tenant-managed", reached: true},
		{orgID: "tenant-viewer"},
		{orgID: "tenant-foreign"},
	}

	request := 0
	for credentialName, attach := range credentials {
		for _, tc := range tests {
			for _, path := range []string{"/api/config/export", "/api/config/import"} {
				name := credentialName + " " + tc.orgID + " " + path
				t.Run(name, func(t *testing.T) {
					request++
					body := &countingConfigTransferBody{reader: strings.NewReader(`{not-json`)}
					req := httptest.NewRequest(http.MethodPost, path, body)
					req.RemoteAddr = "198.51.100." + strconv.Itoa(request) + ":1234"
					if tc.orgID != "" {
						req.Header.Set("X-Pulse-Org-ID", tc.orgID)
					}
					attach(req)
					rec := httptest.NewRecorder()
					f.router.Handler().ServeHTTP(rec, req)
					if tc.reached {
						if rec.Code != http.StatusBadRequest || body.reads == 0 {
							t.Fatalf("status %d, reads %d: %s", rec.Code, body.reads, rec.Body.String())
						}
						return
					}
					if rec.Code != http.StatusForbidden || body.reads != 0 {
						t.Fatalf("status %d, reads %d: %s", rec.Code, body.reads, rec.Body.String())
					}
				})
			}
		}
	}
	if len(f.cfg.APITokens) != 0 {
		t.Fatalf("transfer matrix created tokens: %+v", f.cfg.APITokens)
	}
}

// TestConfigImportTokenAuthorityBoundsAccessCredentials restores an archive
// that carries its own full-access API token and SSO provider. A settings:write
// token imports the settings but must not thereby issue the archive's
// credentials; an instance-wide full-access token restores them as before.
func TestConfigImportTokenAuthorityBoundsAccessCredentials(t *testing.T) {
	const passphrase = "synthetic-passphrase"
	writeRaw := "synthetic-write-token-123.12345678"
	fullRaw := "synthetic-full-token-123.12345678"
	archivedRaw := "synthetic-archived-token-123.12345678"

	source := config.NewConfigPersistence(t.TempDir())
	archivedRecord := newTokenRecord(t, archivedRaw, []string{config.ScopeWildcard}, nil)
	if err := source.SaveAPITokens([]config.APITokenRecord{archivedRecord}); err != nil {
		t.Fatalf("save archived tokens: %v", err)
	}
	if err := source.SaveSSOConfig(enabledConfigTransferSSO(config.SSOProviderTypeOIDC)); err != nil {
		t.Fatalf("save archived SSO config: %v", err)
	}
	archive := func() string {
		t.Helper()
		data, err := source.ExportConfig(passphrase)
		if err != nil {
			t.Fatalf("export archive: %v", err)
		}
		body, err := json.Marshal(map[string]string{"data": data, "passphrase": passphrase})
		if err != nil {
			t.Fatalf("marshal import request: %v", err)
		}
		return string(body)
	}

	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, writeRaw, []string{config.ScopeSettingsWrite}, nil),
		newTokenRecord(t, fullRaw, []string{config.ScopeWildcard}, nil),
	)
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	persistence := config.NewConfigPersistence(cfg.DataPath)
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("save destination tokens: %v", err)
	}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)

	importAs := func(token, remoteAddr string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(archive()))
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-API-Token", token)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("import status = %d: %s", rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode import response: %v", err)
		}
		return response
	}
	ssoProviders := func() int {
		t.Helper()
		sso, err := persistence.LoadSSOConfig()
		if err != nil {
			t.Fatalf("load destination SSO config: %v", err)
		}
		if sso == nil {
			return 0
		}
		return len(sso.Providers)
	}

	response := importAs(writeRaw, "127.0.2.1:1234")
	if response["accessCredentialsRestored"] != false {
		t.Fatalf("settings:write import response = %v, want accessCredentialsRestored=false", response)
	}
	if cfg.IsValidAPIToken(archivedRaw) {
		t.Fatal("settings:write import issued the archive's full-access token")
	}
	if !cfg.IsValidAPIToken(writeRaw) || !cfg.IsValidAPIToken(fullRaw) {
		t.Fatal("settings:write import replaced the destination's API tokens")
	}
	if got := ssoProviders(); got != 0 {
		t.Fatalf("settings:write import restored %d SSO providers", got)
	}

	response = importAs(fullRaw, "127.0.2.2:1234")
	if response["accessCredentialsRestored"] != true {
		t.Fatalf("full-access import response = %v, want accessCredentialsRestored=true", response)
	}
	if !cfg.IsValidAPIToken(archivedRaw) || cfg.IsValidAPIToken(writeRaw) {
		t.Fatal("full-access import did not replace the API token inventory")
	}
	if got := ssoProviders(); got != 1 {
		t.Fatalf("full-access import restored %d SSO providers, want 1", got)
	}
}

// endSessionOnRead ends a session when the handler first reads the request
// body, as a caller who signs out while the archive is still outstanding does.
type endSessionOnRead struct {
	reader  io.Reader
	session string
}

func (b *endSessionOnRead) Read(p []byte) (int, error) {
	if b.session != "" {
		GetSessionStore().DeleteSession(b.session)
		b.session = ""
	}
	return b.reader.Read(p)
}

// TestConfigImportOrganizationManagerKeepsAccessCredentials restores, into an
// organization, an archive that carries an action-scoped API token bound to
// that organization and acting for another member. The organization's owner
// manages its settings and may import them, but may not issue its tokens
// (POST /api/security/tokens is instance administration), so the import must
// not issue the archive's either, also when the owner's session ends before
// the archive is read. An instance administrator who also manages the
// organization restores them as before.
func TestConfigImportOrganizationManagerKeepsAccessCredentials(t *testing.T) {
	f := newBasicTenantFixture(t)
	const (
		passphrase  = "synthetic-passphrase"
		orgID       = "tenant-managed"
		archivedRaw = "synthetic-archived-token-123.12345678"
	)

	source := config.NewConfigPersistence(t.TempDir())
	archivedRecord := newTokenRecord(t, archivedRaw, []string{config.ScopeActionsPlan, config.ScopeActionsApprove, config.ScopeActionsExecute}, nil)
	archivedRecord.OrgID = orgID
	setAPITokenOwnerUserID(&archivedRecord, basicTenantAdmin)
	if err := source.SaveAPITokens([]config.APITokenRecord{archivedRecord}); err != nil {
		t.Fatalf("save archived tokens: %v", err)
	}
	if err := source.SaveSSOConfig(enabledConfigTransferSSO(config.SSOProviderTypeOIDC)); err != nil {
		t.Fatalf("save archived SSO config: %v", err)
	}

	destination, err := f.router.multiTenant.GetPersistence(orgID)
	if err != nil {
		t.Fatalf("organization persistence: %v", err)
	}
	restored := func() (tokens, ssoProviders int) {
		t.Helper()
		records, err := destination.LoadAPITokens()
		if err != nil {
			t.Fatalf("load organization tokens: %v", err)
		}
		for _, record := range records {
			if record.Hash == archivedRecord.Hash {
				tokens++
			}
		}
		sso, err := destination.LoadSSOConfig()
		if err != nil {
			t.Fatalf("load organization SSO config: %v", err)
		}
		if sso != nil {
			ssoProviders = len(sso.Providers)
		}
		return tokens, ssoProviders
	}
	importAs := func(user, remoteAddr string, signOutBeforeRead bool) map[string]any {
		t.Helper()
		data, err := source.ExportConfig(passphrase)
		if err != nil {
			t.Fatalf("export archive: %v", err)
		}
		body, err := json.Marshal(map[string]string{"data": data, "passphrase": passphrase})
		if err != nil {
			t.Fatalf("marshal import request: %v", err)
		}
		sessionToken := "synthetic-import-session-" + user + "-" + remoteAddr
		GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "198.51.100.1", user)
		archive := &endSessionOnRead{reader: strings.NewReader(string(body))}
		if signOutBeforeRead {
			archive.session = sessionToken
		}
		req := httptest.NewRequest(http.MethodPost, "/api/config/import", archive)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Pulse-Org-ID", orgID)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
		req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
		rec := httptest.NewRecorder()
		f.router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("import as %s status = %d: %s", user, rec.Code, rec.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode import response: %v", err)
		}
		return response
	}

	for i, signOutBeforeRead := range []bool{false, true} {
		response := importAs("managed-owner", "127.0.3."+strconv.Itoa(i+1)+":1234", signOutBeforeRead)
		if response["accessCredentialsRestored"] != false {
			t.Fatalf("organization owner import (signed out before read: %v) response = %v, want accessCredentialsRestored=false", signOutBeforeRead, response)
		}
		if tokens, ssoProviders := restored(); tokens != 0 || ssoProviders != 0 {
			t.Fatalf("organization owner import (signed out before read: %v) issued %d archived tokens and %d SSO providers", signOutBeforeRead, tokens, ssoProviders)
		}
	}

	response := importAs(basicTenantAdmin, "127.0.3.3:1234", false)
	if response["accessCredentialsRestored"] != true {
		t.Fatalf("instance administrator import response = %v, want accessCredentialsRestored=true", response)
	}
	if tokens, ssoProviders := restored(); tokens != 1 || ssoProviders != 1 {
		t.Fatalf("instance administrator import restored %d archived tokens and %d SSO providers, want 1 and 1", tokens, ssoProviders)
	}
}
