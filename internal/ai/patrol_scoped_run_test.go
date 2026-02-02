package ai

import (
	"context"
	"testing"
	"time"
)

func TestRunScopedPatrol_Disabled(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: false}
	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node-1"}})
	// No panic and no state changes expected.
}

func TestRunScopedPatrol_RequeueWhenRunInProgress(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: true}
	ps.runInProgress = true

	scope := PatrolScope{ResourceIDs: []string{"node-1"}, Reason: TriggerReasonManual}
	ps.runScopedPatrol(context.Background(), scope)

	if !ps.runInProgress {
		t.Fatalf("expected runInProgress to remain true when run is already in progress")
	}
}

func TestRunScopedPatrol_StuckRunClearsAndNoStateProvider(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.config = PatrolConfig{Enabled: true}
	ps.runInProgress = true
	ps.runStartedAt = time.Now().Add(-21 * time.Minute)

	ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{"node-1"}, Reason: TriggerReasonManual})

	if ps.runInProgress {
		t.Fatalf("expected stuck run to be cleared")
	}
}
