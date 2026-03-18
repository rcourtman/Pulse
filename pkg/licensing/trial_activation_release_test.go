//go:build release

package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestTrialActivationPublicKey_EnvOverrideHostedModeRelease(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_HOSTED_MODE", "true")
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(publicKey))

	got, err := TrialActivationPublicKey()
	if err != nil {
		t.Fatalf("TrialActivationPublicKey: %v", err)
	}
	if string(got) != string(publicKey) {
		t.Fatal("public key mismatch")
	}
}

func TestTrialActivationPublicKey_EnvOverrideRejectedWithoutHostedModeRelease(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_HOSTED_MODE", "false")
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(publicKey))

	_, err = TrialActivationPublicKey()
	if err != ErrTrialActivationPublicKeyMissing {
		t.Fatalf("TrialActivationPublicKey error = %v, want %v", err, ErrTrialActivationPublicKeyMissing)
	}
}
