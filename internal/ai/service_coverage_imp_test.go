package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestService_QuickAnalysis(t *testing.T) {
	svc := NewService(nil, nil)

	// Case 1: No provider configured
	_, err := svc.QuickAnalysis(context.Background(), QuickAnalysisRequest{Prompt: "test"})
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("Expected error about provider not enabled, got: %v", err)
	}

	// Case 2: Configured
	mockProv := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			if req.Model != "fast-model" {
				return nil, nil // Should use patrol model
			}
			if req.ExecutionID != "patrol-run-123" {
				t.Fatalf("execution_id=%q want patrol-run-123", req.ExecutionID)
			}
			return &providers.ChatResponse{
				Content:      "Analysis Result",
				Model:        "fast-model-response",
				InputTokens:  12,
				OutputTokens: 5,
			}, nil
		},
		nameFunc: func() string { return "mock-provider" },
	}
	svc.provider = mockProv
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "fast-model",
	}

	res, err := svc.QuickAnalysis(context.Background(), QuickAnalysisRequest{
		Prompt:      "Analysis prompt",
		ExecutionID: "patrol-run-123",
		UseCase:     "patrol",
	})
	if err != nil {
		t.Fatalf("QuickAnalysis failed: %v", err)
	}
	if res != "Analysis Result" {
		t.Errorf("Unexpected result: %s", res)
	}
	events := svc.ListCostEvents(1)
	if len(events) != 1 {
		t.Fatalf("expected QuickAnalysis usage event, got %d", len(events))
	}
	if events[0].Provider == "" || events[0].UseCase != "patrol" || events[0].InputTokens != 12 || events[0].OutputTokens != 5 {
		t.Fatalf("unexpected QuickAnalysis usage event: %+v", events[0])
	}

	// Case 3: Empty response
	mockProv.chatFunc = func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
		return &providers.ChatResponse{Content: ""}, nil
	}
	_, err = svc.QuickAnalysis(context.Background(), QuickAnalysisRequest{Prompt: "test"})
	if err == nil {
		t.Error("Expected error for empty response")
	}
}

func TestService_QuickAnalysisSanitizesExternalModelRequest(t *testing.T) {
	svc := NewService(nil, nil)
	resource := unifiedresources.Resource{
		ID:   "agent/pve-secret",
		Type: unifiedresources.ResourceTypeAgent,
		Name: "pve-secret",
		Tags: []string{"restricted"},
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:   []string{"pve-secret.lan"},
			IPAddresses: []string{"10.0.0.5"},
		},
	}
	svc.SetUnifiedResourceProvider(&mockUnifiedResourceProvider{
		getAllFunc: func() []unifiedresources.Resource {
			return []unifiedresources.Resource{resource}
		},
	})

	var seen providers.ChatRequest
	svc.provider = &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			seen = req
			return &providers.ChatResponse{Content: "ok"}, nil
		},
	}
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "openai:gpt-4o",
	}

	if _, err := svc.QuickAnalysis(context.Background(), QuickAnalysisRequest{
		Prompt: "check pve-secret at pve-secret.lan and 10.0.0.5",
	}); err != nil {
		t.Fatalf("QuickAnalysis failed: %v", err)
	}
	combined := seen.Messages[0].Content + "\n" + seen.Messages[1].Content
	for _, raw := range []string{"pve-secret", "pve-secret.lan", "10.0.0.5"} {
		if strings.Contains(combined, raw) {
			t.Fatalf("provider request still contains %q: %s", raw, combined)
		}
	}
}

