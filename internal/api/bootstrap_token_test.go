package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestLoadOrCreateBootstrapToken_EmptyDataPath(t *testing.T) {
	tests := []struct {
		name     string
		dataPath string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs and spaces", "  \t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, created, fullPath, err := loadOrCreateBootstrapToken(tt.dataPath)
			if err == nil {
				t.Error("expected error for empty data path, got nil")
			}
			if token != "" {
				t.Errorf("expected empty token, got %q", token)
			}
			if created {
				t.Error("expected created=false")
			}
			if fullPath != "" {
				t.Errorf("expected empty fullPath, got %q", fullPath)
			}
			if err.Error() != "data path required for bootstrap token" {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestLoadOrCreateBootstrapToken_MkdirAllFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a file where we want a directory (MkdirAll will fail)
	blockingFile := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	// Try to use the file path as a directory path
	token, created, fullPath, err := loadOrCreateBootstrapToken(blockingFile)
	if err == nil {
		t.Error("expected error when MkdirAll fails, got nil")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if created {
		t.Error("expected created=false")
	}
	if fullPath != "" {
		t.Errorf("expected empty fullPath, got %q", fullPath)
	}
}

func TestLoadOrCreateBootstrapToken_WriteFileFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	// Create a temp directory with no write permissions
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o700); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}

	// Remove write permissions
	if err := os.Chmod(readOnlyDir, 0o500); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}
	// Restore permissions for cleanup
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o700)
	})

	token, created, fullPath, err := loadOrCreateBootstrapToken(readOnlyDir)
	if err == nil {
		t.Error("expected error when WriteFile fails, got nil")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if created {
		t.Error("expected created=false")
	}
	// fullPath should be set even on write failure (path was computed before write)
	expectedPath := filepath.Join(readOnlyDir, bootstrapTokenFilename)
	if fullPath != expectedPath {
		t.Errorf("expected fullPath=%q, got %q", expectedPath, fullPath)
	}
}

func TestLoadOrCreateBootstrapToken_EmptyFileContents(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty bootstrap token file
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create empty token file: %v", err)
	}

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err == nil {
		t.Error("expected error for empty file contents, got nil")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if created {
		t.Error("expected created=false")
	}
	if fullPath != tokenPath {
		t.Errorf("expected fullPath=%q, got %q", tokenPath, fullPath)
	}
	if err.Error() != "bootstrap token file is empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadOrCreateBootstrapToken_WhitespaceOnlyFileContents(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a bootstrap token file with only whitespace
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte("   \n\t  \n"), 0o600); err != nil {
		t.Fatalf("failed to create whitespace-only token file: %v", err)
	}

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err == nil {
		t.Error("expected error for whitespace-only file contents, got nil")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if created {
		t.Error("expected created=false")
	}
	if fullPath != tokenPath {
		t.Errorf("expected fullPath=%q, got %q", tokenPath, fullPath)
	}
	if err.Error() != "bootstrap token file is empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadOrCreateBootstrapToken_ReadFileFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	tmpDir := t.TempDir()

	// Create a token file with no read permissions
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte("sometoken"), 0o000); err != nil {
		t.Fatalf("failed to create unreadable token file: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(tokenPath, 0o600)
	})

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err == nil {
		t.Error("expected error when ReadFile fails, got nil")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
	if created {
		t.Error("expected created=false")
	}
	if fullPath != tokenPath {
		t.Errorf("expected fullPath=%q, got %q", tokenPath, fullPath)
	}
}

func TestLoadOrCreateBootstrapToken_Success_NewToken(t *testing.T) {
	tmpDir := t.TempDir()

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if !created {
		t.Error("expected created=true for new token")
	}
	expectedPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if fullPath != expectedPath {
		t.Errorf("expected fullPath=%q, got %q", expectedPath, fullPath)
	}

	// Verify the token was written to disk
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read token file: %v", err)
	}
	// Token is written with trailing newline
	if string(data) != token+"\n" {
		t.Errorf("token file contents mismatch: got %q, want %q", string(data), token+"\n")
	}
}

func TestLoadOrCreateBootstrapToken_Success_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create a token file
	existingToken := "myexistingtoken123"
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte(existingToken+"\n"), 0o600); err != nil {
		t.Fatalf("failed to create existing token file: %v", err)
	}

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != existingToken {
		t.Errorf("expected token=%q, got %q", existingToken, token)
	}
	if created {
		t.Error("expected created=false for existing token")
	}
	if fullPath != tokenPath {
		t.Errorf("expected fullPath=%q, got %q", tokenPath, fullPath)
	}
}

