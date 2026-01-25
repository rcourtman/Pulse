package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/learning"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type stubStateProvider struct{}

func (s *stubStateProvider) GetState() models.StateSnapshot {
	return models.StateSnapshot{}
}

type fakeChatWrapper struct {
	*chat.Service
}

func newTestAISettingsHandlerLite() *AISettingsHandler {
	return &AISettingsHandler{
		legacyAIService: ai.NewService(nil, nil),
		aiServices:      make(map[string]*ai.Service),
	}
}

func TestPreviewTitle(t *testing.T) {
	cases := map[ai.FindingCategory]string{
		ai.FindingCategoryPerformance: "Performance issue detected",
		ai.FindingCategoryCapacity:    "Capacity issue detected",
		ai.FindingCategoryReliability: "Reliability issue detected",
		ai.FindingCategoryBackup:      "Backup issue detected",
		ai.FindingCategorySecurity:    "Security issue detected",
		ai.FindingCategory("other"):   "Potential issue detected",
	}

	for category, expected := range cases {
		if got := previewTitle(category); got != expected {
			t.Fatalf("category %s expected %q, got %q", category, expected, got)
		}
	}
}

func TestPreviewResourceName(t *testing.T) {
	cases := map[string]string{
		"node":             "Node",
		"vm":               "VM",
		"container":        "Container",
		"oci_container":    "Container",
		"docker_host":      "Docker host",
		"docker_container": "Docker container",
		"storage":          "Storage",
		"pbs":              "PBS server",
		"pbs_datastore":    "PBS datastore",
		"pbs_job":          "PBS job",
		"host":             "Host",
		"host_raid":        "RAID array",
		"host_sensor":      "Host sensor",
		"unknown":          "Resource",
	}

	for resourceType, expected := range cases {
		if got := previewResourceName(resourceType); got != expected {
			t.Fatalf("resource %s expected %q, got %q", resourceType, expected, got)
		}
	}
}

func TestRedactFindingsForPreview(t *testing.T) {
	now := time.Now()
	finding := &ai.Finding{
		Key:             "key",
		ResourceID:      "res-1",
		ResourceName:    "db-1",
		ResourceType:    "vm",
		Node:            "node-1",
		Category:        ai.FindingCategoryPerformance,
		Title:           "Original title",
		Description:     "Description",
		Recommendation:  "Recommendation",
		Evidence:        "Evidence",
		AlertID:         "alert-1",
		AcknowledgedAt:  &now,
		SnoozedUntil:    &now,
		ResolvedAt:      &now,
		AutoResolved:    true,
		DismissedReason: "reason",
		UserNote:        "note",
		TimesRaised:     3,
		Suppressed:      true,
		Source:          "original",
	}

	redacted := redactFindingsForPreview([]*ai.Finding{nil, finding})
	if len(redacted) != 1 {
		t.Fatalf("expected 1 redacted finding, got %d", len(redacted))
	}
	got := redacted[0]
	if got.Key != "" || got.ResourceID != "" || got.Node != "" {
		t.Fatalf("expected identifiers to be cleared")
	}
	if got.ResourceName != "VM" {
		t.Fatalf("expected resource name to be preview value, got %q", got.ResourceName)
	}
	if got.Title != "Performance issue detected" {
		t.Fatalf("expected preview title, got %q", got.Title)
	}
	if got.Description != "Upgrade to view full analysis." {
		t.Fatalf("expected preview description")
	}
	if got.Recommendation != "" || got.Evidence != "" || got.AlertID != "" {
		t.Fatalf("expected sensitive fields to be cleared")
	}
	if got.AcknowledgedAt != nil || got.SnoozedUntil != nil || got.ResolvedAt != nil {
		t.Fatalf("expected timestamps to be cleared")
	}
	if got.AutoResolved || got.DismissedReason != "" || got.UserNote != "" {
		t.Fatalf("expected status fields to be cleared")
	}
	if got.TimesRaised != 0 || got.Suppressed {
		t.Fatalf("expected counters to be cleared")
	}
	if got.Source != "preview" {
		t.Fatalf("expected source to be preview")
	}
	if finding.Title != "Original title" {
		t.Fatalf("expected original finding to remain unchanged")
	}
}

func TestRedactPatrolRunHistory(t *testing.T) {
	runs := []ai.PatrolRunRecord{
		{
			ID:           "run-1",
			AIAnalysis:   "analysis",
			InputTokens:  100,
			OutputTokens: 200,
			FindingIDs:   []string{"a", "b"},
		},
	}

	redacted := redactPatrolRunHistory(runs)
	if redacted[0].AIAnalysis != "" || redacted[0].InputTokens != 0 || redacted[0].OutputTokens != 0 {
		t.Fatalf("expected AI analysis fields to be cleared")
	}
	if redacted[0].FindingIDs != nil {
		t.Fatalf("expected finding IDs to be cleared")
	}
}

func TestIsMCPToolCall(t *testing.T) {
	handler := &AISettingsHandler{}
	if !handler.isMCPToolCall("pulse_control_guest(guest_id='102')") {
		t.Fatalf("expected MCP tool call to be detected")
	}
	if !handler.isMCPToolCall("default_api:pulse_get_resource(id='1')") {
		t.Fatalf("expected MCP tool call with default_api prefix")
	}
	if handler.isMCPToolCall("echo hello") {
		t.Fatalf("expected non-tool command to be false")
	}
}

