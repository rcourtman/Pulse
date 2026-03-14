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

func TestShouldCarryForwardPreviousVMMemory_WhenTrustedGuestAgentMeminfoDropsToStatusMem(t *testing.T) {
	const total = uint64(16 << 30)
	const trustedUsed = uint64(4 << 30)
	const fallbackUsed = uint64(15 << 30)

	prev := models.VM{
		Type:         "qemu",
		Status:       "running",
		MemorySource: "guest-agent-meminfo",
		Memory: models.Memory{
			Total: int64(total),
			Used:  int64(trustedUsed),
			Free:  int64(total - trustedUsed),
			Usage: safePercentage(float64(trustedUsed), float64(total)),
		},
		LastSeen: time.Now(),
	}

	if !shouldCarryForwardPreviousVMMemory(prev, "running", "status-mem", total, fallbackUsed, time.Now()) {
		t.Fatal("expected guest-agent meminfo reading to be preserved for one cycle when VM falls back to low-trust status-mem")
	}
}

func TestShouldCarryForwardPreviousVMMemory(t *testing.T) {
	now := time.Now()
	const total = uint64(16 << 30)

	makePrevVM := func(source string, used uint64, lastSeen time.Time) models.VM {
		return models.VM{
			Type:         "qemu",
			Status:       "running",
			MemorySource: source,
			Memory: models.Memory{
				Total: int64(total),
				Used:  int64(used),
				Free:  int64(total - used),
				Usage: safePercentage(float64(used), float64(total)),
			},
			LastSeen: lastSeen,
		}
	}

	tests := []struct {
		name          string
		prev          models.VM
		currentStatus string
		currentSource string
		currentTotal  uint64
		currentUsed   uint64
		want          bool
	}{
		{
			name:          "preserves trusted rrd memavailable when current poll falls back to status mem",
			prev:          makePrevVM("rrd-memavailable", 4<<30, now),
			currentStatus: "running",
			currentSource: "status-mem",
			currentTotal:  total,
			currentUsed:   15 << 30,
			want:          true,
		},
		{
			name:          "does not preserve when both previous and current sources are low trust",
			prev:          makePrevVM("status-mem", 12<<30, now),
			currentStatus: "running",
			currentSource: "status-mem",
			currentTotal:  total,
			currentUsed:   15 << 30,
			want:          false,
		},
		{
			name:          "does not preserve when VM total changed",
			prev:          makePrevVM("guest-agent-meminfo", 4<<30, now),
			currentStatus: "running",
			currentSource: "status-mem",
			currentTotal:  32 << 30,
			currentUsed:   28 << 30,
			want:          false,
		},
		{
			name:          "does not preserve when previous sample is stale",
			prev:          makePrevVM("guest-agent-meminfo", 4<<30, now.Add(-vmMemoryCarryForwardMaxAge-time.Second)),
			currentStatus: "running",
			currentSource: "status-mem",
			currentTotal:  total,
			currentUsed:   15 << 30,
			want:          false,
		},
		{
			name:          "does not preserve when VM is no longer running",
			prev:          makePrevVM("guest-agent-meminfo", 4<<30, now),
			currentStatus: "stopped",
			currentSource: "status-mem",
			currentTotal:  total,
			currentUsed:   0,
			want:          false,
		},
		{
			name:          "does not preserve when usage delta is too small",
			prev:          makePrevVM("guest-agent-meminfo", 4<<30, now),
			currentStatus: "running",
			currentSource: "status-mem",
			currentTotal:  total,
			currentUsed:   4476033511, // ~26.05% vs previous 25%, below the 5-point threshold
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCarryForwardPreviousVMMemory(tt.prev, tt.currentStatus, tt.currentSource, tt.currentTotal, tt.currentUsed, now)
			if got != tt.want {
				t.Fatalf("shouldCarryForwardPreviousVMMemory() = %v, want %v", got, tt.want)
			}
		})
	}
}
