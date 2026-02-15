package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCsrfSessionKey(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
	}{
		{
			name:      "simple session ID",
			sessionID: "abc123",
		},
		{
			name:      "UUID format",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "empty string",
			sessionID: "",
		},
		{
			name:      "long session ID",
			sessionID: "very-long-session-id-that-might-come-from-some-external-system-with-lots-of-characters",
		},
		{
			name:      "special characters",
			sessionID: "session/with/slashes!and@special#chars",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := csrfSessionKey(tc.sessionID)

			// Result should be non-empty (hash output)
			if result == "" {
				t.Error("csrfSessionKey() returned empty string")
			}

			// Result should be deterministic
			result2 := csrfSessionKey(tc.sessionID)
			if result != result2 {
				t.Errorf("csrfSessionKey() not deterministic: %q != %q", result, result2)
			}

			// Different inputs should produce different outputs
			if tc.sessionID != "" {
				different := csrfSessionKey(tc.sessionID + "x")
				if result == different {
					t.Error("csrfSessionKey() produced same hash for different inputs")
				}
			}
		})
	}
}

func TestCsrfTokenHash(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "simple token",
			token: "token123",
		},
		{
			name:  "base64 encoded token",
			token: "dGVzdC10b2tlbi1kYXRh",
		},
		{
			name:  "empty string",
			token: "",
		},
		{
			name:  "long token",
			token: "very-long-token-string-that-might-be-generated-by-a-random-generator-function",
		},
		{
			name:  "special characters",
			token: "token/with+special=chars",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := csrfTokenHash(tc.token)

			// Result should be hex encoded SHA256 (64 characters)
			if len(result) != 64 {
				t.Errorf("csrfTokenHash() length = %d, want 64", len(result))
			}

			// Result should only contain hex characters
			for _, c := range result {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("csrfTokenHash() contains non-hex character: %c", c)
				}
			}

			// Result should be deterministic
			result2 := csrfTokenHash(tc.token)
			if result != result2 {
				t.Errorf("csrfTokenHash() not deterministic: %q != %q", result, result2)
			}

			// Different inputs should produce different outputs
			if tc.token != "" {
				different := csrfTokenHash(tc.token + "x")
				if result == different {
					t.Error("csrfTokenHash() produced same hash for different inputs")
				}
			}
		})
	}
}

func TestCSRFToken_Fields(t *testing.T) {
	now := time.Now()
	token := CSRFToken{
		Hash:    "abcdef123456",
		Expires: now,
	}

	if token.Hash != "abcdef123456" {
		t.Errorf("Hash = %q, want abcdef123456", token.Hash)
	}
	if !token.Expires.Equal(now) {
		t.Errorf("Expires = %v, want %v", token.Expires, now)
	}
}

func TestCSRFTokenData_Fields(t *testing.T) {
	now := time.Now()
	data := CSRFTokenData{
		TokenHash:  "hashvalue",
		SessionKey: "sessionkey",
		ExpiresAt:  now,
	}

	if data.TokenHash != "hashvalue" {
		t.Errorf("TokenHash = %q, want hashvalue", data.TokenHash)
	}
	if data.SessionKey != "sessionkey" {
		t.Errorf("SessionKey = %q, want sessionkey", data.SessionKey)
	}
	if !data.ExpiresAt.Equal(now) {
		t.Errorf("ExpiresAt = %v, want %v", data.ExpiresAt, now)
	}
}

func TestCSRFTokenStore_GenerateAndValidate(t *testing.T) {
	// Create a temporary directory for test data
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "test-session-123"

	// Generate a token
	token := store.GenerateCSRFToken(sessionID)
	if token == "" {
		t.Fatal("GenerateCSRFToken() returned empty string")
	}

	// Token should be valid
	if !store.ValidateCSRFToken(sessionID, token) {
		t.Error("ValidateCSRFToken() returned false for valid token")
	}

	// Wrong token should be invalid
	if store.ValidateCSRFToken(sessionID, "wrong-token") {
		t.Error("ValidateCSRFToken() returned true for wrong token")
	}

	// Different session should be invalid
	if store.ValidateCSRFToken("different-session", token) {
		t.Error("ValidateCSRFToken() returned true for wrong session")
	}
}

func TestCSRFTokenStore_DeleteToken(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "test-session-456"

	// Generate a token
	token := store.GenerateCSRFToken(sessionID)

	// Verify it's valid
	if !store.ValidateCSRFToken(sessionID, token) {
		t.Fatal("Token should be valid after generation")
	}

	// Delete it
	store.DeleteCSRFToken(sessionID)

	// Should no longer be valid
	if store.ValidateCSRFToken(sessionID, token) {
		t.Error("Token should be invalid after deletion")
	}
}

