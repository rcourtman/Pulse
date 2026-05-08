package proxmox

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestClusterClientEndpointFingerprint(t *testing.T) {
	cc := &ClusterClient{
		config: ClientConfig{Fingerprint: "base"},
		endpointFingerprints: map[string]string{
			"node1": "node-fp",
		},
	}

	if got := cc.getEndpointFingerprint("node1"); got != "node-fp" {
		t.Fatalf("expected node fingerprint, got %s", got)
	}
	if got := cc.getEndpointFingerprint("node2"); got != "base" {
		t.Fatalf("expected base fingerprint, got %s", got)
	}
}

func TestClusterClientMarkAndClearError(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: make(map[string]time.Time),
		rateLimitUntil:  map[string]time.Time{"node1": time.Now().Add(30 * time.Second)},
	}

	cc.markUnhealthyWithError("node1", "connection refused")
	if cc.nodeHealth["node1"] {
		t.Fatal("expected node to be unhealthy")
	}
	if errMsg := cc.lastError["node1"]; errMsg == "" || !strings.Contains(errMsg, "Connection refused") {
		t.Fatalf("unexpected error message: %q", errMsg)
	}

	cc.clearEndpointError("node1")
	if !cc.nodeHealth["node1"] {
		t.Fatal("expected node to be healthy after clear")
	}
	if _, ok := cc.lastError["node1"]; ok {
		t.Fatal("expected lastError to be cleared")
	}
	if _, ok := cc.rateLimitUntil["node1"]; ok {
		t.Fatal("expected rate-limit cooldown to be cleared")
	}
}

func TestClusterClientApplyRateLimitCooldown(t *testing.T) {
	cc := &ClusterClient{rateLimitUntil: make(map[string]time.Time)}
	cc.applyRateLimitCooldown("node1", 100*time.Millisecond)
	if when, ok := cc.rateLimitUntil["node1"]; !ok || time.Until(when) <= 0 {
		t.Fatalf("expected cooldown set, got %v", when)
	}
}

func TestClusterClientApplyRateLimitCooldownEmptyEndpoint(t *testing.T) {
	cc := &ClusterClient{rateLimitUntil: make(map[string]time.Time)}
	cc.applyRateLimitCooldown("", 100*time.Millisecond)
	if len(cc.rateLimitUntil) != 0 {
		t.Fatalf("expected no cooldown entry, got %+v", cc.rateLimitUntil)
	}
}

