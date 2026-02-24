package unifiedresources

import (
	"testing"
	"time"
)

func TestResourceRegistry_ListByType(t *testing.T) {
	rr := NewRegistry(nil)

	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{MachineID: "machine-1"},
		},
	})

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "vm-100",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
		{
			SourceID: "vm-101",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-101",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-101"}},
		},
		{
			SourceID: "ct-200",
			Resource: Resource{
				Type:     ResourceTypeLXC,
				Name:     "ct-200",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"ct-200"}},
		},
	})

	got := rr.ListByType(ResourceTypeVM)
	if len(got) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(got))
	}
	for _, r := range got {
		if r.Type != ResourceTypeVM {
			t.Fatalf("expected all resources to be type %q, got %q", ResourceTypeVM, r.Type)
		}
	}

	// Deterministic ordering (sorted by ID).
	wantIDs := []string{
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100"),
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-101"),
	}
	// Hash order isn't meaningful; the contract is lexicographic by ID.
	if wantIDs[0] > wantIDs[1] {
		wantIDs[0], wantIDs[1] = wantIDs[1], wantIDs[0]
	}
	if got[0].ID != wantIDs[0] || got[1].ID != wantIDs[1] {
		t.Fatalf("expected IDs %v, got [%s %s]", wantIDs, got[0].ID, got[1].ID)
	}

	// Returned resources should be copies (mutating the result does not mutate the registry).
	origName := got[0].Name
	got[0].Name = "mutated"
	if r, ok := rr.Get(got[0].ID); !ok || r == nil {
		t.Fatalf("expected Get(%q) to succeed", got[0].ID)
	} else if r.Name != origName {
		t.Fatalf("expected registry resource name %q, got %q", origName, r.Name)
	}
}

func TestResourceRegistry_ListByType_Empty(t *testing.T) {
	rr := NewRegistry(nil)
	if got := rr.ListByType(ResourceTypeVM); len(got) != 0 {
		t.Fatalf("expected empty result, got %d", len(got))
	}
}

func TestResourceRegistry_IngestRecords_UnknownSource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)

	customSource := DataSource("xcp")
	rr.IngestRecords(customSource, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})

	hosts := rr.ListByType(ResourceTypeHost)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host for custom source, got %d", len(hosts))
	}
	if hosts[0].Name != "xcp-host-1" {
		t.Fatalf("expected host name xcp-host-1, got %q", hosts[0].Name)
	}
	targets := rr.SourceTargets(hosts[0].ID)
	if len(targets) != 1 {
		t.Fatalf("expected 1 source target, got %d", len(targets))
	}
	if targets[0].Source != customSource {
		t.Fatalf("expected custom source %q, got %q", customSource, targets[0].Source)
	}
	if targets[0].SourceID != "host-1" {
		t.Fatalf("expected source ID host-1, got %q", targets[0].SourceID)
	}
}

func TestResourceRegistry_BuildChildCounts_ReparentClearsOldParentCount(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 12, 1, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentAID := rr.sourceSpecificID(ResourceTypeHost, SourceProxmox, "node-a")
	parentBID := rr.sourceSpecificID(ResourceTypeHost, SourceProxmox, "node-b")
	vmID := rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100")

	parentA, ok := rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist", parentAID)
	}
	parentB, ok := rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist", parentBID)
	}
	vm, ok := rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist", vmID)
	}
	if parentA.ChildCount != 1 || parentB.ChildCount != 0 {
		t.Fatalf("expected initial child counts parentA=1 parentB=0, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-a" {
		t.Fatalf("expected vm parent name %q, got %q", "node-a", vm.ParentName)
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeHost,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentA, ok = rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist after reparent", parentAID)
	}
	parentB, ok = rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist after reparent", parentBID)
	}
	vm, ok = rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist after reparent", vmID)
	}
	if parentA.ChildCount != 0 || parentB.ChildCount != 1 {
		t.Fatalf("expected child counts parentA=0 parentB=1 after reparent, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-b" {
		t.Fatalf("expected vm parent name %q after reparent, got %q", "node-b", vm.ParentName)
	}
}
