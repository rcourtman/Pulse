package ai

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type noExecutorChatService struct{}

func (n *noExecutorChatService) CreateSession(ctx context.Context) (*ChatSession, error) {
	return &ChatSession{ID: "noop"}, nil
}

func (n *noExecutorChatService) ExecuteStream(ctx context.Context, req ChatExecuteRequest, callback ChatStreamCallback) error {
	return nil
}

func (n *noExecutorChatService) ExecutePatrolStream(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
	return &PatrolStreamResponse{}, nil
}

func (n *noExecutorChatService) GetMessages(ctx context.Context, sessionID string) ([]ChatMessage, error) {
	return nil, nil
}

func (n *noExecutorChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (n *noExecutorChatService) ReloadConfig(ctx context.Context, cfg *config.AIConfig) error {
	return nil
}

func samplePatrolState() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "node-1",
				Status: "online",
				CPU:    0.15,
				Memory: models.Memory{Usage: 20.0},
				Disk:   models.Disk{Usage: 10.0},
			},
		},
	}
}

func TestComputePatrolMaxTurns(t *testing.T) {
	if got := computePatrolMaxTurns(0, nil); got != patrolMinTurns {
		t.Fatalf("expected min turns %d, got %d", patrolMinTurns, got)
	}

	if got := computePatrolMaxTurns(1000, nil); got != patrolMaxTurnsLimit {
		t.Fatalf("expected max turns %d, got %d", patrolMaxTurnsLimit, got)
	}

	quickScope := &PatrolScope{Depth: PatrolDepthQuick}
	if got := computePatrolMaxTurns(0, quickScope); got != patrolQuickMinTurns {
		t.Fatalf("expected quick min turns %d, got %d", patrolQuickMinTurns, got)
	}
	if got := computePatrolMaxTurns(200, quickScope); got != patrolQuickMaxTurns {
		t.Fatalf("expected quick max turns %d, got %d", patrolQuickMaxTurns, got)
	}
}

func TestEnsureInvestigationToolCall_NoOp(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ctx := context.Background()

	var mu sync.Mutex
	completed := []ToolCallRecord{{ToolName: "pulse_query"}}
	raw := []string{"existing"}

	ps.ensureInvestigationToolCall(ctx, &tools.PulseToolExecutor{}, &mu, &completed, &raw, true)
	if len(completed) != 1 || len(raw) != 1 {
		t.Fatalf("expected no changes when investigation tool already present")
	}

	ps.ensureInvestigationToolCall(ctx, nil, &mu, &completed, &raw, true)
	if len(completed) != 1 || len(raw) != 1 {
		t.Fatalf("expected no changes when executor is nil")
	}
}

func TestGetPatrolSystemPrompt_ModeSwitch(t *testing.T) {
	svc := &Service{cfg: &config.AIConfig{PatrolAutoFix: true}}
	ps := NewPatrolService(svc, nil)
	prompt := ps.getPatrolSystemPrompt()
	if !strings.Contains(prompt, "Auto-Fix Mode") || !strings.Contains(prompt, "pulse_control") {
		t.Fatalf("expected auto-fix prompt, got: %s", prompt)
	}

	svc.cfg = &config.AIConfig{PatrolAutoFix: false}
	prompt = ps.getPatrolSystemPrompt()
	if !strings.Contains(prompt, "Observe Only Mode") {
		t.Fatalf("expected observe-only prompt, got: %s", prompt)
	}
}

func TestRunAIAnalysis_EarlyErrors(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		_, err := ps.runAIAnalysis(context.Background(), models.StateSnapshot{}, nil)
		if err == nil {
			t.Fatal("expected error when aiService is nil")
		}
	})

	t.Run("budget exceeded", func(t *testing.T) {
		store := cost.NewStore(cost.DefaultMaxDays)
		store.Record(cost.UsageEvent{
			Provider:     "openai",
			RequestModel: "gpt-4o-mini",
			InputTokens:  1_000_000,
			OutputTokens: 0,
		})
		svc := &Service{
			cfg:       &config.AIConfig{CostBudgetUSD30d: 0.01},
			costStore: store,
		}
		ps := NewPatrolService(svc, nil)
		_, err := ps.runAIAnalysis(context.Background(), models.StateSnapshot{}, nil)
		if err == nil || !strings.Contains(err.Error(), "patrol skipped") {
			t.Fatalf("expected budget error, got %v", err)
		}
	})

	t.Run("chat service nil", func(t *testing.T) {
		svc := &Service{}
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "chat service not available") {
			t.Fatalf("expected chat service error, got %v", err)
		}
	})

	t.Run("executor accessor missing", func(t *testing.T) {
		svc := &Service{}
		svc.SetChatService(&noExecutorChatService{})
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "executor access") {
			t.Fatalf("expected executor access error, got %v", err)
		}
	})

	t.Run("executor nil", func(t *testing.T) {
		svc := &Service{}
		svc.SetChatService(&mockChatService{executor: nil})
		ps := NewPatrolService(svc, nil)
		scope := &PatrolScope{NoStream: true}
		_, err := ps.runAIAnalysis(context.Background(), samplePatrolState(), scope)
		if err == nil || !strings.Contains(err.Error(), "tool executor not available") {
			t.Fatalf("expected executor nil error, got %v", err)
		}
	})
}

func TestSeedResourceInventory_QuietSummary(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	cfg := DefaultPatrolConfig()
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "node-1",
				Status: "online",
				CPU:    0.10,
				Memory: models.Memory{Usage: 10.0},
				Disk:   models.Disk{Usage: 15.0},
			},
			{
				ID:     "node-2",
				Name:   "node-2",
				Status: "online",
				CPU:    0.20,
				Memory: models.Memory{Usage: 20.0},
				Disk:   models.Disk{Usage: 25.0},
			},
		},
	}

	out := ps.seedResourceInventory(state, nil, cfg, time.Now(), true)
	if !strings.Contains(out, "# Nodes: All 2") {
		t.Fatalf("expected quiet node summary, got: %s", out)
	}
}
