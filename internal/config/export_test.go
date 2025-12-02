package config

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// Internal tests for unexported functions in export.go

func TestEncryptDecryptWithPassphrase(t *testing.T) {
	tests := []struct {
		name       string
		plaintext  []byte
		passphrase string
	}{
		{
			name:       "simple text",
			plaintext:  []byte("hello world"),
			passphrase: "secret123",
		},
		{
			name:       "empty string",
			plaintext:  []byte(""),
			passphrase: "password",
		},
		{
			name:       "large data",
			plaintext:  bytes.Repeat([]byte("test data "), 1000),
			passphrase: "mypassphrase",
		},
		{
			name:       "unicode content",
			plaintext:  []byte("こんにちは世界 🌍"),
			passphrase: "pass123",
		},
		{
			name:       "binary data",
			plaintext:  []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
			passphrase: "binary-pass",
		},
		{
			name:       "json data",
			plaintext:  []byte(`{"version":"4.1","nodes":{"pve":[]}}`),
			passphrase: "json-export-pass",
		},
		{
			name:       "long passphrase",
			plaintext:  []byte("test data"),
			passphrase: "this is a very long passphrase that exceeds the normal key length",
		},
		{
			name:       "short passphrase",
			plaintext:  []byte("test data"),
			passphrase: "a",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := encryptWithPassphrase(tc.plaintext, tc.passphrase)
			if err != nil {
				t.Fatalf("encryptWithPassphrase failed: %v", err)
			}

			// Verify encrypted data is different from plaintext
			if bytes.Equal(encrypted, tc.plaintext) && len(tc.plaintext) > 0 {
				t.Error("encrypted data should be different from plaintext")
			}

			// Decrypt
			decrypted, err := decryptWithPassphrase(encrypted, tc.passphrase)
			if err != nil {
				t.Fatalf("decryptWithPassphrase failed: %v", err)
			}

			// Verify roundtrip
			if !bytes.Equal(decrypted, tc.plaintext) {
				t.Errorf("decrypted = %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestEncryptWithPassphrase_UniqueOutput(t *testing.T) {
	plaintext := []byte("test data")
	passphrase := "password"

	// Encrypt twice - should produce different ciphertexts due to random salt/nonce
	encrypted1, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("first encryption failed: %v", err)
	}

	encrypted2, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("second encryption failed: %v", err)
	}

	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("encrypting same data twice should produce different ciphertext")
	}

	// Both should decrypt to the same plaintext
	decrypted1, _ := decryptWithPassphrase(encrypted1, passphrase)
	decrypted2, _ := decryptWithPassphrase(encrypted2, passphrase)

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("both ciphertexts should decrypt to original plaintext")
	}
}

func TestDecryptWithPassphrase_WrongPassphrase(t *testing.T) {
	plaintext := []byte("secret data")

	encrypted, err := encryptWithPassphrase(plaintext, "correct-password")
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Try to decrypt with wrong password - should fail
	_, err = decryptWithPassphrase(encrypted, "wrong-password")
	if err == nil {
		t.Error("decryption with wrong passphrase should fail")
	}
}

func TestDecryptWithPassphrase_TooShort(t *testing.T) {
	// Ciphertext shorter than salt (32 bytes)
	shortCiphertext := []byte("too short")
	_, err := decryptWithPassphrase(shortCiphertext, "password")
	if err == nil {
		t.Error("decryption should fail for ciphertext shorter than salt")
	}

	// Ciphertext exactly 32 bytes (only salt, no actual encrypted data)
	saltOnly := make([]byte, 32)
	_, err = decryptWithPassphrase(saltOnly, "password")
	if err == nil {
		t.Error("decryption should fail when no encrypted data present")
	}
}

func TestDecryptWithPassphrase_TamperedCiphertext(t *testing.T) {
	plaintext := []byte("test data")
	passphrase := "password"

	encrypted, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Tamper with the ciphertext (modify a byte in the middle)
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[len(tampered)/2] ^= 0xff

	// Decryption should fail due to authentication failure
	_, err = decryptWithPassphrase(tampered, passphrase)
	if err == nil {
		t.Error("decryption should fail for tampered ciphertext")
	}
}

