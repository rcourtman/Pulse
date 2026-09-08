package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A cluster entry endpoint may proxy a different target node. Neither the
// selected endpoint nor its successful inventory read grants apt access.
func TestClusterClient_PendingUpdatesConfiguredCredentialAndTarget(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var reads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
				}
				if r.Header.Get("Authorization") != "PVEAPIToken=monitor@pve!readonly=synthetic" {
					t.Error("configured token was not preserved")
				}
				if r.Header.Get("Cookie") != "" {
					t.Error("unexpected browser/session credential")
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"entry","status":"online"},{"node":"target","status":"online"}]}`)
				case "/api2/json/nodes/target/apt/update":
					reads.Add(1)
					w.WriteHeader(status)
					if status == http.StatusOK {
						fmt.Fprint(w, `{"data":[{"Package":"example","Version":"2"}]}`)
					} else {
						fmt.Fprint(w, `{"message":"Permission check failed"}`)
					}
				default:
					t.Errorf("unexpected request path %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			cfg := ClientConfig{Host: server.URL, TokenName: "monitor@pve!readonly", TokenValue: "synthetic"}
			cc := NewClusterClient("routing-test", cfg, []string{server.URL}, nil)
			updates, err := cc.GetNodePendingUpdates(context.Background(), "target")
			if status == http.StatusOK {
				if err != nil || len(updates) != 1 || updates[0].Package != "example" {
					t.Fatalf("unexpected successful result: %v, %v", updates, err)
				}
			} else if err == nil || extractStatusCode(err.Error()) != http.StatusForbidden || len(updates) != 0 {
				t.Fatalf("denial not preserved: %v, %v", updates, err)
			}
			if reads.Load() != 1 {
				t.Fatalf("apt reads = %d; expected one without auth retry", reads.Load())
			}
			if !cc.GetHealthStatus()[server.URL] {
				t.Error("apt permission failure must not poison cluster endpoint health")
			}
		})
	}
}
