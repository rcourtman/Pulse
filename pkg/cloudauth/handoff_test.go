package cloudauth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateHandoffKey(t *testing.T) {
	key, err := GenerateHandoffKey()
	if err != nil {
		t.Fatalf("GenerateHandoffKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(key))
	}
	// Ensure two keys are different (probabilistic but extremely reliable).
	key2, _ := GenerateHandoffKey()
	if string(key) == string(key2) {
		t.Fatal("two generated keys are identical")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	key, _ := GenerateHandoffKey()
	token, err := Sign(key, "alice@example.com", "t-abc123", 60*time.Second)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if token == "" {
		t.Fatal("Sign returned empty token")
	}

	email, tenantID, err := Verify(key, token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", email)
	}
	if tenantID != "t-abc123" {
		t.Errorf("tenantID = %q, want t-abc123", tenantID)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	key, _ := GenerateHandoffKey()
	// Sign with a TTL of -1 second so the token is already expired.
	token, err := Sign(key, "bob@example.com", "t-xyz", -1*time.Second)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	_, _, err = Verify(key, token)
	if err != ErrHandoffExpired {
		t.Fatalf("expected ErrHandoffExpired, got %v", err)
	}
}

func TestVerifyWrongKey(t *testing.T) {
	key1, _ := GenerateHandoffKey()
	key2, _ := GenerateHandoffKey()

	token, _ := Sign(key1, "carol@example.com", "t-111", 60*time.Second)

	_, _, err := Verify(key2, token)
	if err != ErrHandoffInvalid {
		t.Fatalf("expected ErrHandoffInvalid, got %v", err)
	}
}

func TestVerifyTamperedPayload(t *testing.T) {
	key, _ := GenerateHandoffKey()
	token, _ := Sign(key, "dave@example.com", "t-222", 60*time.Second)

	// Tamper with the payload portion (before the dot).
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		t.Fatal("token missing dot separator")
	}

	// Decode payload, modify, re-encode.
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	tampered := make([]byte, len(payloadBytes))
	copy(tampered, payloadBytes)
	// Flip a byte.
	if len(tampered) > 5 {
		tampered[5] ^= 0xFF
	}
	parts[0] = base64.RawURLEncoding.EncodeToString(tampered)
	tamperedToken := parts[0] + "." + parts[1]

	_, _, err := Verify(key, tamperedToken)
	if err != ErrHandoffInvalid {
		t.Fatalf("expected ErrHandoffInvalid, got %v", err)
	}
}

func TestVerifyWithExpiryReturnsTokenExpiry(t *testing.T) {
	key, _ := GenerateHandoffKey()
	ttl := 2 * time.Minute
	before := time.Now().UTC()
	token, err := Sign(key, "eve@example.com", "t-expiry", ttl)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	email, tenantID, expiresAt, err := VerifyWithExpiry(key, token)
	if err != nil {
		t.Fatalf("VerifyWithExpiry: %v", err)
	}
	if email != "eve@example.com" {
		t.Fatalf("email = %q, want eve@example.com", email)
	}
	if tenantID != "t-expiry" {
		t.Fatalf("tenantID = %q, want t-expiry", tenantID)
	}

	lowerBound := before.Add(ttl - 5*time.Second)
	upperBound := before.Add(ttl + 5*time.Second)
	if expiresAt.Before(lowerBound) || expiresAt.After(upperBound) {
		t.Fatalf("expiresAt = %s, expected between %s and %s", expiresAt, lowerBound, upperBound)
	}
}

func TestVerifyEmptyInputs(t *testing.T) {
	key, _ := GenerateHandoffKey()

	_, _, err := Verify(nil, "some-token")
	if err != ErrHandoffInvalid {
		t.Errorf("nil key: expected ErrHandoffInvalid, got %v", err)
	}

	_, _, err = Verify(key, "")
	if err != ErrHandoffInvalid {
		t.Errorf("empty token: expected ErrHandoffInvalid, got %v", err)
	}
}

func TestSignValidation(t *testing.T) {
	key, _ := GenerateHandoffKey()

	_, err := Sign(nil, "a@b.com", "t-1", time.Minute)
	if err == nil {
		t.Error("Sign with nil key should fail")
	}

	_, err = Sign(key, "", "t-1", time.Minute)
	if err == nil {
		t.Error("Sign with empty email should fail")
	}

	_, err = Sign(key, "a@b.com", "", time.Minute)
	if err == nil {
		t.Error("Sign with empty tenantID should fail")
	}
}
