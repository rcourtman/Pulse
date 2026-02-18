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
