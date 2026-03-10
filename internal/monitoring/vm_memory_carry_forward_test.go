package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestPollVMsAndContainersEfficient_PreservesPreviousTrustedVMMemoryForOneCycle(t *testing.T) {
	const total = uint64(8 << 30)
	const trustedUsed = uint64(2 << 30)
	const fallbackUsed = total

	m := &Monitor{
		state:                    models.NewState(),
		guestAgentFSInfoTimeout:  time.Second,
		guestAgentRetries:        1,
		guestAgentNetworkTimeout: time.Second,
		guestAgentOSInfoTimeout:  time.Second,
		guestAgentVersionTimeout: time.Second,
		guestMetadataCache:       make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter:     make(map[string]time.Time),
		rateTracker:              NewRateTracker(),
		metricsHistory:           NewMetricsHistory(100, time.Hour),
		alertManager:             alerts.NewManager(),
		stalenessTracker:         NewStalenessTracker(nil),
		nodeRRDMemCache:          make(map[string]rrdMemCacheEntry),
		vmRRDMemCache:            make(map[string]rrdMemCacheEntry),
		vmAgentMemCache:          make(map[string]agentMemCacheEntry),
	}
	defer m.alertManager.Stop()

	m.state.UpdateVMsForInstance("pve1", []models.VM{
		{
			ID:           "pve1:node1:100",
			Instance:     "pve1",
			Node:         "node1",
			VMID:         100,
			Name:         "vm100",
			Type:         "qemu",
			Status:       "running",
			MemorySource: "rrd-memavailable",
			Memory: models.Memory{
				Total: int64(total),
				Used:  int64(trustedUsed),
				Free:  int64(total - trustedUsed),
				Usage: safePercentage(float64(trustedUsed), float64(total)),
			},
			LastSeen: time.Now(),
		},
	})

	client := &mockPVEClientExtra{
		resources: []proxmox.ClusterResource{
			{Type: "qemu", VMID: 100, Name: "vm100", Node: "node1", Status: "running", MaxMem: total, Mem: fallbackUsed},
		},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			Agent:  proxmox.VMAgentField{Value: 0},
			MaxMem: total,
			Mem:    fallbackUsed,
		},
	}

	success := m.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"})
	if !success {
		t.Fatal("pollVMsAndContainersEfficient failed")
	}

	state := m.GetState()
	if len(state.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(state.VMs))
	}
	vm := state.VMs[0]
	if vm.MemorySource != "previous-snapshot" {
		t.Fatalf("memory source = %q, want previous-snapshot", vm.MemorySource)
	}
	if vm.Memory.Used != int64(trustedUsed) {
		t.Fatalf("memory used = %d, want preserved %d", vm.Memory.Used, trustedUsed)
	}

	snapshotKey := makeGuestSnapshotKey("pve1", "qemu", "node1", 100)
	m.diagMu.RLock()
	snapshot, ok := m.guestSnapshots[snapshotKey]
	m.diagMu.RUnlock()
	if !ok {
		t.Fatal("expected guest snapshot entry")
	}
	if len(snapshot.Notes) == 0 || snapshot.Notes[0] != "preserved-previous-memory-after-low-trust-fallback" {
		t.Fatalf("expected preservation note, got %#v", snapshot.Notes)
	}

	success = m.pollVMsAndContainersEfficient(context.Background(), "pve1", "", false, client, map[string]string{"node1": "online"})
	if !success {
		t.Fatal("second pollVMsAndContainersEfficient failed")
	}

	state = m.GetState()
	vm = state.VMs[0]
	if vm.MemorySource != "status-mem" {
		t.Fatalf("second poll memory source = %q, want status-mem", vm.MemorySource)
	}
	if vm.Memory.Used != int64(fallbackUsed) {
		t.Fatalf("second poll memory used = %d, want fallback %d", vm.Memory.Used, fallbackUsed)
	}
}
