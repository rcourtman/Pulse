package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleGetPatrolStatus_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatrolStatus_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// When patrol service is not initialized, should return safe defaults
	if resp.Running {
		t.Error("expected Running to be false when patrol not initialized")
	}
	if resp.Enabled {
		t.Error("expected Enabled to be false when patrol not initialized")
	}
	if !resp.Healthy {
		t.Error("expected Healthy to be true when patrol not initialized (no issues)")
	}
}

func TestHandleGetPatrolFindings_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/findings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolFindings(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatrolFindings_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/findings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Should return empty array when patrol not initialized
	var findings []interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &findings); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected empty findings, got %d", len(findings))
	}
}

func TestHandleGetPatrolFindings_WithResourceIDFilter(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/findings?resource_id=vm-100", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestHandleForcePatrol_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/run", nil)
	rec := httptest.NewRecorder()

	handler.HandleForcePatrol(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleAcknowledgeFinding_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/acknowledge", nil)
	rec := httptest.NewRecorder()

	handler.HandleAcknowledgeFinding(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleSnoozeFinding_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/snooze", nil)
	rec := httptest.NewRecorder()

	handler.HandleSnoozeFinding(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleResolveFinding_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/resolve", nil)
	rec := httptest.NewRecorder()

	handler.HandleResolveFinding(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleDismissFinding_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/dismiss", nil)
	rec := httptest.NewRecorder()

	handler.HandleDismissFinding(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleDismissFinding_InvalidReason(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Set up auth - needed for authenticated handlers
	InitSessionStore(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Invalid dismiss reason
	body, _ := json.Marshal(map[string]string{
		"finding_id": "test-finding",
		"reason":     "invalid_reason_value",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/dismiss", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleDismissFinding(rec, req)

	// Should fail because patrol service not available (after auth check)
	// But more importantly, validates the request handling path
	if rec.Code == http.StatusOK {
		t.Error("expected non-OK status for missing patrol service or invalid input")
	}
}

func TestHandleSnoozeFinding_DurationValidation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	InitSessionStore(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Missing finding_id
	body, _ := json.Marshal(map[string]interface{}{
		"duration_hours": 24,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/snooze", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleSnoozeFinding(rec, req)

	// Should fail auth or patrol checks
	if rec.Code == http.StatusOK {
		t.Error("expected non-OK status")
	}
}

func TestHandleGetFindingsHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/history", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetFindingsHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatrolRunHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/runs", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolRunHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatrolRunHistory_NoPatrolService(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/runs", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolRunHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// Should return empty array
	var history []interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}
}

func TestHandleSuppressFinding_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/suppress", nil)
	rec := httptest.NewRecorder()

	handler.HandleSuppressFinding(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for GET, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}
