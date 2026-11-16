package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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
		tokens:   make(map[string][]*CSRFToken),
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
		tokens:   make(map[string][]*CSRFToken),
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
		tokens: make(map[string][]*CSRFToken),
	}

	// Add expired token
	expiredKey := csrfSessionKey("expired-session")
	store.tokens[expiredKey] = []*CSRFToken{{
		Hash:    "expired-hash",
		Expires: time.Now().Add(-1 * time.Hour), // Expired
	}}

	// Add valid token
	validKey := csrfSessionKey("valid-session")
	store.tokens[validKey] = []*CSRFToken{{
		Hash:    "valid-hash",
		Expires: time.Now().Add(1 * time.Hour), // Not expired
	}}

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

func TestCSRFTokenStore_CleanupPrunesExpiredTokensWithinSession(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string][]*CSRFToken),
	}

	sessionID := "mixed-session"
	expiredToken := "expired-token"
	validToken := "valid-token"
	store.tokens[csrfSessionKey(sessionID)] = []*CSRFToken{
		{
			Hash:    csrfTokenHash(expiredToken),
			Expires: time.Now().Add(-1 * time.Hour),
		},
		{
			Hash:    csrfTokenHash(validToken),
			Expires: time.Now().Add(1 * time.Hour),
		},
	}

	store.cleanup()

	if store.ValidateCSRFToken(sessionID, expiredToken) {
		t.Fatal("expired token should be pruned from the session")
	}
	if !store.ValidateCSRFToken(sessionID, validToken) {
		t.Fatal("valid token should remain after pruning expired session tokens")
	}
	if got := len(store.tokens[csrfSessionKey(sessionID)]); got != 1 {
		t.Fatalf("expected 1 retained token, got %d", got)
	}
}

func TestCSRFTokenStore_ExpiredTokenInvalid(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string][]*CSRFToken),
	}

	sessionID := "test-session-789"
	token := "test-token"

	// Add token that is already expired
	key := csrfSessionKey(sessionID)
	store.tokens[key] = []*CSRFToken{{
		Hash:    csrfTokenHash(token),
		Expires: time.Now().Add(-1 * time.Second), // Already expired
	}}

	// Should be invalid
	if store.ValidateCSRFToken(sessionID, token) {
		t.Error("Expired token should not validate")
	}
}

func TestCSRFTokenStore_NonexistentSession(t *testing.T) {
	store := &CSRFTokenStore{
		tokens: make(map[string][]*CSRFToken),
	}

	// Should return false for nonexistent session
	if store.ValidateCSRFToken("nonexistent", "any-token") {
		t.Error("Should return false for nonexistent session")
	}
}

func TestCSRFTokenStore_MultipleTokens(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
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
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "regenerate-session"

	// Generate first token
	token1 := store.GenerateCSRFToken(sessionID)

	// Generate second token for same session.
	token2 := store.GenerateCSRFToken(sessionID)

	// Tokens should be different
	if token1 == token2 {
		t.Error("Regenerated token should be different")
	}

	// Recent tokens remain valid so concurrent browser retries do not invalidate
	// one another when the server has to issue replacement CSRF cookies.
	if !store.ValidateCSRFToken(sessionID, token1) {
		t.Error("First token should remain valid after bounded regeneration")
	}
	if !store.ValidateCSRFToken(sessionID, token2) {
		t.Error("New token should be valid")
	}

	if got := len(store.tokens[csrfSessionKey(sessionID)]); got != 2 {
		t.Fatalf("expected 2 recent tokens for session, got %d", got)
	}
}

func TestCSRFTokenStore_BoundsRecentTokensPerSession(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "bounded-session"
	generated := make([]string, 0, maxCSRFTokensPerSession+1)
	for i := 0; i < maxCSRFTokensPerSession+1; i++ {
		generated = append(generated, store.GenerateCSRFToken(sessionID))
	}

	if store.ValidateCSRFToken(sessionID, generated[0]) {
		t.Fatal("oldest token should be evicted once the per-session limit is exceeded")
	}
	for i, token := range generated[1:] {
		if !store.ValidateCSRFToken(sessionID, token) {
			t.Fatalf("recent token %d should remain valid", i+1)
		}
	}

	if got := len(store.tokens[csrfSessionKey(sessionID)]); got != maxCSRFTokensPerSession {
		t.Fatalf("expected %d retained tokens, got %d", maxCSRFTokensPerSession, got)
	}
}

func TestCSRFTokenStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add a token
	store1 := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	sessionID := "persist-session"
	token := store1.GenerateCSRFToken(sessionID)
	replacementToken := store1.GenerateCSRFToken(sessionID)

	// Explicitly save
	store1.save()

	// Verify file was created
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if _, err := os.Stat(csrfFile); os.IsNotExist(err) {
		t.Fatal("CSRF tokens file was not created")
	}

	// Create new store and load
	store2 := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}
	store2.load()

	// Token should still be valid in new store
	if !store2.ValidateCSRFToken(sessionID, token) {
		t.Error("Token should be valid after save/load")
	}
	if !store2.ValidateCSRFToken(sessionID, replacementToken) {
		t.Error("Replacement token should be valid after save/load")
	}
}

func TestCSRFTokenStore_LoadNonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
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
		tokens: make(map[string][]*CSRFToken),
	}

	// Cleanup on empty map should not panic
	store.cleanup()

	if len(store.tokens) != 0 {
		t.Error("tokens map should remain empty after cleanup")
	}
}

func TestCSRFTokenStore_Save_MkdirAllError(t *testing.T) {
	// Create a file where the directory should be - MkdirAll will fail
	tmpDir := t.TempDir()
	blockedPath := filepath.Join(tmpDir, "blocked")

	// Create a file at the path where dataPath should be
	if err := os.WriteFile(blockedPath, []byte("blocking file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: blockedPath, // This is a file, not a directory
	}

	// Add a token to save
	store.tokens["testkey"] = []*CSRFToken{{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}}

	// save should handle error gracefully (logs but doesn't panic)
	store.save()

	// Verify the blocking file still exists (wasn't overwritten)
	data, err := os.ReadFile(blockedPath)
	if err != nil {
		t.Fatalf("blocking file was removed: %v", err)
	}
	if string(data) != "blocking file" {
		t.Error("blocking file was modified")
	}
}

func TestCSRFTokenStore_Save_WriteFileError(t *testing.T) {
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
		tokens:   make(map[string][]*CSRFToken),
		dataPath: readOnlyDir,
	}

	// Add a token
	store.tokens["testkey"] = []*CSRFToken{{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}}

	// Make directory read-only after it exists (MkdirAll succeeds, WriteFile fails)
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to make dir readonly: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0755) // Restore for cleanup
	})

	// save should handle error gracefully (logs but doesn't panic)
	store.save()

	// Verify no file was created
	csrfFile := filepath.Join(readOnlyDir, "csrf_tokens.json")
	if _, err := os.Stat(csrfFile); err == nil {
		t.Error("tokens file should not exist after write failure")
	}
}

