package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestEvalPromptBuilders(t *testing.T) {
	systemPrompt := buildEvalSystemPrompt(false)
	if !strings.Contains(systemPrompt, "patrol_report_finding") || !strings.Contains(systemPrompt, "patrol_get_findings") {
		t.Fatalf("expected eval system prompt to include tool instructions")
	}
	patrolPrompt := (&PatrolService{}).getPatrolSystemPrompt()
	if !strings.Contains(patrolPrompt, strings.Join(tools.PatrolReportFindingRequiredArguments(), ", ")) || !strings.Contains(patrolPrompt, "reporting several findings in parallel") {
		t.Fatalf("expected Patrol prompt to require independently complete report calls")
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
	userPrompt := buildEvalUserPrompt(signals, nil)
	if !strings.Contains(userPrompt, "Signal 1") || !strings.Contains(userPrompt, "CPU high") || !strings.Contains(userPrompt, "cpu=99%") {
		t.Fatalf("unexpected eval user prompt: %s", userPrompt)
	}
}

func TestTriageFlagsToDetectedSignalsPreservesModelOwnedCandidates(t *testing.T) {
	flags := []TriageFlag{
		{ResourceID: "app-1", ResourceName: "api", ResourceType: "app-container", Category: "health", Severity: "warning", Reason: "health check is unhealthy"},
		{ResourceID: "node-1", ResourceName: "node", ResourceType: "node", Category: "connectivity", Severity: "critical", Reason: "node is unreachable"},
		{ResourceID: "vm-1", ResourceName: "db", ResourceType: "vm", Category: "anomaly", Metric: "memory", Severity: "watch", Reason: "memory differs from baseline"},
		{ResourceID: "storage-1", ResourceName: "pool", ResourceType: "storage", Category: "anomaly", Metric: "usage", Severity: "warning", Reason: "usage growth differs from baseline"},
		{ResourceID: "misc-1", ResourceName: "misc", ResourceType: "agent", Category: "custom", Severity: "watch", Reason: "custom triage evidence"},
		{Category: "health", Severity: "warning", Reason: "identity-free evidence must be ignored"},
	}

	signals := triageFlagsToDetectedSignals(flags)
	if len(signals) != 5 {
		t.Fatalf("signal count = %d, want 5: %+v", len(signals), signals)
	}
	wantCategories := []string{"reliability", "reliability", "performance", "capacity", "general"}
	for i, wantCategory := range wantCategories {
		if signals[i].Category != wantCategory {
			t.Fatalf("signal %d category = %q, want %q", i, signals[i].Category, wantCategory)
		}
		if signals[i].Summary != flags[i].Reason || signals[i].Evidence != flags[i].Reason {
			t.Fatalf("signal %d lost deterministic evidence: %+v", i, signals[i])
		}
		if signals[i].ToolCallID != "deterministic-triage" {
			t.Fatalf("signal %d source = %q, want deterministic-triage", i, signals[i].ToolCallID)
		}
	}
}

func TestTriageFlagsForDecisionFloorPrioritizesLifecycleEvidence(t *testing.T) {
	flags := []TriageFlag{
		{ResourceID: "metric", Category: "performance", Reason: "ordinary threshold crossing"},
		{ResourceID: "anomaly", Category: "anomaly", Reason: "learned anomaly"},
		{ResourceID: "health", Category: "health", Reason: "failed health check"},
		{ResourceID: "backup", Category: "backup", Reason: "backup failed"},
		{ResourceID: "network", Category: "connectivity", Reason: "endpoint unreachable"},
	}

	selected := triageFlagsForDecisionFloor(flags)
	if len(selected) != 4 {
		t.Fatalf("selected = %+v, want four lifecycle/anomaly candidates", selected)
	}
	wantIDs := []string{"health", "backup", "network", "anomaly"}
	for i, wantID := range wantIDs {
		if selected[i].ResourceID != wantID {
			t.Fatalf("selected[%d] = %q, want %q", i, selected[i].ResourceID, wantID)
		}
	}

	many := make([]TriageFlag, patrolTriageDecisionFloorMaxCandidates+5)
	for i := range many {
		many[i] = TriageFlag{ResourceID: fmt.Sprintf("health-%d", i), Category: "health"}
	}
	if got := len(triageFlagsForDecisionFloor(many)); got != patrolTriageDecisionFloorMaxCandidates {
		t.Fatalf("bounded candidate count = %d, want %d", got, patrolTriageDecisionFloorMaxCandidates)
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
	var captured PatrolExecuteRequest
	mockCS := &patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			captured = req
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
	wantTools := []string{agentcapabilities.PatrolGetFindingsToolName, agentcapabilities.PatrolReportFindingToolName}
	if !reflect.DeepEqual(captured.AllowedToolNames, wantTools) {
		t.Fatalf("evaluation tools = %v, want %v", captured.AllowedToolNames, wantTools)
	}
	if captured.MaxFindingReports != 1 {
		t.Fatalf("evaluation finding report budget = %d, want one unmatched signal", captured.MaxFindingReports)
	}
}

func TestRunEvaluationPassReusesEstablishedFindingSnapshot(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	var captured PatrolExecuteRequest
	svc.SetChatService(&patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			captured = req
			return &PatrolStreamResponse{}, nil
		},
	})
	adapter := &patrolFindingCreatorAdapter{
		checkedFindings:         true,
		completeFindingSnapshot: true,
		queriedFindings:         []tools.PatrolFindingInfo{{ID: "finding-1", Title: "Existing", ResourceID: "node-1"}},
	}
	ps := NewPatrolService(svc, nil)
	if _, err := ps.runEvaluationPass(context.Background(), adapter, []DetectedSignal{{SignalType: SignalHighCPU}}, "patrol-run-eval"); err != nil {
		t.Fatalf("run evaluation: %v", err)
	}
	if !reflect.DeepEqual(captured.AllowedToolNames, []string{agentcapabilities.PatrolReportFindingToolName}) {
		t.Fatalf("evaluation tools = %v, want report only", captured.AllowedToolNames)
	}
	if captured.MaxFindingReports != 1 {
		t.Fatalf("evaluation finding report budget = %d, want one unmatched signal", captured.MaxFindingReports)
	}
	if !strings.Contains(captured.Prompt, "finding-1") || !strings.Contains(captured.SystemPrompt, "already included") {
		t.Fatalf("evaluation did not reuse established snapshot: system=%q prompt=%q", captured.SystemPrompt, captured.Prompt)
	}
}

