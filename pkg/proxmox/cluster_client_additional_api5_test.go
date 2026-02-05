package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClusterClient_GetStorageContentFiltersBackup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
		case "/api2/json/nodes/node1/storage/local/content":
			fmt.Fprint(w, `{"data":[{"volid":"local:backup/ct-100.tar","content":"backup"},{"volid":"local:iso/ubuntu.iso","content":"iso"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	content, err := cc.GetStorageContent(context.Background(), "node1", "local")
	if err != nil {
		t.Fatalf("GetStorageContent failed: %v", err)
	}
	if len(content) != 1 || content[0].Content != "backup" {
		t.Fatalf("unexpected storage content: %+v", content)
	}
}

func TestClusterClient_GetNodePendingUpdates_PermissionDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/apt/update" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "permission denied")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	updates, err := cc.GetNodePendingUpdates(context.Background(), "node1")
	if err != nil {
		t.Fatalf("expected permission error to be swallowed, got %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected empty updates on permission error, got %+v", updates)
	}
}
