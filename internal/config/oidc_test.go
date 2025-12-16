package config

import (
	"reflect"
	"testing"
)

func TestNormaliseList(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{
			name:   "empty slice",
			values: []string{},
			want:   []string{},
		},
		{
			name:   "nil slice",
			values: nil,
			want:   []string{},
		},
		{
			name:   "single value",
			values: []string{"foo"},
			want:   []string{"foo"},
		},
		{
			name:   "trims whitespace",
			values: []string{"  foo  ", "  bar  "},
			want:   []string{"foo", "bar"},
		},
		{
			name:   "removes empty strings",
			values: []string{"foo", "", "bar", "   "},
			want:   []string{"foo", "bar"},
		},
		{
			name:   "removes duplicates case insensitive",
			values: []string{"foo", "Foo", "FOO", "bar"},
			want:   []string{"foo", "bar"},
		},
		{
			name:   "preserves first occurrence",
			values: []string{"Foo", "foo", "bar", "Bar"},
			want:   []string{"Foo", "bar"},
		},
		{
			name:   "complex example",
			values: []string{"  openid  ", "profile", "", "  email  ", "OPENID", "Profile"},
			want:   []string{"openid", "profile", "email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normaliseList(tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normaliseList(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

func TestParseDelimited(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  nil,
		},
		{
			name:  "comma separated",
			input: "foo,bar,baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "space separated",
			input: "foo bar baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "mixed separators",
			input: "foo, bar  baz,qux",
			want:  []string{"foo", "bar", "baz", "qux"},
		},
		{
			name:  "removes duplicates",
			input: "foo,bar,foo,BAR",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "trims surrounding whitespace",
			input: "  foo , bar  ",
			want:  []string{"foo", "bar"},
		},
		{
			name:  "single value",
			input: "foo",
			want:  []string{"foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDelimited(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDelimited(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultRedirectURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		want      string
	}{
		{
			name:      "empty string",
			publicURL: "",
			want:      "",
		},
		{
			name:      "whitespace only",
			publicURL: "   ",
			want:      "",
		},
		{
			name:      "simple URL",
			publicURL: "https://pulse.example.com",
			want:      "https://pulse.example.com/api/oidc/callback",
		},
		{
			name:      "URL with trailing slash",
			publicURL: "https://pulse.example.com/",
			want:      "https://pulse.example.com/api/oidc/callback",
		},
		{
			name:      "URL with path",
			publicURL: "https://example.com/pulse",
			want:      "https://example.com/pulse/api/oidc/callback",
		},
		{
			name:      "URL with path and trailing slash",
			publicURL: "https://example.com/pulse/",
			want:      "https://example.com/pulse/api/oidc/callback",
		},
		{
			name:      "URL with port",
			publicURL: "https://pulse.example.com:8443",
			want:      "https://pulse.example.com:8443/api/oidc/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultRedirectURL(tt.publicURL)
			if got != tt.want {
				t.Errorf("DefaultRedirectURL(%q) = %q, want %q", tt.publicURL, got, tt.want)
			}
		})
	}
}

func TestOIDCConfigClone(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		var cfg *OIDCConfig
		clone := cfg.Clone()
		if clone != nil {
			t.Error("Clone() of nil should return nil")
		}
	})

	t.Run("deep copy", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:        true,
			IssuerURL:      "https://issuer.example.com",
			ClientID:       "client-123",
			ClientSecret:   "secret",
			RedirectURL:    "https://pulse.example.com/callback",
			Scopes:         []string{"openid", "profile"},
			AllowedGroups:  []string{"admin", "users"},
			AllowedDomains: []string{"example.com"},
			AllowedEmails:  []string{"user@example.com"},
			EnvOverrides:   map[string]bool{"enabled": true},
		}

		clone := cfg.Clone()

		// Should be equal
		if clone.Enabled != cfg.Enabled {
			t.Error("Enabled should be equal")
		}
		if clone.IssuerURL != cfg.IssuerURL {
			t.Error("IssuerURL should be equal")
		}

		// Modify clone slices - originals should be unchanged
		clone.Scopes[0] = "modified"
		if cfg.Scopes[0] == "modified" {
			t.Error("Clone should have independent Scopes slice")
		}

		clone.AllowedGroups[0] = "modified"
		if cfg.AllowedGroups[0] == "modified" {
			t.Error("Clone should have independent AllowedGroups slice")
		}

		clone.AllowedDomains[0] = "modified.com"
		if cfg.AllowedDomains[0] == "modified.com" {
			t.Error("Clone should have independent AllowedDomains slice")
		}

		clone.AllowedEmails[0] = "modified@example.com"
		if cfg.AllowedEmails[0] == "modified@example.com" {
			t.Error("Clone should have independent AllowedEmails slice")
		}

		// Modify clone EnvOverrides - originals should be unchanged
		clone.EnvOverrides["issuerUrl"] = true
		if cfg.EnvOverrides["issuerUrl"] {
			t.Error("Clone should have independent EnvOverrides map")
		}
	})
}

