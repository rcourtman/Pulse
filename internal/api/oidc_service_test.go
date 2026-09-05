package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestOIDCServiceVerifiesTokenWhenProviderHasUnsupportedKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	modulus := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
	jwks := fmt.Sprintf(`{
		"keys": [
			{
				"kty": "OKP",
				"crv": "Ed448",
				"kid": "unsupported",
				"use": "sig",
				"x": "gH1eRK-6hW6ZoAy2k11U4L5uaIaMaZTMCf1cAbsxsYLvTqV2-TQG1PNyLOrhZkMyzUJulMc1wAfH"
			},
			{
				"kty": "RSA",
				"kid": "supported",
				"use": "sig",
				"alg": "RS256",
				"n": %q,
				"e": "AQAB"
			}
		]
	}`, modulus)

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = fmt.Fprintf(w, `{
				"issuer": %q,
				"authorization_endpoint": %q,
				"token_endpoint": %q,
				"jwks_uri": %q
			}`, baseURL, baseURL+"/authorize", baseURL+"/token", baseURL+"/jwks")
		case "/jwks":
			_, _ = w.Write([]byte(jwks))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := NewOIDCService(t.Context(), &config.OIDCConfig{
		Enabled:     true,
		IssuerURL:   server.URL,
		ClientID:    "pulse-client",
		RedirectURL: "http://pulse.example/api/oidc/callback",
		Scopes:      []string{"openid"},
	})
	if err != nil {
		t.Fatalf("initialize Pulse OIDC service: %v", err)
	}
	defer service.stateStore.Stop()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"supported"}`))
	payload := []byte(fmt.Sprintf(`{
		"iss": %q,
		"sub": "pulse-user",
		"aud": "pulse-client",
		"iat": %d,
		"exp": %d
	}`, server.URL, time.Now().Add(-time.Minute).Unix(), time.Now().Add(time.Hour).Unix()))
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := header + "." + encodedPayload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)

	idToken, err := service.verifier.Verify(t.Context(), token)
	if err != nil {
		t.Fatalf("verify ID token against mixed provider key set: %v", err)
	}
	if idToken.Subject != "pulse-user" {
		t.Fatalf("verified subject = %q, want pulse-user", idToken.Subject)
	}
}

func TestNewOIDCHTTPClient_WithCustomCABundle(t *testing.T) {
	// Self-signed TLS server should be rejected by default client
	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Default trust store should fail
	defaultClient, _, err := newOIDCHTTPClient("")
	if err != nil {
		t.Fatalf("failed to build default client: %v", err)
	}
	defaultClient.Timeout = testHTTPTimeout
	if _, err := defaultClient.Get(server.URL); err == nil {
		t.Fatalf("expected self-signed cert failure, got nil error")
	} else {
		var certErr x509.UnknownAuthorityError
		if !errors.As(err, &certErr) {
			t.Fatalf("expected unknown authority error, got: %v", err)
		}
	}

	// Write server certificate to a temp CA bundle
	tempFile, err := os.CreateTemp("", "oidc-ca-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	certDER := server.TLS.Certificates[0].Certificate[0]
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse server certificate: %v", err)
	}
	if err := pem.Encode(tempFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		t.Fatalf("failed to write temp CA bundle: %v", err)
	}

	// Client with custom CA bundle should succeed
	customClient, _, err := newOIDCHTTPClient(tempFile.Name())
	if err != nil {
		t.Fatalf("failed to build custom client: %v", err)
	}
	customClient.Timeout = testHTTPTimeout
	if resp, err := customClient.Get(server.URL); err != nil {
		t.Fatalf("expected successful GET with custom CA bundle, got error: %v", err)
	} else {
		resp.Body.Close()
	}
}

func TestNewOIDCHTTPClient_InvalidBundle(t *testing.T) {
	client, _, err := newOIDCHTTPClient("/nonexistent/oidc-ca.pem")
	if err == nil {
		t.Fatalf("expected error for missing CA bundle, got client: %+v", client)
	}
}

func TestNewOIDCHTTPClient_InvalidPEMData(t *testing.T) {
	// Create a temp file with invalid PEM data
	tempFile, err := os.CreateTemp("", "invalid-ca-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write invalid data that's not a valid PEM certificate
	if _, err := tempFile.WriteString("not a valid PEM certificate"); err != nil {
		t.Fatalf("failed to write invalid data: %v", err)
	}
	tempFile.Close()

	client, _, err := newOIDCHTTPClient(tempFile.Name())
	if err == nil {
		t.Fatalf("expected error for invalid PEM data, got client: %+v", client)
	}
	if !strings.Contains(err.Error(), "does not contain any certificates") {
		t.Errorf("expected 'does not contain any certificates' error, got: %v", err)
	}
}

func TestNewOIDCHTTPClient_BlocksCrossOriginRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	client, _, err := newOIDCHTTPClient("")
	if err != nil {
		t.Fatalf("newOIDCHTTPClient() error = %v", err)
	}
	client.Timeout = testHTTPTimeout

	resp, err := client.Get(origin.URL)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "same origin") {
		t.Fatalf("expected same-origin redirect rejection, got %v", err)
	}
}

const testHTTPTimeout = 2 * time.Second
