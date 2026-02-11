package license

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// EmbeddedPublicKey is the production Ed25519 public key (base64 encoded).
// This should be set at build time via -ldflags or populated with the actual key.
// Example: go build -ldflags "-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=BASE64_KEY"
var EmbeddedPublicKey string = ""

func publicKeyFingerprint(key ed25519.PublicKey) string {
	if len(key) == 0 {
		return ""
	}
	sum := sha256.Sum256(key)
	return "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
}

// InitPublicKey initializes the public key for license validation.
// Priority:
//  1. PULSE_LICENSE_PUBLIC_KEY environment variable (base64 encoded)
//  2. EmbeddedPublicKey (set at compile time via -ldflags)
//  3. If PULSE_LICENSE_DEV_MODE=true, skip validation (development only)
//
// Call this during application startup before any license operations.
func InitPublicKey() {
	// Priority 1: Environment variable
	if envKey := os.Getenv("PULSE_LICENSE_PUBLIC_KEY"); envKey != "" {
		key, err := decodePublicKey(envKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode PULSE_LICENSE_PUBLIC_KEY, trying embedded key")
			// Fall through to try embedded key instead of returning
		} else {
			SetPublicKey(key)
			log.Info().
				Str("source", "environment").
				Str("fingerprint", publicKeyFingerprint(key)).
				Msg("License public key loaded")
			return
		}
	}

	// Priority 2: Embedded key (set at compile time)
	if EmbeddedPublicKey != "" {
		key, err := decodePublicKey(EmbeddedPublicKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode embedded public key")
		} else {
			SetPublicKey(key)
			log.Info().
				Str("source", "embedded").
				Str("fingerprint", publicKeyFingerprint(key)).
				Msg("License public key loaded")
			return
		}
	}

	// No key available
	if os.Getenv("PULSE_LICENSE_DEV_MODE") == "true" {
		log.Warn().Msg("License validation running in DEV MODE - signatures not verified")
	} else {
		log.Warn().Msg("No license public key configured - license activation will fail")
	}
}

// decodePublicKey decodes a base64-encoded Ed25519 public key.
func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	// Remove any whitespace
	encoded = strings.TrimSpace(encoded)

	// Try standard base64 first, then URL-safe
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
	}

	if len(decoded) != ed25519.PublicKeySize {
		return nil, ErrMalformedLicense
	}

	return ed25519.PublicKey(decoded), nil
}
