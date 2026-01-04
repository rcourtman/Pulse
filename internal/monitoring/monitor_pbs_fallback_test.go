package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestMonitor_PollPBSInstance_Fallback_Extra(t *testing.T) {
	// Create a mock PBS server that fails version check but succeeds on datastores
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			http.Error(w, "server error", http.StatusInternalServerError)
		case "/api2/json/admin/datastore":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"store": "store1", "total": 1000, "used": 100},
				},
			})
		case "/api2/json/nodes/localhost/status":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"cpu": 0.1,
					"memory": map[string]interface{}{
						"used":  1024,
						"total": 2048,
					},
					"uptime": 100,
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Initialize PBS Client
	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
		Timeout:    1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	m := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{
				{
					Name:              "pbs-fallback",
					Host:              server.URL,
					MonitorDatastores: true,
				},
			},
		},
		state:            models.NewState(),
		stalenessTracker: NewStalenessTracker(nil),
	}

	ctx := context.Background()
	m.pollPBSInstance(ctx, "pbs-fallback", client)

	// Verify manually using snapshot
	snapshot := m.state.GetSnapshot()
	var inst *models.PBSInstance
	for _, i := range snapshot.PBSInstances {
		if i.ID == "pbs-pbs-fallback" {
			copy := i
			inst = &copy
			break
		}
	}

	if inst == nil {
		t.Fatal("PBS instance not found in state snapshot")
	}

	if inst.Version != "connected" {
		t.Errorf("Expected version 'connected', got '%s'", inst.Version)
	}

	if inst.Status != "online" {
		t.Errorf("Expected status 'online', got '%s'", inst.Status)
	}

	if len(inst.Datastores) != 1 {
		t.Errorf("Expected 1 datastore, got %d", len(inst.Datastores))
	}
}
