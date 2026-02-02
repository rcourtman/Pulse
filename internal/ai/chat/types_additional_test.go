package chat

import (
	"strings"
	"testing"
	"time"
)

func TestResolvedResource_GettersAndAliases(t *testing.T) {
	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		TargetHost:     "alpha",
		AgentID:        "agent-1",
		Adapter:        "qm",
		VMID:           101,
		Node:           "node1",
		AllowedActions: []string{"start"},
		ProviderUID:    "101",
		Kind:           "vm",
		Aliases:        []string{"alpha", "Alpha-VM"},
		Name:           "alpha",
	}

	if res.GetResourceID() != "vm:node1:101" || res.GetResourceType() != "vm" {
		t.Fatalf("expected getters to return structured fields")
	}
	if res.GetTargetHost() != "alpha" || res.GetAgentID() != "agent-1" || res.GetAdapter() != "qm" {
		t.Fatalf("expected routing getters to return values")
	}
	if res.GetVMID() != 101 || res.GetNode() != "node1" {
		t.Fatalf("expected VMID/node getters to return values")
	}
	if len(res.GetAllowedActions()) != 1 || res.GetAllowedActions()[0] != "start" {
		t.Fatalf("expected allowed actions getter to return actions")
	}
	if len(res.GetAliases()) != 2 {
		t.Fatalf("expected aliases getter to return values")
	}
	if !res.HasAlias("alpha-vm") {
		t.Fatalf("expected HasAlias to be case-insensitive")
	}
}

func TestResolvedResource_GetBestExecutor(t *testing.T) {
	res := &ResolvedResource{
		ReachableVia: []ExecutorPath{
			{ExecutorID: "a", Actions: []string{"restart"}, Priority: 1},
			{ExecutorID: "b", Actions: []string{"*"}, Priority: 0},
		},
	}

	best := res.GetBestExecutor("restart")
	if best == nil || best.ExecutorID != "a" {
		t.Fatalf("expected highest priority executor for action")
	}

	best = res.GetBestExecutor("stop")
	if best == nil || best.ExecutorID != "b" {
		t.Fatalf("expected wildcard executor for action")
	}
}

func TestResolvedContext_AccessAndValidation(t *testing.T) {
	ctx := NewResolvedContext("session-1")
	if ctx.HasAnyResources() {
		t.Fatalf("expected empty context")
	}

	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		Name:           "alpha",
		AllowedActions: []string{"start"},
	}
	ctx.AddResource("alpha", res)
	if !ctx.HasAnyResources() {
		t.Fatalf("expected resources to exist")
	}

	if _, ok := ctx.GetResourceByID(res.ResourceID); !ok {
		t.Fatalf("expected GetResourceByID to find resource")
	}

	if _, err := ctx.ValidateResourceID("missing"); err == nil {
		t.Fatalf("expected resource not resolved error")
	}

	if err := ctx.ValidateAction(res.ResourceID, "start"); err != nil {
		t.Fatalf("expected action allowed")
	}
	if err := ctx.ValidateAction(res.ResourceID, "stop"); err == nil {
		t.Fatalf("expected action not allowed")
	}

	if _, err := ctx.ValidateResourceForAction(res.ResourceID, "start"); err != nil {
		t.Fatalf("expected ValidateResourceForAction to succeed: %v", err)
	}
}

func TestResolvedContext_ExplicitAccess(t *testing.T) {
	ctx := NewResolvedContext("session-1")
	res := &ResolvedResource{ResourceID: "node:node1", Name: "node1"}
	ctx.AddResourceWithExplicitAccess(res.Name, res)

	recent := ctx.GetRecentlyAccessedResourcesSorted(5*time.Minute, 10)
	if len(recent) != 1 || recent[0] != res.ResourceID {
		t.Fatalf("expected recently accessed resource to be tracked")
	}
}

func TestResolvedContextErrors(t *testing.T) {
	resErr := (&ResourceNotResolvedError{ResourceID: "missing"}).Error()
	if !strings.Contains(resErr, "missing") {
		t.Fatalf("expected resource error to include ID")
	}

	actionErr := (&ActionNotAllowedError{ResourceID: "vm:1", Action: "stop"}).Error()
	if !strings.Contains(actionErr, "stop") {
		t.Fatalf("expected action error to include action")
	}
}
