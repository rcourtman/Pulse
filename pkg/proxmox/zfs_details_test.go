package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetZFSPoolsWithDetails_MergesDetailWhenAvailable(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("expected Authorization header to be set")
		}

		switch r.URL.Path {
		case "/api2/json/nodes/pve1/disks/zfs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []ZFSPoolStatus{
					{Name: "rpool", Health: "ONLINE", Size: 100, Alloc: 60, Free: 40, Frag: 1, Dedup: 1.0},
					{Name: "data", Health: "DEGRADED", Size: 200, Alloc: 150, Free: 50, Frag: 10, Dedup: 1.2},
				},
			})
			return
		case "/api2/json/nodes/pve1/disks/zfs/rpool":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": ZFSPoolDetail{
					Name:   "rpool",
					State:  "ONLINE",
					Status: "ok",
					Scan:   "none requested",
					Errors: "No known data errors",
					Children: []ZFSPoolDevice{
						{
							Name:  "mirror-0",
							State: "DEGRADED",
							Leaf:  0,
							Children: []ZFSPoolDevice{
								{Name: "sda", State: "ONLINE", Leaf: 1, Read: 1},
								{Name: "sdb", State: "ONLINE", Leaf: 1},
							},
						},
					},
				},
			})
			return
		case "/api2/json/nodes/pve1/disks/zfs/data":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	pools, err := client.GetZFSPoolsWithDetails(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("GetZFSPoolsWithDetails: %v", err)
	}
	if len(pools) != 2 {
		t.Fatalf("expected 2 pools, got %d", len(pools))
	}

	var rpool ZFSPoolInfo
	var dataPool ZFSPoolInfo
	for _, p := range pools {
		switch p.Name {
		case "rpool":
			rpool = p
		case "data":
			dataPool = p
		}
	}

	if rpool.State != "ONLINE" || rpool.Status != "ok" || rpool.Errors == "" || len(rpool.Devices) == 0 {
		t.Fatalf("expected details to be applied to rpool, got: %+v", rpool)
	}
	if dataPool.State != "" || dataPool.Status != "" || dataPool.Errors != "" || len(dataPool.Devices) != 0 {
		t.Fatalf("expected data pool to fall back to list-only info, got: %+v", dataPool)
	}
}
