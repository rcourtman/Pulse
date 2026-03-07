package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDecodeEd25519PrivateKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	_ = pub

	seed := priv.Seed()
	seedEncoded := base64.StdEncoding.EncodeToString(seed)
	gotSeedKey, err := DecodeEd25519PrivateKey(seedEncoded)
	if err != nil {
		t.Fatalf("DecodeEd25519PrivateKey(seed): %v", err)
	}
	if len(gotSeedKey) != ed25519.PrivateKeySize {
		t.Fatalf("seed decoded key length=%d, want %d", len(gotSeedKey), ed25519.PrivateKeySize)
	}

	fullEncoded := base64.StdEncoding.EncodeToString(priv)
	gotFullKey, err := DecodeEd25519PrivateKey(fullEncoded)
	if err != nil {
		t.Fatalf("DecodeEd25519PrivateKey(full): %v", err)
	}
	if string(gotFullKey) != string(priv) {
		t.Fatalf("decoded full private key mismatch")
	}
}

func TestSignAndVerifyTrialActivationToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignTrialActivationToken(priv, TrialActivationClaims{
		OrgID:        "default",
		Email:        "owner@example.com",
		InstanceHost: "pulse.example.com",
		ReturnURL:    "https://pulse.example.com/auth/trial-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	claims, err := VerifyTrialActivationToken(token, pub, "pulse.example.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyTrialActivationToken: %v", err)
	}
	if claims.OrgID != "default" {
		t.Fatalf("claims.OrgID=%q, want %q", claims.OrgID, "default")
	}
	if claims.InstanceHost != "pulse.example.com" {
		t.Fatalf("claims.InstanceHost=%q, want %q", claims.InstanceHost, "pulse.example.com")
	}
}

func TestVerifyTrialActivationToken_HostMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignTrialActivationToken(priv, TrialActivationClaims{
		OrgID:        "default",
		InstanceHost: "pulse-a.example.com",
		ReturnURL:    "https://pulse-a.example.com/auth/trial-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	_, err = VerifyTrialActivationToken(token, pub, "pulse-b.example.com", time.Now())
	if !errors.Is(err, ErrTrialActivationHostMismatch) {
		t.Fatalf("VerifyTrialActivationToken() error=%v, want %v", err, ErrTrialActivationHostMismatch)
	}
}

func TestValidateTrialActivationReturnURL(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		expectedHost string
		wantHost     string
		wantErr      error
	}{
		{
			name:         "https public host",
			raw:          "https://pulse.example.com/auth/trial-activate",
			expectedHost: "pulse.example.com",
			wantHost:     "pulse.example.com",
		},
		{
			name:     "http localhost allowed",
			raw:      "http://localhost:7655/auth/trial-activate",
			wantHost: "localhost",
		},
		{
			name:    "missing return url",
			raw:     "",
			wantErr: ErrTrialActivationReturnURLMissing,
		},
		{
			name:    "http public host rejected",
			raw:     "http://pulse.example.com/auth/trial-activate",
			wantErr: ErrTrialActivationReturnURLInvalid,
		},
		{
			name:    "query string rejected",
			raw:     "https://pulse.example.com/auth/trial-activate?token=x",
			wantErr: ErrTrialActivationReturnURLInvalid,
		},
		{
			name:         "host mismatch rejected",
			raw:          "https://pulse.example.com/auth/trial-activate",
			expectedHost: "other.example.com",
			wantErr:      ErrTrialActivationHostMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, err := ValidateTrialActivationReturnURL(tt.raw, tt.expectedHost)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateTrialActivationReturnURL() error=%v, want %v", err, tt.wantErr)
			}
			if gotHost != tt.wantHost {
				t.Fatalf("ValidateTrialActivationReturnURL() host=%q, want %q", gotHost, tt.wantHost)
			}
		})
	}
}

func TestSignTrialActivationToken_RequiresReturnURL(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	_, err = SignTrialActivationToken(priv, TrialActivationClaims{
		OrgID:        "default",
		InstanceHost: "pulse.example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if !errors.Is(err, ErrTrialActivationReturnURLMissing) {
		t.Fatalf("SignTrialActivationToken() error=%v, want %v", err, ErrTrialActivationReturnURLMissing)
	}
}

func TestTrialActivationPublicKey_EnvOverride(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	embeddedBefore := EmbeddedPublicKey
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	EmbeddedPublicKey = ""

	got, err := TrialActivationPublicKey()
	if err != nil {
		t.Fatalf("TrialActivationPublicKey: %v", err)
	}
	if string(got) != string(pub) {
		t.Fatalf("TrialActivationPublicKey mismatch")
	}
}
