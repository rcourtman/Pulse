package monitoring

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func guestHistoryObservationMonitor(t *testing.T) *Monitor {
	t.Helper()
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.WriteBufferSize = 512
	cfg.FlushInterval = time.Hour
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.WaitForMaintenance(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	return &Monitor{metricsHistory: NewMetricsHistory(128, 24*time.Hour), metricsStore: store}
}

func guestHistoryStoredPoints(t *testing.T, m *Monitor, kind, id, metric string) []metrics.MetricPoint {
	t.Helper()
	m.metricsStore.Flush()
	now := time.Now()
	points, err := m.metricsStore.Query(kind, id, metric, now.Add(-time.Minute), now.Add(time.Minute), 0)
	if err != nil {
		t.Fatal(err)
	}
	return points
}

func TestGuestHistoryMemoryObservationAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		memory models.Memory
		want   bool
	}{
		{name: "legacy", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40}, want: true},
		{name: "current", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40, Observation: models.MemoryObservation{State: "current"}}, want: true},
		{name: "current measured zero", memory: models.Memory{Total: 100, Free: 100, Observation: models.MemoryObservation{State: "current"}}, want: true},
		{name: "last-known", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40, Observation: models.MemoryObservation{State: "last-known"}}},
		{name: "unavailable observation with retained numbers", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40, Observation: models.MemoryObservation{State: "unavailable"}}},
		{name: "unrecognised observation", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40, Observation: models.MemoryObservation{State: "future-state"}}},
		{name: "missing usage", memory: models.UnavailableMemory(100)},
		{name: "NaN", memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: math.NaN()}},
		{name: "contradictory bytes", memory: models.Memory{Total: 100, Used: 140, Free: 60, Usage: 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotUsage, gotUsed := historyMemoryUsage(tc.memory), historyMemoryUsed(tc.memory)
			if tc.want {
				if gotUsage != tc.memory.Usage || gotUsed != float64(tc.memory.Used) {
					t.Errorf("current/legacy observation lost percentage or byte history: %v / %v", gotUsage, gotUsed)
				}
			} else if gotUsage != -1 || gotUsed != -1 {
				t.Errorf("non-current observation became percentage/byte History: %v / %v", gotUsage, gotUsed)
			}
		})
	}
}

