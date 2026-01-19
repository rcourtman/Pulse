package dockeragent

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	originalKeys := trustedPublicKeysPEM
	defer func() {
		trustedPublicKeysPEM = originalKeys
	}()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	trustedPublicKeysPEM = []string{string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))}

	data := []byte("payload")
	sig := ed25519.Sign(privateKey, data)
	signature := base64.StdEncoding.EncodeToString(sig)

	if err := verifySignature(data, signature); err != nil {
		t.Fatalf("expected signature to verify: %v", err)
	}

	if err := verifySignature(data, ""); err == nil {
		t.Fatal("expected missing signature error")
	}
	if err := verifySignature(data, "!!!"); err == nil {
		t.Fatal("expected invalid base64 error")
	}

	// Invalid signature
	invalidSig := base64.StdEncoding.EncodeToString([]byte("bad"))
	if err := verifySignature(data, invalidSig); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestVerifySignatureInvalidKeys(t *testing.T) {
	originalKeys := trustedPublicKeysPEM
	defer func() {
		trustedPublicKeysPEM = originalKeys
	}()

	trustedPublicKeysPEM = []string{"not-pem"}
	if err := verifySignature([]byte("data"), base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected error for invalid pem")
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal rsa: %v", err)
	}
	trustedPublicKeysPEM = []string{string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))}

	if err := verifySignature([]byte("data"), base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatal("expected error for non-ed25519 key")
	}
}

func TestVerifyFileSignature(t *testing.T) {
	originalKeys := trustedPublicKeysPEM
	defer func() {
		trustedPublicKeysPEM = originalKeys
	}()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	trustedPublicKeysPEM = []string{string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))}

	file := filepathJoin(t)
	data := []byte("file")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sig := ed25519.Sign(privateKey, data)
	signature := base64.StdEncoding.EncodeToString(sig)
	if err := verifyFileSignature(file, signature); err != nil {
		t.Fatalf("expected file signature to verify: %v", err)
	}

	if err := verifyFileSignature("missing", signature); err == nil {
		t.Fatal("expected error for missing file")
	}

	if err := verifyFileSignature(file, "!!!"); err == nil {
		t.Fatal("expected base64 error")
	}
}

func filepathJoin(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	return tmp + "/payload"
}
