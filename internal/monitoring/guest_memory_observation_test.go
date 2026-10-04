package monitoring

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Use the served JSON rather than the proposed Go type so this control also
// compiles against the parent that loses observation provenance entirely.
func assertGuestMemoryObservation(t *testing.T, memory models.Memory, state, source string, at time.Time) {
	t.Helper()
	data, err := json.Marshal(memory)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Observation struct {
			State      string     `json:"state"`
			Source     string     `json:"source"`
			ObservedAt *time.Time `json:"observedAt"`
		} `json:"observation"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	got := wire.Observation
	if got.State != state || got.Source != source {
		t.Errorf("memory observation = %s, want %s / %s", data, state, source)
	}
	if at.IsZero() {
		if got.ObservedAt != nil {
			t.Errorf("unobserved memory acquired a timestamp: %s", data)
		}
	} else if got.ObservedAt == nil || !got.ObservedAt.Equal(at) {
		t.Errorf("memory observation time = %v, want original %v", got.ObservedAt, at)
	}
}

func testGuestMemoryObservationLifecycle(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	identity := makeGuestID("provenance", "node", 105)
	registry := unifiedresources.NewRegistry(nil)
	m := &Monitor{rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), resourceStore: unifiedresources.NewMonitorAdapter(registry)}
	client := &vmMemoryTrustStubClient{stubPVEClient: &stubPVEClient{}, vmAgentMemAvailable: 5 * gib,
		vmStatus: &proxmox.VMStatus{Status: "running", MaxMem: 8 * gib, Mem: 8 * gib, Agent: proxmox.VMAgentField{Value: 1}}}
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * gib, Mem: 8 * gib}
	var previous *models.VM
	build := func() models.VM {
		t.Helper()
		vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "provenance", res, client, identity, nil, previous)
		if !ok {
			t.Fatal("guest disappeared")
		}
		m.recordGuestSnapshot("provenance", "qemu", "node", 105, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
		m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
		registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		views := registry.VMs()
		if len(views) != 1 {
			t.Fatal("guest did not reach unified view")
		}
		next := previousVMFromView(views[0])
		previous = &next
		return vm
	}
	assertProjection := func(vm models.VM, state, source string, at time.Time) {
		t.Helper()
		assertGuestMemoryObservation(t, vm.Memory, state, source, at)
		legacy := vm.ToFrontend()
		if legacy.Memory == nil {
			t.Fatal("legacy projection lost memory")
		}
		assertGuestMemoryObservation(t, *legacy.Memory, state, source, at)
		// REST and WebSocket state use this production conversion, including
		// registry cloning and the nested Proxmox payload the drawer consumes.
		front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		data, err := json.Marshal(front.Resources)
		if err != nil {
			t.Fatal(err)
		}
		var wire []struct {
			Proxmox struct {
				Memory models.Memory `json:"memory"`
			} `json:"proxmox"`
		}
		if err := json.Unmarshal(data, &wire); err != nil || len(wire) != 1 {
			t.Fatalf("broadcast guest memory unavailable: %s / %v", data, err)
		}
		assertGuestMemoryObservation(t, wire[0].Proxmox.Memory, state, source, at)
		if wire[0].Proxmox.Memory.Used != vm.Memory.Used || wire[0].Proxmox.Memory.UsageUnavailable != vm.Memory.UsageUnavailable {
			t.Fatal("provenance changed numeric/unknown memory projection")
		}
	}
	initial := build()
	key := guestMemoryCacheKey("provenance", "node", 105)
	original := m.vmAgentMemCache[key].fetchedAt
	if original.IsZero() || initial.Memory.Used != int64(3*gib) {
		t.Fatal("successful QGA observation missing")
	}
	assertProjection(initial, "current", "guest-agent-meminfo", original)
	cached := build()
	assertProjection(cached, "current", "guest-agent-meminfo", original)
	if client.vmAgentMemCalls != 1 {
		t.Fatal("normal cache caused a new guest read")
	}
	res.Lock, client.vmStatus.Lock = "backup", "backup"
	for poll := 0; poll < 4; poll++ {
		locked := build()
		assertProjection(locked, "last-known", "guest-agent-meminfo", original)
		if locked.Memory.Used != initial.Memory.Used || m.vmAgentMemCache[key].fetchedAt != original || client.vmAgentMemCalls != 1 {
			t.Fatal("deferral changed evidence, its age, or guest read admission")
		}
	}
	if got := len(m.metricsHistory.GetGuestMetrics(identity, "memory", time.Hour)); got != 2 {
		t.Errorf("deferred memory became fresh History points: %d, want two normal polls", got)
	}
	entry := m.vmAgentMemCache[key]
	entry.fetchedAt = time.Now().Add(-vmAgentMemCleanupMaxAge - time.Second)
	m.vmAgentMemCache[key] = entry
	expired := build()
	assertProjection(expired, "unavailable", "unavailable", time.Time{})
	if expired.Memory.HasKnownUsage() || client.vmAgentMemCalls != 1 {
		t.Fatal("expired evidence became usage or queued a guest read")
	}
	res.Lock, client.vmStatus.Lock = "", ""
	client.vmAgentMemAvailable = 4 * gib
	resumed := build()
	resumedAt := m.vmAgentMemCache[key].fetchedAt
	assertProjection(resumed, "current", "guest-agent-meminfo", resumedAt)
	if !resumedAt.After(original) || resumed.Memory.Used != int64(4*gib) || client.vmAgentMemCalls != 2 {
		t.Fatal("resumption did not replace original guest observation")
	}
	// Backup/disk deferral must not disqualify an independently current PVE
	// memory reading. Its timestamp belongs to that source, not this poll.
	statusAt := time.Now().Add(-time.Second)
	res.Lock, client.vmStatus.Lock = "backup", "backup"
	client.vmStatus.ObservedAt = statusAt
	client.vmStatus.MemInfo = &proxmox.VMMemInfo{Total: 8 * gib, Available: 6 * gib}
	live := build()
	assertProjection(live, "current", "available-field", statusAt)
	if live.Memory.Used != int64(2*gib) || live.GuestAgentStatus != "deferred" || client.vmAgentMemCalls != 2 {
		t.Fatal("independent PVE memory was replaced by deferred QGA evidence")
	}
	res.Status = "stopped"
	stopped := build()
	assertProjection(stopped, "unavailable", "powered-off", time.Time{})
	if stopped.Memory.Used != 0 {
		t.Fatal("powered-off numeric compatibility changed")
	}
}
