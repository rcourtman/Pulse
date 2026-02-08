package truenas

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestStatusFromPoolExhaustiveZFSStates(t *testing.T) {
	tests := []struct {
		status string
		want   unifiedresources.ResourceStatus
	}{
		{"ONLINE", unifiedresources.StatusOnline},
		{"online", unifiedresources.StatusOnline},
		{"HEALTHY", unifiedresources.StatusOnline},
		{"DEGRADED", unifiedresources.StatusWarning},
		{"degraded", unifiedresources.StatusWarning},
		{"FAULTED", unifiedresources.StatusOffline},
		{"OFFLINE", unifiedresources.StatusOffline},
		{"REMOVED", unifiedresources.StatusOffline},
		{"UNAVAIL", unifiedresources.StatusOffline},
		{"faulted", unifiedresources.StatusOffline},
		{"offline", unifiedresources.StatusOffline},
		{"removed", unifiedresources.StatusOffline},
		{"unavail", unifiedresources.StatusOffline},
		{"", unifiedresources.StatusUnknown},
		{"SOMETHING_ELSE", unifiedresources.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := statusFromPool(Pool{Status: tt.status})
			if got != tt.want {
				t.Fatalf("statusFromPool(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusFromDatasetStates(t *testing.T) {
	tests := []struct {
		name    string
		dataset Dataset
		want    unifiedresources.ResourceStatus
	}{
		{"mounted writable", Dataset{Mounted: true, ReadOnly: false}, unifiedresources.StatusOnline},
		{"mounted readonly", Dataset{Mounted: true, ReadOnly: true}, unifiedresources.StatusWarning},
		{"unmounted", Dataset{Mounted: false}, unifiedresources.StatusOffline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusFromDataset(tt.dataset)
			if got != tt.want {
				t.Fatalf("statusFromDataset(%+v) = %q, want %q", tt.dataset, got, tt.want)
			}
		})
	}
}

func TestDatasetStateTag(t *testing.T) {
	tests := []struct {
		name    string
		dataset Dataset
		want    string
	}{
		{"mounted writable", Dataset{Mounted: true, ReadOnly: false}, "state:mounted"},
		{"mounted readonly", Dataset{Mounted: true, ReadOnly: true}, "state:readonly"},
		{"unmounted", Dataset{Mounted: false}, "state:unmounted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := datasetStateTag(tt.dataset)
			if got != tt.want {
				t.Fatalf("datasetStateTag(%+v) = %q, want %q", tt.dataset, got, tt.want)
			}
		})
	}
}

func TestPoolRecordHasZFSStorageMeta(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	provider := NewDefaultProvider()
	records := provider.Records()

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage {
			continue
		}
		if !hasTag(record.Resource.Tags, "pool") && !hasTag(record.Resource.Tags, "dataset") {
			continue
		}
		if record.Resource.Storage == nil {
			t.Fatalf("expected StorageMeta on storage resource %q", record.Resource.Name)
		}
		if !record.Resource.Storage.IsZFS {
			t.Fatalf("expected IsZFS=true on storage resource %q", record.Resource.Name)
		}
		if hasTag(record.Resource.Tags, "pool") && record.Resource.Storage.Type != "zfs-pool" {
			t.Fatalf("expected Type=zfs-pool on pool %q, got %q", record.Resource.Name, record.Resource.Storage.Type)
		}
		if hasTag(record.Resource.Tags, "dataset") && record.Resource.Storage.Type != "zfs-dataset" {
			t.Fatalf("expected Type=zfs-dataset on dataset %q, got %q", record.Resource.Name, record.Resource.Storage.Type)
		}
	}
}

func TestPoolRecordTagsIncludeHealthStatus(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	fixtures := DefaultFixtures()
	provider := NewProvider(fixtures)
	records := provider.Records()

	for _, record := range records {
		if !hasTag(record.Resource.Tags, "pool") {
			continue
		}

		for _, pool := range fixtures.Pools {
			if record.Resource.Name == pool.Name {
				expectedTag := "health:" + strings.ToLower(strings.TrimSpace(pool.Status))
				if !hasTag(record.Resource.Tags, expectedTag) {
					t.Fatalf("pool %q missing health tag %q, got %v", pool.Name, expectedTag, record.Resource.Tags)
				}
			}
		}
	}
}

func hasTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if existing == tag {
			return true
		}
	}
	return false
}