func TestService_RecordIncidentRunbook(t *testing.T) {
	svc := NewService(nil, nil)
	canonicalStore := unifiedresources.NewMemoryStore()
	svc.resourceExportStore = canonicalStore
	svc.resourceExportStoreOrgID = svc.orgID

	// Case 1: No store -> should safe return
	svc.RecordIncidentRunbook("vm-1", "alert1", "rb1", "title", memory.OutcomeResolved, true, "msg")

	// Case 2: Invalid inputs -> should safe return
	svc.incidentStore = memory.NewIncidentStore(memory.IncidentStoreConfig{})
	svc.RecordIncidentRunbook("vm-1", "", "rb1", "title", memory.OutcomeResolved, true, "msg")
	svc.RecordIncidentRunbook("vm-1", "alert1", "", "title", memory.OutcomeResolved, true, "msg")

	// Case 3: Valid
	svc.RecordIncidentRunbook("vm-1", "alert1", "rb1", "title", memory.OutcomeResolved, true, "msg")

	timeline := svc.incidentStore.GetTimelineByAlertIdentifier("alert1")
	if timeline == nil {
		t.Fatal("expected incident timeline to be created")
	}
	found := false
	for _, ev := range timeline.Events {
		if ev.Type != memory.IncidentEventRunbook {
			continue
		}
		if ev.Details == nil {
			continue
		}
		if ev.Details["runbook_id"] == "rb1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected runbook event to be recorded, got %d events", len(timeline.Events))
	}

	changes, err := canonicalStore.GetRecentChanges("vm-1", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) == 0 {
		t.Fatal("expected canonical runbook change")
	}
	if changes[0].Kind != unifiedresources.ChangeRunbookExecuted {
		t.Fatalf("Kind = %q, want %q", changes[0].Kind, unifiedresources.ChangeRunbookExecuted)
	}
}

func TestService_SetIncidentStore_AttachesCanonicalTimelineProjection(t *testing.T) {
	svc := NewService(nil, nil)
	canonicalStore := unifiedresources.NewMemoryStore()

	svc.resourceExportStore = canonicalStore
	svc.resourceExportStoreOrgID = svc.orgID

	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	svc.SetIncidentStore(incidentStore)

	alertStartedAt := time.Now().UTC().Add(-15 * time.Minute).Truncate(time.Second)
	alert := &alerts.Alert{
		ID:           "alert-service-projected",
		Type:         "cpu",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "vm-service-projected",
		ResourceName: "vm-service-projected",
		StartTime:    alertStartedAt,
		Value:        94,
		Threshold:    80,
	}
	incidentStore.RecordAlertFired(alert)
	incidentStore.RecordAnalysis(alert.ID, "analysis", nil)

	for _, change := range []*unifiedresources.ResourceChange{
		unifiedresources.BuildAlertTimelineChange(alert.ResourceID, unifiedresources.ChangeAlertFired, alertStartedAt, "", unifiedresources.AlertTimelineChange{
			AlertIdentifier: alert.ID,
			AlertType:       alert.Type,
			AlertLevel:      string(alert.Level),
			AlertValue:      alert.Value,
			AlertThreshold:  alert.Threshold,
		}),
		unifiedresources.BuildRunbookExecutionChange(alert.ResourceID, alert.ID, "agent:pulse-patrol", "rb-service", "Restart service", "resolved", true, "ok", nil),
	} {
		if err := canonicalStore.RecordChange(*change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}

	timeline := incidentStore.GetTimelineByAlertIdentifier(alert.ID)
	if timeline == nil {
		t.Fatal("expected projected timeline")
	}

	foundRunbook := false
	for _, event := range timeline.Events {
		if event.Type == memory.IncidentEventRunbook {
			foundRunbook = true
			break
		}
	}
	if !foundRunbook {
		t.Fatal("expected canonical runbook event in projected timeline")
	}
}

func TestAbsFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.0, 1.0},
		{-1.0, 1.0},
		{0.0, 0.0},
		{-0.0001, 0.0001},
	}

	for _, tt := range tests {
		if got := absFloat(tt.input); got != tt.expected {
			t.Errorf("absFloat(%f) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}

func TestService_GetKnowledgeStore(t *testing.T) {
	svc := NewService(nil, nil)
	if svc.GetKnowledgeStore() != nil {
		t.Error("Expected nil store initially")
	}

	// Set it indirectly? Or just set field via reflection/if exported
	// knowledgeStore is unexported.
	// But it is initialized in NewService? No, it's nil in tests usually.

	// Creating a knowledge store is complex due to dependencies.
	// But we can check the getter safely handles nil.
}
