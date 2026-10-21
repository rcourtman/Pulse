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

// Ordinary GET-only provider observations, not guest/storage operations. A
// current rollup or sibling cannot supply the missing mount's used bytes.
func testLXCMountDiskObservationLifecycle(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	gaps := []struct{ name, row string }{
		{"missing-used", `{"total":1048576000}`},
		{"null-used", `{"total":1048576000,"used":null}`},
		{"missing-total", `{"used":0}`},
		{"null-total", `{"total":null,"used":0}`},
		{"null-row", `null`},
		{"zero-capacity", `{"total":0,"used":0}`},
	}
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, mount := range []string{"rootfs", "mp0"} {
				for _, gap := range gaps {
					t.Run(kind+"/"+collector+"/"+mount+"/"+gap.name, func(t *testing.T) {
						const instance, node, vmid = "mount-observation", "member", 106
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
							switch {
							case strings.HasSuffix(r.URL.Path, "/resources"), strings.HasSuffix(r.URL.Path, "/lxc"):
								fmt.Fprint(w, `{"data":[{"type":"lxc","node":"member","vmid":106,"name":"mount-container","status":"running","cpu":0.2,"cpus":2,"maxmem":8388608,"mem":2097152,"uptime":600,"maxdisk":1048576000,"disk":209715200}]}`)
							case strings.HasSuffix(r.URL.Path, "/status/current"):
								statusCalls.Add(1)
								row := `{"total":1048576000,"used":1027604480}`
								if phase.Load() == 1 {
									row = gap.row
								}
								if phase.Load() == 2 {
									row = `{"total":1048576000,"used":0}`
								}
								root, data := `{"total":1048576000,"used":209715200}`, row
								if mount == "rootfs" {
									root, data = row, root
								}
								fmt.Fprintf(w, `{"data":{"status":"running","diskinfo":{"rootfs":%s,"mp0":%s}}}`, root, data)
							case strings.HasSuffix(r.URL.Path, "/config"):
								fmt.Fprint(w, `{"data":{"ostype":"debian","rootfs":"local:subvol-106-disk-0,size=1G","mp0":"local:subvol-106-disk-1,mp=/data,size=1G","onboot":1}}`)
							case strings.HasSuffix(r.URL.Path, "/interfaces"), strings.HasSuffix(r.URL.Path, "/nodes"):
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
						cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
						cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
						am.UpdateConfig(cfg)
						m := &Monitor{config: &config.Config{}, state: models.NewState(), alertManager: am, rateTracker: NewRateTracker(), metricsHistory: NewMetricsHistory(32, time.Hour), resourceStore: unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))}
						m.SetDockerChecker(&mockDockerChecker{results: map[int]bool{}})
						frames := map[string]any{}
						collect := func(label string) models.Disk {
							t.Helper()
							start, before := time.Now(), statusCalls.Load()
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
							if len(cts) != 1 || statusCalls.Load()-before != 1 {
								t.Fatal("lost container or repeated status read")
							}
							ct := cts[0]
							if ct.ID != id || ct.CPU != .2 || ct.Memory.Used != 2097152 || ct.Status != "running" || ct.OnBoot == nil || !*ct.OnBoot || ct.Disk.Usage != 20 {
								t.Fatalf("mount contaminated independent identity/config/CPU/memory/rollup: %+v", ct)
							}
							if len(ct.Disks) != 2 {
								t.Fatalf("lost mount inventory: %+v", ct.Disks)
							}
							var selected models.Disk
							for _, disk := range ct.Disks {
								if disk.Type == mount {
									selected = disk
								} else if disk.Usage != 20 {
									t.Fatalf("sibling observation changed: %+v", disk)
								}
							}
							wantMount, wantDevice := "/data", "local:subvol-106-disk-1"
							if mount == "rootfs" {
								wantMount, wantDevice = "/", "local:subvol-106-disk-0"
							}
							if selected.Mountpoint != wantMount || selected.Device != wantDevice {
								t.Fatalf("mount identity changed: %+v", selected)
							}
							m.state.UpdateContainers(cts)
							m.updateResourceStore(m.currentStateWithScope())
							front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
							if len(front.Resources) != 1 {
								t.Fatal("canonical resource lost")
							}
							wire, err := json.Marshal(front.Resources[0].Proxmox)
							var facet struct {
								Disks []models.Disk `json:"disks"`
							}
							if err != nil || json.Unmarshal(wire, &facet) != nil || len(facet.Disks) != 2 {
								t.Fatal("public mount projection lost")
							}
							matched := false
							for _, disk := range facet.Disks {
								if disk.Mountpoint == wantMount {
									matched = disk == selected
								}
							}
							if !matched {
								t.Fatalf("canonical mount tuple/presence changed: %s", wire)
							}
							frames[label] = map[string]any{"resources": front.Resources, "active": am.GetActiveAlerts(), "container": ct}
							return selected
						}
						full := collect("full")
						if full.Usage != 98 || len(am.GetActiveAlerts()) != 1 {
							t.Fatalf("full-mount positive control failed: %+v %+v", full, am.GetActiveAlerts())
						}
						original := am.GetActiveAlerts()[0]
						if err := am.AcknowledgeAlert(original.ID, "operator"); err != nil {
							t.Fatal(err)
						}
						collect("acknowledged")
						before, _ := json.Marshal(am.GetActiveAlerts())
						phase.Store(1)
						for i := 0; i < 2; i++ {
							unknown := collect("unknown")
							if unknown.Usage != -1 || unknown.Free != 0 {
								t.Errorf("missing mount counters became measured used/free: %+v", unknown)
							}
							after, _ := json.Marshal(am.GetActiveAlerts())
							if !bytes.Equal(before, after) {
								t.Errorf("unavailable mount changed original incident/ack: before=%s after=%s", before, after)
							}
						}
						phase.Store(2)
						zero := collect("zero")
						collect("zero-confirmed")
						if zero.Usage != 0 || zero.Total != 1048576000 || zero.Used != 0 || zero.Free != zero.Total || len(am.GetActiveAlerts()) != 0 {
							t.Errorf("measured zero failed ordinary recovery: %+v %+v", zero, am.GetActiveAlerts())
						}
						// Current History charts the independent listing rollup, not
						// per-mount samples. A gap must not inject a fabricated zero.
						history := m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)
						if len(history) != 6 {
							t.Fatalf("independent rollup History lost: %d", len(history))
						}
						for _, point := range history {
							if point.Value != 20 {
								t.Errorf("per-mount gap/zero replaced rollup History: %v", point.Value)
							}
						}
						if unexpected.Load() != 0 {
							t.Fatal("unexpected provider operations")
						}
						if os.Getenv("PULSE_LXC_MOUNT_FRAMES") == "1" && kind == "client" && collector == "node" && gap.name == "missing-used" {
							wire, err := json.Marshal(frames)
							if err != nil {
								t.Fatal(err)
							}
							t.Logf("LXC_MOUNT_FRAMES %s %s", mount, wire)
						}
					})
				}
			}
		}
	}
}
