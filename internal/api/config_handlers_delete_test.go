package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleDeleteNode(t *testing.T) {
	// Setup temporary directory for config persistence
	tempDir, err := os.MkdirTemp("", "pulse-delete-node-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Setup dummy configuration with a PVE node
	dummyCfg := &config.Config{
		PVEInstances: []config.PVEInstance{
			{Name: "pve1", Host: "10.0.0.1"},
			{Name: "pve2", Host: "10.0.0.2"},
		},
		PBSInstances: []config.PBSInstance{
			{Name: "pbs1", Host: "10.0.0.3"},
		},
	}
	dummyCfg.DataPath = tempDir

	// Create handler with dummy persistence
	handler := newTestConfigHandlers(t, dummyCfg)

	tests := []struct {
		name           string
		nodeID         string
		expectedStatus int
		verifyDeletion func(*testing.T, *config.Config)
	}{
		{
			name:           "success_delete_pve_node",
			nodeID:         "pve-0", // Delete first PVE node (pve1)
			expectedStatus: http.StatusOK,
			verifyDeletion: func(t *testing.T, c *config.Config) {
				if len(c.PVEInstances) != 1 {
					t.Errorf("expected 1 PVE instance, got %d", len(c.PVEInstances))
				}
				if len(c.PVEInstances) > 0 && c.PVEInstances[0].Name != "pve2" {
					t.Errorf("expected remaining node to be pve2, got %s", c.PVEInstances[0].Name)
				}
			},
		},
		{
			name:           "success_delete_pbs_node",
			nodeID:         "pbs-0",
			expectedStatus: http.StatusOK,
			verifyDeletion: func(t *testing.T, c *config.Config) {
				if len(c.PBSInstances) != 0 {
					t.Errorf("expected 0 PBS instances, got %d", len(c.PBSInstances))
				}
			},
		},
		{
			name:           "fail_invalid_format",
			nodeID:         "invalidid",
			expectedStatus: http.StatusBadRequest,
			verifyDeletion: nil,
		},
		{
			name:           "fail_invalid_index",
			nodeID:         "pve-999",
			expectedStatus: http.StatusNotFound,
			verifyDeletion: nil,
		},
		{
			name:           "fail_invalid_type",
			nodeID:         "unknown-0",
			expectedStatus: http.StatusNotFound, // Handler returns 404 for unknown types
			verifyDeletion: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/config/nodes/"+tt.nodeID, nil)
			w := httptest.NewRecorder()

			handler.HandleDeleteNode(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", w.Code, tt.expectedStatus)
			}

			if tt.verifyDeletion != nil {
				tt.verifyDeletion(t, dummyCfg)
			}

			// For successful requests, ensure response is valid JSON
			if w.Code == http.StatusOK {
				var response map[string]string
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("response was not valid JSON: %v", err)
				}
				if response["status"] != "success" {
					t.Errorf("expected status 'success', got '%s'", response["status"])
				}
			}
		})
	}
}
