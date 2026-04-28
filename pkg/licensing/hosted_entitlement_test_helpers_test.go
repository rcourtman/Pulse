package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func installHostedEntitlementPublicKeyForTest(t *testing.T, pub ed25519.PublicKey) {
	t.Helper()

	encoded := base64.StdEncoding.EncodeToString(pub)
	embeddedBefore := EmbeddedPublicKey
	EmbeddedPublicKey = encoded
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	t.Setenv(HostedEntitlementPublicKeyEnvVar, encoded)
}
