package chat

import (
	"testing"
)

func TestSessionStore_KnowledgeAndToolSets(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	ka1 := store.GetKnowledgeAccumulator(session.ID)
	ka2 := store.NewKnowledgeAccumulatorForRun(session.ID)
	if ka1 == ka2 {
		t.Fatalf("expected new knowledge accumulator for run")
	}

	toolSet := map[string]bool{"pulse_query": true}
	store.SetToolSet(session.ID, toolSet)
	got := store.GetToolSet(session.ID)
	if got == nil || !got["pulse_query"] {
		t.Fatalf("expected tool set entry")
	}
	got["pulse_query"] = false
	if store.GetToolSet(session.ID)["pulse_query"] != true {
		t.Fatalf("expected tool set to be copied")
	}

	updated := store.AddToolSet(session.ID, map[string]bool{"pulse_metrics": true})
	if !updated["pulse_metrics"] {
		t.Fatalf("expected tool set to include additions")
	}
}

func TestSessionStore_ResolvedContextLifecycle(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		Name:           "alpha",
		TargetHost:     "alpha",
		AllowedActions: []string{"start"},
	}
	store.AddResolvedResource(session.ID, res.Name, res)

	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "start"); err != nil {
		t.Fatalf("expected action to be allowed: %v", err)
	}
	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "stop"); err == nil {
		t.Fatalf("expected action to be blocked")
	}

	store.ClearResolvedContext(session.ID)
	if _, err := store.ValidateResourceForAction(session.ID, res.ResourceID, "start"); err == nil {
		t.Fatalf("expected resource to be unresolved after clear")
	}
}

func TestSessionStore_ClearSessionState(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Set up context, FSM, and toolset
	res := &ResolvedResource{ResourceID: "node:node1", Name: "node1", ResourceType: "node"}
	store.AddResolvedResource(session.ID, res.Name, res)
	ctx := store.GetResolvedContext(session.ID)
	ctx.PinResource(res.ResourceID)

	fsm := store.GetSessionFSM(session.ID)
	fsm.State = StateVerifying
	store.SetToolSet(session.ID, map[string]bool{"pulse_query": true})
	store.GetKnowledgeAccumulator(session.ID)

	store.ClearSessionState(session.ID, true)
	if !store.GetResolvedContext(session.ID).HasAnyResources() {
		t.Fatalf("expected pinned resources to remain")
	}
	if fsm.State != StateReading {
		t.Fatalf("expected FSM to keep progress when pinned resources remain")
	}
	if store.GetToolSet(session.ID) == nil {
		t.Fatalf("expected toolset to remain when keepPinned=true")
	}

	store.ClearSessionState(session.ID, false)
	if store.GetToolSet(session.ID) != nil {
		t.Fatalf("expected toolset to be cleared when keepPinned=false")
	}
}

func TestSessionStore_ResetFSMAndCleanupContext(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	session, err := store.Create()
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	fsm := store.GetSessionFSM(session.ID)
	fsm.State = StateVerifying
	store.ResetSessionFSM(session.ID, true)
	if fsm.State != StateReading {
		t.Fatalf("expected ResetSessionFSM keep progress to move to READING")
	}

	fsm.State = StateVerifying
	store.ResetSessionFSM(session.ID, false)
	if fsm.State != StateResolving {
		t.Fatalf("expected ResetSessionFSM full reset to move to RESOLVING")
	}

	store.AddResolvedResource(session.ID, "node1", &ResolvedResource{ResourceID: "node:node1", Name: "node1"})
	store.cleanupResolvedContext(session.ID)
	if store.GetResolvedContext(session.ID).HasAnyResources() {
		t.Fatalf("expected cleanupResolvedContext to remove resources")
	}
}
