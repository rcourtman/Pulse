package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const (
	legacyPatrolAlertIdentifierField = "legacy_alert_id"
	legacyPatrolAlertIDField         = "alert_id"
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

func TestHandleGetPatrolStatus_NoAIService(t *testing.T) {
	t.Parallel()

	// Handler with no legacy AI service should not panic.
	handler := &AISettingsHandler{}

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

	if resp.Running {
		t.Error("expected Running to be false when AI service missing")
	}
	if resp.Enabled {
		t.Error("expected Enabled to be false when AI service missing")
	}
	if !resp.Healthy {
		t.Error("expected Healthy to be true when AI service missing")
	}
	if resp.LicenseStatus == "" {
		t.Error("expected LicenseStatus to be set when AI service missing")
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

func TestHandleGetPatrolRunHistory_EmitsCanonicalAlertIdentifier(t *testing.T) {
	t.Parallel()

	handler := createTestAIHandler(t)
	patrol := &ai.PatrolService{}
	store := ai.NewPatrolRunHistoryStore(10)
	store.Add(ai.PatrolRunRecord{
		ID:               "run-1",
		StartedAt:        time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		CompletedAt:      time.Date(2026, 3, 1, 10, 1, 0, 0, time.UTC),
		DurationMs:       60000,
		Type:             "patrol",
		AlertIdentifier:  "instance:node:100::metric/cpu",
		ResourcesChecked: 1,
		FindingsSummary:  "ok",
		FindingIDs:       []string{},
		Status:           "healthy",
	})
	setUnexportedField(t, patrol, "runHistoryStore", store)
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/runs", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolRunHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var history []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one history item, got %d", len(history))
	}
	if history[0]["alert_identifier"] != "instance:node:100::metric/cpu" {
		t.Fatalf("expected canonical alert_identifier, got %#v", history[0]["alert_identifier"])
	}
	if _, ok := history[0][legacyPatrolAlertIdentifierField]; ok {
		t.Fatalf(
			"did not expect %s in patrol history response, got %#v",
			legacyPatrolAlertIdentifierField,
			history[0][legacyPatrolAlertIdentifierField],
		)
	}
	if _, ok := history[0][legacyPatrolAlertIDField]; ok {
		t.Fatalf(
			"did not expect %s in patrol history response, got %#v",
			legacyPatrolAlertIDField,
			history[0][legacyPatrolAlertIDField],
		)
	}
}

func TestHandleGetPatrolRunHistory_PreservesExplicitEmptySnapshotCollections(t *testing.T) {
	t.Parallel()

	handler := createTestAIHandler(t)
	patrol := &ai.PatrolService{}
	store := ai.NewPatrolRunHistoryStore(10)
	store.Add(ai.PatrolRunRecord{
		ID:                        "run-1",
		StartedAt:                 time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		CompletedAt:               time.Date(2026, 3, 1, 10, 1, 0, 0, time.UTC),
		DurationMs:                60000,
		Type:                      "scoped",
		TriggerReason:             "alert_fired",
		ScopeResourceIDs:          []string{"seed-resource"},
		EffectiveScopeResourceIDs: []string{},
		ScopeResourceTypes:        []string{"vm"},
		ResourcesChecked:          2,
		PMGChecked:                1,
		ExistingFindings:          1,
		RejectedFindings:          1,
		FindingsSummary:           "ok",
		FindingIDs:                []string{},
		Status:                    "healthy",
		TriageFlags:               2,
		TriageSkippedLLM:          true,
		ToolCallCount:             0,
	})
	setUnexportedField(t, patrol, "runHistoryStore", store)
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/runs", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolRunHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var history []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one history item, got %d", len(history))
	}

	run := history[0]
	if values, ok := run["effective_scope_resource_ids"].([]interface{}); !ok || len(values) != 0 {
		t.Fatalf("expected explicit empty effective_scope_resource_ids, got %#v", run["effective_scope_resource_ids"])
	}
	if values, ok := run["finding_ids"].([]interface{}); !ok || len(values) != 0 {
		t.Fatalf("expected explicit empty finding_ids, got %#v", run["finding_ids"])
	}
	if got := run["pmg_checked"]; got != float64(1) {
		t.Fatalf("expected pmg_checked=1, got %#v", got)
	}
	if got := run["rejected_findings"]; got != float64(1) {
		t.Fatalf("expected rejected_findings=1, got %#v", got)
	}
	if got := run["triage_flags"]; got != float64(2) {
		t.Fatalf("expected triage_flags=2, got %#v", got)
	}
	if got := run["triage_skipped_llm"]; got != true {
		t.Fatalf("expected triage_skipped_llm=true, got %#v", got)
	}
}

func TestHandleGetPatrolRunHistory_InvalidOrOversizedLimitIsNormalized(t *testing.T) {
	t.Parallel()

	handler := createTestAIHandler(t)
	patrol := &ai.PatrolService{}
	store := ai.NewPatrolRunHistoryStore(200)
	for i := 0; i < 120; i++ {
		store.Add(ai.PatrolRunRecord{
			ID:               fmt.Sprintf("run-%03d", i),
			StartedAt:        time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
			CompletedAt:      time.Date(2026, 3, 1, 10, 1, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
			DurationMs:       60000,
			Type:             "patrol",
			ResourcesChecked: 1,
			FindingsSummary:  "ok",
			FindingIDs:       []string{},
			Status:           "healthy",
		})
	}
	setUnexportedField(t, patrol, "runHistoryStore", store)
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)

	tests := []struct {
		name       string
		query      string
		wantLength int
	}{
		{name: "zero_uses_default", query: "?limit=0", wantLength: 50},
		{name: "negative_uses_default", query: "?limit=-5", wantLength: 50},
		{name: "cap_at_100", query: "?limit=250", wantLength: 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/runs"+tc.query, nil)
			rec := httptest.NewRecorder()

			handler.HandleGetPatrolRunHistory(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
			}

			var history []map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if len(history) != tc.wantLength {
				t.Fatalf("history length = %d, want %d", len(history), tc.wantLength)
			}
		})
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
