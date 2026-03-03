package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func setupAIHandlerWithPatrolAtPath(t *testing.T, dataPath string) (*AISettingsHandler, *ai.PatrolService, *unified.UnifiedStore, *learning.LearningStore, *config.ConfigPersistence) {
	t.Helper()

	cfg := &config.Config{DataPath: dataPath}
	persistence := config.NewConfigPersistence(dataPath)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.legacyAIService.SetStateProvider(&stubStateProvider{})
	if err := handler.SetPatrolFindingsPersistence(ai.NewFindingsPersistenceAdapter(persistence)); err != nil {
		t.Fatalf("set patrol findings persistence: %v", err)
	}

	patrol := handler.legacyAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	unifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	handler.SetUnifiedStore(unifiedStore)

	learningStore := learning.NewLearningStore(learning.LearningStoreConfig{})
	handler.SetLearningStore(learningStore)

	return handler, patrol, unifiedStore, learningStore, persistence
}

func setupAIHandlerWithPatrol(t *testing.T) (*AISettingsHandler, *ai.PatrolService, *unified.UnifiedStore, *learning.LearningStore) {
	t.Helper()

	handler, patrol, unifiedStore, learningStore, _ := setupAIHandlerWithPatrolAtPath(t, t.TempDir())
	return handler, patrol, unifiedStore, learningStore
}

func addPatrolFinding(t *testing.T, patrol *ai.PatrolService, id string, detectedAt time.Time) *ai.Finding {
	t.Helper()

	finding := &ai.Finding{
		ID:           id,
		Key:          "key-" + id,
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		Title:        "CPU spike",
		Description:  "CPU high",
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
		DetectedAt:   detectedAt,
		LastSeenAt:   detectedAt,
	}
	patrol.GetFindings().Add(finding)
	return finding
}

func addUnifiedFinding(store *unified.UnifiedStore, id string, detectedAt time.Time) *unified.UnifiedFinding {
	finding := &unified.UnifiedFinding{
		ID:           id,
		Source:       unified.SourceAIPatrol,
		Severity:     unified.SeverityWarning,
		Category:     unified.CategoryPerformance,
		ResourceID:   "vm-1",
		ResourceName: "vm-1",
		ResourceType: "vm",
		Title:        "CPU spike",
		Description:  "CPU high",
		DetectedAt:   detectedAt,
	}
	store.AddFromAI(finding)
	return finding
}

