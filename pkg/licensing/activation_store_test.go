package licensing

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestActivationStatePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-activation-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("create persistence: %v", err)
	}

	state := &ActivationState{
		InstallationID:      "inst_abc",
		InstallationToken:   "pit_live_secret",
		LicenseID:           "lic_123",
		GrantJWT:            "header.payload.signature",
		GrantJTI:            "grant_xyz",
		GrantExpiresAt:      1700000000,
		InstanceFingerprint: "fp-uuid-1234",
		LicenseServerURL:    "https://license.example.com",
		ActivatedAt:         1699000000,
		LastRefreshedAt:     1699500000,
	}

	t.Run("save and load round-trip", func(t *testing.T) {
		if err := p.SaveActivationState(state); err != nil {
			t.Fatalf("SaveActivationState: %v", err)
		}

		loaded, err := p.LoadActivationState()
		if err != nil {
			t.Fatalf("LoadActivationState: %v", err)
		}
		if loaded == nil {
			t.Fatal("expected non-nil state")
		}

		if loaded.InstallationID != state.InstallationID {
			t.Errorf("InstallationID = %q, want %q", loaded.InstallationID, state.InstallationID)
		}
		if loaded.InstallationToken != state.InstallationToken {
			t.Errorf("InstallationToken = %q, want %q", loaded.InstallationToken, state.InstallationToken)
		}
		if loaded.LicenseID != state.LicenseID {
			t.Errorf("LicenseID = %q, want %q", loaded.LicenseID, state.LicenseID)
		}
		if loaded.GrantJWT != state.GrantJWT {
			t.Errorf("GrantJWT = %q, want %q", loaded.GrantJWT, state.GrantJWT)
		}
		if loaded.GrantJTI != state.GrantJTI {
			t.Errorf("GrantJTI = %q, want %q", loaded.GrantJTI, state.GrantJTI)
		}
		if loaded.GrantExpiresAt != state.GrantExpiresAt {
			t.Errorf("GrantExpiresAt = %d, want %d", loaded.GrantExpiresAt, state.GrantExpiresAt)
		}
		if loaded.InstanceFingerprint != state.InstanceFingerprint {
			t.Errorf("InstanceFingerprint = %q, want %q", loaded.InstanceFingerprint, state.InstanceFingerprint)
		}
		if loaded.LicenseServerURL != state.LicenseServerURL {
			t.Errorf("LicenseServerURL = %q, want %q", loaded.LicenseServerURL, state.LicenseServerURL)
		}
		if loaded.ActivatedAt != state.ActivatedAt {
			t.Errorf("ActivatedAt = %d, want %d", loaded.ActivatedAt, state.ActivatedAt)
		}
		if loaded.LastRefreshedAt != state.LastRefreshedAt {
			t.Errorf("LastRefreshedAt = %d, want %d", loaded.LastRefreshedAt, state.LastRefreshedAt)
		}
	})

	t.Run("file is encrypted on disk", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, ActivationStateFileName)
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		// The file should be base64-encoded encrypted data, not raw JSON.
		if data[0] == '{' {
			t.Error("activation state file should be encrypted, not raw JSON")
		}
		// The secret token should not appear in plaintext.
		if searchString(string(data), "pit_live_secret") {
			t.Error("installation token should not appear in plaintext on disk")
		}
	})

	t.Run("ActivationStateExists", func(t *testing.T) {
		if !p.ActivationStateExists() {
			t.Error("expected ActivationStateExists=true after save")
		}
	})

	t.Run("file permissions", func(t *testing.T) {
		statePath := filepath.Join(tmpDir, ActivationStateFileName)
		info, err := os.Stat(statePath)
		if err != nil {
			t.Fatalf("stat file: %v", err)
		}
		if got := info.Mode().Perm(); got != 0600 {
			t.Errorf("file perms = %o, want 600", got)
		}
	})

	t.Run("clear", func(t *testing.T) {
		if err := p.ClearActivationState(); err != nil {
			t.Fatalf("ClearActivationState: %v", err)
		}
		if p.ActivationStateExists() {
			t.Error("expected ActivationStateExists=false after clear")
		}

		loaded, err := p.LoadActivationState()
		if err != nil {
			t.Fatalf("LoadActivationState after clear: %v", err)
		}
		if loaded != nil {
			t.Error("expected nil state after clear")
		}
	})

	t.Run("clear non-existent is no-op", func(t *testing.T) {
		if err := p.ClearActivationState(); err != nil {
			t.Fatalf("ClearActivationState on non-existent: %v", err)
		}
	})

	t.Run("save nil state errors", func(t *testing.T) {
		if err := p.SaveActivationState(nil); err == nil {
			t.Error("expected error when saving nil state")
		}
	})

	t.Run("load from empty dir", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "pulse-activation-empty-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		pEmpty, err := NewPersistence(emptyDir)
		if err != nil {
			t.Fatalf("create persistence: %v", err)
		}

		loaded, err := pEmpty.LoadActivationState()
		if err != nil {
			t.Fatalf("LoadActivationState from empty: %v", err)
		}
		if loaded != nil {
			t.Error("expected nil state from empty dir")
		}
	})

	t.Run("ActivationStateExists rejects symlinks", func(t *testing.T) {
		linkDir, err := os.MkdirTemp("", "pulse-activation-symlink-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}
		defer os.RemoveAll(linkDir)

		// Create a real file and symlink to it with the activation state name.
		target := filepath.Join(linkDir, "real-file")
		if err := os.WriteFile(target, []byte("test"), 0600); err != nil {
			t.Fatalf("write target: %v", err)
		}
		link := filepath.Join(linkDir, ActivationStateFileName)
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}

		pLink, err := NewPersistence(linkDir)
		if err != nil {
			t.Fatalf("create persistence: %v", err)
		}
		if pLink.ActivationStateExists() {
			t.Error("ActivationStateExists should reject symlinks")
		}
	})

	t.Run("decrypt with wrong key fails", func(t *testing.T) {
		// Save with the real persistence
		if err := p.SaveActivationState(state); err != nil {
			t.Fatalf("save: %v", err)
		}

		pWrong := &Persistence{
			configDir:     tmpDir,
			encryptionKey: "wrong-key",
			machineID:     "wrong-machine",
		}

		_, err := pWrong.LoadActivationState()
		if err == nil {
			t.Error("expected error when decrypting with wrong key")
		}
	})

	t.Run("plaintext activation file rewrites encrypted storage on load", func(t *testing.T) {
		plainDir, err := os.MkdirTemp("", "pulse-activation-plaintext-*")
		if err != nil {
			t.Fatalf("create temp dir: %v", err)
		}
		defer os.RemoveAll(plainDir)

		pPlain, err := NewPersistence(plainDir)
		if err != nil {
			t.Fatalf("create persistence: %v", err)
		}

		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("marshal plaintext state: %v", err)
		}

		statePath := filepath.Join(plainDir, ActivationStateFileName)
		if err := os.WriteFile(statePath, raw, 0600); err != nil {
			t.Fatalf("write plaintext activation state: %v", err)
		}

		loaded, err := pPlain.LoadActivationState()
		if err != nil {
			t.Fatalf("LoadActivationState plaintext: %v", err)
		}
		if loaded == nil {
			t.Fatal("expected non-nil state from plaintext migration")
		}
		if loaded.InstallationToken != state.InstallationToken {
			t.Fatalf("InstallationToken = %q, want %q", loaded.InstallationToken, state.InstallationToken)
		}
		if loaded.GrantJWT != state.GrantJWT {
			t.Fatalf("GrantJWT = %q, want %q", loaded.GrantJWT, state.GrantJWT)
		}

		rewritten, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("read rewritten activation state: %v", err)
		}
		if bytes.Equal(rewritten, raw) {
			t.Fatal("expected plaintext activation state file to be rewritten encrypted")
		}
		if searchString(string(rewritten), "pit_live_secret") {
			t.Fatal("installation token should not remain in plaintext after migration rewrite")
		}
	})
}
