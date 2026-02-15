package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionHash(t *testing.T) {
	tests := []struct {
		name   string
		token1 string
		token2 string
		same   bool
	}{
		{
			name:   "same token produces same hash",
			token1: "test-token-123",
			token2: "test-token-123",
			same:   true,
		},
		{
			name:   "different tokens produce different hashes",
			token1: "token-a",
			token2: "token-b",
			same:   false,
		},
		{
			name:   "empty token produces consistent hash",
			token1: "",
			token2: "",
			same:   true,
		},
		{
			name:   "empty vs non-empty token",
			token1: "",
			token2: "something",
			same:   false,
		},
		{
			name:   "case sensitive",
			token1: "Token",
			token2: "token",
			same:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash1 := sessionHash(tc.token1)
			hash2 := sessionHash(tc.token2)

			if tc.same && hash1 != hash2 {
				t.Errorf("sessionHash(%q) = %q, sessionHash(%q) = %q; want same", tc.token1, hash1, tc.token2, hash2)
			}
			if !tc.same && hash1 == hash2 {
				t.Errorf("sessionHash(%q) and sessionHash(%q) produced same hash %q; want different", tc.token1, tc.token2, hash1)
			}
		})
	}
}

func TestSessionHash_Format(t *testing.T) {
	hash := sessionHash("test-token")

	// SHA256 produces 64 hex characters
	if len(hash) != 64 {
		t.Errorf("sessionHash length = %d, want 64", len(hash))
	}

	// Should only contain hex characters
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("sessionHash contains non-hex character: %c", c)
		}
	}
}

func TestSessionHash_Deterministic(t *testing.T) {
	token := "reproducible-token-value"

	// Call multiple times
	results := make([]string, 100)
	for i := range results {
		results[i] = sessionHash(token)
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("sessionHash call %d returned %q, want %q", i, results[i], results[0])
		}
	}
}

func TestSessionData_Fields(t *testing.T) {
	now := time.Now()
	duration := 24 * time.Hour

	data := SessionData{
		ExpiresAt:        now.Add(duration),
		CreatedAt:        now,
		UserAgent:        "Mozilla/5.0",
		IP:               "192.168.1.100",
		OriginalDuration: duration,
	}

	if data.ExpiresAt.Before(data.CreatedAt) {
		t.Error("ExpiresAt should be after CreatedAt")
	}
	if data.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q, want Mozilla/5.0", data.UserAgent)
	}
	if data.IP != "192.168.1.100" {
		t.Errorf("IP = %q, want 192.168.1.100", data.IP)
	}
	if data.OriginalDuration != duration {
		t.Errorf("OriginalDuration = %v, want %v", data.OriginalDuration, duration)
	}
}

func TestSessionStore_CreateAndValidate(t *testing.T) {
	tmpDir := t.TempDir()

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	token := "test-session-token"
	duration := time.Hour

	// Create session
	store.CreateSession(token, duration, "TestAgent", "127.0.0.1", "testuser")

	// Validate session
	if !store.ValidateSession(token) {
		t.Error("ValidateSession returned false for valid session")
	}

	// Verify internal state
	store.mu.RLock()
	key := sessionHash(token)
	session, exists := store.sessions[key]
	store.mu.RUnlock()

	if !exists {
		t.Fatal("Session not found in store")
	}
	if session.UserAgent != "TestAgent" {
		t.Errorf("UserAgent = %q, want TestAgent", session.UserAgent)
	}
	if session.IP != "127.0.0.1" {
		t.Errorf("IP = %q, want 127.0.0.1", session.IP)
	}
	if session.OriginalDuration != duration {
		t.Errorf("OriginalDuration = %v, want %v", session.OriginalDuration, duration)
	}
}

func TestSessionStore_ValidateSession_NonExistent(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	if store.ValidateSession("nonexistent-token") {
		t.Error("ValidateSession returned true for nonexistent session")
	}
}

