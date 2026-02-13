package config

import (
	"fmt"
	"net/url"
	"strings"
)

// SSOProviderType defines the type of SSO provider
type SSOProviderType string

const (
	SSOProviderTypeOIDC SSOProviderType = "oidc"
	SSOProviderTypeSAML SSOProviderType = "saml"
)

// SSOProvider represents a single SSO identity provider configuration.
// It can be either OIDC or SAML type.
type SSOProvider struct {
	// Common fields
	ID          string          `json:"id"`                    // Unique identifier (auto-generated if empty)
	Name        string          `json:"name"`                  // Display name (e.g., "Corporate SSO", "Google", "Okta")
	Type        SSOProviderType `json:"type"`                  // "oidc" or "saml"
	Enabled     bool            `json:"enabled"`               // Whether this provider is active
	DisplayName string          `json:"displayName,omitempty"` // Button text on login page (defaults to Name)
	IconURL     string          `json:"iconUrl,omitempty"`     // Optional icon URL for login button
	Priority    int             `json:"priority,omitempty"`    // Display order (lower = higher priority)

	// Access restrictions (apply to both OIDC and SAML)
	AllowedGroups  []string `json:"allowedGroups,omitempty"`  // Restrict to specific groups
	AllowedDomains []string `json:"allowedDomains,omitempty"` // Restrict to email domains
	AllowedEmails  []string `json:"allowedEmails,omitempty"`  // Restrict to specific emails

	// Role mapping (apply to both OIDC and SAML)
	GroupsClaim       string            `json:"groupsClaim,omitempty"`       // Claim/attribute containing groups
	GroupRoleMappings map[string]string `json:"groupRoleMappings,omitempty"` // Map groups to Pulse roles

	// OIDC-specific configuration
	OIDC *OIDCProviderConfig `json:"oidc,omitempty"`

	// SAML-specific configuration
	SAML *SAMLProviderConfig `json:"saml,omitempty"`
}

// OIDCProviderConfig contains OIDC-specific settings
type OIDCProviderConfig struct {
	IssuerURL     string   `json:"issuerUrl"`
	ClientID      string   `json:"clientId"`
	ClientSecret  string   `json:"clientSecret,omitempty"`
	RedirectURL   string   `json:"redirectUrl,omitempty"` // Optional - auto-detected if empty
	LogoutURL     string   `json:"logoutUrl,omitempty"`   // Optional OIDC end-session URL
	Scopes        []string `json:"scopes,omitempty"`      // Defaults to ["openid", "profile", "email"]
	UsernameClaim string   `json:"usernameClaim,omitempty"`
	EmailClaim    string   `json:"emailClaim,omitempty"`
	CABundle      string   `json:"caBundle,omitempty"` // Path to CA bundle for self-signed certs

	// Internal tracking
	ClientSecretSet bool            `json:"clientSecretSet,omitempty"` // True if secret is configured (don't expose actual value)
	EnvOverrides    map[string]bool `json:"-"`                         // Fields locked by env vars
}

