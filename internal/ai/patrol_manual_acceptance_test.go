package ai

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestForcePatrolAcceptanceOwnsStatusAndProducesOneHistoryRecord(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	service := NewService(persistence, nil)
	service.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model"}
	service.provider = &mockProvider{}

	providerRelease := make(chan struct{})
	providerEntered := make(chan struct{})
	var providerCalls atomic.Int32
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	service.SetChatService(&mockChatService{
		executor: executor,
		executePatrolStreamFunc: func(_ context.Context, _ PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			providerCalls.Add(1)
			close(providerEntered)
			<-providerRelease
			content, err := json.Marshal(struct {
				Text string `json:"text"`
			}{Text: "No actionable issues found."})
			if err != nil {
				return nil, err
			}
			callback(ChatStreamEvent{Type: "content", Data: content})
			return &PatrolStreamResponse{
				Content:      "No actionable issues found.",
				InputTokens:  12,
				OutputTokens: 5,
			}, nil
		},
	})

	patrol := NewPatrolService(service, &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{{ID: "node-1", Name: "pve-1", Status: "online"}},
		},
	})
	patrol.SetConfig(PatrolConfig{
		Enabled:      true,
		AnalyzeNodes: true,
	})

	acceptance, accepted := patrol.ForcePatrol(context.Background())
	if !accepted {
		t.Fatal("manual run was not accepted")
	}
	if acceptance.RunID == "" || acceptance.StartedAt.IsZero() {
		t.Fatalf("incomplete acceptance: %+v", acceptance)
	}

	status := patrol.GetStatus()
	if !status.Running {
		t.Fatal("accepted run was not synchronously visible as running")
	}
	if status.CurrentRunID != acceptance.RunID {
		t.Fatalf("status run id = %q, want %q", status.CurrentRunID, acceptance.RunID)
	}

	if duplicate, duplicateAccepted := patrol.ForcePatrol(context.Background()); duplicateAccepted {
		t.Fatalf("duplicate run was accepted: %+v", duplicate)
	}

	select {
	case <-providerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted run did not reach the controlled provider")
	}
	if calls := providerCalls.Load(); calls != 1 {
		t.Fatalf("provider calls = %d, want 1", calls)
	}

	close(providerRelease)
	deadline := time.Now().Add(3 * time.Second)
	for patrol.GetStatus().Running && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if patrol.GetStatus().Running {
		t.Fatal("manual run did not finish")
	}

	history := patrol.GetRunHistory(10)
	if len(history) != 1 {
		t.Fatalf("history records = %d, want 1: %+v", len(history), history)
	}
	if history[0].ID != acceptance.RunID {
		t.Fatalf("history run id = %q, want %q", history[0].ID, acceptance.RunID)
	}
	if calls := providerCalls.Load(); calls != 1 {
		t.Fatalf("provider calls after completion = %d, want 1", calls)
	}
}

func TestAcceptedManualPatrolRecordsRuntimeStateFailure(t *testing.T) {
	patrol := NewPatrolService(nil, nil)
	patrol.SetConfig(PatrolConfig{Enabled: true})

	acceptance, accepted := patrol.ForcePatrol(context.Background())
	if !accepted {
		t.Fatal("manual run was not accepted")
	}

	deadline := time.Now().Add(2 * time.Second)
	for patrol.GetStatus().Running && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if patrol.GetStatus().Running {
		t.Fatal("manual run did not terminate")
	}

	history := patrol.GetRunHistory(10)
	if len(history) != 1 {
		t.Fatalf("history records = %d, want 1: %+v", len(history), history)
	}
	if history[0].ID != acceptance.RunID {
		t.Fatalf("history run id = %q, want %q", history[0].ID, acceptance.RunID)
	}
	if history[0].Status != "error" || history[0].ErrorCount != 1 {
		t.Fatalf("history failure = status %q, errors %d", history[0].Status, history[0].ErrorCount)
	}
}
