package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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

func TestRunEvaluationPass(t *testing.T) {
	ps := NewPatrolService(&Service{}, nil)
	_, err := ps.runEvaluationPass(context.Background(), nil, []DetectedSignal{{SignalType: SignalHighCPU}}, "patrol-run-eval")
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
	resp, err := ps.runEvaluationPass(context.Background(), nil, []DetectedSignal{{SignalType: SignalHighCPU}}, "patrol-run-eval")
	if err != nil {
		t.Fatalf("expected evaluation pass to succeed, got %v", err)
	}
	if resp == nil || resp.InputTokens != 10 || resp.OutputTokens != 20 {
		t.Fatalf("unexpected evaluation response: %+v", resp)
	}
}
