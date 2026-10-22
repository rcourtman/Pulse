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

// Complete GET-only synthetic PVE observations. Storage allocations are not
// filesystem usage, even if their volume name resembles a container's root.
// This test also compiles over the parent to establish the original failure.
func TestLXCFilesystemUsageIgnoresStorageAllocation(t *testing.T) {
	wasMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, wasMock) })
	const mib = uint64(1024 * 1024)
	cases := []struct {
		name, volume, gap string
		used, size        uint64
	}{
		{"thin-root", "local:subvol-106-disk-0", `,"maxdisk":1048576000`, 20 * mib, 1000 * mib},
		{"different-capacity", "local:subvol-106-disk-0", `,"maxdisk":1048576000,"disk":null`, 990 * mib, 2000 * mib},
		{"other-root", "local:subvol-106-disk-2", `,"disk":0`, 20 * mib, 1000 * mib},
		{"prefix-lookalike", "local:subvol-106-disk-01", ``, 20 * mib, 1000 * mib},
	}
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			for _, tc := range cases {
				t.Run(kind+"/"+collector+"/"+tc.name, func(t *testing.T) {
					const instance, node, vmid = "storage-isolation", "member", 106
					id := makeGuestID(instance, node, vmid)
					var phase, storageReads, contentReads, unexpected atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=fixture" {
							unexpected.Add(1)
							t.Error("method or configured fixture authentication changed")
							w.WriteHeader(403)
							return
						}
						w.Header().Set("Content-Type", "application/json")
						fields := `,"maxdisk":1048576000,"disk":1027604480`
						if phase.Load() == 2 {
							fields = tc.gap
						}
						if phase.Load() == 3 {
							fields = `,"maxdisk":1048576000,"disk":0`
						}
						switch {
						case strings.HasSuffix(r.URL.Path, "/resources"), strings.HasSuffix(r.URL.Path, "/lxc"):
							fmt.Fprintf(w, `{"data":[{"type":"lxc","node":"member","vmid":106,"name":"storage-container","status":"running","cpu":0.2,"cpus":2,"maxmem":8388608,"mem":2097152,"uptime":600%s}]}`, fields)
						case strings.HasSuffix(r.URL.Path, "/nodes"):
							fmt.Fprint(w, `{"data":[]}`)
						case strings.HasSuffix(r.URL.Path, "/storage"):
							storageReads.Add(1)
							if phase.Load() == 0 {
								fmt.Fprint(w, `{"data":[]}`)
							} else {
								fmt.Fprint(w, `{"data":[{"storage":"local","content":"rootdir,images","active":1,"enabled":1}]}`)
							}
						case strings.HasSuffix(r.URL.Path, "/content"):
							contentReads.Add(1)
							fmt.Fprintf(w, `{"data":[{"volid":%q,"content":"rootdir","vmid":106,"used":%d,"size":%d}]}`, tc.volume, tc.used, tc.size)
						case strings.HasSuffix(r.URL.Path, "/status/current"):
							fmt.Fprint(w, `{"data":{"status":"running"}}`)
						case strings.HasSuffix(r.URL.Path, "/config"):
							// The root is not necessarily disk-0, and configured size is not
							// evidence of either filesystem bytes used or free.
							fmt.Fprint(w, `{"data":{"ostype":"debian","rootfs":"local:subvol-106-disk-0,size=1G","onboot":1}}`)
						case strings.HasSuffix(r.URL.Path, "/interfaces"):
							fmt.Fprint(w, `{"data":[]}`)
						default:
							unexpected.Add(1)
							t.Errorf("unexpected read %s", r.URL.Path)
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
							t.Fatalf("lost inventory: %+v", cts)
						}
						ct := cts[0]
						m.state.UpdateContainers(cts)
						m.updateResourceStore(m.currentStateWithScope())
						front := m.buildBroadcastFrontendStateFromSnapshot(m.GetState(), m.mockModeFence.begin())
						if len(front.Resources) != 1 {
							t.Fatal("public resource lost")
						}
						if ct.Disk.Usage < 0 && front.Resources[0].Disk != nil {
							t.Error("unavailable filesystem became a current resource metric")
						}
						if ct.Disk.Usage >= 0 && (front.Resources[0].Disk == nil || front.Resources[0].Disk.Current != ct.Disk.Usage) {
							t.Error("resource percentage differs from filesystem")
						}
						if ct.ID != id || ct.CPU != .2 || ct.Memory.Used != 2097152 || ct.Status != "running" || ct.OnBoot == nil || !*ct.OnBoot {
							t.Errorf("independent guest evidence lost: %+v", ct)
						}
						frames[label] = map[string]any{"resources": front.Resources, "active": am.GetActiveAlerts(), "container": ct}
						return ct
					}
					full := collect("initial")
					if full.Disk.Usage != 98 || len(am.GetActiveAlerts()) != 1 {
						t.Fatalf("positive full filesystem control failed: %+v %+v", full.Disk, am.GetActiveAlerts())
					}
					original := am.GetActiveAlerts()[0]
					if err := am.AcknowledgeAlert(original.ID, "operator"); err != nil {
						t.Fatal(err)
					}
					phase.Store(1)
					for i := 0; i < 2; i++ {
						ct := collect("allocation-conflict")
						if ct.Disk.Total != 1000*int64(mib) || ct.Disk.Used != 980*int64(mib) || ct.Disk.Usage != 98 || len(ct.Disks) != 1 || ct.Disks[0].Usage != 98 {
							t.Errorf("allocation replaced measured filesystem: %+v rows=%+v", ct.Disk, ct.Disks)
						}
						active := am.GetActiveAlerts()
						if len(active) != 1 || active[0].ID != original.ID || active[0].StartTime != original.StartTime || !active[0].Acknowledged || active[0].Value != 98 {
							t.Errorf("allocation changed original acknowledged occurrence: %+v", active)
						}
					}
					history := m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)
					if len(history) != 3 {
						t.Errorf("lost measured history: %d", len(history))
					}
					for _, point := range history {
						if point.Value != 98 {
							t.Errorf("allocation entered filesystem History: %v", point.Value)
						}
					}
					before, _ := json.Marshal(am.GetActiveAlerts())
					phase.Store(2)
					for i := 0; i < 2; i++ {
						ct := collect("unknown")
						if ct.Disk.Usage >= 0 || len(ct.Disks) != 1 || ct.Disks[0].Usage >= 0 {
							t.Errorf("allocation supplied invented missing filesystem usage: %+v", ct)
						}
						after, _ := json.Marshal(am.GetActiveAlerts())
						if !bytes.Equal(before, after) {
							t.Error("missing filesystem observation changed original incident/acknowledgement")
						}
					}
					if n := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); n != 3 {
						t.Errorf("allocation filled missing filesystem History: %d", n)
					}
					phase.Store(3)
					zero := collect("zero")
					collect("zero-confirmed")
					if zero.Disk.Usage != 0 || zero.Disk.Used != 0 || zero.Disk.Total != 1000*int64(mib) || len(am.GetActiveAlerts()) != 0 {
						t.Errorf("allocation blocked measured empty recovery: %+v %+v", zero.Disk, am.GetActiveAlerts())
					}
					if n := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); n != 5 {
						t.Errorf("measured-zero History lost: %d", n)
					}
					if storageReads.Load() != 0 || contentReads.Load() != 0 || unexpected.Load() != 0 {
						t.Errorf("guest collector performed unrelated storage reads: storage=%d content=%d unexpected=%d", storageReads.Load(), contentReads.Load(), unexpected.Load())
					}
					if os.Getenv("PULSE_LXC_STORAGE_FRAMES") == "1" && kind == "client" && collector == "node" && tc.name == "thin-root" {
						wire, err := json.Marshal(frames)
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("LXC_STORAGE_FRAMES %s", wire)
					}
				})
			}
		}
	}
}
