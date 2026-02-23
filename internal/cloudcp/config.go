package cloudcp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// CPConfig holds all configuration for the control plane.
type CPConfig struct {
	DataDir                           string
	Environment                       string
	BindAddress                       string
	Port                              int
	AdminKey                          string
	BaseURL                           string
	PublicStatus                      bool
	PublicMetrics                     bool
	WebhookRateLimitPerMinute         int
	MagicLinkVerifyRateLimitPerMinute int
	SessionAuthRateLimitPerMinute     int
	AdminRateLimitPerMinute           int
	AccountAPIRateLimitPerMinute      int
	PortalAPIRateLimitPerMinute       int
	PulseImage                        string
	DockerNetwork                     string
	TenantMemoryLimit                 int64 // bytes
	TenantCPUShares                   int64
	AllowDockerlessProvisioning       bool
	StripeWebhookSecret               string
	StripeAPIKey                      string
	TrialSignupPriceID                string
	TrialActivationPrivateKey         string
	RequireEmailProvider              bool
	ResendAPIKey                      string // Resend API key (optional — if empty, emails are logged)
	EmailFrom                         string // Sender email address (e.g. "noreply@pulserelay.pro")
}

// TenantsDir returns the directory where per-tenant data is stored.
func (c *CPConfig) TenantsDir() string {
	return filepath.Join(c.DataDir, "tenants")
}

// ControlPlaneDir returns the directory for control plane's own data (registry DB, etc).
func (c *CPConfig) ControlPlaneDir() string {
	return filepath.Join(c.DataDir, "control-plane")
}

