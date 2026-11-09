package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Both read and explicit persistence must refuse a corrupt key without
// changing saved setup access. Restoration here is deliberate test setup,
// never a recovery action inferred or performed by the product.
func TestBootstrapCorruptKeyPreservesOriginalSetupAccess(t *testing.T) {
	for _, legacyPlaintext := range []bool{false, true} {
		name := "encrypted"
		if legacyPlaintext {
			name = "legacy_plaintext"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), "missing-key"))
			const token = "synthetic-original-setup-token"
			path, err := Persist(dir, token, time.Unix(1700000000, 0))
			if err != nil {
				t.Fatal(err)
			}
			keyPath := filepath.Join(dir, ".encryption.key")
			originalKey, err := os.ReadFile(keyPath)
			if err != nil {
				t.Fatal(err)
			}
			if legacyPlaintext {
				if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			originalState, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			invalidKey := []byte("truncated-synthetic-key")
			if err := os.WriteFile(keyPath, invalidKey, 0600); err != nil {
				t.Fatal(err)
			}
			keyInfo, err := os.Stat(keyPath)
			if err != nil {
				t.Fatal(err)
			}
			generated := 0
			generator := func() (string, error) { generated++; return "unrequested-replacement-token", nil }
			got, created, gotPath, migrated, err := LoadOrCreate(dir, generator)
			if err == nil || !strings.Contains(err.Error(), "refusing to replace") || got != "" || created || migrated || gotPath != path {
				t.Errorf("corrupt key must refuse setup load without replacement/migration: token_returned=%t created=%t migrated=%t error=%v", got != "", created, migrated, err)
			}
			if generated != 0 {
				t.Error("read failure generated a new setup token")
			}
			if gotPath, err := Persist(dir, "unrequested-replacement-token", time.Now()); err == nil || gotPath != path {
				t.Errorf("explicit token persistence bypassed the corrupt key: error=%v", err)
			}
			if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, invalidKey) {
				t.Errorf("setup access overwrote the invalid key: %v", err)
			}
			if got, err := os.Stat(keyPath); err != nil || !os.SameFile(keyInfo, got) || got.Mode() != keyInfo.Mode() {
				t.Errorf("setup access replaced invalid key inode/mode: %v", err)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, originalState) {
				t.Errorf("setup access overwrote saved token state: %v", err)
			}
			assertNoBootstrapTemporaryFiles(t, dir)

			if err := os.WriteFile(keyPath, originalKey, 0600); err != nil {
				t.Fatal(err)
			}
			got, created, gotPath, migrated, err = LoadOrCreate(dir, generator)
			if err != nil || got != token || created || migrated != legacyPlaintext || gotPath != path || generated != 0 {
				t.Errorf("restoring the original key did not recover the original setup access: created=%t migrated=%t error=%v", created, migrated, err)
			}
			if got, err := os.ReadFile(keyPath); err != nil || !bytes.Equal(got, originalKey) {
				t.Errorf("recovered read changed original key: %v", err)
			}
			if !legacyPlaintext {
				if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, originalState) {
					t.Errorf("recovered read changed encrypted token bytes: %v", err)
				}
			}
		})
	}
}
