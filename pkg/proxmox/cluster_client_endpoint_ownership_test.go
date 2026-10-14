package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClusterClientEndpointOwnershipRoutingAndTrust(t *testing.T) {
	declared := newClusterTrustFixture(t)
	other := newClusterTrustFixture(t)
	declaredPin := clusterTrustPin(declared.active.Load())
	input := []string{declared.server.URL}
	cfg := clusterTrustConfig("https://primary.trust.invalid", declaredPin)
	// This is a routing/ownership assertion, not a one-second latency SLO.
	// Keep local fixture work bounded without making race instrumentation,
	// TLS scheduling or diagnostic output an ownership failure. Production
	// timeouts and the separate cancellation/timeout tests are unchanged.
	cfg.Timeout = 5 * time.Second
	cc := NewClusterClient("owned-endpoints", cfg, input, nil)

	// Reusing the caller's configuration storage must not declare a new member,
	// change the live failover target, or permit first-use trust for that target.
	input[0] = other.server.URL
	cc.mu.RLock()
	mayTrustOther := cc.canTrustMemberOnFirstUseLocked(other.server.URL)
	cc.mu.RUnlock()
	if mayTrustOther {
		t.Error("caller-owned endpoint edit declared a new first-use trust target")
	}
	before := declared.requests.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started := time.Now()
	// The first single-endpoint read also performs a connectivity check. Pin
	// the authenticated target, not the implementation's number of API probes.
	if nodes, err := cc.GetNodes(ctx); err != nil || len(nodes) != 1 || declared.requests.Load() <= before {
		t.Errorf("caller-owned endpoint edit interrupted ordinary pinned monitoring: %v", err)
	}
	t.Logf("bounded routing read: elapsed=%s client_timeout=%s declared_requests=%d injected_requests=%d", time.Since(started), cfg.Timeout, declared.requests.Load()-before, other.requests.Load())

	// Use a real local verifier mismatch, not provider text. Rejected admission
	// must do no additional capture handshake, authenticated request or pin write.
	lastErr := clusterTrustMismatch(t, other, declaredPin)
	handshakes := other.handshakes.Load()
	client, err, retried := cc.refreshTOFUFingerprintAndRetry(context.Background(), other.server.URL, time.Second, lastErr)
	if client != nil {
		defer client.httpClient.CloseIdleConnections()
	}
	if client != nil || retried || !errors.Is(err, lastErr) || other.handshakes.Load() != handshakes || other.requests.Load() != 0 {
		t.Error("caller-owned endpoint edit admitted capture or authenticated first-use retry")
	}
	cc.mu.RLock()
	_, captured := cc.endpointFingerprints[other.server.URL]
	cc.mu.RUnlock()
	if captured || checkedClusterFingerprint(t, cc, declared.server.URL) != declaredPin {
		t.Error("caller-owned endpoint edit changed the client's trust state")
	}
	t.Logf("observed injected endpoint authenticated requests=%d; original pin unchanged=%t", other.requests.Load(), checkedClusterFingerprint(t, cc, declared.server.URL) == declaredPin)
}

