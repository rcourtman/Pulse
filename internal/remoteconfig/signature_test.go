package remoteconfig

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"
)

func TestVerifyConfigPayloadSignature_WithEnvKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	payload := SignedConfigPayload{
		AgentID:         "host-1",
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(time.Minute),
		CommandsEnabled: nil,
		Settings: map[string]interface{}{
			"interval": "1m",
		},
	}

	sig, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatalf("SignConfigPayload: %v", err)
	}

	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	if err := VerifyConfigPayloadSignature(payload, sig); err != nil {
		t.Fatalf("VerifyConfigPayloadSignature: %v", err)
	}
}

func TestBuildDesiredConfigMetadataStableAndSensitiveToDecisions(t *testing.T) {
	commandsEnabled := true
	firstSettings := map[string]interface{}{
		"b": 2,
		"a": map[string]interface{}{
			"d": "x",
			"c": []interface{}{"y", float64(3)},
		},
	}
	secondSettings := map[string]interface{}{
		"a": map[string]interface{}{
			"c": []interface{}{"y", float64(3)},
			"d": "x",
		},
		"b": 2,
	}

	first, err := BuildDesiredConfigMetadata(&commandsEnabled, firstSettings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata first: %v", err)
	}
	second, err := BuildDesiredConfigMetadata(&commandsEnabled, secondSettings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata second: %v", err)
	}
	if first.Version != desiredConfigFingerprintVersion {
		t.Fatalf("unexpected version: %q", first.Version)
	}
	if first.Hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if first != second {
		t.Fatalf("expected stable metadata for reordered settings, got %#v and %#v", first, second)
	}

	disabled := false
	withDifferentCommandDecision, err := BuildDesiredConfigMetadata(&disabled, firstSettings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata disabled: %v", err)
	}
	if withDifferentCommandDecision.Hash == first.Hash {
		t.Fatalf("expected command decision to affect desired config fingerprint")
	}
}

func TestValidateDesiredConfigMetadata(t *testing.T) {
	commandsEnabled := true
	settings := map[string]interface{}{"interval": "1m"}
	metadata, err := BuildDesiredConfigMetadata(&commandsEnabled, settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata: %v", err)
	}

	if err := ValidateDesiredConfigMetadata(metadata, &commandsEnabled, settings); err != nil {
		t.Fatalf("ValidateDesiredConfigMetadata: %v", err)
	}

	tampered := metadata
	tampered.Hash = "sha256:0000"
	if err := ValidateDesiredConfigMetadata(tampered, &commandsEnabled, settings); err == nil {
		t.Fatalf("expected tampered desired config metadata to fail")
	}
}

func TestVerifyConfigPayloadSignatureCoversDesiredConfigMetadata(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	commandsEnabled := true
	settings := map[string]interface{}{"interval": "1m"}
	metadata, err := BuildDesiredConfigMetadata(&commandsEnabled, settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata: %v", err)
	}
	payload := SignedConfigPayload{
		AgentID:         "host-1",
		IssuedAt:        time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(time.Minute),
		CommandsEnabled: &commandsEnabled,
		Settings:        settings,
		DesiredConfig:   &metadata,
	}

	sig, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatalf("SignConfigPayload: %v", err)
	}
	if err := VerifyConfigPayloadSignature(payload, sig); err != nil {
		t.Fatalf("VerifyConfigPayloadSignature: %v", err)
	}

	tampered := metadata
	tampered.Hash = "sha256:0000"
	payload.DesiredConfig = &tampered
	if err := VerifyConfigPayloadSignature(payload, sig); err == nil {
		t.Fatalf("expected signature verification to fail after desired metadata tamper")
	}
}

func TestDecodeEd25519PrivateKey(t *testing.T) {
	if _, err := DecodeEd25519PrivateKey(""); err == nil {
		t.Fatal("expected error for empty key")
	}
	if _, err := DecodeEd25519PrivateKey("not-base64"); err == nil {
		t.Fatal("expected error for invalid base64")
	}

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	full := base64.StdEncoding.EncodeToString(priv)
	decoded, err := DecodeEd25519PrivateKey(full)
	if err != nil {
		t.Fatalf("DecodeEd25519PrivateKey full: %v", err)
	}
	if !bytes.Equal(decoded, priv) {
		t.Fatal("expected decoded private key to match")
	}

	seed := base64.StdEncoding.EncodeToString(priv.Seed())
	decoded, err = DecodeEd25519PrivateKey(seed)
	if err != nil {
		t.Fatalf("DecodeEd25519PrivateKey seed: %v", err)
	}
	if !bytes.Equal(decoded.Seed(), priv.Seed()) {
		t.Fatal("expected decoded seed to match")
	}
}

func TestTrustedConfigPublicKeys(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))
	keys, err := trustedConfigPublicKeys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d err=%v", len(keys), err)
	}

	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: raw})
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(block))
	keys, err = trustedConfigPublicKeys()
	if err != nil || len(keys) != 1 {
		t.Fatalf("expected 1 pem key, got %d err=%v", len(keys), err)
	}

	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", "not-base64")
	if _, err := trustedConfigPublicKeys(); err == nil {
		t.Fatal("expected error for invalid public key")
	}
}

func TestMarshalCanonicalValue(t *testing.T) {
	input := map[string]interface{}{
		"b": 1,
		"a": []interface{}{
			map[string]interface{}{"d": "x", "c": "y"},
			2,
		},
	}

	data, err := marshalCanonicalValue(input)
	if err != nil {
		t.Fatalf("marshalCanonicalValue error: %v", err)
	}
	expected := `{"a":[{"c":"y","d":"x"},2],"b":1}`
	if string(data) != expected {
		t.Fatalf("unexpected canonical JSON: %s", string(data))
	}
}
