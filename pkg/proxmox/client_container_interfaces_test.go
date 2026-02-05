package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetContainerInterfaces_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/interfaces" {
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

	_, err = client.GetContainerInterfaces(context.Background(), "node1", 101)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "failed to get container interfaces") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetContainerInterfaces_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/interfaces" {
			fmt.Fprint(w, `{"data":[{"name":"eth0","ip-addresses":[{"ip-address":"10.0.0.2","ip-address-type":"ipv4"}]}]}`)
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

	ifaces, err := client.GetContainerInterfaces(context.Background(), "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}
	if len(ifaces[0].IPAddresses) != 1 || ifaces[0].IPAddresses[0].Address != "10.0.0.2" {
		t.Fatalf("unexpected ip addresses: %+v", ifaces[0].IPAddresses)
	}
}
