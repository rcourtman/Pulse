package proxmox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClusterClient_InitialHealthCheck_VMErrorTreatedHealthy(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "No QEMU guest agent")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node2","status":"online"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server2.Close()

	cfg := ClientConfig{Host: server1.URL, TokenName: "u@p!t", TokenValue: "v", VerifySSL: false}
	cc := NewClusterClient("test", cfg, []string{server1.URL, server2.URL}, nil)

	health := cc.GetHealthStatusWithErrors()
	if !health[server1.URL].Healthy {
		t.Fatalf("expected vm-specific error endpoint to be healthy, got %+v", health[server1.URL])
	}
	if health[server1.URL].LastError != "" {
		t.Fatalf("expected no last error, got %q", health[server1.URL].LastError)
	}
	if !health[server2.URL].Healthy {
		t.Fatalf("expected second endpoint healthy, got %+v", health[server2.URL])
	}
}

func TestClusterClient_InitialHealthCheck_NewClientFailure(t *testing.T) {
	endpoints := []string{"http://example.invalid", "http://example2.invalid"}
	cfg := ClientConfig{Host: endpoints[0], User: "invalid", Password: "pw"}
	cc := NewClusterClient("test", cfg, endpoints, nil)

	health := cc.GetHealthStatusWithErrors()
	for _, ep := range endpoints {
		status := health[ep]
		if status.Healthy {
			t.Fatalf("expected %s to be unhealthy", ep)
		}
		if !strings.Contains(status.LastError, "invalid user format") {
			t.Fatalf("unexpected error for %s: %q", ep, status.LastError)
		}
	}
}
