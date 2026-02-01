package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func setupAIHandlerWithIntelligence(t *testing.T) (*AISettingsHandler, *ai.PatrolService) {
	t.Helper()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})

	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	return handler, patrol
}

func seedPatternDetector(now time.Time) *ai.PatternDetector {
	detector := ai.NewPatternDetector(ai.PatternDetectorConfig{
		MinOccurrences:  2,
		PatternWindow:   24 * time.Hour,
		PredictionLimit: 48 * time.Hour,
	})

	detector.RecordEvent(ai.HistoricalEvent{
		ResourceID: "vm-1",
		EventType:  ai.EventHighCPU,
		Timestamp:  now.Add(-1 * time.Hour),
		Duration:   10 * time.Minute,
	})
	detector.RecordEvent(ai.HistoricalEvent{
		ResourceID: "vm-1",
		EventType:  ai.EventHighCPU,
		Timestamp:  now,
		Duration:   5 * time.Minute,
	})

	return detector
}

func seedCorrelationDetector(now time.Time) *ai.CorrelationDetector {
	detector := ai.NewCorrelationDetector(ai.CorrelationConfig{
		MinOccurrences:    1,
		CorrelationWindow: 10 * time.Minute,
		RetentionWindow:   24 * time.Hour,
		MaxEvents:         100,
	})

	detector.RecordEvent(ai.CorrelationEvent{
		ResourceID:   "node-1",
		ResourceName: "node-1",
		ResourceType: "node",
		EventType:    ai.CorrelationEventHighCPU,
		Timestamp:    now.Add(-2 * time.Minute),
	})
	detector.RecordEvent(ai.CorrelationEvent{
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
		EventType:    ai.CorrelationEventRestart,
		Timestamp:    now.Add(-1 * time.Minute),
	})

	return detector
}

func TestHandleGetPatterns_UnlockedWithData(t *testing.T) {
	// License gates were removed from intelligence endpoints (9279358c).
	// Even with allow=false, data is returned without redaction.
	t.Setenv("PULSE_MOCK_MODE", "true")
	handler, _ := setupAIHandlerWithIntelligence(t)

	handler.legacyAIService.SetPatternDetector(seedPatternDetector(time.Now()))
	handler.legacyAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/patterns", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatterns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-License-Required") != "" {
		t.Fatalf("expected no license header after license gates removed, got %q", rec.Header().Get("X-License-Required"))
	}

	var resp struct {
		Patterns        []map[string]interface{} `json:"patterns"`
		Count           int                      `json:"count"`
		LicenseRequired bool                     `json:"license_required"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 {
		t.Fatalf("count = %d, want 1", resp.Count)
	}
	if len(resp.Patterns) != 1 {
		t.Fatalf("expected patterns to be returned (not redacted), got %d", len(resp.Patterns))
	}
	if resp.LicenseRequired {
		t.Fatalf("expected license_required=false")
	}
}

func TestHandleGetPredictions_WithData(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	handler, _ := setupAIHandlerWithIntelligence(t)

	handler.legacyAIService.SetPatternDetector(seedPatternDetector(time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/predictions?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPredictions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Predictions []struct {
			ResourceID string `json:"resource_id"`
			IsOverdue  bool   `json:"is_overdue"`
		} `json:"predictions"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Predictions) != 1 {
		t.Fatalf("predictions count = %d, want 1", resp.Count)
	}
	if resp.Predictions[0].ResourceID != "vm-1" {
		t.Fatalf("resource_id = %s, want vm-1", resp.Predictions[0].ResourceID)
	}
	if resp.Predictions[0].IsOverdue {
		t.Fatalf("expected prediction to not be overdue")
	}
}

func TestHandleGetCorrelations_WithData(t *testing.T) {
	t.Setenv("PULSE_MOCK_MODE", "true")
	handler, _ := setupAIHandlerWithIntelligence(t)

	handler.legacyAIService.SetCorrelationDetector(seedCorrelationDetector(time.Now()))

	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/correlations?resource_id=vm-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Correlations []struct {
			TargetID     string `json:"target_id"`
			EventPattern string `json:"event_pattern"`
		} `json:"correlations"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Correlations) != 1 {
		t.Fatalf("correlations count = %d, want 1", resp.Count)
	}
	if resp.Correlations[0].TargetID != "vm-1" {
		t.Fatalf("target_id = %s, want vm-1", resp.Correlations[0].TargetID)
	}
	if resp.Correlations[0].EventPattern == "" {
		t.Fatalf("expected event_pattern to be set")
	}
}
