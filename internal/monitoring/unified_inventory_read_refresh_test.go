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
