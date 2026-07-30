package pbs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestGetDatastoresFilteredSkipsDetailRequestsForExcludedStores(t *testing.T) {
	var mu sync.Mutex
	var requestedPaths []string

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedPaths = append(requestedPaths, r.URL.Path)
		mu.Unlock()

		switch r.URL.Path {
		case "/api2/json/admin/datastore":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{
					{"store": "internal"},
					{"store": "exthdd1500gb"},
				},
			})
		case "/api2/json/admin/datastore/internal/rrd":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case "/api2/json/admin/datastore/internal/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"total": 100.0,
					"used":  25.0,
					"avail": 75.0,
				},
			})
		case "/api2/json/admin/datastore/internal/gc":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	})
	defer server.Close()

	datastores, err := client.GetDatastoresFiltered(
		context.Background(),
		func(name string) bool { return name != "exthdd1500gb" },
	)
	if err != nil {
		t.Fatalf("GetDatastoresFiltered failed: %v", err)
	}
	if len(datastores) != 1 || datastores[0].Store != "internal" {
		t.Fatalf("datastores = %+v, want only internal", datastores)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range requestedPaths {
		if strings.Contains(path, "exthdd1500gb") {
			t.Fatalf("excluded datastore received a detail request: %s", path)
		}
	}
}
