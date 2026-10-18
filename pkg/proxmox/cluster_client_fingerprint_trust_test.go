package proxmox

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These certificates and authentication values belong only to secret-free,
// guest-local fixtures. A changed certificate is not a production probe.
func clusterTrustCertificate(t *testing.T) *tls.Certificate {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func clusterTrustPin(cert *tls.Certificate) string {
	sha := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sha[:])
}

type clusterTrustFixture struct {
	server     *httptest.Server
	active     atomic.Pointer[tls.Certificate]
	requests   atomic.Int32
	handshakes atomic.Int32
	capture    atomic.Bool
	entered    chan struct{}
	resume     chan struct{}
}

func newClusterTrustFixture(t *testing.T) *clusterTrustFixture {
	t.Helper()
	f := &clusterTrustFixture{entered: make(chan struct{}), resume: make(chan struct{})}
	f.active.Store(clusterTrustCertificate(t))
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
			t.Error("lost fixture authentication")
		}
		if r.URL.Path != "/api2/json/nodes" {
			t.Errorf("unexpected fixture path %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
	}))
	f.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	f.server.TLS = &tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		f.handshakes.Add(1)
		cert := f.active.Load()
		if f.capture.CompareAndSwap(true, false) {
			close(f.entered)
			<-f.resume
		}
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*cert}}, nil
	}}
	f.server.StartTLS()
	t.Cleanup(f.server.Close)
	return f
}

func clusterTrustConfig(host, pin string) ClientConfig {
	return ClientConfig{Host: host, Fingerprint: pin, VerifySSL: true, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}
}

func clusterTrustMismatch(t *testing.T, f *clusterTrustFixture, pin string) error {
	t.Helper()
	client, err := NewClient(clusterTrustConfig(f.server.URL, pin))
	if err != nil {
		t.Fatal(err)
	}
	defer client.httpClient.CloseIdleConnections()
	_, err = client.GetNodes(context.Background())
	if !isFingerprintMismatchError(err) {
		t.Fatalf("expected real local pin failure, got %v", err)
	}
	return err
}

