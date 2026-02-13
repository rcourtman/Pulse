package cloudcp

import (
	"strings"
	"testing"
)

func setRequiredCPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_ADMIN_KEY", "test-key")
	t.Setenv("CP_BASE_URL", "https://cloud.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	// Clear relevant env vars
	for _, key := range []string{
		"CP_ADMIN_KEY", "CP_BASE_URL", "STRIPE_WEBHOOK_SECRET",
		"CP_DATA_DIR", "CP_BIND_ADDRESS", "CP_PORT",
	} {
		t.Setenv(key, "")
	}

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
}

func TestLoadConfig_AllRequired(t *testing.T) {
	setRequiredCPEnv(t)

	// Clear optional vars to use defaults
	for _, key := range []string{
		"CP_DATA_DIR", "CP_BIND_ADDRESS", "CP_PORT",
		"CP_PULSE_IMAGE", "CP_DOCKER_NETWORK",
		"CP_TENANT_MEMORY_LIMIT", "CP_TENANT_CPU_SHARES",
		"STRIPE_API_KEY",
	} {
		t.Setenv(key, "")
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.AdminKey != "test-key" {
		t.Errorf("AdminKey = %q, want %q", cfg.AdminKey, "test-key")
	}
	if cfg.BaseURL != "https://cloud.example.com" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Port != 8443 {
		t.Errorf("Port = %d, want 8443", cfg.Port)
	}
	if cfg.DataDir != "/data" {
		t.Errorf("DataDir = %q, want /data", cfg.DataDir)
	}
	if cfg.BindAddress != "0.0.0.0" {
		t.Errorf("BindAddress = %q, want 0.0.0.0", cfg.BindAddress)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("CP_ADMIN_KEY", "key")
	t.Setenv("CP_BASE_URL", "https://test.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_x")
	t.Setenv("CP_PORT", "9000")
	t.Setenv("CP_DATA_DIR", "/custom/data")
	t.Setenv("CP_BIND_ADDRESS", "127.0.0.1")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.DataDir != "/custom/data" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	if cfg.BindAddress != "127.0.0.1" {
		t.Errorf("BindAddress = %q", cfg.BindAddress)
	}
}

func TestLoadConfig_InvalidPortParse(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_PORT", "not-a-number")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_PORT")
	}
	if !strings.Contains(err.Error(), "CP_PORT must be a valid integer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_InvalidPortRange(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_PORT", "0")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_PORT range")
	}
	if !strings.Contains(err.Error(), "CP_PORT must be between 1 and 65535") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfig_InvalidTenantLimits(t *testing.T) {
	t.Run("memory limit", func(t *testing.T) {
		setRequiredCPEnv(t)
		t.Setenv("CP_TENANT_MEMORY_LIMIT", "0")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid CP_TENANT_MEMORY_LIMIT")
		}
		if !strings.Contains(err.Error(), "CP_TENANT_MEMORY_LIMIT must be greater than 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cpu shares", func(t *testing.T) {
		setRequiredCPEnv(t)
		t.Setenv("CP_TENANT_CPU_SHARES", "-10")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid CP_TENANT_CPU_SHARES")
		}
		if !strings.Contains(err.Error(), "CP_TENANT_CPU_SHARES must be greater than 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadConfig_InvalidBaseURL(t *testing.T) {
	setRequiredCPEnv(t)
	t.Setenv("CP_BASE_URL", "cloud.example.com")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid CP_BASE_URL")
	}
	if !strings.Contains(err.Error(), "CP_BASE_URL must use http or https scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTenantsDir(t *testing.T) {
	cfg := &CPConfig{DataDir: "/data"}
	if got := cfg.TenantsDir(); got != "/data/tenants" {
		t.Errorf("TenantsDir = %q, want /data/tenants", got)
	}
}

func TestControlPlaneDir(t *testing.T) {
	cfg := &CPConfig{DataDir: "/data"}
	if got := cfg.ControlPlaneDir(); got != "/data/control-plane" {
		t.Errorf("ControlPlaneDir = %q, want /data/control-plane", got)
	}
}

func TestEnvOrDefaultInt64(t *testing.T) {
	const key = "TEST_CP_ENV_OR_DEFAULT_INT64"
	t.Setenv(key, "")
	if got := envOrDefaultInt64(key, 99); got != 99 {
		t.Fatalf("envOrDefaultInt64 unset = %d, want 99", got)
	}

	t.Setenv(key, " 1234 ")
	if got := envOrDefaultInt64(key, 99); got != 1234 {
		t.Fatalf("envOrDefaultInt64 valid = %d, want 1234", got)
	}

	t.Setenv(key, "not-an-int")
	if got := envOrDefaultInt64(key, 99); got != 99 {
		t.Fatalf("envOrDefaultInt64 invalid = %d, want 99", got)
	}
}
