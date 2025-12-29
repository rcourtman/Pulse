package dockeragent

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// trustedPublicKeysPEM contains a list of trusted release public keys.
// In a real build, these would be injected via ldflags.
// Using a list allows for key rotation (start signing with new key, retire old key later).
var trustedPublicKeysPEM = []string{
	`-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAlbXZQRx8jgMzwpXbbjOGcnA+9TG0lms/auxbPzY+Tdo=
-----END PUBLIC KEY-----`,
}

// verifySignature checks if the provided binary data matches the signature
// using ANY of the trusted Ed25519 public keys.
func verifySignature(binaryData []byte, signatureBase64 string) error {
	if signatureBase64 == "" {
		return errors.New("missing signature")
	}

	// Decode the signature once
	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("invalid base64 signature: %w", err)
	}

	var lastErr error

	// Try each trusted key
	for _, keyPEM := range trustedPublicKeysPEM {
		block, _ := pem.Decode([]byte(keyPEM))
		if block == nil || block.Type != "PUBLIC KEY" {
			lastErr = errors.New("failed to decode one of the trusted public keys")
			continue
		}

		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse trusted public key: %w", err)
			continue
		}

		edPub, ok := pub.(ed25519.PublicKey)
		if !ok {
			lastErr = errors.New("trusted key is not an Ed25519 public key")
			continue
		}

		// If verification succeeds, we are valid!
		if ed25519.Verify(edPub, binaryData, sigBytes) {
			return nil
		}

		lastErr = errors.New("signature verification failed for key")
	}

	// If we're here, no key verified the signature
	if lastErr != nil {
		return fmt.Errorf("cryptographic signature verification failed against all trusted keys")
	}
	return errors.New("no trusted keys available for verification")
}

// verifyFileSignature reads the file and verifies its signature.
func verifyFileSignature(path string, signatureBase64 string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file for verification: %w", err)
	}
	return verifySignature(data, signatureBase64)
}
