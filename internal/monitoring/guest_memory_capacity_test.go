package monitoring

import (
	"context"
	"encoding/json"
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
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// These controls run unchanged against the parent. The configured ceiling is
// deliberately different from the guest sample; this is not a native VM probe.
func testGuestMemorySampleCapacitySelection(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	id := makeGuestID("capacity", "node", 105)
	for _, tc := range []struct {
		name                string
		status              proxmox.VMStatus
		info                *proxmox.LinuxMemoryAvailability
		agent               *models.Host
		wantTotal, wantUsed uint64
		wantSource          string
		calls               int
	}{
		{name: "status-available", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Total: 4 * gib, Available: 3 * gib}}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "available-field"},
		{name: "status-free-cache", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Total: 4 * gib, Free: gib / 2, Buffers: gib / 2, Cached: 2 * gib}}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "derived-free-buffers-cached"},
		{name: "status-total-used-gap", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Total: 4 * gib, Used: gib, Free: gib / 2}}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "derived-total-minus-used"},
		{name: "qga-available", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: 4 * gib, EffectiveAvailable: 3 * gib, Source: "meminfo-available"}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "guest-agent-meminfo", calls: 1},
		{name: "qga-derived", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: 4 * gib, EffectiveAvailable: 3 * gib, Source: "meminfo-derived"}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "guest-agent-meminfo-derived", calls: 1},
		{name: "qga-explicit-zero-available", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: 4 * gib, Source: "meminfo-available"}, wantTotal: 4 * gib, wantUsed: 4 * gib, wantSource: "guest-agent-meminfo", calls: 1},
		{name: "qga-invalid-available-exceeds-sample-total", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: 4 * gib, EffectiveAvailable: 5 * gib, Source: "meminfo-available"}, wantTotal: 20 * gib, wantUsed: 20 * gib, wantSource: "status-mem", calls: 1},
		{name: "qga-unrepresentable-sample-total", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: uint64(math.MaxInt64) + 1, Source: "meminfo-available"}, wantTotal: 20 * gib, wantUsed: 20 * gib, wantSource: "status-mem", calls: 1},
		{name: "linked-agent", agent: &models.Host{Memory: models.Memory{Total: 4 * int64(gib), Used: int64(gib), Free: 3 * int64(gib), Usage: 25}}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "agent"},
		{name: "linked-agent-host-sized-rejected", agent: &models.Host{Memory: models.Memory{Total: 64 * int64(gib), Used: int64(gib), Free: 63 * int64(gib), Usage: 100.0 / 64}}, wantTotal: 20 * gib, wantUsed: 20 * gib, wantSource: "status-mem"},
		{name: "balloon-freemem-keeps-low-trust", status: proxmox.VMStatus{Balloon: 4 * gib, FreeMem: 3 * gib}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "status-freemem"},
		{name: "ballooninfo-freemem-keeps-low-trust", status: proxmox.VMStatus{BalloonInfo: &proxmox.VMBalloonInfo{TotalMem: 4 * gib, FreeMem: 3 * gib}}, wantTotal: 4 * gib, wantUsed: gib, wantSource: "status-freemem"},
		{name: "missing-status-total-compatibility", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Available: 3 * gib}}, wantTotal: 20 * gib, wantUsed: 17 * gib, wantSource: "available-field"},
		{name: "availability-only-compatibility", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{EffectiveAvailable: 3 * gib, Source: "meminfo-available"}, wantTotal: 20 * gib, wantUsed: 17 * gib, wantSource: "guest-agent-meminfo", calls: 1},
		{name: "matching-capacity-unchanged", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Total: 20 * gib, Available: 15 * gib}}, wantTotal: 20 * gib, wantUsed: 5 * gib, wantSource: "available-field"},
		{name: "status-total-above-configured-ceiling-rejected", status: proxmox.VMStatus{MemInfo: &proxmox.VMMemInfo{Total: 24 * gib, Available: 18 * gib}}, wantTotal: 20 * gib, wantSource: "unavailable"},
		{name: "qga-total-above-configured-ceiling-rejected", status: proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}, info: &proxmox.LinuxMemoryAvailability{Total: 24 * gib, EffectiveAvailable: 18 * gib, Source: "meminfo-available"}, wantTotal: 20 * gib, wantUsed: 20 * gib, wantSource: "status-mem", calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Monitor{}
			client := &guestMemoryAgentTestClient{stubPVEClient: &stubPVEClient{}, memInfo: tc.info}
			status := tc.status
			status.MaxMem, status.Mem = 20*gib, 20*gib
			linked := map[string]models.Host{}
			if tc.agent != nil {
				linked[id] = *tc.agent
			}
			total, used, source, deferred := m.resolveGuestStatusMemory(context.Background(), client, "capacity", "guest", "node", 105, id, &status, linked, 20*gib, "cluster-resources", &VMMemoryRaw{})
			if total != tc.wantTotal || used != tc.wantUsed || source != tc.wantSource || deferred || client.memCalls != tc.calls || client.rrdCalls != 0 {
				t.Errorf("selected %d/%d (%s, deferred=%t, calls=%d), want %d/%d (%s, calls=%d)", used, total, source, deferred, client.memCalls, tc.wantUsed, tc.wantTotal, tc.wantSource, tc.calls)
			}
			e := unifiedresources.QualifyGuestMemory(safePercentage(float64(used), float64(total)), models.MemoryObservation{State: "current", Source: source, ObservedAt: time.Now()}, true, true, time.Now())
			wantPressure := source == "available-field" || source == "derived-free-buffers-cached" || source == "guest-agent-meminfo" || source == "guest-agent-meminfo-derived" || source == "agent"
			if e.PressureKnown != wantPressure {
				t.Error("capacity selection changed source qualification")
			}
		})
	}
}

