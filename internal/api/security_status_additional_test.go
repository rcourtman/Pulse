package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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