func TestCSRFTokenStore_Save_RenameError(t *testing.T) {
	// Create a directory at the target path to cause rename error
	tmpDir := t.TempDir()

	// Create a directory at the exact path where csrf_tokens.json should go
	csrfFilePath := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.Mkdir(csrfFilePath, 0755); err != nil {
		t.Fatalf("failed to create blocking directory: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	// Add a token
	store.tokens["testkey"] = []*CSRFToken{{
		Hash:    "testhash",
		Expires: time.Now().Add(time.Hour),
	}}

	// save should handle rename error gracefully
	store.save()

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
		tokens:   make(map[string][]*CSRFToken),
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
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	// Should not panic
	store.load()

	if len(store.tokens) != 0 {
		t.Errorf("store should be empty after loading invalid JSON, got %d tokens", len(store.tokens))
	}
}

func TestCSRFTokenStore_Load_MigratesLegacyFormat(t *testing.T) {
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
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	store.load()

	if len(store.tokens) != 2 {
		t.Fatalf("store should load 2 active legacy tokens, got %d", len(store.tokens))
	}

	if !store.ValidateCSRFToken("session-1", "token-value-1") {
		t.Fatal("session-1 token should validate after legacy load")
	}
	if !store.ValidateCSRFToken("session-2", "token-value-2") {
		t.Fatal("session-2 token should validate after legacy load")
	}
	if store.ValidateCSRFToken("session-expired", "expired-token") {
		t.Fatal("expired legacy CSRF token should be filtered during load")
	}

	if _, ok := store.tokens[csrfSessionKey("session-1")]; !ok {
		t.Fatal("session-1 should be stored under its hashed session key")
	}
	if _, ok := store.tokens[csrfSessionKey("session-2")]; !ok {
		t.Fatal("session-2 should be stored under its hashed session key")
	}

	savedData, err := os.ReadFile(csrfFile)
	if err != nil {
		t.Fatalf("failed to read migrated csrf tokens file: %v", err)
	}

	var persisted []*CSRFTokenData
	if err := json.Unmarshal(savedData, &persisted); err != nil {
		t.Fatalf("migrated csrf tokens file should be hashed array format: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("migrated csrf tokens file should contain 2 active tokens, got %d", len(persisted))
	}
}

func TestCSRFTokenStore_Load_CurrentFormat_SkipsNilAndExpired(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current format JSON with expired and nil entries (snake_case fields)
	// to verify nil records and expired tokens are ignored during load.
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
		tokens:   make(map[string][]*CSRFToken),
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

func TestCSRFTokenStore_Load_CurrentFormat_RetainsRecentTokensForSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "persisted-concurrent-session"
	sessionKey := csrfSessionKey(sessionID)
	records := []*CSRFTokenData{
		{
			TokenHash:  csrfTokenHash("token-one"),
			SessionKey: sessionKey,
			ExpiresAt:  time.Now().Add(time.Hour),
		},
		{
			TokenHash:  csrfTokenHash("token-two"),
			SessionKey: sessionKey,
			ExpiresAt:  time.Now().Add(time.Hour),
		},
	}

	data, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("failed to marshal current format records: %v", err)
	}
	csrfFile := filepath.Join(tmpDir, "csrf_tokens.json")
	if err := os.WriteFile(csrfFile, data, 0600); err != nil {
		t.Fatalf("failed to write current format JSON: %v", err)
	}

	store := &CSRFTokenStore{
		tokens:   make(map[string][]*CSRFToken),
		dataPath: tmpDir,
	}

	store.load()

	if !store.ValidateCSRFToken(sessionID, "token-one") {
		t.Fatal("first persisted token should validate")
	}
	if !store.ValidateCSRFToken(sessionID, "token-two") {
		t.Fatal("second persisted token should validate")
	}
	if got := len(store.tokens[sessionKey]); got != 2 {
		t.Fatalf("expected 2 retained tokens for session, got %d", got)
	}
}

func TestCSRFTokenStore_InitReconfiguresDataPath(t *testing.T) {
	resetCSRFStoreForTests()
	t.Cleanup(resetCSRFStoreForTests)

	dirOne := t.TempDir()
	dirTwo := t.TempDir()

	InitCSRFStore(dirOne)
	token := GetCSRFStore().GenerateCSRFToken("session-one")

	InitCSRFStore(dirTwo)
	if GetCSRFStore().ValidateCSRFToken("session-one", token) {
		t.Fatal("reconfigured csrf store should not retain tokens from the previous data path")
	}

	GetCSRFStore().GenerateCSRFToken("session-two")
	if _, err := os.Stat(filepath.Join(dirOne, "csrf_tokens.json")); err != nil {
		t.Fatalf("expected original csrf tokens file to remain readable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirTwo, "csrf_tokens.json")); err != nil {
		t.Fatalf("expected reconfigured csrf tokens file to exist: %v", err)
	}
}

func TestEnsureCSRFStoreReturnsOwnedStoreForPath(t *testing.T) {
	resetCSRFStoreForTests()
	t.Cleanup(resetCSRFStoreForTests)

	dirOne := t.TempDir()
	dirTwo := t.TempDir()

	storeOne := ensureCSRFStore(dirOne)
	if storeOne == nil {
		t.Fatal("ensureCSRFStore should initialize a store for a valid data path")
	}
	if storeOne != GetCSRFStore() {
		t.Fatal("ensureCSRFStore should return the active store for the configured data path")
	}

	storeTwo := ensureCSRFStore(dirTwo)
	if storeTwo == nil {
		t.Fatal("ensureCSRFStore should initialize a reconfigured store")
	}
	if storeTwo != GetCSRFStore() {
		t.Fatal("ensureCSRFStore should return the reconfigured active store")
	}
	if storeOne == storeTwo {
		t.Fatal("ensureCSRFStore should return a distinct store after data-path reconfiguration")
	}

	select {
	case <-storeOne.workerDone:
	default:
		t.Fatal("reconfigured csrf store should shut down the previous worker")
	}
}

func TestCSRFTokenStore_ShutdownReturnsWithoutWaitingForWorkerReceive(t *testing.T) {
	storeDir := t.TempDir()
	InitCSRFStore(storeDir)
	store := GetCSRFStore()
	t.Cleanup(resetCSRFStoreForTests)

	done := make(chan struct{})
	go func() {
		store.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Shutdown should not block waiting for the background worker to receive a stop signal")
	}
}

func csrfStartupStore(dir string) *CSRFTokenStore {
	store := &CSRFTokenStore{tokens: make(map[string][]*CSRFToken), dataPath: dir}
	store.load()
	return store
}

func csrfUnreadFixture(t *testing.T, data []byte) (*CSRFTokenStore, os.FileInfo) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "csrf_tokens.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return csrfStartupStore(dir), info
}

// Check retained bytes through an ordinary private backup read, not by opening
// the original before a formerly unreadable mode has been normalized.
func csrfCheckPreserved(t *testing.T, dir string, original os.FileInfo, data []byte) string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(dir, ".csrf-tokens-unread-*"))
	if err != nil || len(dirs) != 1 {
		t.Fatalf("expected exactly one preserved original, found %d: %v", len(dirs), err)
	}
	info, err := os.Stat(dirs[0])
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatalf("preservation directory is not private: %v", err)
	}
	path := filepath.Join(dirs[0], "csrf_tokens.json")
	info, err = os.Stat(path)
	if err != nil || !os.SameFile(original, info) || info.Mode().Perm() != 0600 {
		t.Fatalf("original inode was not retained owner-private: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("retained original is not byte-exact/readable: %v", err)
	}
	return path
}

