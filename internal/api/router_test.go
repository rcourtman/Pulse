package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// mockMonitor implements a minimal monitor for testing
type mockMonitor struct {
	state *models.State
}

func (m *mockMonitor) GetState() models.State {
	if m.state == nil {
		return *models.NewState()
	}
	return *m.state
}

func (m *mockMonitor) GetStartTime() time.Time {
	return time.Now()
}

func TestAPIEndpoints(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Port:            3000,
		AllowedOrigins:  "*",
		AlertsEnabled:   true,
		PollingInterval: 5 * time.Second,
	}

	// Create mock monitor with test data
	monitor := &mockMonitor{
		state: &models.State{
			Nodes: []models.Node{
				{
					ID:       "test-node",
					Name:     "test-node",
					Instance: "test",
					Status:   "online",
					Type:     "node",
					CPU:      10.5,
					Memory: models.Memory{
						Total: 16000000000,
						Used:  8000000000,
						Free:  8000000000,
						Usage: 50.0,
					},
					Disk: models.Disk{
						Total: 100000000000,
						Used:  50000000000,
						Free:  50000000000,
						Usage: 50.0,
					},
					Uptime:    3600,
					LastSeen:  time.Now(),
				},
			},
			VMs: []models.VM{
				{
					ID:       "test-vm",
					VMID:     100,
					Name:     "test-vm",
					Node:     "test-node",
					Instance: "test",
					Status:   "running",
					Type:     "qemu",
					CPU:      5.5,
					CPUs:     2,
					Memory: models.Memory{
						Total: 4000000000,
						Used:  2000000000,
						Free:  2000000000,
						Usage: 50.0,
					},
					NetworkIn:  1000000,
					NetworkOut: 2000000,
					DiskRead:   500000,
					DiskWrite:  600000,
					LastSeen:   time.Now(),
				},
			},
			LastUpdate: time.Now(),
		},
	}

	// Create WebSocket hub
	wsHub := websocket.NewHub(func() interface{} {
		state := monitor.GetState()
		return state.ToFrontend()
	})

	// Create router
	router := NewRouter(cfg, monitor, wsHub)

	// Test cases
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:           "Health endpoint",
			method:         "GET",
			path:           "/api/health",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				if err := json.Unmarshal(body, &health); err != nil {
					t.Fatalf("Failed to unmarshal health response: %v", err)
				}
				if health["status"] != "healthy" {
					t.Errorf("Expected status 'healthy', got %v", health["status"])
				}
			},
		},
		{
			name:           "State endpoint",
			method:         "GET",
			path:           "/api/state",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var state models.StateFrontend
				if err := json.Unmarshal(body, &state); err != nil {
					t.Fatalf("Failed to unmarshal state response: %v", err)
				}
				
				// Validate nodes
				if len(state.Nodes) != 1 {
					t.Errorf("Expected 1 node, got %d", len(state.Nodes))
				}
				if len(state.Nodes) > 0 {
					node := state.Nodes[0]
					if node.ID != "test-node" {
						t.Errorf("Expected node ID 'test-node', got %s", node.ID)
					}
					if node.Mem != 8000000000 {
						t.Errorf("Expected mem 8000000000, got %d", node.Mem)
					}
					if node.MaxMem != 16000000000 {
						t.Errorf("Expected maxmem 16000000000, got %d", node.MaxMem)
					}
				}
				
				// Validate VMs
				if len(state.VMs) != 1 {
					t.Errorf("Expected 1 VM, got %d", len(state.VMs))
				}
				if len(state.VMs) > 0 {
					vm := state.VMs[0]
					if vm.NetIn != 1000000 {
						t.Errorf("Expected netin 1000000, got %d", vm.NetIn)
					}
					if vm.NetOut != 2000000 {
						t.Errorf("Expected netout 2000000, got %d", vm.NetOut)
					}
				}
			},
		},
		{
			name:           "Version endpoint",
			method:         "GET",
			path:           "/api/version",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var version map[string]interface{}
				if err := json.Unmarshal(body, &version); err != nil {
					t.Fatalf("Failed to unmarshal version response: %v", err)
				}
				if version["runtime"] != "go" {
					t.Errorf("Expected runtime 'go', got %v", version["runtime"])
				}
			},
		},
		{
			name:           "Invalid method",
			method:         "POST",
			path:           "/api/health",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			
			router.ServeHTTP(rec, req)
			
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			
			if tt.validateBody != nil && rec.Code == http.StatusOK {
				tt.validateBody(t, rec.Body.Bytes())
			}
		})
	}
}

// TestFieldNameConsistency ensures frontend field names match expectations
func TestFieldNameConsistency(t *testing.T) {
	// Create a test state
	state := models.State{
		Nodes: []models.Node{{
			Memory: models.Memory{Used: 1000, Total: 2000},
			Disk:   models.Disk{Used: 3000, Total: 4000},
		}},
		VMs: []models.VM{{
			NetworkIn:  5000,
			NetworkOut: 6000,
			DiskRead:   7000,
			DiskWrite:  8000,
		}},
	}

	// Convert to frontend format
	frontend := state.ToFrontend()
	
	// Marshal to JSON
	data, err := json.Marshal(frontend)
	if err != nil {
		t.Fatalf("Failed to marshal frontend state: %v", err)
	}
	
	// Parse back as generic map to check field names
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	// Check nodes have correct field names
	nodes := result["nodes"].([]interface{})
	if len(nodes) > 0 {
		node := nodes[0].(map[string]interface{})
		
		// These should be the frontend field names
		requiredFields := []string{"mem", "maxmem", "disk", "maxdisk"}
		for _, field := range requiredFields {
			if _, ok := node[field]; !ok {
				t.Errorf("Node missing required field '%s'", field)
			}
		}
		
		// These should NOT exist (backend names)
		prohibitedFields := []string{"memory", "Memory"}
		for _, field := range prohibitedFields {
			if _, ok := node[field]; ok {
				t.Errorf("Node has prohibited field '%s'", field)
			}
		}
	}
	
	// Check VMs have correct field names
	vms := result["vms"].([]interface{})
	if len(vms) > 0 {
		vm := vms[0].(map[string]interface{})
		
		// Check I/O field names
		ioFields := map[string]string{
			"netin":     "NetworkIn",
			"netout":    "NetworkOut", 
			"diskread":  "DiskRead",
			"diskwrite": "DiskWrite",
		}
		
		for frontend, backend := range ioFields {
			if _, ok := vm[frontend]; !ok {
				t.Errorf("VM missing required field '%s'", frontend)
			}
			if _, ok := vm[backend]; ok {
				t.Errorf("VM has backend field '%s' instead of '%s'", backend, frontend)
			}
		}
	}
}