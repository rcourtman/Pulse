package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClusterClient_GetNodePendingUpdates_NoHealthyNodes(t *testing.T) {
	cc := &ClusterClient{
		name:       "test",
		endpoints:  []string{"node1", "node2"},
		nodeHealth: map[string]bool{"node1": false, "node2": false},
		lastError:  make(map[string]string),
		lastHealthCheck: map[string]time.Time{
			"node1": time.Now(),
			"node2": time.Now(),
		},
		rateLimitUntil: make(map[string]time.Time),
	}

	updates, err := cc.GetNodePendingUpdates(context.Background(), "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected empty updates on no healthy nodes, got %+v", updates)
	}
}

func TestClusterClient_GetVMAgentVersion_QemuGA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/info" {
			fmt.Fprint(w, `{"data":{"result":{"qemu-ga":{"version":"7.2.0"}}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	version, err := cc.GetVMAgentVersion(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentVersion failed: %v", err)
	}
	if version != "7.2.0" {
		t.Fatalf("unexpected version: %s", version)
	}
}
