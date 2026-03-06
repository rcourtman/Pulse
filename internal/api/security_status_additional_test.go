package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type allowRulesAuthorizer struct {
	rules map[string]bool
}

func (a *allowRulesAuthorizer) Authorize(_ context.Context, action string, resource string) (bool, error) {
	if a == nil {
		return false, nil
	}
	return a.rules[action+":"+resource], nil
}

func TestSecurityStatusIgnoresInvalidTokenHeader(t *testing.T) {
	rawToken := "status-valid-token-123.12345678"
	record := newTokenRecord(t, rawToken, nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-API-Token", "invalid-token")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hint, ok := payload["apiTokenHint"].(string); ok && hint != "" {
		t.Fatalf("expected apiTokenHint to be empty for invalid token, got %q", hint)
	}
	if _, ok := payload["tokenScopes"]; ok {
		t.Fatalf("expected tokenScopes to be omitted for invalid token")
	}
}

func TestSecurityStatusIgnoresBearerTokenHeader(t *testing.T) {
	rawToken := "status-bearer-token-123.12345678"
	record := newTokenRecord(t, rawToken, nil, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hint, ok := payload["apiTokenHint"].(string); ok && hint != "" {
		t.Fatalf("expected apiTokenHint to be empty for bearer token, got %q", hint)
	}
	if _, ok := payload["tokenScopes"]; ok {
		t.Fatalf("expected tokenScopes to be omitted for bearer token")
	}
}

func TestSecurityStatusExposesPersistedSSOProvider(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	ssoCfg := config.NewSSOConfig()
	if err := ssoCfg.AddProvider(config.SSOProvider{
		ID:      "test-oidc",
		Name:    "Test OIDC",
		Type:    config.SSOProviderTypeOIDC,
		Enabled: true,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL:    "https://id.example.test",
			ClientID:     "pulse-client",
			ClientSecret: "secret",
		},
	}); err != nil {
		t.Fatalf("failed to add SSO provider: %v", err)
	}
	if err := config.NewConfigPersistence(cfg.DataPath).SaveSSOConfig(ssoCfg); err != nil {
		t.Fatalf("failed to persist SSO config: %v", err)
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	rawProviders, ok := payload["ssoProviders"].([]interface{})
	if !ok || len(rawProviders) == 0 {
		t.Fatalf("expected persisted SSO providers in security status, got %#v", payload["ssoProviders"])
	}

	firstProvider, ok := rawProviders[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected provider object, got %#v", rawProviders[0])
	}

	if got := firstProvider["id"]; got != "test-oidc" {
		t.Fatalf("provider id = %v, want test-oidc", got)
	}
	if got := firstProvider["type"]; got != "oidc" {
		t.Fatalf("provider type = %v, want oidc", got)
	}
	if got := firstProvider["loginUrl"]; got != "/api/oidc/test-oidc/login" {
		t.Fatalf("provider loginUrl = %v, want /api/oidc/test-oidc/login", got)
	}
}

func TestSecurityStatusExposesSettingsCapabilitiesForScopedToken(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&allowRulesAuthorizer{
		rules: map[string]bool{
			"admin:users":      true,
			"read:audit_logs":  true,
			"admin:audit_logs": true,
		},
	})
	defer auth.SetAuthorizer(prevAuthorizer)

	rawToken := "status-cap-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload struct {
		SettingsCapabilities securityStatusSettingsCapabilities `json:"settingsCapabilities"`
		TokenScopes          []string                           `json:"tokenScopes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !payload.SettingsCapabilities.APIAccess {
		t.Fatalf("expected apiAccess capability for scoped admin token")
	}
	if !payload.SettingsCapabilities.Authentication {
		t.Fatalf("expected authentication capability for settings admin token")
	}
	if !payload.SettingsCapabilities.SingleSignOn {
		t.Fatalf("expected singleSignOn capability for scoped admin token")
	}
	if !payload.SettingsCapabilities.Roles || !payload.SettingsCapabilities.Users {
		t.Fatalf("expected RBAC capabilities for scoped admin token")
	}
	if !payload.SettingsCapabilities.AuditLog || !payload.SettingsCapabilities.AuditWebhooks {
		t.Fatalf("expected audit capabilities for scoped admin token")
	}
	if !payload.SettingsCapabilities.Relay {
		t.Fatalf("expected relay capability for settings admin token")
	}
	if payload.SettingsCapabilities.BillingAdmin {
		t.Fatalf("did not expect billingAdmin capability for API token")
	}
	if len(payload.TokenScopes) != 2 {
		t.Fatalf("expected token scopes in response, got %#v", payload.TokenScopes)
	}
}

func TestSecurityStatusUsesRBACForProxyCapabilities(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&allowRulesAuthorizer{
		rules: map[string]bool{
			"admin:users": true,
		},
	})
	defer auth.SetAuthorizer(prevAuthorizer)

	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cfg := &config.Config{
		ProxyAuthSecret:     "proxy-secret",
		ProxyAuthUserHeader: "X-Proxy-User",
		ProxyAuthRoleHeader: "X-Proxy-Roles",
		ProxyAuthAdminRole:  "admin",
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-Proxy-Secret", "proxy-secret")
	req.Header.Set("X-Proxy-User", "viewer")
	req.Header.Set("X-Proxy-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload struct {
		SettingsCapabilities securityStatusSettingsCapabilities `json:"settingsCapabilities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SettingsCapabilities.Authentication {
		t.Fatalf("did not expect authentication capability for non-admin proxy user")
	}
	if !payload.SettingsCapabilities.SingleSignOn {
		t.Fatalf("expected SSO capability for RBAC-authorized proxy user")
	}
	if !payload.SettingsCapabilities.Roles || !payload.SettingsCapabilities.Users {
		t.Fatalf("expected RBAC management capabilities for RBAC-authorized proxy user")
	}
	if payload.SettingsCapabilities.Relay {
		t.Fatalf("did not expect relay capability for non-admin proxy user")
	}
}

func TestSecurityStatusRestrictsSessionCapabilitiesToConfiguredAdmin(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&allowRulesAuthorizer{
		rules: map[string]bool{
			"admin:users":     true,
			"read:audit_logs": true,
		},
	})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hash"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	sessionToken := "session-capabilities-token"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "member")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload struct {
		SettingsCapabilities securityStatusSettingsCapabilities `json:"settingsCapabilities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.SettingsCapabilities.Authentication {
		t.Fatalf("did not expect authentication capability for non-admin session user")
	}
	if payload.SettingsCapabilities.Roles || payload.SettingsCapabilities.Users {
		t.Fatalf("did not expect RBAC capabilities for non-admin session user")
	}
	if payload.SettingsCapabilities.AuditLog {
		t.Fatalf("did not expect audit capability for non-admin session user")
	}
}
