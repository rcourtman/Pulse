package api

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRecoveryToken_Fields(t *testing.T) {
	now := time.Now()
	expiry := now.Add(time.Hour)
	usedAt := now.Add(30 * time.Minute)

	token := RecoveryToken{
		Token:     "abc123token",
		CreatedAt: now,
		ExpiresAt: expiry,
		Used:      true,
		UsedAt:    usedAt,
		IP:        "192.168.1.100",
	}

	if token.Token != "abc123token" {
		t.Errorf("Token = %q, want abc123token", token.Token)
	}
	if !token.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", token.CreatedAt, now)
	}
	if !token.ExpiresAt.Equal(expiry) {
		t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, expiry)
	}
	if !token.Used {
		t.Error("Used = false, want true")
	}
	if !token.UsedAt.Equal(usedAt) {
		t.Errorf("UsedAt = %v, want %v", token.UsedAt, usedAt)
	}
	if token.IP != "192.168.1.100" {
		t.Errorf("IP = %q, want 192.168.1.100", token.IP)
	}
}

func newTestRecoveryStore(t *testing.T) *RecoveryTokenStore {
	t.Helper()
	return &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    t.TempDir(),
		stopCleanup: make(chan struct{}),
	}
}

func TestRecoveryTokenStore_GenerateRecoveryToken(t *testing.T) {
	store := newTestRecoveryStore(t)

	token, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Token should be 64 hex characters (32 bytes)
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64", len(token))
	}

	// Should be valid hex
	if _, err := hex.DecodeString(token); err != nil {
		t.Errorf("token is not valid hex: %v", err)
	}

	// Should be stored
	store.mu.RLock()
	stored, exists := store.tokens[token]
	store.mu.RUnlock()

	if !exists {
		t.Fatal("token not found in store")
	}
	if stored.Used {
		t.Error("new token should not be marked as used")
	}
	if stored.ExpiresAt.Before(time.Now()) {
		t.Error("new token should not be expired")
	}
}

func TestRecoveryTokenStore_GenerateRecoveryToken_Uniqueness(t *testing.T) {
	store := newTestRecoveryStore(t)

	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := store.GenerateRecoveryToken(time.Hour)
		if err != nil {
			t.Fatalf("GenerateRecoveryToken failed on iteration %d: %v", i, err)
		}
		if tokens[token] {
			t.Errorf("duplicate token generated: %s", token)
		}
		tokens[token] = true
	}
}

