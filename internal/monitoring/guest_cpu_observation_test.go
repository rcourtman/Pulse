package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
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

// Complete synthetic GET-only inventories, not native guests or QGA commands.
// Both listing routes keep the listing CPU authority and independent memory.
func testGuestCPUObservationLifecycle(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, guestType := range []string{"qemu", "lxc"} {
				for _, gap := range []struct{ name, cpu string }{
					{"missing", ""}, {"null", `,"cpu":null`}, {"negative", `,"cpu":-0.2`},
				} {
					t.Run(kind+"/"+collector+"/"+guestType+"/"+gap.name, func(t *testing.T) {
						const instance, node, vmid = "cpu-observation", "member", 106
						id := makeGuestID(instance, node, vmid)
						var phase, statusCalls, unexpected atomic.Int32
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
								unexpected.Add(1)
								t.Error("fixture method/authentication changed")
								w.WriteHeader(403)
								return
							}
							w.Header().Set("Content-Type", "application/json")
							cpu := `,"cpu":0.98`
							if phase.Load() == 1 {
								cpu = gap.cpu
							}
							if phase.Load() == 2 {
								cpu = `,"cpu":0`
							}
							switch {
							case strings.HasSuffix(r.URL.Path, "/resources"), strings.HasSuffix(r.URL.Path, "/qemu"), strings.HasSuffix(r.URL.Path, "/lxc"):
								fmt.Fprintf(w, `{"data":[{"type":%q,"node":"member","vmid":106,"name":"cpu-guest","status":"running","cpus":2,"maxcpu":2,"maxmem":8388608,"mem":2097152,"uptime":600,"maxdisk":1048576000,"disk":209715200%s}]}`, guestType, cpu)
							case strings.HasSuffix(r.URL.Path, "/status/current"):
								statusCalls.Add(1)
								// The existing listing owns CPU; a different detail CPU
								// must not silently change its meaning or sample time.
								fmt.Fprint(w, `{"data":{"status":"running","agent":0,"cpu":0.05,"mem":2097152,"maxmem":8388608,"meminfo":{"total":8388608,"available":6291456},"diskinfo":{"rootfs":{"total":1048576000,"used":209715200}}}}`)
							case strings.HasSuffix(r.URL.Path, "/config"):
								fmt.Fprint(w, `{"data":{"ostype":"debian","rootfs":"local:subvol-106-disk-0,size=1G","onboot":1}}`)
							case strings.HasSuffix(r.URL.Path, "/interfaces"), strings.HasSuffix(r.URL.Path, "/nodes"), strings.HasSuffix(r.URL.Path, "/storage"), strings.HasSuffix(r.URL.Path, "/rrddata"):
								fmt.Fprint(w, `{"data":[]}`)
							default:
								unexpected.Add(1)
								t.Errorf("unexpected request %s", r.URL.Path)
								http.NotFound(w, r)
							}
						}))
						defer server.Close()
						client := windowsBootstrapClient(t, server.URL, kind)
						am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
						defer am.Stop()
						cfg := am.GetConfig()
						cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
						cfg.GuestDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
						cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"cpu": 0}}
						am.UpdateConfig(cfg)
						m := &Monitor{config: &config.Config{}, state: models.NewState(), alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
						m.SetDockerChecker(&mockDockerChecker{results: map[int]bool{}})
						frames := map[string]any{}
						collect := func(label string) float64 {
							t.Helper()
							start, before := time.Now(), statusCalls.Load()
							var vms []models.VM
							var cts []models.Container
							if collector == "node" {
								nodes := []proxmox.Node{{Node: node, Status: "online"}}
								if guestType == "qemu" {
									vms = m.collectVMsWithNodes(context.Background(), instance, "", false, client, nodes, map[string]string{node: "online"})
								} else {
									cts = m.collectContainersWithNodes(context.Background(), instance, "", false, client, nodes, map[string]string{node: "online"})
								}
							} else {
								resources, err := client.GetClusterResources(context.Background(), "vm")
								if err != nil || len(resources) != 1 {
									t.Fatalf("cluster read: %v %+v", err, resources)
								}
								if guestType == "qemu" {
									vm, ok := m.handleClusterVMResource(context.Background(), instance, resources[0], id, client, nil, nil)
									if !ok {
										t.Fatal("VM lost")
									}
									vms = []models.VM{vm}
								} else {
									ct, ok := m.handleClusterContainerResource(context.Background(), instance, resources[0], id, client, nil, nil)
									if !ok {
										t.Fatal("container lost")
									}
									cts = []models.Container{ct}
								}
								m.recordGuestMetrics(vms, cts, start)
							}
							if len(vms)+len(cts) != 1 || statusCalls.Load()-before != 1 {
								t.Fatal("lost guest or repeated status read")
							}
							var cpu float64
							var memory models.Memory
							var gotID, status string
							var cores int
							if guestType == "qemu" {
								cpu, memory, gotID, status, cores = vms[0].CPU, vms[0].Memory, vms[0].ID, vms[0].Status, vms[0].CPUs
							} else {
								cpu, memory, gotID, status, cores = cts[0].CPU, cts[0].Memory, cts[0].ID, cts[0].Status, cts[0].CPUs
							}
							if gotID != id || status != "running" || memory.Total != 8388608 || memory.Used != 2097152 || cores != 2 {
								t.Fatalf("CPU gap contaminated independent identity/status/memory/capacity: %s %s %+v %d", gotID, status, memory, cores)
							}
							m.state.UpdateVMs(vms)
							m.state.UpdateContainers(cts)
							m.updateResourceStore(m.currentStateWithScope())
							front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
							if len(front.Resources) != 1 {
								t.Fatal("canonical resource lost")
							}
							if cpu < 0 && front.Resources[0].CPU != nil {
								t.Fatalf("unknown CPU kept a current public reading: %+v", front.Resources[0].CPU)
							}
							if cpu >= 0 && (front.Resources[0].CPU == nil || front.Resources[0].CPU.Current != cpu*100) {
								t.Fatalf("measured CPU lost or changed units: %+v, ratio %v", front.Resources[0].CPU, cpu)
							}
							frames[label] = map[string]any{"resources": front.Resources, "active": am.GetActiveAlerts(), "cpu": cpu}
							return cpu
						}
						if cpu := collect("busy"); cpu != .98 || len(am.GetActiveAlerts()) != 1 {
							t.Fatalf("busy positive control failed: cpu=%v, alerts=%+v", cpu, am.GetActiveAlerts())
						}
						for _, alert := range am.GetActiveAlerts() {
							if err := am.AcknowledgeAlert(alert.ID, "operator"); err != nil {
								t.Fatal(err)
							}
						}
						collect("acknowledged")
						before, _ := json.Marshal(am.GetActiveAlerts())
						phase.Store(1)
						for i := 0; i < 2; i++ {
							if cpu := collect("unknown"); cpu != -1 {
								t.Fatalf("missing CPU became measured usage %v", cpu)
							}
							after, _ := json.Marshal(am.GetActiveAlerts())
							if !bytes.Equal(before, after) || len(am.GetRecentlyResolved()) != 0 {
								t.Fatalf("unknown CPU changed original incident/ack: before=%s, after=%s", before, after)
							}
						}
						if n := len(m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)); n != 2 {
							t.Fatalf("unknown CPU entered History: %d", n)
						}
						phase.Store(2)
						if cpu := collect("idle"); cpu != 0 {
							t.Fatal("measured idle CPU lost")
						}
						collect("idle-confirmed")
						if len(am.GetActiveAlerts()) != 0 || len(am.GetRecentlyResolved()) != 1 {
							t.Fatal("measured zero failed ordinary recovery")
						}
						history := m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)
						if len(history) != 4 || history[0].Value != 98 || history[1].Value != 98 || history[2].Value != 0 || history[3].Value != 0 {
							t.Fatalf("CPU History injected idle samples or lost real zero: %+v", history)
						}
						if n := len(m.metricsHistory.GetGuestMetrics(id, "memory", time.Hour)); n != 6 {
							t.Fatalf("CPU gap suppressed independent memory History: %d", n)
						}
						if unexpected.Load() != 0 {
							t.Fatal("unexpected provider operations")
						}
						if os.Getenv("PULSE_GUEST_CPU_FRAMES") == "1" && kind == "client" && collector == "node" && gap.name == "missing" {
							wire, err := json.Marshal(frames)
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("GUEST_CPU_FRAMES %s %s", guestType, wire)
						}
					})
				}
			}
		}
	}
}

func TestGuestCPUUsageRejectsNonFiniteAndNegativeReadings(t *testing.T) {
	for _, cpu := range []float64{-1, -.2, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if guestCPUUsage(cpu) != -1 {
			t.Fatalf("invalid CPU became idle: %v", cpu)
		}
	}
	for _, cpu := range []float64{0, .0058, .98, 23} {
		if guestCPUUsage(cpu) != cpu {
			t.Fatalf("valid CPU changed: %v", cpu)
		}
	}
}
