package monitoring

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Regression tests for discussion #1638: the cluster-endpoint discovery-policy
// check ran raw DNS lookups per node per poll cycle. The default policy (only
// the injected 169.254.0.0/16 blocklist) must never touch the resolver, and
// custom policies must memoize their verdict so repeat polls stay off DNS.

func issue1638CountingLookup(t *testing.T, calls *int, ips map[string][]net.IP) {
	t.Helper()
	oldLookup := lookupIPFunc
	lookupIPFunc = func(host string) ([]net.IP, error) {
		*calls++
		if resolved, ok := ips[host]; ok {
			return resolved, nil
		}
		return nil, errors.New("no such host")
	}
	t.Cleanup(func() {
		lookupIPFunc = oldLookup
	})
	resetDiscoveryPolicyDecisionCache()
	t.Cleanup(resetDiscoveryPolicyDecisionCache)
}

func TestIssue1638DefaultPolicySkipsDNSResolution(t *testing.T) {
	calls := 0
	issue1638CountingLookup(t, &calls, map[string][]net.IP{
		"node-a.example.com": {net.ParseIP("192.168.1.5")},
	})

	// NormalizeDiscoveryConfig injects the default link-local blocklist, so
	// this is what every install without an explicit policy runs with.
	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{})
	endpoint := config.ClusterEndpoint{NodeName: "node-a", Host: "node-a.example.com"}

	for poll := 0; poll < 5; poll++ {
		got := clusterEndpointRuntimeURL(endpoint, true, false, discoveryCfg)
		if got != "https://node-a.example.com:8006" {
			t.Fatalf("poll %d: runtime URL = %q, want %q", poll, got, "https://node-a.example.com:8006")
		}
	}

	if calls != 0 {
		t.Fatalf("default link-local-only policy hit the resolver %d times, want 0", calls)
	}
}

func TestIssue1638DefaultPolicyStillBlocksLiteralLinkLocal(t *testing.T) {
	calls := 0
	issue1638CountingLookup(t, &calls, nil)

	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{})
	endpoint := config.ClusterEndpoint{NodeName: "node-b", IP: "169.254.10.20"}

	if got := clusterEndpointRuntimeURL(endpoint, false, false, discoveryCfg); got != "" {
		t.Fatalf("literal link-local endpoint allowed through default blocklist: %q", got)
	}
	if calls != 0 {
		t.Fatalf("literal IP evaluation hit the resolver %d times, want 0", calls)
	}
}

func TestIssue1638CustomPolicyResolvesOncePerEndpointAcrossPolls(t *testing.T) {
	calls := 0
	issue1638CountingLookup(t, &calls, map[string][]net.IP{
		"allowed.local": {net.ParseIP("10.0.0.10")},
		"blocked.local": {net.ParseIP("192.168.1.10")},
	})

	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.0.0.0/8"},
	})
	allowed := config.ClusterEndpoint{NodeName: "node-a", Host: "allowed.local"}
	blocked := config.ClusterEndpoint{NodeName: "node-b", Host: "blocked.local"}

	for poll := 0; poll < 20; poll++ {
		if got := clusterEndpointRuntimeURL(allowed, true, false, discoveryCfg); got != "https://allowed.local:8006" {
			t.Fatalf("poll %d: allowed endpoint URL = %q", poll, got)
		}
		if got := clusterEndpointRuntimeURL(blocked, true, false, discoveryCfg); got != "" {
			t.Fatalf("poll %d: blocked endpoint unexpectedly allowed: %q", poll, got)
		}
	}

	if calls != 2 {
		t.Fatalf("repeat polls hit the resolver %d times, want exactly 2 (one per endpoint host)", calls)
	}
}

func TestIssue1638CustomPolicyDecisionExpiresAfterTTL(t *testing.T) {
	calls := 0
	issue1638CountingLookup(t, &calls, map[string][]net.IP{
		"allowed.local": {net.ParseIP("10.0.0.10")},
	})

	baseTime := time.Now()
	oldNow := discoveryPolicyTimeNow
	discoveryPolicyTimeNow = func() time.Time { return baseTime }
	t.Cleanup(func() {
		discoveryPolicyTimeNow = oldNow
	})

	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{
		SubnetAllowlist: []string{"10.0.0.0/8"},
	})
	endpoint := config.ClusterEndpoint{NodeName: "node-a", Host: "allowed.local"}

	clusterEndpointRuntimeURL(endpoint, true, false, discoveryCfg)
	clusterEndpointRuntimeURL(endpoint, true, false, discoveryCfg)
	if calls != 1 {
		t.Fatalf("resolver hit %d times inside TTL, want 1", calls)
	}

	discoveryPolicyTimeNow = func() time.Time { return baseTime.Add(discoveryPolicyDecisionTTL + time.Second) }
	clusterEndpointRuntimeURL(endpoint, true, false, discoveryCfg)
	if calls != 2 {
		t.Fatalf("resolver hit %d times after TTL expiry, want 2", calls)
	}
}

func TestIssue1638KeyscanFailureIsNotRetriedEveryCycle(t *testing.T) {
	scans := 0
	manager, err := NewKnownHostsManager(
		filepath.Join(t.TempDir(), "known_hosts"),
		WithKeyscanFunc(func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
			scans++
			return nil, errors.New("connection refused")
		}),
	)
	if err != nil {
		t.Fatalf("NewKnownHostsManager: %v", err)
	}

	for cycle := 0; cycle < 5; cycle++ {
		if err := manager.Ensure(context.Background(), "unreachable.local"); err == nil {
			t.Fatalf("cycle %d: expected error from failing keyscan", cycle)
		}
	}

	if scans != 1 {
		t.Fatalf("failing host was keyscanned %d times within backoff window, want 1", scans)
	}
}

type issue1638CountingRunner struct {
	runs int
}

func (r *issue1638CountingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.runs++
	return nil, errors.New("connection refused")
}

func TestIssue1638TemperatureSSHFailureBacksOff(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "id_ed25519_test")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	tc := NewTemperatureCollectorWithPort("root", keyPath, 22)
	tc.hostKeys = nil
	runner := &issue1638CountingRunner{}
	tc.runner = runner

	// First cycle attempts both the sensors and the RPi fallback command.
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature: %v", err)
	}
	if runner.runs != 2 {
		t.Fatalf("first cycle ran %d ssh commands, want 2", runner.runs)
	}

	// Subsequent cycles inside the backoff window must not exec ssh again.
	for cycle := 0; cycle < 5; cycle++ {
		if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
			t.Fatalf("cycle %d: CollectTemperature: %v", cycle, err)
		}
	}
	if runner.runs != 2 {
		t.Fatalf("backoff window still ran ssh, total %d commands, want 2", runner.runs)
	}
}
