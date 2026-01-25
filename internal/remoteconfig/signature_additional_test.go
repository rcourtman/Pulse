package remoteconfig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestDecodeEd25519PrivateKeyInvalidLength(t *testing.T) {
	raw := []byte("short")
	encoded := base64.StdEncoding.EncodeToString(raw)
	if _, err := DecodeEd25519PrivateKey(encoded); err == nil {
		t.Fatalf("expected invalid length error")
	}
}

func TestSignConfigPayloadMissingKey(t *testing.T) {
	if _, err := SignConfigPayload(SignedConfigPayload{}, nil); err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestSignConfigPayloadInvalidSettings(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	payload := SignedConfigPayload{
		HostID:    "host-1",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
		Settings:  map[string]interface{}{"bad": func() {}},
	}
	if _, err := SignConfigPayload(payload, priv); err == nil {
		t.Fatalf("expected settings error")
	}
}

func TestVerifyConfigPayloadSignatureInvalidBase64(t *testing.T) {
	payload := SignedConfigPayload{
		HostID:    "host",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := VerifyConfigPayloadSignature(payload, "not-base64"); err == nil {
		t.Fatalf("expected base64 error")
	}
}

func TestVerifyConfigPayloadSignatureMissing(t *testing.T) {
	payload := SignedConfigPayload{
		HostID:    "host",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := VerifyConfigPayloadSignature(payload, ""); err == nil {
		t.Fatalf("expected missing signature error")
	}
}

func TestTrustedConfigPublicKeysErrors(t *testing.T) {
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", "")
	if keys, err := trustedConfigPublicKeys(); err != nil || len(keys) == 0 {
		t.Fatalf("expected default keys, got %d err=%v", len(keys), err)
	}

	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", ",")
	if _, err := trustedConfigPublicKeys(); err == nil {
		t.Fatalf("expected no trusted keys error")
	}

	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("nope")})
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(block))
	if _, err := trustedConfigPublicKeys(); err == nil {
		t.Fatalf("expected no trusted keys error for wrong PEM type")
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	rsaPub, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(rsaPub))
	if _, err := trustedConfigPublicKeys(); err == nil || !strings.Contains(err.Error(), "Ed25519") {
		t.Fatalf("expected ed25519 error, got %v", err)
	}

	garbage := base64.StdEncoding.EncodeToString([]byte("garbage"))
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", garbage)
	if _, err := trustedConfigPublicKeys(); err == nil {
		t.Fatalf("expected parse error for garbage")
	}

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	block = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: raw})
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(block))
	if keys, err := trustedConfigPublicKeys(); err != nil || len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d err=%v", len(keys), err)
	}

	badBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("bad")})
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(badBlock))
	if _, err := trustedConfigPublicKeys(); err == nil {
		t.Fatalf("expected parse error for invalid pem")
	}

	rsaBlock := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: rsaPub})
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(rsaBlock))
	if _, err := trustedConfigPublicKeys(); err == nil || !strings.Contains(err.Error(), "Ed25519") {
		t.Fatalf("expected pem ed25519 error, got %v", err)
	}

	multi := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("nope")}), block...)
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", string(multi))
	if keys, err := trustedConfigPublicKeys(); err != nil || len(keys) != 1 {
		t.Fatalf("expected 1 key from multi pem, got %d err=%v", len(keys), err)
	}
}

func TestVerifyConfigPayloadSignatureFailure(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pub))

	payload := SignedConfigPayload{
		HostID:    "host-1",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
		Settings:  map[string]interface{}{"key": "value"},
	}
	if err := VerifyConfigPayloadSignature(payload, base64.StdEncoding.EncodeToString([]byte("bad"))); err == nil {
		t.Fatalf("expected verification failure")
	}
}

func TestCanonicalConfigPayloadEmptySettings(t *testing.T) {
	payload := SignedConfigPayload{
		HostID:    "host-1",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	data, err := canonicalConfigPayload(payload)
	if err != nil {
		t.Fatalf("canonicalConfigPayload error: %v", err)
	}
	if !strings.Contains(string(data), `"hostId":"host-1"`) {
		t.Fatalf("unexpected payload: %s", string(data))
	}
}

func TestMarshalSortedMapEmptyAndInvalid(t *testing.T) {
	if data, err := marshalSortedMap(map[string]interface{}{}); err != nil || data != nil {
		t.Fatalf("expected nil for empty map")
	}

	if _, err := marshalSortedMap(map[string]interface{}{"bad": func() {}}); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestVerifyConfigPayloadSignatureCanonicalError(t *testing.T) {
	payload := SignedConfigPayload{
		HostID:    "host",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
		Settings:  map[string]interface{}{"bad": func() {}},
	}
	if err := VerifyConfigPayloadSignature(payload, base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatalf("expected canonical error")
	}
}

func TestMarshalCanonicalValueSliceError(t *testing.T) {
	if _, err := marshalCanonicalValue([]interface{}{func() {}}); err == nil {
		t.Fatalf("expected slice marshal error")
	}
}

func TestVerifyConfigPayloadSignatureTrustedKeysError(t *testing.T) {
	payload := SignedConfigPayload{
		HostID:    "host",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", "not-base64")
	if err := VerifyConfigPayloadSignature(payload, base64.StdEncoding.EncodeToString([]byte("sig"))); err == nil {
		t.Fatalf("expected trusted key error")
	}
}

func TestTrustedConfigPublicKeysPKIXEd25519(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	t.Setenv("PULSE_AGENT_CONFIG_PUBLIC_KEYS", base64.StdEncoding.EncodeToString(pkix))
	if keys, err := trustedConfigPublicKeys(); err != nil || len(keys) != 1 {
		t.Fatalf("expected 1 pkix key, got %d err=%v", len(keys), err)
	}
}
