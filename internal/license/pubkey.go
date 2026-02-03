package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// EmbeddedPublicKey is the production Ed25519 public key (base64 encoded).
// This should be set at build time via -ldflags or populated with the actual key.
// Example: go build -ldflags "-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=BASE64_KEY"
var EmbeddedPublicKey string = ""

// EmbeddedLegacyPublicKey is the previous production public key (base64 encoded).
// Used for dual-key verification during key rotation to validate old licenses.
// Set at build time via -ldflags alongside EmbeddedPublicKey.
var EmbeddedLegacyPublicKey string = ""

// InitPublicKey initializes the public key(s) for license validation.
// Primary key priority:
//  1. PULSE_LICENSE_PUBLIC_KEY environment variable (base64 encoded)
//  2. EmbeddedPublicKey (set at compile time via -ldflags)
//
// Legacy key priority (for dual-key verification during key rotation):
//  1. PULSE_LICENSE_LEGACY_PUBLIC_KEY environment variable (base64 encoded)
//  2. EmbeddedLegacyPublicKey (set at compile time via -ldflags)
//
// If PULSE_LICENSE_DEV_MODE=true, skip validation (development only).
// Call this during application startup before any license operations.
func InitPublicKey() {
	devMode := os.Getenv("PULSE_LICENSE_DEV_MODE") == "true"
	primaryLoaded := false
	legacyLoaded := false

	// Load primary public key
	if envKey := os.Getenv("PULSE_LICENSE_PUBLIC_KEY"); envKey != "" {
		key, err := decodePublicKey(envKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode PULSE_LICENSE_PUBLIC_KEY")
		} else {
			SetPublicKey(key)
			log.Info().Msg("License public key loaded from environment")
			primaryLoaded = true
		}
	}
	if !primaryLoaded && EmbeddedPublicKey != "" {
		key, err := decodePublicKey(EmbeddedPublicKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode embedded public key")
		} else {
			SetPublicKey(key)
			log.Info().Msg("License public key loaded from embedded key")
			primaryLoaded = true
		}
	}

	// Load legacy public key (for dual-key verification)
	if envKey := os.Getenv("PULSE_LICENSE_LEGACY_PUBLIC_KEY"); envKey != "" {
		key, err := decodePublicKey(envKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode PULSE_LICENSE_LEGACY_PUBLIC_KEY")
		} else {
			SetLegacyPublicKey(key)
			log.Info().Msg("Legacy license public key loaded from environment")
			legacyLoaded = true
		}
	}
	if !legacyLoaded && EmbeddedLegacyPublicKey != "" {
		key, err := decodePublicKey(EmbeddedLegacyPublicKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to decode embedded legacy public key")
		} else {
			SetLegacyPublicKey(key)
			log.Info().Msg("Legacy license public key loaded from embedded key")
			legacyLoaded = true
		}
	}

	// Log status
	if primaryLoaded && legacyLoaded {
		log.Info().Msg("Dual-key license verification enabled (primary + legacy)")
	} else if primaryLoaded {
		log.Info().Msg("Single-key license verification enabled")
	} else if legacyLoaded {
		log.Warn().Msg("Only legacy key loaded - new licenses will not validate")
	} else if devMode {
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
