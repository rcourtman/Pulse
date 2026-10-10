package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

type incompleteFilesystemClient struct {
	mockPVEClientExtended
	filesystems []proxmox.VMFileSystem
}

func (c *incompleteFilesystemClient) GetVMFSInfo(context.Context, string, int) ([]proxmox.VMFileSystem, error) {
	return c.filesystems, nil
}

func TestVMFilesystemIncompleteAlternateClientCannotPublishRemainder(t *testing.T) {
	for name, bad := range map[string]proxmox.VMFileSystem{
		"overused":        {TotalBytes: 9000, UsedBytes: 9001},
		"signed-overflow": {TotalBytes: uint64(math.MaxInt64) + 1, UsedBytes: 0},
	} {
		t.Run(name, func(t *testing.T) {
			bad.Mountpoint, bad.Type, bad.Disk = "/data", "ext4", "/dev/vdb1"
			peer := proxmox.VMFileSystem{Mountpoint: "/", Type: "ext4", Disk: "/dev/vda1", TotalBytes: 1000, UsedBytes: 0}
			c := &incompleteFilesystemClient{filesystems: []proxmox.VMFileSystem{peer, bad}}
			m := &Monitor{}
			res := proxmox.ClusterResource{Node: "node", VMID: 105, MaxDisk: 10000}
			total, used, free, usage, disks, fresh, reason := m.updateVMDisksFromGuestAgentFSInfo(context.Background(), "partial", res, c, 10000, 0, -1)
			if total != 10000 || used != 0 || free != 10000 || usage != -1 || len(disks) != 0 || fresh || reason != "agent-error" {
				t.Fatalf("invalid peer became a complete current aggregate: %d/%d/%d usage=%v rows=%+v fresh=%t reason=%q", total, used, free, usage, disks, fresh, reason)
			}
			// A real subsequent complete inventory may remove the other volume.
			c.filesystems = []proxmox.VMFileSystem{peer}
			total, used, free, usage, disks, fresh, reason = m.updateVMDisksFromGuestAgentFSInfo(context.Background(), "partial", res, c, 10000, 0, -1)
			if total != 1000 || used != 0 || free != 1000 || usage != 0 || len(disks) != 1 || !fresh || reason != "" {
				t.Fatal("complete inventory removal/zero was treated as partial")
			}
		})
	}
}

