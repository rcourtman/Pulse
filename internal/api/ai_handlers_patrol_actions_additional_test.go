package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type scopedPatrolStateProvider struct {
	state models.StateSnapshot
}

func (p *scopedPatrolStateProvider) ReadSnapshot() models.StateSnapshot { return p.state }

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
	return addPatrolFindingForResource(t, patrol, id, detectedAt, "vm-1", "CPU spike")
}

func addPatrolFindingForResource(t *testing.T, patrol *ai.PatrolService, id string, detectedAt time.Time, resourceID string, title string) *ai.Finding {
	t.Helper()

	finding := &ai.Finding{
		ID:           id,
		Key:          "key-" + id,
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryPerformance,
		Title:        title,
		Description:  "CPU high",
		ResourceID:   resourceID,
		ResourceName: resourceID,
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

func addPatrolRuntimeFinding(t *testing.T, patrol *ai.PatrolService, id string, detectedAt time.Time) *ai.Finding {
	t.Helper()

	finding := &ai.Finding{
		ID:           id,
		Key:          "ai-patrol-error",
		Severity:     ai.FindingSeverityWarning,
		Category:     ai.FindingCategoryReliability,
		Title:        "Pulse Patrol: Provider billing or quota issue",
		Description:  "Patrol could not complete because provider billing or quota needs attention",
		ResourceID:   "ai-service",
		ResourceName: "Pulse Patrol Service",
		ResourceType: "service",
		DetectedAt:   detectedAt,
		LastSeenAt:   detectedAt,
	}
	patrol.GetFindings().Add(finding)
	return finding
}

func addUnifiedRuntimeFinding(store *unified.UnifiedStore, id string, detectedAt time.Time) *unified.UnifiedFinding {
	finding := &unified.UnifiedFinding{
		ID:           id,
		Source:       unified.SourceAIPatrol,
		Severity:     unified.SeverityWarning,
		Category:     unified.CategoryReliability,
		ResourceID:   "ai-service",
		ResourceName: "Pulse Patrol Service",
		ResourceType: "service",
		Title:        "Pulse Patrol: Provider billing or quota issue",
		Description:  "Patrol could not complete because provider billing or quota needs attention",
		DetectedAt:   detectedAt,
	}
	store.AddFromAI(finding)
	return finding
}

type agentStableErrorEnvelope = agentcapabilities.ErrorEnvelope

func TestHandleGetPatrolStatus_DistinguishesLastFullPatrolFromLastActivity(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	lastPatrolAt := time.Date(2026, 3, 12, 9, 30, 0, 0, time.UTC)
	lastActivityAt := lastPatrolAt.Add(8 * time.Minute)
	setUnexportedField(t, patrol, "lastFullPatrol", lastPatrolAt)
	setUnexportedField(t, patrol, "lastActivity", lastActivityAt)

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.LastPatrolAt == nil || !resp.LastPatrolAt.Equal(lastPatrolAt) {
		t.Fatalf("last_patrol_at = %v, want %v", resp.LastPatrolAt, lastPatrolAt)
	}
	if resp.LastActivityAt == nil || !resp.LastActivityAt.Equal(lastActivityAt) {
		t.Fatalf("last_activity_at = %v, want %v", resp.LastActivityAt, lastActivityAt)
	}
}

func TestHandleGetPatrolStatus_ExposesScopedTriggerStatus(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	patrol.SetTriggerManager(ai.NewTriggerManager(ai.DefaultTriggerManagerConfig()))
	patrol.SetEventTriggerConfig(ai.PatrolEventTriggerConfig{
		AlertTriggersEnabled:   false,
		AnomalyTriggersEnabled: true,
	})

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.TriggerStatus == nil {
		t.Fatal("expected trigger_status to be populated")
	}
	if resp.TriggerStatus.AlertTriggersEnabled {
		t.Fatal("expected alert trigger source to report disabled")
	}
	if !resp.TriggerStatus.AnomalyTriggersEnabled {
		t.Fatal("expected anomaly trigger source to report enabled")
	}
	if resp.TriggerStatus.EventTriggersBlocked {
		t.Fatal("expected trigger status not to report a runtime block")
	}
}

func TestHandleGetPatrolStatus_ExposesEventTriggerRuntimeBlock(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	patrol.SetTriggerManager(ai.NewTriggerManager(ai.DefaultTriggerManagerConfig()))
	patrol.SetEventTriggerConfig(ai.PatrolEventTriggerConfig{
		AlertTriggersEnabled:   true,
		AnomalyTriggersEnabled: true,
	})
	patrol.SetEventTriggerBlock(ai.BackgroundAutomationEventTriggerBlock())

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp PatrolStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.TriggerStatus == nil {
		t.Fatal("expected trigger_status to be populated")
	}
	if !resp.TriggerStatus.EventTriggersBlocked {
		t.Fatal("expected trigger status to expose runtime block")
	}
	if resp.TriggerStatus.EventTriggersBlockedReason != ai.EventTriggerBlockReasonBackgroundAutomationDisabled {
		t.Fatalf("event trigger block reason = %q", resp.TriggerStatus.EventTriggersBlockedReason)
	}
	if resp.TriggerStatus.EventTriggersBlockedMessage == "" {
		t.Fatal("expected user-facing event trigger block message")
	}
	if !resp.TriggerStatus.AlertTriggersEnabled || !resp.TriggerStatus.AnomalyTriggersEnabled {
		t.Fatal("expected runtime block not to rewrite configured trigger preferences")
	}
}

func TestPatrolActionHandlers_NoAIService_ReturnStructuredServiceUnavailable(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	handler := newTestAISettingsHandler(cfg, nil, nil)

	tests := []struct {
		name          string
		method        string
		path          string
		body          string
		handler       func(http.ResponseWriter, *http.Request)
		agentEnvelope bool
	}{
		{
			name:    "force_patrol",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/run",
			handler: handler.HandleForcePatrol,
		},
		{
			name:          "acknowledge",
			method:        http.MethodPost,
			path:          "/api/ai/patrol/acknowledge",
			body:          `{"finding_id":"finding-1"}`,
			handler:       handler.HandleAcknowledgeFinding,
			agentEnvelope: true,
		},
		{
			name:          "snooze",
			method:        http.MethodPost,
			path:          "/api/ai/patrol/snooze",
			body:          `{"finding_id":"finding-1","duration_hours":1}`,
			handler:       handler.HandleSnoozeFinding,
			agentEnvelope: true,
		},
		{
			name:          "resolve",
			method:        http.MethodPost,
			path:          "/api/ai/patrol/resolve",
			body:          `{"finding_id":"finding-1"}`,
			handler:       handler.HandleResolveFinding,
			agentEnvelope: true,
		},
		{
			name:    "note",
			method:  http.MethodPost,
			path:    "/api/ai/patrol/findings/note",
			body:    `{"finding_id":"finding-1","note":"keep watching"}`,
			handler: handler.HandleSetFindingNote,
		},
		{
			name:          "dismiss",
			method:        http.MethodPost,
			path:          "/api/ai/patrol/dismiss",
			body:          `{"finding_id":"finding-1","reason":"expected_behavior"}`,
			handler:       handler.HandleDismissFinding,
			agentEnvelope: true,
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
			req := newLoopbackRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
			}

			if tc.agentEnvelope {
				var resp agentStableErrorEnvelope
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Error != "patrol_unavailable" {
					t.Fatalf("error = %q, want patrol_unavailable", resp.Error)
				}
				if resp.Message != "Pulse Patrol service not available" {
					t.Fatalf("message = %q, want Pulse Patrol service not available", resp.Message)
				}
				return
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

func TestPatrolFindingLifecycleHandlers_ReturnAgentStableValidationErrors(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)

	tests := []struct {
		name        string
		path        string
		body        string
		handler     func(http.ResponseWriter, *http.Request)
		wantMessage string
		wantDetails map[string]string
	}{
		{
			name:        "acknowledge_missing_finding_id",
			path:        "/api/ai/patrol/acknowledge",
			body:        `{}`,
			handler:     handler.HandleAcknowledgeFinding,
			wantMessage: "finding_id is required",
			wantDetails: map[string]string{"finding_id": "required"},
		},
		{
			name:        "snooze_invalid_duration",
			path:        "/api/ai/patrol/snooze",
			body:        `{"finding_id":"finding-1","duration_hours":0}`,
			handler:     handler.HandleSnoozeFinding,
			wantMessage: "duration_hours must be positive",
			wantDetails: map[string]string{"duration_hours": "must be positive"},
		},
		{
			name:        "dismiss_invalid_reason",
			path:        "/api/ai/patrol/dismiss",
			body:        `{"finding_id":"finding-1","reason":"invalid_reason"}`,
			handler:     handler.HandleDismissFinding,
			wantMessage: "Invalid reason. Valid values: not_an_issue, expected_behavior, will_fix_later",
			wantDetails: map[string]string{"reason": "must be one of not_an_issue, expected_behavior, will_fix_later"},
		},
		{
			name:        "resolve_invalid_json",
			path:        "/api/ai/patrol/resolve",
			body:        `{`,
			handler:     handler.HandleResolveFinding,
			wantMessage: "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}

			var resp agentStableErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error != "invalid_finding_request" {
				t.Fatalf("error = %q, want invalid_finding_request", resp.Error)
			}
			if resp.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", resp.Message, tt.wantMessage)
			}
			for key, want := range tt.wantDetails {
				if got := resp.Details[key]; got != want {
					t.Fatalf("details[%q] = %q, want %q; details=%v", key, got, want, resp.Details)
				}
			}
			if len(tt.wantDetails) == 0 && len(resp.Details) != 0 {
				t.Fatalf("details = %v, want empty", resp.Details)
			}
		})
	}
}

func TestPatrolFindingLifecycleHandlers_ReturnAgentStableNotFoundErrors(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)

	tests := []struct {
		name        string
		path        string
		body        string
		handler     func(http.ResponseWriter, *http.Request)
		wantMessage string
	}{
		{
			name:        "acknowledge",
			path:        "/api/ai/patrol/acknowledge",
			body:        `{"finding_id":"missing"}`,
			handler:     handler.HandleAcknowledgeFinding,
			wantMessage: "Finding not found",
		},
		{
			name:        "snooze",
			path:        "/api/ai/patrol/snooze",
			body:        `{"finding_id":"missing","duration_hours":1}`,
			handler:     handler.HandleSnoozeFinding,
			wantMessage: "Finding not found or already resolved",
		},
		{
			name:        "resolve",
			path:        "/api/ai/patrol/resolve",
			body:        `{"finding_id":"missing"}`,
			handler:     handler.HandleResolveFinding,
			wantMessage: "Finding not found or already resolved",
		},
		{
			name:        "dismiss",
			path:        "/api/ai/patrol/dismiss",
			body:        `{"finding_id":"missing","reason":"expected_behavior"}`,
			handler:     handler.HandleDismissFinding,
			wantMessage: "Finding not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
			}

			var resp agentStableErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error != "finding_not_found" {
				t.Fatalf("error = %q, want finding_not_found", resp.Error)
			}
			if resp.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", resp.Message, tt.wantMessage)
			}
		})
	}
}

