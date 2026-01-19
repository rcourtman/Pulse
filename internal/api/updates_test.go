package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

// MockUpdateManager implements UpdateManager interface for testing
type MockUpdateManager struct {
	CheckForUpdatesFunc    func(ctx context.Context, channel string) (*updates.UpdateInfo, error)
	ApplyUpdateFunc        func(ctx context.Context, req updates.ApplyUpdateRequest) error
	GetStatusFunc          func() updates.UpdateStatus
	GetSSECachedStatusFunc func() (updates.UpdateStatus, time.Time)
	AddSSEClientFunc       func(w http.ResponseWriter, clientID string) *updates.SSEClient
	RemoveSSEClientFunc    func(clientID string)
}

func (m *MockUpdateManager) CheckForUpdatesWithChannel(ctx context.Context, channel string) (*updates.UpdateInfo, error) {
	if m.CheckForUpdatesFunc != nil {
		return m.CheckForUpdatesFunc(ctx, channel)
	}
	return nil, nil
}

func (m *MockUpdateManager) ApplyUpdate(ctx context.Context, req updates.ApplyUpdateRequest) error {
	if m.ApplyUpdateFunc != nil {
		return m.ApplyUpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockUpdateManager) GetStatus() updates.UpdateStatus {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc()
	}
	return updates.UpdateStatus{}
}

func (m *MockUpdateManager) GetSSECachedStatus() (updates.UpdateStatus, time.Time) {
	if m.GetSSECachedStatusFunc != nil {
		return m.GetSSECachedStatusFunc()
	}
	return updates.UpdateStatus{}, time.Time{}
}

func (m *MockUpdateManager) AddSSEClient(w http.ResponseWriter, clientID string) *updates.SSEClient {
	if m.AddSSEClientFunc != nil {
		return m.AddSSEClientFunc(w, clientID)
	}
	return nil
}

func (m *MockUpdateManager) RemoveSSEClient(clientID string) {
	if m.RemoveSSEClientFunc != nil {
		m.RemoveSSEClientFunc(clientID)
	}
}

