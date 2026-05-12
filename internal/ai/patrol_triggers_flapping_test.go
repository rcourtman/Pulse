package ai

import "testing"

func TestFlappingPostmortemPatrolScope(t *testing.T) {
	scope := FlappingPostmortemPatrolScope("alert-vm-100-cpu", "vm-100", "vm", "cpu")

	if scope.Reason != TriggerReasonAlertFlapping {
		t.Errorf("expected reason %s, got %s", TriggerReasonAlertFlapping, scope.Reason)
	}
	if scope.Priority != triggerPriorityAnomaly {
		t.Errorf("expected priority %d, got %d", triggerPriorityAnomaly, scope.Priority)
	}
	if scope.Depth != PatrolDepthQuick {
		t.Errorf("expected quick depth, got %v", scope.Depth)
	}
	if len(scope.ResourceIDs) != 1 || scope.ResourceIDs[0] != "vm-100" {
		t.Errorf("expected resource vm-100, got %v", scope.ResourceIDs)
	}
	if len(scope.ResourceTypes) != 1 || scope.ResourceTypes[0] != "vm" {
		t.Errorf("expected resource type vm, got %v", scope.ResourceTypes)
	}
	if scope.AlertIdentifier != "alert-vm-100-cpu" {
		t.Errorf("expected alertIdentifier alert-vm-100-cpu, got %s", scope.AlertIdentifier)
	}
	if scope.Context != "Flapping: cpu" {
		t.Errorf("expected context 'Flapping: cpu', got %q", scope.Context)
	}
}

func TestFlappingPostmortemPatrolScope_EmptyAlertType(t *testing.T) {
	scope := FlappingPostmortemPatrolScope("alert-id", "vm-100", "vm", "")
	if scope.Context != "Flapping detected" {
		t.Errorf("expected context 'Flapping detected', got %q", scope.Context)
	}
}

func TestTriggerPatrol_FlappingGatedByAlertTriggersEnabled(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	tm.SetEventTriggerConfig(PatrolEventTriggerConfig{
		AlertTriggersEnabled:   false,
		AnomalyTriggersEnabled: true,
	})

	scope := FlappingPostmortemPatrolScope("alert-1", "vm-100", "vm", "cpu")
	if accepted := tm.TriggerPatrol(scope); accepted {
		t.Fatal("expected flapping postmortem trigger to be rejected when alert source is disabled")
	}
}

func TestTriggerPatrol_FlappingAcceptedWhenAlertTriggersEnabled(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	tm.SetEventTriggerConfig(PatrolEventTriggerConfig{
		AlertTriggersEnabled:   true,
		AnomalyTriggersEnabled: false,
	})

	scope := FlappingPostmortemPatrolScope("alert-1", "vm-100", "vm", "memory")
	if accepted := tm.TriggerPatrol(scope); !accepted {
		t.Fatal("expected flapping postmortem trigger to be accepted when alert source is enabled")
	}
}

func TestIsEventDrivenTrigger_IncludesFlapping(t *testing.T) {
	if !isEventDrivenTrigger(TriggerReasonAlertFlapping) {
		t.Fatal("expected TriggerReasonAlertFlapping to be treated as event-driven")
	}
}