func TestGuestHistoryObservationContinuity(t *testing.T) {
	m := guestHistoryObservationMonitor(t)
	now := time.Now()
	vm := models.VM{ID: "history:node:105", Instance: "history", Node: "node", VMID: 105, Type: "qemu", Status: "running", CPU: .1, LastSeen: now,
		Memory: models.Memory{Total: 100, Used: 40, Free: 60, Usage: 40, Observation: models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: now}},
		Disk:   models.Disk{Total: 100, Used: 50, Free: 50, Usage: 50}, DiskWrite: 10,
		IORateValidity: models.IORateValidity{Explicit: true, DiskWrite: true}}
	ct := models.Container{ID: "history:node:106", Status: "running", LastSeen: now, CPU: .2, Memory: vm.Memory, Disk: vm.Disk}
	record := func() {
		t.Helper()
		vm.LastSeen, ct.LastSeen = time.Now(), time.Now()
		m.recordGuestMetrics([]models.VM{vm}, []models.Container{ct}, time.Now().Add(-time.Second))
	}
	record()
	type series struct{ kind, id, metric string }
	seriesToPreserve := []series{{"vm", vm.ID, "memory"}, {"vm", vm.ID, "memoryused"}, {"vm", vm.ID, "disk"}, {"container", ct.ID, "memory"}, {"container", ct.ID, "memoryused"}}
	originalMemory := make(map[series][]MetricPoint)
	originalStored := make(map[series][]metrics.MetricPoint)
	for _, s := range seriesToPreserve {
		originalMemory[s] = m.metricsHistory.GetGuestMetrics(s.id, s.metric, time.Hour)
		originalStored[s] = guestHistoryStoredPoints(t, m, s.kind, s.id, s.metric)
		if len(originalMemory[s]) != 1 || len(originalStored[s]) != 1 {
			t.Fatalf("fresh %s %s failed to establish both histories", s.kind, s.metric)
		}
	}
	// Persisted points use second precision. Cross a second before the failed
	// polls so a falsely renewed point cannot hide behind the original key.
	time.Sleep(1100 * time.Millisecond)
	for i, state := range []string{"last-known", "unavailable", "future-state"} {
		vm.Memory.Observation.State, ct.Memory.Observation.State = state, state
		vm.GuestAgentStatus = "available" // Not just backup/cooldown deferral.
		vm.DiskStatusReason = []string{"no-status", "agent-error", "prev-agent-timeout"}[i]
		record()
	}
	for _, s := range seriesToPreserve {
		if got := m.metricsHistory.GetGuestMetrics(s.id, s.metric, time.Hour); !reflect.DeepEqual(got, originalMemory[s]) {
			t.Errorf("%s %s renewed or changed its last trusted in-memory observations", s.kind, s.metric)
		}
		if got := guestHistoryStoredPoints(t, m, s.kind, s.id, s.metric); !reflect.DeepEqual(got, originalStored[s]) {
			t.Errorf("%s %s renewed or changed its last trusted persisted observations", s.kind, s.metric)
		}
	}
	for _, metric := range []string{"cpu", "diskwrite"} {
		if got := m.metricsHistory.GetGuestMetrics(vm.ID, metric, time.Hour); len(got) != 4 {
			t.Errorf("independently current %s was suppressed: %d points", metric, len(got))
		}
	}
	if got := m.metricsHistory.GetGuestMetrics(ct.ID, "disk", time.Hour); len(got) != 4 {
		t.Error("independently current container filesystem was suppressed by memory state")
	}
	// Current measured zero is recovery evidence, not missing usage. The
	// original points are kept; this repair never deletes existing History.
	time.Sleep(1100 * time.Millisecond)
	vm.Memory = models.Memory{Total: 100, Free: 100, Observation: models.MemoryObservation{State: "current", Source: "available-field", ObservedAt: time.Now()}}
	ct.Memory = vm.Memory
	vm.Disk = models.Disk{Total: 100, Free: 100}
	vm.DiskStatusReason = ""
	record()
	for _, s := range seriesToPreserve {
		points := m.metricsHistory.GetGuestMetrics(s.id, s.metric, time.Hour)
		if len(points) != 2 || points[1].Value != 0 || !reflect.DeepEqual(points[:1], originalMemory[s]) {
			t.Errorf("%s %s did not resume with current zero while preserving old evidence", s.kind, s.metric)
		}
		stored := guestHistoryStoredPoints(t, m, s.kind, s.id, s.metric)
		if len(stored) != 2 || stored[1].Value != 0 || !reflect.DeepEqual(stored[:1], originalStored[s]) {
			t.Errorf("%s %s persistent recovery lost current zero or original evidence", s.kind, s.metric)
		}
		for _, duration := range []time.Duration{time.Minute, 4 * time.Hour} {
			chart := m.GetGuestMetricsForChart(s.id, s.kind, s.id, duration)[s.metric]
			if len(chart) != 2 || chart[0].Value != originalMemory[s][0].Value || chart[1].Value != 0 {
				t.Errorf("%s %s chart (%v) presented retained polls as new readings: %+v", s.kind, s.metric, duration, chart)
			}
		}
	}
}

func TestGuestHistoryPreservesIndependentCurrentMemory(t *testing.T) {
	for _, source := range []string{"available-field", "agent", "guest-agent-meminfo"} {
		for _, snapshotSource := range []string{"", "previous-snapshot", "guest-agent-meminfo"} {
			t.Run(source+"/diagnostic-"+snapshotSource, func(t *testing.T) {
				m := guestHistoryObservationMonitor(t)
				now := time.Now()
				vm := models.VM{ID: "independent:node:105", Instance: "independent", Node: "node", VMID: 105, Status: "running", LastSeen: now, GuestAgentStatus: "deferred",
					Memory: models.Memory{Total: 100, Used: 25, Free: 75, Usage: 25, Observation: models.MemoryObservation{State: "current", Source: source, ObservedAt: now.Add(-time.Second)}}}
				if snapshotSource != "" {
					m.recordGuestSnapshot(vm.Instance, "qemu", vm.Node, vm.VMID, GuestMemorySnapshot{Status: "running", RetrievedAt: now, MemorySource: snapshotSource})
				}
				m.recordGuestMetrics([]models.VM{vm}, nil, now.Add(-time.Second))
				for _, metric := range []string{"memory", "memoryused"} {
					if got := m.metricsHistory.GetGuestMetrics(vm.ID, metric, time.Hour); len(got) != 1 || got[0].Value != 25 {
						t.Errorf("current selected %s/%s lost History to unrelated deferred disk or diagnostic source", source, metric)
					}
					if got := guestHistoryStoredPoints(t, m, "vm", vm.ID, metric); len(got) != 1 || got[0].Value != 25 {
						t.Errorf("current selected %s/%s lost persistent History", source, metric)
					}
				}
			})
		}
	}
	// Preserve the historical guard for an unannotated deferred guest. New
	// producer observations, not a change to numeric/display availability,
	// are what let independently current memory bypass that legacy heuristic.
	m := guestHistoryObservationMonitor(t)
	vm := models.VM{ID: "legacy-deferred", Status: "running", LastSeen: time.Now(), GuestAgentStatus: "deferred", Memory: models.Memory{Total: 100, Used: 25, Free: 75, Usage: 25}}
	m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
	for _, metric := range []string{"memory", "memoryused"} {
		if got := m.metricsHistory.GetGuestMetrics(vm.ID, metric, time.Hour); len(got) != 0 {
			t.Errorf("unannotated deferral lost its historical guard: %s", metric)
		}
	}
}

