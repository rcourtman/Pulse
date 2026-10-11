package tlsutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Each fixture owns a distinct parseable leaf, local socket and generated key.
// It serves no provider credentials and records whether capture sent any HTTP.
func fingerprintAuthorityFixture(t *testing.T, network, address string) (*httptest.Server, string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen(network, address)
	if err != nil {
		t.Fatal(err) // IPv6/default-port support is a requirement, not a skipped proof.
	}
	t.Cleanup(func() { listener.Close() })
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	leaf, err := x509.CreateCertificate(rand.Reader, template, template, public, key)
	if err != nil {
		t.Fatal(err)
	}
	requests := &atomic.Int32{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	server.Listener.Close()
	server.Listener = listener
	server.TLS = &tls.Config{MinVersion: minimumTLSVersion, Certificates: []tls.Certificate{{Certificate: [][]byte{leaf}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)
	digest := sha256.Sum256(leaf)
	return server, hex.EncodeToString(digest[:]), requests
}

func TestFetchFingerprintConnectedAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, network, address string
	}{
		{"ipv4", "tcp4", "127.0.0.1:0"},
		{"ipv6", "tcp6", "[::1]:0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, expected, requests := fingerprintAuthorityFixture(t, tc.network, tc.address)
			for _, input := range []string{server.URL, server.URL + "/proxy/base/", strings.TrimPrefix(server.URL, "https://"), " " + strings.Replace(server.URL, "https://", "HTTPS://", 1) + " "} {
				fingerprint, err := FetchFingerprint(input)
				if err != nil || fingerprint != expected {
					t.Fatalf("capture at %q = %q, %v; want the owned leaf", input, fingerprint, err)
				}
			}
			if requests.Load() != 0 {
				t.Fatal("fingerprint capture sent HTTP rather than just a TLS handshake")
			}
		})
	}
}

func TestFetchFingerprintLegacyDefaultConnected(t *testing.T) {
	for _, tc := range []struct {
		name, network, address string
		inputs                 []string
	}{
		{"ipv4", "tcp4", "127.0.0.1:8006", []string{"127.0.0.1", " 127.0.0.1 "}},
		{"ipv6", "tcp6", "[::1]:8006", []string{"::1", "[::1]"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, expected, requests := fingerprintAuthorityFixture(t, tc.network, tc.address)
			for _, input := range tc.inputs {
				fingerprint, err := FetchFingerprint(input)
				if err != nil || fingerprint != expected {
					t.Fatalf("bare-host capture at %q = %q, %v; want the owned legacy-port leaf", input, fingerprint, err)
				}
			}
			if requests.Load() != 0 {
				t.Fatal("bare-host capture sent HTTP")
			}
		})
	}
}