// SAMLProviderConfig contains SAML 2.0 specific settings
type SAMLProviderConfig struct {
	// Identity Provider settings (from IdP metadata)
	IDPMetadataURL string `json:"idpMetadataUrl,omitempty"` // URL to fetch IdP metadata (preferred)
	IDPMetadataXML string `json:"idpMetadataXml,omitempty"` // Raw XML metadata (alternative to URL)
	IDPSSOURL      string `json:"idpSsoUrl,omitempty"`      // SSO URL (extracted from metadata or manual)
	IDPSLOURL      string `json:"idpSloUrl,omitempty"`      // Single Logout URL (optional)
	IDPCertificate string `json:"idpCertificate,omitempty"` // IdP signing certificate (PEM format)
	IDPCertFile    string `json:"idpCertFile,omitempty"`    // Path to IdP certificate file (alternative)
	IDPEntityID    string `json:"idpEntityId,omitempty"`    // IdP Entity ID (extracted from metadata or manual)
	IDPIssuer      string `json:"idpIssuer,omitempty"`      // Alias for IDPEntityID for compatibility

	// Service Provider settings (Pulse as SP)
	SPEntityID           string `json:"spEntityId,omitempty"`           // Pulse's Entity ID (auto-generated if empty)
	SPACSPath            string `json:"spAcsPath,omitempty"`            // Assertion Consumer Service path (default: /api/saml/{id}/acs)
	SPMetadataPath       string `json:"spMetadataPath,omitempty"`       // SP Metadata path (default: /api/saml/{id}/metadata)
	SPCertificate        string `json:"spCertificate,omitempty"`        // SP signing certificate (PEM format, optional)
	SPPrivateKey         string `json:"spPrivateKey,omitempty"`         // SP private key (PEM format, optional)
	SPCertFile           string `json:"spCertFile,omitempty"`           // Path to SP certificate file
	SPKeyFile            string `json:"spKeyFile,omitempty"`            // Path to SP private key file
	SignRequests         bool   `json:"signRequests,omitempty"`         // Sign SAML requests (requires SP cert/key)
	WantAssertionsSigned bool   `json:"wantAssertionsSigned,omitempty"` // Require signed assertions (recommended)
	AllowUnencrypted     bool   `json:"allowUnencrypted,omitempty"`     // Allow unencrypted assertions (less secure)

	// Attribute mapping
	UsernameAttr  string `json:"usernameAttr,omitempty"` // SAML attribute for username (default: NameID)
	EmailAttr     string `json:"emailAttr,omitempty"`    // SAML attribute for email
	GroupsAttr    string `json:"groupsAttr,omitempty"`   // SAML attribute for groups
	FirstNameAttr string `json:"firstNameAttr,omitempty"`
	LastNameAttr  string `json:"lastNameAttr,omitempty"`

	// Advanced settings
	NameIDFormat       string `json:"nameIdFormat,omitempty"`       // NameID format (default: unspecified)
	ForceAuthn         bool   `json:"forceAuthn,omitempty"`         // Force re-authentication
	AllowIDPInitiated  bool   `json:"allowIdpInitiated,omitempty"`  // Allow IdP-initiated SSO (security risk)
	RelayStateTemplate string `json:"relayStateTemplate,omitempty"` // Custom relay state template
}

// SSOConfig holds the complete SSO configuration with multiple providers
type SSOConfig struct {
	// Providers is the list of configured SSO providers
	Providers []SSOProvider `json:"providers,omitempty"`

	// DefaultProviderID is the provider to use when none is specified
	// If empty and only one provider exists, that provider is used
	DefaultProviderID string `json:"defaultProviderId,omitempty"`

	// AllowMultipleProviders enables showing provider selection on login page
	// If false and multiple providers exist, only the default is shown
	AllowMultipleProviders bool `json:"allowMultipleProviders,omitempty"`
}

// NewSSOConfig creates a new empty SSO configuration
func NewSSOConfig() *SSOConfig {
	return &SSOConfig{
		Providers:              []SSOProvider{},
		AllowMultipleProviders: true,
	}
}

// GetProvider returns a provider by ID
func (c *SSOConfig) GetProvider(id string) *SSOProvider {
	if c == nil {
		return nil
	}
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			return &c.Providers[i]
		}
	}
	return nil
}

// GetEnabledProviders returns all enabled providers sorted by priority
func (c *SSOConfig) GetEnabledProviders() []SSOProvider {
	if c == nil {
		return nil
	}
	result := make([]SSOProvider, 0)
	for _, p := range c.Providers {
		if p.Enabled {
			result = append(result, p)
		}
	}
	// Sort by priority (lower = first)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Priority < result[i].Priority {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// GetDefaultProvider returns the default provider
func (c *SSOConfig) GetDefaultProvider() *SSOProvider {
	if c == nil {
		return nil
	}
	if c.DefaultProviderID != "" {
		return c.GetProvider(c.DefaultProviderID)
	}
	enabled := c.GetEnabledProviders()
	if len(enabled) > 0 {
		return &enabled[0]
	}
	return nil
}

// HasEnabledProviders returns true if at least one provider is enabled
func (c *SSOConfig) HasEnabledProviders() bool {
	return len(c.GetEnabledProviders()) > 0
}

// AddProvider adds a new provider to the configuration
func (c *SSOConfig) AddProvider(p SSOProvider) error {
	if c == nil {
		return fmt.Errorf("sso config is nil")
	}
	if p.ID == "" {
		return fmt.Errorf("provider id is required")
	}
	if c.GetProvider(p.ID) != nil {
		return fmt.Errorf("provider with id %q already exists", p.ID)
	}
	c.Providers = append(c.Providers, p)
	return nil
}

// UpdateProvider updates an existing provider
func (c *SSOConfig) UpdateProvider(p SSOProvider) error {
	if c == nil {
		return fmt.Errorf("sso config is nil")
	}
	for i := range c.Providers {
		if c.Providers[i].ID == p.ID {
			c.Providers[i] = p
			return nil
		}
	}
	return fmt.Errorf("provider with id %q not found", p.ID)
}

// RemoveProvider removes a provider by ID
func (c *SSOConfig) RemoveProvider(id string) error {
	if c == nil {
		return fmt.Errorf("sso config is nil")
	}
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			c.Providers = append(c.Providers[:i], c.Providers[i+1:]...)
			// Clear default if we removed it
			if c.DefaultProviderID == id {
				c.DefaultProviderID = ""
			}
			return nil
		}
	}
	return fmt.Errorf("provider with id %q not found", id)
}

