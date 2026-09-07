package chat

import (
	"context"
	"testing"
)

func TestAbortSession(t *testing.T) {
	svc := &Service{}
	if err := svc.AbortSession(context.Background(), "sess-1"); err == nil {
		t.Fatalf("expected error when service not started")
	}

	loop := &AgenticLoop{aborted: make(map[string]bool)}
	svc.agenticLoop = loop
	svc.started = true
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
