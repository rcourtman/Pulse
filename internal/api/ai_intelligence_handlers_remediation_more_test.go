package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
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

	handler := &AISettingsHandler{legacyAIService: svc}
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

	handler := &AISettingsHandler{legacyAIService: svc}
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

func TestHandleGetRemediationPlans_StatusMapping(t *testing.T) {
	engine := remediation.NewEngine(remediation.EngineConfig{})
	plan := &remediation.RemediationPlan{
		ID:          "plan-1",
		Title:       "Critical fix",
		Description: "fix it",
		RiskLevel:   remediation.RiskCritical,
		Steps: []remediation.RemediationStep{
			{Order: 0, Command: "echo ok"},
		},
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if _, err := engine.ApprovePlan(plan.ID, "tester"); err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}

	handler := &AISettingsHandler{}
	handler.SetRemediationEngine(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans?limit=5", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetRemediationPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	plans := payload["plans"].([]interface{})
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	planView := plans[0].(map[string]interface{})
	if planView["risk_level"] != string(remediation.RiskHigh) {
		t.Fatalf("expected risk_level high, got %#v", planView["risk_level"])
	}
	if planView["status"] != "approved" {
		t.Fatalf("expected status approved, got %#v", planView["status"])
	}
}

func TestHandleGetRemediationPlan_Success(t *testing.T) {
	engine := remediation.NewEngine(remediation.EngineConfig{})
	plan := &remediation.RemediationPlan{
		ID:    "plan-2",
		Title: "Fix",
		Steps: []remediation.RemediationStep{{Order: 0, Command: "echo ok"}},
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	handler := &AISettingsHandler{}
	handler.SetRemediationEngine(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans/plan-2?plan_id=plan-2", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetRemediationPlan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var got remediation.RemediationPlan
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "plan-2" {
		t.Fatalf("unexpected plan id: %s", got.ID)
	}
}

func TestHandleApproveRemediationPlan_Success(t *testing.T) {
	engine := remediation.NewEngine(remediation.EngineConfig{})
	plan := &remediation.RemediationPlan{
		ID:    "plan-3",
		Title: "Approve",
		Steps: []remediation.RemediationStep{{Order: 0, Command: "echo ok"}},
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	handler := &AISettingsHandler{}
	handler.SetRemediationEngine(engine)

	body := []byte(`{"plan_id":"plan-3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/remediation/plans/plan-3/approve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleApproveRemediationPlan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