func testGuestMemorySampleCapacityCacheBinding(t *testing.T) {
	now := time.Now()
	memory := models.Memory{Total: 4000, Used: 1000, Free: 500, Cache: 2500, Usage: 25}
	key := guestMemoryCacheKey("capacity", "node", 105)
	for _, tc := range []struct {
		name        string
		cachedTotal uint64
		want        bool
	}{
		{"same-tuple", 4000, true}, {"different-denominator-same-available", 5000, false}, {"legacy-availability-only", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &Monitor{vmAgentMemCache: map[string]agentMemCacheEntry{key: {fetchedAt: now, info: proxmox.LinuxMemoryAvailability{Total: tc.cachedTotal, Free: 500, EffectiveAvailable: 3000, Source: "meminfo-available"}}}}
			observation, ok := m.cachedGuestMemoryObservation("capacity", "node", 105, memory, now)
			if ok != tc.want || ok && !observation.ObservedAt.Equal(now) {
				t.Errorf("origin bound to wrong tuple: %+v, %t", observation, ok)
			}
			prev := &GuestMemorySnapshot{GuestType: "qemu", Status: "running", Instance: "capacity", Node: "node", VMID: 105, MemorySource: "guest-agent-meminfo", Memory: memory, Raw: VMMemoryRaw{StatusMaxMem: 20000}}
			retained, ok := m.deferredVMGuestMemory("capacity", "node", 105, 20000, prev, now)
			if ok != tc.want || ok && !reflect.DeepEqual(retained, memory) {
				t.Errorf("deferral lost tuple: %+v, %t", retained, ok)
			}
			if _, ok := m.deferredVMGuestMemory("capacity", "node", 105, 4000, prev, now); ok {
				t.Error("resize to old guest total retained earlier tuple")
			}
			if _, ok := m.deferredVMGuestMemory("other", "node", 105, 20000, prev, now); ok {
				t.Error("retention crossed instance identity")
			}
		})
	}
}