func TestHandleAcknowledgeFinding_PatrolAndUnified(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-ack", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-ack", detectedAt)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/acknowledge", strings.NewReader(`{"finding_id":"finding-ack"}`))
	rec := httptest.NewRecorder()

	handler.HandleAcknowledgeFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-ack")
	if patrolFinding == nil || patrolFinding.AcknowledgedAt == nil {
		t.Fatalf("expected patrol finding to be acknowledged")
	}

	unifiedFinding := unifiedStore.Get("finding-ack")
	if unifiedFinding == nil || unifiedFinding.AcknowledgedAt == nil {
		t.Fatalf("expected unified finding to be acknowledged")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleAcknowledgeFinding_UnifiedOnly(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-1 * time.Hour)
	addUnifiedFinding(unifiedStore, "finding-unified-only", detectedAt)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/acknowledge", strings.NewReader(`{"finding_id":"finding-unified-only"}`))
	rec := httptest.NewRecorder()

	handler.HandleAcknowledgeFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if patrol.GetFindings().Get("finding-unified-only") != nil {
		t.Fatalf("expected patrol finding to be absent")
	}

	unifiedFinding := unifiedStore.Get("finding-unified-only")
	if unifiedFinding == nil || unifiedFinding.AcknowledgedAt == nil {
		t.Fatalf("expected unified finding to be acknowledged")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleSnoozeFinding_CapsDuration(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-30 * time.Minute)
	addPatrolFinding(t, patrol, "finding-snooze", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-snooze", detectedAt)

	body := `{"finding_id":"finding-snooze","duration_hours":200}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/snooze", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleSnoozeFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	message, _ := resp["message"].(string)
	if !strings.Contains(message, "168") {
		t.Fatalf("expected capped duration in message, got %q", message)
	}

	patrolFinding := patrol.GetFindings().Get("finding-snooze")
	if patrolFinding == nil || patrolFinding.SnoozedUntil == nil {
		t.Fatalf("expected snoozed patrol finding")
	}
	if patrolFinding.SnoozedUntil.Before(time.Now().Add(167 * time.Hour)) {
		t.Fatalf("snooze duration not applied")
	}

	unifiedFinding := unifiedStore.Get("finding-snooze")
	if unifiedFinding == nil || unifiedFinding.SnoozedUntil == nil {
		t.Fatalf("expected snoozed unified finding")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleResolveFinding_SetsResolved(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-resolve", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-resolve", detectedAt)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/resolve", strings.NewReader(`{"finding_id":"finding-resolve"}`))
	rec := httptest.NewRecorder()

	handler.HandleResolveFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-resolve")
	if patrolFinding == nil || patrolFinding.ResolvedAt == nil {
		t.Fatalf("expected patrol finding to be resolved")
	}

	unifiedFinding := unifiedStore.Get("finding-resolve")
	if unifiedFinding == nil || unifiedFinding.ResolvedAt == nil {
		t.Fatalf("expected unified finding to be resolved")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleDismissFinding_ValidReason(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-3 * time.Hour)
	addPatrolFinding(t, patrol, "finding-dismiss", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-dismiss", detectedAt)

	payload := map[string]string{
		"finding_id": "finding-dismiss",
		"reason":     "expected_behavior",
		"note":       "known load test",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/dismiss", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleDismissFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-dismiss")
	if patrolFinding == nil || patrolFinding.DismissedReason != "expected_behavior" {
		t.Fatalf("expected patrol finding to be dismissed")
	}
	if patrolFinding.UserNote != "known load test" {
		t.Fatalf("expected patrol note to be recorded")
	}

	unifiedFinding := unifiedStore.Get("finding-dismiss")
	if unifiedFinding == nil || unifiedFinding.DismissedReason != "expected_behavior" {
		t.Fatalf("expected unified finding to be dismissed")
	}
	if unifiedFinding.UserNote != "known load test" {
		t.Fatalf("expected unified note to be recorded")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleSuppressFinding_SetsSuppressed(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-45 * time.Minute)
	addPatrolFinding(t, patrol, "finding-suppress", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-suppress", detectedAt)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/suppress", strings.NewReader(`{"finding_id":"finding-suppress"}`))
	rec := httptest.NewRecorder()

	handler.HandleSuppressFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-suppress")
	if patrolFinding == nil || !patrolFinding.Suppressed {
		t.Fatalf("expected patrol finding to be suppressed")
	}
	if patrolFinding.DismissedReason != "suppressed" {
		t.Fatalf("expected patrol dismissal reason to be set")
	}

	unifiedFinding := unifiedStore.Get("finding-suppress")
	if unifiedFinding == nil || !unifiedFinding.Suppressed {
		t.Fatalf("expected unified finding to be suppressed")
	}
	if unifiedFinding.DismissedReason != "not_an_issue" {
		t.Fatalf("expected unified dismissal reason to be not_an_issue")
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 1 {
		t.Fatalf("feedback records = %d, want 1", stats.TotalFeedbackRecords)
	}
}

func TestHandleUndismissFinding_RestoresDismissedState(t *testing.T) {
	handler, patrol, unifiedStore, _ := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-undismiss", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-undismiss", detectedAt)

	if !patrol.GetFindings().Dismiss("finding-undismiss", "not_an_issue", "false positive") {
		t.Fatalf("expected patrol dismiss to succeed")
	}
	if !unifiedStore.Dismiss("finding-undismiss", "not_an_issue", "false positive") {
		t.Fatalf("expected unified dismiss to succeed")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/undismiss", strings.NewReader(`{"finding_id":"finding-undismiss"}`))
	rec := httptest.NewRecorder()

	handler.HandleUndismissFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-undismiss")
	if patrolFinding == nil {
		t.Fatalf("expected patrol finding to exist")
	}
	if patrolFinding.DismissedReason != "" || patrolFinding.Suppressed {
		t.Fatalf("expected patrol finding to be restored, got dismissed_reason=%q suppressed=%v", patrolFinding.DismissedReason, patrolFinding.Suppressed)
	}

	unifiedFinding := unifiedStore.Get("finding-undismiss")
	if unifiedFinding == nil {
		t.Fatalf("expected unified finding to exist")
	}
	if unifiedFinding.DismissedReason != "" || unifiedFinding.Suppressed {
		t.Fatalf("expected unified finding to be restored, got dismissed_reason=%q suppressed=%v", unifiedFinding.DismissedReason, unifiedFinding.Suppressed)
	}
}

func TestHandleUndismissFinding_NotFound(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/undismiss", strings.NewReader(`{"finding_id":"missing"}`))
	rec := httptest.NewRecorder()

	handler.HandleUndismissFinding(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleUndismissFinding_PersistsAcrossRestart(t *testing.T) {
	dataPath := t.TempDir()
	handler, patrol, unifiedStore, _, persistence := setupAIHandlerWithPatrolAtPath(t, dataPath)

	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-restart", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-restart", detectedAt)

	dismissBody := `{"finding_id":"finding-restart","reason":"not_an_issue","note":"false positive"}`
	dismissReq := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/dismiss", strings.NewReader(dismissBody))
	dismissRec := httptest.NewRecorder()
	handler.HandleDismissFinding(dismissRec, dismissReq)
	if dismissRec.Code != http.StatusOK {
		t.Fatalf("dismiss status = %d, want 200", dismissRec.Code)
	}

	undismissReq := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/undismiss", strings.NewReader(`{"finding_id":"finding-restart"}`))
	undismissRec := httptest.NewRecorder()
	handler.HandleUndismissFinding(undismissRec, undismissReq)
	if undismissRec.Code != http.StatusOK {
		t.Fatalf("undismiss status = %d, want 200", undismissRec.Code)
	}

	if err := patrol.GetFindings().ForceSave(); err != nil {
		t.Fatalf("force save: %v", err)
	}

	onDisk, err := persistence.LoadAIFindings()
	if err != nil {
		t.Fatalf("load on-disk findings: %v", err)
	}
	record := onDisk.Findings["finding-restart"]
	if record == nil {
		t.Fatalf("expected finding-restart to be on disk")
	}
	if record.DismissedReason != "" || record.Suppressed {
		t.Fatalf("expected on-disk finding restored, got dismissed_reason=%q suppressed=%v", record.DismissedReason, record.Suppressed)
	}

	handlerAfterRestart, patrolAfterRestart, _, _, _ := setupAIHandlerWithPatrolAtPath(t, dataPath)
	_ = handlerAfterRestart

	reloaded := patrolAfterRestart.GetFindings().Get("finding-restart")
	if reloaded == nil {
		t.Fatalf("expected finding to reload after restart")
	}
	if reloaded.DismissedReason != "" || reloaded.Suppressed {
		t.Fatalf("expected reloaded finding restored, got dismissed_reason=%q suppressed=%v", reloaded.DismissedReason, reloaded.Suppressed)
	}
}

func TestHandleGetFindingsHistory_StartTimeFilter(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	oldTime := time.Now().Add(-3 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)
	addPatrolFinding(t, patrol, "finding-old", oldTime)
	addPatrolFinding(t, patrol, "finding-recent", recentTime)

	startTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/history?start_time="+startTime, nil)
	rec := httptest.NewRecorder()

	handler.HandleGetFindingsHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var findings []ai.Finding
	if err := json.Unmarshal(rec.Body.Bytes(), &findings); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if findings[0].ID != "finding-recent" {
		t.Fatalf("expected recent finding, got %s", findings[0].ID)
	}
}

func TestHandleForcePatrol_ConfigDisabled(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	cfg := ai.DefaultPatrolConfig()
	cfg.Enabled = false
	patrol.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/run", nil)
	rec := httptest.NewRecorder()

	handler.HandleForcePatrol(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Triggered patrol run") {
		t.Fatalf("expected success message")
	}
}
