package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetNodesRejectsOversizedErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.Error(w, strings.Repeat("x", int(maxResponseBodyBytes)+1), http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.GetNodes(context.Background())
	if err == nil {
		t.Fatal("expected oversized body error, got nil")
	}
	if !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("expected size-limit error, got: %v", err)
	}
}
