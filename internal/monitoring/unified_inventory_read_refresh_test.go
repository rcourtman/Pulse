package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A stale store lets the tests exercise read admission without a wall-clock
// freshness sleep. Registry publication itself remains the real adapter path.
type staleInventoryReadStore struct {
	*unifiedresources.MonitorAdapter
}

func (*staleInventoryReadStore) LastRebuiltAt() time.Time { return time.Now().Add(-time.Hour) }
func (s *staleInventoryReadStore) TryReplaceRegistryForRead(snapshot models.StateSnapshot, _ time.Duration, supply func() map[unifiedresources.DataSource][]unifiedresources.IngestRecord) bool {
	return s.MonitorAdapter.TryReplaceRegistryForRead(snapshot, 0, supply)
}

func TestUnifiedInventoryReadRefreshesWithoutFrontendStateRead(t *testing.T) {
	state := models.NewState()
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	adapter.PopulateFromSnapshot(models.StateSnapshot{})
	m := &Monitor{state: state, resourceStore: &staleInventoryReadStore{adapter}}
	state.UpdateVMs([]models.VM{{ID: "site-node-100", Instance: "site", Node: "node", VMID: 100, Name: "headless-vm", Type: "qemu", Status: "running"}})
	resources, _, _ := m.UnifiedResourceSnapshotWithStaleThresholds()
	if len(resources) != 1 || resources[0].Name != "headless-vm" || resources[0].Type != unifiedresources.ResourceTypeVM {
		t.Fatalf("unattended snapshot did not publish VM inventory: %#v", resources)
	}
}

func TestUnifiedInventoryWarmReadDoesNotWaitForIngestPublication(t *testing.T) {
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	adapter.PopulateFromSnapshot(models.StateSnapshot{})
	m := &Monitor{state: models.NewState(), resourceStore: &staleInventoryReadStore{adapter}}
	m.resourcePublishMu.Lock()
	defer m.resourcePublishMu.Unlock()
	done := make(chan struct{})
	go func() { m.UnifiedResourceSnapshot(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("warm unattended read queued behind an ingest publication")
	}
}

func containerDiskPercent(t *testing.T, rs unifiedresources.ReadState, name string) float64 {
	t.Helper()
	for _, container := range rs.Containers() {
		if container.Name() == name {
			return container.DiskPercent()
		}
	}
	t.Fatalf("read state lists no container %q", name)
	return 0
}

// A Proxmox poll publishes state and then evaluates alerts. The store Patrol
// reads is rebuilt at ingest boundaries and on frontend and REST reads, not
// after that poll. On 14 Nov 2026 a container's disk alert cleared at 3.3%
// while the store still held 92%, and the Patrol run the cleared alert started
// reported the disk still full. A reader's refresh must publish that poll even
// when the store was rebuilt moments before it.
func TestReaderRefreshPublishesThePollItsAlertJudged(t *testing.T) {
	state := models.NewState()
	full := models.Container{
		ID: "pmxlab:pmxlab:111", Instance: "pmxlab", Node: "pmxlab", VMID: 111, Name: "nextcloud",
		Type: "lxc", Status: "running", Disk: models.Disk{Total: 1000, Used: 920, Free: 80, Usage: 92},
	}
	state.UpdateContainers([]models.Container{full})
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{state: state}
	m.SetResourceStore(adapter)
	if got := containerDiskPercent(t, adapter, "nextcloud"); got != 92 {
		t.Fatalf("store after wiring holds %.1f%%, want 92", got)
	}

	freed := full
	freed.Disk = models.Disk{Total: 1000, Used: 33, Free: 967, Usage: 3.3}
	state.UpdateContainers([]models.Container{freed})

	unifiedresources.RefreshForRead(adapter)
	if got := containerDiskPercent(t, adapter, "nextcloud"); got < 3.2 || got > 3.4 {
		t.Fatalf("a reader's refresh left the store at %.1f%%, want the collected 3.3%%", got)
	}
}
