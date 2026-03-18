package cloudcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseDomainFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "https-domain", in: "https://cloud.pulserelay.pro", want: "cloud.pulserelay.pro"},
		{name: "http-with-port-and-path", in: "http://cloud.pulserelay.pro:8443/api/v1", want: "cloud.pulserelay.pro"},
		{name: "no-scheme-with-path", in: "cloud.pulserelay.pro/path", want: "cloud.pulserelay.pro"},
		{name: "bare-domain", in: "cloud.pulserelay.pro", want: "cloud.pulserelay.pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseDomainFromURL(tt.in); got != tt.want {
				t.Fatalf("baseDomainFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRun_LoadConfigError(t *testing.T) {
	t.Setenv("CP_ADMIN_KEY", "")
	t.Setenv("CP_BASE_URL", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")

	err := Run(context.Background(), "test-version")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "load config:") {
		t.Fatalf("Run() error = %q, want load config prefix", err)
	}
}

func TestRun_CreateTenantsDirError(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", filePath, err)
	}

	t.Setenv("CP_DATA_DIR", filePath)
	t.Setenv("CP_ADMIN_KEY", "test-admin-key")
	t.Setenv("CP_BASE_URL", "https://cloud.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")

	err := Run(context.Background(), "test-version")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "create tenants dir:") {
		t.Fatalf("Run() error = %q, want create tenants dir error", err)
	}
}

func TestRun_CreateControlPlaneDirError(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "control-plane")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", filePath, err)
	}

	t.Setenv("CP_DATA_DIR", tempDir)
	t.Setenv("CP_ADMIN_KEY", "test-admin-key")
	t.Setenv("CP_BASE_URL", "https://cloud.example.com")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test")
	t.Setenv("CP_ENV", "development")
	t.Setenv("CP_REQUIRE_EMAIL_PROVIDER", "false")

	err := Run(context.Background(), "test-version")
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "create control-plane dir:") {
		t.Fatalf("Run() error = %q, want create control-plane dir error", err)
	}
}
