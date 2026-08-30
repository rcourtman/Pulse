package collectorlifecycle

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testBearer = "collector-private-bearer"

func TestClientUsesSystemCA(t *testing.T) {
	server := newTLSServer(t, successfulReductionHandler(t))
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	previousSystemCertPool := loadSystemCertPool
	loadSystemCertPool = func() (*x509.CertPool, error) { return pool, nil }
	t.Cleanup(func() { loadSystemCertPool = previousSystemCertPool })

	client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority with system CA: %v", err)
	}
}

func TestClientUsesCustomCA(t *testing.T) {
	server := newTLSServer(t, successfulReductionHandler(t))
	client, err := New(Config{
		PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID(), CACertPath: writeServerCertificate(t, server),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority with custom CA: %v", err)
	}
}

func TestClientUsesExactDERLeafPin(t *testing.T) {
	server := newTLSServer(t, successfulReductionHandler(t))
	fingerprint := sha256.Sum256(server.Certificate().Raw)
	client, err := New(Config{
		PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID(), ServerFingerprint: hex.EncodeToString(fingerprint[:]),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority with exact pin: %v", err)
	}
}

func TestFingerprintMismatchNeverAuthorizesHandler(t *testing.T) {
	var authorized atomic.Bool
	server := newTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			authorized.Store(true)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	client, err := New(Config{
		PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID(), ServerFingerprint: strings.Repeat("00", sha256.Size),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err == nil || !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Fatalf("ReduceAuthority error = %v, want fingerprint mismatch", err)
	}
	if authorized.Load() {
		t.Fatal("mismatched TLS peer received the collector Authorization header")
	}
}

func TestNonLoopbackHTTPIsRejectedBeforeTokenRead(t *testing.T) {
	for _, rawURL := range []string{"http://192.0.2.10:7655", "http://agent.localhost:7655"} {
		_, err := New(Config{PulseURL: rawURL, TokenFile: filepath.Join(t.TempDir(), "missing")})
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "http") {
			t.Fatalf("New(%q) error = %v, want non-loopback HTTP rejection", rawURL, err)
		}
	}
}

func TestExactLifecycleLoopbackHost(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "127.0.0.2", "::1", "::ffff:127.0.0.1"} {
		if !exactLifecycleLoopbackHost(host) {
			t.Errorf("exactLifecycleLoopbackHost(%q) = false", host)
		}
	}
	for _, host := range []string{"agent.localhost", "0.0.0.0", "192.0.2.10"} {
		if exactLifecycleLoopbackHost(host) {
			t.Errorf("exactLifecycleLoopbackHost(%q) = true", host)
		}
	}
}

func TestTokenFileRejectsSymlinkAndOversizeContent(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.token")
	if err := os.WriteFile(target, []byte(testBearer), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link.token")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{PulseURL: "http://127.0.0.1:7655", TokenFile: link, TokenOwnerUID: testTokenOwnerUID()}); err == nil {
		t.Fatal("New accepted symlinked token file")
	}
	oversize := filepath.Join(directory, "oversize.token")
	if err := os.WriteFile(oversize, []byte(strings.Repeat("x", maximumBearerBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{PulseURL: "http://127.0.0.1:7655", TokenFile: oversize, TokenOwnerUID: testTokenOwnerUID()}); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("New oversize token error = %v", err)
	}
}

func TestLoopbackHTTPIsAccepted(t *testing.T) {
	server := httptest.NewServer(successfulReductionHandler(t))
	defer server.Close()
	client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority over loopback HTTP: %v", err)
	}
}

func TestConfiguredCollectorOwned0640TokenIsAccepted(t *testing.T) {
	server := httptest.NewServer(successfulReductionHandler(t))
	defer server.Close()
	tokenFile := writeToken(t)
	if err := os.Chmod(tokenFile, 0640); err != nil {
		t.Fatal(err)
	}
	client, err := New(Config{PulseURL: server.URL, TokenFile: tokenFile, TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority with collector-owned 0640 token: %v", err)
	}
}

func TestLifecycleTransportNeverUsesEnvironmentProxy(t *testing.T) {
	var proxyRequests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		proxyRequests.Add(1)
		http.Error(w, "proxy must not be used", http.StatusBadGateway)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)
	server := httptest.NewServer(successfulReductionHandler(t))
	defer server.Close()
	client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("collector lifecycle transport = %T, want *http.Transport", client.http.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("collector lifecycle transport configured an environment proxy")
	}
	if err := client.ReduceAuthority(context.Background(), "agent-1", "host.local"); err != nil {
		t.Fatalf("ReduceAuthority: %v", err)
	}
	if proxyRequests.Load() != 0 {
		t.Fatalf("environment proxy received %d lifecycle requests", proxyRequests.Load())
	}
}

func TestRedirectIsRejectedWithoutAuthorizingDestination(t *testing.T) {
	var destinationAuthorized atomic.Bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			destinationAuthorized.Store(true)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, destination.URL+"/forged-success", http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	client, err := New(Config{PulseURL: source.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	err = client.ReduceAuthority(context.Background(), "agent-1", "host.local")
	if err == nil || !strings.Contains(err.Error(), "returned redirect") {
		t.Fatalf("ReduceAuthority error = %v, want redirect rejection", err)
	}
	if destinationAuthorized.Load() {
		t.Fatal("redirect destination received the collector Authorization header")
	}
}

func TestVerifyRegistrationRequiresFreshAuthenticatedEvidence(t *testing.T) {
	prior := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer "+testBearer {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.URL.Query().Get("id") != "agent-1" {
			t.Errorf("lookup id = %q", request.URL.Query().Get("id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-1","hostname":"host.local","lastSeen":"2026-08-30T12:00:01Z"}}`))
	}))
	defer server.Close()
	client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()
	registration, err := client.VerifyRegistration(context.Background(), "agent-1", "", prior)
	if err != nil {
		t.Fatalf("VerifyRegistration: %v", err)
	}
	if !registration.LastSeen.Equal(prior.Add(time.Second)) {
		t.Fatalf("LastSeen = %s", registration.LastSeen)
	}
	if _, err := client.VerifyRegistration(context.Background(), "agent-1", "", prior.Add(time.Second)); !errors.Is(err, ErrRegistrationPending) {
		t.Fatalf("stale VerifyRegistration error = %v, want ErrRegistrationPending", err)
	}
}

func TestVerifyRegistrationChecksBothBoundIdentities(t *testing.T) {
	for _, test := range []struct {
		name             string
		responseHostname string
		wantErr          bool
	}{
		{name: "short and FQDN are equivalent", responseHostname: "host-1.example.test"},
		{name: "different full hostname is rejected", responseHostname: "host-1.other.test", wantErr: true},
		{name: "different short hostname is rejected", responseHostname: "host-2", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":true,"agent":{"id":"agent-1","hostname":"` + test.responseHostname + `","lastSeen":"2026-08-30T12:00:01Z"}}`))
			}))
			defer server.Close()
			client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer client.Close()
			_, err = client.VerifyRegistration(context.Background(), "agent-1", "host-1.example.test", time.Time{})
			if test.wantErr && !errors.Is(err, ErrRegistrationPending) {
				t.Fatalf("VerifyRegistration error = %v, want ErrRegistrationPending", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("VerifyRegistration: %v", err)
			}
		})
	}
}

func TestVerifyRegistrationClassifiesCredentialRejection(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
		body string
		want error
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, want: ErrCredentialRejected},
		{name: "scope forbidden", code: http.StatusForbidden, body: `{"code":"forbidden"}`, want: ErrCredentialRejected},
		{name: "binding pending", code: http.StatusForbidden, body: `{"code":"agent_lookup_forbidden"}`, want: ErrRegistrationPending},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.code)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := New(Config{PulseURL: server.URL, TokenFile: writeToken(t), TokenOwnerUID: testTokenOwnerUID()})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer client.Close()
			_, err = client.VerifyRegistration(context.Background(), "agent-1", "", time.Time{})
			if !errors.Is(err, test.want) {
				t.Fatalf("VerifyRegistration error = %v, want %v", err, test.want)
			}
		})
	}
}

func successfulReductionHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/agents/collector/reduce-authority" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
			http.NotFound(w, request)
			return
		}
		if got := request.Header.Get("Authorization"); got != "Bearer "+testBearer {
			t.Errorf("Authorization = %q", got)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func newTLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

func writeServerCertificate(t *testing.T, server *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "server-ca.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeToken(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "collector.token")
	if err := os.WriteFile(path, []byte(testBearer), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
