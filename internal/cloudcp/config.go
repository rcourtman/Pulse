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
	DataDir             string
	BindAddress         string
	Port                int
	AdminKey            string
	BaseURL             string
	PulseImage          string
	DockerNetwork       string
	TenantMemoryLimit   int64 // bytes
	TenantCPUShares     int64
	StripeWebhookSecret string
	StripeAPIKey        string
	ResendAPIKey        string // Resend API key (optional — if empty, emails are logged)
	EmailFrom           string // Sender email address (e.g. "noreply@pulserelay.pro")
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

	cfg := &CPConfig{
		DataDir:             envOrDefault("CP_DATA_DIR", "/data"),
		BindAddress:         envOrDefault("CP_BIND_ADDRESS", "0.0.0.0"),
		Port:                port,
		AdminKey:            strings.TrimSpace(os.Getenv("CP_ADMIN_KEY")),
		BaseURL:             strings.TrimSpace(os.Getenv("CP_BASE_URL")),
		PulseImage:          envOrDefault("CP_PULSE_IMAGE", "ghcr.io/rcourtman/pulse:latest"),
		DockerNetwork:       envOrDefault("CP_DOCKER_NETWORK", "pulse-cloud"),
		TenantMemoryLimit:   tenantMemoryLimit,
		TenantCPUShares:     tenantCPUShares,
		StripeWebhookSecret: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		StripeAPIKey:        strings.TrimSpace(os.Getenv("STRIPE_API_KEY")),
		ResendAPIKey:        strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		EmailFrom:           envOrDefault("PULSE_EMAIL_FROM", "noreply@pulserelay.pro"),
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
			return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}
