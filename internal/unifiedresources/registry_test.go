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