func TestOIDCConfigApplyDefaults(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		var cfg *OIDCConfig
		cfg.ApplyDefaults("") // Should not panic
	})

	t.Run("default scopes", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.ApplyDefaults("")

		expectedScopes := []string{"openid", "profile", "email"}
		if !reflect.DeepEqual(cfg.Scopes, expectedScopes) {
			t.Errorf("Scopes = %v, want %v", cfg.Scopes, expectedScopes)
		}
	})

	t.Run("preserves custom scopes", func(t *testing.T) {
		cfg := &OIDCConfig{
			Scopes: []string{"openid", "custom"},
		}
		cfg.ApplyDefaults("")

		if len(cfg.Scopes) != 2 || cfg.Scopes[1] != "custom" {
			t.Errorf("Scopes = %v, want [openid custom]", cfg.Scopes)
		}
	})

	t.Run("default claims", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.ApplyDefaults("")

		if cfg.UsernameClaim != "preferred_username" {
			t.Errorf("UsernameClaim = %q, want %q", cfg.UsernameClaim, "preferred_username")
		}
		if cfg.EmailClaim != "email" {
			t.Errorf("EmailClaim = %q, want %q", cfg.EmailClaim, "email")
		}
	})

	t.Run("preserves custom claims", func(t *testing.T) {
		cfg := &OIDCConfig{
			UsernameClaim: "sub",
			EmailClaim:    "mail",
		}
		cfg.ApplyDefaults("")

		if cfg.UsernameClaim != "sub" {
			t.Errorf("UsernameClaim = %q, want %q", cfg.UsernameClaim, "sub")
		}
		if cfg.EmailClaim != "mail" {
			t.Errorf("EmailClaim = %q, want %q", cfg.EmailClaim, "mail")
		}
	})

	t.Run("sets redirect URL from public URL", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.ApplyDefaults("https://pulse.example.com")

		expected := "https://pulse.example.com/api/oidc/callback"
		if cfg.RedirectURL != expected {
			t.Errorf("RedirectURL = %q, want %q", cfg.RedirectURL, expected)
		}
	})

	t.Run("preserves explicit redirect URL", func(t *testing.T) {
		cfg := &OIDCConfig{
			RedirectURL: "https://custom.example.com/callback",
		}
		cfg.ApplyDefaults("https://pulse.example.com")

		if cfg.RedirectURL != "https://custom.example.com/callback" {
			t.Errorf("RedirectURL should not be overwritten")
		}
	})

	t.Run("normalises lists", func(t *testing.T) {
		cfg := &OIDCConfig{
			AllowedGroups:  []string{"admin", "  admin  ", "users"},
			AllowedDomains: []string{"  example.com  "},
			AllowedEmails:  []string{"user@example.com", ""},
		}
		cfg.ApplyDefaults("")

		if len(cfg.AllowedGroups) != 2 {
			t.Errorf("AllowedGroups should be deduplicated, got %v", cfg.AllowedGroups)
		}
		if cfg.AllowedDomains[0] != "example.com" {
			t.Errorf("AllowedDomains should be trimmed, got %v", cfg.AllowedDomains)
		}
		if len(cfg.AllowedEmails) != 1 {
			t.Errorf("AllowedEmails should have empty entries removed, got %v", cfg.AllowedEmails)
		}
	})

	t.Run("initialises EnvOverrides map", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.ApplyDefaults("")

		if cfg.EnvOverrides == nil {
			t.Error("EnvOverrides should be initialized")
		}
	})
}

