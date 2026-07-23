package config

import (
	"reflect"
	"strings"
	"testing"
)

// legacyOIDCEnvKeys is the complete set of v5 OIDC_* environment variables read
// by LegacyOIDCConfigFromEnv. setLegacyOIDCEnv baseline-clears every one of them
// so each subtest is fully independent of the ambient process environment.
var legacyOIDCEnvKeys = []string{
	"OIDC_ENABLED",
	"OIDC_ISSUER_URL",
	"OIDC_CLIENT_ID",
	"OIDC_CLIENT_SECRET",
	"OIDC_REDIRECT_URL",
	"OIDC_LOGOUT_URL",
	"OIDC_USERNAME_CLAIM",
	"OIDC_EMAIL_CLAIM",
	"OIDC_GROUPS_CLAIM",
	"OIDC_CA_BUNDLE",
	"OIDC_SCOPES",
	"OIDC_ALLOWED_GROUPS",
	"OIDC_ALLOWED_DOMAINS",
	"OIDC_ALLOWED_EMAILS",
	"OIDC_GROUP_ROLE_MAPPINGS",
}

// setLegacyOIDCEnv resets every legacy OIDC_* variable to empty and then applies
// the given overrides, using t.Setenv so values are restored when the subtest
// ends.
func setLegacyOIDCEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	for _, key := range legacyOIDCEnvKeys {
		t.Setenv(key, "")
	}
	for key, value := range overrides {
		t.Setenv(key, value)
	}
}

