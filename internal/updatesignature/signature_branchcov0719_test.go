package updatesignature

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestDecodePrivateKey(t *testing.T) {
	t.Run("full key roundtrip", func(t *testing.T) {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		encoded := base64.StdEncoding.EncodeToString(privateKey)

		decoded, err := DecodePrivateKey(encoded)
		if err != nil {
			t.Fatalf("DecodePrivateKey: %v", err)
		}
		if !bytesEqual(decoded, privateKey) {
			t.Fatalf("decoded key = %x, want %x", []byte(decoded), []byte(privateKey))
		}
		if !decoded.Equal(privateKey) {
			t.Fatalf("decoded key does not equal original")
		}
	})

	t.Run("seed roundtrip derives same key", func(t *testing.T) {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		seed := privateKey.Seed()
		encoded := base64.StdEncoding.EncodeToString(seed)

		decoded, err := DecodePrivateKey(encoded)
		if err != nil {
			t.Fatalf("DecodePrivateKey from seed: %v", err)
		}
		if !decoded.Equal(privateKey) {
			t.Fatalf("seed-derived key = %x, want %x", []byte(decoded), []byte(privateKey))
		}
	})

	t.Run("empty returns error and nil key", func(t *testing.T) {
		decoded, err := DecodePrivateKey("")
		if err == nil {
			t.Fatal("expected error for empty input, got nil")
		}
		if !strings.Contains(err.Error(), "empty signing key") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "empty signing key")
		}
		if decoded != nil {
			t.Fatalf("expected nil key, got %x", []byte(decoded))
		}
	})

	t.Run("whitespace only returns empty error", func(t *testing.T) {
		decoded, err := DecodePrivateKey("   \t\n  ")
		if err == nil {
			t.Fatal("expected error for whitespace input, got nil")
		}
		if !strings.Contains(err.Error(), "empty signing key") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "empty signing key")
		}
		if decoded != nil {
			t.Fatalf("expected nil key, got %x", []byte(decoded))
		}
	})

	t.Run("invalid base64 returns wrapped error and nil key", func(t *testing.T) {
		decoded, err := DecodePrivateKey("not-base64!!!")
		if err == nil {
			t.Fatal("expected error for invalid base64, got nil")
		}
		if !strings.Contains(err.Error(), "invalid base64 signing key") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "invalid base64 signing key")
		}
		var inner interface{ Unwrap() error }
		if !errors.As(err, &inner) {
			t.Fatalf("expected wrapped error, got %T: %v", err, err)
		}
		if decoded != nil {
			t.Fatalf("expected nil key, got %x", []byte(decoded))
		}
	})

	t.Run("valid base64 wrong length returns error and nil key", func(t *testing.T) {
		bogus := []byte{1, 2, 3, 4, 5}
		encoded := base64.StdEncoding.EncodeToString(bogus)

		decoded, err := DecodePrivateKey(encoded)
		if err == nil {
			t.Fatal("expected error for wrong-length key, got nil")
		}
		if !strings.Contains(err.Error(), "invalid signing key length") {
			t.Fatalf("error = %q, want substring %q", err.Error(), "invalid signing key length")
		}
		if !strings.Contains(err.Error(), "5") {
			t.Fatalf("error = %q, want it to contain the bad length 5", err.Error())
		}
		if decoded != nil {
			t.Fatalf("expected nil key, got %x", []byte(decoded))
		}
	})
}

func TestHasTrustedPublicKeys(t *testing.T) {
	original := EmbeddedTrustedPublicKeys
	t.Cleanup(func() { EmbeddedTrustedPublicKeys = original })

	t.Run("empty returns false", func(t *testing.T) {
		EmbeddedTrustedPublicKeys = ""
		if HasTrustedPublicKeys() {
			t.Fatal("HasTrustedPublicKeys() = true, want false for empty")
		}
	})

	t.Run("whitespace only returns false", func(t *testing.T) {
		EmbeddedTrustedPublicKeys = "   \t\n  "
		if HasTrustedPublicKeys() {
			t.Fatal("HasTrustedPublicKeys() = true, want false for whitespace-only")
		}
	})

	t.Run("populated returns true", func(t *testing.T) {
		EmbeddedTrustedPublicKeys = "c29tZWtleQ=="
		if !HasTrustedPublicKeys() {
			t.Fatal("HasTrustedPublicKeys() = false, want true for populated value")
		}
	})
}

// bytesEqual is a tiny helper to avoid pulling in bytes for a single call.
func bytesEqual(a, b ed25519.PrivateKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
