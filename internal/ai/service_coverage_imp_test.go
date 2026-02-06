package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestService_QuickAnalysis(t *testing.T) {
	svc := NewService(nil, nil)

	// Case 1: No provider configured
	_, err := svc.QuickAnalysis(context.Background(), "test")
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("Expected error about provider not enabled, got: %v", err)
	}

	// Case 2: Configured
	mockProv := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			if req.Model != "fast-model" {
				return nil, nil // Should use patrol model
			}
			return &providers.ChatResponse{
				Content: "Analysis Result",
			}, nil
		},
	}
	svc.provider = mockProv
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "fast-model",
	}

	res, err := svc.QuickAnalysis(context.Background(), "Analysis prompt")
	if err != nil {
		t.Fatalf("QuickAnalysis failed: %v", err)
	}
	if res != "Analysis Result" {
		t.Errorf("Unexpected result: %s", res)
	}

	// Case 3: Empty response
	mockProv.chatFunc = func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
		return &providers.ChatResponse{Content: ""}, nil
	}
	_, err = svc.QuickAnalysis(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for empty response")
	}
}

func TestService_AnalyzeForDiscovery(t *testing.T) {
	svc := NewService(nil, nil)

	// Case 1: No provider
	_, err := svc.AnalyzeForDiscovery(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when provider not configured")
	}

	// Case 2: Not enabled
	svc.provider = &mockProvider{}
	svc.cfg = &config.AIConfig{Enabled: false}
	_, err = svc.AnalyzeForDiscovery(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when AI disabled")
	}

	// Case 3: Success with cost tracking
	mockProv := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{
				Content:      "Discovery Result",
				InputTokens:  10,
				OutputTokens: 20,
			}, nil
		},
		nameFunc: func() string { return "mock-provider" },
	}
	svc.provider = mockProv
	svc.cfg = &config.AIConfig{Enabled: true}

	svc.costStore = cost.NewStore(30) // In-memory store

	res, err := svc.AnalyzeForDiscovery(context.Background(), "test discovery")
	if err != nil {
		t.Fatalf("AnalyzeForDiscovery failed: %v", err)
	}
	if res != "Discovery Result" {
		t.Errorf("Unexpected result: %s", res)
	}

	// Check cost tracking
	summary := svc.costStore.GetSummary(1)
	var pm cost.ProviderModelSummary
	found := false
	for _, p := range summary.ProviderModels {
		if p.Provider == "mock-provider" {
			pm = p
			found = true
			break
		}
	}

	if !found {
		t.Error("Cost tracking failed: provider not found")
	} else if pm.InputTokens != 10 || pm.OutputTokens != 20 {
		t.Errorf("Cost tracking failed, got %d/%d tokens", pm.InputTokens, pm.OutputTokens)
	}
}

func TestService_RecordIncidentRunbook(t *testing.T) {
	svc := NewService(nil, nil)

	// Case 1: No store -> should safe return
	svc.RecordIncidentRunbook("alert1", "rb1", "title", memory.OutcomeResolved, true, "msg")

	// Case 2: Invalid inputs -> should safe return
	svc.incidentStore = memory.NewIncidentStore(memory.IncidentStoreConfig{})
	svc.RecordIncidentRunbook("", "rb1", "title", memory.OutcomeResolved, true, "msg")
	svc.RecordIncidentRunbook("alert1", "", "title", memory.OutcomeResolved, true, "msg")

	// Case 3: Valid
	svc.RecordIncidentRunbook("alert1", "rb1", "title", memory.OutcomeResolved, true, "msg")

	timeline := svc.incidentStore.GetTimelineByAlertID("alert1")
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