func TestCSRFTokenStore_UnreadStartupEverySave(t *testing.T) {
	for _, data := range []struct{ name, raw string }{
		{"zero", ""}, {"whitespace", " \n\t"}, {"invalid", "{broken"},
		{"partial-current", `[{"token_hash":"unknown","session_key":"unknown"},`},
		{"wrong-type", `"not an inventory"`}, {"partial-legacy", `{"unknown":{"token":"unknown"},`},
	} {
		for _, trigger := range []string{"save", "generate", "delete-no-match", "delete-keys-no-match", "periodic", "shutdown"} {
			t.Run(data.name+"/"+trigger, func(t *testing.T) {
				store, original := csrfUnreadFixture(t, []byte(data.raw))
				if len(store.tokens) != 0 {
					t.Fatal("incomplete input admitted partial token authority")
				}
				var fresh string
				switch trigger {
				case "generate":
					fresh = store.GenerateCSRFToken("current-session")
				case "delete-no-match":
					store.DeleteCSRFToken("unknown-session")
				case "delete-keys-no-match":
					store.DeleteCSRFSessionKeys([]string{csrfSessionKey("unknown-session")})
				case "periodic", "shutdown":
					store.saveTicker = time.NewTicker(time.Millisecond)
					store.stopChan = make(chan bool)
					store.workerDone = make(chan struct{})
					go store.backgroundWorker()
					t.Cleanup(store.Shutdown)
					if trigger == "periodic" {
						observed := false
						deadline := time.Now().Add(3 * time.Second)
						for time.Now().Before(deadline) {
							paths, _ := filepath.Glob(filepath.Join(store.dataPath, ".csrf-tokens-unread-*", "csrf_tokens.json"))
							if len(paths) == 1 {
								observed = true
								break
							}
							time.Sleep(time.Millisecond)
						}
						if !observed {
							t.Fatal("periodic worker did not preserve the original before shutdown")
						}
					}
					store.Shutdown()
				default:
					store.save()
				}
				csrfCheckPreserved(t, store.dataPath, original, []byte(data.raw))
				store.save()
				csrfCheckPreserved(t, store.dataPath, original, []byte(data.raw))
				loaded := csrfStartupStore(store.dataPath)
				if len(loaded.tokens) != 0 && fresh == "" {
					t.Fatal("restart loaded unknown retained authority")
				}
				if fresh != "" && !loaded.ValidateCSRFToken("current-session", fresh) {
					t.Fatal("preservation blocked fresh canonical token persistence")
				}
			})
		}
	}
}

