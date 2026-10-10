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

func TestVMFilesystemAliasUsageIsOrderIndependent(t *testing.T) {
	for _, identity := range []string{"linux-device", "windows-guid", "windows-path", "btrfs-fallback", "zfs-fallback"} {
		t.Run(identity, func(t *testing.T) {
			rows := []proxmox.VMFileSystem{
				{Mountpoint: "/", Type: "ext4", Disk: "/dev/vda1", TotalBytes: 1000, UsedBytes: 0},
				{Mountpoint: "/bind", Type: "ext4", Disk: "/dev/vda1", TotalBytes: 1000, UsedBytes: 100},
				{Mountpoint: "/data", Type: "ext4", Disk: "/dev/vda1", TotalBytes: 1000, UsedBytes: 900},
			}
			for i := range rows {
				switch identity {
				case "windows-guid":
					rows[i].Name, rows[i].Type, rows[i].Disk = windowsVolumeC, "NTFS", windowsPhysicalDrive
					rows[i].Mountpoint = []string{"C:\\", "D:\\mount\\", "E:\\mount\\"}[i]
				case "windows-path":
					rows[i].Type, rows[i].Disk = "NTFS", windowsPhysicalDrive
					rows[i].Mountpoint = []string{"C:\\", "c:/", "C:"}[i]
				case "btrfs-fallback", "zfs-fallback":
					rows[i].Type, rows[i].Disk = strings.TrimSuffix(identity, "-fallback"), ""
				}
			}
			for _, order := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
				t.Run(fmt.Sprint(order), func(t *testing.T) {
					fs := []proxmox.VMFileSystem{rows[order[0]], rows[order[1]], rows[order[2]]}
					summary := (&Monitor{}).summarizeVMFSInfo("aliases", proxmox.ClusterResource{}, fs)
					if summary.totalBytes != 1000 || summary.usedBytes != 900 || summary.invalidBytes || len(summary.individualDisks) != 3 {
						t.Fatalf("same volume changed with mount order: total=%d used=%d invalid=%t rows=%d", summary.totalBytes, summary.usedBytes, summary.invalidBytes, len(summary.individualDisks))
					}
					for i, disk := range summary.individualDisks {
						if disk.Total != int64(fs[i].TotalBytes) || disk.Used != int64(fs[i].UsedBytes) || disk.Free != int64(fs[i].TotalBytes-fs[i].UsedBytes) || disk.Mountpoint != fs[i].Mountpoint || disk.Device != fs[i].Disk {
							t.Fatal("aggregate maximum replaced an individual mount observation")
						}
					}
				})
			}
		})
	}
}

func TestVMFilesystemAliasUsagePreservesIndependentCapacityAndBounds(t *testing.T) {
	row := func(device, mount string, total, used uint64) proxmox.VMFileSystem {
		return proxmox.VMFileSystem{Disk: device, Mountpoint: mount, Type: "ext4", TotalBytes: total, UsedBytes: used}
	}
	for _, tc := range []struct {
		name        string
		rows        []proxmox.VMFileSystem
		total, used uint64
	}{
		{"interleaved-volumes", []proxmox.VMFileSystem{row("a", "/a", 1000, 100), row("b", "/b", 2000, 200), row("a", "/bind-a", 1000, 900), row("b", "/bind-b", 2000, 1800)}, 3000, 2700},
		{"genuine-zero", []proxmox.VMFileSystem{row("a", "/a", 1000, 0), row("a", "/bind-a", 1000, 0)}, 1000, 0},
		{"no-device-ext4", []proxmox.VMFileSystem{row("", "/a", 1000, 100), row("", "/b", 1000, 900)}, 2000, 1000},
		{"signed-boundary", []proxmox.VMFileSystem{row("a", "/a", math.MaxInt64, math.MaxInt64-3), row("a", "/bind-a", math.MaxInt64, math.MaxInt64-1)}, math.MaxInt64, math.MaxInt64 - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			summary := (&Monitor{}).summarizeVMFSInfo("aliases", proxmox.ClusterResource{}, tc.rows)
			if summary.totalBytes != tc.total || summary.usedBytes != tc.used || summary.invalidBytes || len(summary.individualDisks) != len(tc.rows) {
				t.Fatalf("capacity/zero/bounds changed: %+v, want %d/%d", summary, tc.total, tc.used)
			}
		})
	}
}