// LoadConfig loads control plane configuration from environment variables.
// A .env file is loaded if present but not required.
func LoadConfig() (*CPConfig, error) {
	// Best-effort .env loading (not required)
	_ = godotenv.Load()

	port, err := envOrDefaultInt("CP_PORT", 8443)
	if err != nil {
		return nil, err
	}
	tenantMemoryLimit, err := envOrDefaultInt64("CP_TENANT_MEMORY_LIMIT", 512*1024*1024) // 512 MiB
	if err != nil {
		return nil, err
	}
	tenantCPUShares, err := envOrDefaultInt64("CP_TENANT_CPU_SHARES", 256)
	if err != nil {
		return nil, err
	}
	webhookRPS, err := envOrDefaultInt("CP_RL_WEBHOOK_PER_MINUTE", 120)
	if err != nil {
		return nil, err
	}
	magicVerifyRPS, err := envOrDefaultInt("CP_RL_MAGIC_VERIFY_PER_MINUTE", 30)
	if err != nil {
		return nil, err
	}
	sessionAuthRPS, err := envOrDefaultInt("CP_RL_SESSION_PER_MINUTE", 60)
	if err != nil {
		return nil, err
	}
	adminRPS, err := envOrDefaultInt("CP_RL_ADMIN_PER_MINUTE", 120)
	if err != nil {
		return nil, err
	}
	accountRPS, err := envOrDefaultInt("CP_RL_ACCOUNT_PER_MINUTE", 300)
	if err != nil {
		return nil, err
	}
	portalRPS, err := envOrDefaultInt("CP_RL_PORTAL_PER_MINUTE", 300)
	if err != nil {
		return nil, err
	}

	cfg := &CPConfig{
		DataDir:                           envOrDefault("CP_DATA_DIR", "/data"),
		Environment:                       normalizeCPEnvironment(envOrDefault("CP_ENV", "production")),
		BindAddress:                       envOrDefault("CP_BIND_ADDRESS", "0.0.0.0"),
		Port:                              port,
		AdminKey:                          strings.TrimSpace(os.Getenv("CP_ADMIN_KEY")),
		BaseURL:                           strings.TrimSpace(os.Getenv("CP_BASE_URL")),
		PublicStatus:                      envOrDefaultBool("CP_PUBLIC_STATUS", false),
		PublicMetrics:                     envOrDefaultBool("CP_PUBLIC_METRICS", false),
		WebhookRateLimitPerMinute:         webhookRPS,
		MagicLinkVerifyRateLimitPerMinute: magicVerifyRPS,
		SessionAuthRateLimitPerMinute:     sessionAuthRPS,
		AdminRateLimitPerMinute:           adminRPS,
		AccountAPIRateLimitPerMinute:      accountRPS,
		PortalAPIRateLimitPerMinute:       portalRPS,
		PulseImage:                        envOrDefault("CP_PULSE_IMAGE", "ghcr.io/rcourtman/pulse:latest"),
		DockerNetwork:                     envOrDefault("CP_DOCKER_NETWORK", "pulse-cloud"),
		TenantMemoryLimit:                 tenantMemoryLimit,
		TenantCPUShares:                   tenantCPUShares,
		AllowDockerlessProvisioning:       envOrDefaultBool("CP_ALLOW_DOCKERLESS_PROVISIONING", false),
		StripeWebhookSecret:               strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripeAPIKey:                      strings.TrimSpace(os.Getenv("STRIPE_API_KEY")),
		TrialSignupPriceID:                strings.TrimSpace(os.Getenv("CP_TRIAL_SIGNUP_PRICE_ID")),
		TrialActivationPrivateKey:         strings.TrimSpace(os.Getenv("CP_TRIAL_ACTIVATION_PRIVATE_KEY")),
		RequireEmailProvider:              envOrDefaultBool("CP_REQUIRE_EMAIL_PROVIDER", true),
		ResendAPIKey:                      strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		EmailFrom:                         envOrDefault("PULSE_EMAIL_FROM", "noreply@pulserelay.pro"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate control plane config: %w", err)
	}
	return cfg, nil
}

func (c *CPConfig) validate() error {
	var missing []string
	if c.AdminKey == "" {
		missing = append(missing, "CP_ADMIN_KEY")
	}
	if c.BaseURL == "" {
		missing = append(missing, "CP_BASE_URL")
	}
	if c.StripeWebhookSecret == "" {
		missing = append(missing, "STRIPE_WEBHOOK_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("CP_PORT must be between 1 and 65535, got %d", c.Port)
	}
	if c.TenantMemoryLimit <= 0 {
		return fmt.Errorf("CP_TENANT_MEMORY_LIMIT must be greater than 0, got %d", c.TenantMemoryLimit)
	}
	if c.TenantCPUShares <= 0 {
		return fmt.Errorf("CP_TENANT_CPU_SHARES must be greater than 0, got %d", c.TenantCPUShares)
	}
	if c.WebhookRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_WEBHOOK_PER_MINUTE must be greater than 0")
	}
	if c.MagicLinkVerifyRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_MAGIC_VERIFY_PER_MINUTE must be greater than 0")
	}
	if c.SessionAuthRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_SESSION_PER_MINUTE must be greater than 0")
	}
	if c.AdminRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_ADMIN_PER_MINUTE must be greater than 0")
	}
	if c.AccountAPIRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_ACCOUNT_PER_MINUTE must be greater than 0")
	}
	if c.PortalAPIRateLimitPerMinute <= 0 {
		return fmt.Errorf("CP_RL_PORTAL_PER_MINUTE must be greater than 0")
	}
	if c.Environment != "development" && c.Environment != "staging" && c.Environment != "production" {
		return fmt.Errorf("CP_ENV must be one of development, staging, production (got %q)", c.Environment)
	}
	if c.Environment == "production" && c.AllowDockerlessProvisioning {
		return fmt.Errorf("CP_ALLOW_DOCKERLESS_PROVISIONING must be false in production")
	}
	if c.RequireEmailProvider {
		if strings.TrimSpace(c.ResendAPIKey) == "" {
			return fmt.Errorf("RESEND_API_KEY is required when CP_REQUIRE_EMAIL_PROVIDER=true")
		}
		if strings.TrimSpace(c.EmailFrom) == "" {
			return fmt.Errorf("PULSE_EMAIL_FROM is required when CP_REQUIRE_EMAIL_PROVIDER=true")
		}
	}
	if strings.TrimSpace(c.StripeAPIKey) != "" && strings.TrimSpace(c.TrialSignupPriceID) == "" {
		return fmt.Errorf("CP_TRIAL_SIGNUP_PRICE_ID is required when STRIPE_API_KEY is configured")
	}
	if strings.TrimSpace(c.StripeAPIKey) != "" {
		stripeMode := stripeSecretKeyMode(c.StripeAPIKey)
		switch c.Environment {
		case "production":
			if stripeMode != "live" {
				return fmt.Errorf("STRIPE_API_KEY must be a live key (sk_live_...) when CP_ENV=production")
			}
		case "staging":
			if stripeMode != "test" {
				return fmt.Errorf("STRIPE_API_KEY must be a test key (sk_test_...) when CP_ENV=staging")
			}
		}
	}

	parsedBaseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("CP_BASE_URL must be a valid URL: %w", err)
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return fmt.Errorf("CP_BASE_URL must use http or https scheme")
	}
	if parsedBaseURL.Host == "" {
		return fmt.Errorf("CP_BASE_URL must include a host")
	}
	return nil
}

func normalizeCPEnvironment(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dev":
		return "development"
	case "prod":
		return "production"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func stripeSecretKeyMode(raw string) string {
	key := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(key, "sk_live_"):
		return "live"
	case strings.HasPrefix(key, "sk_test_"):
		return "test"
	default:
		return "unknown"
	}
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) (int, error) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}

func envOrDefaultInt64(key string, fallback int64) (int64, error) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fallback, fmt.Errorf("%s must be a valid integer: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}

func envOrDefaultBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
