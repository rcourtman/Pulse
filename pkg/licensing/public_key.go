package licensing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

var ErrMalformedPublicKey = errors.New("malformed license public key")

// EmbeddedPublicKey is the production Ed25519 public key (base64 encoded).
// This is typically set at build time via -ldflags:
//
//	-X github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey=BASE64_KEY
var EmbeddedPublicKey string

// PublicKeyFingerprint returns an SHA256 fingerprint for logging.
func PublicKeyFingerprint(key ed25519.PublicKey) string {
	if len(key) == 0 {
		return ""
	}
	sum := sha256.Sum256(key)
	return "SHA256:" + base64.StdEncoding.EncodeToString(sum[:])
}

// InitEmbeddedPublicKey initializes the runtime license public key using
// EmbeddedPublicKey and standard environment precedence rules.
func InitEmbeddedPublicKey() {
	InitPublicKey(EmbeddedPublicKey, isLicenseValidationDevMode(), SetPublicKey)
}

// InitPublicKey initializes the runtime license public key.
// Priority:
// 1. PULSE_LICENSE_PUBLIC_KEY environment variable (base64 encoded)
// 2. embeddedPublicKey parameter (set at compile time via -ldflags)
// 3. If devMode=true, skip validation (development only)
func InitPublicKey(embeddedPublicKey string, devMode bool, setPublicKey func(ed25519.PublicKey)) {
	if setPublicKey == nil {
		log.Error().Msg("license public key init skipped: setPublicKey callback is nil")
		return
	}

	// Priority 1: Environment variable
	if envKey := os.Getenv("PULSE_LICENSE_PUBLIC_KEY"); envKey != "" {
		key, err := DecodePublicKey(envKey)
		if err != nil {
			log.Error().Err(err).Msg("failed to decode PULSE_LICENSE_PUBLIC_KEY, trying embedded key")
			// Fall through to try embedded key instead of returning
		} else {
			setPublicKey(key)
			log.Info().
				Str("source", "environment").
				Str("fingerprint", PublicKeyFingerprint(key)).
				Msg("license public key loaded")
			return
		}
	}

	// Priority 2: Embedded key (set at compile time)
	if embeddedPublicKey != "" {
		key, err := DecodePublicKey(embeddedPublicKey)
		if err != nil {
			log.Error().Err(err).Msg("failed to decode embedded public key")
		} else {
			setPublicKey(key)
			log.Info().
				Str("source", "embedded").
				Str("fingerprint", PublicKeyFingerprint(key)).
				Msg("license public key loaded")
			return
		}
	}

	// No key available
	if devMode {
		log.Warn().Msg("License validation running in DEV MODE - signatures not verified")
	} else {
		log.Warn().Msg("no license public key configured - license activation will fail")
	}
}

// DecodePublicKey decodes a base64-encoded Ed25519 public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
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
		return nil, ErrMalformedPublicKey
	}

	return ed25519.PublicKey(decoded), nil
}
