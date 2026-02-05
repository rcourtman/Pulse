package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestExecuteWithFailoverUsesRecoveredEndpoint(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node2","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer server2.Close()

	cfg := ClientConfig{
		Host:       server1.URL,
		TokenName:  "u@p!t",
		TokenValue: "v",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	cc := NewClusterClient("test", cfg, []string{server1.URL, server2.URL}, nil)

	// Force deterministic selection: only server1 healthy, server2 unhealthy.
	cc.mu.Lock()
	cc.nodeHealth[server1.URL] = true
	cc.nodeHealth[server2.URL] = false
	cc.lastHealthCheck[server2.URL] = time.Now().Add(-11 * time.Second)
	cc.mu.Unlock()

	usedEndpoints := make([]string, 0, 2)
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
		if endpoint == server1.URL {
			return fmt.Errorf("boom")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("executeWithFailover failed: %v", err)
	}
	if len(usedEndpoints) < 2 {
		t.Fatalf("expected failover to second endpoint, used: %v", usedEndpoints)
	}

	health := cc.GetHealthStatus()
	if health[server1.URL] {
		t.Fatal("expected server1 to be unhealthy after failure")
	}
	if !health[server2.URL] {
		t.Fatal("expected server2 to be healthy after recovery")
	}
}