func TestDecryptWithPassphrase_TamperedSalt(t *testing.T) {
	plaintext := []byte("test data")
	passphrase := "password"

	encrypted, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Tamper with the salt (first 32 bytes)
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[0] ^= 0xff

	// Decryption should fail because wrong key will be derived
	_, err = decryptWithPassphrase(tampered, passphrase)
	if err == nil {
		t.Error("decryption should fail for tampered salt")
	}
}

func TestEncryptedData_MinimumSize(t *testing.T) {
	plaintext := []byte("x")
	passphrase := "pass"

	encrypted, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Encrypted data should be at least:
	// - 32 bytes salt
	// - 12 bytes nonce (GCM default)
	// - 1 byte plaintext
	// - 16 bytes auth tag
	minSize := 32 + 12 + 1 + 16
	if len(encrypted) < minSize {
		t.Errorf("encrypted data size = %d, want at least %d", len(encrypted), minSize)
	}
}

func TestExportData_Fields(t *testing.T) {
	ed := ExportData{
		Version: "4.1",
	}

	if ed.Version != "4.1" {
		t.Errorf("Version = %q, want 4.1", ed.Version)
	}

	if ed.GuestMetadata != nil {
		t.Error("GuestMetadata should be nil by default")
	}

	if ed.OIDC != nil {
		t.Error("OIDC should be nil by default")
	}

	if ed.APITokens != nil {
		t.Error("APITokens should be nil by default")
	}
}

func TestEncryptDecrypt_Base64Roundtrip(t *testing.T) {
	// Test the actual export/import flow with base64 encoding
	plaintext := []byte(`{"version":"4.1","exportedAt":"2024-01-01T00:00:00Z"}`)
	passphrase := "export-password"

	// Encrypt
	encrypted, err := encryptWithPassphrase(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	// Base64 encode (as done in ExportConfig)
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	// Base64 decode (as done in ImportConfig)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	// Decrypt
	decrypted, err := decryptWithPassphrase(decoded, passphrase)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptWithPassphrase_EmptyPassphrase(t *testing.T) {
	// While ExportConfig validates empty passphrase, the underlying function
	// should still handle it (or we document it doesn't work with empty)
	plaintext := []byte("test")

	// Empty passphrase is technically allowed by the encryption function
	// (PBKDF2 handles it), but it's very weak
	encrypted, err := encryptWithPassphrase(plaintext, "")
	if err != nil {
		t.Fatalf("encryption with empty passphrase failed: %v", err)
	}

	decrypted, err := decryptWithPassphrase(encrypted, "")
	if err != nil {
		t.Fatalf("decryption with empty passphrase failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip with empty passphrase failed")
	}
}

func TestImportConfig_EmptyPassphrase(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	err := cp.ImportConfig("somedata", "")
	if err == nil {
		t.Error("expected error for empty passphrase")
	}
	if err.Error() != "passphrase is required for import" {
		t.Errorf("expected 'passphrase is required' error, got: %v", err)
	}
}

func TestImportConfig_InvalidBase64(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	err := cp.ImportConfig("not-valid-base64!!!", "somepass")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestImportConfig_WrongPassphrase(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Create valid encrypted data with one passphrase
	plaintext := []byte(`{"version":"4.1","nodes":{"pve":[],"pbs":[],"pmg":[]},"alerts":{},"email":{},"apprise":{},"webhooks":[],"system":{}}`)
	encrypted, err := encryptWithPassphrase(plaintext, "correct-pass")
	if err != nil {
		t.Fatalf("failed to create test data: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	// Try to import with wrong passphrase
	err = cp.ImportConfig(encoded, "wrong-pass")
	if err == nil {
		t.Error("expected error for wrong passphrase")
	}
}

func TestImportConfig_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	// Create valid encrypted data but with invalid JSON content
	plaintext := []byte(`{not valid json`)
	encrypted, err := encryptWithPassphrase(plaintext, "test-pass")
	if err != nil {
		t.Fatalf("failed to create test data: %v", err)
	}
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	err = cp.ImportConfig(encoded, "test-pass")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
