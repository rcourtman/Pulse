package mock

import (
	"fmt"
	"math"
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

func TestBuildFixtureGraphRefreshesPlatformFixtureMetricsFromCanonicalModel(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 45, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	system := graph.PlatformFixtures.TrueNAS.System
	if got, want := system.CPUPercent, SampleMetric("agent", system.Hostname, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed TrueNAS system cpu %.6f, got %.6f", want, got)
	}

	app := graph.PlatformFixtures.TrueNAS.Apps[1]
	if app.Stats == nil {
		t.Fatal("expected refreshed TrueNAS app stats")
	}
	if got, want := app.Stats.CPUPercent, SampleMetric("dockerContainer", app.ID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed TrueNAS app cpu %.6f, got %.6f", want, got)
	}
	if got, want := app.Stats.MemoryBytes, bytesFromPercent(system.MemoryTotalBytes, SampleMetric("dockerContainer", app.ID, "memory", now)); got != want {
		t.Fatalf("expected refreshed TrueNAS app memory bytes %d, got %d", want, got)
	}

	disk := graph.PlatformFixtures.TrueNAS.Disks[0]
	if got, want := disk.Temperature, int(math.Round(SampleMetric("disk", disk.Serial, "smart_temp", now))); got != want {
		t.Fatalf("expected refreshed TrueNAS disk temperature %d, got %d", want, got)
	}

	host := graph.PlatformFixtures.VMware.Hosts[0]
	if host.Metrics == nil || host.Metrics.CPUPercent == nil {
		t.Fatal("expected refreshed VMware host metrics")
	}
	if got, want := *host.Metrics.CPUPercent, SampleMetric("agent", "vc-mock-1:host:host-101", "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed VMware host cpu %.6f, got %.6f", want, got)
	}

	datastore := graph.PlatformFixtures.VMware.Datastores[0]
	wantFree := datastore.Capacity - bytesFromPercent(datastore.Capacity, SampleMetric("storage", "vc-mock-1:datastore:"+datastore.Datastore, "usage", now))
	if datastore.FreeSpace != wantFree {
		t.Fatalf("expected refreshed VMware datastore free space %d, got %d", wantFree, datastore.FreeSpace)
	}
}

func TestBuildFixtureGraphRefreshesStateMetricsFromCanonicalModel(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 45, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	node := graph.State.Nodes[0]
	if got, want := node.CPU*100, SampleMetric("node", node.ID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed node cpu %.6f, got %.6f", want, got)
	}
	if got, want := node.Memory.Usage, SampleMetric("node", node.ID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed node memory %.6f, got %.6f", want, got)
	}

	var runningVMID string
	var runningVMCPU float64
	var runningVMMemory float64
	for _, vm := range graph.State.VMs {
		if vm.Status != "running" {
			continue
		}
		runningVMID = vm.ID
		runningVMCPU = vm.CPU * 100
		runningVMMemory = vm.Memory.Usage
		break
	}
	if runningVMID == "" {
		t.Fatal("expected at least one running VM in canonical fixture graph")
	}
	if got, want := runningVMCPU, SampleMetric("vm", runningVMID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed vm cpu %.6f, got %.6f", want, got)
	}
	if got, want := runningVMMemory, SampleMetric("vm", runningVMID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed vm memory %.6f, got %.6f", want, got)
	}

	var onlineHostID string
	var onlineHostCPU float64
	var onlineHostMemory float64
	for _, host := range graph.State.Hosts {
		if host.Status == "offline" {
			continue
		}
		onlineHostID = host.ID
		onlineHostCPU = host.CPUUsage
		onlineHostMemory = host.Memory.Usage
		break
	}
	if onlineHostID == "" {
		t.Fatal("expected at least one online host in canonical fixture graph")
	}
	if got, want := onlineHostCPU, SampleMetric("agent", onlineHostID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed host cpu %.6f, got %.6f", want, got)
	}
	if got, want := onlineHostMemory, SampleMetric("agent", onlineHostID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed host memory %.6f, got %.6f", want, got)
	}

	storage := graph.State.Storage[0]
	if got, want := storage.Usage, SampleMetric("storage", storage.ID, "usage", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed storage usage %.6f, got %.6f", want, got)
	}

	disk := graph.State.PhysicalDisks[0]
	resourceID := disk.Serial
	if resourceID == "" {
		resourceID = disk.ID
	}
	if resourceID == "" {
		resourceID = fmt.Sprintf("%s-%s-%s", disk.Instance, disk.Node, disk.DevPath)
	}
	if got, want := disk.Temperature, int(math.Round(SampleMetric("disk", resourceID, "smart_temp", now))); got != want {
		t.Fatalf("expected refreshed physical disk temperature %d, got %d", want, got)
	}
}
