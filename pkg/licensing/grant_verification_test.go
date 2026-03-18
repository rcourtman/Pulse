package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

func TestVerifyAndParseGrantJWT_ValidSignature(t *testing.T) {
	setupTestPublicKey(t)

	jwt := makeTestGrantJWT(t, &GrantClaims{
		LicenseID:           "lic_test",
		InstallationID:      "inst_abc",
		State:               "active",
		Tier:                "pro",
		Features:            []string{"relay"},
		MaxMonitoredSystems: 10,
		IssuedAt:            time.Now().Unix(),
		ExpiresAt:           time.Now().Add(72 * time.Hour).Unix(),
	})

	gc, err := verifyAndParseGrantJWT(jwt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gc.LicenseID != "lic_test" {
		t.Errorf("LicenseID = %q, want %q", gc.LicenseID, "lic_test")
	}
	if gc.Tier != "pro" {
		t.Errorf("Tier = %q, want %q", gc.Tier, "pro")
	}
	if gc.MaxMonitoredSystems != 10 {
		t.Errorf("MaxMonitoredSystems = %d, want 10", gc.MaxMonitoredSystems)
	}
}

func TestVerifyAndParseGrantJWT_TamperedPayload(t *testing.T) {
	setupTestPublicKey(t)

	// Create a valid signed JWT.
	jwt := makeTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_test",
		Tier:      "pro",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	// Tamper with the payload: replace it with a different one.
	parts := splitJWT(jwt)
	tampered := &GrantClaims{
		LicenseID: "lic_test",
		Tier:      "enterprise", // changed from "pro"
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	}
	tamperedPayload, _ := json.Marshal(tampered)
	parts[1] = base64.RawURLEncoding.EncodeToString(tamperedPayload)
	tamperedJWT := parts[0] + "." + parts[1] + "." + parts[2]

	_, err := verifyAndParseGrantJWT(tamperedJWT)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got: %v", err)
	}
}

func TestVerifyAndParseGrantJWT_WrongKey(t *testing.T) {
	setupTestPublicKey(t)

	// Sign with a completely different key pair.
	_, wrongPrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate wrong key: %v", err)
	}

	gc := &GrantClaims{
		LicenseID: "lic_test",
		Tier:      "pro",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	}
	payload, _ := json.Marshal(gc)
	jwt := signTestJWT(t, payload, wrongPrivKey)

	_, err = verifyAndParseGrantJWT(jwt)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got: %v", err)
	}
}

func TestVerifyAndParseGrantJWT_NoPublicKey_DevMode(t *testing.T) {
	// Clear public key and enable dev mode.
	prev := currentPublicKey()
	SetPublicKey(nil)
	t.Cleanup(func() { SetPublicKey(prev) })

	prevEnv := os.Getenv("PULSE_LICENSE_DEV_MODE")
	os.Setenv("PULSE_LICENSE_DEV_MODE", "true")
	t.Cleanup(func() { os.Setenv("PULSE_LICENSE_DEV_MODE", prevEnv) })

	// Use an unsigned JWT — should pass in dev mode with no key.
	jwt := makeUnsignedTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_dev",
		Tier:      "pro",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	gc, err := verifyAndParseGrantJWT(jwt)
	if err != nil {
		t.Fatalf("expected success in dev mode, got: %v", err)
	}
	if gc.LicenseID != "lic_dev" {
		t.Errorf("LicenseID = %q, want %q", gc.LicenseID, "lic_dev")
	}
}

func TestVerifyAndParseGrantJWT_NoPublicKey_ProdMode(t *testing.T) {
	// Clear public key and ensure dev mode is OFF.
	prev := currentPublicKey()
	SetPublicKey(nil)
	t.Cleanup(func() { SetPublicKey(prev) })

	prevEnv := os.Getenv("PULSE_LICENSE_DEV_MODE")
	os.Setenv("PULSE_LICENSE_DEV_MODE", "false")
	t.Cleanup(func() { os.Setenv("PULSE_LICENSE_DEV_MODE", prevEnv) })

	jwt := makeUnsignedTestGrantJWT(t, &GrantClaims{
		LicenseID: "lic_prod",
		Tier:      "pro",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(72 * time.Hour).Unix(),
	})

	_, err := verifyAndParseGrantJWT(jwt)
	if !errors.Is(err, ErrNoPublicKey) {
		t.Errorf("expected ErrNoPublicKey, got: %v", err)
	}
}

func TestVerifyAndParseGrantJWT_MalformedJWT(t *testing.T) {
	setupTestPublicKey(t)

	tests := []struct {
		name   string
		jwt    string
		errMsg string
	}{
		{
			name:   "not a JWT",
			jwt:    "not-a-jwt",
			errMsg: "expected 3 parts",
		},
		{
			name:   "too few parts",
			jwt:    "header.payload",
			errMsg: "expected 3 parts",
		},
		{
			name:   "bad signature base64",
			jwt:    "header.payload.!!!invalid!!!",
			errMsg: "decode grant signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verifyAndParseGrantJWT(tt.jwt)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tt.errMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}
