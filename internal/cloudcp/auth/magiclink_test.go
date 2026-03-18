package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateToken_Format(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	token, err := svc.GenerateToken("alice@example.com", "t-abc123")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if !strings.HasPrefix(token, "ml1_") {
		t.Errorf("token should have ml1_ prefix, got %q", token)
	}
	if len(token) < 10 {
		t.Errorf("token too short: %q", token)
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	t1, _ := svc.GenerateToken("alice@example.com", "t-abc123")
	t2, _ := svc.GenerateToken("alice@example.com", "t-abc123")
	if t1 == t2 {
		t.Error("two tokens should be unique")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	token, err := svc.GenerateToken("bob@example.com", "t-xyz")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	result, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if result.Email != "bob@example.com" {
		t.Errorf("email = %q, want bob@example.com", result.Email)
	}
	if result.TenantID != "t-xyz" {
		t.Errorf("tenantID = %q, want t-xyz", result.TenantID)
	}
}

func TestValidateToken_AlreadyUsed(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	token, _ := svc.GenerateToken("carol@example.com", "t-111")

	// First use succeeds.
	_, err = svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("first ValidateToken: %v", err)
	}

	// Second use fails.
	_, err = svc.ValidateToken(token)
	if err != ErrTokenUsed {
		t.Fatalf("expected ErrTokenUsed, got %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	// Override TTL to make tokens expire immediately.
	svc.ttl = -1 * time.Second

	token, _ := svc.GenerateToken("dave@example.com", "t-222")

	_, err = svc.ValidateToken(token)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	defer svc.Close()

	_, err = svc.ValidateToken("ml1_totally-bogus-token")
	if err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestBuildVerifyURL(t *testing.T) {
	tests := []struct {
		baseURL string
		token   string
		want    string
	}{
		{
			baseURL: "https://cloud.pulserelay.pro",
			token:   "ml1_abc123",
			want:    "https://cloud.pulserelay.pro/auth/magic-link/verify?token=ml1_abc123",
		},
		{
			baseURL: "https://cloud.pulserelay.pro/",
			token:   "ml1_def456",
			want:    "https://cloud.pulserelay.pro/auth/magic-link/verify?token=ml1_def456",
		},
		{
			baseURL: "",
			token:   "ml1_test",
			want:    "",
		},
		{
			baseURL: "https://cloud.pulserelay.pro",
			token:   "",
			want:    "",
		},
	}
	for _, tt := range tests {
		got := BuildVerifyURL(tt.baseURL, tt.token)
		if got != tt.want {
			t.Errorf("BuildVerifyURL(%q, %q) = %q, want %q", tt.baseURL, tt.token, got, tt.want)
		}
	}
}

func TestKeyPersistence(t *testing.T) {
	dir := t.TempDir()

	svc1, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService (first): %v", err)
	}

	// Verify key file was created.
	keyPath := filepath.Join(dir, hmacKeyFile)
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("key file not created: %v", err)
	}

	// Generate a token with the first service.
	token, _ := svc1.GenerateToken("eve@example.com", "t-333")
	svc1.Close()

	// Open a second service — it should load the same key and be able to validate the token.
	svc2, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService (second): %v", err)
	}
	defer svc2.Close()

	result, err := svc2.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken across services: %v", err)
	}
	if result.Email != "eve@example.com" {
		t.Errorf("email = %q, want eve@example.com", result.Email)
	}
}

func TestNewService_SecuresDataDirPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cp-data")

	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if got := info.Mode().Perm(); got != privateDirPerm {
		t.Fatalf("data dir perms = %o, want %o", got, privateDirPerm)
	}
}

func TestNewService_HardensExistingPermissiveDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cp-data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}

	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.Close()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if got := info.Mode().Perm(); got != privateDirPerm {
		t.Fatalf("data dir perms = %o, want %o", got, privateDirPerm)
	}
}
