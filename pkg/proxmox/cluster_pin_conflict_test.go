package proxmox

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Real TLS sockets with fixture-only credentials. Conflicting pins must be
// refused before a dial, not merely when one of the selected leaves mismatches.
type conflictingPinFixture struct {
	server                    *httptest.Server
	handshakes, logins, reads atomic.Int32
	pin                       string
}

func newConflictingPinFixture(t *testing.T, mode string) *conflictingPinFixture {
	t.Helper()
	cert := clusterTrustCertificate(t)
	f := &conflictingPinFixture{pin: clusterTrustPin(cert)}
	f.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			f.logins.Add(1)
			if mode == "password-form" && r.Header.Get("Content-Type") == "application/json" {
				w.WriteHeader(http.StatusUnsupportedMediaType)
				fmt.Fprint(w, `{"message":"fixture requires form encoding"}`)
				return
			}
			fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
		case "/api2/json/nodes":
			f.reads.Add(1)
			if mode == "token" {
				if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
					t.Error("lost token fixture authentication")
				}
			} else if r.Header.Get("Cookie") != "PVEAuthCookie=fixture-ticket" {
				t.Error("lost password fixture authentication")
			}
			fmt.Fprint(w, `{"data":[{"node":"fixture","status":"online"}]}`)
		default:
			t.Errorf("left ordinary authentication/nodes path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	f.server.Config.ErrorLog = log.New(io.Discard, "", 0)
	f.server.TLS = &tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		f.handshakes.Add(1)
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*cert}}, nil
	}}
	f.server.StartTLS()
	t.Cleanup(f.server.Close)
	return f
}

func conflictPinConfig(host, pin, mode string, verify bool) ClientConfig {
	cfg := ClientConfig{Host: host, Fingerprint: pin, VerifySSL: verify, Timeout: 2 * time.Second}
	if mode == "token" {
		cfg.TokenName = "fixture@pve!pulse"
		cfg.TokenValue = "fixture"
	} else {
		cfg.User = "fixture@pam"
		cfg.Password = "fixture-password"
	}
	return cfg
}

