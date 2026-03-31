package mock

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestUnifiedResourceSnapshotIncludesPlatformFixtures(t *testing.T) {
	previous := IsMockEnabled()
	SetEnabled(true)
	t.Cleanup(func() { SetEnabled(previous) })

	graph := CurrentFixtureGraph()
	legacyName := ""
	if len(graph.State.VMs) > 0 {
		legacyName = graph.State.VMs[0].Name
	} else if len(graph.State.Containers) > 0 {
		legacyName = graph.State.Containers[0].Name
	}
	if legacyName == "" {
		t.Fatal("expected canonical mock graph to include at least one legacy resource name")
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected canonical mock graph to include VMware host fixtures")
	}

	resources, freshness := UnifiedResourceSnapshot()
	if len(resources) == 0 {
		t.Fatal("expected unified resources in mock mode")
	}
	if freshness.IsZero() {
		t.Fatal("expected non-zero freshness for mock unified resources")
	}

	wantNames := map[string]bool{
		graph.PlatformFixtures.TrueNAS.System.Hostname: false,
		graph.PlatformFixtures.VMware.Hosts[0].Name:    false,
		legacyName: false,
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
