package agentcapabilities

import (
	"reflect"
	"strings"
	"testing"
)

func TestAgentActionableEventKindsReturnsDetachedKnownKinds(t *testing.T) {
	got := AgentActionableEventKinds()
	want := []string{
		string(EventKindFindingCreated),
		string(EventKindApprovalPending),
		string(EventKindActionCompleted),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AgentActionableEventKinds() = %v, want %v", got, want)
	}

	got[0] = "mutated"
	second := AgentActionableEventKinds()
	if second[0] != string(EventKindFindingCreated) {
		t.Fatalf("AgentActionableEventKinds returned aliased slice: %v", second)
	}
}

func TestIsTransportEventKindPinsStreamPlumbing(t *testing.T) {
	for _, kind := range []EventKind{EventKindStreamConnected, EventKindHeartbeat} {
		if !IsTransportEventKind(string(kind)) {
			t.Fatalf("%s must be transport plumbing", kind)
		}
	}
	for _, kind := range AgentActionableEventKinds() {
		if IsTransportEventKind(kind) {
			t.Fatalf("%s must remain agent-actionable", kind)
		}
	}
}

func TestSubscribeEventsDescriptionUsesSharedEventKinds(t *testing.T) {
	description := SubscribeEventsDescription()
	for _, kind := range append(AgentActionableEventKinds(), string(EventKindHeartbeat)) {
		if !strings.Contains(description, kind) {
			t.Fatalf("subscribe_events description missing %q: %s", kind, description)
		}
	}
}
