package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateOIDCSession(t *testing.T) {
	// Create a temporary directory for test sessions
	tmpDir, err := os.MkdirTemp("", "pulse-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewSessionStore(tmpDir)

	// Create an OIDC session with token info
	token := "test-session-token-123"
	oidcInfo := &OIDCTokenInfo{
		RefreshToken:   "test-refresh-token",
		AccessTokenExp: time.Now().Add(1 * time.Hour),
		Issuer:         "https://example.com",
		ClientID:       "test-client-id",
	}

	store.CreateOIDCSession(token, 24*time.Hour, "TestAgent", "127.0.0.1", oidcInfo)

	// Verify session was created
	if !store.ValidateSession(token) {
		t.Error("Created OIDC session should be valid")
	}

	// Verify OIDC token info was stored
	session := store.GetSession(token)
	if session == nil {
		t.Fatal("GetSession returned nil for valid session")
	}

	if session.OIDCRefreshToken != oidcInfo.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %q, want %q", session.OIDCRefreshToken, oidcInfo.RefreshToken)
	}

	if session.OIDCIssuer != oidcInfo.Issuer {
		t.Errorf("Issuer mismatch: got %q, want %q", session.OIDCIssuer, oidcInfo.Issuer)
	}

	if session.OIDCClientID != oidcInfo.ClientID {
		t.Errorf("ClientID mismatch: got %q, want %q", session.OIDCClientID, oidcInfo.ClientID)
	}

	if session.OIDCAccessTokenExp.IsZero() {
		t.Error("AccessTokenExp should not be zero")
	}
}

func TestUpdateOIDCTokens(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewSessionStore(tmpDir)

	token := "test-session-token-456"
	originalExpiry := time.Now().Add(1 * time.Hour)
	oidcInfo := &OIDCTokenInfo{
		RefreshToken:   "original-refresh-token",
		AccessTokenExp: originalExpiry,
		Issuer:         "https://example.com",
		ClientID:       "test-client-id",
	}

	store.CreateOIDCSession(token, 24*time.Hour, "TestAgent", "127.0.0.1", oidcInfo)

	// Update the tokens (simulating a refresh)
	newExpiry := time.Now().Add(2 * time.Hour)
	store.UpdateOIDCTokens(token, "new-refresh-token", newExpiry)

	// Verify tokens were updated
	session := store.GetSession(token)
	if session == nil {
		t.Fatal("GetSession returned nil for valid session")
	}

	if session.OIDCRefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken should be updated: got %q, want %q", session.OIDCRefreshToken, "new-refresh-token")
	}

	if !session.OIDCAccessTokenExp.After(originalExpiry) {
		t.Error("AccessTokenExp should be updated to new expiry")
	}
}

func TestOIDCSessionPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	token := "test-session-token-persist"
	oidcInfo := &OIDCTokenInfo{
		RefreshToken:   "persist-refresh-token",
		AccessTokenExp: time.Now().Add(1 * time.Hour),
		Issuer:         "https://example.com",
		ClientID:       "test-client-id",
	}

	// Create session in first store instance
	store1 := NewSessionStore(tmpDir)
	store1.CreateOIDCSession(token, 24*time.Hour, "TestAgent", "127.0.0.1", oidcInfo)

	// Verify file was written
	sessionFile := filepath.Join(tmpDir, "sessions.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Fatal("Session file should exist after creating session")
	}

	// Create new store instance (simulates restart)
	store2 := NewSessionStore(tmpDir)

	// Verify session was loaded with OIDC info
	session := store2.GetSession(token)
	if session == nil {
		t.Fatal("Session should be restored after reload")
	}

	if session.OIDCRefreshToken != oidcInfo.RefreshToken {
		t.Errorf("RefreshToken should persist: got %q, want %q", session.OIDCRefreshToken, oidcInfo.RefreshToken)
	}

	if session.OIDCIssuer != oidcInfo.Issuer {
		t.Errorf("Issuer should persist: got %q, want %q", session.OIDCIssuer, oidcInfo.Issuer)
	}
}

func TestCreateOIDCSession_NilTokenInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewSessionStore(tmpDir)

	// Create OIDC session with nil token info (no refresh token available)
	token := "test-session-no-refresh"
	store.CreateOIDCSession(token, 24*time.Hour, "TestAgent", "127.0.0.1", nil)

	// Verify session was created
	if !store.ValidateSession(token) {
		t.Error("Session should be valid even without OIDC tokens")
	}

	session := store.GetSession(token)
	if session == nil {
		t.Fatal("GetSession returned nil")
	}

	if session.OIDCRefreshToken != "" {
		t.Errorf("RefreshToken should be empty when nil info passed: got %q", session.OIDCRefreshToken)
	}
}

func TestInvalidateSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewSessionStore(tmpDir)

	token := "test-session-invalidate"
	oidcInfo := &OIDCTokenInfo{
		RefreshToken:   "refresh-token",
		AccessTokenExp: time.Now().Add(1 * time.Hour),
		Issuer:         "https://example.com",
		ClientID:       "test-client-id",
	}

	store.CreateOIDCSession(token, 24*time.Hour, "TestAgent", "127.0.0.1", oidcInfo)

	// Verify session exists
	if !store.ValidateSession(token) {
		t.Error("Session should be valid before invalidation")
	}

	// Invalidate the session
	store.InvalidateSession(token)

	// Verify session no longer exists
	if store.ValidateSession(token) {
		t.Error("Session should be invalid after invalidation")
	}
}