func TestRecoveryTokenStore_GenerateRecoveryToken_ExpiryDurations(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"1 minute", time.Minute},
		{"5 minutes", 5 * time.Minute},
		{"1 hour", time.Hour},
		{"24 hours", 24 * time.Hour},
		{"1 week", 7 * 24 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestRecoveryStore(t)
			beforeGen := time.Now()

			token, err := store.GenerateRecoveryToken(tc.duration)
			if err != nil {
				t.Fatalf("GenerateRecoveryToken failed: %v", err)
			}

			afterGen := time.Now()

			store.mu.RLock()
			stored := store.tokens[token]
			store.mu.RUnlock()

			// ExpiresAt should be approximately beforeGen + duration to afterGen + duration
			expectedMin := beforeGen.Add(tc.duration)
			expectedMax := afterGen.Add(tc.duration)

			if stored.ExpiresAt.Before(expectedMin) || stored.ExpiresAt.After(expectedMax) {
				t.Errorf("ExpiresAt = %v, want between %v and %v", stored.ExpiresAt, expectedMin, expectedMax)
			}
		})
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_ValidToken(t *testing.T) {
	store := newTestRecoveryStore(t)

	token, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Validate token
	if !store.ValidateRecoveryTokenConstantTime(token, "10.0.0.1") {
		t.Error("ValidateRecoveryTokenConstantTime returned false for valid token")
	}

	// Check it's marked as used
	store.mu.RLock()
	stored := store.tokens[token]
	store.mu.RUnlock()

	if !stored.Used {
		t.Error("token should be marked as used after validation")
	}
	if stored.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", stored.IP)
	}
	if stored.UsedAt.IsZero() {
		t.Error("UsedAt should be set")
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_InvalidToken(t *testing.T) {
	store := newTestRecoveryStore(t)

	// Generate a valid token but try to validate with a different one
	_, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	if store.ValidateRecoveryTokenConstantTime("nonexistent-token", "10.0.0.1") {
		t.Error("ValidateRecoveryTokenConstantTime returned true for invalid token")
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_ExpiredToken(t *testing.T) {
	store := newTestRecoveryStore(t)

	// Create an already-expired token directly
	expiredToken := "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	store.mu.Lock()
	store.tokens[expiredToken] = &RecoveryToken{
		Token:     expiredToken,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Used:      false,
	}
	store.mu.Unlock()

	if store.ValidateRecoveryTokenConstantTime(expiredToken, "10.0.0.1") {
		t.Error("ValidateRecoveryTokenConstantTime returned true for expired token")
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_UsedToken(t *testing.T) {
	store := newTestRecoveryStore(t)

	token, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Use the token
	if !store.ValidateRecoveryTokenConstantTime(token, "10.0.0.1") {
		t.Fatal("first validation should succeed")
	}

	// Try to use again
	if store.ValidateRecoveryTokenConstantTime(token, "10.0.0.2") {
		t.Error("ValidateRecoveryTokenConstantTime returned true for already-used token")
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_EmptyStore(t *testing.T) {
	store := newTestRecoveryStore(t)

	if store.ValidateRecoveryTokenConstantTime("any-token", "10.0.0.1") {
		t.Error("ValidateRecoveryTokenConstantTime returned true on empty store")
	}
}

func TestRecoveryTokenStore_ValidateRecoveryTokenConstantTime_ConcurrentUse(t *testing.T) {
	store := newTestRecoveryStore(t)

	token, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Try to use token concurrently from multiple goroutines
	const numGoroutines = 100
	results := make(chan bool, numGoroutines)
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			result := store.ValidateRecoveryTokenConstantTime(token, "10.0.0.1")
			results <- result
		}(i)
	}

	wg.Wait()
	close(results)

	// Count successes - only 1 should succeed
	successes := 0
	for result := range results {
		if result {
			successes++
		}
	}

	if successes != 1 {
		t.Errorf("concurrent validation successes = %d, want exactly 1", successes)
	}
}

func TestRecoveryTokenStore_Cleanup(t *testing.T) {
	store := newTestRecoveryStore(t)

	// Add tokens: one valid, one expired, one used long ago
	validToken := "valid123valid123valid123valid123valid123valid123valid123valid123"
	expiredToken := "expired1expired1expired1expired1expired1expired1expired1expired1"
	usedOldToken := "usedold1usedold1usedold1usedold1usedold1usedold1usedold1usedold1"

	store.mu.Lock()
	store.tokens[validToken] = &RecoveryToken{
		Token:     validToken,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Used:      false,
	}
	store.tokens[expiredToken] = &RecoveryToken{
		Token:     expiredToken,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
		Used:      false,
	}
	store.tokens[usedOldToken] = &RecoveryToken{
		Token:     usedOldToken,
		CreatedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-47 * time.Hour),
		Used:      true,
		UsedAt:    time.Now().Add(-25 * time.Hour), // Used more than 24 hours ago
	}
	store.mu.Unlock()

	// Run cleanup
	store.cleanup()

	store.mu.RLock()
	_, validExists := store.tokens[validToken]
	_, expiredExists := store.tokens[expiredToken]
	_, usedOldExists := store.tokens[usedOldToken]
	store.mu.RUnlock()

	if !validExists {
		t.Error("valid token was incorrectly removed during cleanup")
	}
	if expiredExists {
		t.Error("expired token was not removed during cleanup")
	}
	if usedOldExists {
		t.Error("old used token was not removed during cleanup")
	}
}

func TestRecoveryTokenStore_Cleanup_RemovesExpiredEvenIfRecentlyUsed(t *testing.T) {
	store := newTestRecoveryStore(t)

	// Add a token that's expired but was used recently (less than 24 hours ago)
	// Per the cleanup logic: removes if expired OR used more than 24 hours ago
	// So an expired token will be removed even if recently used
	recentlyUsedToken := "recent1recent1recent1recent1recent1recent1recent1recent1recent1r"

	store.mu.Lock()
	store.tokens[recentlyUsedToken] = &RecoveryToken{
		Token:     recentlyUsedToken,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Used:      true,
		UsedAt:    time.Now().Add(-time.Hour), // Used 1 hour ago (within 24 hours)
	}
	store.mu.Unlock()

	store.cleanup()

	store.mu.RLock()
	_, exists := store.tokens[recentlyUsedToken]
	store.mu.RUnlock()

	// Cleanup removes expired tokens regardless of when they were used
	if exists {
		t.Error("expired token should be removed during cleanup even if recently used")
	}
}

func TestRecoveryTokenStore_Cleanup_KeepsUnexpiredUsedTokens(t *testing.T) {
	store := newTestRecoveryStore(t)

	// A used token that hasn't expired yet and was used recently should be kept
	usedNotExpiredToken := "usednot1usednot1usednot1usednot1usednot1usednot1usednot1usednot1"

	store.mu.Lock()
	store.tokens[usedNotExpiredToken] = &RecoveryToken{
		Token:     usedNotExpiredToken,
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour), // Not expired
		Used:      true,
		UsedAt:    time.Now().Add(-time.Hour), // Used 1 hour ago
	}
	store.mu.Unlock()

	store.cleanup()

	store.mu.RLock()
	_, exists := store.tokens[usedNotExpiredToken]
	store.mu.RUnlock()

	if !exists {
		t.Error("used but not expired token should be kept during cleanup")
	}
}

func TestRecoveryTokenStore_Persistence_Save(t *testing.T) {
	tmpDir := t.TempDir()

	store := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	// Generate a token (which triggers save)
	token, err := store.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Verify file was created
	tokensFile := filepath.Join(tmpDir, "recovery_tokens.json")
	if _, err := os.Stat(tokensFile); os.IsNotExist(err) {
		t.Fatal("recovery_tokens.json was not created")
	}

	// Read the file and verify content includes the token
	data, err := os.ReadFile(tokensFile)
	if err != nil {
		t.Fatalf("failed to read recovery_tokens.json: %v", err)
	}

	// Token should appear in the JSON (at least partially)
	if len(data) == 0 {
		t.Error("recovery_tokens.json is empty")
	}

	// The token string should appear in the file content
	if !containsTokenSubstring(string(data), token[:16]) {
		t.Error("token not found in persisted file")
	}
}

func TestRecoveryTokenStore_Persistence_Load(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first store and generate token
	store1 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	token, err := store1.GenerateRecoveryToken(time.Hour)
	if err != nil {
		t.Fatalf("GenerateRecoveryToken failed: %v", err)
	}

	// Create second store and load
	store2 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}
	store2.load()

	// Token should be loaded and valid
	store2.mu.RLock()
	loaded, exists := store2.tokens[token]
	store2.mu.RUnlock()

	if !exists {
		t.Fatal("token was not loaded from disk")
	}
	if loaded.Used {
		t.Error("loaded token should not be marked as used")
	}
}

func TestRecoveryTokenStore_Persistence_FilterExpiredOnLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store with an expired token
	store1 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	expiredToken := "expired1expired1expired1expired1expired1expired1expired1expired1"
	store1.mu.Lock()
	store1.tokens[expiredToken] = &RecoveryToken{
		Token:     expiredToken,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Used:      false,
	}
	store1.saveUnsafe()
	store1.mu.Unlock()

	// Create second store and load
	store2 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}
	store2.load()

	// Expired token should not be loaded
	store2.mu.RLock()
	_, exists := store2.tokens[expiredToken]
	store2.mu.RUnlock()

	if exists {
		t.Error("expired token should not be loaded from disk")
	}
}