func TestGuestHistoryPollFailureAndRecovery(t *testing.T) {
	const mib = uint64(1024 * 1024)
	var phase atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			if strings.Contains(r.URL.Path, "/106/") {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			meminfo := ""
			if phase.Load() != 1 {
				available := 800 * mib
				if phase.Load() == 2 {
					available = 600 * mib
				}
				meminfo = fmt.Sprintf(`,"meminfo":{"total":%d,"available":%d}`, 1000*mib, available)
			}
			fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d%s}}`, 1000*mib, 1000*mib, meminfo)
		case strings.HasSuffix(r.URL.Path, "file-read"):
			http.Error(w, "permission denied", http.StatusForbidden)
		case strings.HasSuffix(r.URL.Path, "get-fsinfo"):
			if phase.Load() == 1 {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, 300*mib)
		case strings.HasSuffix(r.URL.Path, "network-get-interfaces"):
			fmt.Fprint(w, `{"data":{"result":[]}}`)
		case strings.HasSuffix(r.URL.Path, "get-osinfo"):
			fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
		case strings.HasSuffix(r.URL.Path, "info"):
			fmt.Fprint(w, `{"data":{"result":{"version":"1.0"}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	m := guestHistoryObservationMonitor(t)
	m.rateTracker = NewRateTracker()
	m.guestMetadataLimiter = make(map[string]time.Time)
	registry := unifiedresources.NewRegistry(nil)
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 1000 * mib, Mem: 1000 * mib, MaxDisk: 1000 * mib, Disk: 400 * mib, CPU: .1}
	id := makeGuestID("poll-history", res.Node, res.VMID)
	var previous *models.VM
	build := func() models.VM {
		t.Helper()
		vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "poll-history", res, client, id, nil, previous)
		if !ok {
			t.Fatal("guest disappeared")
		}
		m.recordGuestSnapshot("poll-history", "qemu", res.Node, res.VMID, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
		m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
		registry.IngestSnapshot(models.StateSnapshot{VMs: []models.VM{vm}})
		next := previousVMFromView(registry.VMs()[0])
		previous = &next
		return vm
	}
	initial := build()
	if initial.Memory.Observation.State != "current" || initial.Memory.Usage != 20 || initial.DiskStatusReason != "" {
		t.Fatalf("initial poll did not supply current native observations: %+v", initial)
	}
	phase.Store(1)
	failed := build()
	if failed.Memory.Observation.State != "last-known" || failed.Memory.Usage != 20 || failed.GuestAgentStatus == "deferred" || !strings.HasPrefix(failed.DiskStatusReason, "prev-") {
		t.Fatalf("fixture failed to reproduce ordinary failed-read retention: %+v", failed)
	}
	for _, metric := range []string{"memory", "memoryused", "disk"} {
		if got := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour); len(got) != 1 {
			t.Errorf("real client/poll ordinary failure added %s History: %d points", metric, len(got))
		}
	}
	// A new guest with no status has only a cluster disk allocation. Its
	// positive number is not filesystem evidence, even without prev- prefix.
	other := res
	other.VMID = 106
	newVM, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "poll-history", other, client, makeGuestID("poll-history", "node", 106), nil, nil)
	if !ok || newVM.DiskStatusReason != "no-status" || newVM.Disk.Usage <= 0 {
		t.Fatalf("fixture did not retain the unverified positive allocation: %+v", newVM)
	}
	m.recordGuestMetrics([]models.VM{newVM}, nil, time.Now().Add(-time.Second))
	if got := m.metricsHistory.GetGuestMetrics(newVM.ID, "disk", time.Hour); len(got) != 0 {
		t.Error("unverified cluster disk allocation became a filesystem History sample")
	}
	phase.Store(2)
	resumed := build()
	if resumed.Memory.Observation.State != "current" || resumed.Memory.Usage != 40 || resumed.DiskStatusReason != "" {
		t.Fatalf("recovery did not supply current independent PVE and filesystem observations: %+v", resumed)
	}
	for _, metric := range []string{"memory", "memoryused", "disk"} {
		if got := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour); len(got) != 2 {
			t.Errorf("real current recovery did not resume %s History: %d points", metric, len(got))
		}
	}
}
