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

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// guestPollFixtureHangGuard is the proxmox client deadline for the loopback
// guest poll fixtures. They answer every request at once, so the deadline only
// stops a hung test. A tight one makes the result depend on the machine: one
// late reply defers the poll (lock-unverified) or fences the guest after an
// agent-timeout, and the History and command counts the tests assert change.
const guestPollFixtureHangGuard = 15 * time.Minute

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
	// Wait on the maintenance barrier itself. Startup maintenance finishes when
	// it finishes, and a timer here only fails the test on a busy machine.
	if err := store.WaitForMaintenance(0); err != nil {
		t.Fatal(err)
	}
	return &Monitor{metricsHistory: NewMetricsHistory(128, 24*time.Hour), metricsStore: store}
}

func guestHistoryStoredPoints(t *testing.T, m *Monitor, kind, id, metric string) []metrics.MetricPoint {
	t.Helper()
	m.metricsStore.Flush()
	now := time.Now()
	// Every point was written by the calling test. Reach back far enough that a
	// stalled run cannot age its earliest points out of the window, yet stay
	// inside the raw tier (ranges up to two hours).
	points, err := m.metricsStore.Query(kind, id, metric, now.Add(-time.Hour), now.Add(time.Minute), 0)
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
		// The cycle starts before the guests are observed, as in production.
		cycleStart := time.Now()
		vm.LastSeen, ct.LastSeen = time.Now(), time.Now()
		m.recordGuestMetrics([]models.VM{vm}, []models.Container{ct}, cycleStart)
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
	// The cycle starts before the guest is observed, as in production.
	cycleStart := time.Now()
	vm := models.VM{ID: "legacy-deferred", Status: "running", LastSeen: time.Now(), GuestAgentStatus: "deferred", Memory: models.Memory{Total: 100, Used: 25, Free: 75, Usage: 25}}
	m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
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
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard})
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
		// The cycle starts before collection, as in pollVMsAndContainersEfficient.
		cycleStart := time.Now()
		vm, raw, source, notes, at, ok := m.buildVMFromClusterResource(context.Background(), "poll-history", res, client, id, nil, previous)
		if !ok {
			t.Fatal("guest disappeared")
		}
		m.recordGuestSnapshot("poll-history", "qemu", res.Node, res.VMID, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
		m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
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
	// The cycle starts before collection, as in pollVMsAndContainersEfficient.
	cycleStart := time.Now()
	newVM, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "poll-history", other, client, makeGuestID("poll-history", "node", 106), nil, nil)
	if !ok || newVM.DiskStatusReason != "no-status" || newVM.Disk.Usage <= 0 {
		t.Fatalf("fixture did not retain the unverified positive allocation: %+v", newVM)
	}
	m.recordGuestMetrics([]models.VM{newVM}, nil, cycleStart)
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

