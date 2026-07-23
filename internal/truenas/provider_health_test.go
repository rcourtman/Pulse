package truenas

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
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

func TestBootPoolTagPreservesPoolRole(t *testing.T) {
	if tags := poolTags(Pool{Status: "ONLINE", IsBoot: true}); !hasTag(tags, "boot-pool") {
		t.Fatalf("expected boot-pool tag, got %v", tags)
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
		{"replication readonly", Dataset{Mounted: true, ReadOnly: true, ReadOnlyReason: DatasetReadOnlyReplicationTarget}, unifiedresources.StatusOnline},
		{"locked replication readonly", Dataset{Mounted: false, Locked: true, ReadOnly: true, ReadOnlyReason: DatasetReadOnlyReplicationTarget}, unifiedresources.StatusOffline},
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

func TestDiskHealthMappingDistinguishesUnavailableTelemetryFromFailure(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		health     string
		hasHealth  bool
		wantHealth string
		wantStatus unifiedresources.ResourceStatus
		wantLevel  storagehealth.RiskLevel
	}{
		{
			name:       "online passed",
			status:     "ONLINE",
			wantHealth: "PASSED",
			wantStatus: unifiedresources.StatusOnline,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "online explicit smart unavailable",
			status:     "ONLINE",
			health:     "UNAVAILABLE",
			hasHealth:  true,
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusOnline,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "online explicit smart failed",
			status:     "ONLINE",
			health:     "FAILED",
			hasHealth:  true,
			wantHealth: "FAILED",
			wantStatus: unifiedresources.StatusOnline,
			wantLevel:  storagehealth.RiskCritical,
		},
		{
			name:       "degraded warning",
			status:     "DEGRADED",
			wantHealth: "DEGRADED",
			wantStatus: unifiedresources.StatusWarning,
			wantLevel:  storagehealth.RiskWarning,
		},
		{
			name:       "failed state",
			status:     "FAILED",
			wantHealth: "FAILED",
			wantStatus: unifiedresources.StatusOffline,
			wantLevel:  storagehealth.RiskCritical,
		},
		{
			name:       "faulted state",
			status:     "FAULTED",
			wantHealth: "FAILED",
			wantStatus: unifiedresources.StatusOffline,
			wantLevel:  storagehealth.RiskCritical,
		},
		{
			name:       "zfs unavailable state",
			status:     "UNAVAIL",
			wantHealth: "FAILED",
			wantStatus: unifiedresources.StatusOffline,
			wantLevel:  storagehealth.RiskCritical,
		},
		{
			name:       "null status decoded as empty",
			status:     "",
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusUnknown,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "unknown status",
			status:     "UNKNOWN",
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusUnknown,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "smart unavailable",
			status:     "UNAVAILABLE",
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusUnknown,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "not available",
			status:     "N/A",
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusUnknown,
			wantLevel:  storagehealth.RiskHealthy,
		},
		{
			name:       "provider-specific unavailable text",
			status:     "SMART unavailable",
			wantHealth: "UNKNOWN",
			wantStatus: unifiedresources.StatusUnknown,
			wantLevel:  storagehealth.RiskHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disk := Disk{Name: "sda", Status: tt.status, Health: tt.health, HealthStatusPresent: tt.hasHealth, Model: "Generic SATA"}
			if got := healthFromDisk(disk); got != tt.wantHealth {
				t.Fatalf("healthFromDisk(%q) = %q, want %q", tt.status, got, tt.wantHealth)
			}
			if got := statusFromDisk(disk); got != tt.wantStatus {
				t.Fatalf("statusFromDisk(%q) = %q, want %q", tt.status, got, tt.wantStatus)
			}
			if got := assessDisk(disk).Level; got != tt.wantLevel {
				t.Fatalf("assessDisk(%q).Level = %q, want %q", tt.status, got, tt.wantLevel)
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
		{"replication readonly", Dataset{Mounted: true, ReadOnly: true, ReadOnlyReason: DatasetReadOnlyReplicationTarget}, "state:replication-readonly"},
		{"locked replication readonly", Dataset{Mounted: false, Locked: true, ReadOnly: true, ReadOnlyReason: DatasetReadOnlyReplicationTarget}, "state:locked"},
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
		if got := record.Resource.Storage.Platform; got != "truenas" {
			t.Fatalf("expected Platform=truenas on %q, got %q", record.Resource.Name, got)
		}
		if hasTag(record.Resource.Tags, "pool") {
			if got := record.Resource.Storage.Topology; got != "pool" {
				t.Fatalf("expected Topology=pool on pool %q, got %q", record.Resource.Name, got)
			}
		}
		if hasTag(record.Resource.Tags, "dataset") {
			if got := record.Resource.Storage.Topology; got != "dataset" {
				t.Fatalf("expected Topology=dataset on dataset %q, got %q", record.Resource.Name, got)
			}
		}
		if got := record.Resource.Storage.Protection; got != "zfs" {
			t.Fatalf("expected Protection=zfs on %q, got %q", record.Resource.Name, got)
		}
	}
}

func TestPoolRecordIncludesCanonicalRiskForDegradedPool(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	provider := NewDefaultProvider()
	records := provider.Records()

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage || record.Resource.Name != "archive" {
			continue
		}
		if record.Resource.Storage == nil || record.Resource.Storage.Risk == nil {
			t.Fatalf("expected risk payload on degraded pool record, got %+v", record.Resource.Storage)
		}
		if record.Resource.Storage.Risk.Level != "warning" {
			t.Fatalf("expected warning risk on degraded pool, got %+v", record.Resource.Storage.Risk)
		}
		if len(record.Resource.Storage.Risk.Reasons) == 0 || record.Resource.Storage.Risk.Reasons[0].Code != "zfs_pool_state" {
			t.Fatalf("expected zfs_pool_state reason, got %+v", record.Resource.Storage.Risk.Reasons)
		}
		return
	}

	t.Fatal("expected degraded archive pool record")
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