// Ordinary QEMU-only collection must not turn a malformed large volume into
// recovery/removal merely because a smaller valid volume now reports zero.
// Exercise both collectors, canonical previous state, served JSON, History and
// genuine filesystem alerts. No guest freeze, command bypass or native probe.
func TestVMFilesystemIncompletePollingPreservesHistoryAndBreach(t *testing.T) {
	for _, path := range []string{"cluster", "node"} {
		t.Run(path, func(t *testing.T) {
			var phase, fsCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":1000,"mem":400,"meminfo":{"total":1000,"available":600}}}`)
				case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
					fsCalls.Add(1)
					rows := `{"mountpoint":"/","type":"ext4","disk":[{"dev":"/dev/vda1"}],"total-bytes":1000,"used-bytes":980},{"mountpoint":"/data","type":"ext4","disk":[{"dev":"/dev/vdb1"}],"total-bytes":9000,"used-bytes":8820}`
					switch phase.Load() {
					case 1:
						rows = `{"mountpoint":"/","type":"ext4","disk":[{"dev":"/dev/vda1"}],"total-bytes":1000,"used-bytes":0},{"mountpoint":"/data","type":"ext4","total-bytes":9000}`
					case 2:
						rows = `{"mountpoint":"/","type":"ext4","disk":[{"dev":"/dev/vda1"}],"total-bytes":1000,"used-bytes":0}`
					}
					fmt.Fprintf(w, `{"data":{"result":[%s]}}`, rows)
				case strings.Contains(r.URL.Path, "/agent/"):
					fmt.Fprint(w, `{"data":{"result":[]}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			m := guestHistoryObservationMonitor(t)
			m.alertManager = alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithoutPersistedAlertRestore())
			t.Cleanup(m.alertManager.Stop)
			cfg := m.alertManager.GetConfig()
			cfg.Enabled = true
			cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
			cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
			m.alertManager.UpdateConfig(cfg)
			m.config, m.state, m.rateTracker = &config.Config{}, models.NewState(), NewRateTracker()
			m.guestMetadataLimiter = make(map[string]time.Time)
			m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 1000, Mem: 400, MaxDisk: 10000, CPU: .2}
			id := makeGuestID("partial-filesystems", "node", 105)
			publish := func(vm models.VM) {
				m.state.UpdateVMs([]models.VM{vm})
				m.updateResourceStore(m.currentStateWithScope())
			}
			build := func() models.VM {
				t.Helper()
				previous := m.previousGuestContextForInstance("partial-filesystems").vmsByID[id]
				var vm models.VM
				if path == "node" {
					vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "partial-filesystems", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, map[string]models.VM{id: previous}, nil)
					if len(vms) != 1 {
						t.Fatal("node collector lost guest")
					}
					vm = vms[0]
				} else {
					var ok bool
					vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), "partial-filesystems", res, client, id, nil, &previous)
					if !ok {
						t.Fatal("cluster builder lost guest")
					}
					m.checkGuestAlertsForVM("partial-filesystems", vm)
				}
				publish(vm)
				m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
				view := m.currentModeReadState().VMs()[0]
				if view.DiskStatusReason() != vm.DiskStatusReason || view.DiskObservation() != vm.DiskObservation || !reflect.DeepEqual(previousVMFromView(view).Disks, vm.Disks) {
					t.Fatalf("canonical projection changed completeness, original age or mount rows: vm=%+v view=%+v", vm, previousVMFromView(view))
				}
				if vm.Disk.Usage >= 0 && (view.DiskTotal() != vm.Disk.Total || view.DiskUsed() != vm.Disk.Used || view.DiskPercent() != vm.Disk.Usage) {
					t.Fatal("canonical projection changed a known disk tuple")
				}
				// Allocated capacity without usage is not a canonical metric.
				// Check absence on the wire below, not equality with raw allocation.
				if vm.Disk.Usage < 0 && (view.DiskTotal() != 0 || view.DiskUsed() != 0 || view.DiskPercent() != 0) {
					t.Fatal("canonical projection retained expired disk metrics")
				}
				front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
				wire, err := json.Marshal(front.Resources)
				var served []struct {
					Proxmox unifiedresources.ProxmoxData
					CPU     *models.ResourceMetricFrontend
					Memory  *models.ResourceMetricFrontend
					Disk    *models.ResourceMetricFrontend
				}
				if err != nil || json.Unmarshal(wire, &served) != nil || len(served) != 1 || served[0].Proxmox.DiskStatusReason != vm.DiskStatusReason || strings.Contains(string(wire), "diskObservation") {
					t.Fatalf("served completeness differs or private observation leaked: %s / %v", wire, err)
				}
				if served[0].CPU == nil || served[0].CPU.Current != 20 || served[0].Memory == nil || served[0].Memory.Current != 40 {
					t.Fatalf("independent current CPU/memory metrics disappeared: %s", wire)
				}
				metric := served[0].Disk
				if vm.Disk.Usage < 0 {
					if metric != nil {
						t.Fatal("unavailable disk reading became a measured wire zero")
					}
				} else if metric == nil || metric.Total == nil || metric.Used == nil || *metric.Total != vm.Disk.Total || *metric.Used != vm.Disk.Used || metric.Current != vm.Disk.Usage {
					t.Fatalf("known disk tuple or genuine measured zero lost on the wire: %s", wire)
				}
				return vm
			}
			initial := build()
			if initial.Disk.Total != 10000 || initial.Disk.Used != 9800 || initial.Disk.Usage != 98 || len(initial.Disks) != 2 || initial.DiskStatusReason != "" || initial.DiskObservation.ObservedAt.IsZero() {
				t.Fatalf("initial complete reading missing: %+v", initial)
			}
			active := m.alertManager.GetActiveAlerts()
			if len(active) != 2 {
				t.Fatalf("complete high readings did not establish two filesystem breaches: %+v", active)
			}
			breaches := make(map[string]time.Time)
			for _, alert := range active {
				breaches[alert.ID] = alert.StartTime
			}
			originalHistory := guestHistoryStoredPoints(t, m, "vm", id, "disk")
			if len(originalHistory) != 1 || originalHistory[0].Value != 98 {
				t.Fatal("complete reading did not establish History")
			}
			phase.Store(1)
			for i := 0; i < 3; i++ {
				retained := build()
				if retained.Disk != initial.Disk || !reflect.DeepEqual(retained.Disks, initial.Disks) || retained.DiskStatusReason != "prev-agent-error" || retained.DiskObservation != initial.DiskObservation {
					t.Fatalf("partial poll %d became fresh or removed an unknown volume: %+v", i, retained)
				}
			}
			aged := m.previousGuestContextForInstance("partial-filesystems").vmsByID[id]
			aged.DiskObservation.ObservedAt = time.Now().Add(-recentGuestAgentEvidenceMaxAge - time.Second)
			publish(aged)
			expired := build()
			if expired.Disk.Usage != -1 || expired.DiskStatusReason != "agent-error" || len(expired.Disks) != 0 || !expired.DiskObservation.ObservedAt.IsZero() {
				t.Fatal("repeated partial responses renewed expired complete evidence")
			}
			active = m.alertManager.GetActiveAlerts()
			if len(active) != 2 {
				t.Fatalf("partial/expired evidence cleared a genuine filesystem breach: %+v", active)
			}
			for _, alert := range active {
				if !alert.StartTime.Equal(breaches[alert.ID]) {
					t.Fatal("partial reading replaced a breach occurrence")
				}
			}
			if !reflect.DeepEqual(guestHistoryStoredPoints(t, m, "vm", id, "disk"), originalHistory) || len(m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)) != 1 || len(m.metricsHistory.GetGuestMetrics(id, "cpu", time.Hour)) != 5 || fsCalls.Load() != 5 {
				t.Fatal("partial evidence created disk History, stopped independent CPU History or retried a command")
			}
			// Complete ordinary replacement (the large volume really absent) and
			// measured zero must still remove/recover. History has second keys.
			time.Sleep(1100 * time.Millisecond)
			phase.Store(2)
			recovered := build()
			if recovered.Disk.Total != 1000 || recovered.Disk.Used != 0 || recovered.Disk.Usage != 0 || len(recovered.Disks) != 1 || recovered.DiskStatusReason != "" || !recovered.DiskObservation.ObservedAt.After(initial.DiskObservation.ObservedAt) || len(m.alertManager.GetActiveAlerts()) != 0 || fsCalls.Load() != 6 {
				t.Fatalf("complete ordinary removal/zero failed to recover: %+v", recovered)
			}
			points := guestHistoryStoredPoints(t, m, "vm", id, "disk")
			if len(points) != 2 || points[0] != originalHistory[0] || points[1].Value != 0 {
				t.Fatal("History lost high reading or failed to append genuine zero")
			}
		})
	}
}

func TestVMFilesystemIncompleteColdPollRemainsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/get-fsinfo"):
			fmt.Fprint(w, `{"data":{"result":[{"mountpoint":"/","type":"ext4","total-bytes":1000,"used-bytes":0},{"mountpoint":"/data","type":"ext4","total-bytes":9000}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	res := proxmox.ClusterResource{Node: "node", VMID: 105, MaxDisk: 10000}
	_, _, _, usage, disks, fresh, reason := (&Monitor{}).updateVMDisksFromGuestAgentFSInfo(context.Background(), "cold-partial", res, client, 10000, 0, -1)
	if usage != -1 || fresh || len(disks) != 0 || reason != "agent-error" {
		t.Fatalf("cold partial result became healthy usage: %v/%+v/%t/%q", usage, disks, fresh, reason)
	}
}