func testClusterClientFingerprintTrust(t *testing.T) {
	for _, phase := range []string{"startup", "recovery"} {
		for _, role := range []string{"primary", "member"} {
			t.Run("known-pins/"+phase+"/"+role, func(t *testing.T) {
				f := newClusterTrustFixture(t)
				original := f.active.Load()
				pin := clusterTrustPin(original)
				cfg := clusterTrustConfig(f.server.URL, pin)
				pins := map[string]string{}
				if role == "member" {
					cfg.Host = "https://primary.trust.invalid"
					pins[f.server.URL] = pin
				}
				cc := NewClusterClient("trust", cfg, []string{f.server.URL}, pins)
				// Change the actual leaf while retaining the configured/saved pin.
				f.active.Store(clusterTrustCertificate(t))
				if phase == "startup" {
					// A real second, independently pinned endpoint keeps ordinary
					// cluster reads usable while the changed leaf stays rejected.
					safe := newClusterTrustFixture(t)
					pins[safe.server.URL] = clusterTrustPin(safe.active.Load())
					cc = NewClusterClient("trust", cfg, []string{f.server.URL, safe.server.URL}, pins)
					if _, err := cc.GetNodes(context.Background()); err != nil || safe.requests.Load() != 2 {
						t.Errorf("trusted sibling lost ordinary monitoring: %v", err)
					}
				} else {
					cc.mu.Lock()
					cc.nodeHealth[f.server.URL] = false
					cc.mu.Unlock()
					cc.recoverUnhealthyNodes(context.Background())
				}
				cc.mu.RLock()
				stored := cc.getEndpointFingerprintLocked(f.server.URL)
				healthy, diagnostic := cc.nodeHealth[f.server.URL], cc.lastError[f.server.URL]
				cc.mu.RUnlock()
				if f.requests.Load() != 0 || stored != pin || healthy || !strings.Contains(diagnostic, "independently verify") {
					t.Errorf("changed certificate gained trust: authenticated=%d retained-pin=%t healthy=%t diagnostic=%q", f.requests.Load(), stored == pin, healthy, diagnostic)
				}
				// Re-presenting the trusted leaf restores ordinary monitoring.
				f.active.Store(original)
				f.server.CloseClientConnections()
				cc.mu.Lock()
				cc.nodeHealth[f.server.URL] = false
				delete(cc.lastHealthCheck, f.server.URL)
				cc.mu.Unlock()
				before := f.requests.Load()
				cc.recoverUnhealthyNodes(context.Background())
				if !cc.GetHealthStatus()[f.server.URL] || f.requests.Load() != before+1 || cc.getEndpointFingerprint(f.server.URL) != pin {
					t.Error("restored original certificate did not resume ordinary monitoring under the same pin")
				}
				t.Logf("changed leaf: no authenticated requests; original leaf: one authenticated recovery, retained %s pin", role)
			})
		}
	}

	for _, reason := range []string{"provider-text", "undeclared-endpoint", "cancelled-first-use", "primary-spelling"} {
		t.Run(reason, func(t *testing.T) {
			f := newClusterTrustFixture(t)
			pin := clusterTrustPin(clusterTrustCertificate(t))
			cc := NewClusterClient("trust", clusterTrustConfig("https://primary.trust.invalid", pin), []string{f.server.URL}, nil)
			var lastErr error
			if reason == "provider-text" {
				lastErr = fmt.Errorf("API response: %w", errors.New("certificate fingerprint mismatch: expected x, got y"))
			} else {
				lastErr = clusterTrustMismatch(t, f, pin)
				if reason == "undeclared-endpoint" {
					cc.endpoints = []string{"https://declared.trust.invalid"}
				} else if reason == "primary-spelling" {
					cc.config.Host = strings.TrimPrefix(f.server.URL, "https://") + "/./"
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if reason == "cancelled-first-use" {
				cancel()
			}
			before := f.handshakes.Load()
			client, err, retried := cc.refreshTOFUFingerprintAndRetry(ctx, f.server.URL, time.Second, lastErr)
			if client != nil || retried || !errors.Is(err, lastErr) || f.handshakes.Load() != before || f.requests.Load() != 0 || len(cc.endpointFingerprints) != 0 {
				t.Error("untrusted refresh evidence started capture, authenticated retry or pin mutation")
			}
		})
	}

	t.Run("concurrent-first-use", func(t *testing.T) {
		f := newClusterTrustFixture(t)
		primaryPin := clusterTrustPin(clusterTrustCertificate(t))
		cc := NewClusterClient("trust", clusterTrustConfig("https://primary.trust.invalid", primaryPin), []string{f.server.URL}, nil)
		lastErr := clusterTrustMismatch(t, f, primaryPin)
		f.capture.Store(true)
		done := make(chan error, 1)
		go func() {
			client, err, retried := cc.refreshTOFUFingerprintAndRetry(context.Background(), f.server.URL, time.Second, lastErr)
			if client != nil {
				defer client.httpClient.CloseIdleConnections()
			}
			if !retried && err == nil {
				err = errors.New("missing pinned retry")
			}
			done <- err
		}()
		select {
		case <-f.entered:
		case <-time.After(10 * time.Second):
			close(f.resume)
			t.Fatal("capture did not reach bounded fixture handshake")
		}
		// A competing first-use capture establishes its pin while this older
		// probe is still in flight. The older probe must not replace that winner.
		winner := clusterTrustCertificate(t)
		winnerPin := clusterTrustPin(winner)
		cc.mu.Lock()
		cc.endpointFingerprints[f.server.URL] = winnerPin
		cc.mu.Unlock()
		f.active.Store(winner)
		close(f.resume)
		if err := <-done; err != nil {
			t.Errorf("retry did not use concurrently established trust: %v", err)
		}
		if cc.getEndpointFingerprint(f.server.URL) != winnerPin || f.requests.Load() != 1 {
			t.Error("older first-use capture replaced the winner or failed its authenticated acceptance")
		}
	})

	t.Run("first-use-and-input-ownership", func(t *testing.T) {
		f := newClusterTrustFixture(t)
		primaryPin := clusterTrustPin(clusterTrustCertificate(t))
		input := map[string]string{}
		cc := NewClusterClient("trust", clusterTrustConfig("https://primary.trust.invalid", primaryPin), []string{f.server.URL}, input)
		lastErr := clusterTrustMismatch(t, f, primaryPin)
		before := f.handshakes.Load()
		client, err, retried := cc.refreshTOFUFingerprintAndRetry(context.Background(), f.server.URL, time.Second, lastErr)
		if client != nil {
			defer client.httpClient.CloseIdleConnections()
		}
		if err != nil || !retried || client == nil || f.requests.Load() != 1 || cc.getEndpointFingerprint(f.server.URL) != clusterTrustPin(f.active.Load()) {
			t.Fatalf("declared new member cannot establish first-use trust: %v", err)
		}
		if len(input) != 0 {
			t.Error("runtime trust capture mutated caller-owned saved pins")
		}
		// Exactly one unauthenticated capture handshake and one pinned API
		// handshake follow the initial mismatch. No password or API token is
		// sent by the capture itself.
		if f.handshakes.Load() != before+2 {
			t.Error("first use performed unexpected TLS work")
		}
		f.active.Store(clusterTrustCertificate(t))
		f.server.CloseClientConnections()
		cc.mu.Lock()
		cc.nodeHealth[f.server.URL] = false
		cc.mu.Unlock()
		cc.recoverUnhealthyNodes(context.Background())
		if f.requests.Load() != 1 || cc.GetHealthStatus()[f.server.URL] {
			t.Error("first-use pin was silently replaced on later recovery")
		}
	})

	t.Run("equivalent-known-endpoint", func(t *testing.T) {
		cc := NewClusterClient("trust", clusterTrustConfig("https://primary.trust.invalid", "primary-pin"), nil,
			map[string]string{"https://MEMBER.trust.invalid:443/tenant/": "member-pin"})
		if cc.getEndpointFingerprint("https://member.trust.invalid/tenant") != "member-pin" {
			t.Error("equivalent URL spelling forgot established member trust")
		}
		for _, endpoint := range []string{"http://member.trust.invalid/tenant", "https://member.trust.invalid:8006/tenant", "https://member.trust.invalid/Tenant"} {
			if cc.getEndpointFingerprint(endpoint) != "primary-pin" {
				t.Errorf("independent authority/path inherited another endpoint's pin: %s", endpoint)
			}
		}
	})
}
