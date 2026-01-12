package config

import (
	"testing"
)

func TestNewSSOConfig(t *testing.T) {
	cfg := NewSSOConfig()
	if cfg == nil {
		t.Fatal("NewSSOConfig returned nil")
	}
	if cfg.Providers == nil {
		t.Error("Providers slice should not be nil")
	}
	if len(cfg.Providers) != 0 {
		t.Error("Providers slice should be empty")
	}
	if !cfg.AllowMultipleProviders {
		t.Error("AllowMultipleProviders should default to true")
	}
}

func TestSSOConfig_GetProvider(t *testing.T) {
	cfg := &SSOConfig{
		Providers: []SSOProvider{
			{ID: "p1", Name: "Provider 1"},
			{ID: "p2", Name: "Provider 2"},
		},
	}

	tests := []struct {
		name     string
		id       string
		wantName string
		wantNil  bool
	}{
		{"existing provider", "p1", "Provider 1", false},
		{"second provider", "p2", "Provider 2", false},
		{"non-existent", "p3", "", true},
		{"empty id", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := cfg.GetProvider(tt.id)
			if tt.wantNil {
				if p != nil {
					t.Errorf("GetProvider(%q) = %v, want nil", tt.id, p)
				}
			} else {
				if p == nil {
					t.Fatalf("GetProvider(%q) = nil, want non-nil", tt.id)
				}
				if p.Name != tt.wantName {
					t.Errorf("GetProvider(%q).Name = %q, want %q", tt.id, p.Name, tt.wantName)
				}
			}
		})
	}

	// Test nil config
	var nilCfg *SSOConfig
	if p := nilCfg.GetProvider("p1"); p != nil {
		t.Error("GetProvider on nil config should return nil")
	}
}

func TestSSOConfig_GetEnabledProviders(t *testing.T) {
	cfg := &SSOConfig{
		Providers: []SSOProvider{
			{ID: "p1", Name: "Provider 1", Enabled: false, Priority: 10},
			{ID: "p2", Name: "Provider 2", Enabled: true, Priority: 20},
			{ID: "p3", Name: "Provider 3", Enabled: true, Priority: 5},
			{ID: "p4", Name: "Provider 4", Enabled: true, Priority: 15},
		},
	}

	enabled := cfg.GetEnabledProviders()
	if len(enabled) != 3 {
		t.Fatalf("GetEnabledProviders() returned %d providers, want 3", len(enabled))
	}

	// Should be sorted by priority (ascending)
	expectedOrder := []string{"p3", "p4", "p2"}
	for i, p := range enabled {
		if p.ID != expectedOrder[i] {
			t.Errorf("GetEnabledProviders()[%d].ID = %q, want %q", i, p.ID, expectedOrder[i])
		}
	}

	// Test nil config
	var nilCfg *SSOConfig
	if result := nilCfg.GetEnabledProviders(); result != nil {
		t.Error("GetEnabledProviders on nil config should return nil")
	}
}

func TestSSOConfig_GetDefaultProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SSOConfig
		wantID  string
		wantNil bool
	}{
		{
			name: "explicit default",
			cfg: &SSOConfig{
				DefaultProviderID: "p2",
				Providers: []SSOProvider{
					{ID: "p1", Enabled: true, Priority: 1},
					{ID: "p2", Enabled: true, Priority: 2},
				},
			},
			wantID:  "p2",
			wantNil: false,
		},
		{
			name: "first enabled when no default",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Enabled: false, Priority: 1},
					{ID: "p2", Enabled: true, Priority: 10},
					{ID: "p3", Enabled: true, Priority: 5},
				},
			},
			wantID:  "p3", // lowest priority among enabled
			wantNil: false,
		},
		{
			name: "no enabled providers",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Enabled: false},
				},
			},
			wantNil: true,
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.cfg.GetDefaultProvider()
			if tt.wantNil {
				if p != nil {
					t.Errorf("GetDefaultProvider() = %v, want nil", p)
				}
			} else {
				if p == nil {
					t.Fatal("GetDefaultProvider() = nil, want non-nil")
				}
				if p.ID != tt.wantID {
					t.Errorf("GetDefaultProvider().ID = %q, want %q", p.ID, tt.wantID)
				}
			}
		})
	}
}

