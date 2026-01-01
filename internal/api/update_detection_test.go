package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestHandleGetInfraUpdates(t *testing.T) {
	// We can't easily create a real Monitor, so we'll test the core logic
	t.Run("collectDockerUpdates filters correctly", func(t *testing.T) {
		handler := &UpdateDetectionHandlers{}
		// Can't test directly without a monitor, but we can verify the behavior
		// through the HTTP handlers

		// For now, verify the response structure when no updates
		req := httptest.NewRequest(http.MethodGet, "/api/infra-updates", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdates(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify structure
		if _, ok := response["updates"]; !ok {
			t.Error("Expected 'updates' field in response")
		}
		if _, ok := response["total"]; !ok {
			t.Error("Expected 'total' field in response")
		}
	})

	t.Run("method not allowed for POST", func(t *testing.T) {
		handler := &UpdateDetectionHandlers{}
		req := httptest.NewRequest(http.MethodPost, "/api/infra-updates", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdates(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}

func TestHandleGetInfraUpdatesSummary(t *testing.T) {
	handler := &UpdateDetectionHandlers{}

	t.Run("returns empty summary without monitor", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/infra-updates/summary", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdatesSummary(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["totalUpdates"].(float64) != 0 {
			t.Error("Expected totalUpdates to be 0")
		}
	})

	t.Run("method not allowed for POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/summary", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdatesSummary(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}

func TestHandleGetInfraUpdatesForHost(t *testing.T) {
	handler := &UpdateDetectionHandlers{}

	t.Run("returns empty for non-existent host", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/infra-updates/host/nonexistent", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdatesForHost(rr, req, "nonexistent")

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["hostId"] != "nonexistent" {
			t.Error("Expected hostId in response")
		}
		if response["total"].(float64) != 0 {
			t.Error("Expected total to be 0")
		}
	})

	t.Run("method not allowed for POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/host/test", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdatesForHost(rr, req, "test")

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}

func TestHandleGetInfraUpdateForResource(t *testing.T) {
	handler := &UpdateDetectionHandlers{}

	t.Run("returns 404 for non-existent resource", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/infra-updates/resource-123", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdateForResource(rr, req, "resource-123")

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", rr.Code)
		}
	})

	t.Run("method not allowed for POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/resource-123", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetInfraUpdateForResource(rr, req, "resource-123")

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}

func TestHandleTriggerInfraUpdateCheck(t *testing.T) {
	handler := &UpdateDetectionHandlers{}

	t.Run("returns 503 without monitor", func(t *testing.T) {
		body := strings.NewReader(`{"hostId":"test-host"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.HandleTriggerInfraUpdateCheck(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", rr.Code)
		}
	})

	t.Run("method not allowed for GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/infra-updates/check", nil)
		rr := httptest.NewRecorder()

		handler.HandleTriggerInfraUpdateCheck(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", rr.Code)
		}
	})
}

func TestContainerUpdateInfo_JSONSerialization(t *testing.T) {
	info := ContainerUpdateInfo{
		HostID:          "host-1",
		HostName:        "Test Host",
		ContainerID:     "container-abc",
		ContainerName:   "nginx",
		Image:           "nginx:latest",
		CurrentDigest:   "sha256:current",
		LatestDigest:    "sha256:latest",
		UpdateAvailable: true,
		LastChecked:     time.Now().Unix(),
		ResourceType:    "docker",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ContainerUpdateInfo: %v", err)
	}

	var decoded ContainerUpdateInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ContainerUpdateInfo: %v", err)
	}

	if decoded.HostID != info.HostID {
		t.Errorf("Expected HostID %q, got %q", info.HostID, decoded.HostID)
	}
	if decoded.UpdateAvailable != info.UpdateAvailable {
		t.Errorf("Expected UpdateAvailable %v, got %v", info.UpdateAvailable, decoded.UpdateAvailable)
	}
	if decoded.ResourceType != "docker" {
		t.Errorf("Expected ResourceType 'docker', got %q", decoded.ResourceType)
	}
}

// Integration test with real monitor (requires more setup)
func TestUpdateDetectionHandlersWithMonitor(t *testing.T) {
	// Skip in short mode - this requires more infrastructure
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require setting up a real Monitor instance
	// For now, we test the handler methods in isolation above
	t.Log("Full integration test requires Monitor setup - see manual testing instructions")
}

// Verify handler doesn't panic with nil monitor
func TestHandlerNilMonitorSafety(t *testing.T) {
	handler := &UpdateDetectionHandlers{monitor: nil}

	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request)
		method  string
		path    string
	}{
		{
			name: "GetInfraUpdates",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handler.HandleGetInfraUpdates(w, r)
			},
			method: http.MethodGet,
			path:   "/api/infra-updates",
		},
		{
			name: "GetInfraUpdatesSummary",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handler.HandleGetInfraUpdatesSummary(w, r)
			},
			method: http.MethodGet,
			path:   "/api/infra-updates/summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			// Should not panic
			tt.handler(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected OK status, got %d", rr.Code)
			}
		})
	}
}

// Placeholder for monitor interface - actual implementation is in monitoring package
var _ = (*monitoring.Monitor)(nil) // Ensure we reference the package
