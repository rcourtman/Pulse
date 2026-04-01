package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestStabilizeGuestLowTrustMemoryCarriesForwardTrustedSnapshot(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)

	now := time.Now()
	prev := &GuestMemorySnapshot{
		Status:       "running",
		RetrievedAt:  now.Add(-time.Minute),
		MemorySource: "rrd-memavailable",
		Memory: models.Memory{
			Total: 8 * int64(gib),
			Used:  3 * int64(gib),
			Usage: safePercentage(float64(3*gib), float64(8*gib)),
		},
	}

	used, source, notes := stabilizeGuestLowTrustMemory(prev, "running", "status-freemem", 8*gib, 8*gib, now, false)
	if used != 3*gib {
		t.Fatalf("used = %d, want %d", used, 3*gib)
	}
	if source != "previous-snapshot" {
		t.Fatalf("source = %q, want previous-snapshot", source)
	}
	if len(notes) != 1 || notes[0] != "preserved-previous-memory-after-repeated-low-trust-pattern" {
		t.Fatalf("notes = %#v, want repeated low-trust carry-forward note", notes)
	}
}

func TestStabilizeGuestLowTrustMemoryUsesHealthyGuestAgentEvidence(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)

	now := time.Now()
	prev := &GuestMemorySnapshot{
		Status:       "running",
		RetrievedAt:  now.Add(-2 * time.Minute),
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total: 8 * int64(gib),
			Used:  3 * int64(gib),
			Usage: safePercentage(float64(3*gib), float64(8*gib)),
		},
	}

	used, source, notes := stabilizeGuestLowTrustMemory(prev, "running", "cluster-resources", 8*gib, 8*gib, now, true)
	if used != 3*gib {
		t.Fatalf("used = %d, want %d", used, 3*gib)
	}
	if source != "previous-snapshot" {
		t.Fatalf("source = %q, want previous-snapshot", source)
	}
	if len(notes) != 1 || notes[0] != "preserved-previous-memory-for-healthy-guest-low-trust-full-usage" {
		t.Fatalf("notes = %#v, want healthy guest carry-forward note", notes)
	}
}

func TestHandleClusterVMResourcePreservesHealthyGuestMemoryFromPreviousSnapshot(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	previousSample := time.Now().Add(-time.Minute)
	mon.recordGuestSnapshot("test", "qemu", "node1", 101, GuestMemorySnapshot{
		Name:         "vm-101",
		Status:       "running",
		RetrievedAt:  previousSample,
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total: 8 * int64(gib),
			Used:  3 * int64(gib),
			Free:  5 * int64(gib),
			Usage: safePercentage(float64(3*gib), float64(8*gib)),
		},
	})

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			MaxMem: 8 * gib,
			Mem:    8 * gib,
			Agent:  proxmox.VMAgentField{Value: 1},
		},
	}

	vm, ok := mon.handleClusterVMResource(
		context.Background(),
		"test",
		proxmox.ClusterResource{
			ID:     "qemu/101",
			Type:   "qemu",
			Node:   "node1",
			Name:   "vm-101",
			Status: "running",
			VMID:   101,
			MaxMem: 8 * gib,
			Mem:    8 * gib,
			MaxCPU: 2,
		},
		makeGuestID("test", "node1", 101),
		client,
		nil,
		nil,
	)
	if !ok {
		t.Fatal("handleClusterVMResource() returned ok=false")
	}
	if got := uint64(vm.Memory.Used); got != 3*gib {
		t.Fatalf("vm.Memory.Used = %d, want %d", got, 3*gib)
	}

	key := makeGuestSnapshotKey("test", "qemu", "node1", 101)
	snap, ok := mon.guestSnapshots[key]
	if !ok {
		t.Fatalf("expected guest snapshot %q to be recorded", key)
	}
	if snap.MemorySource != "previous-snapshot" {
		t.Fatalf("snapshot.MemorySource = %q, want previous-snapshot", snap.MemorySource)
	}
	if len(snap.Notes) != 1 || snap.Notes[0] != "preserved-previous-memory-for-healthy-guest-low-trust-full-usage" {
		t.Fatalf("snapshot.Notes = %#v, want healthy guest carry-forward note", snap.Notes)
	}
}

func TestPollVMsWithNodesPreservesHealthyGuestMemoryFromPreviousSnapshot(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	const gib = uint64(1024 * 1024 * 1024)

	mon := newTestPVEMonitor("test")
	defer mon.alertManager.Stop()
	defer mon.notificationMgr.Stop()

	previousSample := time.Now().Add(-time.Minute)
	mon.recordGuestSnapshot("test", "qemu", "node1", 101, GuestMemorySnapshot{
		Name:         "vm-101",
		Status:       "running",
		RetrievedAt:  previousSample,
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total: 8 * int64(gib),
			Used:  3 * int64(gib),
			Free:  5 * int64(gib),
			Usage: safePercentage(float64(3*gib), float64(8*gib)),
		},
	})

	client := &vmMemoryTrustStubClient{
		stubPVEClient: &stubPVEClient{},
		vms: []proxmox.VM{{
			VMID:   101,
			Name:   "vm-101",
			Node:   "node1",
			Status: "running",
			MaxMem: 8 * gib,
			Mem:    8 * gib,
			CPUs:   2,
		}},
		vmStatus: &proxmox.VMStatus{
			Status: "running",
			MaxMem: 8 * gib,
			Mem:    8 * gib,
			Agent:  proxmox.VMAgentField{Value: 1},
		},
	}

	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	mon.pollVMsWithNodes(context.Background(), "test", "", false, client, nodes, nodeEffectiveStatus)

	vms := mon.state.GetSnapshot().VMs
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	if got := uint64(vms[0].Memory.Used); got != 3*gib {
		t.Fatalf("vms[0].Memory.Used = %d, want %d", got, 3*gib)
	}

	key := makeGuestSnapshotKey("test", "qemu", "node1", 101)
	snap, ok := mon.guestSnapshots[key]
	if !ok {
		t.Fatalf("expected guest snapshot %q to be recorded", key)
	}
	if snap.MemorySource != "previous-snapshot" {
		t.Fatalf("snapshot.MemorySource = %q, want previous-snapshot", snap.MemorySource)
	}
	if len(snap.Notes) != 1 || snap.Notes[0] != "preserved-previous-memory-for-healthy-guest-low-trust-full-usage" {
		t.Fatalf("snapshot.Notes = %#v, want healthy guest carry-forward note", snap.Notes)
	}
}
