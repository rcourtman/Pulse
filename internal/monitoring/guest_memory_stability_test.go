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
		MemorySource: "guest-agent-meminfo",
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

func TestStabilizeGuestLowTrustMemoryCarriesForwardAcrossUnavailableReconnect(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)

	now := time.Now()
	prev := &GuestMemorySnapshot{
		Status:       "running",
		RetrievedAt:  now.Add(-30 * time.Second),
		MemorySource: "guest-agent-meminfo",
		Memory: models.Memory{
			Total: 8 * int64(gib),
			Used:  3 * int64(gib),
			Free:  5 * int64(gib),
			Usage: 37.5,
		},
	}

	used, source, notes := stabilizeGuestLowTrustMemory(prev, "running", "unavailable", 8*gib, 0, now, false)
	if used != 3*gib {
		t.Fatalf("used = %d, want %d", used, 3*gib)
	}
	if source != "previous-snapshot" {
		t.Fatalf("source = %q, want previous-snapshot", source)
	}
	if len(notes) != 1 {
		t.Fatalf("notes = %#v, want carry-forward diagnostic", notes)
	}
}

func TestStabilizeGuestLowTrustMemoryUsesHealthyGuestAgentEvidence(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)

	now := time.Now()
	// Stay beyond the two-minute original-source window so this control
	// genuinely exercises the distinct healthy-guest retention path.
	prev := &GuestMemorySnapshot{
		Status:       "running",
		RetrievedAt:  now.Add(-3 * time.Minute),
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total:       8 * int64(gib),
			Used:        3 * int64(gib),
			Usage:       safePercentage(float64(3*gib), float64(8*gib)),
			Observation: models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: now.Add(-3 * time.Minute)},
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

	// Exercise the healthy-guest path after the original-source window,
	// while remaining inside the distinct ten-minute healthy retention window.
	previousSample := time.Now().Add(-3 * time.Minute)
	mon.recordGuestSnapshot("test", "qemu", "node1", 101, GuestMemorySnapshot{
		Name:         "vm-101",
		Status:       "running",
		RetrievedAt:  previousSample,
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total:       8 * int64(gib),
			Used:        3 * int64(gib),
			Free:        5 * int64(gib),
			Usage:       safePercentage(float64(3*gib), float64(8*gib)),
			Observation: models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: previousSample},
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

	// Exercise the healthy-guest path after the original-source window,
	// while remaining inside the distinct ten-minute healthy retention window.
	previousSample := time.Now().Add(-3 * time.Minute)
	mon.recordGuestSnapshot("test", "qemu", "node1", 101, GuestMemorySnapshot{
		Name:         "vm-101",
		Status:       "running",
		RetrievedAt:  previousSample,
		MemorySource: "previous-snapshot",
		Memory: models.Memory{
			Total:       8 * int64(gib),
			Used:        3 * int64(gib),
			Free:        5 * int64(gib),
			Usage:       safePercentage(float64(3*gib), float64(8*gib)),
			Observation: models.MemoryObservation{State: "last-known", Source: "guest-agent-meminfo", ObservedAt: previousSample},
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

// The first malformed refresh must not consume the prior observation's source
// authority. Negative-cache and later failed polls carry the same bounded
// last-known tuple, never a new pressure or History observation.
func TestGuestMemoryRetainedOriginSurvivesRepeatedLowTrustPolls(t *testing.T) {
	original := time.Date(2026, time.October, 13, 5, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, source, state, origin string
		age                         time.Duration
		total                       uint64
		currentSource               string
		want                        bool
	}{
		{"direct", "guest-agent-meminfo", "current", "guest-agent-meminfo", time.Second, 100, "status-mem", true},
		{"retained", "previous-snapshot", "last-known", "guest-agent-meminfo", time.Second, 100, "status-mem", true},
		{"retained-derived", "previous-snapshot", "last-known", "guest-agent-meminfo-derived", time.Minute, 100, "unavailable", true},
		{"inclusive-original-age", "previous-snapshot", "last-known", "guest-agent-meminfo", guestMemoryCarryForwardMaxAge, 100, "status-mem", true},
		{"expired-original", "previous-snapshot", "last-known", "guest-agent-meminfo", guestMemoryCarryForwardMaxAge + time.Nanosecond, 100, "status-mem", false},
		{"future-original", "previous-snapshot", "last-known", "guest-agent-meminfo", -time.Second, 100, "status-mem", false},
		{"legacy-wrapper", "previous-snapshot", "", "", time.Second, 100, "status-mem", false},
		{"unavailable-wrapper", "previous-snapshot", "unavailable", "guest-agent-meminfo", time.Second, 100, "status-mem", false},
		{"unknown-wrapper-state", "previous-snapshot", "future-state", "guest-agent-meminfo", time.Second, 100, "status-mem", false},
		{"unknown-original-source", "previous-snapshot", "last-known", "future-source", time.Second, 100, "status-mem", false},
		{"wrapper-without-origin", "previous-snapshot", "last-known", "previous-snapshot", time.Second, 100, "status-mem", false},
		{"provider-wrapper-is-not-guest-pressure", "previous-snapshot", "last-known", "status-mem", time.Second, 100, "status-mem", false},
		{"provider-label-cannot-borrow-annotation", "status-mem", "last-known", "guest-agent-meminfo", time.Second, 100, "status-mem", false},
		{"resized", "previous-snapshot", "last-known", "guest-agent-meminfo", time.Second, 101, "status-mem", false},
		{"independent-current-memory-wins", "previous-snapshot", "last-known", "guest-agent-meminfo", time.Second, 100, "available-field", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := original.Add(tc.age)
			prev := &GuestMemorySnapshot{Status: "running", MemorySource: tc.source, RetrievedAt: now,
				Memory: models.Memory{Total: 100, Used: 100, Usage: 100, Observation: models.MemoryObservation{State: tc.state, Source: tc.origin, ObservedAt: original}}}
			if tc.state == "" {
				prev.Memory.Observation = models.MemoryObservation{}
			}
			if got := shouldCarryForwardPreviousGuestMemory(prev, "running", tc.currentSource, tc.total, 50, now); got != tc.want {
				t.Errorf("bounded original-source retention=%t, want %t", got, tc.want)
			}
		})
	}
	prev := &GuestMemorySnapshot{Status: "running", MemorySource: "guest-agent-meminfo", RetrievedAt: original,
		Memory: models.Memory{Total: 100, Used: 100, Usage: 100, Observation: models.MemoryObservation{State: "current", Source: "guest-agent-meminfo", ObservedAt: original}}}
	for second := 1; second <= 125; second++ {
		now := original.Add(time.Duration(second) * time.Second)
		used, source, _ := stabilizeGuestLowTrustMemory(prev, "running", "status-mem", 100, 50, now, true)
		if second <= int(guestMemoryCarryForwardMaxAge/time.Second) {
			if used != 100 || source != "previous-snapshot" {
				t.Fatalf("poll %d lost bounded original guest reading: %d/%s", second, used, source)
			}
			prev.MemorySource, prev.Memory.Observation.State = source, "last-known"
			prev.RetrievedAt = now
		} else if used != 50 || source != "status-mem" {
			t.Fatalf("poll %d renewed expired original reading: %d/%s", second, used, source)
		}
		if !prev.Memory.Observation.ObservedAt.Equal(original) {
			t.Fatal("retained poll renewed the original observation time")
		}
	}
}
