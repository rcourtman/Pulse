package tools

import "testing"

func TestPulseToolExecutorCloneIsolatesSessionState(t *testing.T) {
	creator := &mockPatrolFindingCreator{}
	resolved := &mockResolvedContext{}

	original := NewPulseToolExecutor(ExecutorConfig{})
	original.SetContext("node", "node-1", false)
	original.SetOrgID("tenant-a")
	original.SetResolvedContext(resolved)
	original.SetPatrolFindingCreator(creator)
	original.protectedGuests = []string{"101"}

	clone := original.Clone()
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if clone == original {
		t.Fatal("Clone() returned the original executor")
	}
	if clone.GetResolvedContext() != nil {
		t.Fatal("Clone() should start without a session-scoped resolved context")
	}
	if clone.GetPatrolFindingCreator() != creator {
		t.Fatal("Clone() should retain patrol creator availability for the new run")
	}
	if clone.targetType != original.targetType || clone.targetID != original.targetID || clone.isAutonomous != original.isAutonomous {
		t.Fatalf("Clone() lost base execution context: got %q/%q/%v", clone.targetType, clone.targetID, clone.isAutonomous)
	}
	if clone.orgID != original.orgID {
		t.Fatalf("Clone() orgID = %q, want %q", clone.orgID, original.orgID)
	}

	clone.SetContext("vm", "vm-201", true)
	if original.targetType != "node" || original.targetID != "node-1" || original.isAutonomous {
		t.Fatalf("original context mutated after clone update: got %q/%q/%v", original.targetType, original.targetID, original.isAutonomous)
	}

	clone.protectedGuests[0] = "999"
	if original.protectedGuests[0] != "101" {
		t.Fatalf("protectedGuests slice is shared between clone and original: got %q", original.protectedGuests[0])
	}
}

func TestPulseToolExecutorCloneDoesNotSharePatrolCreatorSlot(t *testing.T) {
	creator := &mockPatrolFindingCreator{}

	original := NewPulseToolExecutor(ExecutorConfig{})
	original.SetPatrolFindingCreator(creator)

	clone := original.Clone()
	clone.SetPatrolFindingCreator(nil)

	if original.GetPatrolFindingCreator() != creator {
		t.Fatal("clearing patrol creator on clone mutated the original executor")
	}
	if clone.GetPatrolFindingCreator() != nil {
		t.Fatal("clone patrol creator should be independently mutable")
	}
}