func testClusterClientEndpointOwnershipGuestAdmission(t *testing.T) {
	isolateGuestAgentCapacity(t)
	entered, finish := make(chan struct{}), make(chan struct{})
	var firstCommand atomic.Bool
	var originalCommands, otherRequests atomic.Int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
			t.Error("fixture authentication changed")
		}
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		default:
			originalCommands.Add(1)
			if strings.HasSuffix(r.URL.Path, "/qemu/105/agent/get-fsinfo") && firstCommand.CompareAndSwap(false, true) {
				fmt.Fprint(w, `{"data":{"result":`)
				w.(http.Flusher).Flush()
				close(entered)
				<-finish
				fmt.Fprint(w, `[]}}`)
				return
			}
			backupAgentPayload(w, r)
		}
	}
	primary := httptest.NewServer(http.HandlerFunc(handler))
	defer primary.Close()
	member := httptest.NewServer(http.HandlerFunc(handler))
	defer member.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherRequests.Add(1)
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"other","status":"online"}]}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		backupAgentPayload(w, r)
	}))
	defer other.Close()
	input := []string{member.URL}
	cc := NewClusterClient("owned-guest-targets", ClientConfig{Host: primary.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 5 * time.Second}, input, nil)
	standalone := backupTestClient(t, primary.URL)
	standalone.httpClient.Timeout = 5 * time.Second
	done := make(chan error, 1)
	go func() { _, err := standalone.GetVMFSInfo(context.Background(), "node", 105); done <- err }()
	joined := false
	complete := func() {
		if !joined {
			close(finish)
			joined = true
			if err := <-done; err != nil {
				t.Errorf("original single guest command failed: %v", err)
			}
		}
	}
	defer complete()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("original command did not reach the partial-body barrier")
	}
	input[0] = other.URL
	reads := map[string]func() error{
		"filesystem": func() error { _, err := cc.GetVMFSInfo(context.Background(), "node", 105); return err },
		"os":         func() error { _, err := cc.GetVMAgentInfo(context.Background(), "node", 105); return err },
		"version":    func() error { _, err := cc.GetVMAgentVersion(context.Background(), "node", 105); return err },
		"network":    func() error { _, err := cc.GetVMNetworkInterfaces(context.Background(), "node", 105); return err },
		"meminfo": func() error {
			_, err := cc.GetVMMemoryAvailabilityFromAgent(context.Background(), "node", 105)
			return err
		},
		"legacy-memory": func() error { _, err := cc.GetVMMemAvailableFromAgent(context.Background(), "node", 105); return err },
	}
	for name, read := range reads {
		if err := read(); GuestAgentDeferredReason(err) != "agent-busy" {
			t.Errorf("%s bypassed the originally registered guest channel: %v", name, err)
		}
	}
	if originalCommands.Load() != 1 || otherRequests.Load() != 0 {
		t.Errorf("caller-owned endpoint edit dispatched outside shared admission: original commands=%d, other requests=%d", originalCommands.Load(), otherRequests.Load())
	}
	if _, err := cc.GetVMAgentInfo(context.Background(), "node", 106); err != nil {
		t.Errorf("independent guest lost monitoring: %v", err)
	}
	complete()
	if _, err := cc.GetVMFSInfo(context.Background(), "node", 105); err != nil {
		t.Errorf("completed guest did not resume through the declared member: %v", err)
	}
	if originalCommands.Load() != 3 || otherRequests.Load() != 0 {
		t.Errorf("recovery left declared targets: original commands=%d, other requests=%d", originalCommands.Load(), otherRequests.Load())
	}
	t.Logf("observed original commands=%d, other requests=%d; six busy-read results checked", originalCommands.Load(), otherRequests.Load())
}

func testClusterClientEndpointOwnershipShape(t *testing.T) {
	for _, input := range [][]string{nil, {}, {"invalid://first", "invalid://second", "invalid://first"}} {
		want := slices.Clone(input)
		cc := NewClusterClient("owned-shape", ClientConfig{Host: "https://primary.shape.invalid", TokenName: "fixture@pve!pulse", TokenValue: "fixture"}, input, nil)
		for i := range input {
			input[i] = "invalid://changed"
		}
		if !slices.Equal(cc.endpoints, want) || (cc.endpoints == nil) != (want == nil) {
			t.Error("endpoint snapshot changed order, duplicates, nil/empty shape or ownership")
		}
	}
}

func testClusterClientEndpointOwnershipConcurrentReads(t *testing.T) {
	var declaredRequests, otherRequests atomic.Int32
	newServer := func(name string, requests *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if r.URL.Path != "/api2/json/nodes" || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
				t.Error("ordinary monitoring path or fixture authentication changed")
			}
			fmt.Fprintf(w, `{"data":[{"node":%q,"status":"online"}]}`, name)
		}))
	}
	declared := newServer("declared", &declaredRequests)
	defer declared.Close()
	other := newServer("other", &otherRequests)
	defer other.Close()
	input := []string{declared.URL}
	cc := NewClusterClient("owned-concurrent", ClientConfig{Host: declared.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}, input, nil)
	started, stop, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
				input[0] = other.URL
				runtime.Gosched()
				input[0] = declared.URL
			}
		}
	}()
	defer func() { close(stop); <-joined }()
	<-started
	for i := 0; i < 32; i++ {
		nodes, err := cc.GetNodes(context.Background())
		if err != nil || len(nodes) != 1 || nodes[0].Node != "declared" {
			t.Fatalf("concurrent configuration reuse changed ordinary monitoring: %v", err)
		}
	}
	if declaredRequests.Load() != 33 || otherRequests.Load() != 0 {
		t.Error("concurrent configuration reuse changed the authenticated target")
	}
	t.Logf("observed declared requests=%d, other requests=%d during 32 concurrent-input reads", declaredRequests.Load(), otherRequests.Load())
}
