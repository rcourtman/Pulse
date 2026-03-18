package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetReplicationStatusIgnoresOversizedStatusBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/replication":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"job-1","source":"node1"}]}`)
		case "/api2/json/nodes/node1/replication/job-1/status":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":%q}`, strings.Repeat("x", int(maxResponseBodyBytes)+1))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
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

	jobs, err := client.GetReplicationStatus(context.Background())
	if err != nil {
		t.Fatalf("GetReplicationStatus() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "job-1" {
		t.Fatalf("expected job ID job-1, got %q", jobs[0].ID)
	}
	if jobs[0].LastSyncTime != nil {
		t.Fatalf("expected nil LastSyncTime when status body is oversized, got %v", jobs[0].LastSyncTime)
	}
}