// TestBranchcov0723Am exercises every branch of ApplyLegacyOIDCEnvProvider by
// driving it with concrete (env, SSOConfig, publicURL) inputs and asserting the
// concrete resulting state. Each subtest is independent: it builds its own
// config and scopes its environment via t.Setenv.
func TestBranchcov0723Am(t *testing.T) {
	minEnv := map[string]string{
		"OIDC_ENABLED":    "true",
		"OIDC_ISSUER_URL": "https://idp.example.com",
		"OIDC_CLIENT_ID":  "client-123",
	}

	t.Run("nil_config_returns_false", func(t *testing.T) {
		got := ApplyLegacyOIDCEnvProvider(nil, "https://app.example.com")
		if got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider(nil, ...) = true, want false")
		}
	})

	t.Run("no_legacy_env_returns_false_and_leaves_config_untouched", func(t *testing.T) {
		setLegacyOIDCEnv(t, nil) // every OIDC_* var empty -> OIDC_ENABLED falsy
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = true, want false when no legacy env is set")
		}
		if len(cfg.Providers) != 0 {
			t.Errorf("Providers len = %d, want 0 (config must be untouched)", len(cfg.Providers))
		}
		if cfg.DefaultProviderID != "" {
			t.Errorf("DefaultProviderID = %q, want empty (config must be untouched)", cfg.DefaultProviderID)
		}
	})

	t.Run("partially_set_env_no_issuer_returns_false_no_half_write", func(t *testing.T) {
		// OIDC_ENABLED truthy but no IssuerURL/ClientID -> provider cannot be
		// built, so nothing is written onto the config.
		setLegacyOIDCEnv(t, map[string]string{"OIDC_ENABLED": "true"})
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = true, want false when issuer/client id are missing")
		}
		if len(cfg.Providers) != 0 {
			t.Errorf("Providers len = %d, want 0 (no half-written provider)", len(cfg.Providers))
		}
		if cfg.DefaultProviderID != "" {
			t.Errorf("DefaultProviderID = %q, want empty (no half-write)", cfg.DefaultProviderID)
		}
	})

	t.Run("minimum_required_set_adds_provider_and_writes_every_field", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		if len(cfg.Providers) != 1 {
			t.Fatalf("Providers len = %d, want 1", len(cfg.Providers))
		}
		p := cfg.Providers[0]
		if p.ID != LegacyOIDCProviderID {
			t.Errorf("provider.ID = %q, want %q", p.ID, LegacyOIDCProviderID)
		}
		if p.Name != "Single Sign-On" {
			t.Errorf("provider.Name = %q, want %q", p.Name, "Single Sign-On")
		}
		if p.Type != SSOProviderTypeOIDC {
			t.Errorf("provider.Type = %q, want %q", p.Type, SSOProviderTypeOIDC)
		}
		if !p.Enabled {
			t.Errorf("provider.Enabled = false, want true")
		}
		if p.DisplayName != "Single Sign-On" {
			t.Errorf("provider.DisplayName = %q, want %q", p.DisplayName, "Single Sign-On")
		}
		if !p.RuntimeManaged {
			t.Errorf("provider.RuntimeManaged = false, want true")
		}
		if p.OIDC == nil {
			t.Fatal("provider.OIDC is nil, want populated OIDC config")
		}
		if p.OIDC.IssuerURL != "https://idp.example.com" {
			t.Errorf("OIDC.IssuerURL = %q, want %q", p.OIDC.IssuerURL, "https://idp.example.com")
		}
		if p.OIDC.ClientID != "client-123" {
			t.Errorf("OIDC.ClientID = %q, want %q", p.OIDC.ClientID, "client-123")
		}
		if p.OIDC.ClientSecret != "" {
			t.Errorf("OIDC.ClientSecret = %q, want empty", p.OIDC.ClientSecret)
		}
		wantRedirect := "https://app.example.com" + DefaultOIDCCallbackPath
		if p.OIDC.RedirectURL != wantRedirect {
			t.Errorf("OIDC.RedirectURL = %q, want %q", p.OIDC.RedirectURL, wantRedirect)
		}
		if p.OIDC.LogoutURL != "" {
			t.Errorf("OIDC.LogoutURL = %q, want empty", p.OIDC.LogoutURL)
		}
		if p.OIDC.UsernameClaim != "preferred_username" {
			t.Errorf("OIDC.UsernameClaim = %q, want %q", p.OIDC.UsernameClaim, "preferred_username")
		}
		if p.OIDC.EmailClaim != "email" {
			t.Errorf("OIDC.EmailClaim = %q, want %q", p.OIDC.EmailClaim, "email")
		}
		if p.OIDC.CABundle != "" {
			t.Errorf("OIDC.CABundle = %q, want empty", p.OIDC.CABundle)
		}
		if p.OIDC.ClientSecretSet {
			t.Errorf("OIDC.ClientSecretSet = true, want false (no secret provided)")
		}
		wantScopes := []string{"openid", "profile", "email"}
		if !reflect.DeepEqual(p.OIDC.Scopes, wantScopes) {
			t.Errorf("OIDC.Scopes = %v, want %v", p.OIDC.Scopes, wantScopes)
		}
		if p.OIDC.EnvOverrides == nil {
			t.Error("OIDC.EnvOverrides is nil, want non-nil override map")
		} else if !p.OIDC.EnvOverrides["enabled"] || !p.OIDC.EnvOverrides["issuerUrl"] || !p.OIDC.EnvOverrides["clientId"] {
			t.Errorf("OIDC.EnvOverrides = %v, want enabled/issuerUrl/clientId all true", p.OIDC.EnvOverrides)
		}
		if cfg.DefaultProviderID != LegacyOIDCProviderID {
			t.Errorf("DefaultProviderID = %q, want %q (empty default should be assigned)", cfg.DefaultProviderID, LegacyOIDCProviderID)
		}
	})

	t.Run("empty_public_url_yields_empty_redirect_url", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		p := cfg.Providers[0]
		if p.OIDC == nil || p.OIDC.RedirectURL != "" {
			t.Errorf("derived RedirectURL = %q, want empty when publicURL is empty", p.OIDC.RedirectURL)
		}
	})

	t.Run("trailing_slash_public_url_produces_no_double_slash", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com/")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		p := cfg.Providers[0]
		wantRedirect := "https://app.example.com" + DefaultOIDCCallbackPath
		if p.OIDC.RedirectURL != wantRedirect {
			t.Errorf("RedirectURL = %q, want %q (no double slash)", p.OIDC.RedirectURL, wantRedirect)
		}
		if strings.Contains(p.OIDC.RedirectURL, "//api") {
			t.Errorf("RedirectURL %q contains a double slash before the callback path", p.OIDC.RedirectURL)
		}
	})

	t.Run("client_secret_propagates_and_sets_secret_flag", func(t *testing.T) {
		setLegacyOIDCEnv(t, mergeEnv(minEnv, map[string]string{"OIDC_CLIENT_SECRET": "s3cr3t"}))
		cfg := NewSSOConfig()

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		p := cfg.Providers[0]
		if p.OIDC.ClientSecret != "s3cr3t" {
			t.Errorf("OIDC.ClientSecret = %q, want %q", p.OIDC.ClientSecret, "s3cr3t")
		}
		if !p.OIDC.ClientSecretSet {
			t.Errorf("OIDC.ClientSecretSet = false, want true when a secret is provided")
		}
		if p.OIDC.EnvOverrides == nil || !p.OIDC.EnvOverrides["clientSecret"] {
			t.Errorf("OIDC.EnvOverrides[clientSecret] not marked true, want true")
		}
	})

	t.Run("dangling_default_id_reassigned_to_legacy_provider", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := NewSSOConfig()
		cfg.DefaultProviderID = "does-not-exist" // points at no real provider

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		if cfg.DefaultProviderID != LegacyOIDCProviderID {
			t.Errorf("DefaultProviderID = %q, want %q (dangling default should be reassigned)", cfg.DefaultProviderID, LegacyOIDCProviderID)
		}
		if len(cfg.Providers) != 1 {
			t.Errorf("Providers len = %d, want 1", len(cfg.Providers))
		}
	})

	t.Run("existing_valid_default_id_is_preserved", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := NewSSOConfig()
		if err := cfg.AddProvider(SSOProvider{ID: "other", Name: "Other", Type: SSOProviderTypeOIDC, Enabled: true}); err != nil {
			t.Fatalf("AddProvider failed: %v", err)
		}
		cfg.DefaultProviderID = "other"

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if !got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = false, want true")
		}
		if cfg.DefaultProviderID != "other" {
			t.Errorf("DefaultProviderID = %q, want %q (valid existing default must be preserved)", cfg.DefaultProviderID, "other")
		}
		if len(cfg.Providers) != 2 {
			t.Errorf("Providers len = %d, want 2", len(cfg.Providers))
		}
	})

	t.Run("existing_runtime_managed_provider_is_overwritten", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := &SSOConfig{
			Providers: []SSOProvider{
				{
					ID:             LegacyOIDCProviderID,
					Name:           "Old Runtime",
					Type:           SSOProviderTypeOIDC,
					Enabled:        true,
					RuntimeManaged: true,
					OIDC: &OIDCProviderConfig{
						IssuerURL: "https://old.example.com",
						ClientID:  "old-client",
					},
				},
			},
			DefaultProviderID: LegacyOIDCProviderID,
		}

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = true, want false when a provider with the same ID already exists")
		}
		if len(cfg.Providers) != 1 {
			t.Fatalf("Providers len = %d, want 1 (existing should be replaced in place, not appended)", len(cfg.Providers))
		}
		p := cfg.Providers[0]
		if p.Name != "Single Sign-On" {
			t.Errorf("provider.Name = %q, want %q (runtime-managed provider should be overwritten)", p.Name, "Single Sign-On")
		}
		if p.OIDC == nil || p.OIDC.IssuerURL != "https://idp.example.com" {
			got := "<nil>"
			if p.OIDC != nil {
				got = p.OIDC.IssuerURL
			}
			t.Errorf("provider.OIDC.IssuerURL = %q, want %q (runtime-managed provider should be overwritten)", got, "https://idp.example.com")
		}
		if p.OIDC == nil || p.OIDC.ClientID != "client-123" {
			t.Errorf("provider.OIDC.ClientID not overwritten, want %q", "client-123")
		}
		if !p.RuntimeManaged {
			t.Errorf("provider.RuntimeManaged = false, want true (overwrite must keep runtime flag)")
		}
	})

	t.Run("existing_persisted_provider_is_left_untouched", func(t *testing.T) {
		setLegacyOIDCEnv(t, minEnv)
		cfg := &SSOConfig{
			Providers: []SSOProvider{
				{
					ID:             LegacyOIDCProviderID,
					Name:           "Persisted",
					Type:           SSOProviderTypeOIDC,
					Enabled:        true,
					RuntimeManaged: false, // persisted provider
					OIDC: &OIDCProviderConfig{
						IssuerURL: "https://persisted.example.com",
						ClientID:  "persisted-client",
					},
				},
			},
			DefaultProviderID: LegacyOIDCProviderID,
		}

		got := ApplyLegacyOIDCEnvProvider(cfg, "https://app.example.com")
		if got {
			t.Fatalf("ApplyLegacyOIDCEnvProvider = true, want false when a persisted provider with the same ID already exists")
		}
		if len(cfg.Providers) != 1 {
			t.Fatalf("Providers len = %d, want 1", len(cfg.Providers))
		}
		p := cfg.Providers[0]
		if p.Name != "Persisted" {
			t.Errorf("provider.Name = %q, want %q (persisted provider must not be overwritten by legacy env)", p.Name, "Persisted")
		}
		if p.OIDC == nil || p.OIDC.IssuerURL != "https://persisted.example.com" {
			t.Errorf("provider.OIDC.IssuerURL changed, want %q left untouched", "https://persisted.example.com")
		}
		if p.OIDC == nil || p.OIDC.ClientID != "persisted-client" {
			t.Errorf("provider.OIDC.ClientID changed, want %q left untouched", "persisted-client")
		}
		if p.RuntimeManaged {
			t.Errorf("provider.RuntimeManaged = true, want false (persisted provider must not gain runtime flag)")
		}
	})
}

// mergeEnv returns a new map combining base with overrides (overrides win).
func mergeEnv(base, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}
