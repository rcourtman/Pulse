package crypto

import (
	"bytes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type errReader struct {
	err error
}

func (e errReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func withDefaultDataDir(t *testing.T, dir string) {
	t.Helper()
	orig := defaultDataDirFn
	defaultDataDirFn = func() string { return dir }
	t.Cleanup(func() { defaultDataDirFn = orig })
}

func withLegacyKeyPath(t *testing.T, path string) {
	t.Helper()
	orig := legacyKeyPath
	legacyKeyPath = path
	t.Cleanup(func() { legacyKeyPath = orig })
}

func withRandReader(t *testing.T, r io.Reader) {
	t.Helper()
	orig := randReader
	randReader = r
	t.Cleanup(func() { randReader = orig })
}

func withNewGCM(t *testing.T, fn func(cipher.Block) (cipher.AEAD, error)) {
	t.Helper()
	orig := newGCM
	newGCM = fn
	t.Cleanup(func() { newGCM = orig })
}

func TestDeriveKeyValidation(t *testing.T) {
	tests := []struct {
		name    string
		cm      *CryptoManager
		purpose string
		length  int
	}{
		{
			name:    "nil manager",
			cm:      nil,
			purpose: "storage",
			length:  32,
		},
		{
			name:    "empty manager key",
			cm:      &CryptoManager{},
			purpose: "storage",
			length:  32,
		},
		{
			name:    "zero length",
			cm:      &CryptoManager{key: make([]byte, 32)},
			purpose: "storage",
			length:  0,
		},
		{
			name:    "negative length",
			cm:      &CryptoManager{key: make([]byte, 32)},
			purpose: "storage",
			length:  -1,
		},
		{
			name:    "empty purpose",
			cm:      &CryptoManager{key: make([]byte, 32)},
			purpose: "",
			length:  32,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cm.DeriveKey(tc.purpose, tc.length)
			if err == nil {
				t.Fatal("DeriveKey() expected error")
			}
		})
	}
}

func TestDeriveKeyDeterministicAndPurposeScoped(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 1)
	}

	cm := &CryptoManager{key: masterKey}

	first, err := cm.DeriveKey("storage", 32)
	if err != nil {
		t.Fatalf("DeriveKey() first call error: %v", err)
	}
	second, err := cm.DeriveKey("storage", 32)
	if err != nil {
		t.Fatalf("DeriveKey() second call error: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("DeriveKey() should be deterministic for same purpose/length")
	}
	if bytes.Equal(first, masterKey) {
		t.Fatal("DeriveKey() should not return the raw master key bytes")
	}

	otherPurpose, err := cm.DeriveKey("session", 32)
	if err != nil {
		t.Fatalf("DeriveKey() other purpose error: %v", err)
	}
	if bytes.Equal(first, otherPurpose) {
		t.Fatal("DeriveKey() should produce distinct keys for different purposes")
	}

	short, err := cm.DeriveKey("storage", 16)
	if err != nil {
		t.Fatalf("DeriveKey() short length error: %v", err)
	}
	if len(short) != 16 {
		t.Fatalf("DeriveKey() short length = %d, want 16", len(short))
	}
	if !bytes.Equal(first[:16], short) {
		t.Fatal("DeriveKey() output stream prefix mismatch for shorter length")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := t.TempDir()

	// Create the crypto manager
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello")},
		{"medium", []byte("this is a medium length test string for encryption")},
		{"with nulls", []byte("test\x00with\x00null\x00bytes")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}},
		{"unicode", []byte("こんにちは世界")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := cm.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error: %v", err)
			}

			// Encrypted data should be different from plaintext (unless empty)
			if len(tc.plaintext) > 0 && bytes.Equal(encrypted, tc.plaintext) {
				t.Error("Encrypt() returned plaintext unchanged")
			}

			// Decrypt should return original
			decrypted, err := cm.Decrypt(encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error: %v", err)
			}

			if !bytes.Equal(decrypted, tc.plaintext) {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncryptDecryptString(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	testStrings := []string{
		"",
		"hello world",
		"password123!@#",
		"unicode: 日本語 中文 한국어",
		"special chars: \n\t\r\\\"'",
	}

	for _, s := range testStrings {
		t.Run(s, func(t *testing.T) {
			encrypted, err := cm.EncryptString(s)
			if err != nil {
				t.Fatalf("EncryptString() error: %v", err)
			}

			// Should be base64 encoded (printable)
			for _, c := range encrypted {
				if c > 127 {
					t.Errorf("EncryptString() contains non-ASCII: %c", c)
				}
			}

			decrypted, err := cm.DecryptString(encrypted)
			if err != nil {
				t.Fatalf("DecryptString() error: %v", err)
			}

			if decrypted != s {
				t.Errorf("DecryptString() = %q, want %q", decrypted, s)
			}
		})
	}
}

func TestEncryptionKeyPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first crypto manager - should generate key
	cm1, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() first call error: %v", err)
	}

	// Encrypt something
	plaintext := []byte("test data for key persistence")
	encrypted, err := cm1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	// Create second crypto manager - should load same key
	cm2, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() second call error: %v", err)
	}

	// Should be able to decrypt with second manager
	decrypted, err := cm2.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() with second manager error: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Second crypto manager couldn't decrypt data from first")
	}
}

func TestEncryptionKeyFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	keyPath := filepath.Join(tmpDir, ".encryption.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}

	// Key file should have restricted permissions (0600)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("Key file permissions = %o, want 0600", mode)
	}
}

func TestNewCryptoManagerAt_DefaultDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	withDefaultDataDir(t, tmpDir)

	cm, err := NewCryptoManagerAt("")
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}
	if cm.keyPath != filepath.Join(tmpDir, ".encryption.key") {
		t.Fatalf("keyPath = %q, want %q", cm.keyPath, filepath.Join(tmpDir, ".encryption.key"))
	}
}

func TestNewCryptoManagerAt_KeyError(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))
	err := os.WriteFile(filepath.Join(tmpDir, "nodes.enc"), []byte("data"), 0600)
	if err != nil {
		t.Fatalf("Failed to create encrypted file: %v", err)
	}

	_, err = NewCryptoManagerAt(tmpDir)
	if err == nil {
		t.Fatal("Expected error when encrypted data exists without a key")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	// Try to decrypt garbage
	_, err = cm.Decrypt([]byte("not encrypted data"))
	if err == nil {
		t.Error("Decrypt() should fail on invalid data")
	}

	// Try to decrypt empty
	_, err = cm.Decrypt([]byte{})
	if err == nil {
		t.Error("Decrypt() should fail on empty data")
	}

	// Try to decrypt data that's too short for nonce
	_, err = cm.Decrypt([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Error("Decrypt() should fail on data too short for nonce")
	}
}

func TestGetOrCreateKeyAt_InvalidBase64(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	keyPath := filepath.Join(tmpDir, ".encryption.key")
	if err := os.WriteFile(keyPath, []byte("not-base64"), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	key, err := getOrCreateKeyAt(tmpDir)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestGetOrCreateKeyAt_DefaultDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	withDefaultDataDir(t, tmpDir)
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	key, err := getOrCreateKeyAt("")
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestGetOrCreateKeyAt_InvalidLength(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	shortKey := make([]byte, 16)
	for i := range shortKey {
		shortKey[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(shortKey)
	if err := os.WriteFile(filepath.Join(tmpDir, ".encryption.key"), []byte(encoded), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	key, err := getOrCreateKeyAt(tmpDir)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestGetOrCreateKeyAt_SkipMigrationWhenPathsMatch(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(tmpDir, ".encryption.key"))

	key, err := getOrCreateKeyAt(tmpDir)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}

func TestGetOrCreateKeyAt_MigrateSuccess(t *testing.T) {
	legacyDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, ".encryption.key")
	withLegacyKeyPath(t, legacyPath)

	oldKey := make([]byte, 32)
	for i := range oldKey {
		oldKey[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(oldKey)
	if err := os.WriteFile(legacyPath, []byte(encoded), 0600); err != nil {
		t.Fatalf("Failed to write legacy key: %v", err)
	}

	newDir := t.TempDir()
	key, err := getOrCreateKeyAt(newDir)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if !bytes.Equal(key, oldKey) {
		t.Fatalf("migrated key mismatch")
	}
	contents, err := os.ReadFile(filepath.Join(newDir, ".encryption.key"))
	if err != nil {
		t.Fatalf("Failed to read migrated key: %v", err)
	}
	if string(contents) != encoded {
		t.Fatalf("migrated key contents mismatch")
	}
}

func TestGetOrCreateKeyAt_MigrateMkdirError(t *testing.T) {
	legacyDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, ".encryption.key")
	withLegacyKeyPath(t, legacyPath)

	oldKey := make([]byte, 32)
	for i := range oldKey {
		oldKey[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(oldKey)
	if err := os.WriteFile(legacyPath, []byte(encoded), 0600); err != nil {
		t.Fatalf("Failed to write legacy key: %v", err)
	}

	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "datafile")
	if err := os.WriteFile(dataFile, []byte("x"), 0600); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}

	key, err := getOrCreateKeyAt(dataFile)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if !bytes.Equal(key, oldKey) {
		t.Fatalf("expected legacy key on mkdir error")
	}
}

func TestGetOrCreateKeyAt_MigrateWriteError(t *testing.T) {
	legacyDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, ".encryption.key")
	withLegacyKeyPath(t, legacyPath)

	oldKey := make([]byte, 32)
	for i := range oldKey {
		oldKey[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(oldKey)
	if err := os.WriteFile(legacyPath, []byte(encoded), 0600); err != nil {
		t.Fatalf("Failed to write legacy key: %v", err)
	}

	newDir := t.TempDir()
	keyPath := filepath.Join(newDir, ".encryption.key")
	if err := os.MkdirAll(keyPath, 0700); err != nil {
		t.Fatalf("Failed to create key path dir: %v", err)
	}

	key, err := getOrCreateKeyAt(newDir)
	if err != nil {
		t.Fatalf("getOrCreateKeyAt() error: %v", err)
	}
	if !bytes.Equal(key, oldKey) {
		t.Fatalf("expected legacy key on write error")
	}
}

func TestGetOrCreateKeyAt_EncryptedDataExists(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	if err := os.WriteFile(filepath.Join(tmpDir, "nodes.enc"), []byte("data"), 0600); err != nil {
		t.Fatalf("Failed to write encrypted file: %v", err)
	}

	_, err := getOrCreateKeyAt(tmpDir)
	if err == nil {
		t.Fatal("Expected error when encrypted data exists")
	}
}

func TestGetOrCreateKeyAt_RandReaderError(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))
	withRandReader(t, errReader{err: errors.New("read failed")})

	_, err := getOrCreateKeyAt(tmpDir)
	if err == nil {
		t.Fatal("Expected error from rand reader")
	}
}

func TestGetOrCreateKeyAt_CreateDirError(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	dataFile := filepath.Join(tmpDir, "datafile")
	if err := os.WriteFile(dataFile, []byte("x"), 0600); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}

	_, err := getOrCreateKeyAt(dataFile)
	if err == nil {
		t.Fatal("Expected error when creating directory")
	}
}

func TestGetOrCreateKeyAt_SaveKeyError(t *testing.T) {
	tmpDir := t.TempDir()
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	keyPath := filepath.Join(tmpDir, ".encryption.key")
	if err := os.MkdirAll(keyPath, 0700); err != nil {
		t.Fatalf("Failed to create key path dir: %v", err)
	}

	_, err := getOrCreateKeyAt(tmpDir)
	if err == nil {
		t.Fatal("Expected error when saving key")
	}
}

func TestDecryptStringInvalidBase64(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	// Invalid base64
	_, err = cm.DecryptString("not!valid@base64#string")
	if err == nil {
		t.Error("DecryptString() should fail on invalid base64")
	}
}

func TestEncryptionUniqueness(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	plaintext := []byte("same plaintext")

	// Encrypt the same data twice
	encrypted1, _ := cm.Encrypt(plaintext)
	encrypted2, _ := cm.Encrypt(plaintext)

	// Should produce different ciphertext (due to random nonce)
	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("Encrypting same plaintext produced identical ciphertext (nonce reuse?)")
	}

	// But both should decrypt to same plaintext
	decrypted1, _ := cm.Decrypt(encrypted1)
	decrypted2, _ := cm.Decrypt(encrypted2)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Different ciphertexts didn't decrypt to same plaintext")
	}
}

func TestEncryptInvalidKey(t *testing.T) {
	cm := &CryptoManager{key: []byte("short")}
	if _, err := cm.Encrypt([]byte("data")); err == nil {
		t.Fatal("Expected error for invalid key length")
	}
}

func TestDecryptInvalidKey(t *testing.T) {
	cm := &CryptoManager{key: []byte("short")}
	if _, err := cm.Decrypt([]byte("data")); err == nil {
		t.Fatal("Expected error for invalid key length")
	}
}

func TestEncryptNonceReadError(t *testing.T) {
	withRandReader(t, errReader{err: errors.New("nonce read error")})
	cm := &CryptoManager{key: make([]byte, 32)}
	if _, err := cm.Encrypt([]byte("data")); err == nil {
		t.Fatal("Expected error reading nonce")
	}
}

func TestEncryptDecryptGCMError(t *testing.T) {
	withNewGCM(t, func(cipher.Block) (cipher.AEAD, error) {
		return nil, errors.New("gcm error")
	})

	cm := &CryptoManager{key: make([]byte, 32)}
	if _, err := cm.Encrypt([]byte("data")); err == nil {
		t.Fatal("Expected Encrypt error from GCM")
	}
	if _, err := cm.Decrypt([]byte("data")); err == nil {
		t.Fatal("Expected Decrypt error from GCM")
	}
}

func TestEncryptStringError(t *testing.T) {
	cm := &CryptoManager{key: []byte("short")}
	if _, err := cm.EncryptString("data"); err == nil {
		t.Fatal("Expected EncryptString error")
	}
}

func TestDecryptStringError(t *testing.T) {
	cm := &CryptoManager{key: make([]byte, 32)}
	encoded := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := cm.DecryptString(encoded); err == nil {
		t.Fatal("Expected DecryptString error")
	}
}

func TestNewCryptoManagerRefusesOrphanedData(t *testing.T) {
	// Ensure we don't accidentally read a real production key during the legacy-migration path.
	withLegacyKeyPath(t, filepath.Join(t.TempDir(), ".encryption.key"))

	tmpDir := t.TempDir()

	// Create an encrypted data file without a key
	encFile := filepath.Join(tmpDir, "nodes.enc")
	err := os.WriteFile(encFile, []byte("fake encrypted data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should fail because encrypted data exists but no key
	_, err = NewCryptoManagerAt(tmpDir)
	if err == nil {
		t.Error("NewCryptoManagerAt() should fail when encrypted data exists without key")
	}
}

func TestLargeDataEncryption(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	// Create 1MB of random-ish data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	encrypted, err := cm.Encrypt(largeData)
	if err != nil {
		t.Fatalf("Encrypt() large data error: %v", err)
	}

	decrypted, err := cm.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() large data error: %v", err)
	}

	if !bytes.Equal(decrypted, largeData) {
		t.Error("Large data round-trip failed")
	}
}

func TestEncryptRefusesAfterKeyDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCryptoManagerAt(tmpDir)
	if err != nil {
		t.Fatalf("NewCryptoManagerAt() error: %v", err)
	}

	// Encrypt should work initially
	plaintext := []byte("test data")
	encrypted, err := cm.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Initial Encrypt() failed: %v", err)
	}

	// Decrypt should also work
	_, err = cm.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Initial Decrypt() failed: %v", err)
	}

	// Now delete the key file (simulating what happened in the bug)
	keyPath := filepath.Join(tmpDir, ".encryption.key")
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("Failed to remove key file: %v", err)
	}

	// Encrypt should now FAIL to prevent orphaned data
	_, err = cm.Encrypt([]byte("new data"))
	if err == nil {
		t.Error("Encrypt() should fail after key file is deleted")
	}

	// Decrypt should still work (key is in memory)
	decrypted, err := cm.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() should still work with in-memory key: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypt() returned wrong data")
	}
}
