package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigPersistenceFailsWhenEncryptedDataPresentWithoutKey(t *testing.T) {
	dir := t.TempDir()

	// The crypto package tries to migrate keys from /etc/pulse/.encryption.key
	// We need to temporarily rename it if it exists to properly test this scenario
	systemKeyPath := "/etc/pulse/.encryption.key"
	backupKeyPath := "/etc/pulse/.encryption.key.test-backup"

	if _, err := os.Stat(systemKeyPath); err == nil {
		// Key exists - temporarily rename it
		if err := os.Rename(systemKeyPath, backupKeyPath); err != nil {
			t.Skipf("cannot rename system encryption key for test isolation: %v", err)
		}
		t.Cleanup(func() {
			// Restore the key after test
			os.Rename(backupKeyPath, systemKeyPath)
		})
	}

	// Simulate existing encrypted data without providing the encryption key.
	if err := os.WriteFile(filepath.Join(dir, "nodes.enc"), []byte("ciphertext"), 0600); err != nil {
		t.Fatalf("failed to write simulated encrypted file: %v", err)
	}

	if _, err := newConfigPersistence(dir); err == nil {
		t.Fatalf("expected error when initializing persistence without encryption key")
	}
}
