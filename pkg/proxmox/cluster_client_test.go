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

func TestClusterClientIgnoresGuestAgentTimeoutForHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"test","status":"online","cpu":0,"maxcpu":1,"mem":0,"maxmem":1,"disk":0,"maxdisk":1,"uptime":1,"level":"normal"}]}`)
		case "/api2/json/nodes/test/qemu/100/agent/get-fsinfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"data":null,"message":"VM 100 qmp command 'guest-get-fsinfo' failed - got timeout\n"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
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

	_, err := cc.GetVMFSInfo(ctx, "test", 100)
	if err == nil {
		t.Fatalf("expected VM guest agent timeout error, got nil")
	}

	health := cc.GetHealthStatusWithErrors()
	endpointHealth, ok := health[server.URL]
	if !ok {
		t.Fatalf("expected health entry for endpoint %s", server.URL)
	}
	if !endpointHealth.Healthy {
		t.Fatalf("expected endpoint to remain healthy after VM-specific guest agent error, got %+v", endpointHealth)
	}
	if endpointHealth.LastError != "" {
		t.Fatalf("expected last error to remain empty for VM-specific failures, got %q", endpointHealth.LastError)
	}
}
