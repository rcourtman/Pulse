package tools

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
)

func TestCurrentResourceAddressingKeepsTheSelectedCanonicalOwner(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{
		{ID: "a:node1:101", Instance: "a", Node: "node1", VMID: 101, Name: "first", Type: "qemu"},
		{ID: "b:node1:101", Instance: "b", Node: "node1", VMID: 101, Name: "second", Type: "qemu"},
	}})
	selected := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "b", 101)
	resource := &mockResource{resourceID: selected, providerUID: "101", kind: "vm", aliases: []string{"first"}, targetHost: "node1"}
	executor := NewPulseToolExecutor(ExecutorConfig{ReadState: rr})
	target, err := executor.commandTargetForResolvedResource(resource)
	if err != nil || target != selected {
		t.Fatalf("selected owner rebound through provider UID or alias: %q %v", target, err)
	}
	if got := canonicalQueryIDForResolvedResource(resource); got != selected {
		t.Fatalf("selected read uses unscoped ID %s", got)
	}
	resource.resourceID = "vm-unknown"
	if target, err := executor.commandTargetForResolvedResource(resource); err == nil {
		t.Fatalf("missing owner rebound to %s", target)
	}
}
