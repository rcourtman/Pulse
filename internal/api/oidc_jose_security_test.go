package api

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Keep the upstream resource bound, rather than raising a process-global limit
// to make an IdP compatible. The boundary is exercised without generating an
// oversized private key or doing an expensive RSA verification.
func TestOIDCJOSEJWKModulusBound(t *testing.T) {
	if jose.MaxRSAModulusBits != 4096 {
		t.Fatalf("RSA JWK modulus limit = %d, want upstream default 4096", jose.MaxRSAModulusBits)
	}
	for _, bits := range []int{2048, 4096, 4097, 8192} {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			n := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
			n.Add(n, big.NewInt(1))
			body, err := json.Marshal(map[string]string{
				"kty": "RSA", "e": "AQAB", "kid": "bounded", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(n.Bytes()),
			})
			if err != nil {
				t.Fatal(err)
			}
			var key jose.JSONWebKey
			err = json.Unmarshal(body, &key)
			if bits > 4096 {
				if err == nil {
					t.Fatalf("parsed out-of-range %d-bit RSA JWK", bits)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse supported %d-bit RSA JWK: %v", bits, err)
			}
			pub, ok := key.Key.(*rsa.PublicKey)
			if !ok || pub.N.BitLen() != bits {
				t.Fatalf("supported JWK changed: %T %+v", key.Key, key)
			}
		})
	}
}

// Exercise Pulse's real discovery and remote-JWKS consumer, not just jose's API.
// No verifier skips, algorithm overrides or global library settings are used.
func TestOIDCServiceJOSECompatibilityAndRejection(t *testing.T) {
	rsa2048, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rsa4096, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, ed, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		key        crypto.Signer
		alg        string
		mutate     func(map[string]any)
		badSig     bool
		wantReject bool
	}{
		{name: "RSA2048", key: rsa2048, alg: "RS256"},
		{name: "RSA4096", key: rsa4096, alg: "RS256"},
		{name: "P256", key: ec, alg: "ES256"},
		{name: "Ed25519", key: ed, alg: "EdDSA"},
		{name: "bad signature", key: rsa2048, alg: "RS256", badSig: true, wantReject: true},
		{name: "unsigned algorithm", key: rsa2048, alg: "none", wantReject: true},
		{name: "wrong issuer", key: rsa2048, alg: "RS256", mutate: func(c map[string]any) { c["iss"] = "https://other.invalid" }, wantReject: true},
		{name: "wrong audience", key: rsa2048, alg: "RS256", mutate: func(c map[string]any) { c["aud"] = "another-client" }, wantReject: true},
		{name: "expired", key: rsa2048, alg: "RS256", mutate: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, wantReject: true},
		{name: "P256 cannot use ES384", key: ec, alg: "ES384", wantReject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const kid = "provider key with spaces"
			jwk := jose.JSONWebKey{Key: tc.key.Public(), KeyID: kid, Use: "sig"}
			jwks, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
			if err != nil {
				t.Fatal(err)
			}
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					base := "http://" + r.Host
					_ = json.NewEncoder(w).Encode(map[string]any{
						"issuer": base, "authorization_endpoint": base + "/authorize",
						"token_endpoint": base + "/token", "jwks_uri": base + "/jwks",
						"id_token_signing_alg_values_supported": []string{"RS256", "ES256", "ES384", "EdDSA"},
					})
				case "/jwks":
					_, _ = w.Write(jwks)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			service, err := NewOIDCService(t.Context(), &config.OIDCConfig{
				Enabled: true, IssuerURL: server.URL, ClientID: "pulse-client",
				RedirectURL: "http://pulse.invalid/api/oidc/callback", Scopes: []string{"openid"},
			})
			if err != nil {
				t.Fatalf("initialize Pulse OIDC: %v", err)
			}
			defer service.Stop()
			claims := map[string]any{
				"iss": server.URL, "aud": "pulse-client", "sub": "pulse user",
				"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
			}
			if tc.mutate != nil {
				tc.mutate(claims)
			}
			token := signOIDCJOSEFixture(t, tc.key, tc.alg, kid, claims, tc.badSig)
			verified, err := service.verifier.Verify(t.Context(), token)
			if tc.wantReject {
				if err == nil {
					t.Fatalf("OIDC accepted %s", tc.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("verify supported provider token: %v", err)
			}
			if verified.Subject != "pulse user" {
				t.Fatalf("token parser rewrote subject: %q", verified.Subject)
			}
		})
	}
}

// Stdlib signing is independent of the upgraded library under test. In the
// curve-mismatch case the signature is mathematically valid on P256/SHA384;
// JOSE must still reject that key/algorithm combination.
func signOIDCJOSEFixture(t *testing.T, key crypto.Signer, alg, kid string, claims map[string]any, badSig bool) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": alg, "kid": kid})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	input := enc(header) + "." + enc(payload)
	digest := sha256.Sum256([]byte(input))
	var signature []byte
	switch k := key.(type) {
	case *rsa.PrivateKey:
		signature, err = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, digest[:])
	case *ecdsa.PrivateKey:
		hash := digest[:]
		if alg == "ES384" {
			sum := sha512.Sum384([]byte(input))
			hash = sum[:]
		}
		var r, s *big.Int
		r, s, err = ecdsa.Sign(rand.Reader, k, hash)
		if err == nil {
			n := (k.Curve.Params().BitSize + 7) / 8
			signature = make([]byte, 2*n)
			r.FillBytes(signature[:n])
			s.FillBytes(signature[n:])
		}
	case ed25519.PrivateKey:
		signature = ed25519.Sign(k, []byte(input))
	default:
		t.Fatalf("unsupported fixture signer %T", key)
	}
	if err != nil {
		t.Fatalf("sign fixture: %v", err)
	}
	if badSig {
		signature[0] ^= 1
	}
	return input + "." + enc(signature)
}
