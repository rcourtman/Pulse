package mock

import (
	"testing"
	"time"

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

func TestBuildFixtureGraphRebasesPlatformFixtureTimestampsForDemoRuntime(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 0, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(now) {
		t.Fatalf("expected TrueNAS collectedAt %s, got %s", now, got)
	}
	if got := graph.PlatformFixtures.VMware.CollectedAt; !got.Equal(now) {
		t.Fatalf("expected VMware collectedAt %s, got %s", now, got)
	}
	if got := graph.PlatformFixtures.TrueNAS.System.CollectedAt; got.IsZero() || got.Before(now.Add(-2*time.Minute)) || got.After(now) {
		t.Fatalf("expected rebased TrueNAS system collectedAt near %s, got %s", now, got)
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 || len(graph.PlatformFixtures.VMware.Hosts[0].RecentEvents) == 0 {
		t.Fatal("expected canonical VMware fixtures with recent events")
	}
	if got := graph.PlatformFixtures.VMware.Hosts[0].RecentEvents[0].CreatedAt; got.IsZero() || got.Before(now.Add(-2*time.Hour)) || got.After(now) {
		t.Fatalf("expected rebased VMware event timestamp near %s, got %s", now, got)
	}
}

func TestFixtureGraphUpdateMetricsKeepsPlatformFixtureFreshnessCurrent(t *testing.T) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false

	start := time.Date(2026, time.March, 31, 17, 30, 0, 0, time.UTC)
	later := start.Add(12 * time.Minute)

	graph := buildFixtureGraph(cfg, start)
	graph.UpdateMetrics(cfg, later)

	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(later) {
		t.Fatalf("expected rebased TrueNAS collectedAt %s, got %s", later, got)
	}
	if got := graph.PlatformFixtures.VMware.CollectedAt; !got.Equal(later) {
		t.Fatalf("expected rebased VMware collectedAt %s, got %s", later, got)
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 || len(graph.PlatformFixtures.VMware.Hosts[0].RecentEvents) == 0 {
		t.Fatal("expected canonical VMware fixtures with host events")
	}
	if got := graph.PlatformFixtures.VMware.Hosts[0].RecentEvents[0].CreatedAt; got.Before(later.Add(-2*time.Hour)) || got.After(later) {
		t.Fatalf("expected VMware event timestamp to remain fresh near %s, got %s", later, got)
	}
}
