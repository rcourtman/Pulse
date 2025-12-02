package api

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewOIDCHTTPClient_WithCustomCABundle(t *testing.T) {
	// Self-signed TLS server should be rejected by default client
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

const testHTTPTimeout = 2 * time.Second