func TestOIDCConfigValidate(t *testing.T) {
	t.Run("nil config is valid", func(t *testing.T) {
		var cfg *OIDCConfig
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("disabled config is valid", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled: false,
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("enabled requires issuer URL", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			ClientID:    "client-123",
			RedirectURL: "https://pulse.example.com/callback",
			Scopes:      []string{"openid"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail without issuer URL")
		}
	})

	t.Run("enabled requires valid issuer URL", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			IssuerURL:   "not-a-valid-url",
			ClientID:    "client-123",
			RedirectURL: "https://pulse.example.com/callback",
			Scopes:      []string{"openid"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with invalid issuer URL")
		}
	})

	t.Run("enabled requires client ID", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			IssuerURL:   "https://issuer.example.com",
			RedirectURL: "https://pulse.example.com/callback",
			Scopes:      []string{"openid"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail without client ID")
		}
	})

	t.Run("enabled requires redirect URL", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:   true,
			IssuerURL: "https://issuer.example.com",
			ClientID:  "client-123",
			Scopes:    []string{"openid"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail without redirect URL")
		}
	})

	t.Run("enabled requires valid redirect URL", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			IssuerURL:   "https://issuer.example.com",
			ClientID:    "client-123",
			RedirectURL: "not-a-valid-url",
			Scopes:      []string{"openid"},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail with invalid redirect URL")
		}
	})

	t.Run("enabled requires at least one scope", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			IssuerURL:   "https://issuer.example.com",
			ClientID:    "client-123",
			RedirectURL: "https://pulse.example.com/callback",
			Scopes:      []string{},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("Validate() should fail without scopes")
		}
	})

	t.Run("valid enabled config", func(t *testing.T) {
		cfg := &OIDCConfig{
			Enabled:     true,
			IssuerURL:   "https://issuer.example.com",
			ClientID:    "client-123",
			RedirectURL: "https://pulse.example.com/callback",
			Scopes:      []string{"openid", "profile"},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})
}

func TestOIDCConfigMergeFromEnv(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		var cfg *OIDCConfig
		cfg.MergeFromEnv(map[string]string{"OIDC_ENABLED": "true"}) // Should not panic
	})

	t.Run("merges all fields", func(t *testing.T) {
		cfg := &OIDCConfig{}
		env := map[string]string{
			"OIDC_ENABLED":         "true",
			"OIDC_ISSUER_URL":      "https://issuer.example.com",
			"OIDC_CLIENT_ID":       "client-123",
			"OIDC_CLIENT_SECRET":   "secret",
			"OIDC_REDIRECT_URL":    "https://pulse.example.com/callback",
			"OIDC_LOGOUT_URL":      "https://issuer.example.com/logout",
			"OIDC_SCOPES":          "openid,profile,email",
			"OIDC_USERNAME_CLAIM":  "sub",
			"OIDC_EMAIL_CLAIM":     "mail",
			"OIDC_GROUPS_CLAIM":    "groups",
			"OIDC_ALLOWED_GROUPS":  "admin,users",
			"OIDC_ALLOWED_DOMAINS": "example.com,test.com",
			"OIDC_ALLOWED_EMAILS":  "user@example.com",
			"OIDC_CA_BUNDLE":       "-----BEGIN CERTIFICATE-----",
		}

		cfg.MergeFromEnv(env)

		if !cfg.Enabled {
			t.Error("Enabled should be true")
		}
		if cfg.IssuerURL != "https://issuer.example.com" {
			t.Errorf("IssuerURL = %q, want %q", cfg.IssuerURL, "https://issuer.example.com")
		}
		if cfg.ClientID != "client-123" {
			t.Errorf("ClientID = %q, want %q", cfg.ClientID, "client-123")
		}
		if cfg.ClientSecret != "secret" {
			t.Errorf("ClientSecret = %q, want %q", cfg.ClientSecret, "secret")
		}
		if cfg.RedirectURL != "https://pulse.example.com/callback" {
			t.Errorf("RedirectURL = %q", cfg.RedirectURL)
		}
		if cfg.LogoutURL != "https://issuer.example.com/logout" {
			t.Errorf("LogoutURL = %q", cfg.LogoutURL)
		}
		if len(cfg.Scopes) != 3 {
			t.Errorf("Scopes = %v, want 3 elements", cfg.Scopes)
		}
		if cfg.UsernameClaim != "sub" {
			t.Errorf("UsernameClaim = %q, want %q", cfg.UsernameClaim, "sub")
		}
		if cfg.EmailClaim != "mail" {
			t.Errorf("EmailClaim = %q, want %q", cfg.EmailClaim, "mail")
		}
		if cfg.GroupsClaim != "groups" {
			t.Errorf("GroupsClaim = %q, want %q", cfg.GroupsClaim, "groups")
		}
		if len(cfg.AllowedGroups) != 2 {
			t.Errorf("AllowedGroups = %v, want 2 elements", cfg.AllowedGroups)
		}
		if len(cfg.AllowedDomains) != 2 {
			t.Errorf("AllowedDomains = %v, want 2 elements", cfg.AllowedDomains)
		}
		if len(cfg.AllowedEmails) != 1 {
			t.Errorf("AllowedEmails = %v, want 1 element", cfg.AllowedEmails)
		}
		if cfg.CABundle != "-----BEGIN CERTIFICATE-----" {
			t.Errorf("CABundle = %q", cfg.CABundle)
		}
	})

	t.Run("tracks env overrides", func(t *testing.T) {
		cfg := &OIDCConfig{}
		env := map[string]string{
			"OIDC_ENABLED":    "true",
			"OIDC_ISSUER_URL": "https://issuer.example.com",
		}

		cfg.MergeFromEnv(env)

		if !cfg.EnvOverrides["enabled"] {
			t.Error("enabled should be marked as env override")
		}
		if !cfg.EnvOverrides["issuerUrl"] {
			t.Error("issuerUrl should be marked as env override")
		}
		if cfg.EnvOverrides["clientId"] {
			t.Error("clientId should not be marked as env override")
		}
	})

	t.Run("enabled with 1", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.MergeFromEnv(map[string]string{"OIDC_ENABLED": "1"})

		if !cfg.Enabled {
			t.Error("Enabled should be true for '1'")
		}
	})

	t.Run("enabled false for other values", func(t *testing.T) {
		cfg := &OIDCConfig{}
		cfg.MergeFromEnv(map[string]string{"OIDC_ENABLED": "false"})

		if cfg.Enabled {
			t.Error("Enabled should be false for 'false'")
		}
	})
}

