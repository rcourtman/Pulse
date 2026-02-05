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
}

func TestClusterClientApplyRateLimitCooldown(t *testing.T) {
	cc := &ClusterClient{rateLimitUntil: make(map[string]time.Time)}
	cc.applyRateLimitCooldown("node1", 100*time.Millisecond)
	if when, ok := cc.rateLimitUntil["node1"]; !ok || time.Until(when) <= 0 {
		t.Fatalf("expected cooldown set, got %v", when)
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
