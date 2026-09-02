package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestHandleGetPatrolDigest_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/digest", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolDigest(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetPatrolDigest_RejectsInvalidWindow(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	for _, query := range []string{"?days=0", "?days=31", "?days=week"} {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/digest"+query, nil)
		rec := httptest.NewRecorder()
		handler.HandleGetPatrolDigest(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected %d, got %d: %s", query, http.StatusBadRequest, rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if payload["code"] != "invalid_patrol_digest_days" {
			t.Fatalf("%s: error payload = %v", query, payload)
		}
	}
}

func TestHandleGetPatrolDigest_NoPatrolServiceReturnsZeroShape(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/digest", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolDigest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var digest ai.PatrolDigest
	if err := json.Unmarshal(rec.Body.Bytes(), &digest); err != nil {
		t.Fatal(err)
	}
	if digest.Window.Days != ai.PatrolDigestDefaultDays || !digest.Window.HistoryComplete {
		t.Fatalf("window = %+v", digest.Window)
	}
	if digest.Mode != config.PatrolAutonomyMonitor {
		t.Fatalf("mode = %q, want monitor when no paid autonomy is configured", digest.Mode)
	}
	if digest.Runs.Total != 0 || digest.Findings.New != 0 || digest.Actions.Pending != 0 {
		t.Fatalf("expected zero digest, got %s", rec.Body.String())
	}
	var wire map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	investigations, ok := wire["investigations"].(map[string]any)
	if !ok || investigations["by_outcome"] == nil {
		t.Fatalf("by_outcome must serialise as an object: %v", wire["investigations"])
	}
}

func TestHandleGetPatrolDigest_SummarisesRunsAndPatrolActions(t *testing.T) {
	t.Parallel()
	handler := createTestAIHandler(t)
	now := time.Now().UTC()

	patrol := &ai.PatrolService{}
	runs := ai.NewPatrolRunHistoryStore(10)
	runs.Add(ai.PatrolRunRecord{
		ID: "run-1", StartedAt: now.Add(-2 * time.Hour), CompletedAt: now.Add(-2*time.Hour + time.Minute),
		Type: "patrol", TriggerReason: "scheduled", ResourcesChecked: 12, NewFindings: 2, ResolvedFindings: 1,
		FindingIDs: []string{}, Status: "issues_found",
	})
	runs.Add(ai.PatrolRunRecord{
		ID: "run-2", StartedAt: now.Add(-9 * 24 * time.Hour), CompletedAt: now.Add(-9*24*time.Hour + time.Minute),
		Type: "patrol", TriggerReason: "scheduled", ResourcesChecked: 12, NewFindings: 7, FindingIDs: []string{}, Status: "issues_found",
	})
	setUnexportedField(t, patrol, "runHistoryStore", runs)
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)

	resources := NewResourceHandlers(&config.Config{DataPath: t.TempDir()})
	t.Cleanup(func() { _ = resources.CloseStores() })
	store, err := resources.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []unifiedresources.ActionAuditRecord{
		attentionReceiptTestRecord("patrol-verified", patrolActionOriginSurface, now.Add(-time.Hour), true),
		attentionReceiptTestRecord("patrol-unverified", patrolActionOriginSurface, now.Add(-time.Hour), false),
		attentionReceiptTestRecord("assistant", "assistant", now.Add(-time.Hour), true),
	} {
		if err := store.RecordActionAudit(record); err != nil {
			t.Fatalf("RecordActionAudit(%s): %v", record.ID, err)
		}
	}
	handler.SetResourceStoreProvider(resources.getStore)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/digest?days=7", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolDigest(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	var digest ai.PatrolDigest
	if err := json.Unmarshal(rec.Body.Bytes(), &digest); err != nil {
		t.Fatal(err)
	}
	if digest.Runs.Total != 1 || digest.Runs.Checks != 12 || digest.Runs.ResourcesCovered != 12 {
		t.Fatalf("runs = %+v, want only the run inside the window", digest.Runs)
	}
	if digest.Findings.New != 2 || digest.Findings.Resolved != 1 || digest.Findings.AutoResolved != 1 {
		t.Fatalf("findings = %+v", digest.Findings)
	}
	if digest.Actions.Proposed != 2 || digest.Actions.Executed != 2 || digest.Actions.Verified != 1 {
		t.Fatalf("actions = %+v, want only Patrol-origin actions", digest.Actions)
	}
	if digest.Runs.LastRunAt == nil {
		t.Fatal("expected last_run_at from the run inside the window")
	}
}
