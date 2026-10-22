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

// Actual HTTP admission, both collectors, canonical projection and History.
// No native guest, backup, restart, probe or uncertainty-fence manipulation.
func TestGuestNonemptyWhitespaceLockObservation(t *testing.T) {
	for _, kind := range []string{"client", "cluster-client"} {
		for _, collector := range []string{"cluster", "node"} {
			t.Run(kind+"/"+collector, func(t *testing.T) {
				const mib = uint64(1024 * 1024)
				var locked atomic.Bool
				locked.Store(true)
				var commands atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/nodes"):
						fmt.Fprint(w, `{"data":[]}`)
					case strings.HasSuffix(r.URL.Path, "/config"):
						if locked.Load() {
							fmt.Fprint(w, `{"data":{"lock":" \t\u00a0\u2003\r\n "}}`)
						} else {
							fmt.Fprint(w, `{"data":{}}`)
						}
					case strings.HasSuffix(r.URL.Path, "/status/current"):
						// Independent status telemetry remains usable. The operation
						// lock is deliberately present only in the config endpoint.
						fmt.Fprintf(w, `{"data":{"status":"running","agent":1,"maxmem":%d,"meminfo":{"total":%d,"available":%d}}}`, 8*mib, 8*mib, 6*mib)
					case strings.Contains(r.URL.Path, "/agent/"):
						commands.Add(1)
						switch {
						case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
							fmt.Fprintf(w, `{"data":{"result":[{"mountpoint":"C:\\","type":"ntfs","total-bytes":%d,"used-bytes":%d}]}}`, 1000*mib, 400*mib)
						case strings.HasSuffix(r.URL.Path, "/get-osinfo"):
							fmt.Fprint(w, `{"data":{"name":"Microsoft Windows Server 2022","id":"mswindows"}}`)
						case strings.HasSuffix(r.URL.Path, "/info"):
							fmt.Fprint(w, `{"data":{"result":{"version":"fixture"}}}`)
						default:
							fmt.Fprint(w, `{"data":{"result":[]}}`)
						}
					default:
						t.Errorf("unexpected fixture request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				client := windowsBootstrapClient(t, server.URL, kind)
				// The node fallback also runs the accepted guest's alert check.
				// Give that production effect its normal, fixture-owned manager.
				alertManager := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
				t.Cleanup(alertManager.Stop)
				m := &Monitor{
					config: &config.Config{}, state: models.NewState(), rateTracker: NewRateTracker(),
					alertManager:            alertManager,
					metricsHistory:          NewMetricsHistory(16, time.Hour),
					resourceStore:           unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
					guestAgentFSInfoTimeout: windowsPollingHangGuard, guestAgentNetworkTimeout: windowsPollingHangGuard,
					guestAgentOSInfoTimeout: windowsPollingHangGuard, guestAgentVersionTimeout: windowsPollingHangGuard,
					guestMetadataLimiter: make(map[string]time.Time),
				}
				res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "lock-evidence-guest", VMID: 105, Status: "running", CPU: 0.1, MaxMem: 8 * mib, MaxDisk: 1000 * mib}
				id := makeGuestID("lock-evidence", res.Node, res.VMID)
				projections := make(map[string]any)
				build := func(label string, previous *models.VM) models.VM {
					t.Helper()
					cycleStart := time.Now()
					var vm models.VM
					if collector == "node" {
						prior := make(map[string]models.VM)
						if previous != nil {
							prior[id] = *previous
						}
						vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "lock-evidence", res.Node, []proxmox.VM{{VMID: res.VMID, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, prior, nil)
						if len(vms) != 1 {
							t.Fatal("node collector lost the guest")
						}
						vm = vms[0]
					} else {
						var ok bool
						vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), "lock-evidence", res, client, id, nil, previous)
						if !ok {
							t.Fatal("cluster collector lost the guest")
						}
					}
					m.state.UpdateVMs([]models.VM{vm})
					m.updateResourceStore(m.currentStateWithScope())
					m.recordGuestMetrics([]models.VM{vm}, nil, cycleStart)
					front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
					projections[label] = front.Resources
					if vm.ID != id || vm.Status != "running" || vm.Memory.Used != int64(2*mib) || vm.Memory.Observation.State != "current" {
						t.Fatalf("independent identity/status/memory lost: %+v", vm)
					}
					return vm
				}
				unknown := build("unavailable", nil)
				if commands.Load() != 0 || unknown.GuestAgentStatus != "deferred" || unknown.DiskStatusReason != "lock-unverified" || unknown.Disk.Usage != -1 || len(unknown.Disks) != 0 {
					t.Fatalf("malformed lock dispatched guest work or invented usage: commands=%d vm=%+v", commands.Load(), unknown)
				}
				locked.Store(false)
				initial := build("current", nil)
				if initial.DiskStatusReason != "" || initial.GuestAgentStatus != "available" || initial.Disk.Usage != 40 || len(initial.Disks) != 1 {
					t.Fatalf("genuine clearance did not resume filesystem reads: %+v", initial)
				}
				before := commands.Load()
				locked.Store(true)
				retained := build("retained", &initial)
				if commands.Load() != before || retained.GuestAgentStatus != "deferred" || retained.DiskStatusReason != "prev-lock-unverified" || retained.Disk.Used != initial.Disk.Used || !retained.DiskObservation.ObservedAt.Equal(initial.DiskObservation.ObservedAt) {
					t.Fatal("malformed lock renewed, replaced or queried retained disk evidence")
				}
				if got := len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)); got != 1 {
					t.Fatalf("malformed lock invented a disk History sample: %d", got)
				}
				locked.Store(false)
				resumed := build("resumed", &retained)
				if commands.Load() != before+1 || resumed.GuestAgentStatus != "available" || resumed.DiskStatusReason != "" || resumed.Disk.Usage != 40 {
					t.Fatal("ordinary clearance did not restore the same guest's current reading")
				}
				wire, err := json.Marshal(projections)
				if err != nil {
					t.Fatal(err)
				}
				// These actual public resource projections feed the browser proof;
				// the fixture contains no customer data, credentials or endpoints.
				t.Logf("NONEMPTY_LOCK_PROJECTION %s", wire)
			})
		}
	}
}
