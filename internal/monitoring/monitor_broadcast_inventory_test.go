package monitoring

import (
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func broadcastInventoryFixture(vmCount int) *Monitor {
	state := models.NewState()
	state.Nodes = []models.Node{{ID: "node-a"}}
	state.VMs = make([]models.VM, vmCount)
	for i := range state.VMs {
		state.VMs[i] = models.VM{
			ID:          fmt.Sprintf("vm-%d", i),
			IPAddresses: []string{"192.0.2.10"},
			Tags:        []string{"fixture", "backup"},
			Disks:       []models.Disk{{Mountpoint: "/", Total: 100, Used: 40}},
		}
	}
	state.Containers = make([]models.Container, 3)
	state.Hosts = []models.Host{
		{ID: "vm-agent", LinkedVMID: "vm-0", Disks: []models.Disk{{Mountpoint: "/data", Total: 200, Used: 60}}},
		{ID: "standalone-agent"},
	}
	state.PBSInstances = make([]models.PBSInstance, 4)
	state.PBSBackups = make([]models.PBSBackup, 5)
	for i := range state.PBSBackups {
		state.PBSBackups[i] = models.PBSBackup{ID: fmt.Sprintf("backup-%d", i), Files: []string{"index.json.blob"}}
	}
	state.PhysicalDisks = make([]models.PhysicalDisk, 6)
	return &Monitor{state: state}
}

// The parent's debug path projected these lengths from GetState, including
// linked host disk enrichment and the mock fixture selection.
func fullBroadcastInventoryCounts(m *Monitor) models.StateInventoryCounts {
	state := m.GetState()
	return models.StateInventoryCounts{
		Nodes:         len(state.Nodes),
		VMs:           len(state.VMs),
		Containers:    len(state.Containers),
		Hosts:         len(state.Hosts),
		PBSInstances:  len(state.PBSInstances),
		PBSBackups:    len(state.Backups.PBS),
		PhysicalDisks: len(state.PhysicalDisks),
	}
}

func TestBroadcastInventoryCountsMatchFullState(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)

	m := broadcastInventoryFixture(24)
	for _, tt := range []struct {
		name    string
		monitor *Monitor
	}{{"uninitialized", &Monitor{}}, {"empty", &Monitor{state: models.NewState()}}, {"mixed", m}} {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := tt.monitor.broadcastInventoryCounts(), fullBroadcastInventoryCounts(tt.monitor); got != want {
				t.Fatalf("counts = %+v; want %+v", got, want)
			}
		})
	}
	liveCounts := m.broadcastInventoryCounts()
	synctest.Test(t, func(t *testing.T) {
		mustSetMockEnabled(t, true)
		defer mustSetMockEnabled(t, false)

		want := fullBroadcastInventoryCounts(m)
		if want == liveCounts || want == (models.StateInventoryCounts{}) {
			t.Fatal("mock fixture must be distinguishable from the live and empty inventories")
		}
		if got := m.broadcastInventoryCounts(); got != want {
			t.Fatalf("mock counts = %+v; want %+v", got, want)
		}
	})
}

func TestBroadcastInventoryCountsDuringWritesAndReset(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)

	m := broadcastInventoryFixture(24)
	live := m.state
	full := m.broadcastInventoryCounts()
	one := full
	one.VMs = 1
	empty := models.StateInventoryCounts{}

	start, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		<-start
		vms := live.GetSnapshot().VMs
		for range 100 {
			live.UpdateVMs(vms[:1])
			live.UpdateVMs(vms)
			m.mu.Lock()
			m.resetStateLocked()
			m.mu.Unlock()
			m.mu.Lock()
			m.state = live
			m.mu.Unlock()
		}
	}()
	close(start)
	for range 1000 {
		got := m.broadcastInventoryCounts()
		if got != full && got != one && got != empty {
			t.Errorf("read mixed inventory states: %+v", got)
			break
		}
	}
	<-done
	m.mu.Lock()
	m.resetStateLocked()
	m.mu.Unlock()
	if got := m.broadcastInventoryCounts(); got != empty {
		t.Fatalf("counts retained the old inventory after reset: %+v", got)
	}
}

var broadcastInventoryCountsSink models.StateInventoryCounts

func TestBroadcastInventoryCountsAvoidInventoryCopies(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)

	m := broadcastInventoryFixture(1000)
	legacy := testing.AllocsPerRun(10, func() { broadcastInventoryCountsSink = fullBroadcastInventoryCounts(m) })
	current := testing.AllocsPerRun(10, func() { broadcastInventoryCountsSink = m.broadcastInventoryCounts() })
	t.Logf("live diagnostic with 1000 VMs: full=%.0f count-only=%.0f allocs/read", legacy, current)
	if legacy == 0 || current != 0 {
		t.Fatalf("expected observable full-state copies and no count-only allocations")
	}
}

func BenchmarkBroadcastInventoryCounts(b *testing.B) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(b, false)
	b.Cleanup(func() { mustSetMockEnabled(b, previous) })

	for _, size := range []int{24, 1000} {
		m := broadcastInventoryFixture(size)
		for _, path := range []struct {
			name string
			read func(*Monitor) models.StateInventoryCounts
		}{{"full", fullBroadcastInventoryCounts}, {"counts", (*Monitor).broadcastInventoryCounts}} {
			b.Run(fmt.Sprintf("%d/%s", size, path.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					broadcastInventoryCountsSink = path.read(m)
				}
			})
		}
	}
}
