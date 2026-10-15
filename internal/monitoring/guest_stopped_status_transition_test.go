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

// Synthetic ordinary HTTP observations, not a native guest shutdown/restart or
// backup experiment. A running inventory can precede a complete stopped status.
// The newer state must own publication, command eligibility and memory lifecycle.
func TestGuestStoppedDetailedStatusTransition(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	for _, agent := range []int{0, 1} {
		for _, kind := range []string{"client", "cluster-client"} {
			for _, collector := range []string{"cluster", "node"} {
				t.Run(fmt.Sprintf("stopped-agent-%d/%s/%s", agent, kind, collector), func(t *testing.T) {
					const mib = uint64(1024 * 1024)
					const instance = "stop-transition"
					const node = "member"
					const vmid = 105
					id := makeGuestID(instance, node, vmid)
					var phase, commands, fileReads, statuses atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
							t.Error("configured fixture authentication changed")
							w.WriteHeader(http.StatusForbidden)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						p := phase.Load()
						switch {
						case strings.HasSuffix(r.URL.Path, "/nodes"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.HasSuffix(r.URL.Path, "/config"):
							fmt.Fprint(w, `{"data":{"onboot":1}}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							statuses.Add(1)
							if p == 1 || p == 4 {
								fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 8*mib, 4*mib)
							} else {
								fmt.Fprintf(w, `{"data":{"status":"stopped","agent":%d}}`, agent)
							}
						case strings.Contains(r.URL.Path, "/agent/"):
							commands.Add(1)
							switch {
							case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
								used := 400 * mib
								if p == 4 {
									used = 0 // A new measured zero must remain a valid reading.
								}
								fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used)
							case strings.HasSuffix(r.URL.Path, "/network-get-interfaces"):
								fmt.Fprint(w, `{"data":{"result":[{"name":"eth0","ip-addresses":[{"ip-address":"192.0.2.105","ip-address-type":"ipv4","prefix":24}]}]}}`)
							case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
								fmt.Fprint(w, `{"data":{"name":"Linux","version":"fixture"}}`)
							case strings.HasSuffix(r.URL.Path, "/info"):
								fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
							case strings.HasSuffix(r.URL.Path, "/file-read"):
								fileReads.Add(1)
								if r.URL.Query().Get("file") != "/proc/meminfo" {
									t.Error("unexpected guest file")
								}
								available := uint64(6144)
								if p == 4 {
									available = 4096
								}
								content := fmt.Sprintf("MemTotal: 8192 kB\nMemAvailable: %d kB\nMemFree: 1024 kB\n", available)
								_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"content": content, "truncated": false}})
							default:
								t.Errorf("unexpected guest command: %s", r.URL.Path)
								http.NotFound(w, r)
							}
						default:
							t.Errorf("unexpected fixture request: %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					t.Cleanup(server.Close)
					client := windowsBootstrapClient(t, server.URL, kind)
					am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
					t.Cleanup(am.Stop)
					m := &Monitor{
						config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(), alertManager: am,
						metricsHistory: NewMetricsHistory(32, time.Hour), guestMetadataLimiter: make(map[string]time.Time),
						resourceStore:           unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
						guestAgentFSInfoTimeout: windowsPollingHangGuard, guestAgentNetworkTimeout: windowsPollingHangGuard,
						guestAgentOSInfoTimeout: windowsPollingHangGuard, guestAgentVersionTimeout: windowsPollingHangGuard,
					}
					projections := make(map[string]any)
					previous := make(map[string]models.VM)
					var initial models.VM
					originalCPU := m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)
					for p, label := range []string{"cold-stopped", "running", "stopped", "stopped-again", "resumed", "listed-stopped"} {
						phase.Store(int32(p))
						cycleStart := time.Now()
						res := proxmox.ClusterResource{Type: "qemu", Node: node, Name: "transition-vm", VMID: vmid, Status: "running", MaxMem: 8 * mib, Mem: 4 * mib, MaxDisk: 1000 * mib, CPU: .2, ObservedAt: cycleStart}
						if p == 5 {
							res.Status = "stopped"
						}
						// A linked agent's older inventory must not override a newer
						// stopped provider state or renew its filesystem observation.
						linked := make(map[string]models.Host)
						if p == 2 || p == 3 {
							linked[id] = models.Host{ID: "linked-fixture", Status: "online", LastSeen: initial.LastSeen, Disks: []models.Disk{{Total: 1000 * int64(mib), Used: 900 * int64(mib), Free: 100 * int64(mib), Usage: 90, Mountpoint: "/"}}}
						}
						before := commands.Load()
						var vms []models.VM
						if collector == "cluster" {
							vms = m.collectClusterVMResources(context.Background(), instance, []indexedClusterResource{{resource: res, guestID: id}}, client, previous, linked)
						} else {
							vms, _ = m.pollNodeVMsWithClusterResourceBuilder(context.Background(), instance, node, []proxmox.VM{{VMID: vmid, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU, ObservedAt: cycleStart}}, client, previous, linked)
						}
						if len(vms) != 1 || vms[0].ID != id {
							t.Fatalf("%s lost canonical guest: %+v", label, vms)
						}
						vm := vms[0]
						m.recordGuestMetrics(vms, nil, cycleStart)
						if p == 1 || p == 4 {
							wantUsed, wantDisk := int64(2*mib), int64(400*mib)
							if p == 4 {
								wantUsed, wantDisk = int64(4*mib), 0
							}
							if vm.Status != "running" || vm.CPU != res.CPU || vm.GuestAgentStatus != "available" || vm.Memory.Used != wantUsed || vm.Memory.Observation.State != "current" || vm.Disk.Used != wantDisk || vm.DiskStatusReason != "" || commands.Load() <= before {
								t.Fatalf("%s did not obtain its own current readings: %+v", label, vm)
							}
							if p == 1 {
								initial = vm
								originalCPU = m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)
							}
						} else {
							if vm.Status != "stopped" || vm.CPU != 0 || vm.GuestAgentStatus != "" || vm.GuestAgentExpected || !vm.GuestAgentEvidence.ObservedAt.IsZero() || vm.DiskStatusReason != "vm-stopped" || vm.Disk.Usage != -1 || len(vm.Disks) != 0 || !vm.DiskObservation.ObservedAt.IsZero() || vm.Memory.Used != 0 || vm.Memory.Observation.Source != "powered-off" || vm.OnBoot == nil || !*vm.OnBoot {
								t.Errorf("%s newer stopped observation lost to stale running inventory: %+v", label, vm)
							}
							if commands.Load() != before {
								t.Errorf("%s sent %d guest commands after observing stopped", label, commands.Load()-before)
							}
							if _, exists := m.vmAgentMemCache[guestMemoryCacheKey(instance, node, vmid)]; exists {
								t.Error("stopped observation retained pre-stop memory cache")
							}
							if p <= 3 && !reflect.DeepEqual(m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour), originalCPU) {
								t.Error("stopped guest fabricated current CPU History")
							}
						}
						m.state.UpdateVMs(vms)
						m.updateResourceStore(m.currentStateWithScope())
						view := m.currentModeReadState().VMs()[0]
						evidence := view.GuestAgentEvidence()
						if view.RuntimeStatus() != vm.Status || view.DiskStatusReason() != vm.DiskStatusReason || evidence.Explicit != vm.GuestAgentEvidence.Explicit || !evidence.ObservedAt.Equal(vm.GuestAgentEvidence.ObservedAt) {
							t.Error("canonical view changed accepted guest state")
						}
						previous[id] = previousVMFromView(view)
						front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
						servedWire, err := json.Marshal(front.Resources)
						var served []struct{ Proxmox unifiedresources.ProxmoxData }
						if err != nil || json.Unmarshal(servedWire, &served) != nil || len(served) != 1 || served[0].Proxmox.GuestAgentStatus != vm.GuestAgentStatus {
							t.Fatal("served resource lost accepted guest-agent state")
						}
						projections[label] = front.Resources
					}
					if statuses.Load() != 5 || fileReads.Load() != 2 {
						t.Errorf("extra status requests or stale pre-stop memory on resume: statuses=%d memory-reads=%d", statuses.Load(), fileReads.Load())
					}
					for _, metric := range []string{"cpu", "memory", "disk"} {
						if got := len(m.metricsHistory.GetGuestMetrics(id, metric, time.Hour)); got != 2 {
							t.Errorf("%s History has %d points; only running observations are samples", metric, got)
						}
					}
					wire, err := json.Marshal(projections)
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("STOP_TRANSITION_PROJECTIONS %s", wire)
				})
			}
		}
	}
}