func TestHandleAddSuppressionRuleRejectsImplicitBroadScope(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	req := newLoopbackRequest(
		http.MethodPost,
		"/api/ai/patrol/suppressions",
		strings.NewReader(`{"resource_id":"","resource_name":"Any","category":"capacity","description":"too broad"}`),
	)
	rec := httptest.NewRecorder()

	handler.HandleAddSuppressionRule(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rules := patrol.GetFindings().GetSuppressionRules(); len(rules) != 0 {
		t.Fatalf("expected no suppression rules, got %d", len(rules))
	}
}

func TestHandleAddSuppressionRuleAllowsExplicitBroadScope(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	req := newLoopbackRequest(
		http.MethodPost,
		"/api/ai/patrol/suppressions",
		strings.NewReader(`{"resource_id":"","resource_name":"Any","category":"capacity","description":"known capacity noise","allow_broad_scope":true}`),
	)
	rec := httptest.NewRecorder()

	handler.HandleAddSuppressionRule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	rules := patrol.GetFindings().GetSuppressionRules()
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(rules))
	}
	if rules[0].ResourceID != "" || rules[0].Category != ai.FindingCategoryCapacity {
		t.Fatalf("unexpected rule scope: resource=%q category=%q", rules[0].ResourceID, rules[0].Category)
	}
}

