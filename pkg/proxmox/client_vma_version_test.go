package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetVMAgentVersion_MapResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/info" {
			fmt.Fprint(w, `{"data":{"result":{"version":{"version":"3.1"}}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	version, err := client.GetVMAgentVersion(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentVersion failed: %v", err)
	}
	if version != "3.1" {
		t.Fatalf("unexpected version: %s", version)
	}
}

func TestClient_GetVMAgentVersion_NoVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/info" {
			fmt.Fprint(w, `{"data":{"result":{"foo":"bar"}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	version, err := client.GetVMAgentVersion(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentVersion failed: %v", err)
	}
	if version != "" {
		t.Fatalf("expected empty version, got %q", version)
	}
}
