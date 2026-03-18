package relay

import (
	"encoding/base64"
	"regexp"
	"testing"
)

func TestGenerateIdentityKeyPair(t *testing.T) {
	priv, pub, fp, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatalf("GenerateIdentityKeyPair() error: %v", err)
	}

	// Private key should base64-decode to 64 bytes (Ed25519 private key)
	privBytes, err := base64.StdEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privBytes) != 64 {
		t.Errorf("private key length: got %d, want 64", len(privBytes))
	}

	// Public key should base64-decode to 32 bytes (Ed25519 public key)
	pubBytes, err := base64.StdEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("public key length: got %d, want 32", len(pubBytes))
	}

	// Fingerprint format: 32 hex pairs separated by colons = 95 chars
	if len(fp) != 95 {
		t.Errorf("fingerprint length: got %d, want 95", len(fp))
	}
	fpPattern := regexp.MustCompile(`^([0-9A-F]{2}:){31}[0-9A-F]{2}$`)
	if !fpPattern.MatchString(fp) {
		t.Errorf("fingerprint format invalid: %q", fp)
	}
}

func TestComputeFingerprint_Deterministic(t *testing.T) {
	_, pub, fp1, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatalf("GenerateIdentityKeyPair() error: %v", err)
	}

	fp2, err := ComputeFingerprint(pub)
	if err != nil {
		t.Fatalf("ComputeFingerprint() error: %v", err)
	}

	if fp1 != fp2 {
		t.Errorf("fingerprints differ for same key: %q vs %q", fp1, fp2)
	}
}

func TestGenerateIdentityKeyPair_Unique(t *testing.T) {
	_, pub1, _, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatalf("first GenerateIdentityKeyPair() error: %v", err)
	}

	_, pub2, _, err := GenerateIdentityKeyPair()
	if err != nil {
		t.Fatalf("second GenerateIdentityKeyPair() error: %v", err)
	}

	if pub1 == pub2 {
		t.Error("two calls produced identical public keys")
	}
}

func TestComputeFingerprint_InvalidBase64(t *testing.T) {
	_, err := ComputeFingerprint("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestComputeFingerprint_WrongKeyLength(t *testing.T) {
	// 16 bytes instead of 32 — valid base64 but wrong length for Ed25519
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := ComputeFingerprint(shortKey)
	if err == nil {
		t.Error("expected error for 16-byte key, got nil")
	}

	// 64 bytes — also wrong
	longKey := base64.StdEncoding.EncodeToString(make([]byte, 64))
	_, err = ComputeFingerprint(longKey)
	if err == nil {
		t.Error("expected error for 64-byte key, got nil")
	}
}
