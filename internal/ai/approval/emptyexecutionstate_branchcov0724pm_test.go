package approval

import "testing"

// This file raises branch coverage on the unexported approval.emptyExecutionState
// helper (store.go:155), which constructs a blank ExecutionState ready for JSON
// transport. Its only behaviour is allocating non-nil map/slice collections via
// normalizeCollections so callers (and persisted payloads) never see null fields,
// even before any data is populated.
//
// emptyExecutionState takes no inputs and has no conditional branches of its own,
// so the coverage target is its observable contract: the zero-value shape and the
// independence of every returned pointer and collection.

func TestBranchcov0724pmEmptyExecutionStateShape(t *testing.T) {
	state := emptyExecutionState()

	if state == nil {
		t.Fatal("emptyExecutionState() returned nil pointer")
	}

	if state.ID != "" {
		t.Errorf("ID = %q, want empty", state.ID)
	}

	// The three collection fields must be non-nil and empty: this is the whole
	// point of normalizeCollections, and it is what keeps JSON payloads from
	// emitting null for these fields on a fresh state.
	if state.OriginalRequest == nil {
		t.Error("OriginalRequest = nil, want non-nil empty map")
	} else if len(state.OriginalRequest) != 0 {
		t.Errorf("len(OriginalRequest) = %d, want 0", len(state.OriginalRequest))
	}
	if state.PendingToolCall == nil {
		t.Error("PendingToolCall = nil, want non-nil empty map")
	} else if len(state.PendingToolCall) != 0 {
		t.Errorf("len(PendingToolCall) = %d, want 0", len(state.PendingToolCall))
	}
	if state.Messages == nil {
		t.Error("Messages = nil, want non-nil empty slice")
	} else if len(state.Messages) != 0 {
		t.Errorf("len(Messages) = %d, want 0", len(state.Messages))
	}

	// Time fields stay at the zero value on a fresh state; callers (CreateExecutionState)
	// are responsible for stamping CreatedAt/ExpiresAt.
	if !state.CreatedAt.IsZero() {
		t.Errorf("CreatedAt = %v, want zero time", state.CreatedAt)
	}
	if !state.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %v, want zero time", state.ExpiresAt)
	}
}

// TestBranchcov0724pmEmptyExecutionStateIndependentPointers proves every call
// returns a distinct allocation, and that mutating one state's collections
// cannot leak into another. This guards against a future change that reuses a
// shared backing map/slice.
func TestBranchcov0724pmEmptyExecutionStateIndependentPointers(t *testing.T) {
	first := emptyExecutionState()
	second := emptyExecutionState()

	if first == second {
		t.Fatal("emptyExecutionState() returned the same pointer twice; results must be independent")
	}

	// Mutate the first state's collections.
	first.OriginalRequest["k"] = "v"
	first.PendingToolCall["pending"] = true
	first.Messages = append(first.Messages, map[string]interface{}{"role": "user"})

	// The second state must remain empty and unaffected.
	if len(second.OriginalRequest) != 0 {
		t.Errorf("second.OriginalRequest leaked from first: %v", second.OriginalRequest)
	}
	if len(second.PendingToolCall) != 0 {
		t.Errorf("second.PendingToolCall leaked from first: %v", second.PendingToolCall)
	}
	if len(second.Messages) != 0 {
		t.Errorf("second.Messages leaked from first: %v", second.Messages)
	}

	// A third call must also be independent from the mutated first.
	third := emptyExecutionState()
	if len(third.OriginalRequest) != 0 || len(third.Messages) != 0 {
		t.Errorf("third state was not freshly allocated: orig=%v msgs=%v",
			third.OriginalRequest, third.Messages)
	}
}
