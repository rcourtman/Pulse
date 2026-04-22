//go:build !release

package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// GenerateLicenseForTesting creates a test license (DO NOT USE IN PRODUCTION).
// This helper is intentionally excluded from release builds.
func GenerateLicenseForTesting(email string, tier Tier, expiresIn time.Duration) (string, error) {
	claims := Claims{
		LicenseID: fmt.Sprintf("test_%d", time.Now().UnixNano()),
		Email:     email,
		Tier:      tier,
		IssuedAt:  time.Now().Unix(),
	}
	if expiresIn > 0 {
		claims.ExpiresAt = time.Now().Add(expiresIn).Unix()
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal test license claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := base64.RawURLEncoding.EncodeToString([]byte("test-signature-not-valid"))

	return header + "." + payload + "." + signature, nil
}

// GenerateGrantJWTForTesting creates a signed grant JWT and returns the matching
// public key so tests can install it via SetPublicKey before verification.
// This helper is intentionally excluded from release builds.
func GenerateGrantJWTForTesting(claims GrantClaims) (string, ed25519.PublicKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("generate grant test key pair: %w", err)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", nil, fmt.Errorf("marshal grant test claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signedData := []byte(header + "." + payload)
	signature := ed25519.Sign(privateKey, signedData)

	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature), publicKey, nil
}