func TestSessionStore_ValidateSession_Expired(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	token := "expired-token"
	key := sessionHash(token)

	// Add an already-expired session
	store.mu.Lock()
	store.sessions[key] = &SessionData{
		ExpiresAt: time.Now().Add(-time.Hour), // Expired an hour ago
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	store.mu.Unlock()

	if store.ValidateSession(token) {
		t.Error("ValidateSession returned true for expired session")
	}
}

func TestSessionStore_DeleteSession(t *testing.T) {
	tmpDir := t.TempDir()

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	token := "delete-me-token"

	// Create session
	store.CreateSession(token, time.Hour, "", "", "testuser")

	// Verify exists
	if !store.ValidateSession(token) {
		t.Fatal("Session should exist before deletion")
	}

	// Delete session
	store.DeleteSession(token)

	// Verify deleted
	if store.ValidateSession(token) {
		t.Error("Session should not exist after deletion")
	}
}

func TestSessionStore_ValidateAndExtendSession(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	token := "extend-me-token"
	duration := time.Hour

	// Create session
	store.CreateSession(token, duration, "", "", "testuser")

	// Get original expiry
	store.mu.RLock()
	key := sessionHash(token)
	originalExpiry := store.sessions[key].ExpiresAt
	store.mu.RUnlock()

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Validate and extend
	if !store.ValidateAndExtendSession(token) {
		t.Error("ValidateAndExtendSession returned false for valid session")
	}

	// Verify expiry was extended
	store.mu.RLock()
	newExpiry := store.sessions[key].ExpiresAt
	store.mu.RUnlock()

	if !newExpiry.After(originalExpiry) {
		t.Error("Session expiry was not extended")
	}
}

func TestSessionStore_ValidateAndExtendSession_NonExistent(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	if store.ValidateAndExtendSession("nonexistent-token") {
		t.Error("ValidateAndExtendSession returned true for nonexistent session")
	}
}

func TestSessionStore_ValidateAndExtendSession_Expired(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	token := "expired-extend-token"
	key := sessionHash(token)

	// Add an already-expired session
	store.mu.Lock()
	store.sessions[key] = &SessionData{
		ExpiresAt:        time.Now().Add(-time.Hour),
		CreatedAt:        time.Now().Add(-2 * time.Hour),
		OriginalDuration: time.Hour,
	}
	store.mu.Unlock()

	if store.ValidateAndExtendSession(token) {
		t.Error("ValidateAndExtendSession returned true for expired session")
	}
}

func TestSessionStore_ValidateAndExtendSession_ZeroDuration(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	token := "zero-duration-token"
	key := sessionHash(token)

	// Add session with zero original duration
	store.mu.Lock()
	store.sessions[key] = &SessionData{
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedAt:        time.Now(),
		OriginalDuration: 0, // Zero duration - should not extend
	}
	store.mu.Unlock()

	originalExpiry := store.sessions[key].ExpiresAt

	// Validate and extend
	if !store.ValidateAndExtendSession(token) {
		t.Error("ValidateAndExtendSession returned false for valid session")
	}

	// Expiry should not change (zero duration means no sliding)
	store.mu.RLock()
	newExpiry := store.sessions[key].ExpiresAt
	store.mu.RUnlock()

	if !newExpiry.Equal(originalExpiry) {
		t.Error("Session with zero OriginalDuration should not have expiry extended")
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	tokens := []string{"token-1", "token-2", "token-3"}

	// Create multiple sessions
	for _, token := range tokens {
		store.CreateSession(token, time.Hour, "Agent-"+token, "", "testuser")
	}

	// All should be valid
	for _, token := range tokens {
		if !store.ValidateSession(token) {
			t.Errorf("ValidateSession(%q) = false, want true", token)
		}
	}

	// Delete middle one
	store.DeleteSession("token-2")

	// First and last should still be valid
	if !store.ValidateSession("token-1") {
		t.Error("token-1 should still be valid")
	}
	if store.ValidateSession("token-2") {
		t.Error("token-2 should be deleted")
	}
	if !store.ValidateSession("token-3") {
		t.Error("token-3 should still be valid")
	}
}

func TestSessionStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and add session
	store1 := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	token := "persistent-token"
	store1.CreateSession(token, time.Hour, "PersistAgent", "10.0.0.1", "persistuser")

	// Verify file was created
	sessionsFile := filepath.Join(tmpDir, "sessions.json")
	if _, err := os.Stat(sessionsFile); os.IsNotExist(err) {
		t.Fatal("sessions.json was not created")
	}

	// Create new store that should load from disk
	store2 := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}
	store2.load()

	// Session should be loaded
	if !store2.ValidateSession(token) {
		t.Error("Session was not persisted and loaded correctly")
	}

	// Verify metadata was preserved
	store2.mu.RLock()
	key := sessionHash(token)
	session := store2.sessions[key]
	store2.mu.RUnlock()

	if session == nil {
		t.Fatal("Session not found after load")
	}
	if session.UserAgent != "PersistAgent" {
		t.Errorf("UserAgent = %q, want PersistAgent", session.UserAgent)
	}
	if session.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", session.IP)
	}
}

