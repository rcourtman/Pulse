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

// A complete JSON prefix on a short HTTP body is not a status observation.
// These synthetic ordinary reads use the real clients/collectors and public
// projection. No native guest, QGA command, backup or restart is exercised.
func testGuestDetailedStatusObservationRecovery(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				const mib = uint64(1024 * 1024)
				var phase, calls, guestCommands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						calls.Add(1)
						available := 6 * mib
						if phase.Load() == 0 || phase.Load() == 2 {
							w.Header().Set("Content-Length", "1000")
							available = 7 * mib // Bogus 12.5% in a complete JSON prefix.
						} else if phase.Load() == 3 {
							available = 0 // A genuinely complete measured zero.
						}
						fmt.Fprintf(w, `{"data":{"status":"running","agent":0,"maxmem":%d,"meminfo":{"total":%d,"available":%d}}}`, 8*mib, 8*mib, available)
					case strings.Contains(r.URL.Path, "/agent/"):
						guestCommands.Add(1)
						http.NotFound(w, r)
					default:
						t.Errorf("unexpected fixture request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				client := windowsBootstrapClient(t, server.URL, kind)
				manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(manager.Stop)
				m := &Monitor{
					config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(), alertManager: manager,
					metricsHistory: NewMetricsHistory(16, time.Hour), guestMetadataLimiter: make(map[string]time.Time),
					resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
				}
				res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "status-evidence-guest", VMID: 105, Status: "running", CPU: .1, MaxMem: 8 * mib, Mem: 4 * mib, MaxDisk: 1000 * mib}
				id := makeGuestID("status-evidence", res.Node, res.VMID)
				projections := make(map[string]any)
				var previous *models.VM
				var original time.Time
				for step, label := range []string{"listing", "current", "retained", "resumed"} {
					phase.Store(int32(step))
					res.ObservedAt = time.Now()
					cycleStart := time.Now()
					var vm models.VM
					if collector == "node" {
						prior := make(map[string]models.VM)
						if previous != nil {
							prior[id] = *previous
						}
						vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "status-evidence", res.Node, []proxmox.VM{{VMID: res.VMID, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU, ObservedAt: res.ObservedAt}}, client, prior, nil)
						if len(vms) != 1 {
							t.Fatal("node collector lost guest")
						}
						vm = vms[0]
					} else {
						var ok bool
						var raw VMMemoryRaw
						var source string
						var notes []string
						var at time.Time
						vm, raw, source, notes, at, ok = m.buildVMFromClusterResource(context.Background(), "status-evidence", res, client, id, nil, previous)
						if !ok {
							t.Fatal("cluster builder lost guest")
						}
						m.recordGuestSnapshot("status-evidence", "qemu", res.Node, res.VMID, GuestMemorySnapshot{Status: vm.Status, RetrievedAt: at, MemorySource: source, Memory: vm.Memory, Raw: raw, Notes: notes})
					}
					wantUsed := []int64{int64(4 * mib), int64(2 * mib), int64(2 * mib), int64(8 * mib)}[step]
					wantState := "current"
					wantSource := "available-field"
					if step == 0 {
						wantSource = "cluster-resources"
					} else if step == 2 {
						wantState = "last-known"
					}
					if vm.ID != id || vm.Status != "running" || vm.CPU != .1 || vm.Memory.Used != wantUsed || vm.Memory.Observation.State != wantState || vm.Memory.Observation.Source != wantSource {
						t.Fatalf("%s: incomplete status replaced independent/original evidence: %+v", label, vm)
					}
					if step == 1 {
						original = vm.Memory.Observation.ObservedAt
					} else if step == 2 && !vm.Memory.Observation.ObservedAt.Equal(original) {
						t.Fatal("rejected detailed status renewed retained memory")
					}
					m.state.UpdateVMs([]models.VM{vm})
					m.updateResourceStore(m.currentStateWithScope())
					m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
					front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
					projections[label] = front.Resources
					if view := m.currentModeReadState().VMs()[0]; view.MemoryUsed() != wantUsed {
						t.Fatal("canonical read model lost selected memory")
					}
					copy := vm
					previous = &copy
				}
				points := m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)
				if len(points) != 3 || points[0].Value != 50 || points[1].Value != 25 || points[2].Value != 100 || calls.Load() != 4 || guestCommands.Load() != 0 {
					t.Fatalf("incomplete status invented History or repeated/guest requests: points=%+v statuses=%d guest=%d", points, calls.Load(), guestCommands.Load())
				}
				wire, err := json.Marshal(projections)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("DETAILED_STATUS_PROJECTION %s", wire)
			})
		}
	}
}