// Validate checks the SSO configuration for errors
func (c *SSOConfig) Validate() error {
	if c == nil {
		return nil
	}

	seenIDs := make(map[string]bool)
	for i, p := range c.Providers {
		if p.ID == "" {
			return fmt.Errorf("provider %d: id is required", i)
		}
		if seenIDs[p.ID] {
			return fmt.Errorf("provider %d: duplicate id %q", i, p.ID)
		}
		seenIDs[p.ID] = true

		if p.Name == "" {
			return fmt.Errorf("provider %q: name is required", p.ID)
		}

		if p.Type != SSOProviderTypeOIDC && p.Type != SSOProviderTypeSAML {
			return fmt.Errorf("provider %q: invalid type %q (must be 'oidc' or 'saml')", p.ID, p.Type)
		}

		if p.Enabled {
			if err := validateProvider(&p); err != nil {
				return fmt.Errorf("provider %q: %w", p.ID, err)
			}
		}
	}

	if c.DefaultProviderID != "" && c.GetProvider(c.DefaultProviderID) == nil {
		return fmt.Errorf("default provider %q not found", c.DefaultProviderID)
	}

	return nil
}

func validateProvider(p *SSOProvider) error {
	switch p.Type {
	case SSOProviderTypeOIDC:
		if p.OIDC == nil {
			return fmt.Errorf("oidc configuration is required for oidc provider")
		}
		return validateOIDCProvider(p.OIDC)
	case SSOProviderTypeSAML:
		if p.SAML == nil {
			return fmt.Errorf("saml configuration is required for saml provider")
		}
		return validateSAMLProvider(p.SAML)
	}
	return nil
}

func validateOIDCProvider(cfg *OIDCProviderConfig) error {
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return fmt.Errorf("issuer url is required")
	}
	if _, err := url.ParseRequestURI(cfg.IssuerURL); err != nil {
		return fmt.Errorf("invalid issuer url: %w", err)
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return fmt.Errorf("client id is required")
	}
	return nil
}

func validateSAMLProvider(cfg *SAMLProviderConfig) error {
	// Must have either metadata URL, metadata XML, or manual SSO URL
	hasMetadata := strings.TrimSpace(cfg.IDPMetadataURL) != "" || strings.TrimSpace(cfg.IDPMetadataXML) != ""
	hasManual := strings.TrimSpace(cfg.IDPSSOURL) != ""

	if !hasMetadata && !hasManual {
		return fmt.Errorf("either idp metadata (url or xml) or idp sso url is required")
	}

	if cfg.IDPMetadataURL != "" {
		if _, err := url.ParseRequestURI(cfg.IDPMetadataURL); err != nil {
			return fmt.Errorf("invalid idp metadata url: %w", err)
		}
	}

	if cfg.IDPSSOURL != "" {
		if _, err := url.ParseRequestURI(cfg.IDPSSOURL); err != nil {
			return fmt.Errorf("invalid idp sso url: %w", err)
		}
	}

	// If signing is enabled, certificate and key are required
	if cfg.SignRequests {
		hasCert := strings.TrimSpace(cfg.SPCertificate) != "" || strings.TrimSpace(cfg.SPCertFile) != ""
		hasKey := strings.TrimSpace(cfg.SPPrivateKey) != "" || strings.TrimSpace(cfg.SPKeyFile) != ""
		if !hasCert || !hasKey {
			return fmt.Errorf("sp certificate and private key are required when signing requests")
		}
	}

	return nil
}

// Clone creates a deep copy of the SSO configuration
func (c *SSOConfig) Clone() *SSOConfig {
	if c == nil {
		return nil
	}

	clone := &SSOConfig{
		DefaultProviderID:      c.DefaultProviderID,
		AllowMultipleProviders: c.AllowMultipleProviders,
		Providers:              make([]SSOProvider, len(c.Providers)),
	}

	for i, p := range c.Providers {
		clone.Providers[i] = cloneProvider(p)
	}

	return clone
}

