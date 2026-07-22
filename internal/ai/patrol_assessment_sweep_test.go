package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAssessmentSweepPromptBuilders(t *testing.T) {
	systemPrompt := buildAssessmentSweepSystemPrompt()
	if !strings.Contains(systemPrompt, "patrol_assess_finding") {
		t.Fatalf("expected sweep system prompt to name the assessment tool")
	}
	if !strings.Contains(systemPrompt, "uncertain") || !strings.Contains(systemPrompt, "Never skip a finding") {
		t.Fatalf("expected sweep system prompt to allow uncertain and forbid skipping")
	}
	if !strings.Contains(systemPrompt, "Do NOT investigate further") {
		t.Fatalf("expected sweep system prompt to forbid re-investigation")
	}

	longEvidence := strings.Repeat("x", 600)
	pending := []*Finding{
		{
			ID:           "0e7c5dbb86bdebe9",
			Title:        "Powered Off on docker-host-edge-02",
			Severity:     FindingSeverityWarning,
			ResourceID:   "docker-host-edge-02",
			ResourceName: "docker-host-edge-02",
			ResourceType: "docker",
			Evidence:     longEvidence,
		},
	}
	userPrompt := buildAssessmentSweepUserPrompt(pending)
	if !strings.Contains(userPrompt, "0e7c5dbb86bdebe9") || !strings.Contains(userPrompt, "Powered Off on docker-host-edge-02") {
		t.Fatalf("expected sweep user prompt to carry finding id and title: %s", userPrompt)
	}
	if strings.Contains(userPrompt, longEvidence) {
		t.Fatalf("expected long evidence to be truncated")
	}
	if !strings.Contains(userPrompt, strings.Repeat("x", 500)+"…") {
		t.Fatalf("expected truncated evidence marker in prompt")
	}
}

func TestRunAssessmentSweepFilesRequest(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)

	var captured PatrolExecuteRequest
	svc.SetChatService(&patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			captured = req
			return &PatrolStreamResponse{Content: "ok", InputTokens: 5, OutputTokens: 9}, nil
		},
	})

	ps := NewPatrolService(svc, nil)
	ps.findings.Add(&Finding{
		ID:           "0e7c5dbb86bdebe9",
		Title:        "Powered Off on docker-host-edge-02",
		Severity:     FindingSeverityWarning,
		ResourceID:   "docker-host-edge-02",
		ResourceName: "docker-host-edge-02",
		ResourceType: "docker",
	})

	resp, err := ps.runAssessmentSweep(context.Background(), []string{"0e7c5dbb86bdebe9"}, "patrol-run-sweep")
	if err != nil {
		t.Fatalf("expected sweep to succeed, got %v", err)
	}
	if resp == nil || resp.InputTokens != 5 || resp.OutputTokens != 9 {
		t.Fatalf("unexpected sweep response: %+v", resp)
	}
	if captured.SessionID != "patrol-assess" {
		t.Fatalf("expected patrol-assess session, got %q", captured.SessionID)
	}
	if captured.MaxTurns != 3 {
		t.Fatalf("expected maxTurns len(pending)+2=3, got %d", captured.MaxTurns)
	}
	if !strings.Contains(captured.Prompt, "0e7c5dbb86bdebe9") {
		t.Fatalf("expected prompt to list the missing finding id")
	}
}

func TestRunAssessmentSweepSkipsWhenFindingsGone(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)

	called := false
	svc.SetChatService(&patrolMockChatService{
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			called = true
			return &PatrolStreamResponse{Content: "ok"}, nil
		},
	})

	ps := NewPatrolService(svc, nil)
	resp, err := ps.runAssessmentSweep(context.Background(), []string{"no-longer-active"}, "patrol-run-sweep")
	if err != nil {
		t.Fatalf("expected nil error when nothing pending, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected nil response when nothing pending, got %+v", resp)
	}
	if called {
		t.Fatal("expected no model call when the missing findings are no longer active")
	}
}
