package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A complete QGA file-read can still contain contradictory memory counters.
// Both ordinary collectors must preserve the independently known provider
// reading, not invent healthy guest pressure or clear an existing incident.
// These are synthetic loopback controls, not native guest/restart acceptance.
func TestGuestMeminfoIntegrityPollingContract(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	const gib = uint64(1024 * 1024 * 1024)
	// Exercise each refusal branch through the real polling consumer, not
	// just the over-total estimate used by the original connected fixture.
	for _, rejection := range []struct{ name, content string }{
		{"derived-over-total", "MemTotal: 4194304 kB\nMemFree: 3145728 kB\nBuffers: 0 kB\nCached: 2097152 kB\n"},
		{"free-over-total-with-shmem", "MemTotal: 4194304 kB\nMemFree: 6291456 kB\nBuffers: 0 kB\nCached: 524288 kB\nShmem: 3145728 kB\n"},
		{"buffers-over-total", "MemTotal: 4194304 kB\nMemFree: 524288 kB\nBuffers: 6291456 kB\nCached: 524288 kB\n"},
		{"cached-over-total", "MemTotal: 4194304 kB\nMemFree: 524288 kB\nBuffers: 0 kB\nCached: 6291456 kB\n"},
		{"reclaimable-over-total", "MemTotal: 4194304 kB\nMemFree: 524288 kB\nBuffers: 0 kB\nCached: 524288 kB\nSReclaimable: 6291456 kB\n"},
		{"shmem-over-total", "MemTotal: 4194304 kB\nMemFree: 524288 kB\nBuffers: 0 kB\nCached: 524288 kB\nShmem: 6291456 kB\n"},
		{"explicit-available-with-impossible-free", "MemTotal: 4194304 kB\nMemAvailable: 2097152 kB\nMemFree: 6291456 kB\n"},
		{"derived-sum-overflow", "MemTotal: 18014398509481983 kB\nMemFree: 18014398509481982 kB\nBuffers: 0 kB\nCached: 2 kB\n"},
	} {
		for _, kind := range []string{"client", "cluster-client"} {
			for _, collector := range []string{"cluster", "node"} {
				t.Run(rejection.name+"/"+kind+"/"+collector, func(t *testing.T) {
					var phase, commands, fileReads atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
							t.Error("configured fixture authentication changed")
							w.WriteHeader(http.StatusForbidden)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						switch {
						case strings.HasSuffix(r.URL.Path, "/nodes"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.HasSuffix(r.URL.Path, "/config"):
							if phase.Load() == 2 {
								fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
							} else {
								fmt.Fprint(w, `{"data":{}}`)
							}
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							lock := ""
							if phase.Load() == 2 {
								lock = `,"lock":"backup"`
							}
							fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d,"cpu":0.1%s}}`, 8*gib, 4*gib, lock)
						case strings.Contains(r.URL.Path, "/agent/"):
							commands.Add(1)
							switch {
							case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
								fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, gib, gib/4)
							case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
								fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
							case strings.HasSuffix(r.URL.Path, "/file-read"):
								fileReads.Add(1)
								if r.URL.Query().Get("file") != "/proc/meminfo" {
									t.Error("unexpected guest file")
								}
								content := "MemTotal: 4194304 kB\nMemFree: 0 kB\nMemAvailable: 0 kB\n"
								if phase.Load() == 1 {
									content = rejection.content
								}
								if phase.Load() == 3 {
									content = "MemTotal: 4194304 kB\nMemFree: 524288 kB\nBuffers: 524288 kB\nCached: 1048576 kB\nSReclaimable: 262144 kB\nShmem: 262144 kB\n"
								}
								if phase.Load() == 4 {
									content = "MemTotal: 4194304 kB\nMemFree: 2097152 kB\nBuffers: 0 kB\nCached: 2097152 kB\n"
								}
								_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": content, "truncated": false}})
							default:
								fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
							}
						default:
							http.NotFound(w, r)
						}
					}))
					t.Cleanup(server.Close)
					client := windowsBootstrapClient(t, server.URL, kind)
					am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
					t.Cleanup(am.Stop)
					cfg := am.GetConfig()
					cfg.Enabled = true
					cfg.GuestDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
					cfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
					cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
					cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 0, "cpu": 0, "disk": 0}}
					am.UpdateConfig(cfg)
					registry := unifiedresources.NewRegistry(nil)
					m := &Monitor{config: &config.Config{}, state: models.NewState(), alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
					id := makeGuestID("meminfo-integrity", "node", 105)
					previous := map[string]models.VM{}
					build := func() models.VM {
						t.Helper()
						// Bound this synthetic poll independently of the executor. No
						// product request, admission or retry timeout is changed.
						ctx, cancel := context.WithTimeout(context.Background(), windowsPollingHangGuard)
						defer cancel()
						res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "cache-aware-guest", VMID: 105, Status: "running", MaxMem: 8 * gib, Mem: 4 * gib, MaxDisk: gib, CPU: .1}
						var vms []models.VM
						if collector == "cluster" {
							vms = m.collectClusterVMResources(ctx, "meminfo-integrity", []indexedClusterResource{{resource: res, guestID: id}}, client, previous, nil)
						} else {
							vms, _ = m.pollNodeVMsWithClusterResourceBuilder(ctx, "meminfo-integrity", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: "running", MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: gib, CPU: .1}}, client, previous, nil)
						}
						if len(vms) != 1 {
							t.Fatalf("collector lost guest: %d", len(vms))
						}
						m.recordGuestMetrics(vms, nil, time.Now().Add(-time.Second))
						m.state.UpdateVMs(vms)
						// Use the production canonical projection, without fabricating the
						// full monitor's persistence/identity wiring in this polling fixture.
						registry.IngestSnapshot(models.StateSnapshot{VMs: vms})
						views := registry.VMs()
						if len(views) != 1 {
							t.Fatalf("canonical ingest lost guest: %d", len(views))
						}
						view := views[0]
						if vms[0].Memory.HasKnownUsage() && (view.MemoryTotal() != vms[0].Memory.Total || view.MemoryUsed() != vms[0].Memory.Used || view.MemoryPercent() != vms[0].Memory.Usage) {
							t.Error("canonical view changed producer tuple")
						}
						if !vms[0].Memory.HasKnownUsage() && view.MemoryEvidence(time.Now()).Available {
							t.Error("unavailable memory became a canonical measurement")
						}
						previous[id] = previousVMFromView(view)
						return vms[0]
					}
					assertMemoryAlerts := func(want int) {
						t.Helper()
						if len(am.GetActiveAlerts()) != want {
							t.Errorf("active alerts=%d, want %d", len(am.GetActiveAlerts()), want)
						}
						for _, a := range am.GetActiveAlerts() {
							if a.Type != "memory" || a.Value != 100 {
								t.Errorf("untrusted input changed established incident: type=%s value=%v", a.Type, a.Value)
							}
						}
					}
					key := guestMemoryCacheKey("meminfo-integrity", "node", 105)
					expireMemory := func() {
						entry := m.vmAgentMemCache[key]
						entry.fetchedAt = time.Now().Add(-2 * vmAgentMemNegativeTTL)
						m.vmAgentMemCache[key] = entry
					}
					// With no successful guest observation, reject the impossible
					// estimate and keep the independently supplied provider tuple.
					phase.Store(1)
					corrupt := build()
					if corrupt.Memory.Observation.Source != "status-mem" || corrupt.Memory.Total != int64(8*gib) || corrupt.Memory.Used != int64(4*gib) || corrupt.Memory.Usage != 50 || corrupt.GuestAgentStatus != "available" || !m.vmAgentMemCache[key].negative {
						t.Errorf("cold contradictory sample became trustworthy or a transport pause: %+v", corrupt.Memory)
					}
					if registry.VMs()[0].MemoryEvidence(time.Now()).PressureKnown {
						t.Error("independent provider reading acquired guest-pressure authority")
					}
					assertMemoryAlerts(0)
					phase.Store(0)
					expireMemory()
					pressure := build()
					if pressure.Memory.Usage != 100 || pressure.Memory.Total != int64(4*gib) {
						t.Error("valid pressure sample changed")
					}
					assertMemoryAlerts(1)
					trustedHistory := map[string][]MetricPoint{}
					for _, metric := range []string{"memory", "memoryused"} {
						trustedHistory[metric] = m.metricsHistory.GetGuestMetrics(id, metric, time.Hour)
						if len(trustedHistory[metric]) != 2 {
							t.Fatalf("cold provider and qualified pressure did not establish %s History", metric)
						}
					}
					assertHistoryUnchanged := func() {
						t.Helper()
						for metric, original := range trustedHistory {
							if got := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour); !reflect.DeepEqual(got, original) {
								t.Errorf("rejected or deferred evidence renewed %s History: %+v", metric, got)
							}
						}
					}
					phase.Store(1)
					expireMemory()
					retained := build()
					if retained.Memory.Usage != 100 || retained.Memory.Total != pressure.Memory.Total || retained.Memory.Observation.State != "last-known" || !retained.Memory.Observation.ObservedAt.Equal(pressure.Memory.Observation.ObservedAt) || !m.vmAgentMemCache[key].negative {
						t.Errorf("corrupt refresh lost original bounded last-known evidence: %+v", retained.Memory)
					}
					if registry.VMs()[0].MemoryEvidence(time.Now()).PressureKnown {
						t.Error("held guest evidence became current pressure")
					}
					assertMemoryAlerts(1)
					assertHistoryUnchanged()
					before := fileReads.Load()
					negative := m.vmAgentMemCache[key]
					_ = build()
					if fileReads.Load() != before || m.vmAgentMemCache[key] != negative {
						t.Error("negative cache retried or renewed corrupt memory")
					}
					assertMemoryAlerts(1)
					assertHistoryUnchanged()
					beforeCommands := commands.Load()
					phase.Store(2)
					locked := build()
					if commands.Load() != beforeCommands || locked.GuestAgentStatus != "deferred" {
						t.Error("integrity rejection weakened backup-lock admission")
					}
					assertMemoryAlerts(1)
					assertHistoryUnchanged()
					phase.Store(3)
					expireMemory()
					recovered := build()
					if recovered.Memory.Observation.Source != "guest-agent-meminfo-derived" || recovered.Memory.Usage != 50 || recovered.Memory.Total != int64(4*gib) || recovered.Memory.Free != int64(gib/2) || recovered.Memory.Cache != int64(3*gib/2) || !registry.VMs()[0].MemoryEvidence(time.Now()).PressureKnown {
						t.Errorf("normal valid recovery lost tuple/trust: %+v", recovered.Memory)
					}
					assertMemoryAlerts(0)
					phase.Store(4)
					expireMemory()
					zero := build()
					if zero.Memory.Used != 0 || zero.Memory.Usage != 0 || zero.Memory.Observation.Source != "guest-agent-meminfo-derived" || !registry.VMs()[0].MemoryEvidence(time.Now()).PressureKnown {
						t.Error("genuine measured zero usage was rejected")
					}
					assertMemoryAlerts(0)
					for metric, values := range map[string][]float64{
						"memory":     {50, 100, 50, 0},
						"memoryused": {float64(4 * gib), float64(4 * gib), float64(2 * gib), 0},
					} {
						points := m.metricsHistory.GetGuestMetrics(id, metric, time.Hour)
						if len(points) != len(values) || !reflect.DeepEqual(points[:2], trustedHistory[metric]) {
							t.Errorf("recovery lost original or added unqualified %s History: %+v", metric, points)
							continue
						}
						for i, value := range values {
							if points[i].Value != value {
								t.Errorf("%s point %d=%v, want %v", metric, i, points[i].Value, value)
							}
						}
					}
					if fileReads.Load() != 5 {
						t.Errorf("ordinary cold-malformed/pressure/malformed/resumed/zero reads=%d, want 5", fileReads.Load())
					}
					data, err := json.Marshal(map[string]any{"rejection": rejection.name, "client": kind, "collector": collector, "pressure": pressure, "corrupt": corrupt, "retained": retained, "recovered": recovered, "zero": zero, "history": m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("MEMINFO_BROWSER_JSON:%s", data)
				})
			}
		}
	}
}
