package chat

import (
	"encoding/json"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
	"time"
)

type handoffIdentityRegistry struct {
	*unifiedresources.ResourceRegistry
}

func (p handoffIdentityRegistry) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	return p.ListByType(t)
}

func TestResolvedContextSameNameOwnersSurvivePersistence(t *testing.T) {
	rc := NewResolvedContext("two-sites")
	for _, id := range []string{"vm-site-a", "vm-site-b"} {
		rc.AddResource("duplicate", &ResolvedResource{ResourceID: id, Name: "duplicate", Kind: "vm", Aliases: []string{"duplicate", "101"}, ResolvedAt: time.Now()})
	}
	if _, ok := rc.GetResource("duplicate"); ok {
		t.Fatal("duplicate name picked an owner")
	}
	if _, ok := rc.GetResolvedResourceByAlias("101"); ok {
		t.Fatal("duplicate native ID picked an owner")
	}
	encoded, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	var restored ResolvedContext
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"vm-site-a", "vm-site-b"} {
		res, ok := restored.GetResourceByID(id)
		if !ok || res.ResourceID != id {
			t.Fatalf("lost owner %s after round trip: %s", id, encoded)
		}
	}
	if _, ok := restored.GetResource("duplicate"); ok {
		t.Fatal("reload revived ambiguous name")
	}
	restored.PinResource("vm-site-b")
	restored.Clear(true)
	if res, ok := restored.GetResource("duplicate"); !ok || res.ResourceID != "vm-site-b" {
		t.Fatal("removing one owner failed to restore the remaining unique alias")
	}
}

func TestResolvedContextCanonicalHandoffDoesNotRebuildProviderIdentity(t *testing.T) {
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{
		{ID: "site-a:node1:101", Instance: "site-a", Node: "node1", VMID: 101, Name: "duplicate", Type: "qemu"},
		{ID: "site-b:node1:101", Instance: "site-b", Node: "node1", VMID: 101, Name: "duplicate", Type: "qemu"},
	}})
	rc := NewResolvedContext("two-sites")
	for _, vm := range rr.VMs() {
		reg, ok := tools.CanonicalHandoffResourceRegistration(handoffIdentityRegistry{rr}, vm.ID(), "stale-label", "agent", "wrong-node")
		if !ok || reg.ResourceID != vm.ID() || reg.Kind != "vm" {
			t.Fatalf("client label/type overrode owner: %+v", reg)
		}
		rc.AddResolvedResource(reg)
		rc.MarkExplicitAccess(reg.ResourceID)
	}
	if len(rc.ListResolvedResources()) != 2 {
		t.Fatal("canonical registrations conflated provider scopes")
	}
	for _, vm := range rr.VMs() {
		if res, ok := rc.GetResourceByID(vm.ID()); !ok || res.VMID != 101 || res.Kind != "vm" {
			t.Fatal("canonical ID lost native executor coordinates")
		}
	}
	if _, ok := tools.CanonicalHandoffResourceRegistration(handoffIdentityRegistry{rr}, "vm-missing", "duplicate", "vm", "node1"); ok {
		t.Fatal("unknown explicit ID recovered through display name")
	}
	if _, ok := tools.CanonicalHandoffResourceRegistration(handoffIdentityRegistry{rr}, "vm:node1:101", "duplicate", "vm", "node1"); ok {
		t.Fatal("ambiguous legacy selection picked a site")
	}
	if _, ok := tools.CanonicalHandoffResourceRegistration(handoffIdentityRegistry{rr}, "", "duplicate", "vm", ""); ok {
		t.Fatal("name-only handoff picked a site")
	}
}

func TestResolvedContextReadsLegacyNameKeyedJSON(t *testing.T) {
	old := map[string]any{"session_id": "legacy", "resources": map[string]*ResolvedResource{
		"old-label": {ResourceID: "vm-old", Name: "current-label", Kind: "vm", Aliases: []string{"old-label"}},
	}}
	encoded, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var rc ResolvedContext
	if err := json.Unmarshal(encoded, &rc); err != nil {
		t.Fatal(err)
	}
	if res, ok := rc.GetResourceByID("vm-old"); !ok || res.Name != "current-label" {
		t.Fatal("legacy context lost its contained resource ID")
	}
	if res, ok := rc.GetResource("old-label"); !ok || res.ResourceID != "vm-old" {
		t.Fatal("legacy alias was not retained")
	}
}
