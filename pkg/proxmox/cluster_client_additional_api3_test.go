package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClusterClient_GetContainerInterfaces_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/interfaces" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "boom")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	_, err := cc.GetContainerInterfaces(context.Background(), "node1", 101)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}