func TestCSRFTokenStore_Cleanup(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string]*CSRFToken),
	}

	// Add expired token
	expiredKey := csrfSessionKey("expired-session")
	store.tokens[expiredKey] = &CSRFToken{
		Hash:    "expired-hash",
		Expires: time.Now().Add(-1 * time.Hour), // Expired
	}

	// Add valid token
	validKey := csrfSessionKey("valid-session")
	store.tokens[validKey] = &CSRFToken{
		Hash:    "valid-hash",
		Expires: time.Now().Add(1 * time.Hour), // Not expired
	}

	// Run cleanup
	store.cleanup()

	// Expired token should be removed
	if _, exists := store.tokens[expiredKey]; exists {
		t.Error("Expired token should be removed after cleanup")
	}

	// Valid token should remain
	if _, exists := store.tokens[validKey]; !exists {
		t.Error("Valid token should remain after cleanup")
	}
}

func TestCSRFTokenStore_ExpiredTokenInvalid(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string]*CSRFToken),
	}

	sessionID := "test-session-789"
	token := "test-token"

	// Add token that is already expired
	key := csrfSessionKey(sessionID)
	store.tokens[key] = &CSRFToken{
		Hash:    csrfTokenHash(token),
		Expires: time.Now().Add(-1 * time.Second), // Already expired
	}

	// Should be invalid
	if store.ValidateCSRFToken(sessionID, token) {
		t.Error("Expired token should not validate")
	}
}

func TestCSRFTokenStore_NonexistentSession(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string]*CSRFToken),
	}

	// Should return false for nonexistent session
	if store.ValidateCSRFToken("nonexistent", "any-token") {
		t.Error("Should return false for nonexistent session")
	}
}

func TestCSRFTokenStore_MultipleTokens(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	// Generate tokens for multiple sessions
	session1 := "session-1"
	session2 := "session-2"
	session3 := "session-3"

	token1 := store.GenerateCSRFToken(session1)
	token2 := store.GenerateCSRFToken(session2)
	token3 := store.GenerateCSRFToken(session3)

	// All tokens should be different
	if token1 == token2 || token2 == token3 || token1 == token3 {
		t.Error("Generated tokens should be unique")
	}

	// Each token should only be valid for its session
	if !store.ValidateCSRFToken(session1, token1) {
		t.Error("Token1 should be valid for session1")
	}
	if store.ValidateCSRFToken(session1, token2) {
		t.Error("Token2 should not be valid for session1")
	}
	if !store.ValidateCSRFToken(session2, token2) {
		t.Error("Token2 should be valid for session2")
	}
}

func TestCSRFTokenStore_RegenerateToken(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "regenerate-session"

	// Generate first token
	token1 := store.GenerateCSRFToken(sessionID)

	// Generate second token for same session (replaces first)
	token2 := store.GenerateCSRFToken(sessionID)

	// Tokens should be different
	if token1 == token2 {
		t.Error("Regenerated token should be different")
	}

	// Only new token should be valid
	if store.ValidateCSRFToken(sessionID, token1) {
		t.Error("Old token should be invalid after regeneration")
	}
	if !store.ValidateCSRFToken(sessionID, token2) {
		t.Error("New token should be valid")
	}
}

func TestCSRFTokenStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add a token
	store1 := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "persist-session"
	token := store1.GenerateCSRFToken(sessionID)

	// Explicitly save
	store1.save()

	// Verify file was created
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if _, err := os.Stat(csrfFile); os.IsNotExist(err) {
		t.Fatal("CSRF tokens file was not created")
	}

	// Create new store and load
	store2 := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}
	store2.load()

	// Token should still be valid in new store
	if !store2.ValidateCSRFToken(sessionID, token) {
		t.Error("Token should be valid after save/load")
	}
}

func TestCSRFTokenStore_LoadNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: filepath.Join(tmpDir, "nonexistent"),
	}

	// Should not panic when loading from nonexistent directory
	store.load()

	if store.tokens == nil {
		t.Error("tokens map should be initialized after load")
	}
}

func TestCSRFTokenStore_EmptyTokensMap(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string]*CSRFToken),
	}

	// Cleanup on empty map should not panic
	store.cleanup()

	if len(store.tokens) != 0 {
		t.Error("tokens map should remain empty after cleanup")
	}
}