func TestHandleCheckUpdates_Success(t *testing.T) {
	mockManager := &MockUpdateManager{
		CheckForUpdatesFunc: func(ctx context.Context, channel string) (*updates.UpdateInfo, error) {
			return &updates.UpdateInfo{
				Available:      true,
				LatestVersion:  "v1.2.3",
				CurrentVersion: "v1.0.0",
			}, nil
		},
	}

	h := NewUpdateHandlers(mockManager, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/check", nil)

	h.HandleCheckUpdates(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var info updates.UpdateInfo
	json.NewDecoder(w.Body).Decode(&info)
	if !info.Available || info.LatestVersion != "v1.2.3" {
		t.Errorf("Unexpected response: %+v", info)
	}
}

func TestHandleCheckUpdates_Error(t *testing.T) {
	mockManager := &MockUpdateManager{
		CheckForUpdatesFunc: func(ctx context.Context, channel string) (*updates.UpdateInfo, error) {
			return nil, errors.New("github down")
		},
	}

	h := NewUpdateHandlers(mockManager, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/check", nil)

	h.HandleCheckUpdates(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandleApplyUpdate_Success(t *testing.T) {
	mockManager := &MockUpdateManager{
		ApplyUpdateFunc: func(ctx context.Context, req updates.ApplyUpdateRequest) error {
			if req.DownloadURL != "http://example.com/update.tar.gz" {
				return errors.New("wrong url")
			}
			return nil
		},
	}

	h := NewUpdateHandlers(mockManager, nil)
	w := httptest.NewRecorder()
	body := `{"downloadUrl": "http://example.com/update.tar.gz"}`
	r := httptest.NewRequest("POST", "/updates/apply", strings.NewReader(body))

	h.HandleApplyUpdate(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Note: ApplyUpdate runs in background, so we just check it was accepted
}

func TestHandleUpdateStatus_Fresh(t *testing.T) {
	mockManager := &MockUpdateManager{
		GetStatusFunc: func() updates.UpdateStatus {
			return updates.UpdateStatus{Status: "idle"}
		},
	}

	h := NewUpdateHandlers(mockManager, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/status", nil)

	h.HandleUpdateStatus(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("X-Cache") != "MISS" {
		t.Error("Expected X-Cache: MISS")
	}
}

func TestHandleUpdateStatus_Cached(t *testing.T) {
	mockManager := &MockUpdateManager{
		GetStatusFunc: func() updates.UpdateStatus {
			return updates.UpdateStatus{Status: "fresh"}
		},
		GetSSECachedStatusFunc: func() (updates.UpdateStatus, time.Time) {
			return updates.UpdateStatus{Status: "cached"}, time.Now()
		},
	}

	h := NewUpdateHandlers(mockManager, nil)

	// First request - MISS
	r1 := httptest.NewRequest("GET", "/updates/status", nil)
	r1.RemoteAddr = "1.2.3.4:1234"
	w1 := httptest.NewRecorder()
	h.HandleUpdateStatus(w1, r1)

	if w1.Header().Get("X-Cache") != "MISS" {
		t.Error("Expected first request to be MISS")
	}

	// Second request immediately after - HIT
	r2 := httptest.NewRequest("GET", "/updates/status", nil)
	r2.RemoteAddr = "1.2.3.4:5678" // Same IP
	w2 := httptest.NewRecorder()
	h.HandleUpdateStatus(w2, r2)

	if w2.Header().Get("X-Cache") != "HIT" {
		t.Error("Expected second request to be HIT")
	}

	var status updates.UpdateStatus
	json.NewDecoder(w2.Body).Decode(&status)
	if status.Status != "cached" {
		t.Errorf("Expected cached status, got %s", status.Status)
	}
}

func TestHandleUpdateStream(t *testing.T) {
	mockManager := &MockUpdateManager{
		AddSSEClientFunc: func(w http.ResponseWriter, clientID string) *updates.SSEClient {
			return &updates.SSEClient{
				ID:      clientID,
				Done:    make(chan bool),
				Flusher: w.(http.Flusher),
			}
		},
		RemoveSSEClientFunc: func(clientID string) {},
	}

	h := NewUpdateHandlers(mockManager, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/stream", nil)

	// Create context that we can cancel to simulate client disconnect
	ctx, cancel := context.WithCancel(context.Background())
	r = r.WithContext(ctx)

	// This blocks until context cancel, so run in goroutine
	done := make(chan bool)
	go func() {
		h.HandleUpdateStream(w, r)
		close(done)
	}()

	// Give it a moment to establish
	time.Sleep(50 * time.Millisecond)

	// Cancel/Disconnect
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("HandleUpdateStream didn't return after context cancel")
	}

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("Expected text/event-stream content type")
	}
}

func TestHandleListUpdateHistory(t *testing.T) {
	tmp := t.TempDir()
	history, _ := updates.NewUpdateHistory(tmp)

	// Pre-populate history
	history.CreateEntry(context.Background(), updates.UpdateHistoryEntry{
		EventID:   "test-entry",
		Status:    updates.StatusSuccess,
		VersionTo: "v1.2.3",
	})

	h := NewUpdateHandlers(&MockUpdateManager{}, history)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/history", nil)

	h.HandleListUpdateHistory(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var entries []updates.UpdateHistoryEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
}

func TestHandleGetUpdateHistoryEntry(t *testing.T) {
	tmp := t.TempDir()
	history, _ := updates.NewUpdateHistory(tmp)

	// Pre-populate history
	history.CreateEntry(context.Background(), updates.UpdateHistoryEntry{
		EventID:   "test-entry-1",
		Status:    updates.StatusSuccess,
		VersionTo: "v1.2.3",
	})

	h := NewUpdateHandlers(&MockUpdateManager{}, history)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/updates/history/entry?id=test-entry-1", nil)

	h.HandleGetUpdateHistoryEntry(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var entry updates.UpdateHistoryEntry
	json.NewDecoder(w.Body).Decode(&entry)
	if entry.EventID != "test-entry-1" {
		t.Errorf("Expected EventID test-entry-1, got %s", entry.EventID)
	}
}

func TestGetClientIP(t *testing.T) {
	// Re-include the IP tests as they were useful
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expected   string
	}{
		{"RemoteAddr", "1.2.3.4:1234", nil, "1.2.3.4"},
		{"XFF", "1.1.1.1:1234", map[string]string{"X-Forwarded-For": "2.2.2.2"}, "2.2.2.2"},
		{"X-Real-IP", "1.1.1.1:1234", map[string]string{"X-Real-IP": "3.3.3.3"}, "3.3.3.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			// getClientIP is strict internal but exposed via tests in same package
			ip := getClientIP(r)
			if ip != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestDoCleanupRateLimits(t *testing.T) {
	h := NewUpdateHandlers(nil, nil)
	now := time.Now()
	h.statusRateLimits["old"] = now.Add(-15 * time.Minute)
	h.statusRateLimits["new"] = now.Add(-5 * time.Minute)

	h.doCleanupRateLimits(now)

	if _, ok := h.statusRateLimits["old"]; ok {
		t.Error("Old entry not cleaned up")
	}
	if _, ok := h.statusRateLimits["new"]; !ok {
		t.Error("New entry cleaned up prematurely")
	}
}

type mockUpdater struct {
	updates.Updater
	prepareFunc func(ctx context.Context, req updates.UpdateRequest) (*updates.UpdatePlan, error)
}

func (m *mockUpdater) PrepareUpdate(ctx context.Context, req updates.UpdateRequest) (*updates.UpdatePlan, error) {
	return m.prepareFunc(ctx, req)
}

func TestHandleGetUpdatePlan(t *testing.T) {
	// Set mock mode so GetCurrentVersion returns "mock"
	t.Setenv("PULSE_MOCK_MODE", "true")

	mu := &mockUpdater{
		prepareFunc: func(ctx context.Context, req updates.UpdateRequest) (*updates.UpdatePlan, error) {
			return &updates.UpdatePlan{
				Instructions: []string{"test"},
			}, nil
		},
	}

	h := NewUpdateHandlers(nil, nil)
	h.registry.Register("mock", mu)

	// Test missing version
	r := httptest.NewRequest("GET", "/api/updates/plan", nil)
	w := httptest.NewRecorder()
	h.HandleGetUpdatePlan(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	// Test success
	r = httptest.NewRequest("GET", "/api/updates/plan?version=v1.2.3", nil)
	w = httptest.NewRecorder()
	h.HandleGetUpdatePlan(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var plan updates.UpdatePlan
	json.NewDecoder(w.Body).Decode(&plan)
	if len(plan.Instructions) != 1 {
		t.Errorf("Expected 1 instruction, got %d", len(plan.Instructions))
	}
}
