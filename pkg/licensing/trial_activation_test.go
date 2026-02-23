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
