package monitoring

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

// Regression tests for discussion #1638: the cluster-endpoint discovery-policy
// check ran raw DNS lookups per node per poll cycle, and the SSH collectors
// re-executed ssh against hosts that kept failing. The policy check must now go
// through the shared cached resolver — which keeps DNS at roughly one query per
// host per cache refresh while still enforcing the blocklist against resolved
// addresses — and the SSH backoffs must only escalate for work that actually
// ran.

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
}

// issue1638FakeClock pins the SSH backoff clock so compounding, decay, and
// expiry are observable without sleeping.
type issue1638FakeClock struct {
	now time.Time
}

func (c *issue1638FakeClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func issue1638UseFakeClock(t *testing.T) *issue1638FakeClock {
	t.Helper()
	clock := &issue1638FakeClock{now: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)}
	old := sshBackoffNow
	sshBackoffNow = func() time.Time { return clock.now }
	t.Cleanup(func() {
		sshBackoffNow = old
	})
	return clock
}

// TestIssue1638DefaultPolicyBlocksHostnameResolvingToLinkLocal pins the
// blocklist enforcement that the first fix traded away. The default policy is
// just the injected 169.254.0.0/16 blocklist, and it has to apply to whatever a
// hostname endpoint resolves to, not only to literal link-local IPs — otherwise
// a name pointed at the metadata range walks straight through.
func TestIssue1638DefaultPolicyBlocksHostnameResolvingToLinkLocal(t *testing.T) {
	calls := 0
	issue1638CountingLookup(t, &calls, map[string][]net.IP{
		"imds.example.com": {net.ParseIP("169.254.169.254")},
		"node-a.local":     {net.ParseIP("192.168.1.5")},
	})

	// NormalizeDiscoveryConfig injects the default link-local blocklist, so
	// this is what every install without an explicit policy runs with.
	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{})

	blocked := config.ClusterEndpoint{NodeName: "node-imds", Host: "imds.example.com"}
	if got := clusterEndpointRuntimeURL(blocked, true, false, discoveryCfg); got != "" {
		t.Fatalf("hostname resolving into 169.254.0.0/16 was allowed: %q", got)
	}

	allowed := config.ClusterEndpoint{NodeName: "node-a", Host: "node-a.local"}
	if got := clusterEndpointRuntimeURL(allowed, true, false, discoveryCfg); got != "https://node-a.local:8006" {
		t.Fatalf("ordinary hostname endpoint URL = %q, want %q", got, "https://node-a.local:8006")
	}
}

