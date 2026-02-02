package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIsInvestigationTool(t *testing.T) {
	if !isInvestigationTool("pulse_query") || !isInvestigationTool("pulse_metrics") || !isInvestigationTool("pulse_storage") || !isInvestigationTool("pulse_read") {
		t.Fatal("expected investigation tools to be recognized")
	}
	if isInvestigationTool("pulse_control") {
		t.Fatal("expected non-investigation tool to be false")
	}
}

func TestFormatToolResult(t *testing.T) {
	result := tools.CallToolResult{
		Content: []tools.Content{
			{Type: "text", Text: "first"},
			{Type: "resource", URI: "file://ignored"},
			{Type: "text", Text: "second"},
		},
	}
	if got := formatToolResult(result); got != "first\nsecond" {
		t.Fatalf("formatToolResult returned %q", got)
	}

	if got := formatToolResult(tools.CallToolResult{}); got != "" {
		t.Fatalf("expected empty result for no content, got %q", got)
	}
}

func TestEvalPromptBuilders(t *testing.T) {
	systemPrompt := buildEvalSystemPrompt()
	if !strings.Contains(systemPrompt, "patrol_report_finding") || !strings.Contains(systemPrompt, "patrol_get_findings") {
		t.Fatalf("expected eval system prompt to include tool instructions")
	}

	signals := []DetectedSignal{
		{
			SignalType:        SignalHighCPU,
			ResourceID:        "node-1",
			ResourceName:      "node-1",
			ResourceType:      "node",
			SuggestedSeverity: "warning",
			Category:          "performance",
			Summary:           "CPU high",
			Evidence:          "cpu=99%",
		},
	}
	userPrompt := buildEvalUserPrompt(signals)
	if !strings.Contains(userPrompt, "Signal 1") || !strings.Contains(userPrompt, "CPU high") || !strings.Contains(userPrompt, "cpu=99%") {
		t.Fatalf("unexpected eval user prompt: %s", userPrompt)
	}
}

func TestSignalHelpersAndFindingsFromSignals(t *testing.T) {
	cases := []struct {
		signal       DetectedSignal
		wantKey      string
		wantTitle    string
		recSubstring string
	}{
		{signal: DetectedSignal{SignalType: SignalSMARTFailure}, wantKey: "smart-failure", wantTitle: "SMART health check failed", recSubstring: "disk"},
		{signal: DetectedSignal{SignalType: SignalHighCPU}, wantKey: "cpu-high", wantTitle: "High CPU usage detected", recSubstring: "CPU"},
		{signal: DetectedSignal{SignalType: SignalHighMemory}, wantKey: "memory-high", wantTitle: "High memory usage detected", recSubstring: "memory"},
		{signal: DetectedSignal{SignalType: SignalHighDisk}, wantKey: "disk-high", wantTitle: "Storage usage is high", recSubstring: "storage"},
		{signal: DetectedSignal{SignalType: SignalBackupFailed}, wantKey: "backup-failed", wantTitle: "Backup failed", recSubstring: "backup"},
		{signal: DetectedSignal{SignalType: SignalBackupStale}, wantKey: "backup-stale", wantTitle: "Backup is stale", recSubstring: "backup"},
		{signal: DetectedSignal{SignalType: SignalActiveAlert}, wantKey: "active-alert", wantTitle: "Active alert detected", recSubstring: "alert"},
		{signal: DetectedSignal{SignalType: SignalType("unknown")}, wantKey: "deterministic-signal", wantTitle: "Infrastructure signal detected", recSubstring: "Investigate"},
	}

	for _, c := range cases {
		if got := signalKey(c.signal); got != c.wantKey {
			t.Fatalf("signalKey(%s) = %s, want %s", c.signal.SignalType, got, c.wantKey)
		}
		if got := signalTitle(c.signal); got != c.wantTitle {
			t.Fatalf("signalTitle(%s) = %s, want %s", c.signal.SignalType, got, c.wantTitle)
		}
		if !strings.Contains(defaultRecommendationForSignal(c.signal), c.recSubstring) {
			t.Fatalf("unexpected recommendation for %s: %s", c.signal.SignalType, defaultRecommendationForSignal(c.signal))
		}
	}

	ps := NewPatrolService(nil, nil)
	adapter := newPatrolFindingCreatorAdapter(ps, models.StateSnapshot{})

	signals := []DetectedSignal{
		{
			SignalType:        SignalHighCPU,
			ResourceID:        "node-1",
			ResourceName:      "",
			ResourceType:      "",
			SuggestedSeverity: "",
			Category:          "",
			Summary:           "CPU high",
			Evidence:          "cpu=99%",
		},
		{
			SignalType:        SignalBackupFailed,
			ResourceID:        "vm-101",
			ResourceName:      "vm-101",
			ResourceType:      "vm",
			SuggestedSeverity: "warning",
			Category:          "backup",
			Summary:           "Backup failed",
			Evidence:          "job failed",
		},
	}

	created := ps.createFindingsFromSignals(adapter, signals)
	if created != len(signals) {
		t.Fatalf("expected %d findings created, got %d", len(signals), created)
	}
}

func TestRunEvaluationPass(t *testing.T) {
	ps := NewPatrolService(&Service{}, nil)
	_, err := ps.runEvaluationPass(context.Background(), nil, []DetectedSignal{{SignalType: SignalHighCPU}})
	if err == nil {
		t.Fatal("expected error when chat service is unavailable")
	}

	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	mockCS := &patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			return &PatrolStreamResponse{Content: "ok", InputTokens: 10, OutputTokens: 20}, nil
		},
	}
	svc.SetChatService(mockCS)

	ps.aiService = svc
	resp, err := ps.runEvaluationPass(context.Background(), nil, []DetectedSignal{{SignalType: SignalHighCPU}})
	if err != nil {
		t.Fatalf("expected evaluation pass to succeed, got %v", err)
	}
	if resp == nil || resp.InputTokens != 10 || resp.OutputTokens != 20 {
		t.Fatalf("unexpected evaluation response: %+v", resp)
	}
}
