package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func invalidSavedKeyFixtures() map[string][]byte {
	return map[string][]byte{
		"empty":            {},
		"whitespace":       []byte(" \n\t"),
		"invalid_base64":   []byte("not-base64"),
		"truncated_base64": []byte("AQIDBA"),
		"short_key":        []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))),
		"long_key":         []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 48))),
	}
}

// Corruption is not a first-run request or permission to choose a different
// key. This includes state whose encrypted bytes are in a JSON-named file,
// outside the older .enc inventory guard.
func TestInvalidSavedPrimaryKeyPreservesAuthority(t *testing.T) {
	for name, invalid := range invalidSavedKeyFixtures() {
		for _, legacyPresent := range []bool{false, true} {
			legacyName := "without_legacy"
			if legacyPresent {
				legacyName = "with_different_valid_legacy"
			}
			t.Run(name+"/"+legacyName, func(t *testing.T) {
				root := t.TempDir()
				dataDir := filepath.Join(root, "data")
				if err := os.Mkdir(dataDir, 0700); err != nil {
					t.Fatal(err)
				}
				keyPath := filepath.Join(dataDir, encryptionKeyFileName)
				if err := os.WriteFile(keyPath, invalid, 0400); err != nil {
					t.Fatal(err)
				}
				keyInfo, err := os.Stat(keyPath)
				if err != nil {
					t.Fatal(err)
				}
				legacyPath := filepath.Join(root, "legacy-key")
				t.Setenv("PULSE_LEGACY_KEY_PATH", legacyPath)
				legacy := []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{91}, encryptionKeyLength)))
				if legacyPresent {
					if err := os.WriteFile(legacyPath, legacy, 0600); err != nil {
						t.Fatal(err)
					}
				}
				statePath := filepath.Join(dataDir, "saved-state.json")
				state := []byte("opaque saved ciphertext; never evidence of a clean install")
				if err := os.WriteFile(statePath, state, 0600); err != nil {
					t.Fatal(err)
				}

				manager, err := NewCryptoManagerAt(dataDir)
				if manager != nil || !errors.Is(err, errInvalidKeyMaterial) {
					t.Errorf("invalid existing primary must refuse initialization with its cause: manager=%t error=%v", manager != nil, err)
				}
				assertSavedKeyFileUnchanged(t, keyPath, invalid, keyInfo)
				if got, err := os.ReadFile(statePath); err != nil || !bytes.Equal(got, state) {
					t.Errorf("failed initialization changed saved state: error=%v", err)
				}
				if legacyPresent {
					if got, err := os.ReadFile(legacyPath); err != nil || !bytes.Equal(got, legacy) {
						t.Errorf("failed initialization changed legacy key: error=%v", err)
					}
				} else if _, err := os.Stat(legacyPath); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("failed initialization created an absent legacy key: error=%v", err)
				}
				entries, err := os.ReadDir(dataDir)
				if err != nil || len(entries) != 2 {
					t.Errorf("failed initialization changed the data inventory: error=%v", err)
				}
			})
		}
	}
}

func TestInvalidSavedLegacyKeyIsNotNewInstall(t *testing.T) {
	for name, invalid := range invalidSavedKeyFixtures() {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			legacyPath := filepath.Join(root, "legacy-key")
			if err := os.WriteFile(legacyPath, invalid, 0400); err != nil {
				t.Fatal(err)
			}
			legacyInfo, err := os.Stat(legacyPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PULSE_LEGACY_KEY_PATH", legacyPath)
			dataDir := filepath.Join(root, "not-yet-created")
			manager, err := NewCryptoManagerAt(dataDir)
			if manager != nil || !errors.Is(err, errInvalidKeyMaterial) {
				t.Errorf("invalid existing legacy must refuse initialization with its cause: manager=%t error=%v", manager != nil, err)
			}
			assertSavedKeyFileUnchanged(t, legacyPath, invalid, legacyInfo)
			if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("invalid legacy key must not create a new data directory/key: error=%v", err)
			}
		})
	}
}

func TestValidPrimaryKeyDoesNotConsultInvalidLegacy(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, encryptionKeyFileName)
	valid := []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{23}, encryptionKeyLength)))
	if err := os.WriteFile(keyPath, valid, 0600); err != nil {
		t.Fatal(err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy-key")
	invalid := []byte("invalid legacy is not the primary authority")
	if err := os.WriteFile(legacyPath, invalid, 0400); err != nil {
		t.Fatal(err)
	}
	legacyInfo, err := os.Stat(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_LEGACY_KEY_PATH", legacyPath)
	manager, err := NewCryptoManagerAt(dir)
	if err != nil || manager == nil {
		t.Fatalf("valid primary was not reused: %v", err)
	}
	if !bytes.Equal(manager.key, bytes.Repeat([]byte{23}, encryptionKeyLength)) {
		t.Error("valid primary authority changed")
	}
	assertSavedKeyFileUnchanged(t, keyPath, valid, keyInfo)
	assertSavedKeyFileUnchanged(t, legacyPath, invalid, legacyInfo)
}

func assertSavedKeyFileUnchanged(t *testing.T, path string, want []byte, before os.FileInfo) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("initialization replaced saved key bytes: error=%v", err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Errorf("initialization removed saved key: %v", err)
		return
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Error("initialization replaced the key inode or permissions")
	}
}

func TestUnreadableLegacyKeyCannotStartNewAuthority(t *testing.T) {
	root := t.TempDir()
	legacyPath := filepath.Join(root, "legacy-key")
	valid := []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{59}, encryptionKeyLength)))
	if err := os.WriteFile(legacyPath, valid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(legacyPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(legacyPath, 0600) })
	if _, err := os.ReadFile(legacyPath); err == nil {
		t.Skip("test execution identity can read mode-0000 files")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Fatal(err)
	}
	t.Setenv("PULSE_LEGACY_KEY_PATH", legacyPath)
	dataDir := filepath.Join(root, "not-yet-created")
	manager, err := NewCryptoManagerAt(dataDir)
	if manager != nil || !errors.Is(err, os.ErrPermission) {
		t.Errorf("unreadable legacy key must stop initialization with its read cause: manager=%t error=%v", manager != nil, err)
	}
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("unreadable legacy key caused new directory/key creation: %v", err)
	}
	if err := os.Chmod(legacyPath, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(legacyPath); err != nil || !bytes.Equal(got, valid) {
		t.Errorf("failed read changed legacy key: %v", err)
	}
}
