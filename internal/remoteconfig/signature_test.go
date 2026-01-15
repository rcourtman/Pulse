package remoteconfig

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"
)

func TestVerifyConfigPayloadSignature_WithEnvKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	payload := SignedConfigPayload{
		HostID:          "host-1",
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
