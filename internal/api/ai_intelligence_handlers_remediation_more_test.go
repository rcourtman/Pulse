package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
)

func TestHandleGetRemediations_WithLog(t *testing.T) {
	svc := newEnabledAIService(t)
	log := ai.NewRemediationLog(ai.RemediationLogConfig{MaxRecords: 10})
	record := ai.RemediationRecord{
		ResourceID:   "vm-1",
		ResourceType: "vm",
		ResourceName: "vm-one",
		FindingID:    "finding-1",
		Problem:      "cpu high",
		Action:       "restart",
		Outcome:      ai.OutcomeResolved,
		Automatic:    true,
		Timestamp:    time.Now(),
		Duration:     2 * time.Second,
	}
	if err := log.Log(record); err != nil {
		t.Fatalf("log remediation: %v", err)
	}

	patrol := svc.GetPatrolService()
	setUnexportedField(t, patrol, "remediationLog", log)

	handler := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/remediations?hours=1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("expected count 1, got %#v", payload["count"])
	}
	stats := payload["stats"].(map[string]interface{})
	if stats["resolved"] != float64(1) || stats["automatic"] != float64(1) {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestHandleGetRemediations_FilterFinding(t *testing.T) {
	svc := newEnabledAIService(t)
	log := ai.NewRemediationLog(ai.RemediationLogConfig{MaxRecords: 10})
	record := ai.RemediationRecord{
		ResourceID:   "vm-2",
		ResourceType: "vm",
		ResourceName: "vm-two",
		FindingID:    "finding-2",
		Problem:      "disk",
		Action:       "cleanup",
		Outcome:      ai.OutcomePartial,
		Automatic:    false,
		Timestamp:    time.Now(),
	}
	if err := log.Log(record); err != nil {
		t.Fatalf("log remediation: %v", err)
	}

	patrol := svc.GetPatrolService()
	setUnexportedField(t, patrol, "remediationLog", log)

	handler := &AISettingsHandler{defaultAIService: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/remediations?finding_id=finding-2", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("expected count 1, got %#v", payload["count"])
	}
}

func TestHandleGetRemediations_LicenseHeader(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/remediations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Header().Get("X-License-Required") != "true" {
		t.Fatalf("expected license header to be set")
	}
	if rec.Header().Get("X-License-Feature") != license.FeatureAIAutoFix {
		t.Fatalf("expected license feature header")
	}
}