func TestIssue1638DefaultPolicyStillBlocksLiteralLinkLocalWithoutDNS(t *testing.T) {
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

func TestIssue1638CustomPolicyEnforcedPerPoll(t *testing.T) {
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
}

// issue1638CountingResolver counts the DNS queries the shared cached resolver
// actually issues, as opposed to the lookups the policy check asks for.
type issue1638CountingResolver struct {
	queries int32
	answers map[string][]string
}

func (r *issue1638CountingResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	atomic.AddInt32(&r.queries, 1)
	if answer, ok := r.answers[host]; ok {
		return answer, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func (r *issue1638CountingResolver) LookupAddr(_ context.Context, addr string) ([]string, error) {
	atomic.AddInt32(&r.queries, 1)
	return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
}

// TestIssue1638PolicyLookupsStayCachedAcrossPolls is the #1638 win itself:
// with the decision cache gone, repeat poll cycles must still not generate DNS
// traffic, because the policy check resolves through the same process-global
// cached resolver that pkg/tlsutil dials with.
func TestIssue1638PolicyLookupsStayCachedAcrossPolls(t *testing.T) {
	// Names unique to this test so the process-global cache cannot be warmed
	// or poisoned by another test in the package.
	const (
		allowedHost = "issue1638-allowed.invalid"
		blockedHost = "issue1638-blocked.invalid"
	)

	counting := &issue1638CountingResolver{answers: map[string][]string{
		allowedHost: {"10.0.0.10"},
		blockedHost: {"169.254.169.254"},
	}}

	resolver := tlsutil.GetDNSResolver()
	previous := resolver.Resolver
	resolver.Resolver = counting
	t.Cleanup(func() {
		resolver.Resolver = previous
	})

	discoveryCfg := config.NormalizeDiscoveryConfig(config.DiscoveryConfig{})
	allowed := config.ClusterEndpoint{NodeName: "node-a", Host: allowedHost}
	blocked := config.ClusterEndpoint{NodeName: "node-b", Host: blockedHost}

	for poll := 0; poll < 20; poll++ {
		if got := clusterEndpointRuntimeURL(allowed, true, false, discoveryCfg); got != "https://"+allowedHost+":8006" {
			t.Fatalf("poll %d: allowed endpoint URL = %q", poll, got)
		}
		if got := clusterEndpointRuntimeURL(blocked, true, false, discoveryCfg); got != "" {
			t.Fatalf("poll %d: link-local endpoint unexpectedly allowed: %q", poll, got)
		}
	}

	if queries := atomic.LoadInt32(&counting.queries); queries != 2 {
		t.Fatalf("20 poll cycles issued %d DNS queries, want exactly 2 (one per endpoint host)", queries)
	}
}

func TestIssue1638KeyscanFailureIsNotRetriedEveryCycle(t *testing.T) {
	clock := issue1638UseFakeClock(t)

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

	// Suppressed calls must be distinguishable from a real refusal so callers
	// don't escalate their own backoff for work that never ran.
	suppressed := manager.Ensure(context.Background(), "unreachable.local")
	if !errors.Is(suppressed, ErrKeyscanSuppressed) {
		t.Fatalf("suppressed Ensure error = %v, want it to wrap ErrKeyscanSuppressed", suppressed)
	}

	// Resetting the manager (as applying settings does) retries immediately
	// rather than waiting out the window.
	manager.ResetFailures()
	if err := manager.Ensure(context.Background(), "unreachable.local"); err == nil {
		t.Fatal("expected error from failing keyscan after reset")
	}
	if scans != 2 {
		t.Fatalf("reset did not retry the keyscan, scans = %d, want 2", scans)
	}

	// Backoff compounds only while failures keep arriving inside the window.
	clock.advance(keyscanFailureInitialBackoff + time.Second)
	if err := manager.Ensure(context.Background(), "unreachable.local"); err == nil {
		t.Fatal("expected error once the first backoff window expired")
	}
	if scans != 3 {
		t.Fatalf("expired backoff did not retry, scans = %d, want 3", scans)
	}
	clock.advance(keyscanFailureInitialBackoff + time.Second)
	if err := manager.Ensure(context.Background(), "unreachable.local"); !errors.Is(err, ErrKeyscanSuppressed) {
		t.Fatalf("second failure should have doubled the window, got %v", err)
	}
	if scans != 3 {
		t.Fatalf("doubled backoff window still re-scanned, scans = %d, want 3", scans)
	}
}

func TestIssue1638KeyscanTimeoutDoesNotCompound(t *testing.T) {
	clock := issue1638UseFakeClock(t)

	scans := 0
	manager, err := NewKnownHostsManager(
		filepath.Join(t.TempDir(), "known_hosts"),
		WithKeyscanFunc(func(ctx context.Context, host string, port int, timeout time.Duration) ([]byte, error) {
			scans++
			return nil, fmt.Errorf("ssh-keyscan gave up: %w", context.DeadlineExceeded)
		}),
	)
	if err != nil {
		t.Fatalf("NewKnownHostsManager: %v", err)
	}

	// Three consecutive timeouts, each one window apart. A compounding backoff
	// would have suppressed the third; a timeout is not evidence about the
	// host, so the window stays at the floor.
	for cycle := 0; cycle < 3; cycle++ {
		if err := manager.Ensure(context.Background(), "slow.local"); err == nil {
			t.Fatalf("cycle %d: expected timeout error", cycle)
		}
		clock.advance(keyscanFailureInitialBackoff + time.Second)
	}

	if scans != 3 {
		t.Fatalf("timeouts compounded the keyscan backoff, scans = %d, want 3", scans)
	}
}

type issue1638CountingRunner struct {
	runs int
	err  error
}

func (r *issue1638CountingRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.runs++
	if r.err != nil {
		return nil, r.err
	}
	return nil, errors.New("connection refused")
}

func issue1638TemperatureCollector(t *testing.T) (*TemperatureCollector, *issue1638CountingRunner) {
	t.Helper()
	tc, runner, _ := issue1638TemperatureCollectorWithKey(t)
	return tc, runner
}

func issue1638TemperatureCollectorWithKey(t *testing.T) (*TemperatureCollector, *issue1638CountingRunner, string) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "id_ed25519_test")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	tc := NewTemperatureCollectorWithPort("root", keyPath, 22)
	tc.hostKeys = nil
	runner := &issue1638CountingRunner{}
	tc.runner = runner
	return tc, runner, keyPath
}

// TestIssue1638RepairedSSHKeyClearsBackoff pins the escape hatch from the
// backoff: replacing the SSH key is the usual fix for whatever opened the
// window, so the next cycle must retry instead of waiting the window out.
func TestIssue1638RepairedSSHKeyClearsBackoff(t *testing.T) {
	issue1638UseFakeClock(t)
	tc, runner, keyPath := issue1638TemperatureCollectorWithKey(t)

	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature: %v", err)
	}
	if runner.runs != 2 {
		t.Fatalf("first cycle ran %d ssh commands, want 2", runner.runs)
	}

	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature inside backoff: %v", err)
	}
	if runner.runs != 2 {
		t.Fatalf("backoff window still ran ssh, total %d commands, want 2", runner.runs)
	}

	// Operator installs a working key.
	if err := os.WriteFile(keyPath, []byte("repaired"), 0o600); err != nil {
		t.Fatalf("rewrite key: %v", err)
	}
	if err := os.Chtimes(keyPath, time.Now().Add(time.Minute), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("touch key: %v", err)
	}

	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature after key repair: %v", err)
	}
	if runner.runs != 4 {
		t.Fatalf("repaired key did not clear the backoff, total %d commands, want 4", runner.runs)
	}
}

