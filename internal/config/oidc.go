package config

import (
	"fmt"
	"net/url"
	"strings"
)

// defaultOIDCScopes defines the scopes we request when none are provided.
var defaultOIDCScopes = []string{"openid", "profile", "email"}

// DefaultOIDCCallbackPath is the path we expose for the OIDC redirect handler.
const DefaultOIDCCallbackPath = "/api/oidc/callback"

// OIDCConfig captures configuration required to integrate with an OpenID Connect provider.
type OIDCConfig struct {
	Enabled        bool            `json:"enabled"`
	IssuerURL      string          `json:"issuerUrl"`
	ClientID       string          `json:"clientId"`
	ClientSecret   string          `json:"clientSecret,omitempty"`
	RedirectURL    string          `json:"redirectUrl"`
	Scopes         []string        `json:"scopes,omitempty"`
	UsernameClaim  string          `json:"usernameClaim,omitempty"`
	EmailClaim     string          `json:"emailClaim,omitempty"`
	GroupsClaim    string          `json:"groupsClaim,omitempty"`
	AllowedGroups  []string        `json:"allowedGroups,omitempty"`
	AllowedDomains []string        `json:"allowedDomains,omitempty"`
	AllowedEmails  []string        `json:"allowedEmails,omitempty"`
	EnvOverrides   map[string]bool `json:"-"`
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
		return fmt.Errorf("oidc redirect url is required when OIDC is enabled")
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

// parseDelimited converts a delimiter-separated string into a clean slice.
func parseDelimited(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	// Accept either comma or whitespace separation; replace commas with spaces then split.
	normalised := strings.ReplaceAll(input, ",", " ")
	parts := strings.Fields(normalised)
	return normaliseList(parts)
}

// MergeFromEnv overrides config values with environment provided pairs.
func (c *OIDCConfig) MergeFromEnv(env map[string]string) {
	if c == nil {
		return
	}

	if c.EnvOverrides == nil {
		c.EnvOverrides = make(map[string]bool)
	}

	if val, ok := env["OIDC_ENABLED"]; ok {
		c.Enabled = val == "true" || val == "1"
		c.EnvOverrides["enabled"] = true
	}
	if val, ok := env["OIDC_ISSUER_URL"]; ok {
		c.IssuerURL = val
		c.EnvOverrides["issuerUrl"] = true
	}
	if val, ok := env["OIDC_CLIENT_ID"]; ok {
		c.ClientID = val
		c.EnvOverrides["clientId"] = true
	}
	if val, ok := env["OIDC_CLIENT_SECRET"]; ok {
		c.ClientSecret = val
		c.EnvOverrides["clientSecret"] = true
	}
	if val, ok := env["OIDC_REDIRECT_URL"]; ok {
		c.RedirectURL = val
		c.EnvOverrides["redirectUrl"] = true
	}
	if val, ok := env["OIDC_SCOPES"]; ok {
		c.Scopes = parseDelimited(val)
		c.EnvOverrides["scopes"] = true
	}
	if val, ok := env["OIDC_USERNAME_CLAIM"]; ok {
		c.UsernameClaim = val
		c.EnvOverrides["usernameClaim"] = true
	}
	if val, ok := env["OIDC_EMAIL_CLAIM"]; ok {
		c.EmailClaim = val
		c.EnvOverrides["emailClaim"] = true
	}
	if val, ok := env["OIDC_GROUPS_CLAIM"]; ok {
		c.GroupsClaim = val
		c.EnvOverrides["groupsClaim"] = true
	}
	if val, ok := env["OIDC_ALLOWED_GROUPS"]; ok {
		c.AllowedGroups = parseDelimited(val)
		c.EnvOverrides["allowedGroups"] = true
	}
	if val, ok := env["OIDC_ALLOWED_DOMAINS"]; ok {
		c.AllowedDomains = parseDelimited(val)
		c.EnvOverrides["allowedDomains"] = true
	}
	if val, ok := env["OIDC_ALLOWED_EMAILS"]; ok {
		c.AllowedEmails = parseDelimited(val)
		c.EnvOverrides["allowedEmails"] = true
	}
}
