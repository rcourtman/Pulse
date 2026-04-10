package unifiedresources

import (
	"testing"
	"time"
)

func TestCompareResourcesByCanonicalNameUsesDeterministicTieBreakers(t *testing.T) {
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	left := Resource{
		ID:       "storage-b",
		Type:     ResourceTypeStorage,
		Name:     "Backup-Vault-A",
		Status:   StatusOnline,
		LastSeen: now,
	}
	right := Resource{
		ID:       "storage-a",
		Type:     ResourceTypeStorage,
		Name:     "backup-vault-a",
		Status:   StatusOnline,
		LastSeen: now,
	}

	if got := CompareResourcesByCanonicalName(left, right); got <= 0 {
		t.Fatalf("expected storage-b to sort after storage-a for equal canonical names, got %d", got)
	}
	if got := CompareResourcesByCanonicalName(right, left); got >= 0 {
		t.Fatalf("expected storage-a to sort before storage-b for equal canonical names, got %d", got)
	}
}

func TestRegistryListUsesDeterministicNameTieBreakers(t *testing.T) {
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	registry := NewRegistry(nil)
	registry.IngestResources([]Resource{
		{ID: "storage-z", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
		{ID: "storage-b", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
		{ID: "agent-1", Type: ResourceTypeAgent, Name: "alpha-host", Status: StatusOnline, LastSeen: now},
		{ID: "storage-a", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
	})

	got := registry.List()
	gotIDs := make([]string, 0, len(got))
	for _, resource := range got {
		gotIDs = append(gotIDs, resource.ID)
	}

	assertStringSlice(t, gotIDs, []string{"agent-1", "storage-a", "storage-b", "storage-z"})
}
