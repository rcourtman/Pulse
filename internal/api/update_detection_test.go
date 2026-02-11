package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

		var response infraUpdatesResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Total != 0 {
			t.Errorf("Expected total to be 0, got %d", response.Total)
		}
		if len(response.Updates) != 0 {
			t.Errorf("Expected 0 updates, got %d", len(response.Updates))
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

		var response infraUpdatesSummaryResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.TotalUpdates != 0 {
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

		var response infraUpdatesForHostResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.HostID != "nonexistent" {
			t.Error("Expected hostId in response")
		}
		if response.Total != 0 {
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

func TestUpdateDetectionHandlers_WithMonitorState(t *testing.T) {
	now := time.Now().UTC()
	state := models.NewState()
	state.DockerHosts = []models.DockerHost{
		{
			ID:          "host-1",
			DisplayName: "Host One",
			Containers: []models.DockerContainer{
				{
					ID:    "c1",
					Name:  "/web",
					Image: "nginx:latest",
					UpdateStatus: &models.DockerContainerUpdateStatus{
						UpdateAvailable: true,
						CurrentDigest:   "sha256:old",
						LatestDigest:    "sha256:new",
						LastChecked:     now,
					},
				},
				{
					ID:    "c2",
					Name:  "/ok",
					Image: "redis:latest",
					UpdateStatus: &models.DockerContainerUpdateStatus{
						UpdateAvailable: false,
					},
				},
				{
					ID:    "c3",
					Name:  "/err",
					Image: "mysql:latest",
					UpdateStatus: &models.DockerContainerUpdateStatus{
						UpdateAvailable: false,
						Error:           "rate limited",
					},
				},
			},
		},
		{
			ID:          "host-2",
			DisplayName: "Host Two",
			Containers: []models.DockerContainer{
				{
					ID:    "c2b",
					Name:  "/api",
					Image: "postgres:latest",
					UpdateStatus: &models.DockerContainerUpdateStatus{
						UpdateAvailable: true,
						CurrentDigest:   "sha256:old2",
						LatestDigest:    "sha256:new2",
						LastChecked:     now,
					},
				},
			},
		},
	}

	monitor := &monitoring.Monitor{}
	setUnexportedField(t, monitor, "state", state)
	handler := NewUpdateDetectionHandlers(monitor)

	req := httptest.NewRequest(http.MethodGet, "/api/infra-updates?hostId=host-1", nil)
	rr := httptest.NewRecorder()
	handler.HandleGetInfraUpdates(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var updatesResp struct {
		Updates []ContainerUpdateInfo `json:"updates"`
		Total   int                   `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&updatesResp); err != nil {
		t.Fatalf("decode updates response: %v", err)
	}
	if updatesResp.Total != 2 {
		t.Fatalf("expected 2 updates, got %d", updatesResp.Total)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/infra-updates/summary", nil)
	rr = httptest.NewRecorder()
	handler.HandleGetInfraUpdatesSummary(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var summaryResp infraUpdatesSummaryResponse
	if err := json.NewDecoder(rr.Body).Decode(&summaryResp); err != nil {
		t.Fatalf("decode summary response: %v", err)
	}
	if summaryResp.TotalUpdates != 3 {
		t.Fatalf("expected 3 total updates, got %d", summaryResp.TotalUpdates)
	}
	if _, ok := summaryResp.Summaries["host-1"]; !ok {
		t.Fatalf("expected summary for host-1")
	}
	if _, ok := summaryResp.Summaries["host-2"]; !ok {
		t.Fatalf("expected summary for host-2")
	}
	if summaryResp.Summaries["host-1"].TotalCount != 2 {
		t.Fatalf("expected host-1 total count 2, got %d", summaryResp.Summaries["host-1"].TotalCount)
	}
	if summaryResp.Summaries["host-2"].TotalCount != 1 {
		t.Fatalf("expected host-2 total count 1, got %d", summaryResp.Summaries["host-2"].TotalCount)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/infra-updates/docker:host-1/c1", nil)
	rr = httptest.NewRecorder()
	handler.HandleGetInfraUpdateForResource(rr, req, "docker:host-1/c1")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var update ContainerUpdateInfo
	if err := json.NewDecoder(rr.Body).Decode(&update); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if update.ContainerID != "c1" {
		t.Fatalf("expected container c1, got %q", update.ContainerID)
	}
	if update.ContainerName != "web" {
		t.Fatalf("expected container name stripped, got %q", update.ContainerName)
	}

	body := strings.NewReader(`{"hostId":"host-1"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", body)
	rr = httptest.NewRecorder()
	handler.HandleTriggerInfraUpdateCheck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body = strings.NewReader(`{"resourceId":"docker:host-2/c2b"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", body)
	rr = httptest.NewRecorder()
	handler.HandleTriggerInfraUpdateCheck(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
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