func cloneProvider(p SSOProvider) SSOProvider {
	clone := p

	// Clone slices
	if p.AllowedGroups != nil {
		clone.AllowedGroups = append([]string{}, p.AllowedGroups...)
	}
	if p.AllowedDomains != nil {
		clone.AllowedDomains = append([]string{}, p.AllowedDomains...)
	}
	if p.AllowedEmails != nil {
		clone.AllowedEmails = append([]string{}, p.AllowedEmails...)
	}
	if p.GroupRoleMappings != nil {
		clone.GroupRoleMappings = make(map[string]string, len(p.GroupRoleMappings))
		for k, v := range p.GroupRoleMappings {
			clone.GroupRoleMappings[k] = v
		}
	}

	// Clone OIDC config
	if p.OIDC != nil {
		oidc := *p.OIDC
		if p.OIDC.Scopes != nil {
			oidc.Scopes = append([]string{}, p.OIDC.Scopes...)
		}
		if p.OIDC.EnvOverrides != nil {
			oidc.EnvOverrides = make(map[string]bool, len(p.OIDC.EnvOverrides))
			for k, v := range p.OIDC.EnvOverrides {
				oidc.EnvOverrides[k] = v
			}
		}
		clone.OIDC = &oidc
	}

	// Clone SAML config
	if p.SAML != nil {
		saml := *p.SAML
		clone.SAML = &saml
	}

	return clone
}

// MigrateFromOIDCConfig converts legacy OIDCConfig to new SSOConfig format
func MigrateFromOIDCConfig(oidc *OIDCConfig) *SSOConfig {
	if oidc == nil {
		return NewSSOConfig()
	}

	sso := NewSSOConfig()

	// Only migrate if OIDC was actually configured
	if oidc.IssuerURL == "" || oidc.ClientID == "" {
		return sso
	}

	provider := SSOProvider{
		ID:                "legacy-oidc",
		Name:              "Single Sign-On",
		Type:              SSOProviderTypeOIDC,
		Enabled:           oidc.Enabled,
		AllowedGroups:     oidc.AllowedGroups,
		AllowedDomains:    oidc.AllowedDomains,
		AllowedEmails:     oidc.AllowedEmails,
		GroupsClaim:       oidc.GroupsClaim,
		GroupRoleMappings: oidc.GroupRoleMappings,
		OIDC: &OIDCProviderConfig{
			IssuerURL:       oidc.IssuerURL,
			ClientID:        oidc.ClientID,
			ClientSecret:    oidc.ClientSecret,
			RedirectURL:     oidc.RedirectURL,
			LogoutURL:       oidc.LogoutURL,
			Scopes:          oidc.Scopes,
			UsernameClaim:   oidc.UsernameClaim,
			EmailClaim:      oidc.EmailClaim,
			CABundle:        oidc.CABundle,
			ClientSecretSet: oidc.ClientSecret != "",
			EnvOverrides:    oidc.EnvOverrides,
		},
	}

	sso.Providers = append(sso.Providers, provider)
	sso.DefaultProviderID = provider.ID

	return sso
}

// ToLegacyOIDCConfig converts the first OIDC provider back to legacy format for backwards compatibility
func (c *SSOConfig) ToLegacyOIDCConfig() *OIDCConfig {
	if c == nil {
		return NewOIDCConfig()
	}

	for _, p := range c.Providers {
		if p.Type == SSOProviderTypeOIDC && p.OIDC != nil {
			return &OIDCConfig{
				Enabled:           p.Enabled,
				IssuerURL:         p.OIDC.IssuerURL,
				ClientID:          p.OIDC.ClientID,
				ClientSecret:      p.OIDC.ClientSecret,
				RedirectURL:       p.OIDC.RedirectURL,
				LogoutURL:         p.OIDC.LogoutURL,
				Scopes:            p.OIDC.Scopes,
				UsernameClaim:     p.OIDC.UsernameClaim,
				EmailClaim:        p.OIDC.EmailClaim,
				GroupsClaim:       p.GroupsClaim,
				AllowedGroups:     p.AllowedGroups,
				AllowedDomains:    p.AllowedDomains,
				AllowedEmails:     p.AllowedEmails,
				GroupRoleMappings: p.GroupRoleMappings,
				CABundle:          p.OIDC.CABundle,
				EnvOverrides:      p.OIDC.EnvOverrides,
			}
		}
	}

	return NewOIDCConfig()
}