// Both ordinary collectors and concrete HTTP clients feed canonical views,
// History and the production alert call. Lock deferral, age and resize controls
// keep a held value from becoming a new breach or recovery. No guest is frozen.
func testGuestMemorySampleCapacityPolling(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	for _, clientKind := range []string{"direct", "cluster"} {
		for _, collector := range []string{"cluster-resources", "node-list"} {
			t.Run(clientKind+"/"+collector, func(t *testing.T) {
				var phase, commands, memoryReads atomic.Int32
				var configured atomic.Uint64
				configured.Store(20 * gib)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						lock := ""
						if phase.Load() == 1 {
							lock = `,"lock":"backup"`
						}
						fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d%s}}`, configured.Load(), configured.Load(), lock)
					case strings.HasSuffix(r.URL.Path, "/config"):
						if phase.Load() == 1 {
							fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
						} else {
							fmt.Fprint(w, `{"data":{}}`)
						}
					case strings.Contains(r.URL.Path, "/agent/"):
						commands.Add(1)
						switch {
						case strings.HasSuffix(r.URL.Path, "/file-read"):
							memoryReads.Add(1)
							total, available, free := 4*gib, 3*gib, gib/2
							if phase.Load() == 2 {
								available, free = 0, 0
							}
							if phase.Load() == 4 {
								total, available, free = 8*gib, 6*gib, gib
							}
							json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": fmt.Sprintf("MemAvailable: %d kB\nMemFree: %d kB\nMemTotal: %d kB\n", available/1024, free/1024, total/1024)}})
						case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
							fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, gib, gib/4)
						case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
							fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
						case strings.HasSuffix(r.URL.Path, "/version"):
							fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
						default:
							fmt.Fprint(w, `{"data":{"result":[]}}`)
						}
					case strings.HasSuffix(r.URL.Path, "/version"):
						fmt.Fprint(w, `{"data":{"version":"9.2-fixture"}}`)
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				pcfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}
				var client PVEClientInterface
				if clientKind == "direct" {
					var err error
					client, err = proxmox.NewClient(pcfg)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					client = proxmox.NewClusterClient("capacity", pcfg, []string{server.URL}, nil)
				}
				am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				defer am.Stop()
				acfg := am.GetConfig()
				acfg.Enabled = true
				acfg.GuestDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
				acfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
				acfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
				acfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 0, "cpu": 0, "disk": 0}}
				am.UpdateConfig(acfg)
				m := &Monitor{config: &config.Config{}, alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time), vmAgentMemCache: make(map[string]agentMemCacheEntry)}
				registry := unifiedresources.NewRegistry(nil)
				id := makeGuestID("capacity", "node", 105)
				previous := map[string]models.VM{}
				build := func() models.VM {
					t.Helper()
					res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: configured.Load(), Mem: configured.Load(), MaxDisk: gib, CPU: .1}
					var vms []models.VM
					if collector == "cluster-resources" {
						vms = m.collectClusterVMResources(context.Background(), "capacity", []indexedClusterResource{{resource: res, guestID: id}}, client, previous, nil)
					} else {
						vms, _ = m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "capacity", "node", []proxmox.VM{{VMID: 105, Name: "guest", Status: "running", MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: gib, CPU: .1}}, client, previous, nil)
					}
					if len(vms) != 1 {
						t.Fatalf("collector lost VM: %v", vms)
					}
					vm := vms[0]
					m.recordGuestMetrics(vms, nil, time.Now().Add(-time.Second))
					registry.IngestSnapshot(models.StateSnapshot{VMs: vms})
					view := registry.VMs()[0]
					if vm.Memory.HasKnownUsage() && (view.MemoryTotal() != vm.Memory.Total || view.MemoryUsed() != vm.Memory.Used || view.MemoryPercent() != vm.Memory.Usage) {
						t.Error("canonical view changed selected tuple")
					}
					if !vm.Memory.HasKnownUsage() && view.MemoryEvidence(time.Now()).Available {
						t.Error("unavailable memory became a canonical measurement")
					}
					previous[id] = previousVMFromView(view)
					return vm
				}
				assertHealthy := func(vm models.VM, total uint64) {
					t.Helper()
					if vm.Memory.Total != int64(total) || vm.Memory.Used != int64(total/4) || vm.Memory.Usage != 25 || vm.Memory.Free != int64(total/8) || vm.Memory.Cache != int64(total*5/8) {
						t.Errorf("healthy sample mixed capacities: %+v", vm.Memory)
					}
					if !vm.Memory.HasKnownUsage() {
						t.Error("healthy guest became unavailable")
					}
				}
				initial := build()
				assertHealthy(initial, 4*gib)
				if len(am.GetActiveAlerts()) != 0 {
					t.Error("healthy ballooned guest created false memory alert")
				}
				snapshot := m.previousGuestSnapshot("capacity", "qemu", "node", 105)
				rawBytes, _ := json.Marshal(snapshot.Raw)
				var raw map[string]uint64
				json.Unmarshal(rawBytes, &raw)
				if raw["guestAgentMemTotal"] != 4*gib || raw["statusMaxmem"] != 20*gib {
					t.Errorf("raw evidence lost guest/configured capacity distinction: %s", rawBytes)
				}
				if memoryReads.Load() != 1 {
					t.Error("initial sample repeated optional memory read")
				}
				origin := initial.Memory.Observation.ObservedAt
				before := commands.Load()
				phase.Store(1)
				for i := 0; i < 3; i++ {
					held := build()
					assertHealthy(held, 4*gib)
					if held.Memory.Observation.State != "last-known" || !held.Memory.Observation.ObservedAt.Equal(origin) || held.GuestAgentStatus != "deferred" {
						t.Error("lock renewed memory origin or lost shared deferral")
					}
				}
				if commands.Load() != before || len(m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)) != 1 {
					t.Error("lock queued QGA or repeated memory History")
				}
				// A resize cannot retain the old tuple even when its new maximum
				// happens to equal the earlier guest sample's total.
				configured.Store(4 * gib)
				resized := build()
				if resized.Memory.HasKnownUsage() || resized.Memory.Observation.State != "unavailable" {
					t.Errorf("resize retained earlier tuple: %+v", resized.Memory)
				}
				configured.Store(20 * gib)
				m.recordGuestSnapshot("capacity", "qemu", "node", 105, *snapshot)
				aged := *snapshot
				aged.Memory.Observation.ObservedAt = time.Now().Add(-11 * time.Minute)
				m.recordGuestSnapshot("capacity", "qemu", "node", 105, aged)
				key := guestMemoryCacheKey("capacity", "node", 105)
				entry := m.vmAgentMemCache[key]
				entry.fetchedAt = aged.Memory.Observation.ObservedAt
				m.vmAgentMemCache[key] = entry
				expired := build()
				if expired.Memory.HasKnownUsage() || expired.Memory.Observation.State != "unavailable" || commands.Load() != before {
					t.Errorf("expiry fabricated observation or command: %+v", expired.Memory)
				}
				phase.Store(2)
				pressure := build()
				if pressure.Memory.Total != 4*int64(gib) || pressure.Memory.Used != 4*int64(gib) || pressure.Memory.Usage != 100 || pressure.Memory.Observation.State != "current" || len(am.GetActiveAlerts()) != 1 {
					t.Errorf("real full-pressure sample suppressed: %+v; alerts=%d", pressure.Memory, len(am.GetActiveAlerts()))
				}
				for _, a := range am.GetActiveAlerts() {
					if a.Type != "memory" || a.Value != 100 {
						t.Error("wrong pressure alert")
					}
				}
				phase.Store(3)
				entry = m.vmAgentMemCache[key]
				entry.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
				m.vmAgentMemCache[key] = entry
				recovered := build()
				assertHealthy(recovered, 4*gib)
				if len(am.GetActiveAlerts()) != 0 || !recovered.Memory.Observation.ObservedAt.After(pressure.Memory.Observation.ObservedAt) {
					t.Error("current healthy sample did not resolve pressure with a new origin")
				}
				phase.Store(4)
				configured.Store(24 * gib)
				entry = m.vmAgentMemCache[key]
				entry.fetchedAt = time.Now().Add(-2 * vmAgentMemCacheTTL)
				m.vmAgentMemCache[key] = entry
				assertHealthy(build(), 8*gib)
				if memoryReads.Load() != 4 {
					t.Errorf("normal refresh read count = %d, want 4", memoryReads.Load())
				}
				points := m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)
				if len(points) != 4 {
					t.Errorf("current History count = %d, want 4", len(points))
				}
				for i, p := range points {
					want := 25.0
					if i == 1 {
						want = 100
					}
					if p.Value != want {
						t.Errorf("History[%d]=%v, want %v", i, p.Value, want)
					}
				}
			})
		}
	}
}
