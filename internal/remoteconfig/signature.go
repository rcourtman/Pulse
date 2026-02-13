package remoteconfig

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// trustedConfigPublicKeysPEM contains trusted Ed25519 public keys for config verification.
// In production builds, inject keys via ldflags to support rotation.
var trustedConfigPublicKeysPEM = strings.TrimSpace(`
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAlbXZQRx8jgMzwpXbbjOGcnA+9TG0lms/auxbPzY+Tdo=
-----END PUBLIC KEY-----
`)

// SignedConfigPayload is the canonical payload used for config signing.
type SignedConfigPayload struct {
	HostID          string
	IssuedAt        time.Time
	ExpiresAt       time.Time
	CommandsEnabled *bool
	Settings        map[string]interface{}
}

// DecodeEd25519PrivateKey decodes a base64-encoded Ed25519 private key or seed.
func DecodeEd25519PrivateKey(encoded string) (ed25519.PrivateKey, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("empty signing key")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 signing key: %w", err)
	}

	switch len(raw) {
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	default:
		return nil, fmt.Errorf("invalid signing key length: %d", len(raw))
	}
}

// SignConfigPayload signs the canonical config payload and returns a base64 signature.
func SignConfigPayload(payload SignedConfigPayload, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) == 0 {
		return "", errors.New("missing private key")
	}

	canonical, err := canonicalConfigPayload(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize config payload: %w", err)
	}

	signature := ed25519.Sign(privateKey, canonical)
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifyConfigPayloadSignature verifies a base64 signature against the trusted public keys.
func VerifyConfigPayloadSignature(payload SignedConfigPayload, signatureBase64 string) error {
	if signatureBase64 == "" {
		return errors.New("missing signature")
	}

	canonical, err := canonicalConfigPayload(payload)
	if err != nil {
		return fmt.Errorf("canonicalize config payload: %w", err)
	}

	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	keys, err := trustedConfigPublicKeys()
	if err != nil {
		return fmt.Errorf("load trusted config public keys: %w", err)
	}

	for _, key := range keys {
		if ed25519.Verify(key, canonical, signature) {
			return nil
		}
	}

	return errors.New("config signature verification failed against all trusted keys")
}

func canonicalConfigPayload(payload SignedConfigPayload) ([]byte, error) {
	type canonicalPayload struct {
		HostID          string          `json:"hostId"`
		IssuedAt        string          `json:"issuedAt"`
		ExpiresAt       string          `json:"expiresAt"`
		CommandsEnabled *bool           `json:"commandsEnabled,omitempty"`
		Settings        json.RawMessage `json:"settings,omitempty"`
	}

	var settings json.RawMessage
	if len(payload.Settings) > 0 {
		data, err := marshalSortedMap(payload.Settings)
		if err != nil {
			return nil, fmt.Errorf("marshal canonical settings: %w", err)
		}
		settings = data
	}

	canonical := canonicalPayload{
		HostID:          payload.HostID,
		IssuedAt:        payload.IssuedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt:       payload.ExpiresAt.UTC().Format(time.RFC3339Nano),
		CommandsEnabled: payload.CommandsEnabled,
		Settings:        settings,
	}

	data, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical payload: %w", err)
	}
	return data, nil
}

func trustedConfigPublicKeys() ([]ed25519.PublicKey, error) {
	raw := utils.GetenvTrim("PULSE_AGENT_CONFIG_PUBLIC_KEYS")
	if raw == "" {
		raw = strings.TrimSpace(trustedConfigPublicKeysPEM)
	}

	var keys []ed25519.PublicKey

	if strings.Contains(raw, "BEGIN PUBLIC KEY") {
		for {
			block, rest := pem.Decode([]byte(raw))
			if block == nil {
				break
			}
			raw = string(rest)
			if block.Type != "PUBLIC KEY" {
				continue
			}
			pub, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse trusted public key: %w", err)
			}
			edPub, ok := pub.(ed25519.PublicKey)
			if !ok {
				return nil, errors.New("trusted key is not an Ed25519 public key")
			}
			keys = append(keys, edPub)
		}
	} else {
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			decoded, err := base64.StdEncoding.DecodeString(part)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 public key: %w", err)
			}
			if len(decoded) == ed25519.PublicKeySize {
				keys = append(keys, ed25519.PublicKey(decoded))
				continue
			}

			pub, err := x509.ParsePKIXPublicKey(decoded)
			if err != nil {
				return nil, fmt.Errorf("failed to parse trusted public key: %w", err)
			}
			edPub, ok := pub.(ed25519.PublicKey)
			if !ok {
				return nil, errors.New("trusted key is not an Ed25519 public key")
			}
			keys = append(keys, edPub)
		}
	}

	if len(keys) == 0 {
		return nil, errors.New("no trusted config keys available")
	}

	return keys, nil
}

func marshalSortedMap(values map[string]interface{}) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteByte('{')

	for i, key := range keys {
		if i > 0 {
			builder.WriteByte(',')
		}
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("marshal settings key %q: %w", key, err)
		}
		valueJSON, err := marshalCanonicalValue(values[key])
		if err != nil {
			return nil, fmt.Errorf("marshal settings value for key %q: %w", key, err)
		}

		builder.Write(keyJSON)
		builder.WriteByte(':')
		builder.Write(valueJSON)
	}

	builder.WriteByte('}')
	return []byte(builder.String()), nil
}

func marshalCanonicalValue(value interface{}) ([]byte, error) {
	switch typed := value.(type) {
	case map[string]interface{}:
		data, err := marshalSortedMap(typed)
		if err != nil {
			return nil, fmt.Errorf("marshal nested map value: %w", err)
		}
		return data, nil
	case []interface{}:
		var builder strings.Builder
		builder.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				builder.WriteByte(',')
			}
			itemJSON, err := marshalCanonicalValue(item)
			if err != nil {
				return nil, fmt.Errorf("marshal list item %d: %w", i, err)
			}
			builder.Write(itemJSON)
		}
		builder.WriteByte(']')
		return []byte(builder.String()), nil
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("marshal scalar value (%T): %w", typed, err)
		}
		return data, nil
	}
}
