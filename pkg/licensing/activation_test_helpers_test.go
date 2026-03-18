package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"sync"
	"testing"
)

// testKeyPair holds a lazily-initialized Ed25519 key pair for tests.
var (
	testKeyOnce    sync.Once
	testPublicKey  ed25519.PublicKey
	testPrivateKey ed25519.PrivateKey
)

// testKeyPairInit generates the shared test key pair exactly once.
func testKeyPairInit() {
	testKeyOnce.Do(func() {
		var err error
		testPublicKey, testPrivateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			panic("generate test key pair: " + err.Error())
		}
	})
}

// setupTestPublicKey installs the test public key for signature verification
// and restores the previous key when the test completes.
func setupTestPublicKey(t *testing.T) {
	t.Helper()
	testKeyPairInit()

	prev := currentPublicKey()
	SetPublicKey(testPublicKey)
	t.Cleanup(func() { SetPublicKey(prev) })
}

// makeTestGrantJWT creates a properly signed test grant JWT.
// The test public key must be installed via setupTestPublicKey for
// verifyAndParseGrantJWT to accept the token.
func makeTestGrantJWT(t *testing.T, gc *GrantClaims) string {
	t.Helper()
	testKeyPairInit()

	payload, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal grant claims: %v", err)
	}
	return signTestJWT(t, payload, testPrivateKey)
}

// makeTestJWT creates a properly signed test JWT with the given raw payload.
func makeTestJWT(t *testing.T, payload string) string {
	t.Helper()
	testKeyPairInit()
	return signTestJWT(t, []byte(payload), testPrivateKey)
}

// makeUnsignedTestGrantJWT creates an unsigned test JWT (placeholder signature).
// Use only for tests that specifically exercise parseGrantJWTUnsafe.
func makeUnsignedTestGrantJWT(t *testing.T, gc *GrantClaims) string {
	t.Helper()
	payload, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal grant claims: %v", err)
	}
	return makeUnsignedTestJWT(t, string(payload))
}

// makeUnsignedTestJWT creates an unsigned test JWT with a placeholder signature.
func makeUnsignedTestJWT(t *testing.T, payload string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-sig"))
	return header + "." + payloadB64 + "." + sig
}

// signTestJWT signs payload bytes with the given private key and returns a JWT string.
func signTestJWT(t *testing.T, payload []byte, privKey ed25519.PrivateKey) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`))
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signedData := []byte(header + "." + payloadB64)
	sig := ed25519.Sign(privKey, signedData)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return header + "." + payloadB64 + "." + sigB64
}
