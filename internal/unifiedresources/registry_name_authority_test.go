package unifiedresources

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"testing"
)

func TestProviderRenameKeepsCanonicalIdentityAndRefreshesName(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{{ID: "cluster-a:old-node:101", Instance: "cluster-a", Node: "old-node", VMID: 101, Name: "before", Type: "qemu"}}})
	id := rr.VMs()[0].ID()
	rr.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{{ID: "cluster-a:new-node:101", Instance: "cluster-a", Node: "new-node", VMID: 101, Name: "after", Type: "qemu"}}})
	resource, ownedID, found := rr.GetByReference(id)
	if !found || ownedID != id || resource.Name != "after" || resource.Proxmox.NodeName != "new-node" {
		t.Fatalf("provider rename lost current owned identity: resource=%+v id=%s found=%v", resource, ownedID, found)
	}
	if resource.Canonical == nil || resource.Canonical.DisplayName != "after" {
		t.Fatalf("canonical display did not refresh: %+v", resource.Canonical)
	}
}

func TestProviderRenamePreservesPresentAgentNameAuthority(t *testing.T) {
	store := NewMemoryStore()
	rr := NewRegistry(nil)
	snapshot := models.StateSnapshot{
		VMs:   []models.VM{{ID: "cluster-a:node1:101", Instance: "cluster-a", Node: "node1", VMID: 101, Name: "provider-name", Type: "qemu"}},
		Hosts: []models.Host{{ID: "native-agent", Hostname: "agent-name", Status: "online", Platform: "linux"}},
	}
	rr.IngestSnapshot(snapshot)
	guestID, agentID := rr.VMs()[0].ID(), rr.Hosts()[0].ID()
	if err := store.AddLink(ResourceLink{ResourceA: guestID, ResourceB: agentID, PrimaryID: guestID}); err != nil {
		t.Fatal(err)
	}
	rr = NewRegistry(store)
	rr.IngestSnapshot(snapshot)
	before, _, found := rr.GetByReference(guestID)
	if !found || before.Agent == nil {
		t.Fatal("fixture did not attach an agent to the guest")
	}
	provider, _ := resourceFromVM(models.VM{ID: "cluster-a:node1:101", Instance: "cluster-a", Node: "node1", VMID: 101, Name: "provider-renamed", Type: "qemu"})
	rr.IngestRecords(SourceProxmox, []IngestRecord{{SourceID: "cluster-a:node1:101", Resource: provider}})
	after, _, found := rr.GetByReference(guestID)
	if !found || after.ID != guestID || after.Agent == nil || after.Name != before.Name {
		t.Fatalf("provider rename displaced present agent authority: before=%+v after=%+v", before, after)
	}
}
