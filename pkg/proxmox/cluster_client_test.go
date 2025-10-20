package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterClientHandlesRateLimitWithoutMarkingUnhealthy(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			current := atomic.AddInt32(&requestCount, 1)
			if current == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"error":"rate limited"}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online","cpu":0,"maxcpu":1,"mem":0,"maxmem":1,"disk":0,"maxdisk":1,"uptime":1,"level":"normal"}]}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{}}`)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "pulse@pve!token",
		TokenValue: "sometokenvalue",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	cc := NewClusterClient("test-cluster", cfg, []string{server.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	nodes, err := cc.GetNodes(ctx)
	if err != nil {
		t.Fatalf("expected GetNodes to succeed after retry, got error: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after retry, got %d", len(nodes))
	}

	health := cc.GetHealthStatus()
	if healthy, ok := health[server.URL]; !ok || !healthy {
		t.Fatalf("expected endpoint %s to remain healthy, got health map: %+v", server.URL, health)
	}

	if atomic.LoadInt32(&requestCount) < 2 {
		t.Fatalf("expected at least 2 requests to backend, got %d", requestCount)
	}
}
