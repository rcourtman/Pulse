package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestPollPBSInstanceDoesNotQueryExcludedDatastoreDetails(t *testing.T) {
	var mu sync.Mutex
	var requestedPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestedPaths = append(requestedPaths, r.URL.Path)
		mu.Unlock()

		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"version": "3.4"},
			})
		case "/api2/json/nodes/localhost/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
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
		case "/api2/json/admin/datastore/internal/namespace":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!pulse-token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	monitor := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{{
				Name:              "pbs-excludes",
				Host:              server.URL,
				MonitorDatastores: true,
				ExcludeDatastores: []string{"ext*"},
			}},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	monitor.pollPBSInstance(context.Background(), "pbs-excludes", client)

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("PBS instances = %+v, want one", snapshot.PBSInstances)
	}
	datastores := snapshot.PBSInstances[0].Datastores
	if len(datastores) != 1 || datastores[0].Name != "internal" {
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
