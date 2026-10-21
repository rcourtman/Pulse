package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// Ordinary GET-only synthetic API observations, not native guests or storage.
// This test compiles over the parent to establish the original false readings.
func TestLXCDiskUsagePresenceLifecycle(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	gaps := []struct{ name, fields string }{
		{"missing-used", `,"maxdisk":1048576000`},
		{"null-used", `,"maxdisk":1048576000,"disk":null`},
		{"missing-total", `,"disk":0`},
		{"null-total", `,"maxdisk":null,"disk":0`},
		{"missing-both", ``},
		{"zero-capacity", `,"maxdisk":0,"disk":0`},
	}
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, gap := range gaps {
				t.Run(kind+"/"+collector+"/"+gap.name, func(t *testing.T) {
					const instance, node, vmid = "disk-presence", "member", 106
					id := makeGuestID(instance, node, vmid)
					var phase, requests atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						requests.Add(1)
						if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
							t.Error("fixture method or credential changed")
							w.WriteHeader(403)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						fields := `,"maxdisk":1048576000,"disk":1027604480`
						if phase.Load() == 1 {
							fields = gap.fields
						}
						if phase.Load() == 2 {
							fields = `,"maxdisk":1048576000,"disk":0`
						}
						switch {
						case strings.HasSuffix(r.URL.Path, "/resources"), strings.HasSuffix(r.URL.Path, "/lxc"):
							fmt.Fprintf(w, `{"data":[{"type":"lxc","node":"member","vmid":106,"name":"disk-container","status":"running","cpu":0.2,"cpus":2,"maxmem":8388608,"mem":2097152,"uptime":600%s}]}`, fields)
						case strings.HasSuffix(r.URL.Path, "/nodes"), strings.HasSuffix(r.URL.Path, "/storage"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							// Status-only detail must not manufacture an aggregate disk sample.
							fmt.Fprint(w, `{"data":{"status":"running"}}`)
						case strings.HasSuffix(r.URL.Path, "/config"):
							fmt.Fprint(w, `{"data":{"ostype":"debian","rootfs":"local:subvol-106-disk-0,size=1G","onboot":1}}`)
						case strings.HasSuffix(r.URL.Path, "/interfaces"):
							fmt.Fprint(w, `{"data":[]}`)
						default:
							t.Errorf("unexpected request %s", r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					client := windowsBootstrapClient(t, server.URL, kind)
					am := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
					defer am.Stop()
					am.UpdateConfig(alerts.AlertConfig{Enabled: true, ActivationState: alerts.ActivationActive,
						GuestDefaults:        alerts.ThresholdConfig{Disk: &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}},
						MetricTimeThresholds: map[string]map[string]int{}, TimeThresholds: map[string]int{}})
					adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
					m := &Monitor{config: &config.Config{}, state: models.NewState(), alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), resourceStore: adapter}
					m.SetDockerChecker(&mockDockerChecker{results: map[int]bool{}})
					frames := map[string]any{}
					collect := func(label string) models.Container {
						t.Helper()
						start := time.Now()
						var cts []models.Container
						if collector == "node" {
							cts = m.collectContainersWithNodes(context.Background(), instance, "", false, client, []proxmox.Node{{Node: node, Status: "online"}}, map[string]string{node: "online"})
						} else {
							resources, err := client.GetClusterResources(context.Background(), "vm")
							if err != nil || len(resources) != 1 {
								t.Fatalf("cluster read: %v %+v", err, resources)
							}
							ct, ok := m.handleClusterContainerResource(context.Background(), instance, resources[0], id, client, nil, nil)
							if !ok {
								t.Fatal("container lost")
							}
							cts = []models.Container{ct}
							m.recordGuestMetrics(nil, cts, start)
						}
						if len(cts) != 1 {
							t.Fatalf("lost container: %+v", cts)
						}
						ct := cts[0]
						m.state.UpdateContainers(cts)
						m.updateResourceStore(m.currentStateWithScope())
						front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
						if len(front.Resources) != 1 {
							t.Fatal("canonical resource lost")
						}
						if ct.Disk.Usage < 0 && front.Resources[0].Disk != nil {
							t.Errorf("unavailable disk escaped as a current canonical metric: %+v", front.Resources[0].Disk)
						}
						if ct.Disk.Usage == 0 && (front.Resources[0].Disk == nil || front.Resources[0].Disk.Current != 0) {
							t.Error("canonical resource dropped measured zero")
						}

						frames[label] = map[string]any{"resources": front.Resources, "active": am.GetActiveAlerts(), "container": ct}
						if ct.ID != id || ct.CPU != .2 || ct.Status != "running" || ct.Memory.Used != 2097152 || ct.OnBoot == nil || !*ct.OnBoot {
							t.Errorf("lost independent identity/config/CPU/memory: %+v", ct)
						}
						return ct
					}
					full := collect("full")
					if full.Disk.Usage != 98 || len(am.GetActiveAlerts()) != 1 {
						t.Fatalf("full disk control did not fire: disk=%+v alerts=%+v", full.Disk, am.GetActiveAlerts())
					}
					for _, a := range am.GetActiveAlerts() {
						if err := am.AcknowledgeAlert(a.ID, "operator"); err != nil {
							t.Fatal(err)
						}
					}
					before, _ := json.Marshal(am.GetActiveAlerts())
					phase.Store(1)
					for poll := 0; poll < 2; poll++ {
						unknown := collect("unknown")
						if unknown.Disk.Usage >= 0 || len(unknown.Disks) != 1 || unknown.Disks[0].Usage >= 0 {
							t.Errorf("unavailable counters became measured usage: aggregate=%+v rows=%+v", unknown.Disk, unknown.Disks)
						}
						after, _ := json.Marshal(am.GetActiveAlerts())
						if !bytes.Equal(before, after) {
							t.Errorf("unavailable counters changed original incident/ack: before=%s after=%s", before, after)
						}
					}
					if n := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); n != 1 {
						t.Errorf("unavailable counters entered disk History: %d points", n)
					}
					phase.Store(2)
					zero := collect("zero")
					collect("zero-confirmed")
					if zero.Disk.Usage != 0 || zero.Disk.Total <= 0 || zero.Disk.Used != 0 || len(am.GetActiveAlerts()) != 0 {
						t.Errorf("explicit zero did not recover normally: %+v alerts=%+v", zero, am.GetActiveAlerts())
					}
					if n := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); n != 3 {
						t.Errorf("measured zero missing from History: %d points", n)
					}
					if os.Getenv("PULSE_LXC_DISK_FRAMES") == "1" && kind == "client" && collector == "cluster" && gap.name == "missing-used" {
						wire, err := json.Marshal(frames)
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("LXC_DISK_FRAMES %s", wire)
					}
					if requests.Load() == 0 {
						t.Fatal("fixture did not exercise HTTP")
					}
				})
			}
		}
	}
}
