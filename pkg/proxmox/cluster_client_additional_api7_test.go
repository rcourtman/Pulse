package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClusterClient_IsClusterMember_NodeCountFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"},{"node":"node2","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/status" {
			fmt.Fprint(w, `{"data":[{"type":"node","name":"node1"},{"type":"node","name":"node2"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	member, err := cc.IsClusterMember(context.Background())
	if err != nil {
		t.Fatalf("IsClusterMember failed: %v", err)
	}
	if !member {
		t.Fatal("expected cluster membership to be true via node count fallback")
	}
}

func TestClusterClient_IsClusterMember_SingleNode(t *testing.T) {
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

	member, err := cc.IsClusterMember(context.Background())
	if err != nil {
		t.Fatalf("IsClusterMember failed: %v", err)
	}
	if member {
		t.Fatal("expected cluster membership to be false for single-node response")
	}
}
