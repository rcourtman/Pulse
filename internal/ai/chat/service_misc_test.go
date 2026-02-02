package chat

import (
	"context"
	"testing"
	"time"
)

func TestAbortSession(t *testing.T) {
	svc := &Service{}
	if err := svc.AbortSession(context.Background(), "sess-1"); err == nil {
		t.Fatalf("expected error when service not started")
	}

	loop := &AgenticLoop{aborted: make(map[string]bool)}
	svc.agenticLoop = loop
	if err := svc.AbortSession(context.Background(), "sess-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loop.aborted["sess-2"] {
		t.Fatalf("expected session to be marked aborted")
	}
}

func TestResolvedContext_GetResourceAliasAndMiss(t *testing.T) {
	ctx := NewResolvedContext("session")
	res := &ResolvedResource{
		ResourceID:   "node:alpha",
		Name:         "alpha",
		Aliases:      []string{"@alpha", "alpha-node"},
		ResourceType: "node",
	}
	ctx.AddResource(res.Name, res)

	if got, ok := ctx.GetResource("@alpha"); !ok || got == nil {
		t.Fatalf("expected alias lookup to succeed")
	}
	if got, ok := ctx.GetResource("missing"); ok || got != nil {
		t.Fatalf("expected missing resource to return false")
	}
}

func TestResolvedContext_TouchInitializesMap(t *testing.T) {
	ctx := &ResolvedContext{}
	ctx.touch("node:1")
	if ctx.lastAccessed == nil {
		t.Fatalf("expected lastAccessed map to be initialized")
	}
	if _, ok := ctx.lastAccessed["node:1"]; !ok {
		t.Fatalf("expected access time to be recorded")
	}
}

func TestSessionFSM_CleanupExpiredRecoveries(t *testing.T) {
	fsm := &SessionFSM{}
	fsm.cleanupExpiredRecoveries()
	if fsm.PendingRecoveries == nil {
		t.Fatalf("expected pending recoveries to be initialized")
	}

	now := time.Now()
	fsm.PendingRecoveries["old"] = &PendingRecovery{CreatedAt: now.Add(-2 * RecoveryTTL)}
	fsm.PendingRecoveries["new"] = &PendingRecovery{CreatedAt: now.Add(-time.Minute)}

	fsm.cleanupExpiredRecoveries()
	if _, ok := fsm.PendingRecoveries["old"]; ok {
		t.Fatalf("expected expired recovery to be removed")
	}
	if _, ok := fsm.PendingRecoveries["new"]; !ok {
		t.Fatalf("expected recent recovery to remain")
	}
}
