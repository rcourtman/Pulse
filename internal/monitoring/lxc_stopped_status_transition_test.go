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
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Ordinary synthetic HTTP observations through both clients and collectors.
// No container, guest agent, backup, SSH command or native restart is exercised.
func TestLXCStoppedDetailedStatusTransition(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				const mib = uint64(1024 * 1024)
				const instance, node, vmid = "lxc-stop-transition", "member", 106
				id := makeGuestID(instance, node, vmid)
				var phase, statuses, interfaces, configs atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
						t.Error("fixture authentication changed")
						w.WriteHeader(http.StatusForbidden)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					p := phase.Load()
					listedStatus, mem, disk := "running", 4*mib, 400*mib
					if p == 5 {
						listedStatus = "stopped"
					}
					if p == 4 {
						mem, disk = 2*mib, 0
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"), strings.HasSuffix(r.URL.Path, "/storage"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/lxc"):
						fmt.Fprintf(w, `{"data":[{"vmid":106,"name":"transition-container","status":%q,"cpu":0.2,"cpus":2,"maxmem":%d,"mem":%d,"maxdisk":%d,"disk":%d,"uptime":600}]}`, listedStatus, 8*mib, mem, 1000*mib, disk)
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						statuses.Add(1)
						status := "stopped"
						if p == 1 || p == 4 {
							status = "running"
						} else if p == 7 {
							status = "paused"
						}
						if p == 6 {
							// Even a complete JSON prefix is not a complete observation.
							w.Header().Set("Content-Length", "1000")
						}
						fmt.Fprintf(w, `{"data":{"status":%q}}`, status)
					case strings.HasSuffix(r.URL.Path, "/config"):
						configs.Add(1)
						fmt.Fprint(w, `{"data":{"ostype":"debian","rootfs":"local:subvol-106-disk-0,size=1G","onboot":1}}`)
					case strings.HasSuffix(r.URL.Path, "/interfaces"):
						interfaces.Add(1)
						fmt.Fprint(w, `{"data":[{"name":"eth0","inet":"192.0.2.106/24"}]}`)
					default:
						t.Errorf("unexpected fixture request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				t.Cleanup(server.Close)
				client := windowsBootstrapClient(t, server.URL, kind)
				manager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(manager.Stop)
				m := &Monitor{
					config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(), alertManager: manager,
					metricsHistory: NewMetricsHistory(32, time.Hour),
					resourceStore:  unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
				}
				checker := &mockDockerChecker{results: map[int]bool{}}
				m.SetDockerChecker(checker)
				projections := make(map[string]any)
				for p, label := range []string{"cold-stopped", "running", "stopped", "stopped-again", "resumed", "listed-stopped", "rejected-stopped-detail", "paused"} {
					phase.Store(int32(p))
					cycleStart := time.Now()
					res := proxmox.ClusterResource{Type: "lxc", Node: node, Name: "transition-container", VMID: vmid, Status: "running", CPU: .2, MaxCPU: 2, MaxMem: 8 * mib, Mem: 4 * mib, MaxDisk: 1000 * mib, Disk: 400 * mib, Uptime: 600, ObservedAt: cycleStart}
					if p == 5 {
						res.Status = "stopped"
					} else if p == 4 {
						res.Mem, res.Disk = 2*mib, 0
					}
					beforeInterfaces, beforeChecks := interfaces.Load(), checker.callCount()
					var containers []models.Container
					if collector == "node" {
						containers = m.collectContainersWithNodes(context.Background(), instance, "", false, client, []proxmox.Node{{Node: node, Status: "online"}}, map[string]string{node: "online"})
					} else {
						// Older independent agent memory must not make a stopped
						// container's current memory eligible again.
						linked := map[string]models.Host{id: {LastSeen: cycleStart.Add(-time.Second), Memory: models.Memory{Total: int64(8 * mib), Used: int64(7 * mib), Free: int64(mib), Usage: 87.5}}}
						if p == 1 || p == 4 || p == 6 {
							linked = nil
						}
						ct, raw, source, at, ok := m.buildContainerFromClusterResource(context.Background(), instance, res, client, nil, linked)
						if !ok {
							t.Fatal("cluster builder lost container")
						}
						m.recordGuestSnapshot(instance, "lxc", node, vmid, GuestMemorySnapshot{Status: ct.Status, RetrievedAt: at, MemorySource: source, Memory: ct.Memory, Raw: raw})
						containers = m.CheckContainersForDocker(context.Background(), []models.Container{ct})
						m.recordGuestMetrics(nil, containers, cycleStart)
					}
					if len(containers) != 1 || containers[0].ID != id {
						t.Fatalf("%s lost canonical container: %+v", label, containers)
					}
					ct := containers[0]
					running := p == 1 || p == 4 || p == 6
					if running {
						if ct.Status != "running" || ct.CPU != .2 || ct.Memory.Used != int64(res.Mem) || ct.Memory.Observation.State != "current" || ct.Disk.Used != int64(res.Disk) || ct.Disk.Usage != float64(res.Disk)*100/float64(res.MaxDisk) || interfaces.Load() != beforeInterfaces+1 || checker.callCount() != beforeChecks+1 {
							t.Errorf("%s lost ordinary or rejected-detail fallback readings: %+v", label, ct)
						}
					} else {
						wantStatus := "stopped"
						if p == 7 {
							wantStatus = "paused"
						}
						if ct.Status != wantStatus || ct.CPU != 0 || ct.Memory.Used != 0 || ct.Memory.Observation.Source != "powered-off" || ct.Memory.Observation.State != "unavailable" || ct.Disk.Usage != 0 || ct.NetworkIn != 0 || ct.NetworkOut != 0 || ct.DiskRead != 0 || ct.DiskWrite != 0 {
							t.Errorf("%s newer non-running container lost to stale running inventory: %+v", label, ct)
						}
						if interfaces.Load() != beforeInterfaces || checker.callCount() != beforeChecks {
							t.Errorf("%s admitted running-only interface or Docker work", label)
						}
					}
					if ct.OnBoot == nil || !*ct.OnBoot || ct.OSName == "" || ct.Disk.Total != int64(res.MaxDisk) {
						t.Errorf("%s lost ordinary config/capacity metadata", label)
					}
					wantPoints := 0
					for _, prior := range []int{1, 4, 6} {
						if prior <= p {
							wantPoints++
						}
					}
					for _, metric := range []string{"cpu", "memory", "disk"} {
						if got := len(m.metricsHistory.GetGuestMetrics(id, metric, time.Hour)); got != wantPoints {
							t.Errorf("%s admitted non-running %s History: %d points, want %d", label, metric, got, wantPoints)
						}
					}
					m.state.UpdateContainers(containers)
					m.updateResourceStore(m.currentStateWithScope())
					view := m.currentModeReadState().Containers()[0]
					if view.RuntimeStatus() != ct.Status || view.MemoryUsed() != ct.Memory.Used {
						t.Error("canonical read model changed accepted runtime/memory")
					}
					front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
					projections[label] = front.Resources
				}
				if statuses.Load() != 7 || interfaces.Load() != 3 || checker.callCount() != 3 || configs.Load() != 8 {
					t.Errorf("unexpected request counts: status=%d interfaces=%d docker=%d config=%d", statuses.Load(), interfaces.Load(), checker.callCount(), configs.Load())
				}
				wire, err := json.Marshal(projections)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("LXC_STOP_TRANSITION_PROJECTIONS %s", wire)
			})
		}
	}
}