func TestCSRFTokenStore_UnreadStartupReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary-user POSIX file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "csrf_tokens.json")
	data, err := json.Marshal([]CSRFTokenData{{SessionKey: csrfSessionKey("old-session"), TokenHash: csrfTokenHash("old-token"), ExpiresAt: time.Now().Add(time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Fatal("fixture did not fail a real read")
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	store := csrfStartupStore(dir)
	fresh := store.GenerateCSRFToken("current-session")
	csrfCheckPreserved(t, dir, original, data)
	for _, current := range []*CSRFTokenStore{store, csrfStartupStore(dir)} {
		if current.ValidateCSRFToken("old-session", "old-token") || !current.ValidateCSRFToken("current-session", fresh) {
			t.Fatal("read recovery restored unknown tokens or lost current tokens")
		}
	}
}

func TestCSRFTokenStore_UnreadStartupFailureRecovery(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary-user POSIX directory permissions")
	}
	const raw = "synthetic broken inventory"
	store, original := csrfUnreadFixture(t, []byte(raw))
	t.Cleanup(func() { _ = os.Chmod(store.dataPath, 0700) })
	if err := os.Chmod(store.dataPath, 0500); err != nil {
		t.Fatal(err)
	}
	target := store.GenerateCSRFToken("target-session")
	control := store.GenerateCSRFToken("control-session")
	store.DeleteCSRFToken("target-session")
	store.DeleteCSRFSessionKeys([]string{csrfSessionKey("already-absent")})
	if store.ValidateCSRFToken("target-session", target) || !store.ValidateCSRFToken("control-session", control) {
		t.Fatal("preservation failure blocked live withdrawal/validation")
	}
	path := filepath.Join(store.dataPath, "csrf_tokens.json")
	got, err := os.ReadFile(path)
	if err != nil || string(got) != raw {
		t.Fatal("failed preservation overwrote the unknown original")
	}
	if err := os.Chmod(store.dataPath, 0700); err != nil {
		t.Fatal(err)
	}
	store.save()
	csrfCheckPreserved(t, store.dataPath, original, []byte(raw))
	loaded := csrfStartupStore(store.dataPath)
	if loaded.ValidateCSRFToken("target-session", target) || !loaded.ValidateCSRFToken("control-session", control) {
		t.Fatal("later storage repair lost control or revived withdrawn tokens")
	}
}

func TestCSRFTokenStore_UnreadStartupSnapshotFailureRecovery(t *testing.T) {
	const raw = "synthetic broken inventory"
	store, original := csrfUnreadFixture(t, []byte(raw))
	store.snapshotWrite = func(*os.File, []byte) (int, error) { return 0, nil }
	target := store.GenerateCSRFToken("target-session")
	preserved := csrfCheckPreserved(t, store.dataPath, original, []byte(raw))
	if _, err := os.Stat(filepath.Join(store.dataPath, "csrf_tokens.json")); !os.IsNotExist(err) {
		t.Fatal("failed fresh snapshot was published")
	}
	if !store.ValidateCSRFToken("target-session", target) {
		t.Fatal("failed publication rejected current memory-only token")
	}
	store.DeleteCSRFToken("target-session")
	store.snapshotWrite = nil
	control := store.GenerateCSRFToken("control-session")
	if got := csrfCheckPreserved(t, store.dataPath, original, []byte(raw)); got != preserved {
		t.Fatal("later save moved the already-preserved original again")
	}
	loaded := csrfStartupStore(store.dataPath)
	if loaded.ValidateCSRFToken("target-session", target) || !loaded.ValidateCSRFToken("control-session", control) {
		t.Fatal("snapshot recovery revived withdrawal or lost current tokens")
	}
	data, err := os.ReadFile(filepath.Join(store.dataPath, "csrf_tokens.json"))
	if err != nil || bytes.Contains(data, []byte(control)) || bytes.Contains(data, []byte("control-session")) {
		t.Fatal("fresh canonical snapshot disclosed a raw token or session")
	}
}

func TestCSRFTokenStore_UnreadStartupNonregular(t *testing.T) {
	for _, kind := range []string{"directory", "symlink", "dangling-symlink"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" && kind != "directory" {
				t.Skip("requires unprivileged symlink creation")
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "csrf_tokens.json")
			target := filepath.Join(dir, "unowned-target")
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				target = filepath.Join(path, "marker")
			} else if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if kind != "dangling-symlink" {
				if err := os.WriteFile(target, []byte("synthetic unread inventory"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			original, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			store := csrfStartupStore(dir)
			fresh := store.GenerateCSRFToken("current-session")
			store.save()
			current, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			// Multiple atomic saves can recycle the old entry's inode. Inode
			// equality alone does not prove that a link stayed a link.
			if current.Mode().Type() != original.Mode().Type() || !os.SameFile(original, current) || !store.ValidateCSRFToken("current-session", fresh) {
				t.Fatal("unread path was changed or current memory-only token rejected")
			}
			if kind != "directory" {
				if got, err := os.Readlink(path); err != nil || got != target {
					t.Fatal("save replaced or retargeted the unread link")
				}
			}
			if kind == "dangling-symlink" {
				if _, err := os.Lstat(target); !os.IsNotExist(err) {
					t.Fatal("save created the unknown target")
				}
			} else if got, err := os.ReadFile(target); err != nil || string(got) != "synthetic unread inventory" {
				t.Fatal("save changed a nonregular path's target/contents")
			}
			// Moving the unread entry, not making its target readable, releases
			// the hold. Never implicitly load the newly available unknown tokens.
			if err := os.Rename(path, path+".operator-moved"); err != nil {
				t.Fatal(err)
			}
			store.DeleteCSRFToken("current-session")
			control := store.GenerateCSRFToken("control-session")
			loaded := csrfStartupStore(dir)
			if loaded.ValidateCSRFToken("current-session", fresh) || !loaded.ValidateCSRFToken("control-session", control) {
				t.Fatal("operator movement did not persist exactly current authority")
			}
		})
	}
}

func TestCSRFTokenStore_ReadableStartupCompatibility(t *testing.T) {
	for _, raw := range []string{"[]", "null", "{}"} {
		t.Run(raw, func(t *testing.T) {
			store, _ := csrfUnreadFixture(t, []byte(raw))
			token := store.GenerateCSRFToken("current-session")
			if !csrfStartupStore(store.dataPath).ValidateCSRFToken("current-session", token) {
				t.Fatal("complete empty format blocked ordinary fresh persistence")
			}
			dirs, err := filepath.Glob(filepath.Join(store.dataPath, ".csrf-tokens-unread-*"))
			if err != nil || len(dirs) != 0 {
				t.Fatal("valid empty input was unnecessarily preserved")
			}
		})
	}
}

// A real registered write must still require a session-bound CSRF token. The
// malformed body reaches body admission (400) only after both auth checks.
func TestCSRFTokenStore_UnreadStartupRegisteredAdmission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "csrf_tokens.json")
	const raw = "synthetic unread inventory"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_DATA_DIR", dir)
	hash, err := internalauth.HashPassword("SyntheticPassword!1")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AuthUser: "synthetic-admin", AuthPass: hash}
	router := newTestRouter(t, cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	const session = "synthetic-unread-csrf-session"
	router.sessionStore.CreateSession(session, time.Hour, "fixture", "192.0.2.1", "synthetic-admin")
	token := router.csrfStore.GenerateCSRFToken(session)
	csrfCheckPreserved(t, dir, original, []byte(raw))
	for _, tc := range []struct {
		name, token string
		want        int
	}{{"valid", token, http.StatusBadRequest}, {"missing", "", http.StatusForbidden}, {"wrong", "wrong", http.StatusForbidden}, {"other-session", router.csrfStore.GenerateCSRFToken("other-session"), http.StatusForbidden}, {"revoked", token, http.StatusForbidden}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "revoked" {
				router.csrfStore.DeleteCSRFSessionKeys([]string{csrfSessionKey(session)})
			}
			request := httptest.NewRequest(http.MethodPost, "https://pulse.example.test/api/security/tokens", strings.NewReader("{not-json"))
			request.Header.Set("Origin", "https://pulse.example.test")
			request.Header.Set("Sec-Fetch-Site", "same-origin")
			request.Header.Set("X-CSRF-Token", tc.token)
			request.AddCookie(&http.Cookie{Name: sessionCookieName(true), Value: session})
			response := httptest.NewRecorder()
			router.Handler().ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("registered admission: got %d want %d", response.Code, tc.want)
			}
		})
	}
	if csrfStartupStore(dir).ValidateCSRFToken(session, token) {
		t.Fatal("restart restored withdrawn tokens from preserved material")
	}
}