func testGuestDetailedContainerStatusRejectsPrefixOncePerPoll(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		t.Run(kind, func(t *testing.T) {
			var corrected atomic.Bool
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/nodes"):
					fmt.Fprint(w, `{"data":[]}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					calls.Add(1)
					if !corrected.Load() {
						w.Header().Set("Content-Length", "1000")
					}
					fmt.Fprint(w, `{"data":{"status":"running","diskread":9000000,"netin":8000000,"ip":"192.0.2.105"}}`)
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{"ostype":"debian"}}`)
				case strings.HasSuffix(r.URL.Path, "/interfaces"):
					fmt.Fprint(w, `{"data":[]}`)
				default:
					t.Errorf("unexpected fixture request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := windowsBootstrapClient(t, server.URL, kind)
			m := &Monitor{config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(),
				resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
			projections := make(map[string]any)
			project := func(label string, ct models.Container) {
				m.state.UpdateContainers([]models.Container{ct})
				m.updateResourceStore(m.currentStateWithScope())
				front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{Containers: []models.Container{ct}}, m.mockModeFence.begin())
				projections[label] = front.Resources
			}
			res := proxmox.ClusterResource{Type: "lxc", Node: "node", Name: "status-evidence-container", VMID: 106, Status: "running", CPU: .2, MaxMem: 1000, Mem: 400, MaxDisk: 2000, Disk: 600, DiskRead: 4096, NetIn: 2048, ObservedAt: time.Now()}
			ct, _, _, _, ok := m.buildContainerFromClusterResource(context.Background(), "status-evidence", res, client, nil, nil)
			if !ok || ct.Memory.Used != 400 || ct.Disk.Used != 600 || len(ct.IPAddresses) != 0 || calls.Load() != 1 {
				t.Fatalf("rejected container status escaped or was fetched twice: %+v / calls=%d", ct, calls.Load())
			}
			project("rejected", ct)
			key := makeGuestRateKey("status-evidence", "lxc", res.VMID)
			m.rateTracker.mu.RLock()
			initial := *m.rateTracker.history[key]
			m.rateTracker.mu.RUnlock()
			if initial.diskRead.value != 4096 || initial.networkIn.value != 2048 || !initial.lastObservedAt.Equal(res.ObservedAt) {
				t.Fatal("rejected status replaced listing counters or their origin")
			}
			corrected.Store(true)
			ct, _, _, _, ok = m.buildContainerFromClusterResource(context.Background(), "status-evidence", res, client, nil, nil)
			if !ok || len(ct.IPAddresses) != 1 || ct.IPAddresses[0] != "192.0.2.105" || calls.Load() != 2 {
				t.Fatal("complete ordinary container status failed to recover once")
			}
			m.rateTracker.mu.RLock()
			resumed := *m.rateTracker.history[key]
			m.rateTracker.mu.RUnlock()
			if resumed.diskRead.value != 9000000 || resumed.networkIn.value != 8000000 || !resumed.lastObservedAt.After(res.ObservedAt) {
				t.Fatal("complete status did not own new counter observations")
			}
			project("resumed", ct)
			wire, err := json.Marshal(projections)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("DETAILED_CONTAINER_STATUS_PROJECTION %s", wire)
		})
	}
}
