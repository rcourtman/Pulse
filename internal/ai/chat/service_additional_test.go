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

	service.SetIncidentRecorderProvider(nil)
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

func TestServiceEffectiveControlLevelUsesResolver(t *testing.T) {
	service := NewService(Config{
		AIConfig: &config.AIConfig{ControlLevel: config.ControlLevelAutonomous},
		ControlLevelResolver: func(*config.AIConfig) string {
			return config.ControlLevelReadOnly
		},
	})

	service.mu.RLock()
	got := service.effectiveControlLevelLocked()
	service.mu.RUnlock()

	if got != tools.ControlLevelReadOnly {
		t.Fatalf("expected resolver-clamped control level %q, got %q", tools.ControlLevelReadOnly, got)
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
		ControlLevelResolver: func(cfg *config.AIConfig) string {
			return config.EffectiveControlLevelForEntitlement(cfg.GetControlLevel(), false)
		},
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
		t.Fatalf("expected autonomous setting to be clamped to %q, got %q", tools.ControlLevelControlled, gotLevel)
	}
}

// TestPatrolServiceSessionLifecycle was removed: it tested the deleted chat/patrol.go bridge.
