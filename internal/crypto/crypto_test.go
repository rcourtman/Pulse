package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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

func TestNewCryptoManagerRefusesOrphanedData(t *testing.T) {
	// Skip if production key exists - migration code will always find and use it
	if _, err := os.Stat("/etc/pulse/.encryption.key"); err == nil {
		t.Skip("Skipping: production encryption key exists at /etc/pulse/.encryption.key - migration will find it")
	}

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
