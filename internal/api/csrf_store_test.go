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
