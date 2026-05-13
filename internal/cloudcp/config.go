package cloudcp

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
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
	TrustedProxyCIDRs                 []string
	TenantMemoryLimit                 int64 // bytes
	TenantCPUShares                   int64
	TenantLogMaxSize                  string
	TenantLogMaxFile                  int
	AllowDockerlessProvisioning       bool
	StorageGuardrailsEnabled          bool
	StorageRootPath                   string
	StorageDataPath                   string
	StorageDockerPath                 string
	StorageMinRootAvailableBytes      int64
	StorageMinDataAvailableBytes      int64
	StorageMinDockerAvailableBytes    int64
	StorageMaxDockerBuildCacheBytes   int64
	ProofTenantMaxAge                 time.Duration
	ProofTenantMatchers               []string
	StripeWebhookSecret               string
	StripeAPIKey                      string
	PublicCloudSignupEnabled          bool
	TrialSignupPriceID                string // Cloud Starter (default tier) price ID
	CloudPowerPriceID                 string // Cloud Power tier price ID (optional)
	CloudMaxPriceID                   string // Cloud Max tier price ID (optional)
	LicenseServerURL                  string
	LicenseAdminToken                 string
	TrialActivationPrivateKey         string
	TrialActivationPublicKey          string
	RequireEmailProvider              bool
	ResendAPIKey                      string // Resend API key (optional — if empty, emails are logged)
	EmailFrom                         string // Sender email address (e.g. "noreply@pulserelay.pro")
	EmailReplyTo                      string // Support reply address for transactional email
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

	dataDir := envOrDefault("CP_DATA_DIR", "/data")
	environment := normalizeCPEnvironment(envOrDefault("CP_ENV", "production"))
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
	tenantLogMaxFile, err := envOrDefaultInt("CP_TENANT_LOG_MAX_FILE", 3)
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
	storageMinRootAvailable, err := envOrDefaultBytes("CP_STORAGE_MIN_ROOT_AVAILABLE", 10*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	storageMinDataAvailable, err := envOrDefaultBytes("CP_STORAGE_MIN_DATA_AVAILABLE", 5*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	storageMinDockerAvailable, err := envOrDefaultBytes("CP_STORAGE_MIN_DOCKER_AVAILABLE", 10*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	storageMaxDockerBuildCache, err := envOrDefaultBytes("CP_STORAGE_MAX_DOCKER_BUILD_CACHE", 2*1024*1024*1024)
	if err != nil {
		return nil, err
	}
	proofTenantMaxAge, err := envOrDefaultDuration("CP_PROOF_TENANT_MAX_AGE", 24*time.Hour)
	if err != nil {
		return nil, err
	}

	cfg := &CPConfig{
		DataDir:                           dataDir,
		Environment:                       environment,
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
		TrustedProxyCIDRs:                 parseTrustedProxyCIDRValues("CP_TRUSTED_PROXY_CIDRS", "PULSE_TRUSTED_PROXY_CIDRS"),
		TenantMemoryLimit:                 tenantMemoryLimit,
		TenantCPUShares:                   tenantCPUShares,
		TenantLogMaxSize:                  envOrDefault("CP_TENANT_LOG_MAX_SIZE", "10m"),
		TenantLogMaxFile:                  tenantLogMaxFile,
		AllowDockerlessProvisioning:       envOrDefaultBool("CP_ALLOW_DOCKERLESS_PROVISIONING", false),
		StorageGuardrailsEnabled:          envOrDefaultBool("CP_STORAGE_GUARDRAILS_ENABLED", environment != "development"),
		StorageRootPath:                   envOrDefault("CP_STORAGE_ROOT_PATH", "/"),
		StorageDataPath:                   envOrDefault("CP_STORAGE_DATA_PATH", dataDir),
		StorageDockerPath:                 envOrDefault("CP_STORAGE_DOCKER_PATH", "/var/lib/docker"),
		StorageMinRootAvailableBytes:      storageMinRootAvailable,
		StorageMinDataAvailableBytes:      storageMinDataAvailable,
		StorageMinDockerAvailableBytes:    storageMinDockerAvailable,
		StorageMaxDockerBuildCacheBytes:   storageMaxDockerBuildCache,
		ProofTenantMaxAge:                 proofTenantMaxAge,
		ProofTenantMatchers:               parseCSVEnv("CP_PROOF_TENANT_MATCHERS", "proof,canary,rehearsal,msp_prod,ownerseed,owner_seed"),
		StripeWebhookSecret:               strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripeAPIKey:                      strings.TrimSpace(os.Getenv("STRIPE_API_KEY")),
		PublicCloudSignupEnabled:          envOrDefaultBool("CP_PUBLIC_CLOUD_SIGNUP_ENABLED", false),
		TrialSignupPriceID:                strings.TrimSpace(os.Getenv("CP_TRIAL_SIGNUP_PRICE_ID")),
		CloudPowerPriceID:                 strings.TrimSpace(os.Getenv("CP_CLOUD_POWER_PRICE_ID")),
		CloudMaxPriceID:                   strings.TrimSpace(os.Getenv("CP_CLOUD_MAX_PRICE_ID")),
		LicenseServerURL:                  envOrDefault("PULSE_LICENSE_SERVER_URL", "https://license.pulserelay.pro"),
		LicenseAdminToken:                 strings.TrimSpace(os.Getenv("PULSE_LICENSE_ADMIN_TOKEN")),
		TrialActivationPrivateKey:         strings.TrimSpace(os.Getenv("CP_TRIAL_ACTIVATION_PRIVATE_KEY")),
		RequireEmailProvider:              envOrDefaultBool("CP_REQUIRE_EMAIL_PROVIDER", true),
		ResendAPIKey:                      strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		EmailFrom:                         envOrDefault("PULSE_EMAIL_FROM", "noreply@pulserelay.pro"),
		EmailReplyTo:                      envOrDefault("PULSE_EMAIL_REPLY_TO", "support@pulserelay.pro"),
	}
	if strings.TrimSpace(cfg.TrialActivationPrivateKey) != "" {
		publicKey, err := deriveTrialActivationPublicKey(cfg.TrialActivationPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("derive trial activation public key: %w", err)
		}
		cfg.TrialActivationPublicKey = publicKey
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate control plane config: %w", err)
	}
	return cfg, nil
}

func deriveTrialActivationPublicKey(encodedPrivateKey string) (string, error) {
	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(encodedPrivateKey))
	if err != nil {
		return "", err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid derived trial activation public key")
	}
	return base64.StdEncoding.EncodeToString(publicKey), nil
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
	if strings.TrimSpace(c.TenantLogMaxSize) == "" {
		return fmt.Errorf("CP_TENANT_LOG_MAX_SIZE must not be empty")
	}
	if c.TenantLogMaxFile <= 0 {
		return fmt.Errorf("CP_TENANT_LOG_MAX_FILE must be greater than 0")
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
	if c.StorageGuardrailsEnabled {
		if strings.TrimSpace(c.StorageRootPath) == "" {
			return fmt.Errorf("CP_STORAGE_ROOT_PATH is required when CP_STORAGE_GUARDRAILS_ENABLED=true")
		}
		if strings.TrimSpace(c.StorageDataPath) == "" {
			return fmt.Errorf("CP_STORAGE_DATA_PATH is required when CP_STORAGE_GUARDRAILS_ENABLED=true")
		}
		if strings.TrimSpace(c.StorageDockerPath) == "" {
			return fmt.Errorf("CP_STORAGE_DOCKER_PATH is required when CP_STORAGE_GUARDRAILS_ENABLED=true")
		}
		if c.StorageMinRootAvailableBytes <= 0 {
			return fmt.Errorf("CP_STORAGE_MIN_ROOT_AVAILABLE must be greater than 0")
		}
		if c.StorageMinDataAvailableBytes <= 0 {
			return fmt.Errorf("CP_STORAGE_MIN_DATA_AVAILABLE must be greater than 0")
		}
		if c.StorageMinDockerAvailableBytes <= 0 {
			return fmt.Errorf("CP_STORAGE_MIN_DOCKER_AVAILABLE must be greater than 0")
		}
		if c.StorageMaxDockerBuildCacheBytes <= 0 {
			return fmt.Errorf("CP_STORAGE_MAX_DOCKER_BUILD_CACHE must be greater than 0")
		}
	}
	if c.ProofTenantMaxAge <= 0 {
		return fmt.Errorf("CP_PROOF_TENANT_MAX_AGE must be greater than 0")
	}
	if c.RequireEmailProvider {
		if strings.TrimSpace(c.ResendAPIKey) == "" {
			return fmt.Errorf("RESEND_API_KEY is required when CP_REQUIRE_EMAIL_PROVIDER=true")
		}
		if strings.TrimSpace(c.EmailFrom) == "" {
			return fmt.Errorf("PULSE_EMAIL_FROM is required when CP_REQUIRE_EMAIL_PROVIDER=true")
		}
		if strings.TrimSpace(c.EmailReplyTo) == "" {
			return fmt.Errorf("PULSE_EMAIL_REPLY_TO is required when CP_REQUIRE_EMAIL_PROVIDER=true")
		}
	}
	if strings.TrimSpace(c.StripeAPIKey) != "" && c.PublicCloudSignupEnabled && strings.TrimSpace(c.TrialSignupPriceID) == "" {
		return fmt.Errorf("CP_TRIAL_SIGNUP_PRICE_ID is required when STRIPE_API_KEY is configured")
	}
	if strings.TrimSpace(c.StripeAPIKey) != "" && strings.TrimSpace(c.TrialActivationPrivateKey) == "" {
		return fmt.Errorf("CP_TRIAL_ACTIVATION_PRIVATE_KEY is required when STRIPE_API_KEY is configured")
	}
	if err := validateCloudStripePriceID(c.Environment, c.StripeAPIKey, "CP_TRIAL_SIGNUP_PRICE_ID", c.TrialSignupPriceID, "cloud_starter"); err != nil {
		return err
	}
	if err := validateCloudStripePriceID(c.Environment, c.StripeAPIKey, "CP_CLOUD_POWER_PRICE_ID", c.CloudPowerPriceID, "cloud_power"); err != nil {
		return err
	}
	if err := validateCloudStripePriceID(c.Environment, c.StripeAPIKey, "CP_CLOUD_MAX_PRICE_ID", c.CloudMaxPriceID, "cloud_max"); err != nil {
		return err
	}
	if strings.TrimSpace(c.LicenseServerURL) == "" && strings.TrimSpace(c.LicenseAdminToken) != "" {
		return fmt.Errorf("PULSE_LICENSE_SERVER_URL is required when PULSE_LICENSE_ADMIN_TOKEN is configured")
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

func validateCloudStripePriceID(environment, stripeAPIKey, envName, priceID, wantPlanVersion string) error {
	trimmed := strings.TrimSpace(priceID)
	if trimmed == "" {
		return nil
	}

	if environment != "production" && stripeSecretKeyMode(stripeAPIKey) == "test" {
		if strings.HasPrefix(trimmed, "price_") {
			return nil
		}
		return fmt.Errorf("%s must be a Stripe price id (price_...) in %s test mode, got %q", envName, environment, trimmed)
	}

	planVersion, ok := pkglicensing.PlanVersionForPriceID(trimmed)
	if !ok {
		return fmt.Errorf("%s must map to the canonical %s Stripe price, got unknown price id %q", envName, wantPlanVersion, trimmed)
	}
	if planVersion != wantPlanVersion {
		return fmt.Errorf("%s must map to the canonical %s Stripe price, got %q (%s)", envName, wantPlanVersion, trimmed, planVersion)
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

func envOrDefaultBytes(key string, fallback int64) (int64, error) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n, err := parseByteSize(v)
		if err != nil {
			return fallback, fmt.Errorf("%s must be a valid byte size: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}

func envOrDefaultDuration(key string, fallback time.Duration) (time.Duration, error) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		duration, err := time.ParseDuration(v)
		if err != nil {
			return fallback, fmt.Errorf("%s must be a valid duration: %w", key, err)
		}
		return duration, nil
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

func parseByteSize(raw string) (int64, error) {
	value := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	if value == "" {
		return 0, fmt.Errorf("empty value")
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return n, nil
	}

	i := 0
	for i < len(value) {
		ch := value[i]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			i++
			continue
		}
		break
	}
	if i == 0 || i == len(value) {
		return 0, fmt.Errorf("expected number followed by unit")
	}
	number, err := strconv.ParseFloat(value[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("parse number: %w", err)
	}
	if number < 0 {
		return 0, fmt.Errorf("must not be negative")
	}
	multiplier := int64(0)
	switch value[i:] {
	case "B":
		multiplier = 1
	case "K", "KB", "KIB":
		multiplier = 1024
	case "M", "MB", "MIB":
		multiplier = 1024 * 1024
	case "G", "GB", "GIB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB", "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit %q", value[i:])
	}
	return int64(number * float64(multiplier)), nil
}

func parseCSVEnv(key, fallback string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		raw = fallback
	}
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		values = append(values, entry)
	}
	return values
}

func parseTrustedProxyCIDRValues(keys ...string) []string {
	values := make([]string, 0)
	seen := make(map[string]struct{})
	for _, key := range keys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		for _, entry := range strings.Split(raw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, ok := seen[entry]; ok {
				continue
			}
			seen[entry] = struct{}{}
			values = append(values, entry)
		}
	}
	return values
}
