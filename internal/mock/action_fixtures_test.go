package mock

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestCurrentFixtureGraphCarriesActionInboxFixtures(t *testing.T) {
	previous := IsMockEnabled()
	if err := SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = SetEnabled(previous) })

	graph := CurrentFixtureGraph()
	if len(graph.ActionFixtures) != 6 {
		t.Fatalf("action fixture count = %d, want 6", len(graph.ActionFixtures))
	}
	resources, _ := graph.UnifiedResourceSnapshot()
	resourceIDs := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resourceIDs[resource.ID] = struct{}{}
	}

	states := make(map[unifiedresources.ActionState]int)
	for _, fixture := range graph.ActionFixtures {
		if _, err := unifiedresources.NormalizeActionAuditRecord(fixture.Audit); err != nil {
			t.Fatalf("normalize action %q: %v", fixture.Audit.ID, err)
		}
		if _, ok := resourceIDs[fixture.Audit.Request.ResourceID]; !ok {
			t.Fatalf("action %q references non-graph resource %q", fixture.Audit.ID, fixture.Audit.Request.ResourceID)
		}
		if fixture.Audit.Plan.PolicyDecision.Status != unifiedresources.ActionPolicyDecisionResolved {
			t.Fatalf("action %q policy status = %q", fixture.Audit.ID, fixture.Audit.Plan.PolicyDecision.Status)
		}
		if len(fixture.Events) == 0 {
			t.Fatalf("action %q has no lifecycle events", fixture.Audit.ID)
		}
		states[fixture.Audit.State]++
	}

	for _, state := range []unifiedresources.ActionState{
		unifiedresources.ActionStatePending,
		unifiedresources.ActionStateApproved,
		unifiedresources.ActionStateExecuting,
		unifiedresources.ActionStateCompleted,
		unifiedresources.ActionStateRejected,
		unifiedresources.ActionStateFailed,
	} {
		if states[state] != 1 {
			t.Fatalf("state %q fixture count = %d, want 1", state, states[state])
		}
	}
}

func TestActionFixturesReturnsDefensiveCopies(t *testing.T) {
	previous := IsMockEnabled()
	if err := SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() { _ = SetEnabled(previous) })

	first := ActionFixtures()
	if len(first) == 0 {
		t.Fatal("expected action fixtures")
	}
	originalID := first[0].Audit.ID
	first[0].Audit.ID = "mutated"
	first[0].Audit.Request.Params["unexpected"] = true
	first[0].Events[0].Message = "mutated"

	second := ActionFixtures()
	if second[0].Audit.ID != originalID {
		t.Fatalf("fixture ID mutated through returned copy: %q", second[0].Audit.ID)
	}
	if len(second[0].Audit.Request.Params) != 0 {
		t.Fatalf("fixture params mutated through returned copy: %#v", second[0].Audit.Request.Params)
	}
	if second[0].Events[0].Message == "mutated" {
		t.Fatal("fixture event mutated through returned copy")
	}
}
