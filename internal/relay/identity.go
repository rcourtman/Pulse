package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// GenerateIdentityKeyPair creates a new Ed25519 keypair for instance identity
// and returns the base64-encoded private key, public key, and a colon-separated
// uppercase hex fingerprint of the public key.
func GenerateIdentityKeyPair() (privateKeyB64, publicKeyB64, fingerprint string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	privateKeyB64 = base64.StdEncoding.EncodeToString(priv)
	publicKeyB64 = base64.StdEncoding.EncodeToString(pub)

	fingerprint, err = ComputeFingerprint(publicKeyB64)
	if err != nil {
		return "", "", "", fmt.Errorf("compute fingerprint: %w", err)
	}

	return privateKeyB64, publicKeyB64, fingerprint, nil
}

// ComputeFingerprint computes a SHA256 fingerprint from a base64-encoded
// Ed25519 public key. The result is formatted as colon-separated uppercase
// hex pairs, e.g. "AB:CD:EF:12:...".
func ComputeFingerprint(publicKeyB64 string) (string, error) {
	pubBytes, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return "", fmt.Errorf("invalid public key length: got %d, want %d", len(pubBytes), ed25519.PublicKeySize)
	}

	hash := sha256.Sum256(pubBytes)

	parts := make([]string, len(hash))
	for i, b := range hash {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":"), nil
}
