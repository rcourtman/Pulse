package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestMakeOIDCResponse_NilConfig(t *testing.T) {
	t.Parallel()

	resp := makeOIDCResponse(nil, "https://pulse.example.com")

	// Should return a default config when nil is passed
	if resp.Enabled {
		t.Error("expected Enabled to be false for nil config")
	}

	// Should have appropriate defaults
	if resp.DefaultRedirect == "" {
		t.Error("expected DefaultRedirect to be set")
	}
}

func TestMakeOIDCResponse_EmptyConfig(t *testing.T) {
	t.Parallel()

	cfg := config.NewOIDCConfig()
	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	if resp.Enabled {
		t.Error("expected Enabled to be false for new config")
	}
	if resp.ClientSecretSet {
		t.Error("expected ClientSecretSet to be false when no secret")
	}
}

func TestMakeOIDCResponse_EnabledWithSecret(t *testing.T) {
	t.Parallel()

	cfg := &config.OIDCConfig{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com",
		ClientID:     "pulse-client",
		ClientSecret: "super-secret-value",
		RedirectURL:  "https://pulse.example.com/auth/callback",
		Scopes:       []string{"openid", "profile", "email"},
		UsernameClaim: "preferred_username",
		EmailClaim:   "email",
		GroupsClaim:  "groups",
	}

	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	if !resp.Enabled {
		t.Error("expected Enabled to be true")
	}
	if resp.IssuerURL != "https://auth.example.com" {
		t.Errorf("expected IssuerURL 'https://auth.example.com', got %q", resp.IssuerURL)
	}
	if resp.ClientID != "pulse-client" {
		t.Errorf("expected ClientID 'pulse-client', got %q", resp.ClientID)
	}
	if !resp.ClientSecretSet {
		t.Error("expected ClientSecretSet to be true when secret is set")
	}
	// Client secret should NOT be exposed in response
	// (this is handled by the response struct not having a ClientSecret field)

	if len(resp.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(resp.Scopes))
	}
	if resp.UsernameClaim != "preferred_username" {
		t.Errorf("expected UsernameClaim 'preferred_username', got %q", resp.UsernameClaim)
	}
	if resp.EmailClaim != "email" {
		t.Errorf("expected EmailClaim 'email', got %q", resp.EmailClaim)
	}
}

func TestMakeOIDCResponse_WithEnvOverrides(t *testing.T) {
	t.Parallel()

	cfg := &config.OIDCConfig{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ClientID:  "env-client",
		EnvOverrides: map[string]bool{
			"OIDC_ENABLED":    true,
			"OIDC_ISSUER_URL": true,
			"OIDC_CLIENT_ID":  true,
		},
	}

	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	if resp.EnvOverrides == nil {
		t.Fatal("expected EnvOverrides to be set")
	}
	if len(resp.EnvOverrides) != 3 {
		t.Errorf("expected 3 env overrides, got %d", len(resp.EnvOverrides))
	}
	if !resp.EnvOverrides["OIDC_ENABLED"] {
		t.Error("expected OIDC_ENABLED override to be true")
	}
	if !resp.EnvOverrides["OIDC_ISSUER_URL"] {
		t.Error("expected OIDC_ISSUER_URL override to be true")
	}
}

func TestMakeOIDCResponse_AllowedFilters(t *testing.T) {
	t.Parallel()

	cfg := &config.OIDCConfig{
		Enabled:        true,
		IssuerURL:      "https://auth.example.com",
		ClientID:       "pulse-client",
		AllowedGroups:  []string{"admins", "ops"},
		AllowedDomains: []string{"example.com", "corp.example.com"},
		AllowedEmails:  []string{"admin@example.com"},
	}

	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	if len(resp.AllowedGroups) != 2 {
		t.Errorf("expected 2 allowed groups, got %d", len(resp.AllowedGroups))
	}
	if len(resp.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(resp.AllowedDomains))
	}
	if len(resp.AllowedEmails) != 1 {
		t.Errorf("expected 1 allowed email, got %d", len(resp.AllowedEmails))
	}
}

func TestMakeOIDCResponse_CABundle(t *testing.T) {
	t.Parallel()

	cfg := &config.OIDCConfig{
		Enabled:   true,
		IssuerURL: "https://auth.internal.example.com",
		ClientID:  "pulse-client",
		CABundle:  "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
	}

	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	if resp.CABundle == "" {
		t.Error("expected CABundle to be set")
	}
	if resp.CABundle != cfg.CABundle {
		t.Error("expected CABundle to match input")
	}
}

func TestMakeOIDCResponse_SlicesCopied(t *testing.T) {
	t.Parallel()

	// Ensure that slices are copied, not shared
	cfg := &config.OIDCConfig{
		Enabled:       true,
		IssuerURL:     "https://auth.example.com",
		ClientID:      "pulse-client",
		Scopes:        []string{"openid", "profile"},
		AllowedGroups: []string{"admins"},
	}

	resp := makeOIDCResponse(cfg, "https://pulse.example.com")

	// Modify the original slices
	cfg.Scopes = append(cfg.Scopes, "email")
	cfg.AllowedGroups[0] = "modified"

	// Response should not be affected
	if len(resp.Scopes) != 2 {
		t.Errorf("expected response scopes to be unchanged, got %d", len(resp.Scopes))
	}
	if resp.AllowedGroups[0] == "modified" {
		t.Error("response AllowedGroups should be a copy, not a reference")
	}
}