func TestCSRFTokenStore_SaveUnsafe_MkdirAllError(t *testing.T) {
	// Create a file where the directory should be - MkdirAll will fail
	tmpDir := t.TempDir()
	blockedPath := filepath.Join(tmpDir, "blocked")

	// Create a file at the path where dataPath should be
	if err := os.WriteFile(blockedPath, []byte("blocking file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: blockedPath, // This is a file, not a directory
	}

	// Add a token to save
	store.tokens["testkey"] = &CSRFToken{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}

	// saveUnsafe should handle error gracefully (logs but doesn't panic)
	store.saveUnsafe()

	// Verify the blocking file still exists (wasn't overwritten)
	data, err := os.ReadFile(blockedPath)
	if err != nil {
		t.Fatalf("blocking file was removed: %v", err)
	}
	if string(data) != "blocking file" {
		t.Error("blocking file was modified")
	}
}

func TestCSRFTokenStore_SaveUnsafe_WriteFileError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test running as root")
	}
	// Create a read-only directory to prevent file creation
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")

	if err := os.Mkdir(readOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: readOnlyDir,
	}

	// Add a token
	store.tokens["testkey"] = &CSRFToken{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}

	// Make directory read-only after it exists (MkdirAll succeeds, WriteFile fails)
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to make dir readonly: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0755) // Restore for cleanup
	})

	// saveUnsafe should handle error gracefully (logs but doesn't panic)
	store.saveUnsafe()

	// Verify no file was created
	csrfFile := filepath.Join(readOnlyDir, "csrf_tokens.json")
	if _, err := os.Stat(csrfFile); err == nil {
		t.Error("tokens file should not exist after write failure")
	}
}

func TestCSRFTokenStore_SaveUnsafe_RenameError(t *testing.T) {
	// Create a directory at the target path to cause rename error
	tmpDir := t.TempDir()

	// Create a directory at the exact path where csrf_tokens.json should go
	csrfFilePath := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.Mkdir(csrfFilePath, 0755); err != nil {
		t.Fatalf("failed to create blocking directory: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	// Add a token
	store.tokens["testkey"] = &CSRFToken{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}

	// saveUnsafe should handle rename error gracefully
	store.saveUnsafe()

	// The blocking directory should still exist (rename failed)
	info, err := os.Stat(csrfFilePath)
	if err != nil {
		t.Fatalf("blocking directory was removed: %v", err)
	}
	if !info.IsDir() {
		t.Error("blocking directory was replaced with a file")
	}
}

func TestCSRFTokenStore_Load_ReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory where the tokens file should be - reading it will fail
	csrfPath := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.Mkdir(csrfPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	// Should not panic and should log error
	store.load()

	if len(store.tokens) != 0 {
		t.Errorf("store should be empty after read error, got %d tokens", len(store.tokens))
	}
}

func TestCSRFTokenStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON that won't match either format
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.WriteFile(csrfFile, []byte("not valid json at all"), 0600); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	// Should not panic
	store.load()

	if len(store.tokens) != 0 {
		t.Errorf("store should be empty after loading invalid JSON, got %d tokens", len(store.tokens))
	}
}

func TestCSRFTokenStore_Load_LegacyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create legacy format JSON (map[sessionID]tokenData) with snake_case fields
	legacyJSON := `{
		"session-1": {"token": "token-value-1", "session_id": "session-1", "expires_at": "2099-12-31T23:59:59Z"},
		"session-2": {"token": "token-value-2", "session_id": "session-2", "expires_at": "2099-12-31T23:59:59Z"},
		"session-expired": {"token": "expired-token", "session_id": "session-expired", "expires_at": "2020-01-01T00:00:00Z"}
	}`
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.WriteFile(csrfFile, []byte(legacyJSON), 0600); err != nil {
		t.Fatalf("failed to write legacy format JSON: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	store.load()

	// Should load the two non-expired tokens (session-1 and session-2)
	// The expired one (session-expired) should be skipped
	if len(store.tokens) != 2 {
		t.Errorf("expected 2 tokens from legacy format, got %d", len(store.tokens))
	}

	// Verify tokens are hashed and can be validated
	// The legacy format stores raw tokens, which get hashed during migration
	if !store.ValidateCSRFToken("session-1", "token-value-1") {
		t.Error("should validate token for session-1 after legacy migration")
	}
	if !store.ValidateCSRFToken("session-2", "token-value-2") {
		t.Error("should validate token for session-2 after legacy migration")
	}
}

func TestCSRFTokenStore_Load_CurrentFormat_SkipsNilAndExpired(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current format JSON with expired and nil entries (snake_case fields)
	// This tests the nil check and expiration check in lines 238-240
	currentJSON := `[
		{"token_hash": "hash1", "session_key": "key1", "expires_at": "2099-12-31T23:59:59Z"},
		null,
		{"token_hash": "hash2", "session_key": "key2", "expires_at": "2020-01-01T00:00:00Z"}
	]`
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.WriteFile(csrfFile, []byte(currentJSON), 0600); err != nil {
		t.Fatalf("failed to write current format JSON: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string]*CSRFToken),
		dataPath: tmpDir,
	}

	store.load()

	// Should load only the valid, non-expired token (key1)
	// null entry and expired entry (key2) should be skipped
	if len(store.tokens) != 1 {
		t.Errorf("expected 1 token after filtering, got %d", len(store.tokens))
	}

	// Verify the valid token was loaded
	if _, exists := store.tokens["key1"]; !exists {
		t.Error("expected key1 to be loaded")
	}
}
