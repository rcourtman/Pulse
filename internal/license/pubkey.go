package license

import (
	"crypto/ed25519"
	"strings"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// EmbeddedPublicKey is the production Ed25519 public key (base64 encoded).
// This should be set at build time via -ldflags or populated with the actual key.
// Example: go build -ldflags "-X github.com/rcourtman/pulse-go-rewrite/internal/license.EmbeddedPublicKey=BASE64_KEY"
// Preferred path: github.com/rcourtman/pulse-go-rewrite/pkg/licensing.EmbeddedPublicKey
var EmbeddedPublicKey string = ""

func init() {
	// Compatibility bridge while ldflags transition from internal/license -> pkg/licensing.
	if strings.TrimSpace(pkglicensing.EmbeddedPublicKey) == "" && strings.TrimSpace(EmbeddedPublicKey) != "" {
		pkglicensing.EmbeddedPublicKey = EmbeddedPublicKey
	}
	if strings.TrimSpace(EmbeddedPublicKey) == "" && strings.TrimSpace(pkglicensing.EmbeddedPublicKey) != "" {
		EmbeddedPublicKey = pkglicensing.EmbeddedPublicKey
	}
}

func publicKeyFingerprint(key ed25519.PublicKey) string {
	return pkglicensing.PublicKeyFingerprint(key)
}

// InitPublicKey initializes the public key for license validation.
func InitPublicKey() {
	embedded := strings.TrimSpace(EmbeddedPublicKey)
	if embedded == "" {
		embedded = strings.TrimSpace(pkglicensing.EmbeddedPublicKey)
	}
	pkglicensing.InitPublicKey(embedded, isLicenseValidationDevMode(), SetPublicKey)
}

// decodePublicKey decodes a base64-encoded Ed25519 public key.
func decodePublicKey(encoded string) (ed25519.PublicKey, error) {
	return pkglicensing.DecodePublicKey(encoded)
}