func TestGenerateBootstrapToken(t *testing.T) {
	// Test that tokens are generated
	token, err := generateBootstrapToken()
	if err != nil {
		t.Fatalf("generateBootstrapToken() error: %v", err)
	}
	if token == "" {
		t.Error("generateBootstrapToken() returned empty string")
	}

	// Test token length (24 bytes = 48 hex characters)
	if len(token) != 48 {
		t.Errorf("generateBootstrapToken() length = %d, want 48", len(token))
	}

	// Test that tokens are unique
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := generateBootstrapToken()
		if err != nil {
			t.Fatalf("generateBootstrapToken() error on iteration %d: %v", i, err)
		}
		if tokens[tok] {
			t.Errorf("generateBootstrapToken() generated duplicate token: %s", tok)
		}
		tokens[tok] = true
	}

	// Test that token is valid hex
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateBootstrapToken() contains non-hex character: %c", c)
		}
	}
}

func TestBootstrapTokenValid(t *testing.T) {
	t.Run("nil router returns false", func(t *testing.T) {
		var r *Router = nil
		if r.bootstrapTokenValid("anything") {
			t.Error("expected false for nil router")
		}
	})

	t.Run("empty hash returns false", func(t *testing.T) {
		r := &Router{bootstrapTokenHash: ""}
		if r.bootstrapTokenValid("anything") {
			t.Error("expected false when bootstrapTokenHash is empty")
		}
	})

	t.Run("empty token returns false", func(t *testing.T) {
		// Generate a token and create a router with its hash
		token, err := generateBootstrapToken()
		if err != nil {
			t.Fatalf("generateBootstrapToken() error: %v", err)
		}
		r := &Router{}
		// Create a token and store its hash - use loadOrCreateBootstrapToken to get a hash indirectly
		tmpDir := t.TempDir()
		loadedToken, _, _, err := loadOrCreateBootstrapToken(tmpDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken() error: %v", err)
		}
		// Since we can't directly access HashAPIToken, we need to use initializeBootstrapToken
		// Instead, let's test with a known hash by using the auth package
		_ = token
		_ = loadedToken
		// For this test, we just need any non-empty hash
		r.bootstrapTokenHash = "somehash"

		if r.bootstrapTokenValid("") {
			t.Error("expected false for empty token")
		}
		if r.bootstrapTokenValid("   ") {
			t.Error("expected false for whitespace-only token")
		}
	})

	t.Run("valid token returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		token, _, _, err := loadOrCreateBootstrapToken(tmpDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken() error: %v", err)
		}

		// Create a router that simulates having loaded the token
		// We need to use the auth package to hash the token
		cfg := &config.Config{DataPath: tmpDir}
		r := &Router{config: cfg}
		r.initializeBootstrapToken()

		if !r.bootstrapTokenValid(token) {
			t.Error("expected true for valid token")
		}
	})

	t.Run("invalid token returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, _, _, err := loadOrCreateBootstrapToken(tmpDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken() error: %v", err)
		}

		cfg := &config.Config{DataPath: tmpDir}
		r := &Router{config: cfg}
		r.initializeBootstrapToken()

		if r.bootstrapTokenValid("wrongtoken") {
			t.Error("expected false for wrong token")
		}
	})

	t.Run("token with whitespace is trimmed", func(t *testing.T) {
		tmpDir := t.TempDir()
		token, _, _, err := loadOrCreateBootstrapToken(tmpDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken() error: %v", err)
		}

		cfg := &config.Config{DataPath: tmpDir}
		r := &Router{config: cfg}
		r.initializeBootstrapToken()

		// Token with leading/trailing whitespace should still validate
		if !r.bootstrapTokenValid("  " + token + "  ") {
			t.Error("expected true for token with surrounding whitespace")
		}
	})
}

func TestInitializeBootstrapToken_LoadOrCreateFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission tests not reliable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	// Create a temp directory with a file where the data path expects a directory
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("block"), 0o600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	// DataPath points to a file, so MkdirAll will fail inside loadOrCreateBootstrapToken
	cfg := &config.Config{DataPath: blockingFile}
	r := &Router{config: cfg}

	// Should not panic, but should fail and leave bootstrapTokenHash empty
	r.initializeBootstrapToken()

	if r.bootstrapTokenHash != "" {
		t.Errorf("expected empty bootstrapTokenHash when loadOrCreateBootstrapToken fails, got %q", r.bootstrapTokenHash)
	}
	if r.bootstrapTokenPath != "" {
		t.Errorf("expected empty bootstrapTokenPath when loadOrCreateBootstrapToken fails, got %q", r.bootstrapTokenPath)
	}
}

