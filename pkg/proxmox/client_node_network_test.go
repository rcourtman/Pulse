package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetNodeNetworkInterfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/network" {
			fmt.Fprint(w, `{"data":[{"iface":"vmbr0","type":"bridge","address":"10.0.0.1"}]}`)
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

	ifaces, err := client.GetNodeNetworkInterfaces(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetNodeNetworkInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Iface != "vmbr0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}
}

func TestClient_GetNodeNetworkInterfaces_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes/node1/network" {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte("no content"))
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

	_, err = client.GetNodeNetworkInterfaces(context.Background(), "node1")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "failed to get node network interfaces") {
		t.Fatalf("unexpected error: %v", err)
	}
}
