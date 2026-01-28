package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func TestServiceSettersAndAutonomousMode(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := &AgenticLoop{}
	service := &Service{
		executor:    executor,
		agenticLoop: loop,
	}

	service.SetIncidentRecorderProvider(nil)
	service.SetEventCorrelatorProvider(nil)
	service.SetTopologyProvider(nil)
	service.SetKnowledgeStoreProvider(nil)

	service.SetAutonomousMode(true)
	if !service.autonomousMode {
		t.Fatalf("expected autonomousMode true")
	}
	if !loop.autonomousMode {
		t.Fatalf("expected agentic loop to be autonomous")
	}
}

func TestServiceExecuteCommand_NoExecutor(t *testing.T) {
	service := &Service{}
	_, _, err := service.ExecuteCommand(context.Background(), "ls", "")
	if err == nil {
		t.Fatalf("expected error when executor is unavailable")
	}
}

// TestPatrolServiceSessionLifecycle was removed: it tested the deleted chat/patrol.go bridge.