func TestSessionStore_LoadExpiredSessions(t *testing.T) {
	tmpDir := t.TempDir()

	// Manually create a sessions file with an expired session
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	token := "already-expired"
	key := sessionHash(token)

	// Add expired session
	store.mu.Lock()
	store.sessions[key] = &SessionData{
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	store.mu.Unlock()
	store.save()

	// Create new store and load
	store2 := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}
	store2.load()

	// Expired session should not be loaded
	store2.mu.RLock()
	_, exists := store2.sessions[key]
	store2.mu.RUnlock()

	if exists {
		t.Error("Expired session should not be loaded from disk")
	}
}

func TestSessionStore_Cleanup(t *testing.T) {
	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: t.TempDir(),
		stopChan: make(chan bool),
	}

	// Add a valid and an expired session
	validKey := sessionHash("valid-session")
	expiredKey := sessionHash("expired-session")

	store.mu.Lock()
	store.sessions[validKey] = &SessionData{
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	store.sessions[expiredKey] = &SessionData{
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	store.mu.Unlock()

	// Run cleanup
	store.cleanup()

	// Valid session should remain
	store.mu.RLock()
	_, validExists := store.sessions[validKey]
	_, expiredExists := store.sessions[expiredKey]
	store.mu.RUnlock()

	if !validExists {
		t.Error("Valid session was incorrectly removed during cleanup")
	}
	if expiredExists {
		t.Error("Expired session was not removed during cleanup")
	}
}

func TestSessionPersistedStruct_Fields(t *testing.T) {
	now := time.Now()
	duration := 2 * time.Hour

	persisted := sessionPersisted{
		Key:              "abc123",
		ExpiresAt:        now.Add(duration),
		CreatedAt:        now,
		UserAgent:        "TestUA",
		IP:               "1.2.3.4",
		OriginalDuration: duration,
	}

	if persisted.Key != "abc123" {
		t.Errorf("Key = %q, want abc123", persisted.Key)
	}
	if persisted.UserAgent != "TestUA" {
		t.Errorf("UserAgent = %q, want TestUA", persisted.UserAgent)
	}
	if persisted.IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", persisted.IP)
	}
	if persisted.OriginalDuration != duration {
		t.Errorf("OriginalDuration = %v, want %v", persisted.OriginalDuration, duration)
	}
}

