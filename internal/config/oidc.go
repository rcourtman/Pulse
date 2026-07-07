package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// defaultOIDCScopes defines the scopes we request when none are provided.
var defaultOIDCScopes = []string{"openid", "profile", "email"}

// DefaultOIDCCallbackPath is the path we expose for the OIDC redirect handler.
const DefaultOIDCCallbackPath = "/api/oidc/callback"

// LegacyOIDCProviderID is the synthetic SSO provider ID used for v5 OIDC
// configuration migrated from oidc.enc or OIDC_* environment variables.
const LegacyOIDCProviderID = "legacy-oidc"

// OIDCConfig captures configuration required to integrate with an OpenID Connect provider.
type OIDCConfig struct {
	Enabled           bool              `json:"enabled"`
	IssuerURL         string            `json:"issuerUrl"`
	ClientID          string            `json:"clientId"`
	ClientSecret      string            `json:"clientSecret,omitempty"`
	RedirectURL       string            `json:"redirectUrl"`
	LogoutURL         string            `json:"logoutUrl,omitempty"`
	Scopes            []string          `json:"scopes,omitempty"`
	UsernameClaim     string            `json:"usernameClaim,omitempty"`
	EmailClaim        string            `json:"emailClaim,omitempty"`
	GroupsClaim       string            `json:"groupsClaim,omitempty"`
	AllowedGroups     []string          `json:"allowedGroups,omitempty"`
	AllowedDomains    []string          `json:"allowedDomains,omitempty"`
	AllowedEmails     []string          `json:"allowedEmails,omitempty"`
	GroupRoleMappings map[string]string `json:"groupRoleMappings,omitempty"`
	CABundle          string            `json:"caBundle,omitempty"`
	EnvOverrides      map[string]bool   `json:"-"`
}

// NewOIDCConfig returns an instance populated with sensible defaults.
func NewOIDCConfig() *OIDCConfig {
	cfg := &OIDCConfig{}
	cfg.ApplyDefaults("")
	return cfg
}

// Clone returns a deep copy of the configuration.
func (c *OIDCConfig) Clone() *OIDCConfig {
	if c == nil {
		return nil
	}

	clone := *c
	clone.Scopes = append([]string{}, c.Scopes...)
	clone.AllowedGroups = append([]string{}, c.AllowedGroups...)
	clone.AllowedDomains = append([]string{}, c.AllowedDomains...)
	clone.AllowedEmails = append([]string{}, c.AllowedEmails...)
	clone.CABundle = c.CABundle
	if c.GroupRoleMappings != nil {
		clone.GroupRoleMappings = make(map[string]string, len(c.GroupRoleMappings))
		for k, v := range c.GroupRoleMappings {
			clone.GroupRoleMappings[k] = v
		}
	}
	if c.EnvOverrides != nil {
		clone.EnvOverrides = make(map[string]bool, len(c.EnvOverrides))
		for k, v := range c.EnvOverrides {
			clone.EnvOverrides[k] = v
		}
	}
	return &clone
}

// ApplyDefaults normalises the configuration and injects default values where needed.
func (c *OIDCConfig) ApplyDefaults(publicURL string) {
	if c == nil {
		return
	}

	c.CABundle = strings.TrimSpace(c.CABundle)

	if len(c.Scopes) == 0 {
		c.Scopes = append([]string{}, defaultOIDCScopes...)
	} else {
		c.Scopes = normaliseList(c.Scopes)
	}

	if c.UsernameClaim = strings.TrimSpace(c.UsernameClaim); c.UsernameClaim == "" {
		c.UsernameClaim = "preferred_username"
	}
	if c.EmailClaim = strings.TrimSpace(c.EmailClaim); c.EmailClaim == "" {
		c.EmailClaim = "email"
	}
	c.GroupsClaim = strings.TrimSpace(c.GroupsClaim)

	c.AllowedGroups = normaliseList(c.AllowedGroups)
	c.AllowedDomains = normaliseList(c.AllowedDomains)
	c.AllowedEmails = normaliseList(c.AllowedEmails)

	if c.GroupRoleMappings == nil {
		c.GroupRoleMappings = make(map[string]string)
	}

	if c.EnvOverrides == nil {
		c.EnvOverrides = make(map[string]bool)
	}

	if strings.TrimSpace(c.RedirectURL) == "" {
		c.RedirectURL = DefaultRedirectURL(publicURL)
	}
}

