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
		"interval":      "45s",
		"enable_docker": true,
		"log_level":     "debug",
	}
	secondSettings := map[string]interface{}{
		"log_level":     "debug",
		"enable_docker": true,
		"interval":      "45s",
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

	withDifferentSettings, err := BuildDesiredConfigMetadata(&commandsEnabled, map[string]interface{}{
		"interval":      "30s",
		"enable_docker": true,
		"log_level":     "debug",
	})
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata changed settings: %v", err)
	}
	if withDifferentSettings.Hash == first.Hash {
		t.Fatalf("expected applied settings to affect desired config fingerprint")
	}
}

func TestBuildDesiredConfigMetadataIgnoresUnknownUnappliedSettings(t *testing.T) {
	commandsEnabled := true
	settings := map[string]interface{}{
		"interval":    "1m",
		"token_value": "secret-like-value",
	}
	withoutUnknown := map[string]interface{}{
		"interval": "1m",
	}

	got, err := BuildDesiredConfigMetadata(&commandsEnabled, settings)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata with unknown: %v", err)
	}
	want, err := BuildDesiredConfigMetadata(&commandsEnabled, withoutUnknown)
	if err != nil {
		t.Fatalf("BuildDesiredConfigMetadata without unknown: %v", err)
	}
	if got != want {
		t.Fatalf("expected unknown unapplied settings to be excluded from desired metadata, got %#v want %#v", got, want)
	}
}

func TestHasAppliedDesiredConfig(t *testing.T) {
	commandsEnabled := false
	tests := []struct {
		name            string
		commandsEnabled *bool
		settings        map[string]interface{}
		want            bool
	}{
		{name: "empty default", want: false},
		{
			name:     "unknown unapplied settings only",
			settings: map[string]interface{}{"token_value": "redacted"},
			want:     false,
		},
		{
			name:     "applied profile setting",
			settings: map[string]interface{}{"interval": "1m"},
			want:     true,
		},
		{
			name:            "command override",
			commandsEnabled: &commandsEnabled,
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAppliedDesiredConfig(tt.commandsEnabled, tt.settings); got != tt.want {
				t.Fatalf("HasAppliedDesiredConfig() = %v, want %v", got, tt.want)
			}
		})
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

func TestConfigSignatureUsesLegacyPayloadShape(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	commandsEnabled := true
	settings := map[string]interface{}{"interval": "1m"}
	issuedAt := time.Date(2026, 5, 13, 17, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(15 * time.Minute)
	payload := SignedConfigPayload{
		AgentID:         "host-1",
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
		CommandsEnabled: &commandsEnabled,
		Settings:        settings,
	}

	canonical, err := canonicalConfigPayload(payload)
	if err != nil {
		t.Fatalf("canonicalConfigPayload: %v", err)
	}
	expectedCanonical := `{"agentId":"host-1","issuedAt":"2026-05-13T17:00:00Z","expiresAt":"2026-05-13T17:15:00Z","commandsEnabled":true,"settings":{"interval":"1m"}}`
	if string(canonical) != expectedCanonical {
		t.Fatalf("canonical payload = %s, want %s", string(canonical), expectedCanonical)
	}

	sig, err := SignConfigPayload(payload, priv)
	if err != nil {
		t.Fatalf("SignConfigPayload: %v", err)
	}
	if err := VerifyConfigPayloadSignature(payload, sig); err != nil {
		t.Fatalf("VerifyConfigPayloadSignature: %v", err)
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