func TestSSOConfig_HasEnabledProviders(t *testing.T) {
	tests := []struct {
		name string
		cfg  *SSOConfig
		want bool
	}{
		{
			name: "has enabled",
			cfg: &SSOConfig{
				Providers: []SSOProvider{{ID: "p1", Enabled: true}},
			},
			want: true,
		},
		{
			name: "none enabled",
			cfg: &SSOConfig{
				Providers: []SSOProvider{{ID: "p1", Enabled: false}},
			},
			want: false,
		},
		{
			name: "empty providers",
			cfg:  &SSOConfig{Providers: []SSOProvider{}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasEnabledProviders(); got != tt.want {
				t.Errorf("HasEnabledProviders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSSOConfig_AddProvider(t *testing.T) {
	cfg := NewSSOConfig()

	// Add first provider
	err := cfg.AddProvider(SSOProvider{ID: "p1", Name: "Provider 1"})
	if err != nil {
		t.Fatalf("AddProvider() error = %v, want nil", err)
	}
	if len(cfg.Providers) != 1 {
		t.Errorf("len(Providers) = %d, want 1", len(cfg.Providers))
	}

	// Add second provider
	err = cfg.AddProvider(SSOProvider{ID: "p2", Name: "Provider 2"})
	if err != nil {
		t.Fatalf("AddProvider() error = %v, want nil", err)
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("len(Providers) = %d, want 2", len(cfg.Providers))
	}

	// Duplicate ID should fail
	err = cfg.AddProvider(SSOProvider{ID: "p1", Name: "Duplicate"})
	if err == nil {
		t.Error("AddProvider() with duplicate ID should return error")
	}

	// Empty ID should fail
	err = cfg.AddProvider(SSOProvider{ID: "", Name: "No ID"})
	if err == nil {
		t.Error("AddProvider() with empty ID should return error")
	}

	// Nil config should fail
	var nilCfg *SSOConfig
	err = nilCfg.AddProvider(SSOProvider{ID: "p1"})
	if err == nil {
		t.Error("AddProvider() on nil config should return error")
	}
}

func TestSSOConfig_UpdateProvider(t *testing.T) {
	cfg := &SSOConfig{
		Providers: []SSOProvider{
			{ID: "p1", Name: "Original"},
		},
	}

	// Update existing
	err := cfg.UpdateProvider(SSOProvider{ID: "p1", Name: "Updated"})
	if err != nil {
		t.Fatalf("UpdateProvider() error = %v, want nil", err)
	}
	if cfg.Providers[0].Name != "Updated" {
		t.Errorf("Provider name = %q, want %q", cfg.Providers[0].Name, "Updated")
	}

	// Update non-existent
	err = cfg.UpdateProvider(SSOProvider{ID: "p2", Name: "Non-existent"})
	if err == nil {
		t.Error("UpdateProvider() for non-existent provider should return error")
	}

	// Nil config
	var nilCfg *SSOConfig
	err = nilCfg.UpdateProvider(SSOProvider{ID: "p1"})
	if err == nil {
		t.Error("UpdateProvider() on nil config should return error")
	}
}

func TestSSOConfig_RemoveProvider(t *testing.T) {
	cfg := &SSOConfig{
		DefaultProviderID: "p1",
		Providers: []SSOProvider{
			{ID: "p1", Name: "Provider 1"},
			{ID: "p2", Name: "Provider 2"},
		},
	}

	// Remove non-existent
	err := cfg.RemoveProvider("p3")
	if err == nil {
		t.Error("RemoveProvider() for non-existent provider should return error")
	}

	// Remove existing (and default)
	err = cfg.RemoveProvider("p1")
	if err != nil {
		t.Fatalf("RemoveProvider() error = %v, want nil", err)
	}
	if len(cfg.Providers) != 1 {
		t.Errorf("len(Providers) = %d, want 1", len(cfg.Providers))
	}
	if cfg.DefaultProviderID != "" {
		t.Errorf("DefaultProviderID = %q, want empty (cleared)", cfg.DefaultProviderID)
	}

	// Remove last provider
	err = cfg.RemoveProvider("p2")
	if err != nil {
		t.Fatalf("RemoveProvider() error = %v, want nil", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("len(Providers) = %d, want 0", len(cfg.Providers))
	}

	// Nil config
	var nilCfg *SSOConfig
	err = nilCfg.RemoveProvider("p1")
	if err == nil {
		t.Error("RemoveProvider() on nil config should return error")
	}
}

func TestSSOConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SSOConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name:    "empty config",
			cfg:     &SSOConfig{},
			wantErr: false,
		},
		{
			name: "valid OIDC provider",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "oidc1",
						Name:    "OIDC Provider",
						Type:    SSOProviderTypeOIDC,
						Enabled: true,
						OIDC: &OIDCProviderConfig{
							IssuerURL: "https://idp.example.com",
							ClientID:  "client123",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid SAML provider with metadata URL",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "saml1",
						Name:    "SAML Provider",
						Type:    SSOProviderTypeSAML,
						Enabled: true,
						SAML: &SAMLProviderConfig{
							IDPMetadataURL: "https://idp.example.com/metadata",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid SAML provider with SSO URL",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "saml1",
						Name:    "SAML Provider",
						Type:    SSOProviderTypeSAML,
						Enabled: true,
						SAML: &SAMLProviderConfig{
							IDPSSOURL: "https://idp.example.com/sso",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing provider ID",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "", Name: "Test"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate provider ID",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Name: "Provider 1"},
					{ID: "p1", Name: "Provider 2"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing provider name",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Name: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid provider type",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Name: "Test", Type: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "enabled OIDC without config",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Name: "Test", Type: SSOProviderTypeOIDC, Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "enabled SAML without config",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{ID: "p1", Name: "Test", Type: SSOProviderTypeSAML, Enabled: true},
				},
			},
			wantErr: true,
		},
		{
			name: "OIDC missing issuer URL",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "p1",
						Name:    "Test",
						Type:    SSOProviderTypeOIDC,
						Enabled: true,
						OIDC:    &OIDCProviderConfig{ClientID: "client123"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "OIDC missing client ID",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "p1",
						Name:    "Test",
						Type:    SSOProviderTypeOIDC,
						Enabled: true,
						OIDC:    &OIDCProviderConfig{IssuerURL: "https://idp.example.com"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "SAML missing metadata/SSO URL",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "p1",
						Name:    "Test",
						Type:    SSOProviderTypeSAML,
						Enabled: true,
						SAML:    &SAMLProviderConfig{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "SAML signing without cert/key",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "p1",
						Name:    "Test",
						Type:    SSOProviderTypeSAML,
						Enabled: true,
						SAML: &SAMLProviderConfig{
							IDPMetadataURL: "https://idp.example.com/metadata",
							SignRequests:   true,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid default provider ID",
			cfg: &SSOConfig{
				DefaultProviderID: "nonexistent",
				Providers: []SSOProvider{
					{ID: "p1", Name: "Test"},
				},
			},
			wantErr: true,
		},
		{
			name: "disabled provider skips validation",
			cfg: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:      "p1",
						Name:    "Test",
						Type:    SSOProviderTypeOIDC,
						Enabled: false,
						OIDC:    nil, // Would fail if enabled
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSSOConfig_Clone(t *testing.T) {
	original := &SSOConfig{
		DefaultProviderID:      "p1",
		AllowMultipleProviders: true,
		Providers: []SSOProvider{
			{
				ID:             "p1",
				Name:           "Provider 1",
				Type:           SSOProviderTypeOIDC,
				Enabled:        true,
				AllowedGroups:  []string{"admin", "users"},
				AllowedDomains: []string{"example.com"},
				GroupRoleMappings: map[string]string{
					"admin": "administrator",
				},
				OIDC: &OIDCProviderConfig{
					IssuerURL:    "https://idp.example.com",
					ClientID:     "client123",
					ClientSecret: "secret",
					Scopes:       []string{"openid", "profile"},
				},
			},
			{
				ID:      "p2",
				Name:    "Provider 2",
				Type:    SSOProviderTypeSAML,
				Enabled: false,
				SAML: &SAMLProviderConfig{
					IDPMetadataURL: "https://idp2.example.com/metadata",
				},
			},
		},
	}

	clone := original.Clone()

	// Verify deep copy
	if clone == original {
		t.Error("Clone() returned same pointer")
	}
	if clone.DefaultProviderID != original.DefaultProviderID {
		t.Error("DefaultProviderID not cloned")
	}
	if clone.AllowMultipleProviders != original.AllowMultipleProviders {
		t.Error("AllowMultipleProviders not cloned")
	}
	if len(clone.Providers) != len(original.Providers) {
		t.Error("Providers slice length mismatch")
	}

	// Modify clone and verify original unchanged
	clone.Providers[0].Name = "Modified"
	if original.Providers[0].Name == "Modified" {
		t.Error("Modifying clone affected original - not a deep copy")
	}

	clone.Providers[0].AllowedGroups[0] = "modified"
	if original.Providers[0].AllowedGroups[0] == "modified" {
		t.Error("Modifying clone's AllowedGroups affected original")
	}

	clone.Providers[0].GroupRoleMappings["admin"] = "modified"
	if original.Providers[0].GroupRoleMappings["admin"] == "modified" {
		t.Error("Modifying clone's GroupRoleMappings affected original")
	}

	clone.Providers[0].OIDC.ClientSecret = "modified"
	if original.Providers[0].OIDC.ClientSecret == "modified" {
		t.Error("Modifying clone's OIDC config affected original")
	}

	// Test nil clone
	var nilCfg *SSOConfig
	if result := nilCfg.Clone(); result != nil {
		t.Error("Clone() on nil should return nil")
	}
}

func TestMigrateFromOIDCConfig(t *testing.T) {
	tests := []struct {
		name    string
		oidc    *OIDCConfig
		wantLen int
		checkFn func(*testing.T, *SSOConfig)
	}{
		{
			name:    "nil OIDC config",
			oidc:    nil,
			wantLen: 0,
		},
		{
			name:    "empty OIDC config",
			oidc:    &OIDCConfig{},
			wantLen: 0,
		},
		{
			name: "valid OIDC config",
			oidc: &OIDCConfig{
				Enabled:        true,
				IssuerURL:      "https://idp.example.com",
				ClientID:       "client123",
				ClientSecret:   "secret",
				RedirectURL:    "https://app.example.com/callback",
				AllowedGroups:  []string{"admin"},
				AllowedDomains: []string{"example.com"},
				GroupsClaim:    "groups",
				GroupRoleMappings: map[string]string{
					"admin": "administrator",
				},
			},
			wantLen: 1,
			checkFn: func(t *testing.T, sso *SSOConfig) {
				p := sso.GetProvider("legacy-oidc")
				if p == nil {
					t.Fatal("legacy-oidc provider not found")
				}
				if p.Type != SSOProviderTypeOIDC {
					t.Errorf("Type = %q, want %q", p.Type, SSOProviderTypeOIDC)
				}
				if !p.Enabled {
					t.Error("Provider should be enabled")
				}
				if p.OIDC.IssuerURL != "https://idp.example.com" {
					t.Errorf("IssuerURL = %q, want %q", p.OIDC.IssuerURL, "https://idp.example.com")
				}
				if p.OIDC.ClientID != "client123" {
					t.Errorf("ClientID = %q, want %q", p.OIDC.ClientID, "client123")
				}
				if len(p.AllowedGroups) != 1 || p.AllowedGroups[0] != "admin" {
					t.Errorf("AllowedGroups = %v, want [admin]", p.AllowedGroups)
				}
				if sso.DefaultProviderID != "legacy-oidc" {
					t.Errorf("DefaultProviderID = %q, want %q", sso.DefaultProviderID, "legacy-oidc")
				}
			},
		},
		{
			name: "OIDC config missing client ID (not migrated)",
			oidc: &OIDCConfig{
				IssuerURL: "https://idp.example.com",
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sso := MigrateFromOIDCConfig(tt.oidc)
			if sso == nil {
				t.Fatal("MigrateFromOIDCConfig returned nil")
			}
			if len(sso.Providers) != tt.wantLen {
				t.Errorf("len(Providers) = %d, want %d", len(sso.Providers), tt.wantLen)
			}
			if tt.checkFn != nil {
				tt.checkFn(t, sso)
			}
		})
	}
}

func TestSSOConfig_ToLegacyOIDCConfig(t *testing.T) {
	tests := []struct {
		name    string
		sso     *SSOConfig
		wantNil bool
		checkFn func(*testing.T, *OIDCConfig)
	}{
		{
			name:    "nil SSO config",
			sso:     nil,
			wantNil: false, // Returns NewOIDCConfig()
		},
		{
			name:    "no OIDC providers",
			sso:     &SSOConfig{},
			wantNil: false,
		},
		{
			name: "SAML only",
			sso: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:   "saml1",
						Type: SSOProviderTypeSAML,
						SAML: &SAMLProviderConfig{},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "with OIDC provider",
			sso: &SSOConfig{
				Providers: []SSOProvider{
					{
						ID:            "oidc1",
						Type:          SSOProviderTypeOIDC,
						Enabled:       true,
						AllowedGroups: []string{"admin"},
						GroupsClaim:   "groups",
						OIDC: &OIDCProviderConfig{
							IssuerURL:    "https://idp.example.com",
							ClientID:     "client123",
							ClientSecret: "secret",
						},
					},
				},
			},
			checkFn: func(t *testing.T, oidc *OIDCConfig) {
				if !oidc.Enabled {
					t.Error("Enabled should be true")
				}
				if oidc.IssuerURL != "https://idp.example.com" {
					t.Errorf("IssuerURL = %q, want %q", oidc.IssuerURL, "https://idp.example.com")
				}
				if oidc.ClientID != "client123" {
					t.Errorf("ClientID = %q, want %q", oidc.ClientID, "client123")
				}
				if oidc.GroupsClaim != "groups" {
					t.Errorf("GroupsClaim = %q, want %q", oidc.GroupsClaim, "groups")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidc := tt.sso.ToLegacyOIDCConfig()
			if oidc == nil {
				t.Fatal("ToLegacyOIDCConfig returned nil")
			}
			if tt.checkFn != nil {
				tt.checkFn(t, oidc)
			}
		})
	}
}

// Test URL validation
func TestValidateOIDCProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *OIDCProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &OIDCProviderConfig{
				IssuerURL: "https://idp.example.com",
				ClientID:  "client123",
			},
			wantErr: false,
		},
		{
			name: "invalid issuer URL",
			cfg: &OIDCProviderConfig{
				IssuerURL: "not-a-url",
				ClientID:  "client123",
			},
			wantErr: true,
		},
		{
			name: "empty issuer URL",
			cfg: &OIDCProviderConfig{
				IssuerURL: "",
				ClientID:  "client123",
			},
			wantErr: true,
		},
		{
			name: "whitespace issuer URL",
			cfg: &OIDCProviderConfig{
				IssuerURL: "   ",
				ClientID:  "client123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOIDCProvider(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOIDCProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSAMLProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SAMLProviderConfig
		wantErr bool
	}{
		{
			name: "valid with metadata URL",
			cfg: &SAMLProviderConfig{
				IDPMetadataURL: "https://idp.example.com/metadata",
			},
			wantErr: false,
		},
		{
			name: "valid with metadata XML",
			cfg: &SAMLProviderConfig{
				IDPMetadataXML: "<xml>...</xml>",
			},
			wantErr: false,
		},
		{
			name: "valid with SSO URL",
			cfg: &SAMLProviderConfig{
				IDPSSOURL: "https://idp.example.com/sso",
			},
			wantErr: false,
		},
		{
			name:    "missing all required fields",
			cfg:     &SAMLProviderConfig{},
			wantErr: true,
		},
		{
			name: "invalid metadata URL",
			cfg: &SAMLProviderConfig{
				IDPMetadataURL: "not-a-url",
			},
			wantErr: true,
		},
		{
			name: "invalid SSO URL",
			cfg: &SAMLProviderConfig{
				IDPSSOURL: "not-a-url",
			},
			wantErr: true,
		},
		{
			name: "signing enabled without cert",
			cfg: &SAMLProviderConfig{
				IDPMetadataURL: "https://idp.example.com/metadata",
				SignRequests:   true,
			},
			wantErr: true,
		},
		{
			name: "signing enabled with cert but no key",
			cfg: &SAMLProviderConfig{
				IDPMetadataURL: "https://idp.example.com/metadata",
				SignRequests:   true,
				SPCertificate:  "-----BEGIN CERTIFICATE-----...",
			},
			wantErr: true,
		},
		{
			name: "signing enabled with cert file and key file",
			cfg: &SAMLProviderConfig{
				IDPMetadataURL: "https://idp.example.com/metadata",
				SignRequests:   true,
				SPCertFile:     "/path/to/cert.pem",
				SPKeyFile:      "/path/to/key.pem",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSAMLProvider(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSAMLProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
