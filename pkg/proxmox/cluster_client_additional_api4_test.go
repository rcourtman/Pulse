package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClusterClient_GetZFSPoolsWithDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
		case "/api2/json/nodes/node1/disks/zfs":
			fmt.Fprint(w, `{"data":[{"name":"rpool","health":"ONLINE"}]}`)
		case "/api2/json/nodes/node1/disks/zfs/rpool":
			fmt.Fprint(w, `{"data":{"name":"rpool","state":"ONLINE","status":"ok","scan":"none","errors":"0"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	pools, err := cc.GetZFSPoolsWithDetails(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetZFSPoolsWithDetails failed: %v", err)
	}
	if len(pools) != 1 || pools[0].Name != "rpool" {
		t.Fatalf("unexpected pools: %+v", pools)
	}
	if pools[0].State != "ONLINE" {
		t.Fatalf("expected state ONLINE, got %q", pools[0].State)
	}
}

func TestClusterClient_IsQuorate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/status" {
			fmt.Fprint(w, `{"data":[{"type":"cluster","name":"pve","quorate":1}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	quorate, err := cc.IsQuorate(context.Background())
	if err != nil {
		t.Fatalf("IsQuorate failed: %v", err)
	}
	if !quorate {
		t.Fatal("expected quorate true")
	}
}

func TestClusterClient_IsQuorateStandalone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/status" {
			fmt.Fprint(w, `{"data":[{"type":"node","name":"node1"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	quorate, err := cc.IsQuorate(context.Background())
	if err != nil {
		t.Fatalf("IsQuorate failed: %v", err)
	}
	if !quorate {
		t.Fatal("expected quorate true for standalone node")
	}
}

func TestClusterClient_GetDisks_NoHealthyNodes(t *testing.T) {
	cc := &ClusterClient{
		name:            "test",
		endpoints:       []string{"node1", "node2"},
		clients:         make(map[string]*Client),
		nodeHealth:      map[string]bool{"node1": false, "node2": false},
		lastError:       make(map[string]string),
		lastHealthCheck: map[string]time.Time{"node1": time.Now(), "node2": time.Now()},
		rateLimitUntil:  make(map[string]time.Time),
	}

	disks, err := cc.GetDisks(context.Background(), "node1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(disks) != 0 {
		t.Fatalf("expected empty disks on no healthy nodes, got %+v", disks)
	}
}
