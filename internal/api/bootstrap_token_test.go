package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateBootstrapToken(t *testing.T) {
	token, err := generateBootstrapToken()
	if err != nil {
		t.Fatalf("generateBootstrapToken() error = %v", err)
	}

	// Token should be 48 hex characters (24 bytes * 2)
	if len(token) != 48 {
		t.Errorf("generateBootstrapToken() token length = %d, want 48", len(token))
	}

	// Token should only contain hex characters
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateBootstrapToken() contains non-hex character: %c", c)
		}
	}
}

func TestGenerateBootstrapToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)

	// Generate multiple tokens and verify they're all unique
	for i := 0; i < 100; i++ {
		token, err := generateBootstrapToken()
		if err != nil {
			t.Fatalf("generateBootstrapToken() error on iteration %d: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("generateBootstrapToken() produced duplicate token on iteration %d", i)
		}
		tokens[token] = true
	}
}

func TestLoadOrCreateBootstrapToken_EmptyPath(t *testing.T) {
	token, created, fullPath, err := loadOrCreateBootstrapToken("")
	if err == nil {
		t.Error("loadOrCreateBootstrapToken(\"\") expected error for empty path")
	}
	if token != "" {
		t.Errorf("token = %q, want empty", token)
	}
	if created {
		t.Error("created should be false for error case")
	}
	if fullPath != "" {
		t.Errorf("fullPath = %q, want empty", fullPath)
	}
}

func TestLoadOrCreateBootstrapToken_WhitespacePath(t *testing.T) {
	_, _, _, err := loadOrCreateBootstrapToken("   ")
	if err == nil {
		t.Error("loadOrCreateBootstrapToken(\"   \") expected error for whitespace path")
	}
}

func TestLoadOrCreateBootstrapToken_NewToken(t *testing.T) {
	tmpDir := t.TempDir()

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}

	if !created {
		t.Error("created should be true for new token")
	}

	if len(token) != 48 {
		t.Errorf("token length = %d, want 48", len(token))
	}

	expectedPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if fullPath != expectedPath {
		t.Errorf("fullPath = %q, want %q", fullPath, expectedPath)
	}

	// Verify file was created with correct content
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read token file: %v", err)
	}

	fileContent := strings.TrimSpace(string(data))
	if fileContent != token {
		t.Errorf("file content = %q, want %q", fileContent, token)
	}
}

func TestLoadOrCreateBootstrapToken_ExistingToken(t *testing.T) {
	tmpDir := t.TempDir()
	existingToken := "abcdef123456789012345678901234567890123456789012"

	// Create existing token file
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte(existingToken+"\n"), 0o600); err != nil {
		t.Fatalf("Failed to create existing token file: %v", err)
	}

	token, created, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}

	if created {
		t.Error("created should be false for existing token")
	}

	if token != existingToken {
		t.Errorf("token = %q, want %q", token, existingToken)
	}

	if fullPath != tokenPath {
		t.Errorf("fullPath = %q, want %q", fullPath, tokenPath)
	}
}

func TestLoadOrCreateBootstrapToken_EmptyTokenFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty token file
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte(""), 0o600); err != nil {
		t.Fatalf("Failed to create empty token file: %v", err)
	}

	_, _, _, err := loadOrCreateBootstrapToken(tmpDir)
	if err == nil {
		t.Error("loadOrCreateBootstrapToken() expected error for empty token file")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error message should mention empty, got: %v", err)
	}
}

func TestLoadOrCreateBootstrapToken_WhitespaceOnlyTokenFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create token file with only whitespace
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)
	if err := os.WriteFile(tokenPath, []byte("   \n\t  "), 0o600); err != nil {
		t.Fatalf("Failed to create whitespace token file: %v", err)
	}

	_, _, _, err := loadOrCreateBootstrapToken(tmpDir)
	if err == nil {
		t.Error("loadOrCreateBootstrapToken() expected error for whitespace-only token file")
	}
}

func TestLoadOrCreateBootstrapToken_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "path")

	token, created, fullPath, err := loadOrCreateBootstrapToken(nestedDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}

	if !created {
		t.Error("created should be true for new token")
	}

	if len(token) != 48 {
		t.Errorf("token length = %d, want 48", len(token))
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("nested directory should have been created")
	}

	// Verify token file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		t.Error("token file should exist")
	}
}

func TestLoadOrCreateBootstrapToken_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, fullPath, err := loadOrCreateBootstrapToken(tmpDir)
	if err != nil {
		t.Fatalf("loadOrCreateBootstrapToken() error = %v", err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("Failed to stat token file: %v", err)
	}

	// Check file permissions (should be 0600)
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestBootstrapTokenFilename(t *testing.T) {
	if bootstrapTokenFilename != ".bootstrap_token" {
		t.Errorf("bootstrapTokenFilename = %q, want %q", bootstrapTokenFilename, ".bootstrap_token")
	}
}

func TestBootstrapTokenHeader(t *testing.T) {
	if bootstrapTokenHeader != "X-Setup-Token" {
		t.Errorf("bootstrapTokenHeader = %q, want %q", bootstrapTokenHeader, "X-Setup-Token")
	}
}

func TestRouter_BootstrapTokenValid_NilRouter(t *testing.T) {
	var r *Router
	if r.bootstrapTokenValid("sometoken") {
		t.Error("nil router should return false")
	}
}

func TestRouter_BootstrapTokenValid_EmptyHash(t *testing.T) {
	r := &Router{
		bootstrapTokenHash: "",
	}
	if r.bootstrapTokenValid("sometoken") {
		t.Error("empty hash should return false")
	}
}

func TestRouter_BootstrapTokenValid_EmptyToken(t *testing.T) {
	r := &Router{
		bootstrapTokenHash: "somehash",
	}
	if r.bootstrapTokenValid("") {
		t.Error("empty token should return false")
	}
}

func TestRouter_BootstrapTokenValid_WhitespaceToken(t *testing.T) {
	r := &Router{
		bootstrapTokenHash: "somehash",
	}
	if r.bootstrapTokenValid("   ") {
		t.Error("whitespace-only token should return false")
	}
}

func TestRouter_ClearBootstrapToken_NilRouter(t *testing.T) {
	var r *Router
	// Should not panic
	r.clearBootstrapToken()
}

func TestRouter_ClearBootstrapToken(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)

	// Create a token file
	if err := os.WriteFile(tokenPath, []byte("testtoken\n"), 0o600); err != nil {
		t.Fatalf("Failed to create token file: %v", err)
	}

	r := &Router{
		bootstrapTokenHash: "somehash",
		bootstrapTokenPath: tokenPath,
	}

	r.clearBootstrapToken()

	// Verify hash and path are cleared
	if r.bootstrapTokenHash != "" {
		t.Errorf("bootstrapTokenHash = %q, want empty", r.bootstrapTokenHash)
	}
	if r.bootstrapTokenPath != "" {
		t.Errorf("bootstrapTokenPath = %q, want empty", r.bootstrapTokenPath)
	}

	// Verify file was deleted
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Error("token file should have been deleted")
	}
}

func TestRouter_ClearBootstrapToken_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, bootstrapTokenFilename)

	r := &Router{
		bootstrapTokenHash: "somehash",
		bootstrapTokenPath: tokenPath,
	}

	// Should not error when file doesn't exist
	r.clearBootstrapToken()

	if r.bootstrapTokenHash != "" {
		t.Errorf("bootstrapTokenHash = %q, want empty", r.bootstrapTokenHash)
	}
}
