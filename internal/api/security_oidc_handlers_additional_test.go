package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSecurityOIDCHandlers_GetConfig(t *testing.T) {
	cfg := &config.Config{
		PublicURL: "https://pulse.example.com",
		OIDC: &config.OIDCConfig{
			Enabled:       true,
			IssuerURL:     "https://issuer.example.com",
			ClientID:      "client-id",
			ClientSecret:  "super-secret",
			RedirectURL:   "https://pulse.example.com/oidc/callback",
			Scopes:        []string{"openid"},
			UsernameClaim: "sub",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
	}
	router := &Router{config: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/security/oidc", nil)
	rr := httptest.NewRecorder()

	router.handleOIDCConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp oidcResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Enabled {
		t.Fatalf("expected OIDC to be enabled")
	}
	if resp.IssuerURL != cfg.OIDC.IssuerURL {
		t.Fatalf("unexpected issuer url")
	}
	if !resp.ClientSecretSet {
		t.Fatalf("expected client secret to be marked as set")
	}
}

func TestSecurityOIDCHandlers_UpdateSaveFailure(t *testing.T) {
	cfg := &config.Config{
		PublicURL: "https://pulse.example.com",
		OIDC: &config.OIDCConfig{
			ClientSecret: "old-secret",
			CABundle:     "old-bundle",
		},
	}
	router := &Router{config: cfg}

	payload := map[string]any{
		"enabled":           true,
		"issuerUrl":         "https://issuer.example.com",
		"clientId":          "client-id",
		"clientSecret":      "new-secret",
		"redirectUrl":       "https://pulse.example.com/oidc/callback",
		"scopes":            []string{"openid"},
		"usernameClaim":     "sub",
		"emailClaim":        "email",
		"groupsClaim":       "groups",
		"clearClientSecret": true,
		"caBundle":          "new-bundle",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/api/security/oidc", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	router.handleOIDCConfig(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	var apiErr APIError
	if err := json.NewDecoder(rr.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if apiErr.Code != "save_failed" {
		t.Fatalf("unexpected error code: %s", apiErr.Code)
	}
}