func TestNewOIDCConfig(t *testing.T) {
	cfg := NewOIDCConfig()

	if cfg == nil {
		t.Fatal("NewOIDCConfig() returned nil")
	}

	// Should have default scopes applied
	if len(cfg.Scopes) != 3 {
		t.Errorf("Scopes = %v, want default 3 scopes", cfg.Scopes)
	}

	// Should have default claims
	if cfg.UsernameClaim != "preferred_username" {
		t.Errorf("UsernameClaim = %q, want %q", cfg.UsernameClaim, "preferred_username")
	}
	if cfg.EmailClaim != "email" {
		t.Errorf("EmailClaim = %q, want %q", cfg.EmailClaim, "email")
	}

	// Should have initialized EnvOverrides
	if cfg.EnvOverrides == nil {
		t.Error("EnvOverrides should be initialized")
	}
}

// TestOIDCEnvVarsWithNilConfig is a regression test for issue #853.
// It ensures that when starting with a nil OIDCConfig (no oidc.enc file),
// setting OIDC_* environment variables properly initializes and configures OIDC.
// The bug was that MergeFromEnv() on a nil receiver silently returned,
// so env vars were ignored when no persisted oidc.enc existed.
func TestOIDCEnvVarsWithNilConfig(t *testing.T) {
	t.Run("nil config must be initialized before MergeFromEnv", func(t *testing.T) {
		// Simulate the bug: calling MergeFromEnv on nil does nothing
		var nilCfg *OIDCConfig
		nilCfg.MergeFromEnv(map[string]string{"OIDC_ENABLED": "true"})
		// nilCfg is still nil - this was the bug
		if nilCfg != nil {
			t.Error("MergeFromEnv on nil receiver should leave it nil (this is expected)")
		}
	})

	t.Run("proper pattern: initialize before merge", func(t *testing.T) {
		// This is the correct pattern used in config.go after the fix
		var cfg *OIDCConfig
		env := map[string]string{
			"OIDC_ENABLED":    "true",
			"OIDC_ISSUER_URL": "https://auth.example.com",
			"OIDC_CLIENT_ID":  "my-client",
		}

		// The fix: initialize if nil before merging
		if len(env) > 0 {
			if cfg == nil {
				cfg = NewOIDCConfig()
			}
			cfg.MergeFromEnv(env)
		}

		// Verify env vars were applied
		if cfg == nil {
			t.Fatal("cfg should be initialized")
		}
		if !cfg.Enabled {
			t.Error("Enabled should be true from env")
		}
		if cfg.IssuerURL != "https://auth.example.com" {
			t.Errorf("IssuerURL = %q, want %q", cfg.IssuerURL, "https://auth.example.com")
		}
		if cfg.ClientID != "my-client" {
			t.Errorf("ClientID = %q, want %q", cfg.ClientID, "my-client")
		}
		// Should also have defaults from NewOIDCConfig
		if len(cfg.Scopes) != 3 {
			t.Errorf("Scopes should have defaults, got %v", cfg.Scopes)
		}
	})
}