func TestHandleAcknowledgeFinding_PatrolAndUnified(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-2 * time.Hour)
	addPatrolFinding(t, patrol, "finding-ack", detectedAt)
	addUnifiedFinding(unifiedStore, "finding-ack", detectedAt)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/acknowledge", strings.NewReader(`{"finding_id":"finding-ack"}`))
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

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/acknowledge", strings.NewReader(`{"finding_id":"finding-unified-only"}`))
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
	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/snooze", strings.NewReader(body))
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

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/resolve", strings.NewReader(`{"finding_id":"finding-resolve","resolution_note":"fixed after restarting the worker"}`))
	rec := httptest.NewRecorder()

	handler.HandleResolveFinding(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	patrolFinding := patrol.GetFindings().Get("finding-resolve")
	if patrolFinding == nil || patrolFinding.ResolvedAt == nil {
		t.Fatalf("expected patrol finding to be resolved")
	}
	if patrolFinding.UserNote != "fixed after restarting the worker" {
		t.Fatalf("patrol finding resolution note = %q", patrolFinding.UserNote)
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
	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/dismiss", bytes.NewReader(body))
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

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/suppress", strings.NewReader(`{"finding_id":"finding-suppress"}`))
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

func TestHandlePatrolRuntimeFindingActions_AreRejected(t *testing.T) {
	handler, patrol, unifiedStore, learningStore := setupAIHandlerWithPatrol(t)

	detectedAt := time.Now().Add(-30 * time.Minute)
	addPatrolRuntimeFinding(t, patrol, "finding-runtime", detectedAt)
	addUnifiedRuntimeFinding(unifiedStore, "finding-runtime", detectedAt)

	tests := []struct {
		name          string
		path          string
		body          string
		handler       func(http.ResponseWriter, *http.Request)
		wantPhrase    string
		wantAgentCode string
	}{
		{
			name:          "acknowledge",
			path:          "/api/ai/patrol/acknowledge",
			body:          `{"finding_id":"finding-runtime"}`,
			handler:       handler.HandleAcknowledgeFinding,
			wantPhrase:    "cannot be acknowledged manually",
			wantAgentCode: "finding_action_not_allowed",
		},
		{
			name:          "snooze",
			path:          "/api/ai/patrol/snooze",
			body:          `{"finding_id":"finding-runtime","duration_hours":24}`,
			handler:       handler.HandleSnoozeFinding,
			wantPhrase:    "cannot be snoozed manually",
			wantAgentCode: "finding_action_not_allowed",
		},
		{
			name:          "resolve",
			path:          "/api/ai/patrol/resolve",
			body:          `{"finding_id":"finding-runtime"}`,
			handler:       handler.HandleResolveFinding,
			wantPhrase:    "cannot be resolved manually",
			wantAgentCode: "finding_action_not_allowed",
		},
		{
			name:          "dismiss",
			path:          "/api/ai/patrol/dismiss",
			body:          `{"finding_id":"finding-runtime","reason":"expected_behavior","note":"ignore"}`,
			handler:       handler.HandleDismissFinding,
			wantPhrase:    "cannot be dismissed manually",
			wantAgentCode: "finding_action_not_allowed",
		},
		{
			name:       "suppress",
			path:       "/api/ai/patrol/suppress",
			body:       `{"finding_id":"finding-runtime"}`,
			handler:    handler.HandleSuppressFinding,
			wantPhrase: "cannot be suppressed manually",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newLoopbackRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409", rec.Code)
			}
			if tt.wantAgentCode != "" {
				var resp agentStableErrorEnvelope
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Error != tt.wantAgentCode {
					t.Fatalf("error = %q, want %q", resp.Error, tt.wantAgentCode)
				}
				if !strings.Contains(resp.Message, tt.wantPhrase) {
					t.Fatalf("expected message to contain %q, got %q", tt.wantPhrase, resp.Message)
				}
				return
			}
			if !strings.Contains(rec.Body.String(), tt.wantPhrase) {
				t.Fatalf("expected response to contain %q, got %q", tt.wantPhrase, rec.Body.String())
			}
		})
	}

	patrolFinding := patrol.GetFindings().Get("finding-runtime")
	if patrolFinding == nil {
		t.Fatal("expected runtime finding to remain present")
	}
	if patrolFinding.AcknowledgedAt != nil {
		t.Fatal("expected runtime finding to remain unacknowledged")
	}
	if patrolFinding.SnoozedUntil != nil {
		t.Fatal("expected runtime finding to remain unsnoozed")
	}
	if patrolFinding.ResolvedAt != nil {
		t.Fatal("expected runtime finding to remain unresolved")
	}
	if patrolFinding.DismissedReason != "" {
		t.Fatalf("expected runtime finding to remain undismissed, got %q", patrolFinding.DismissedReason)
	}
	if patrolFinding.Suppressed {
		t.Fatal("expected runtime finding to remain unsuppressed")
	}

	unifiedFinding := unifiedStore.Get("finding-runtime")
	if unifiedFinding == nil {
		t.Fatal("expected unified runtime finding to remain present")
	}
	if unifiedFinding.AcknowledgedAt != nil {
		t.Fatal("expected unified runtime finding to remain unacknowledged")
	}
	if unifiedFinding.SnoozedUntil != nil {
		t.Fatal("expected unified runtime finding to remain unsnoozed")
	}
	if unifiedFinding.ResolvedAt != nil {
		t.Fatal("expected unified runtime finding to remain unresolved")
	}
	if unifiedFinding.DismissedReason != "" {
		t.Fatalf("expected unified runtime finding to remain undismissed, got %q", unifiedFinding.DismissedReason)
	}

	stats := learningStore.GetStatistics()
	if stats.TotalFeedbackRecords != 0 {
		t.Fatalf("feedback records = %d, want 0", stats.TotalFeedbackRecords)
	}
}

