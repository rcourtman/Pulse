package updatesignature

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

// EmbeddedTrustedPublicKeys is injected into release builds via ldflags.
// The value is expected to be a comma-separated list of base64-encoded Ed25519
// public keys or PKIX-encoded public keys.
var EmbeddedTrustedPublicKeys string

// HasTrustedPublicKeys reports whether a build embeds trusted update keys.
func HasTrustedPublicKeys() bool {
	return strings.TrimSpace(EmbeddedTrustedPublicKeys) != ""
}

// DecodePrivateKey decodes a base64-encoded Ed25519 private key or seed.
func DecodePrivateKey(encoded string) (ed25519.PrivateKey, error) {
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

// PublicKeyString returns the raw Ed25519 public key as base64.
func PublicKeyString(privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid signing key")
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", errors.New("failed to derive public key")
	}
	return base64.StdEncoding.EncodeToString(publicKey), nil
}

// SignBytes signs a blob and returns a base64-encoded Ed25519 signature.
func SignBytes(data []byte, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid signing key")
	}
	signature := ed25519.Sign(privateKey, data)
	return base64.StdEncoding.EncodeToString(signature), nil
}

// SignFile signs the file at path and returns a base64-encoded Ed25519 signature.
func SignFile(path string, privateKey ed25519.PrivateKey) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file for signing: %w", err)
	}
	return SignBytes(data, privateKey)
}

// VerifyBytes verifies a base64-encoded Ed25519 signature against the embedded trusted keys.
func VerifyBytes(data []byte, signatureBase64 string) error {
	signatureBase64 = strings.TrimSpace(signatureBase64)
	if signatureBase64 == "" {
		return errors.New("missing signature")
	}

	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	keys, err := trustedPublicKeys()
	if err != nil {
		return fmt.Errorf("load trusted update public keys: %w", err)
	}

	for _, key := range keys {
		if ed25519.Verify(key, data, signature) {
			return nil
		}
	}

	return errors.New("signature verification failed against all trusted keys")
}

// VerifyFile verifies a file against a base64-encoded Ed25519 signature.
func VerifyFile(path string, signatureBase64 string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file for verification: %w", err)
	}
	return VerifyBytes(data, signatureBase64)
}

func trustedPublicKeys() ([]ed25519.PublicKey, error) {
	raw := strings.TrimSpace(EmbeddedTrustedPublicKeys)
	if raw == "" {
		return nil, errors.New("no trusted update keys available")
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
			key, err := parsePublicKeyBytes(block.Bytes)
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
		}
	} else {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(part)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 public key: %w", err)
			}
			key, err := parsePublicKeyBytes(decoded)
			if err != nil {
				return nil, err
			}
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return nil, errors.New("no trusted update keys available")
	}

	return keys, nil
}

func parsePublicKeyBytes(raw []byte) (ed25519.PublicKey, error) {
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}

	publicKey, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse trusted public key: %w", err)
	}
	edPublicKey, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("trusted key is not an Ed25519 public key")
	}
	return edPublicKey, nil
}
