package license

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersistence(t *testing.T) {
	// Create a temporary directory for config
	tmpDir, err := os.MkdirTemp("", "pulse-license-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	testLicenseKey := "test-license-key-123"

	t.Run("Save and Load", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		if !p.Exists() {
			t.Error("License file should exist")
		}

		loadedKey, err := p.Load()
		if err != nil {
			t.Fatalf("Failed to load license: %v", err)
		}

		if loadedKey != testLicenseKey {
			t.Errorf("Expected license key %s, got %s", testLicenseKey, loadedKey)
		}
	})

	t.Run("Save and Load with Grace Period", func(t *testing.T) {
		gracePeriodEnd := time.Now().Add(7 * 24 * time.Hour).Unix()
		err := p.SaveWithGracePeriod(testLicenseKey, &gracePeriodEnd)
		if err != nil {
			t.Fatalf("Failed to save license with grace period: %v", err)
		}

		persisted, err := p.LoadWithMetadata()
		if err != nil {
			t.Fatalf("Failed to load license with metadata: %v", err)
		}

		if persisted.LicenseKey != testLicenseKey {
			t.Errorf("Expected license key %s, got %s", testLicenseKey, persisted.LicenseKey)
		}

		if persisted.GracePeriodEnd == nil || *persisted.GracePeriodEnd != gracePeriodEnd {
			t.Errorf("Expected grace period end %v, got %v", gracePeriodEnd, persisted.GracePeriodEnd)
		}
	})

	t.Run("Load non-existent", func(t *testing.T) {
		tmpDirEmpty, _ := os.MkdirTemp("", "pulse-license-test-empty-*")
		defer os.RemoveAll(tmpDirEmpty)

		pEmpty, _ := NewPersistence(tmpDirEmpty)
		key, err := pEmpty.Load()
		if err != nil {
			t.Fatalf("Expected no error for non-existent license, got %v", err)
		}
		if key != "" {
			t.Errorf("Expected empty key, got %s", key)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		if !p.Exists() {
			t.Fatal("License should exist before delete")
		}

		err = p.Delete()
		if err != nil {
			t.Fatalf("Failed to delete license: %v", err)
		}

		if p.Exists() {
			t.Error("License should not exist after delete")
		}
	})

	t.Run("Encryption check", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		// Read the file directly - it should be base64 encoded and encrypted
		licensePath := filepath.Join(tmpDir, LicenseFileName)
		data, err := os.ReadFile(licensePath)
		if err != nil {
			t.Fatalf("Failed to read license file: %v", err)
		}

		if string(data) == testLicenseKey {
			t.Error("License file should be encrypted, not raw text")
		}

		// Ensure it's not JSON either in raw form
		if data[0] == '{' {
			t.Error("License file should be encrypted, not raw JSON")
		}
	})

	t.Run("Decrypt with wrong key material", func(t *testing.T) {
		err := p.Save(testLicenseKey)
		if err != nil {
			t.Fatalf("Failed to save license: %v", err)
		}

		// Create a new persistence with different encryption key
		pWrong := &Persistence{
			configDir:     tmpDir,
			encryptionKey: "different-encryption-key",
			machineID:     "different-machine-id",
		}

		_, err = pWrong.Load()
		if err == nil {
			t.Error("Expected error when decrypting with wrong key material")
		}
	})

	t.Run("Backwards compatibility with machine-id", func(t *testing.T) {
		// This tests the scenario where a license was saved with machine-id
		// (old behavior before persistent key feature) and we're now loading it
		// with a different primary key but same machine-id as fallback

		tmpDirCompat, _ := os.MkdirTemp("", "pulse-license-compat-*")
		defer os.RemoveAll(tmpDirCompat)

		testKey := "compat-test-key"
		machineID := "test-machine-id-12345"

		// Simulate old behavior: directly encrypt with machine-id (without calling Save
		// which would create a persistent key). This is what old installations have.
		pOld := &Persistence{
			configDir:     tmpDirCompat,
			encryptionKey: machineID, // Old behavior used machine-id as encryption key
			machineID:     machineID,
		}

		// Manually encrypt and save without creating persistent key
		persisted := PersistedLicense{LicenseKey: testKey}
		jsonData, _ := json.Marshal(persisted)
		encrypted, err := pOld.encrypt(jsonData)
		if err != nil {
			t.Fatalf("Failed to encrypt license: %v", err)
		}

		// Write directly to simulate old installation
		os.MkdirAll(tmpDirCompat, 0700)
		licensePath := filepath.Join(tmpDirCompat, LicenseFileName)
		encoded := base64.StdEncoding.EncodeToString(encrypted)
		os.WriteFile(licensePath, []byte(encoded), 0600)

		// Verify no persistent key file exists (simulating old installation)
		keyPath := filepath.Join(tmpDirCompat, PersistentKeyFileName)
		if _, err := os.Stat(keyPath); err == nil {
			t.Fatal("Persistent key should not exist for this test")
		}

		// Now try to load with a new primary key but same machine-id as fallback
		// This simulates what happens when a Docker user upgrades and gets a
		// persistent key file, but their old license was encrypted with machine-id
		pNew := &Persistence{
			configDir:     tmpDirCompat,
			encryptionKey: "new-persistent-key", // Different primary key (simulating new container)
			machineID:     machineID,            // Same machine-id for fallback
		}

		loaded, err := pNew.LoadWithMetadata()
		if err != nil {
			t.Fatalf("Failed to load license with machine-id fallback: %v\n"+
				"This means backwards compatibility is broken for existing Docker users", err)
		}

		if loaded.LicenseKey != testKey {
			t.Errorf("Expected license key %s, got %s", testKey, loaded.LicenseKey)
		}
	})
}

func TestPersistenceEnforcesOwnerOnlyPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.Chmod(tmpDir, 0755); err != nil {
		t.Fatalf("failed to relax temp dir perms: %v", err)
	}

	p, err := NewPersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}
	if err := p.Save("owner-only-perm-test"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("failed to stat config dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("config dir perms = %o, want 700", got)
	}

	for _, file := range []string{LicenseFileName, PersistentKeyFileName} {
		path := filepath.Join(tmpDir, file)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", file, err)
		}
		if got := info.Mode().Perm(); got != 0600 {
			t.Fatalf("%s perms = %o, want 600", file, got)
		}
	}
}

func TestNewPersistenceRejectsSymlinkPersistentKey(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "key-target")
	if err := os.WriteFile(target, []byte("test-key"), 0600); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	keyPath := filepath.Join(tmpDir, PersistentKeyFileName)
	if err := os.Symlink(target, keyPath); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	_, err := NewPersistence(tmpDir)
	if err == nil {
		t.Fatal("expected error for symlink persistent key path")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got: %v", err)
	}
}
