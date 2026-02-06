package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigPersistenceFailsWhenEncryptedDataPresentWithoutKey(t *testing.T) {
	dir := t.TempDir()

	// Ensure the crypto migration path can't pick up a real on-disk legacy key.
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))

	// Simulate existing encrypted data without providing the encryption key.
	if err := os.WriteFile(filepath.Join(dir, "nodes.enc"), []byte("ciphertext"), 0600); err != nil {
		t.Fatalf("failed to write simulated encrypted file: %v", err)
	}

	if _, err := newConfigPersistence(dir); err == nil {
		t.Fatalf("expected error when initializing persistence without encryption key")
	}
}
