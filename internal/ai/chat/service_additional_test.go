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

func TestPatrolServiceSessionLifecycle(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore error: %v", err)
	}

	service := &Service{
		sessions: store,
		started:  true,
	}

	patrol := NewPatrolService(service)
	if err := patrol.CreatePatrolSession(context.Background()); err != nil {
		t.Fatalf("CreatePatrolSession error: %v", err)
	}
	if patrol.GetSessionID() == "" {
		t.Fatalf("expected session ID to be set")
	}

	patrol.mu.Lock()
	patrol.running = true
	patrol.mu.Unlock()
	if !patrol.IsRunning() {
		t.Fatalf("expected patrol to be running")
	}

	service.started = false
	if err := patrol.CreatePatrolSession(context.Background()); err == nil {
		t.Fatalf("expected error when service not running")
	}
}