// This is ordinary QEMU-only polling against synthetic endpoints, not a native
// guest probe or a freeze/recovery experiment. The per-mount readings remain
// untouched while overview/served metrics and History stop taking the first
// alias. Both collectors must retain real filesystem alerts and genuine zero.
func TestVMFilesystemAliasUsagePollingAndHistory(t *testing.T) {
	for _, collector := range []string{"cluster", "node"} {
		t.Run(collector, func(t *testing.T) {
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
					row := func(mount string, used int) map[string]any {
						return map[string]any{"mountpoint": mount, "type": "ext4", "disk": []any{map[string]any{"dev": "/dev/vda1"}}, "total-bytes": 1000, "used-bytes": used}
					}
					rows := []any{row("/", 100), row("/bind", 980)}
					if phase.Load() == 1 {
						rows[0], rows[1] = rows[1], rows[0]
					} else if phase.Load() == 2 {
						rows = []any{row("/", 0), row("/bind", 0)}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": rows}})
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
			res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "guest", VMID: 105, Status: "running", MaxMem: 1000, Mem: 400, MaxDisk: 1000, CPU: .2}
			id := makeGuestID("aliases", "node", 105)
			var breachID string
			var breachStart time.Time
			for step, wantUsed := range []int64{980, 980, 0} {
				if step > 0 {
					// Persisted History has second precision; each is a new poll.
					time.Sleep(1100 * time.Millisecond)
				}
				phase.Store(int32(step))
				previous := m.previousGuestContextForInstance("aliases").vmsByID[id]
				var vm models.VM
				if collector == "node" {
					vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "aliases", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, client, map[string]models.VM{id: previous}, nil)
					if len(vms) != 1 {
						t.Fatal("node collector lost guest")
					}
					vm = vms[0]
				} else {
					var ok bool
					vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), "aliases", res, client, id, nil, &previous)
					if !ok {
						t.Fatal("cluster builder lost guest")
					}
					m.checkGuestAlertsForVM("aliases", vm)
				}
				if vm.Disk.Total != 1000 || vm.Disk.Used != wantUsed || vm.Disk.Free != 1000-wantUsed || vm.Disk.Usage != float64(wantUsed)/10 || len(vm.Disks) != 2 || vm.DiskStatusReason != "" || vm.DiskObservation.ObservedAt.IsZero() {
					t.Fatalf("poll %d published first alias instead of volume usage: %+v", step, vm)
				}
				mountUsed := map[string]int64{"/": 100, "/bind": 980}
				if step == 2 {
					mountUsed = map[string]int64{"/": 0, "/bind": 0}
				}
				for _, disk := range vm.Disks {
					if disk.Used != mountUsed[disk.Mountpoint] {
						t.Fatal("poll replaced individual mount observations")
					}
				}
				m.state.UpdateVMs([]models.VM{vm})
				m.updateResourceStore(m.currentStateWithScope())
				m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
				view := m.currentModeReadState().VMs()[0]
				if view.DiskTotal() != 1000 || view.DiskUsed() != wantUsed || view.DiskPercent() != vm.Disk.Usage || !reflect.DeepEqual(previousVMFromView(view).Disks, vm.Disks) {
					t.Fatal("canonical state lost aggregate or individual mount evidence")
				}
				front := m.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, m.mockModeFence.begin())
				wire, err := json.Marshal(front.Resources)
				var served []struct {
					Disk    *models.ResourceMetricFrontend
					Proxmox struct{ Disks []unifiedresources.DiskInfo }
				}
				if err != nil || json.Unmarshal(wire, &served) != nil || len(served) != 1 || served[0].Disk == nil {
					t.Fatalf("served guest differs: %s / %v", wire, err)
				}
				diskMetric := served[0].Disk
				if diskMetric.Total == nil || *diskMetric.Total != 1000 || diskMetric.Used == nil || *diskMetric.Used != wantUsed || diskMetric.Free == nil || *diskMetric.Free != 1000-wantUsed || diskMetric.Current != vm.Disk.Usage {
					t.Fatalf("served aggregate differs: %+v", diskMetric)
				}
				if len(served[0].Proxmox.Disks) != len(vm.Disks) {
					t.Fatal("served guest lost individual mount rows")
				}
				for i, disk := range served[0].Proxmox.Disks {
					observed := vm.Disks[i]
					if disk.Total != observed.Total || disk.Used != observed.Used || disk.Free != observed.Free || disk.Usage != observed.Usage || disk.Mountpoint != observed.Mountpoint || disk.Device != observed.Device || disk.Filesystem != observed.Type {
						t.Fatalf("served mount %d differs from its observation: %+v / %+v", i, disk, observed)
					}
				}
				active := m.alertManager.GetActiveAlerts()
				if step < 2 {
					if len(active) != 1 {
						t.Fatalf("high mount lost its independent alert: %+v", active)
					}
					if step == 0 {
						breachID, breachStart = active[0].ID, active[0].StartTime
					} else if active[0].ID != breachID || !active[0].StartTime.Equal(breachStart) {
						t.Fatal("mount reordering replaced the breach occurrence")
					}
				} else if len(active) != 0 {
					t.Fatal("genuine zero did not clear the filesystem alert")
				}
			}
			memory := m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)
			stored := guestHistoryStoredPoints(t, m, "vm", id, "disk")
			if len(memory) != 3 || len(stored) != 3 || fsCalls.Load() != 3 {
				t.Fatalf("polling repeated commands or lost History: memory=%d stored=%d commands=%d", len(memory), len(stored), fsCalls.Load())
			}
			for i, want := range []float64{98, 98, 0} {
				if memory[i].Value != want || stored[i].Value != want {
					t.Fatalf("History[%d] took arbitrary alias usage: %v/%v, want %v", i, memory[i].Value, stored[i].Value, want)
				}
			}
		})
	}
}