func TestCleanTargetHost(t *testing.T) {
	handler := &AISettingsHandler{}
	if got := handler.cleanTargetHost("delly (The container's host is 'delly')"); got != "delly" {
		t.Fatalf("expected cleaned host, got %q", got)
	}
	if got := handler.cleanTargetHost("delly extra"); got != "delly" {
		t.Fatalf("expected first token, got %q", got)
	}
	if got := handler.cleanTargetHost("  delly "); got != "delly" {
		t.Fatalf("expected trimmed host, got %q", got)
	}
	if got := handler.cleanTargetHost(""); got != "" {
		t.Fatalf("expected empty host")
	}
}

func TestSplitToolArgs(t *testing.T) {
	handler := &AISettingsHandler{}
	args := "action='start', guest_id=\"102\", note='hello, world', path=\"/tmp/a,b\", escaped=\"\\\"quote\\\"\""
	parts := handler.splitToolArgs(args)
	expected := []string{
		"action='start'",
		"guest_id=\"102\"",
		"note='hello, world'",
		"path=\"/tmp/a,b\"",
		"escaped=\"\\\"quote\\\"\"",
	}
	if len(parts) != len(expected) {
		t.Fatalf("expected %d parts, got %d", len(expected), len(parts))
	}
	for i := range expected {
		if strings.TrimSpace(parts[i]) != expected[i] {
			t.Fatalf("expected part %q, got %q", expected[i], parts[i])
		}
	}
}

func TestParseMCPToolCall(t *testing.T) {
	handler := &AISettingsHandler{}
	tool, args, err := handler.parseMCPToolCall("default_api:pulse_control_guest(guest_id=\"102\", action='start')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool != "pulse_control_guest" {
		t.Fatalf("expected tool name pulse_control_guest, got %q", tool)
	}
	if args["guest_id"] != "102" || args["action"] != "start" {
		t.Fatalf("unexpected args: %#v", args)
	}

	tool, args, err = handler.parseMCPToolCall("pulse_run_command()")
	if err != nil {
		t.Fatalf("unexpected error for empty args: %v", err)
	}
	if tool != "pulse_run_command" || len(args) != 0 {
		t.Fatalf("expected empty args, got %#v", args)
	}

	if _, _, err = handler.parseMCPToolCall("pulse_control_guest"); err == nil {
		t.Fatalf("expected error for missing parenthesis")
	}
	if _, _, err = handler.parseMCPToolCall("pulse_control_guest("); err == nil {
		t.Fatalf("expected error for missing closing parenthesis")
	}
}

func TestExecuteMCPToolFix_Errors(t *testing.T) {
	handler := &AISettingsHandler{}
	if _, _, err := handler.executeMCPToolFix(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat handler is missing")
	}

	handler.chatHandler = &AIHandler{}
	if _, _, err := handler.executeMCPToolFix(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error when chat service is missing")
	}

	handler.chatHandler.legacyService = &fakeChatWrapper{}
	if _, _, err := handler.executeMCPToolFix(context.Background(), "pulse_control_guest()", ""); err == nil {
		t.Fatalf("expected error for chat service type mismatch")
	}
}

func TestAISettingsHandler_Setters(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	stateProvider := &stubStateProvider{}
	handler.SetStateProvider(stateProvider)
	if handler.GetStateProvider() != stateProvider {
		t.Fatalf("expected state provider to be set")
	}

	breaker := &circuit.Breaker{}
	handler.SetCircuitBreaker(breaker)
	if handler.GetCircuitBreaker() != breaker {
		t.Fatalf("expected circuit breaker to be set")
	}

	learningStore := &learning.LearningStore{}
	handler.SetLearningStore(learningStore)
	if handler.GetLearningStore() != learningStore {
		t.Fatalf("expected learning store to be set")
	}

	forecastSvc := &forecast.Service{}
	handler.SetForecastService(forecastSvc)
	if handler.GetForecastService() != forecastSvc {
		t.Fatalf("expected forecast service to be set")
	}

	correlator := &proxmox.EventCorrelator{}
	handler.SetProxmoxCorrelator(correlator)
	if handler.GetProxmoxCorrelator() != correlator {
		t.Fatalf("expected correlator to be set")
	}

	engine := &remediation.Engine{}
	handler.SetRemediationEngine(engine)
	if handler.GetRemediationEngine() != engine {
		t.Fatalf("expected remediation engine to be set")
	}
}

func TestAISettingsHandler_RemoveTenantService(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	handler.aiServices["org-1"] = ai.NewService(nil, nil)
	handler.aiServices["default"] = ai.NewService(nil, nil)

	handler.RemoveTenantService("org-1")
	if _, ok := handler.aiServices["org-1"]; ok {
		t.Fatalf("expected org-1 to be removed")
	}

	handler.RemoveTenantService("default")
	if _, ok := handler.aiServices["default"]; !ok {
		t.Fatalf("expected default to remain")
	}
}

func TestAISettingsHandler_IsAIEnabled(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	if handler.IsAIEnabled(context.Background()) {
		t.Fatalf("expected AI to be disabled by default")
	}
}
