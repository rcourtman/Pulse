package config

import (
	"testing"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_EnvOverrides_Detailed(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	envVars := map[string]string{
		"OIDC_ENABLED":         "true",
		"OIDC_ISSUER_URL":      "https://oidc.com",
		"OIDC_CLIENT_ID":       "cid",
		"OIDC_CLIENT_SECRET":   "sec",
		"OIDC_REDIRECT_URL":    "https://pulse.com/callback",
		"OIDC_LOGOUT_URL":      "https://oidc.com/logout",
		"OIDC_SCOPES":          "openid profile email",
		"OIDC_USERNAME_CLAIM":  "preferred_username",
		"OIDC_EMAIL_CLAIM":     "mail",
		"OIDC_GROUPS_CLAIM":    "roles",
		"OIDC_ALLOWED_GROUPS":  "admin,dev",
		"OIDC_ALLOWED_DOMAINS": "example.com",
		"OIDC_ALLOWED_EMAILS":  "user@example.com",
		"OIDC_CA_BUNDLE":       "/path/to/ca",

		"PULSE_AUTH_PASS": "plainpass",
		"TLS_CERT_FILE":   "/etc/cert.pem",
		"TLS_KEY_FILE":    "/etc/key.pem",
		"PULSE_AGENT_URL": "http://agent:9090",

		"DISCOVERY_ENABLED":              "true",
		"DISCOVERY_SUBNET":               "192.168.1.0/24",
		"DISCOVERY_ENVIRONMENT_OVERRIDE": "docker_host",
		"DISCOVERY_SUBNET_ALLOWLIST":     "10.0.0.0/8,192.168.0.0/16",
		"DISCOVERY_SUBNET_BLOCKLIST":     "10.1.0.0/16",
		"DISCOVERY_MAX_HOSTS_PER_SCAN":   "50",
		"DISCOVERY_MAX_CONCURRENT":       "5",
		"DISCOVERY_ENABLE_REVERSE_DNS":   "false",
		"DISCOVERY_SCAN_GATEWAYS":        "false",
		"DISCOVERY_DIAL_TIMEOUT_MS":      "2000",
		"ALLOWED_ORIGINS":                "https://allowed.com",
		"PULSE_PUBLIC_URL":               "https://public.pulse.com",
		"PULSE_PRO_TRIAL_SIGNUP_URL":     "https://billing.example.com/start-pro-trial?source=test",
		"NODE_ENV":                       "production", // Ensure valid origins not defaulted to localhost
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	cfg, err := Load()
	require.NoError(t, err)

	// OIDC
	assert.True(t, cfg.OIDC.Enabled)
	assert.Equal(t, "https://oidc.com", cfg.OIDC.IssuerURL)
	assert.Equal(t, "cid", cfg.OIDC.ClientID)
	assert.Equal(t, "sec", cfg.OIDC.ClientSecret)
	assert.Equal(t, "https://pulse.com/callback", cfg.OIDC.RedirectURL)
	assert.Equal(t, "https://oidc.com/logout", cfg.OIDC.LogoutURL)
	assert.Equal(t, []string{"openid", "profile", "email"}, cfg.OIDC.Scopes)
	assert.Equal(t, "preferred_username", cfg.OIDC.UsernameClaim)
	assert.Equal(t, "mail", cfg.OIDC.EmailClaim)
	assert.Equal(t, "roles", cfg.OIDC.GroupsClaim)
	assert.Equal(t, []string{"admin", "dev"}, cfg.OIDC.AllowedGroups)
	assert.Equal(t, []string{"example.com"}, cfg.OIDC.AllowedDomains)
	assert.Equal(t, []string{"user@example.com"}, cfg.OIDC.AllowedEmails)
	assert.Equal(t, "/path/to/ca", cfg.OIDC.CABundle)

	// Auth
	assert.NotEqual(t, "plainpass", cfg.AuthPass)
	assert.True(t, IsPasswordHashed(cfg.AuthPass))

	// TLS
	assert.Equal(t, "/etc/cert.pem", cfg.TLSCertFile)
	assert.Equal(t, "/etc/key.pem", cfg.TLSKeyFile)

	// Agent
	assert.Equal(t, "http://agent:9090", cfg.AgentConnectURL)
	assert.True(t, cfg.EnvOverrides["PULSE_AGENT_CONNECT_URL"])

	// Discovery
	assert.True(t, cfg.DiscoveryEnabled)
	assert.Equal(t, "192.168.1.0/24", cfg.DiscoverySubnet)
	assert.Equal(t, "docker_host", cfg.Discovery.EnvironmentOverride)
	assert.Len(t, cfg.Discovery.SubnetAllowlist, 2)
	assert.Len(t, cfg.Discovery.SubnetBlocklist, 1)
	assert.Equal(t, 50, cfg.Discovery.MaxHostsPerScan)
	assert.Equal(t, 5, cfg.Discovery.MaxConcurrent)
	assert.False(t, cfg.Discovery.EnableReverseDNS)
	assert.False(t, cfg.Discovery.ScanGateways)
	assert.Equal(t, 2000, cfg.Discovery.DialTimeout)

	// Misc
	assert.Equal(t, "https://allowed.com", cfg.AllowedOrigins)
	assert.Equal(t, "https://public.pulse.com", cfg.PublicURL)
	assert.Equal(t, "https://billing.example.com/start-pro-trial?source=test", cfg.ProTrialSignupURL)
	assert.True(t, cfg.EnvOverrides["PULSE_PRO_TRIAL_SIGNUP_URL"])
}

func TestLoad_EnvOverrides_Invalid(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	t.Setenv("DISCOVERY_MAX_HOSTS_PER_SCAN", "invalid")
	t.Setenv("DISCOVERY_ENVIRONMENT_OVERRIDE", "invalid_env")

	cfg, err := Load()
	require.NoError(t, err)

	// Defaults should remain (or not set)
	assert.NotNil(t, cfg.Discovery)
	assert.NotEqual(t, "invalid_env", cfg.Discovery.EnvironmentOverride)
}

func TestLoad_EnvOverrides_InvalidProTrialSignupURL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)
	t.Setenv("PULSE_PRO_TRIAL_SIGNUP_URL", "javascript:alert(1)")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, pkglicensing.DefaultProTrialSignupURL, cfg.ProTrialSignupURL)
	assert.False(t, cfg.EnvOverrides["PULSE_PRO_TRIAL_SIGNUP_URL"])
}
