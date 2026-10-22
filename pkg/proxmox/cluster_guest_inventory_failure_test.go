package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterGuestInventoriesKeepUnavailableErrors(t *testing.T) {
	cc := &ClusterClient{
		name: "inventory-failure", endpoints: []string{"https://first.invalid", "https://second.invalid"},
		nodeHealth:      map[string]bool{"https://first.invalid": false, "https://second.invalid": false},
		lastHealthCheck: map[string]time.Time{"https://first.invalid": time.Now(), "https://second.invalid": time.Now()},
		lastError:       map[string]string{"https://first.invalid": "connection refused", "https://second.invalid": "authentication failed"},
	}
	reads := map[string]func(context.Context) (int, error){
		"vm": func(ctx context.Context) (int, error) { rows, err := cc.GetVMs(ctx, "member"); return len(rows), err },
		"container": func(ctx context.Context) (int, error) {
			rows, err := cc.GetContainers(ctx, "member")
			return len(rows), err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			if count, err := read(context.Background()); count != 0 || err == nil || !strings.Contains(err.Error(), "no healthy nodes available") || !strings.Contains(err.Error(), "authentication failed") {
				t.Fatalf("unavailable inventory lost its failure: count=%d err=%v", count, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := read(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}

// Provider/proxy text must not turn a real HTTP failure into successful removal.
// The same configured client then accepts ordinary recovery and genuine emptiness.
func TestClusterGuestInventoriesPreserveQuotedFailuresAndRecovery(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var phase atomic.Int32
			var inventories atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api2/json/nodes" {
					fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
					return
				}
				inventories.Add(1)
				switch phase.Load() {
				case 0:
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"no healthy nodes available"}`)
				case 1:
					fmt.Fprint(w, `{"data":[{"vmid":105,"name":"recovered","status":"running"}]}`)
				default:
					fmt.Fprint(w, `{"data":[]}`)
				}
			}))
			defer server.Close()
			cc := NewClusterClient("inventory-failure", ClientConfig{Host: server.URL, TokenName: "fixture@pve!read", TokenValue: "fixture"}, []string{server.URL}, nil)
			reads := []func() (int, error){
				func() (int, error) { rows, err := cc.GetVMs(context.Background(), "member"); return len(rows), err },
				func() (int, error) {
					rows, err := cc.GetContainers(context.Background(), "member")
					return len(rows), err
				},
			}
			for _, read := range reads {
				if count, err := read(); count != 0 || err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) {
					t.Errorf("HTTP failure became empty success: count=%d err=%v", count, err)
				}
			}
			if inventories.Load() != 2 {
				t.Fatalf("completed HTTP refusal retried: %d reads", inventories.Load())
			}
			phase.Store(1)
			for _, read := range reads {
				if count, err := read(); count != 1 || err != nil {
					t.Errorf("ordinary recovery failed: count=%d err=%v", count, err)
				}
			}
			phase.Store(2)
			for _, read := range reads {
				if count, err := read(); count != 0 || err != nil {
					t.Errorf("authoritative empty inventory refused: count=%d err=%v", count, err)
				}
			}
		})
	}
}
