package proxmox

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestGetHealthyClientSingleEndpointFallback(t *testing.T) {
	endpoint := "http://example.invalid"
	cc := &ClusterClient{
		name:            "single",
		endpoints:       []string{endpoint},
		clients:         map[string]*Client{endpoint: {}},
		nodeHealth:      map[string]bool{endpoint: false},
		lastError:       make(map[string]string),
		lastHealthCheck: make(map[string]time.Time),
		rateLimitUntil:  make(map[string]time.Time),
	}

	client, err := cc.getHealthyClient(context.Background())
	if err != nil {
		t.Fatalf("getHealthyClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if !cc.nodeHealth[endpoint] {
		t.Fatal("expected endpoint to be marked healthy after fallback")
	}
}

func TestExecuteWithFailoverMovesToAnotherEndpoint(t *testing.T) {
	endpoint1 := "node1"
	endpoint2 := "node2"
	now := time.Now()
	cc := &ClusterClient{
		name:      "test",
		endpoints: []string{endpoint1, endpoint2},
		clients: map[string]*Client{
			endpoint1: {},
			endpoint2: {},
		},
		nodeHealth: map[string]bool{
			endpoint1: true,
			endpoint2: true,
		},
		lastError: make(map[string]string),
		lastHealthCheck: map[string]time.Time{
			endpoint1: now,
			endpoint2: now,
		},
		rateLimitUntil: make(map[string]time.Time),
	}

	usedEndpoints := make([]string, 0, 2)
	attempts := 0
	err := cc.executeWithFailover(context.Background(), func(c *Client) error {
		endpoint := ""
		cc.mu.RLock()
		for ep, client := range cc.clients {
			if client == c {
				endpoint = ep
				break
			}
		}
		cc.mu.RUnlock()
		if endpoint == "" {
			return fmt.Errorf("failed to resolve endpoint for client")
		}
		usedEndpoints = append(usedEndpoints, endpoint)
		attempts++
		if attempts == 1 {
			// Only connectivity failures should trigger endpoint failover/unhealthy marking.
			return fmt.Errorf("connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("executeWithFailover failed: %v", err)
	}
	if len(usedEndpoints) < 2 {
		t.Fatalf("expected failover to second endpoint, used: %v", usedEndpoints)
	}
	if usedEndpoints[0] == usedEndpoints[1] {
		t.Fatalf("expected failover to a different endpoint, used: %v", usedEndpoints)
	}

	health := cc.GetHealthStatus()
	if health[usedEndpoints[0]] {
		t.Fatal("expected first endpoint to be unhealthy after failure")
	}
	if !health[usedEndpoints[1]] {
		t.Fatal("expected second endpoint to be healthy after success")
	}
}
