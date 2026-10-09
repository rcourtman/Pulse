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
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Synthetic, ordinary QGA-only polling after a successful Windows OS reply.
// The deliberately uncertain Linux read is an adverse fixture, not a replay of
// issue #2619 or a claim about the response/cause on its native installation.
func testGuestWindowsMeminfoPolling(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, path := range []string{"cluster", "node"} {
			t.Run(kind+"/"+path, func(t *testing.T) {
				const mib = uint64(1024 * 1024)
				var poll, fileReads, fsReads, configReads atomic.Int32
				var locked atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
						t.Error("lost configured authentication")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						configReads.Add(1)
						if locked.Load() {
							fmt.Fprint(w, `{"data":{"lock":"backup"}}`)
						} else {
							fmt.Fprint(w, `{"data":{}}`)
						}
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"mem":%d}}`, 8*mib, uint64(2+poll.Load())*mib)
					case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
						fsReads.Add(1)
						used := uint64(200+100*poll.Load()) * mib
						if poll.Load() == 3 {
							used = 0
						}
						fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, used)
					case strings.HasSuffix(r.URL.Path, "/agent/file-read"):
						fileReads.Add(1)
						if r.URL.Query().Get("file") != "/proc/meminfo" {
							t.Error("unexpected guest file")
						}
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, "optional Linux file-read failed")
					case strings.HasSuffix(r.URL.Path, "/agent/network-get-interfaces"):
						fmt.Fprint(w, `{"data":{"result":[{"name":"Ethernet","ip-addresses":[{"ip-address":"192.0.2.10","ip-address-type":"ipv4","prefix":24}]}]}}`)
					case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"):
						fmt.Fprint(w, `{"data":{"name":"Microsoft Windows Server 2022","id":"mswindows"}}`)
					case strings.HasSuffix(r.URL.Path, "/agent/info"):
						fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}
				var client PVEClientInterface
				if kind == "cluster-client" {
					client = proxmox.NewClusterClient("windows", cfg, []string{server.URL}, nil)
				} else {
					var err error
					client, err = proxmox.NewClient(cfg)
					if err != nil {
						t.Fatal(err)
					}
				}
				m := guestHistoryObservationMonitor(t)
				m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(m.alertManager.Stop)
				m.config, m.state, m.rateTracker = &config.Config{}, models.NewState(), NewRateTracker()
				m.guestMetadataLimiter = make(map[string]time.Time)
				registry := unifiedresources.NewRegistry(nil)
				m.resourceStore = unifiedresources.NewMonitorAdapter(registry)
				status := &proxmox.VMStatus{Agent: proxmox.VMAgentField{Value: 1}}
				// Learn from the real, unlocked OS endpoint, not seeded display
				// strings or a configured Proxmox OS hint. No Pulse guest agent.
				_, _, name, _, _, deferred := m.fetchGuestAgentMetadata(context.Background(), client, "windows", "node", "guest", 105, status, false)
				if name != "Microsoft Windows Server 2022" || deferred || fileReads.Load() != 0 {
					t.Fatal("normal OS observation absent")
				}
				metadataKey := guestMetadataCacheKey("windows", "node", 105)
				originalMetadata := m.guestMetadataCache[metadataKey]
				memoryKey := guestMemoryCacheKey("windows", "node", 105)
				// The existing Windows failure backoff has expired: it cannot
				// hide the exact parent's inappropriate read in this control.
				originalMemory := agentMemCacheEntry{negative: true, fetchedAt: time.Now().Add(-2 * vmAgentMemNegativeTTL)}
				m.vmAgentMemCache = map[string]agentMemCacheEntry{memoryKey: originalMemory}
				id := makeGuestID("windows", "node", 105)
				res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 8 * mib, MaxDisk: 1000 * mib}
				build := func() models.VM {
					previous := m.previousGuestContextForInstance("windows").vmsByID[id]
					var vm models.VM
					if path == "node" {
						vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "windows", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, map[string]models.VM{id: previous}, nil)
						if len(vms) != 1 {
							t.Fatal("node collector lost the guest")
						}
						vm = vms[0]
					} else {
						var ok bool
						vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), "windows", res, client, id, nil, &previous)
						if !ok {
							t.Fatal("cluster builder lost the guest")
						}
					}
					m.state.UpdateVMs([]models.VM{vm})
					m.updateResourceStore(m.currentStateWithScope())
					m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
					view := m.currentModeReadState().VMs()[0]
					if view.DiskUsed() != vm.Disk.Used || view.DiskStatusReason() != vm.DiskStatusReason || view.OSName() != vm.OSName {
						t.Error("accepted Windows poll changed at canonical read boundary")
					}
					front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
					wire, err := json.Marshal(front.Resources)
					if err != nil || strings.Contains(string(wire), "windowsGuest") || strings.Contains(string(wire), "osInfoObservedAt") {
						t.Fatal("private OS admission evidence leaked into JSON")
					}
					var served []struct{ Proxmox unifiedresources.ProxmoxData }
					if err := json.Unmarshal(wire, &served); err != nil || len(served) != 1 || served[0].Proxmox.GuestAgentStatus != vm.GuestAgentStatus || served[0].Proxmox.DiskStatusReason != vm.DiskStatusReason {
						t.Fatal("served Windows reading lost its status")
					}
					return vm
				}
				var last models.VM
				for i := int32(1); i <= 3; i++ {
					if i > 1 {
						// Persistent History uses second-resolution keys; separate
						// ordinary observations rather than accepting overwritten points.
						time.Sleep(1100 * time.Millisecond)
					}
					poll.Store(i)
					res.CPU = float64(i) / 10
					last = build()
					wantUsed := int64(200+100*i) * int64(mib)
					if i == 3 {
						wantUsed = 0
					}
					if last.ID != id || last.Disk.Used != wantUsed || last.DiskStatusReason != "" || last.GuestAgentStatus != "available" || last.CPU != res.CPU || last.Memory.Used != int64(2+i)*int64(mib) || last.Memory.Observation.State != "current" {
						t.Errorf("poll %d lost independent Windows readings: disk=%d reason=%q guest=%q memory=%d/%s cpu=%v", i, last.Disk.Used, last.DiskStatusReason, last.GuestAgentStatus, last.Memory.Used, last.Memory.Observation.State, last.CPU)
					}
				}
				if fileReads.Load() != 0 || fsReads.Load() != 3 || !reflect.DeepEqual(m.vmAgentMemCache[memoryKey], originalMemory) || !reflect.DeepEqual(m.guestMetadataCache[metadataKey], originalMetadata) {
					t.Errorf("known Windows sent Linux work or renewed evidence: file=%d fs=%d", fileReads.Load(), fsReads.Load())
				}
				for metric, want := range map[string][]float64{"disk": {30, 40, 0}, "cpu": {10, 20, 30}, "memory": {37.5, 50, 62.5}} {
					points := guestHistoryStoredPoints(t, m, "vm", id, metric)
					var values []float64
					for _, point := range points {
						values = append(values, point.Value)
					}
					if !reflect.DeepEqual(values, want) || len(m.metricsHistory.GetGuestMetrics(id, metric, time.Hour)) != 3 {
						t.Errorf("Windows %s History = %v, want %v", metric, values, want)
					}
				}
				// OS evidence never bypasses a current operation lock.
				beforeConfigs := configReads.Load()
				locked.Store(true)
				blocked := build()
				if blocked.GuestAgentStatus != "deferred" || blocked.DiskStatusReason != "prev-vm-locked" || fsReads.Load() != 3 || fileReads.Load() != 0 || configReads.Load() <= beforeConfigs || blocked.Disk.Used != last.Disk.Used {
					t.Error("Windows classification relaxed backup-lock admission")
				}
				locked.Store(false)
				if resumed := build(); resumed.GuestAgentStatus != "available" || resumed.DiskStatusReason != "" || resumed.Disk.Used != 0 || fsReads.Load() != 4 || fileReads.Load() != 0 {
					t.Error("normal unlocked Windows disk polling did not resume")
				}
			})
		}
	}
}
