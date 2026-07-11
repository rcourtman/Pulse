package unifiedresources

import "testing"

func TestSourceSpecificIDMatchesRegistryIngest(t *testing.T) {
	t.Parallel()

	rr := NewRegistry(nil)

	sourceID := "lab:pve-a:100"

	vm := Resource{
		Type:   ResourceTypeVM,
		Name:   "vm-100",
		Status: StatusOnline,
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{SourceID: sourceID, Resource: vm},
	})

	resources := rr.ListByType(ResourceTypeVM)
	if len(resources) != 1 {
		t.Fatalf("expected 1 VM resource, got %d", len(resources))
	}

	got := resources[0].ID
	want := SourceSpecificID(ResourceTypeVM, SourceProxmox, sourceID)
	if got != want {
		t.Fatalf("resource ID mismatch: got %q want %q", got, want)
	}
}

func TestSourceSpecificIDCanonicalizesSourceIDWhitespace(t *testing.T) {
	t.Parallel()

	got := SourceSpecificID(ResourceTypeVM, SourceProxmox, "  lab:pve-a:100  ")
	want := SourceSpecificID(ResourceTypeVM, SourceProxmox, "lab:pve-a:100")
	if got != want {
		t.Fatalf("SourceSpecificID should trim source ID whitespace: got %q want %q", got, want)
	}
}

func TestResourceIdentityPinEraIDs(t *testing.T) {
	pin := ResourceIdentityPin{
		CanonicalID:  buildHashID(ResourceTypeAgent, "machine:machine-1"),
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-1",
		DMIUUID:      "dmi-1",
		ClusterName:  "homelab",
		Hostname:     "delly.lan",
	}

	got := pin.EraIDs()
	// The pin preserves the full dotted hostname, and eras cover both the
	// full name and the short name the historical derivation hashed.
	want := []string{
		buildHashID(ResourceTypeAgent, "machine:machine-1"),
		buildHashID(ResourceTypeAgent, "dmi:dmi-1"),
		buildHashID(ResourceTypeAgent, "cluster:homelab:delly.lan"),
		buildHashID(ResourceTypeAgent, "hostname:delly.lan"),
		buildHashID(ResourceTypeAgent, "cluster:homelab:delly"),
		buildHashID(ResourceTypeAgent, "hostname:delly"),
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d era IDs, got %d: %v", len(want), len(got), got)
	}
	for _, id := range want {
		found := false
		for _, eraID := range got {
			if eraID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected era set to include %q, got %v", id, got)
		}
	}
}

func TestResourceIdentityPinEraIDsSkipsWeakOnlyKeys(t *testing.T) {
	pin := ResourceIdentityPin{
		CanonicalID:  "agent-custom",
		ResourceType: ResourceTypeAgent,
		Hostname:     "delly",
	}
	got := pin.EraIDs()
	want := []string{"agent-custom", buildHashID(ResourceTypeAgent, "hostname:delly")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("expected era IDs %v, got %v", want, got)
	}
}
