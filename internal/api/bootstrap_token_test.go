package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
		os.Chmod(readOnlyDir, 0o700)
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
		os.Chmod(tokenPath, 0o600)
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