func TestExecuteWithFailoverSkipsUnhealthyMarking(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	err := cc.executeWithFailover(context.Background(), func(*Client) error {
		return fmt.Errorf("No QEMU guest agent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !cc.nodeHealth["node1"] {
		t.Fatal("expected node to remain healthy for VM-specific error")
	}
	if len(cc.lastError) != 0 {
		t.Fatalf("expected no lastError, got %+v", cc.lastError)
	}

	err = cc.executeWithFailover(context.Background(), func(*Client) error {
		return fmt.Errorf("authentication failed")
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if !cc.nodeHealth["node1"] {
		t.Fatal("expected node to remain healthy for auth error")
	}
}

func TestExecuteWithFailoverClearsErrorOnSuccess(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       map[string]string{"node1": "stale"},
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	if err := cc.executeWithFailover(context.Background(), func(*Client) error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cc.lastError["node1"]; ok {
		t.Fatal("expected lastError to be cleared")
	}
}

func TestExecuteWithFailoverNotImplementedDoesNotMarkUnhealthy(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	err := cc.executeWithFailover(context.Background(), func(*Client) error {
		return fmt.Errorf("not implemented status 501")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !cc.nodeHealth["node1"] {
		t.Fatal("expected node to remain healthy for not implemented error")
	}
	if len(cc.lastError) != 0 {
		t.Fatalf("expected no lastError, got %+v", cc.lastError)
	}
}

func TestExecuteWithFailoverRateLimitContextCancel(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := cc.executeWithFailover(ctx, func(*Client) error {
		return fmt.Errorf("status 429: Too Many Requests")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "backing off after rate limit") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cc.rateLimitUntil["node1"]; !ok {
		t.Fatal("expected rate limit cooldown to be recorded")
	}
}

func TestExecuteWithFailoverRateLimitThenSuccess(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	attempts := 0
	err := cc.executeWithFailover(ctx, func(*Client) error {
		attempts++
		if attempts == 1 {
			return fmt.Errorf("status 429: Too Many Requests")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after rate limit, got %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected retry after rate limit, attempts=%d", attempts)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("expected backoff delay, elapsed=%v", time.Since(start))
	}
}

func TestExecuteWithFailoverNodeSpecificStorageError(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	err := cc.executeWithFailover(context.Background(), func(*Client) error {
		return fmt.Errorf("context deadline exceeded /storage")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !cc.nodeHealth["node1"] {
		t.Fatal("expected node to remain healthy for storage timeout")
	}
	if len(cc.lastError) != 0 {
		t.Fatalf("expected no lastError, got %+v", cc.lastError)
	}
}

func TestGetHealthyClientUsesCoolingEndpoints(t *testing.T) {
	cc := &ClusterClient{
		name:            "cluster",
		endpoints:       []string{"node1"},
		clients:         map[string]*Client{"node1": {}},
		nodeHealth:      map[string]bool{"node1": true},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now()},
		rateLimitUntil:  map[string]time.Time{"node1": time.Now().Add(1 * time.Minute)},
	}

	client, err := cc.getHealthyClient(context.Background())
	if err != nil {
		t.Fatalf("getHealthyClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if _, ok := cc.rateLimitUntil["node1"]; !ok {
		t.Fatal("expected cooldown entry to remain while in future")
	}
}

func TestGetHealthyClientNoHealthyEndpointsError(t *testing.T) {
	now := time.Now()
	cc := &ClusterClient{
		name:      "cluster",
		endpoints: []string{"node1", "node2"},
		clients:   map[string]*Client{},
		nodeHealth: map[string]bool{
			"node1": false,
			"node2": false,
		},
		lastError: make(map[string]string),
		lastHealthCheck: map[string]time.Time{
			"node1": now,
			"node2": now,
		},
		rateLimitUntil: make(map[string]time.Time),
	}

	_, err := cc.getHealthyClient(context.Background())
	if err == nil {
		t.Fatal("expected error when no healthy endpoints")
	}
	if !strings.Contains(err.Error(), "no healthy nodes available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGetHealthyClientPreservesPerEndpointReason guards that the wrapped
// "no healthy nodes available" error includes each endpoint's recorded
// failure reason. Previously the wrapper only emitted endpoint URLs and
// the literal phrase "endpoints unreachable", which silently turned auth
// failures into "unreachable" upstream — the connections aggregator's
// auth-error regex never matched, so a token rejected with 401 on every
// endpoint surfaced as `state: "unreachable"` instead of `unauthorized`.
func TestGetHealthyClientPreservesPerEndpointReason(t *testing.T) {
	now := time.Now()

	t.Run("all endpoints failed auth", func(t *testing.T) {
		cc := &ClusterClient{
			name:      "homelab",
			endpoints: []string{"https://192.168.0.5:8006", "https://192.168.0.134:8006"},
			clients:   map[string]*Client{},
			nodeHealth: map[string]bool{
				"https://192.168.0.5:8006":   false,
				"https://192.168.0.134:8006": false,
			},
			lastError: map[string]string{
				"https://192.168.0.5:8006":   "Authentication failed - check API token or credentials",
				"https://192.168.0.134:8006": "Authentication failed - check API token or credentials",
			},
			lastHealthCheck: map[string]time.Time{
				"https://192.168.0.5:8006":   now,
				"https://192.168.0.134:8006": now,
			},
			rateLimitUntil: make(map[string]time.Time),
		}

		_, err := cc.getHealthyClient(context.Background())
		if err == nil {
			t.Fatal("expected error when no healthy endpoints")
		}
		msg := err.Error()
		if !strings.Contains(msg, "Authentication failed") {
			t.Fatalf("expected per-endpoint auth reason in error, got: %v", msg)
		}
		if !strings.Contains(msg, "https://192.168.0.5:8006") || !strings.Contains(msg, "https://192.168.0.134:8006") {
			t.Fatalf("expected both endpoints listed, got: %v", msg)
		}
	})

	t.Run("endpoint with no recorded reason", func(t *testing.T) {
		cc := &ClusterClient{
			name:      "homelab",
			endpoints: []string{"node1"},
			clients:   map[string]*Client{},
			nodeHealth: map[string]bool{
				"node1": false,
			},
			lastError:       map[string]string{}, // intentionally empty
			lastHealthCheck: map[string]time.Time{"node1": now},
			rateLimitUntil:  make(map[string]time.Time),
		}
		// Need >1 endpoint to hit the multi-endpoint code path; add a second.
		cc.endpoints = append(cc.endpoints, "node2")
		cc.nodeHealth["node2"] = false
		cc.lastHealthCheck["node2"] = now

		_, err := cc.getHealthyClient(context.Background())
		if err == nil {
			t.Fatal("expected error when no healthy endpoints")
		}
		msg := err.Error()
		if !strings.Contains(msg, "no recorded reason") {
			t.Fatalf("expected fallback reason text, got: %v", msg)
		}
	})
}