func TestHandleGetFindingsHistory_StartTimeFilter(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)

	oldTime := time.Now().Add(-3 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)
	addPatrolFindingForResource(t, patrol, "finding-old", oldTime, "vm-old", "Old CPU spike")
	addPatrolFindingForResource(t, patrol, "finding-recent", recentTime, "vm-recent", "Recent CPU spike")

	startTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/history?start_time="+startTime, nil)
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

func seedReadyAnthropicPatrolRuntime(t *testing.T, handler *AISettingsHandler) {
	t.Helper()

	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = "anthropic:claude-3-5-sonnet-latest"
	aiCfg.AnthropicAPIKey = "test-key"
	if err := handler.defaultPersistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := handler.defaultAIService.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
}

func TestHandleForcePatrol_ConfigDisabled(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	seedReadyAnthropicPatrolRuntime(t, handler)

	cfg := ai.DefaultPatrolConfig()
	cfg.Enabled = false
	patrol.SetConfig(cfg)

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", nil)
	rec := httptest.NewRecorder()

	handler.HandleForcePatrol(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Triggered patrol run") {
		t.Fatalf("expected success message")
	}
}

func TestHandleForcePatrol_BlocksNotReadyPatrolModel(t *testing.T) {
	handler, _, _, _ := setupAIHandlerWithPatrol(t)

	model := "ollama:deepseek-r1:7b-llama-distill-q4_K_M"
	aiCfg := config.NewDefaultAIConfig()
	aiCfg.Enabled = true
	aiCfg.Model = model
	aiCfg.PatrolModel = model
	aiCfg.OllamaBaseURL = "http://127.0.0.1:11434"
	if err := handler.defaultPersistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := handler.defaultAIService.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", nil)
	rec := httptest.NewRecorder()

	handler.HandleForcePatrol(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var payload APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "patrol_readiness_not_ready" {
		t.Fatalf("code = %q, want patrol_readiness_not_ready", payload.Code)
	}
	if !strings.Contains(payload.ErrorMessage, "reasoning-only model family") {
		t.Fatalf("error = %q", payload.ErrorMessage)
	}
	if payload.Details["model"] != model {
		t.Fatalf("model detail = %q, want %q", payload.Details["model"], model)
	}
}

func TestHandleForcePatrol_CommunityTierIgnoresRecentScopedActivityForFullPatrolRateLimit(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	seedReadyAnthropicPatrolRuntime(t, handler)
	handler.defaultAIService.SetLicenseChecker(communityLicenseChecker{})

	setUnexportedField(t, patrol, "lastActivity", time.Now().Add(-10*time.Minute))

	req := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", nil)
	rec := httptest.NewRecorder()

	handler.HandleForcePatrol(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "patrol_rate_limited") {
		t.Fatalf("expected community force patrol to ignore scoped-only activity, got %s", rec.Body.String())
	}
}

func TestBuildManualScopedPatrolScope(t *testing.T) {
	cases := []struct {
		name   string
		req    manualScopedPatrolRequest
		wantOK bool
		check  func(*testing.T, ai.PatrolScope)
	}{
		{
			name:   "empty body falls back to full run",
			req:    manualScopedPatrolRequest{},
			wantOK: false,
		},
		{
			name:   "whitespace-only ids fall back to full run",
			req:    manualScopedPatrolRequest{ResourceIDs: []string{"  "}},
			wantOK: false,
		},
		{
			name:   "resource ids build a manual quick scope",
			req:    manualScopedPatrolRequest{ResourceIDs: []string{"vm-101", " vm-102 "}, AlertIdentifier: "alert-1", AlertType: "cpu"},
			wantOK: true,
			check: func(t *testing.T, s ai.PatrolScope) {
				t.Helper()
				if s.Reason != ai.TriggerReasonManual {
					t.Errorf("reason = %q, want manual", s.Reason)
				}
				if s.Depth != ai.PatrolDepthQuick {
					t.Errorf("depth = %v, want quick", s.Depth)
				}
				if len(s.ResourceIDs) != 2 || s.ResourceIDs[0] != "vm-101" || s.ResourceIDs[1] != "vm-102" {
					t.Errorf("resource ids = %v", s.ResourceIDs)
				}
				if s.AlertIdentifier != "alert-1" {
					t.Errorf("alert identifier = %q", s.AlertIdentifier)
				}
				if !strings.Contains(s.Context, "cpu") {
					t.Errorf("context = %q, want alert type", s.Context)
				}
			},
		},
		{
			name:   "resource types only still scopes",
			req:    manualScopedPatrolRequest{ResourceTypes: []string{"vm"}},
			wantOK: true,
			check: func(t *testing.T, s ai.PatrolScope) {
				t.Helper()
				if len(s.ResourceTypes) != 1 || s.ResourceTypes[0] != "vm" {
					t.Errorf("resource types = %v", s.ResourceTypes)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope, ok := buildManualScopedPatrolScope(tc.req)
			if ok != tc.wantOK {
				t.Fatalf("hasScope = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			tc.check(t, scope)
		})
	}
}

func TestHandleForcePatrol_ScopedRequestBypassesFullRunCadenceGate(t *testing.T) {
	handler, patrol, _, _ := setupAIHandlerWithPatrol(t)
	seedReadyAnthropicPatrolRuntime(t, handler)
	handler.defaultAIService.SetStateProvider(&scopedPatrolStateProvider{state: models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-101", Name: "web", VMID: 101}},
	}})
	handler.defaultAIService.SetLicenseChecker(communityLicenseChecker{})

	// A recent full patrol puts Community tier inside the 1/hour full-run gate,
	// so a fleet-wide request would be rate-limited.
	setUnexportedField(t, patrol, "lastFullPatrol", time.Now().Add(-10*time.Minute))

	// Scoped request: must succeed despite the full-run cadence window.
	scopedReq := newLoopbackRequest(
		http.MethodPost,
		"/api/ai/patrol/run",
		bytes.NewReader([]byte(`{"resource_ids":["vm-101"],"alert_identifier":"alert-1","alert_type":"cpu"}`)),
	)
	scopedRec := httptest.NewRecorder()
	handler.HandleForcePatrol(scopedRec, scopedReq)

	if scopedRec.Code != http.StatusOK {
		t.Fatalf("scoped status = %d, want 200: %s", scopedRec.Code, scopedRec.Body.String())
	}
	if !strings.Contains(scopedRec.Body.String(), "targeted Patrol check") {
		t.Fatalf("scoped response = %s, want targeted Patrol check message", scopedRec.Body.String())
	}

	// Contrast: an empty body (full run) on the same recent-full state is gated.
	fullReq := newLoopbackRequest(http.MethodPost, "/api/ai/patrol/run", nil)
	fullRec := httptest.NewRecorder()
	handler.HandleForcePatrol(fullRec, fullReq)

	if fullRec.Code != http.StatusTooManyRequests {
		t.Fatalf("full status = %d, want 429 rate limited: %s", fullRec.Code, fullRec.Body.String())
	}
	if !strings.Contains(fullRec.Body.String(), "patrol_rate_limited") {
		t.Fatalf("full response = %s, want patrol_rate_limited", fullRec.Body.String())
	}
}
