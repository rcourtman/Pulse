package truenas

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestRecordsParentPoolFallsBackToDatasetName(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	provider := NewProvider(FixtureSnapshot{
		CollectedAt: time.Unix(1707400000, 0).UTC(),
		System: SystemInfo{
			Hostname: "truenas-main",
			Healthy:  true,
		},
		Pools: []Pool{{
			ID:         "1",
			Name:       "tank",
			Status:     "ONLINE",
			TotalBytes: 1000,
			UsedBytes:  300,
			FreeBytes:  700,
		}},
		Datasets: []Dataset{{
			ID:         "tank/apps",
			Name:       "tank/apps",
			Pool:       "",
			UsedBytes:  120,
			AvailBytes: 80,
			Mounted:    true,
			ReadOnly:   false,
		}},
	})

	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected records")
	}

	for _, record := range records {
		if record.Resource.Type != unifiedresources.ResourceTypeStorage {
			continue
		}
		if record.Resource.Name != "tank/apps" {
			continue
		}
		if record.ParentSourceID != "pool:tank" {
			t.Fatalf("expected dataset parent source id pool:tank, got %q", record.ParentSourceID)
		}
		return
	}

	t.Fatal("dataset record tank/apps not found")
}

func TestStatusFromSystemAndParseBoolBranches(t *testing.T) {
	if got := statusFromSystem(SystemInfo{Healthy: true}); got != unifiedresources.StatusOnline {
		t.Fatalf("statusFromSystem(healthy) = %q, want %q", got, unifiedresources.StatusOnline)
	}
	if got := statusFromSystem(SystemInfo{Healthy: false}); got != unifiedresources.StatusWarning {
		t.Fatalf("statusFromSystem(unhealthy) = %q, want %q", got, unifiedresources.StatusWarning)
	}

	if !parseBool("  YeS ") {
		t.Fatal("expected parseBool to treat yes as true")
	}
	if parseBool("off") {
		t.Fatal("expected parseBool to treat off as false")
	}
}
