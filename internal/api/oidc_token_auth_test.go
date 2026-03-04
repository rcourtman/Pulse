package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAdminEndpointsAllowAPITokenWhenOIDCEnabled(t *testing.T) {
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	rawToken := "oidc-admin-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{record},
	}
	ssoCfg := config.NewSSOConfig()
	if err := ssoCfg.AddProvider(config.SSOProvider{
		ID:      "test-oidc",
		Name:    "Test OIDC",
		Type:    config.SSOProviderTypeOIDC,
		Enabled: true,
		OIDC: &config.OIDCProviderConfig{
			IssuerURL: "https://issuer.example.com",
			ClientID:  "client-id",
		},
	}); err != nil {
		t.Fatalf("failed to add SSO provider: %v", err)
	}
	if err := config.NewConfigPersistence(dataDir).SaveSSOConfig(ssoCfg); err != nil {
		t.Fatalf("failed to persist SSO config: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with API token, got %d", rec.Code)
	}
}
