package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockCryptoManager implements CryptoEncryptor for testing.
type mockCryptoManager struct {
	key []byte
}

func newMockCryptoManager() *mockCryptoManager {
	return &mockCryptoManager{
		key: []byte("0123456789abcdef0123456789abcdef"), // 32 bytes
	}
}

func (m *mockCryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	// Simple XOR for testing (not secure, just for tests)
	result := make([]byte, len(plaintext))
	for i := range plaintext {
		result[i] = plaintext[i] ^ m.key[i%len(m.key)]
	}
	return result, nil
}

func (m *mockCryptoManager) Decrypt(ciphertext []byte) ([]byte, error) {
	// XOR is symmetric
	return m.Encrypt(ciphertext)
}

type taggedMockCryptoManager struct{}

func (taggedMockCryptoManager) Encrypt(plaintext []byte) ([]byte, error) {
	out := make([]byte, 4+len(plaintext))
	copy(out, []byte("enc:"))
	copy(out[4:], plaintext)
	return out, nil
}

func (taggedMockCryptoManager) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 4 || string(ciphertext[:4]) != "enc:" {
		return nil, os.ErrInvalid
	}
	return append([]byte(nil), ciphertext[4:]...), nil
}

func TestNewSigner(t *testing.T) {
	tempDir := t.TempDir()
	crypto := newMockCryptoManager()

	// Create new signer
	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	if !signer.SigningEnabled() {
		t.Error("Expected signing to be enabled")
	}

	// Verify key file was created
	keyPath := filepath.Join(tempDir, ".audit-signing.key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("Key file was not created")
	}

	// Create another signer - should load existing key
	signer2, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner (reload) failed: %v", err)
	}

	// Both signers should produce the same signatures
	event := Event{
		ID:        "test-123",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType: "login",
		User:      "admin",
		IP:        "192.168.1.1",
		Path:      "/api/auth",
		Success:   true,
		Details:   "test details",
	}

	sig1 := signer.Sign(event)
	sig2 := signer2.Sign(event)

	if sig1 != sig2 {
		t.Errorf("Signatures should match: got %s and %s", sig1, sig2)
	}
}

func TestNewSigner_MigratesPlaintextKeyFile(t *testing.T) {
	tempDir := t.TempDir()
	crypto := taggedMockCryptoManager{}

	plaintextKey := []byte("0123456789abcdef0123456789abcdef")
	keyPath := filepath.Join(tempDir, ".audit-signing.key")
	if err := os.WriteFile(keyPath, plaintextKey, 0600); err != nil {
		t.Fatalf("write plaintext key: %v", err)
	}

	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}
	if !signer.SigningEnabled() {
		t.Fatal("expected signing to be enabled after plaintext key migration")
	}

	rewritten, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read rewritten key: %v", err)
	}
	if bytes.Equal(rewritten, plaintextKey) {
		t.Fatal("expected plaintext signing key to be rewritten encrypted")
	}

	reloaded, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner reload failed: %v", err)
	}
	event := Event{
		ID:        "migrate-1",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType: "login",
		User:      "admin",
	}
	if signer.Sign(event) != reloaded.Sign(event) {
		t.Fatal("expected migrated signer and reloaded signer to use the same key")
	}
}

func TestNewSignerWithoutCrypto(t *testing.T) {
	tempDir := t.TempDir()

	// Create signer without crypto manager
	signer, err := NewSigner(tempDir, nil)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	if signer.SigningEnabled() {
		t.Error("Expected signing to be disabled without crypto manager")
	}

	event := Event{
		ID:        "test-123",
		Timestamp: time.Now(),
		EventType: "test",
	}

	sig := signer.Sign(event)
	if sig != "" {
		t.Error("Expected empty signature when signing is disabled")
	}
}

func TestSignerSign(t *testing.T) {
	tempDir := t.TempDir()
	crypto := newMockCryptoManager()

	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	event := Event{
		ID:        "evt-001",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType: "login",
		User:      "testuser",
		IP:        "10.0.0.1",
		Path:      "/api/login",
		Success:   true,
		Details:   "successful login",
	}

	sig := signer.Sign(event)

	// Signature should be hex-encoded (64 characters for SHA256)
	if len(sig) != 64 {
		t.Errorf("Expected signature length 64, got %d", len(sig))
	}

	// Same event should produce same signature
	sig2 := signer.Sign(event)
	if sig != sig2 {
		t.Error("Same event should produce same signature")
	}

	// Different event should produce different signature
	event2 := event
	event2.User = "different"
	sig3 := signer.Sign(event2)
	if sig == sig3 {
		t.Error("Different events should produce different signatures")
	}
}

func TestSignerVerify(t *testing.T) {
	tempDir := t.TempDir()
	crypto := newMockCryptoManager()

	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	event := Event{
		ID:        "evt-002",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType: "config_change",
		User:      "admin",
		IP:        "192.168.1.100",
		Path:      "/api/settings",
		Success:   true,
		Details:   "changed setting X",
	}

	// Sign the event
	event.Signature = signer.Sign(event)

	// Verify should succeed
	if !signer.Verify(event) {
		t.Error("Verify should return true for valid signature")
	}

	// Tamper with event
	tamperedEvent := event
	tamperedEvent.User = "hacker"
	if signer.Verify(tamperedEvent) {
		t.Error("Verify should return false for tampered event")
	}

	// Wrong signature
	wrongSigEvent := event
	wrongSigEvent.Signature = "0000000000000000000000000000000000000000000000000000000000000000"
	if signer.Verify(wrongSigEvent) {
		t.Error("Verify should return false for wrong signature")
	}

	// Empty signature
	noSigEvent := event
	noSigEvent.Signature = ""
	if signer.Verify(noSigEvent) {
		t.Error("Verify should return false for empty signature")
	}
}

func TestSignerCanonicalForm(t *testing.T) {
	tempDir := t.TempDir()
	crypto := newMockCryptoManager()

	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	// Test that canonical form is deterministic
	event := Event{
		ID:        "id123",
		Timestamp: time.Unix(1705315800, 0), // Fixed Unix timestamp
		EventType: "test",
		User:      "user",
		IP:        "1.2.3.4",
		Path:      "/path",
		Success:   true,
		Details:   "details",
	}

	sig1 := signer.Sign(event)
	sig2 := signer.Sign(event)

	if sig1 != sig2 {
		t.Error("Canonical form should be deterministic")
	}

	// Success=false should produce different signature
	event.Success = false
	sig3 := signer.Sign(event)
	if sig1 == sig3 {
		t.Error("Different success value should produce different signature")
	}
}

func TestSignerExportKey(t *testing.T) {
	tempDir := t.TempDir()
	crypto := newMockCryptoManager()

	signer, err := NewSigner(tempDir, crypto)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	key := signer.ExportKey()
	if key == "" {
		t.Error("ExportKey should return non-empty string")
	}

	// Key should be base64 encoded (44 characters for 32 bytes)
	if len(key) != 44 {
		t.Errorf("Expected base64 key length 44, got %d", len(key))
	}
}

func TestSignerExportKeyDisabled(t *testing.T) {
	tempDir := t.TempDir()

	signer, err := NewSigner(tempDir, nil)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}

	key := signer.ExportKey()
	if key != "" {
		t.Error("ExportKey should return empty string when signing is disabled")
	}
}
