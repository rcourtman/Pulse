package licensing

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// makeTestGrantJWT creates a test JWT with the given grant claims.
// The header and signature are placeholder values — only the payload matters.
func makeTestGrantJWT(t *testing.T, gc *GrantClaims) string {
	t.Helper()
	payload, err := json.Marshal(gc)
	if err != nil {
		t.Fatalf("marshal grant claims: %v", err)
	}
	return makeTestJWT(t, string(payload))
}

// makeTestJWT creates a test JWT with the given raw payload string.
func makeTestJWT(t *testing.T, payload string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA"}`))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-sig"))
	return header + "." + payloadB64 + "." + sig
}
