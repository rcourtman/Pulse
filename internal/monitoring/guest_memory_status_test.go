package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func testGuestStatusMemoryAvailabilitySelection(t *testing.T) {
	for _, tc := range []struct {
		name, fields, source string
		available            uint64
	}{
		{"zero-overrides-components", `"available":0,"free":1024,"buffers":1024,"cached":4096`, "available-field", 0},
		{"zero-overrides-total-minus-used", `"available":0,"used":4096,"free":1024`, "available-field", 0},
		{"zero-only", `"available":0`, "available-field", 0},
		{"missing-with-components", `"free":1024,"buffers":1024,"cached":4096`, "derived-free-buffers-cached", 6144},
		{"null-with-components", `"available":null,"free":1024,"buffers":1024,"cached":4096`, "derived-free-buffers-cached", 6144},
		{"absent-is-not-exhausted", `"used":0`, "", 0},
		{"positive-available", `"available":6144,"free":1024,"cached":4096`, "available-field", 6144},
		{"invalid-capacity-is-not-exhausted", `"available":8193`, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sample proxmox.VMMemInfo
			if err := json.Unmarshal([]byte(`{"total":8192,`+tc.fields+`}`), &sample); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				available, source := deriveGuestMemInfoAvailable(&sample, nil)
				if available != tc.available || source != tc.source {
					t.Errorf("selected %d (%s), want %d (%s)", available, source, tc.available, tc.source)
				}
				copy, err := json.Marshal(sample)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(copy, &sample); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// Real HTTP status decoding feeds each ordinary collector and canonical views,
// History, and threshold evaluation. No guest, backup or QGA fault is induced.
func testGuestStatusMemoryAvailabilityPolling(t *testing.T) {
	const gib = uint64(1024 * 1024 * 1024)
	for _, kind := range []string{"direct", "cluster"} {
		for _, collector := range []string{"cluster-resources", "node-list"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				var phase, commands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						// Plausible components deliberately disagree with measured
						// availability. They are not a replacement for that field.
						fields := fmt.Sprintf(`"available":%d,"free":%d,"buffers":%d,"cached":%d`, 3*gib, gib/2, gib/2, 2*gib)
						switch phase.Load() {
						case 1, 3:
							fields = fmt.Sprintf(`"available":0,"free":%d,"buffers":%d,"cached":%d`, gib/2, gib/2, 2*gib)
						case 2:
							fields = `"available":null`
						case 5:
							fields = fmt.Sprintf(`"free":%d,"buffers":%d,"cached":%d`, gib/2, gib/2, 2*gib)
						}
						fmt.Fprintf(w, `{"data":{"status":"running","cpu":0.1,"agent":0,"maxmem":%d,"mem":%d,"meminfo":{"total":%d,%s}}}`, 20*gib, 20*gib, 4*gib, fields)
					case strings.Contains(r.URL.Path, "/agent/"):
						commands.Add(1)
						t.Error("status availability unexpectedly queued QGA")
						http.Error(w, "unexpected QGA read", http.StatusNotFound)
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
				if kind == "direct" {
					var err error
					client, err = proxmox.NewClient(pcfg)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					client = proxmox.NewClusterClient("status-zero", pcfg, []string{server.URL}, nil)
				}
				am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				defer am.Stop()
				acfg := am.GetConfig()
				acfg.Enabled = true
				acfg.GuestDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
				acfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"memory": 0}}
				am.UpdateConfig(acfg)
				m := &Monitor{config: &config.Config{}, alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time)}
				registry := unifiedresources.NewRegistry(nil)
				id := makeGuestID("status-zero", "node", 105)
				previous := map[string]models.VM{}
				build := func() models.VM {
					t.Helper()
					res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 20 * gib, Mem: 20 * gib, CPU: .1}
					var vms []models.VM
					if collector == "cluster-resources" {
						vms = m.collectClusterVMResources(context.Background(), "status-zero", []indexedClusterResource{{resource: res, guestID: id}}, client, previous, nil)
					} else {
						vms, _ = m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "status-zero", "node", []proxmox.VM{{VMID: 105, Name: "guest", Status: "running", MaxMem: res.MaxMem, Mem: res.Mem, CPU: .1}}, client, previous, nil)
					}
					if len(vms) != 1 {
						t.Fatalf("collector lost VM: %v", vms)
					}
					vm := vms[0]
					m.recordGuestMetrics(vms, nil, time.Now().Add(-time.Second))
					registry.IngestSnapshot(models.StateSnapshot{VMs: vms})
					views := registry.VMs()
					if len(views) != 1 {
						t.Fatalf("canonical view lost VM: %v", views)
					}
					view := views[0]
					evidence := view.MemoryEvidence(time.Now())
					if vm.Memory.HasKnownUsage() {
						if view.MemoryTotal() != vm.Memory.Total || view.MemoryUsed() != vm.Memory.Used || view.MemoryPercent() != vm.Memory.Usage || (vm.Memory.Observation.State == "current" && !evidence.PressureKnown) {
							t.Errorf("selected measurement changed at canonical boundary: %+v", evidence)
						}
					} else if evidence.Available || evidence.PressureKnown {
						t.Errorf("missing measurement became pressure: %+v", evidence)
					}
					previous[id] = previousVMFromView(view)
					return vm
				}
				assertCurrent := func(vm models.VM, want float64, source string) {
					t.Helper()
					if !vm.Memory.HasKnownUsage() || vm.Memory.Total != 4*int64(gib) || vm.Memory.Usage != want || vm.Memory.Observation.State != "current" || vm.Memory.Observation.Source != source {
						t.Errorf("current sample lost zero/presence/capacity: %+v", vm.Memory)
					}
				}
				assertCurrent(build(), 25, "available-field")
				if len(am.GetActiveAlerts()) != 0 {
					t.Error("healthy observation alerted")
				}
				phase.Store(1)
				pressure := build()
				assertCurrent(pressure, 100, "available-field")
				if pressure.Memory.Used != pressure.Memory.Total || pressure.Memory.Free != 0 || pressure.Memory.Cache != 0 {
					t.Errorf("exhaustion retained speculative cache: %+v", pressure.Memory)
				}
				active := am.GetActiveAlerts()
				if len(active) != 1 || active[0].Type != "memory" || active[0].Value != 100 {
					t.Errorf("full pressure was not alerted: %+v", active)
				}
				phase.Store(2)
				unknown := build()
				if unknown.Memory.Observation.State == "current" ||
					(unknown.Memory.HasKnownUsage() && (unknown.Memory.Observation.State != "last-known" || unknown.Memory.Used != pressure.Memory.Used || unknown.Memory.Total != pressure.Memory.Total || !unknown.Memory.Observation.ObservedAt.Equal(pressure.Memory.Observation.ObservedAt))) {
					t.Errorf("null availability invented or renewed usage: %+v", unknown.Memory)
				}
				if len(am.GetActiveAlerts()) != 1 {
					t.Error("missing availability cleared real breach")
				}
				phase.Store(3)
				assertCurrent(build(), 100, "available-field")
				phase.Store(4)
				assertCurrent(build(), 25, "available-field")
				if len(am.GetActiveAlerts()) != 0 {
					t.Error("real healthy observation did not recover breach")
				}
				phase.Store(5)
				assertCurrent(build(), 25, "derived-free-buffers-cached")
				points := m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)
				want := []float64{25, 100, 100, 25, 25}
				if len(points) != len(want) {
					t.Fatalf("History lost measurement or gained missing sample: %+v", points)
				}
				for i, point := range points {
					if point.Value != want[i] {
						t.Errorf("History[%d] = %v, want %v", i, point.Value, want[i])
					}
				}
				if commands.Load() != 0 {
					t.Error("status-only collection gained guest commands")
				}
			})
		}
	}
}
