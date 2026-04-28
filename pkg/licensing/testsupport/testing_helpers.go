// Package testsupport contains license fixtures for cross-package tests.
//
// Production code must not import this package. It intentionally lives outside
// pkg/licensing so release-tag tests can build without exposing test signers on
// the production licensing package API.
package testsupport

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// GenerateLicenseForTesting creates a legacy JWT-shaped test license.
func GenerateLicenseForTesting(email string, tier licensing.Tier, expiresIn time.Duration) (string, error) {
	claims := licensing.Claims{
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

// GenerateGrantJWTForTesting creates a signed grant JWT and returns the
// matching public key so tests can install it before verification.
func GenerateGrantJWTForTesting(claims licensing.GrantClaims) (string, ed25519.PublicKey, error) {
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