func TestClusterConflictingSavedPinsNeverAuthenticate(t *testing.T) {
	const diagnostic = "TLS trust conflict - equivalent endpoint URLs have different saved fingerprints; independently verify and reconcile saved trust"
	for _, phase := range []string{"startup", "recovery", "selection"} {
		for _, mode := range []string{"token", "password-json", "password-form"} {
			for _, spelling := range []string{"exact-valid", "exact-other", "normalized-only"} {
				for _, verify := range []bool{true, false} {
					t.Run(fmt.Sprintf("%s/%s/%s/verify=%t", phase, mode, spelling, verify), func(t *testing.T) {
						bad, safe := newConflictingPinFixture(t, mode), newConflictingPinFixture(t, mode)
						otherPin := strings.Repeat("0", 64)
						alias := bad.server.URL + "/./"
						pins := map[string]string{bad.server.URL: bad.pin, alias: otherPin, safe.server.URL: safe.pin}
						endpoint := bad.server.URL
						if spelling == "exact-other" {
							endpoint = alias
						}
						if spelling == "normalized-only" {
							endpoint = bad.server.URL + "/"
						}
						key, _ := clusterEndpointKey(endpoint)
						if aliasKey, ok := clusterEndpointKey(alias); !ok || key != aliasKey {
							t.Fatal("fixture URLs are not equivalent under existing TLS identity")
						}
						saved := make(map[string]string, len(pins))
						for k, v := range pins {
							saved[k] = v
						}
						endpoints := []string{endpoint}
						if phase == "startup" {
							endpoints = append(endpoints, safe.server.URL)
						}
						cc := NewClusterClient("pin-conflict", conflictPinConfig(safe.server.URL, safe.pin, mode, verify), endpoints, pins)
						if phase == "recovery" {
							cc.mu.Lock()
							cc.nodeHealth[endpoint] = false
							cc.mu.Unlock()
							cc.recoverUnhealthyNodes(context.Background())
						}
						if phase != "recovery" {
							_, err := cc.GetNodes(context.Background())
							if phase == "selection" && (err == nil || !strings.Contains(err.Error(), diagnostic)) {
								t.Errorf("selection did not refuse ambiguous trust: %v", err)
							}
							if phase == "startup" && (err != nil || safe.reads.Load() == 0) {
								t.Errorf("independent trusted endpoint lost ordinary monitoring: %v", err)
							}
						}
						cc.mu.RLock()
						healthy, reason, client := cc.nodeHealth[endpoint], cc.lastError[endpoint], cc.clients[endpoint]
						unchanged := reflect.DeepEqual(cc.endpointFingerprints, saved)
						cc.mu.RUnlock()
						if healthy || reason != diagnostic || client != nil {
							t.Errorf("conflicting trust became usable: healthy=%t reason=%q client=%t", healthy, reason, client != nil)
						}
						if bad.handshakes.Load() != 0 || bad.logins.Load() != 0 || bad.reads.Load() != 0 {
							t.Errorf("conflicting saved trust reached the provider: TLS=%d login=%d reads=%d", bad.handshakes.Load(), bad.logins.Load(), bad.reads.Load())
						}
						if !unchanged || !reflect.DeepEqual(pins, saved) {
							t.Error("conflicting trust was silently reconciled or caller pins mutated")
						}
						// A normal configuration reload can use independently reconciled saved
						// trust. Case/colon presentation does not create a new certificate pin.
						formatted := strings.ToUpper(strings.Join(splitPinPairs(bad.pin), ":"))
						corrected := map[string]string{bad.server.URL: bad.pin, alias: formatted}
						recovered := NewClusterClient("pin-reconciled", conflictPinConfig(safe.server.URL, safe.pin, mode, verify), []string{endpoint}, corrected)
						nodes, err := recovered.GetNodes(context.Background())
						if err != nil || len(nodes) != 1 || !recovered.GetHealthStatus()[endpoint] || bad.reads.Load() == 0 {
							t.Errorf("agreed saved pins did not resume ordinary monitoring: %v", err)
						}
						if !reflect.DeepEqual(corrected, map[string]string{bad.server.URL: bad.pin, alias: formatted}) {
							t.Error("reconciled trust input changed")
						}
						for _, cluster := range []*ClusterClient{cc, recovered} {
							for _, client := range cluster.clients {
								client.httpClient.CloseIdleConnections()
							}
						}
					})
				}
			}
		}
	}
}

func splitPinPairs(pin string) []string {
	pairs := make([]string, 0, len(pin)/2)
	for i := 0; i < len(pin); i += 2 {
		pairs = append(pairs, pin[i:i+2])
	}
	return pairs
}

func TestClusterSavedPinScopesStayDistinct(t *testing.T) {
	const endpoint = "https://member.scope.invalid:8006/tenant"
	const pin = "saved-pin"
	for _, other := range []string{"http://member.scope.invalid:8006/tenant", "https://member.scope.invalid:8007/tenant", "https://other.scope.invalid:8006/tenant", "https://member.scope.invalid:8006/Tenant", "https://member.scope.invalid:8006/other"} {
		cc := NewClusterClient("scope", ClientConfig{Host: endpoint, Fingerprint: "primary-pin"}, nil, map[string]string{endpoint: pin, other: "independent-pin"})
		fingerprint, err := cc.getEndpointFingerprint(endpoint)
		if err != nil || fingerprint != pin {
			t.Errorf("distinct endpoint created a saved-trust conflict: %q %v", fingerprint, err)
		}
		// Exercise the pre-existing scope predicate without network. Neither a
		// distinct origin nor another base path may create a conflict for this URL.
		a, okA := clusterEndpointKey(endpoint)
		b, okB := clusterEndpointKey(other)
		if !okA || !okB || a == b {
			t.Error("independent TLS authority/path was collapsed")
		}
	}
}