// DefaultRedirectURL builds a redirect URL using the provided public base URL.
func DefaultRedirectURL(publicURL string) string {
	if strings.TrimSpace(publicURL) == "" {
		return ""
	}
	base := strings.TrimRight(publicURL, "/")
	return base + DefaultOIDCCallbackPath
}

// LegacyOIDCConfigToSSOConfig converts the v5 single-provider OIDC
// configuration into the v6 multi-provider SSO model.
func LegacyOIDCConfigToSSOConfig(legacy *OIDCConfig, publicURL string) (*SSOConfig, bool) {
	provider, ok := LegacyOIDCProviderFromConfig(legacy, publicURL, false)
	if !ok {
		return nil, false
	}
	cfg := NewSSOConfig()
	cfg.Providers = append(cfg.Providers, *provider)
	cfg.DefaultProviderID = provider.ID
	return cfg, true
}

// LegacyOIDCProviderFromConfig converts a v5 OIDC configuration into a single
// v6 SSO OIDC provider.
func LegacyOIDCProviderFromConfig(legacy *OIDCConfig, publicURL string, runtimeManaged bool) (*SSOProvider, bool) {
	if legacy == nil || !legacy.Enabled {
		return nil, false
	}

	cfg := legacy.Clone()
	cfg.ApplyDefaults(publicURL)
	if strings.TrimSpace(cfg.IssuerURL) == "" || strings.TrimSpace(cfg.ClientID) == "" {
		return nil, false
	}

	provider := &SSOProvider{
		ID:                LegacyOIDCProviderID,
		Name:              "Single Sign-On",
		Type:              SSOProviderTypeOIDC,
		Enabled:           true,
		DisplayName:       "Single Sign-On",
		AllowedGroups:     append([]string{}, cfg.AllowedGroups...),
		AllowedDomains:    append([]string{}, cfg.AllowedDomains...),
		AllowedEmails:     append([]string{}, cfg.AllowedEmails...),
		GroupsClaim:       cfg.GroupsClaim,
		GroupRoleMappings: cloneStringMap(cfg.GroupRoleMappings),
		RuntimeManaged:    runtimeManaged,
		OIDC: &OIDCProviderConfig{
			IssuerURL:       strings.TrimSpace(cfg.IssuerURL),
			ClientID:        strings.TrimSpace(cfg.ClientID),
			ClientSecret:    cfg.ClientSecret,
			RedirectURL:     cfg.RedirectURL,
			LogoutURL:       cfg.LogoutURL,
			Scopes:          append([]string{}, cfg.Scopes...),
			UsernameClaim:   cfg.UsernameClaim,
			EmailClaim:      cfg.EmailClaim,
			CABundle:        cfg.CABundle,
			ClientSecretSet: cfg.ClientSecret != "",
			EnvOverrides:    cloneBoolMap(cfg.EnvOverrides),
		},
	}
	return provider, true
}

// LegacyOIDCEnvProvider converts OIDC_* environment variables into a
// runtime-managed SSO provider.
func LegacyOIDCEnvProvider(publicURL string) (*SSOProvider, bool) {
	legacy, ok := LegacyOIDCConfigFromEnv(publicURL)
	if !ok {
		return nil, false
	}
	return LegacyOIDCProviderFromConfig(legacy, publicURL, true)
}