// guestHistoryVMCollector runs one real VM collector against a loopback PVE:
// "cluster-resources" is the normal path, and "per-node" takes over when that
// endpoint is unavailable. Guest 105 has a guest agent whose filesystem read
// fails while phase is 1. Guest 106 never answers a status request, so it only
// ever has a positive cluster allocation.
func guestHistoryVMCollector(t *testing.T, collector string) (*Monitor, *atomic.Int32, func() map[int]models.VM) {
	t.Helper()
	const mib = uint64(1024 * 1024)
	phase := &atomic.Int32{}
	listing := func(vmid int) string {
		return fmt.Sprintf(`{"type":"qemu","id":"qemu/%d","node":"node","name":"guest-%d","vmid":%d,"status":"running","cpu":0.1,"maxcpu":2,"cpus":2,"mem":%d,"maxmem":%d,"disk":%d,"maxdisk":%d}`,
			vmid, vmid, vmid, 1000*mib, 1000*mib, 400*mib, 1000*mib)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/cluster/resources"):
			if collector != "cluster-resources" {
				http.Error(w, "not available", http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, `{"data":[%s,%s]}`, listing(105), listing(106))
		case strings.HasSuffix(r.URL.Path, "/nodes/node/qemu"):
			fmt.Fprintf(w, `{"data":[%s,%s]}`, listing(105), listing(106))
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			if strings.Contains(r.URL.Path, "/106/") {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			meminfo := ""
			if phase.Load() != 1 {
				meminfo = fmt.Sprintf(`,"meminfo":{"total":%d,"available":%d}`, 1000*mib, 800*mib)
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
	t.Cleanup(server.Close)
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard})
	if err != nil {
		t.Fatal(err)
	}
	m := guestHistoryObservationMonitor(t)
	m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
	t.Cleanup(m.alertManager.Stop)
	m.config, m.state, m.rateTracker = &config.Config{}, models.NewState(), NewRateTracker()
	m.guestMetadataLimiter = make(map[string]time.Time)
	m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	online := map[string]string{"node": "online"}
	poll := func() map[int]models.VM {
		t.Helper()
		ctx := context.Background()
		if collector == "per-node" {
			m.pollVMsWithNodes(ctx, "collectors", "", false, client, []proxmox.Node{{Node: "node", Status: "online"}}, online)
		} else if !m.pollVMsAndContainersEfficient(ctx, "collectors", "", false, client, online) {
			t.Fatal("cluster/resources collector fell back")
		}
		// Publication is what supplies the next poll's previous guests.
		m.updateResourceStore(m.currentStateWithScope())
		vms := make(map[int]models.VM)
		for _, vm := range m.state.GetSnapshot().VMs {
			vms[vm.VMID] = vm
		}
		if len(vms) != 2 {
			t.Fatalf("%s collector published %d guests, want 2", collector, len(vms))
		}
		return vms
	}
	return m, phase, poll
}

func TestGuestHistoryObservationAcrossVMCollectors(t *testing.T) {
	for _, collector := range []string{"cluster-resources", "per-node"} {
		t.Run(collector, func(t *testing.T) {
			m, phase, poll := guestHistoryVMCollector(t, collector)
			fetching, allocated := makeGuestID("collectors", "node", 105), makeGuestID("collectors", "node", 106)
			assertPoints := func(when, id, metric string, want int) {
				t.Helper()
				if got := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour); len(got) != want {
					t.Errorf("%s: %s %s in-memory History has %d points, want %d", when, id, metric, len(got), want)
				}
				if got := guestHistoryStoredPoints(t, m, "vm", id, metric); len(got) != want {
					t.Errorf("%s: %s %s persisted History has %d points, want %d", when, id, metric, len(got), want)
				}
			}

			initial := poll()
			if vm := initial[105]; vm.DiskStatusReason != "" || vm.Disk.Usage != 30 || vm.Memory.Observation.State != "current" {
				t.Fatalf("first poll did not read the guest: reason=%q disk=%v memory=%q", vm.DiskStatusReason, vm.Disk.Usage, vm.Memory.Observation.State)
			}
			if vm := initial[106]; vm.DiskStatusReason == "" || vm.Disk.Usage <= 0 {
				t.Fatalf("fixture lost the failed read's positive allocation: reason=%q disk=%v", vm.DiskStatusReason, vm.Disk.Usage)
			}
			assertPoints("first poll", fetching, "disk", 1)
			assertPoints("first poll", fetching, "memory", 1)
			assertPoints("first poll", allocated, "disk", 0)

			// Persisted points use second precision. Cross a second so a falsely
			// renewed point cannot hide behind the original key.
			time.Sleep(1100 * time.Millisecond)
			phase.Store(1)
			failed := poll()
			if vm := failed[105]; !strings.HasPrefix(vm.DiskStatusReason, "prev-") || vm.Disk.Usage != 30 || vm.Memory.Observation.State != "last-known" {
				t.Fatalf("fixture did not retain readings: reason=%q disk=%v memory=%q", vm.DiskStatusReason, vm.Disk.Usage, vm.Memory.Observation.State)
			}
			assertPoints("failed poll", fetching, "disk", 1)
			assertPoints("failed poll", fetching, "memory", 1)
			assertPoints("failed poll", fetching, "memoryused", 1)
			assertPoints("failed poll", allocated, "disk", 0)
			assertPoints("failed poll", fetching, "cpu", 2)

			time.Sleep(1100 * time.Millisecond)
			phase.Store(2)
			if vm := poll()[105]; vm.DiskStatusReason != "" || vm.Disk.Usage != 30 {
				t.Fatalf("recovery did not read the guest again: reason=%q disk=%v", vm.DiskStatusReason, vm.Disk.Usage)
			}
			assertPoints("recovery", fetching, "disk", 2)
			assertPoints("recovery", fetching, "memory", 2)
			assertPoints("recovery", allocated, "disk", 0)
		})
	}
}

func TestMockVMPollingDefersMemoryHistoryToCanonicalSampler(t *testing.T) {
	for _, collector := range []string{"cluster-resources", "per-node"} {
		t.Run(collector, func(t *testing.T) {
			m, _, poll := guestHistoryVMCollector(t, collector)
			mustSetMockEnabled(t, true)
			poll()
			id := makeGuestID("collectors", "node", 105)
			for _, metric := range []string{"cpu", "memory", "disk"} {
				if got := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour); len(got) != 0 {
					t.Errorf("mock mode wrote native %s History: %d points", metric, len(got))
				}
				if got := guestHistoryStoredPoints(t, m, "vm", id, metric); len(got) != 0 {
					t.Errorf("mock mode persisted native %s History: %d points", metric, len(got))
				}
			}
		})
	}
}