func TestRunEvaluationPassRecordsPartialUsageOnStreamError(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:patrol"}
	svc.provider = &mockProvider{nameFunc: func() string { return "mock" }}
	svc.SetChatService(&patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			return &PatrolStreamResponse{InputTokens: 11, OutputTokens: 7}, errors.New("stream interrupted")
		},
	})

	ps := NewPatrolService(svc, nil)
	resp, err := ps.runEvaluationPass(context.Background(), nil, []DetectedSignal{{SignalType: SignalHighCPU}}, "patrol-run-eval")
	if err == nil {
		t.Fatal("expected evaluation error")
	}
	if resp == nil || resp.InputTokens != 11 || resp.OutputTokens != 7 {
		t.Fatalf("expected partial response on evaluation error, got %+v", resp)
	}
	events := svc.ListCostEvents(1)
	if len(events) != 1 {
		t.Fatalf("expected one partial usage event, got %d", len(events))
	}
	if events[0].Provider != "mock" || events[0].RequestModel != "mock:patrol" || events[0].UseCase != "patrol" || events[0].InputTokens != 11 || events[0].OutputTokens != 7 {
		t.Fatalf("unexpected partial usage event: %+v", events[0])
	}
}

func TestPatrolFollowupTraceCollectorCapturesRawInputAndFailures(t *testing.T) {
	collector := newPatrolFollowupTraceCollector("evaluation")
	start, _ := json.Marshal(map[string]any{
		"id": "call-1", "name": agentcapabilities.PatrolReportFindingToolName,
		"input": "normalized", "raw_input": `{"resource_id":"node-1"}`,
	})
	end, _ := json.Marshal(map[string]any{
		"id": "call-1", "name": agentcapabilities.PatrolReportFindingToolName,
		"output": "rejected", "success": false,
	})
	collector.callback(ChatStreamEvent{Type: "tool_start", Data: start})
	collector.callback(ChatStreamEvent{Type: "tool_end", Data: end})
	records := collector.records()
	if len(records) != 1 || records[0].ID != "evaluation/call-1" || records[0].Input != `{"resource_id":"node-1"}` || records[0].Output != "rejected" || records[0].Success {
		t.Fatalf("unexpected follow-up trace: %+v", records)
	}
}
