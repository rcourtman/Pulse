package monitoring

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Alert evaluation resolves a metrics target once per resource per poll, so a
// read-state lookup that copies the registry makes every poll quadratic in
// estate size (#2199). These tests pin the lookup to the live store.

func newReadStateCloneTestMonitor(t testing.TB, vms int) (*Monitor, *unifiedresources.MonitorAdapter, string) {
	t.Helper()
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{LastUpdate: now}
	for i := 0; i < vms; i++ {
		snapshot.VMs = append(snapshot.VMs, models.VM{
			ID:       fmt.Sprintf("lab:pve-%d:%d", i%8, 100+i),
			VMID:     100 + i,
			Name:     fmt.Sprintf("vm-%d", i),
			Node:     fmt.Sprintf("pve-%d", i%8),
			Instance: "lab",
			Status:   "running",
			Type:     "qemu",
			CPU:      0.1,
			Memory:   models.Memory{Total: 4 << 30, Used: 1 << 30},
			LastSeen: now,
		})
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	adapter := unifiedresources.NewMonitorAdapter(registry)
	m := &Monitor{state: models.NewState(), resourceStore: adapter}

	vmID := ""
	for _, resource := range adapter.GetAll() {
		if resource.Type == unifiedresources.ResourceTypeVM {
			vmID = resource.ID
			break
		}
	}
	if vmID == "" {
		t.Fatal("fixture produced no VM resource")
	}
	return m, adapter, vmID
}

func TestGetUnifiedReadStateOrSnapshotReturnsLiveStore(t *testing.T) {
	m, adapter, _ := newReadStateCloneTestMonitor(t, 4)

	readState := m.GetUnifiedReadStateOrSnapshot()
	got, ok := readState.(*unifiedresources.MonitorAdapter)
	if !ok || got != adapter {
		t.Fatalf("read state = %T %p, want the configured store %p", readState, got, adapter)
	}
}

func TestMetricsTargetForResourceDoesNotCopyRegistry(t *testing.T) {
	small, _, smallID := newReadStateCloneTestMonitor(t, 8)
	large, _, largeID := newReadStateCloneTestMonitor(t, 1000)
	if small.MetricsTargetForResource(smallID) == nil || large.MetricsTargetForResource(largeID) == nil {
		t.Fatal("expected metrics targets for fixture VMs")
	}

	smallAllocs := testing.AllocsPerRun(20, func() { _ = small.MetricsTargetForResource(smallID) })
	largeAllocs := testing.AllocsPerRun(20, func() { _ = large.MetricsTargetForResource(largeID) })
	// A registry copy costs several allocations per resource, so any growth
	// from 8 to 1,000 resources means the lookup is copying the estate again.
	if largeAllocs > smallAllocs+8 {
		t.Fatalf("MetricsTargetForResource allocations grew with estate size: %.0f at 8 VMs, %.0f at 1000 VMs", smallAllocs, largeAllocs)
	}
}