func TestSessionStore_SaveUnsafe_MkdirAllError(t *testing.T) {
	// Create a file where the directory should be - MkdirAll will fail
	tmpDir := t.TempDir()
	blockedPath := filepath.Join(tmpDir, "blocked")

	// Create a file at the path where dataPath should be
	if err := os.WriteFile(blockedPath, []byte("blocking file"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: blockedPath, // This is a file, not a directory
		stopChan: make(chan bool),
	}

	// Add a session to save
	key := sessionHash("test-token")
	store.sessions[key] = &SessionData{
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
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

func TestSessionStore_SaveUnsafe_WriteFileError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test running as root")
	}
	// Create a read-only directory to prevent file creation
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")

	if err := os.Mkdir(readOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: readOnlyDir,
		stopChan: make(chan bool),
	}

	// Add a session
	key := sessionHash("test-token")
	store.sessions[key] = &SessionData{
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
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
	sessionsFile := filepath.Join(readOnlyDir, "sessions.json")
	if _, err := os.Stat(sessionsFile); err == nil {
		t.Error("sessions file should not exist after write failure")
	}
}

func TestSessionStore_SaveUnsafe_RenameError(t *testing.T) {
	// Create a directory at the target path to cause rename error
	tmpDir := t.TempDir()

	// Create a directory at the exact path where sessions.json should go
	sessionsFilePath := filepath.Join(tmpDir, "sessions.json")
	if err := os.Mkdir(sessionsFilePath, 0755); err != nil {
		t.Fatalf("failed to create blocking directory: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	// Add a session
	key := sessionHash("test-token")
	store.sessions[key] = &SessionData{
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	// saveUnsafe should handle rename error gracefully
	store.saveUnsafe()

	// The blocking directory should still exist (rename failed)
	info, err := os.Stat(sessionsFilePath)
	if err != nil {
		t.Fatalf("blocking directory was removed: %v", err)
	}
	if !info.IsDir() {
		t.Error("blocking directory was replaced with a file")
	}
}

func TestSessionStore_Load_ReadError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory where the sessions file should be - reading it will fail
	sessionsPath := filepath.Join(tmpDir, "sessions.json")
	if err := os.Mkdir(sessionsPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	// Should not panic and should log error
	store.load()

	if len(store.sessions) != 0 {
		t.Errorf("store should be empty after read error, got %d sessions", len(store.sessions))
	}
}

func TestSessionStore_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid JSON that won't match either format
	sessionsFile := filepath.Join(tmpDir, "sessions.json")
	if err := os.WriteFile(sessionsFile, []byte("not valid json at all"), 0600); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	// Should not panic
	store.load()

	if len(store.sessions) != 0 {
		t.Errorf("store should be empty after loading invalid JSON, got %d sessions", len(store.sessions))
	}
}

func TestSessionStore_Load_LegacyFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create legacy format JSON (map[token]sessionData) with snake_case fields
	// The legacy format uses raw tokens as keys
	legacyJSON := `{
		"raw-token-1": {"expires_at": "2099-12-31T23:59:59Z", "created_at": "2024-01-01T00:00:00Z", "user_agent": "Agent1", "ip": "1.1.1.1"},
		"raw-token-2": {"expires_at": "2099-12-31T23:59:59Z", "created_at": "2024-01-01T00:00:00Z", "user_agent": "Agent2", "ip": "2.2.2.2"},
		"raw-token-expired": {"expires_at": "2020-01-01T00:00:00Z", "created_at": "2019-01-01T00:00:00Z", "user_agent": "ExpiredAgent", "ip": "3.3.3.3"}
	}`
	sessionsFile := filepath.Join(tmpDir, "sessions.json")
	if err := os.WriteFile(sessionsFile, []byte(legacyJSON), 0600); err != nil {
		t.Fatalf("failed to write legacy format JSON: %v", err)
	}

	store := &SessionStore{
		sessions: make(map[string]*SessionData),
		dataPath: tmpDir,
		stopChan: make(chan bool),
	}

	store.load()

	// Should load the two non-expired sessions (raw-token-1 and raw-token-2)
	// The expired one (raw-token-expired) should be skipped
	if len(store.sessions) != 2 {
		t.Errorf("expected 2 sessions from legacy format, got %d", len(store.sessions))
	}

	// Verify sessions are hashed and can be validated
	// The legacy format stores raw tokens as keys, which get hashed during migration
	if !store.ValidateSession("raw-token-1") {
		t.Error("should validate session for raw-token-1 after legacy migration")
	}
	if !store.ValidateSession("raw-token-2") {
		t.Error("should validate session for raw-token-2 after legacy migration")
	}
	// Expired token should not be valid
	if store.ValidateSession("raw-token-expired") {
		t.Error("expired session should not be loaded from legacy format")
	}
}