func TestIssue1638TemperatureSSHFailureBacksOff(t *testing.T) {
	clock := issue1638UseFakeClock(t)
	tc, runner := issue1638TemperatureCollector(t)

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

	// Once the first window expires the host is retried, and that second hard
	// failure doubles the window: a poll one initial window later is suppressed.
	clock.advance(temperatureSSHFailureInitialBackoff + time.Second)
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature after expiry: %v", err)
	}
	if runner.runs != 4 {
		t.Fatalf("expired backoff did not retry, total %d commands, want 4", runner.runs)
	}

	clock.advance(temperatureSSHFailureInitialBackoff + time.Second)
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature inside doubled window: %v", err)
	}
	if runner.runs != 4 {
		t.Fatalf("backoff did not compound, total %d commands, want 4", runner.runs)
	}

	// Applying settings clears the backoff so a repaired key is picked up on
	// the next cycle instead of waiting the window out.
	tc.ResetSSHFailures()
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature after reset: %v", err)
	}
	if runner.runs != 6 {
		t.Fatalf("reset did not retry ssh, total %d commands, want 6", runner.runs)
	}
}

// TestIssue1638TemperatureBackoffDecaysAfterQuietPeriod pins that a host whose
// backoff expired long ago starts over at the floor rather than resuming the
// compounding it had reached.
func TestIssue1638TemperatureBackoffDecaysAfterQuietPeriod(t *testing.T) {
	clock := issue1638UseFakeClock(t)
	tc, runner := issue1638TemperatureCollector(t)

	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature: %v", err)
	}
	if runner.runs != 2 {
		t.Fatalf("first cycle ran %d ssh commands, want 2", runner.runs)
	}

	// Sit out the window and one further window on top of it, then fail again.
	clock.advance(2*temperatureSSHFailureInitialBackoff + time.Second)
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature after quiet period: %v", err)
	}
	if runner.runs != 4 {
		t.Fatalf("quiet-period retry did not run, total %d commands, want 4", runner.runs)
	}

	// The decayed backoff is back at the floor, so one initial window is enough
	// to retry rather than the doubled window a compounding-only policy keeps.
	clock.advance(temperatureSSHFailureInitialBackoff + time.Second)
	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature after decayed window: %v", err)
	}
	if runner.runs != 6 {
		t.Fatalf("backoff did not decay to the floor, total %d commands, want 6", runner.runs)
	}
}

// TestIssue1638TemperatureDoesNotEscalateWhenNoSSHRan covers the case where the
// knownhosts manager suppresses the call inside its own backoff. Nothing was
// executed, so the temperature layer must not record a failure of its own —
// otherwise two independent backoffs compound against a single event.
func TestIssue1638TemperatureDoesNotEscalateWhenNoSSHRan(t *testing.T) {
	issue1638UseFakeClock(t)
	tc, runner := issue1638TemperatureCollector(t)

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
	tc.hostKeys = manager

	// Put the knownhosts manager into its backoff window first.
	if err := manager.Ensure(context.Background(), "node1.local"); err == nil {
		t.Fatal("expected keyscan failure")
	}
	if scans != 1 {
		t.Fatalf("keyscan ran %d times, want 1", scans)
	}

	if _, err := tc.CollectTemperature(context.Background(), "node1.local", "node1"); err != nil {
		t.Fatalf("CollectTemperature: %v", err)
	}

	if runner.runs != 0 {
		t.Fatalf("ssh was executed %d times despite a suppressed host key scan, want 0", runner.runs)
	}
	if scans != 1 {
		t.Fatalf("ssh-keyscan re-ran inside its backoff, scans = %d, want 1", scans)
	}
	tc.sshFailureMu.Lock()
	recorded := len(tc.sshFailures)
	tc.sshFailureMu.Unlock()
	if recorded != 0 {
		t.Fatalf("temperature layer recorded %d failures for an attempt that never ran, want 0", recorded)
	}
}

// TestIssue1638TemperatureDeadlineDoesNotCompound pins the lenient treatment of
// our own collection deadline: it is not evidence that the host is broken, so
// the window holds at the floor and the wasted second probe is skipped.
func TestIssue1638TemperatureDeadlineDoesNotCompound(t *testing.T) {
	clock := issue1638UseFakeClock(t)
	tc, runner := issue1638TemperatureCollector(t)
	runner.err = errors.New("signal: killed")

	expired := func() (context.Context, context.CancelFunc) {
		return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	}

	for cycle := 0; cycle < 3; cycle++ {
		ctx, cancel := expired()
		if _, err := tc.CollectTemperature(ctx, "node1.local", "node1"); err != nil {
			cancel()
			t.Fatalf("cycle %d: CollectTemperature: %v", cycle, err)
		}
		cancel()
		clock.advance(temperatureSSHFailureInitialBackoff + time.Second)
	}

	// One probe per cycle (the RPi fallback is pointless once the deadline has
	// passed) and no compounding, so all three cycles ran.
	if runner.runs != 3 {
		t.Fatalf("deadline handling ran %d ssh commands, want 3 (one per cycle, no fallback, no compounding)", runner.runs)
	}
}