// LegacyOIDCConfigFromEnv reads the v5 OIDC_* environment contract.
func LegacyOIDCConfigFromEnv(publicURL string) (*OIDCConfig, bool) {
	enabledRaw, enabledSet := envValue("OIDC_ENABLED")
	if !enabledSet || !truthyEnv(enabledRaw) {
		return nil, false
	}

	cfg := &OIDCConfig{
		Enabled:           true,
		IssuerURL:         envTrim("OIDC_ISSUER_URL"),
		ClientID:          envTrim("OIDC_CLIENT_ID"),
		ClientSecret:      envTrim("OIDC_CLIENT_SECRET"),
		RedirectURL:       envTrim("OIDC_REDIRECT_URL"),
		LogoutURL:         envTrim("OIDC_LOGOUT_URL"),
		UsernameClaim:     envTrim("OIDC_USERNAME_CLAIM"),
		EmailClaim:        envTrim("OIDC_EMAIL_CLAIM"),
		GroupsClaim:       envTrim("OIDC_GROUPS_CLAIM"),
		CABundle:          envTrim("OIDC_CA_BUNDLE"),
		Scopes:            splitOIDCEnvList(envTrim("OIDC_SCOPES")),
		AllowedGroups:     splitOIDCEnvList(envTrim("OIDC_ALLOWED_GROUPS")),
		AllowedDomains:    splitOIDCEnvList(envTrim("OIDC_ALLOWED_DOMAINS")),
		AllowedEmails:     splitOIDCEnvList(envTrim("OIDC_ALLOWED_EMAILS")),
		GroupRoleMappings: parseOIDCGroupRoleMappings(envTrim("OIDC_GROUP_ROLE_MAPPINGS")),
		EnvOverrides:      map[string]bool{"enabled": true},
	}

	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_ISSUER_URL", "issuerUrl")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_CLIENT_ID", "clientId")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_CLIENT_SECRET", "clientSecret")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_REDIRECT_URL", "redirectUrl")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_LOGOUT_URL", "logoutUrl")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_SCOPES", "scopes")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_USERNAME_CLAIM", "usernameClaim")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_EMAIL_CLAIM", "emailClaim")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_GROUPS_CLAIM", "groupsClaim")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_ALLOWED_GROUPS", "allowedGroups")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_ALLOWED_DOMAINS", "allowedDomains")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_ALLOWED_EMAILS", "allowedEmails")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_GROUP_ROLE_MAPPINGS", "groupRoleMappings")
	markOIDCEnvOverride(cfg.EnvOverrides, "OIDC_CA_BUNDLE", "caBundle")

	cfg.ApplyDefaults(publicURL)
	return cfg, true
}

// Validate performs sanity checks and returns the first error encountered.
func (c *OIDCConfig) Validate() error {
	if c == nil {
		return nil
	}

	if !c.Enabled {
		return nil
	}

	if strings.TrimSpace(c.IssuerURL) == "" {
		return fmt.Errorf("oidc issuer url is required when OIDC is enabled")
	}
	if _, err := url.ParseRequestURI(c.IssuerURL); err != nil {
		return fmt.Errorf("invalid oidc issuer url: %w", err)
	}

	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("oidc client id is required when OIDC is enabled")
	}

	if strings.TrimSpace(c.RedirectURL) == "" {
		return fmt.Errorf("oidc redirect url is required when OIDC is enabled (set PUBLIC_URL environment variable or provide redirect URL manually)")
	}
	if _, err := url.ParseRequestURI(c.RedirectURL); err != nil {
		return fmt.Errorf("invalid oidc redirect url: %w", err)
	}

	if len(c.Scopes) == 0 {
		return fmt.Errorf("oidc scopes must contain at least one entry")
	}

	return nil
}

// normaliseList trims entries, removes blanks, and de-duplicates while preserving order.
func normaliseList(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, value)
	}
	return result
}

func envTrim(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func envValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	return strings.TrimSpace(value), ok
}

func truthyEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func splitOIDCEnvList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	return normaliseList(parts)
}

func parseOIDCGroupRoleMappings(value string) map[string]string {
	parts := splitOIDCEnvList(value)
	if len(parts) == 0 {
		return nil
	}
	mappings := make(map[string]string, len(parts))
	for _, part := range parts {
		group, role, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		group = strings.TrimSpace(group)
		role = strings.TrimSpace(role)
		if group == "" || role == "" {
			continue
		}
		mappings[group] = role
	}
	if len(mappings) == 0 {
		return nil
	}
	return mappings
}

func markOIDCEnvOverride(overrides map[string]bool, envName string, field string) {
	if overrides == nil {
		return
	}
	if _, ok := os.LookupEnv(envName); ok {
		overrides[field] = true
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]bool, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
