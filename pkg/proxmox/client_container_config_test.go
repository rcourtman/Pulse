package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetContainerConfig_NullData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/config" {
			fmt.Fprint(w, `{"data":null}`)
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

	cfg, err := client.GetContainerConfig(context.Background(), "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config map")
	}
	if len(cfg) != 0 {
		t.Fatalf("expected empty config, got %+v", cfg)
	}
}
