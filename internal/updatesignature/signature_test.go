package updatesignature

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestSignAndVerifyBytes(t *testing.T) {
	original := EmbeddedTrustedPublicKeys
	t.Cleanup(func() { EmbeddedTrustedPublicKeys = original })

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	EmbeddedTrustedPublicKeys = base64.StdEncoding.EncodeToString(publicKey)

	signature, err := SignBytes([]byte("payload"), privateKey)
	if err != nil {
		t.Fatalf("sign bytes: %v", err)
	}
	if err := VerifyBytes([]byte("payload"), signature); err != nil {
		t.Fatalf("verify bytes: %v", err)
	}
	if err := VerifyBytes([]byte("payload"), ""); err == nil {
		t.Fatal("expected missing signature error")
	}
	if err := VerifyBytes([]byte("payload"), "!!!"); err == nil {
		t.Fatal("expected invalid signature encoding error")
	}
	if err := VerifyBytes([]byte("different"), signature); err == nil {
		t.Fatal("expected signature mismatch")
	}
}

func TestSignAndVerifyFile(t *testing.T) {
	original := EmbeddedTrustedPublicKeys
	t.Cleanup(func() { EmbeddedTrustedPublicKeys = original })

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	EmbeddedTrustedPublicKeys = base64.StdEncoding.EncodeToString(publicKey)

	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	signature, err := SignFile(path, privateKey)
	if err != nil {
		t.Fatalf("sign file: %v", err)
	}
	if err := VerifyFile(path, signature); err != nil {
		t.Fatalf("verify file: %v", err)
	}
}

func TestPublicKeyString(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	encoded, err := PublicKeyString(privateKey)
	if err != nil {
		t.Fatalf("public key string: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Fatalf("decoded public key length = %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
}

func TestTrustedPublicKeysRejectsInvalidInput(t *testing.T) {
	original := EmbeddedTrustedPublicKeys
	t.Cleanup(func() { EmbeddedTrustedPublicKeys = original })

	EmbeddedTrustedPublicKeys = "not-base64"
	if _, err := trustedPublicKeys(); err == nil {
		t.Fatal("expected invalid base64 key error")
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	rsaBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal rsa key: %v", err)
	}
	EmbeddedTrustedPublicKeys = base64.StdEncoding.EncodeToString(rsaBytes)
	if _, err := trustedPublicKeys(); err == nil {
		t.Fatal("expected non-ed25519 key error")
	}
}

func TestTrustedPublicKeysAcceptsPEM(t *testing.T) {
	original := EmbeddedTrustedPublicKeys
	t.Cleanup(func() { EmbeddedTrustedPublicKeys = original })

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	EmbeddedTrustedPublicKeys = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}))

	keys, err := trustedPublicKeys()
	if err != nil {
		t.Fatalf("trusted public keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("trusted public keys length = %d, want 1", len(keys))
	}
}
