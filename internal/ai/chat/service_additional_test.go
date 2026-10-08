package chat

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestServiceSettersAndAutonomousMode(t *testing.T) {
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	loop := &AgenticLoop{}
	service := &Service{
		executor:    executor,
		agenticLoop: loop,
	}

	service.SetIncidentArchiveProvider(nil)
	service.SetEventCorrelatorProvider(nil)
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

func TestServiceEffectiveControlLevelReadsRetiredAutonomousAsControlled(t *testing.T) {
	service := NewService(Config{
		AIConfig: &config.AIConfig{ControlLevel: config.ControlLevelAutonomous},
	})

	service.mu.RLock()
	got := service.effectiveControlLevelLocked()
	service.mu.RUnlock()

	if got != tools.ControlLevelControlled {
		t.Fatalf("expected retired autonomous setting to read as %q, got %q", tools.ControlLevelControlled, got)
	}
}

func TestControlLevelForRequestAutonomousModeClampsAutonomousToControlled(t *testing.T) {
	requestApprovalMode := false
	if got := controlLevelForRequestAutonomousMode(tools.ControlLevelAutonomous, &requestApprovalMode); got != tools.ControlLevelControlled {
		t.Fatalf("expected request approval mode to clamp autonomous control level to %q, got %q", tools.ControlLevelControlled, got)
	}
	if got := controlLevelForRequestAutonomousMode(tools.ControlLevelReadOnly, &requestApprovalMode); got != tools.ControlLevelReadOnly {
		t.Fatalf("expected request approval mode not to upgrade read-only level, got %q", got)
	}

	requestAutonomousMode := true
	if got := controlLevelForRequestAutonomousMode(tools.ControlLevelControlled, &requestAutonomousMode); got != tools.ControlLevelControlled {
		t.Fatalf("expected request autonomous mode not to upgrade controlled entitlement, got %q", got)
	}
}

func TestServiceUpdateControlSettingsRefreshesEffectiveConfig(t *testing.T) {
	service := NewService(Config{
		AIConfig: &config.AIConfig{ControlLevel: config.ControlLevelReadOnly},
	})

	next := &config.AIConfig{ControlLevel: config.ControlLevelAutonomous}
	service.UpdateControlSettings(next)

	service.mu.RLock()
	gotCfg := service.cfg
	gotLevel := service.effectiveControlLevelLocked()
	service.mu.RUnlock()

	if gotCfg != next {
		t.Fatal("expected UpdateControlSettings to refresh the service config")
	}
	if gotLevel != tools.ControlLevelControlled {
		t.Fatalf("expected retired autonomous setting to read as %q, got %q", tools.ControlLevelControlled, gotLevel)
	}
}

// TestPatrolServiceSessionLifecycle was removed: it tested the deleted chat/patrol.go bridge.
