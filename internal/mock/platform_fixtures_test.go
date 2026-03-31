package mock

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestUnifiedResourceSnapshotIncludesPlatformFixtures(t *testing.T) {
	previous := IsMockEnabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })

	resources, freshness := UnifiedResourceSnapshot()
	if len(resources) == 0 {
		t.Fatal("expected unified resources in mock mode")
	}
	if freshness.IsZero() {
		t.Fatal("expected non-zero freshness for mock unified resources")
	}

	wantNames := map[string]bool{
		"truenas-main":      false,
		"esxi-01.lab.local": false,
		"orders-api-01":     false,
	}
	for _, resource := range resources {
		if _, ok := wantNames[resource.Name]; ok {
			wantNames[resource.Name] = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Fatalf("expected mock unified resources to include %q", name)
		}
	}
}

func TestSupplementalRecordsNormalizesVMwareAlias(t *testing.T) {
	records := SupplementalRecords(unifiedresources.DataSource("vmware-vsphere"))
	if len(records) == 0 {
		t.Fatal("expected records for vmware-vsphere alias")
	}
}