func TestInitializeBootstrapToken_OIDCEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a bootstrap token file first
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte("testtoken\n"), 0o600); err != nil {
		t.Fatalf("failed to create token file: %v", err)
	}

	cfg := &config.Config{
		DataPath: tmpDir,
		OIDC: &config.OIDCConfig{
			Enabled: true,
		},
	}
	r := &Router{
		config:             cfg,
		bootstrapTokenPath: tokenPath, // Set path so clearBootstrapToken can delete the file
	}
	r.initializeBootstrapToken()

	// Token should be cleared when OIDC is enabled
	if r.bootstrapTokenHash != "" {
		t.Error("expected empty bootstrapTokenHash when OIDC is enabled")
	}

	// Token file should be removed
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("expected token file to be deleted when OIDC is enabled")
	}
}

func TestHandleValidateBootstrapToken_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{DataPath: tmpDir}
	r := &Router{config: cfg}
	r.initializeBootstrapToken()

	// Test invalid JSON body triggers json.Decode error
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader("not valid json"))
	rr := httptest.NewRecorder()

	r.handleValidateBootstrapToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid request payload") {
		t.Errorf("expected 'Invalid request payload' error, got %q", rr.Body.String())
	}
}

func TestClearBootstrapToken(t *testing.T) {
	t.Run("nil router does not panic", func(t *testing.T) {
		var r *Router = nil
		// Should not panic
		r.clearBootstrapToken()
	})

	t.Run("clears token hash and path", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, _, tokenPath, err := loadOrCreateBootstrapToken(tmpDir)
		if err != nil {
			t.Fatalf("loadOrCreateBootstrapToken() error: %v", err)
		}

		r := &Router{
			bootstrapTokenHash: "somehash",
			bootstrapTokenPath: tokenPath,
		}

		r.clearBootstrapToken()

		if r.bootstrapTokenHash != "" {
			t.Errorf("expected empty bootstrapTokenHash, got %q", r.bootstrapTokenHash)
		}
		if r.bootstrapTokenPath != "" {
			t.Errorf("expected empty bootstrapTokenPath, got %q", r.bootstrapTokenPath)
		}

		// Verify file was deleted
		if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
			t.Error("expected token file to be deleted")
		}
	})

	t.Run("handles missing file gracefully", func(t *testing.T) {
		r := &Router{
			bootstrapTokenHash: "somehash",
			bootstrapTokenPath: "/nonexistent/path/token",
		}

		// Should not panic, should clear the hash and path
		r.clearBootstrapToken()

		if r.bootstrapTokenHash != "" {
			t.Errorf("expected empty bootstrapTokenHash, got %q", r.bootstrapTokenHash)
		}
		if r.bootstrapTokenPath != "" {
			t.Errorf("expected empty bootstrapTokenPath, got %q", r.bootstrapTokenPath)
		}
	})

	t.Run("handles empty path", func(t *testing.T) {
		r := &Router{
			bootstrapTokenHash: "somehash",
			bootstrapTokenPath: "",
		}

		// Should not panic
		r.clearBootstrapToken()

		if r.bootstrapTokenHash != "" {
			t.Errorf("expected empty bootstrapTokenHash, got %q", r.bootstrapTokenHash)
		}
	})

	t.Run("remove failure logs warning but clears fields", func(t *testing.T) {
		// Point to a directory with contents - os.Remove will fail with ENOTEMPTY
		tmpDir := t.TempDir()
		subFile := filepath.Join(tmpDir, "subfile")
		if err := os.WriteFile(subFile, []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to create subfile: %v", err)
		}

		r := &Router{
			bootstrapTokenHash: "somehash",
			bootstrapTokenPath: tmpDir, // Point to directory, not file
		}

		// Should not panic and should clear fields even if remove fails
		r.clearBootstrapToken()

		if r.bootstrapTokenHash != "" {
			t.Errorf("expected empty bootstrapTokenHash, got %q", r.bootstrapTokenHash)
		}
		if r.bootstrapTokenPath != "" {
			t.Errorf("expected empty bootstrapTokenPath, got %q", r.bootstrapTokenPath)
		}
	})
}
