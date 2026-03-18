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
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func setupAIHandlerWithPatrol(t *testing.T) (*AISettingsHandler, *ai.PatrolService, *unified.UnifiedStore, *learning.LearningStore) {
	t.Helper()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)
	handler.defaultAIService.SetStateProvider(&stubStateProvider{})

	patrol := handler.defaultAIService.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service to be initialized")
	}

	unifiedStore := unified.NewUnifiedStore(unified.DefaultAlertToFindingConfig())
	handler.SetUnifiedStore(unifiedStore)

	learningStore := learning.NewLearningStore(learning.LearningStoreConfig{})
	handler.SetLearningStore(learningStore)

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

type stubQuickstartCreditManager struct {
	remaining int
}

func (m *stubQuickstartCreditManager) HasCredits() bool                { return m.remaining > 0 }
func (m *stubQuickstartCreditManager) CreditsRemaining() int           { return m.remaining }
func (m *stubQuickstartCreditManager) ConsumeCredit() error            { return nil }
func (m *stubQuickstartCreditManager) HasBYOK() bool                   { return false }
func (m *stubQuickstartCreditManager) GetProvider() providers.Provider { return nil }
func (m *stubQuickstartCreditManager) GrantCredits() error             { return nil }

func TestHandleGetPatrolStatus_IncludesQuickstartFields(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)

	handler.defaultAIService.SetQuickstartCredits(&stubQuickstartCreditManager{remaining: 7})
	setUnexportedField(t, handler.defaultAIService, "usingQuickstart", true)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.QuickstartCreditsRemaining != 7 {
		t.Fatalf("quickstart credits remaining = %d, want 7", resp.QuickstartCreditsRemaining)
	}
	if resp.QuickstartCreditsTotal != pkglicensing.QuickstartCreditsTotal {
		t.Fatalf("quickstart credits total = %d, want %d", resp.QuickstartCreditsTotal, pkglicensing.QuickstartCreditsTotal)
	}
	if !resp.UsingQuickstart {
		t.Fatal("expected using_quickstart to be true")
	}
}

func TestPatrolActionHandlers_NoAIService_ReturnStructuredServiceUnavailable(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	handler := newTestAISettingsHandler(cfg, nil, nil)

	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{
			name:    "force_patrol",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/run",
			handler: handler.HandleForcePatrol,
		},
		{
			name:    "acknowledge",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/acknowledge",
			body:    `{"finding_id":"finding-1"}`,
			handler: handler.HandleAcknowledgeFinding,
		},
		{
			name:    "snooze",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/snooze",
			body:    `{"finding_id":"finding-1","duration_hours":1}`,
			handler: handler.HandleSnoozeFinding,
		},
		{
			name:    "resolve",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/resolve",
			body:    `{"finding_id":"finding-1"}`,
			handler: handler.HandleResolveFinding,
		},
		{
			name:    "note",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/findings/note",
			body:    `{"finding_id":"finding-1","note":"keep watching"}`,
			handler: handler.HandleSetFindingNote,
		},
		{
			name:    "dismiss",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/dismiss",
			body:    `{"finding_id":"finding-1","reason":"expected_behavior"}`,
			handler: handler.HandleDismissFinding,
		},
		{
			name:    "suppress",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/suppress",
			body:    `{"finding_id":"finding-1"}`,
			handler: handler.HandleSuppressFinding,
		},
		{
			name:    "clear_all",
			method:  http.MethodDelete,
			path:    "/api/ai/patrol/findings?confirm=true",
			handler: handler.HandleClearAllFindings,
		},
		{
			name:    "add_suppression_rule",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/suppressions",
			body:    `{"description":"known benign workload"}`,
			handler: handler.HandleAddSuppressionRule,
		},
		{
			name:    "delete_suppression_rule",
			method:  http.MethodDelete,
			path:    "/api/ai/patrol/suppressions/rule-1",
			handler: handler.HandleDeleteSuppressionRule,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}

			var resp APIError
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Code != "service_unavailable" {
				t.Fatalf("code = %q, want service_unavailable", resp.Code)
			}
			if resp.ErrorMessage != "Pulse Patrol service not available" {
				t.Fatalf("error = %q, want Pulse Patrol service not available", resp.ErrorMessage)
			}
		})
	}
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