func TestRecoveryTokenStore_Persistence_KeepRecentlyUsedOnLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store with a recently used (but expired) token
	store1 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	recentToken := "recent1recent1recent1recent1recent1recent1recent1recent1recent1r"
	store1.mu.Lock()
	store1.tokens[recentToken] = &RecoveryToken{
		Token:     recentToken,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Used:      true,
		UsedAt:    time.Now().Add(-time.Hour), // Used within 24 hours
	}
	store1.saveUnsafe()
	store1.mu.Unlock()

	// Create second store and load
	store2 := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}
	store2.load()

	// Recently used token should be loaded for audit trail
	store2.mu.RLock()
	_, exists := store2.tokens[recentToken]
	store2.mu.RUnlock()

	if !exists {
		t.Error("recently used token should be loaded from disk for audit trail")
	}
}

func TestRecoveryTokenStore_Load_MissingFile(t *testing.T) {
	store := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    t.TempDir(), // Empty directory
		stopCleanup: make(chan struct{}),
	}

	// Should not panic
	store.load()

	store.mu.RLock()
	count := len(store.tokens)
	store.mu.RUnlock()

	if count != 0 {
		t.Errorf("store should be empty after loading missing file, got %d tokens", count)
	}
}

func TestRecoveryTokenStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON
	tokensFile := filepath.Join(tmpDir, "recovery_tokens.json")
	if err := os.WriteFile(tokensFile, []byte("{invalid json}"), 0600); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	store := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	// Should not panic
	store.load()

	store.mu.RLock()
	count := len(store.tokens)
	store.mu.RUnlock()

	if count != 0 {
		t.Errorf("store should be empty after loading invalid JSON, got %d tokens", count)
	}
}

func TestRecoveryTokenStore_Load_ReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory where the tokens file should be - reading it will fail
	tokensPath := filepath.Join(tmpDir, "recovery_tokens.json")
	if err := os.Mkdir(tokensPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	store := &RecoveryTokenStore{
		tokens:      make(map[string]*RecoveryToken),
		dataPath:    tmpDir,
		stopCleanup: make(chan struct{}),
	}

	// Should not panic and should log error
	store.load()

	store.mu.RLock()
	count := len(store.tokens)
	store.mu.RUnlock()

	if count != 0 {
		t.Errorf("store should be empty after read error, got %d tokens", count)
	}
}

func TestRecoveryTokenStore_StopCleanup(t *testing.T) {
	store := newTestRecoveryStore(t)

	// Start cleanup routine
	done := make(chan struct{})
	go func() {
		store.cleanupRoutine()
		close(done)
	}()

	// Stop it
	close(store.stopCleanup)

	// Wait for routine to exit (with timeout)
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("cleanupRoutine did not stop within timeout")
	}
}

// Helper function
func containsTokenSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
