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

const runtimeMountMiB = int64(1024 * 1024)

type runtimeDataMountFixture struct {
	monitor *Monitor
	client  *proxmox.Client
	phase   atomic.Int32
	fsCalls atomic.Int32
}

// Only synthetic, guest-local HTTP is used. No real guest, Pulse agent,
// backup, mount operation or guest-command safety setting participates.
func newRuntimeDataMountFixture(t *testing.T, fsType, mount string) *runtimeDataMountFixture {
	t.Helper()
	f := &runtimeDataMountFixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			t.Errorf("unexpected modifying request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "read-only fixture", http.StatusMethodNotAllowed)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprint(w, `{"data":{"status":"running","agent":1,"maxmem":1000,"mem":400,"meminfo":{"total":1000,"available":600}}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
			f.fsCalls.Add(1)
			row := func(path, kind, device string, total, used int64) map[string]any {
				return map[string]any{"mountpoint": path, "type": kind, "disk": []any{map[string]any{"dev": device}}, "total-bytes": total * runtimeMountMiB, "used-bytes": used * runtimeMountMiB}
			}
			used, aliasUsed := int64(8820), int64(6000)
			phase := f.phase.Load()
			if phase == 2 {
				used, aliasUsed = 0, 0
			}
			rows := []any{
				row("/", "ext4", "/dev/vda1", 1000, 200),
				row(mount, fsType, "/dev/vdb1", 9000, used),
				row("/data-alias", fsType, "/dev/vdb1", 9000, aliasUsed),
				row("/var/lib/docker/overlay2/layer/merged", "ext4", "/dev/vda1", 1000, 1000),
				row("/var/lib/containers/storage/overlay/layer/merged", "xfs", "/dev/vda1", 1000, 1000),
				row(mount+"/virtual", "overlay", "overlay", 9000, 9000),
				row(mount+"/remote", "nfs4", "remote:/data", 9000, 9000),
				row(mount+"/memory", "tmpfs", "tmpfs", 9000, 9000),
			}
			if phase == 1 {
				rows[1], rows[2] = rows[2], rows[1]
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": rows}})
		case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"), strings.HasSuffix(r.URL.Path, "/agent/info"):
			fmt.Fprint(w, `{"data":{"result":{}}}`)
		case strings.HasSuffix(r.URL.Path, "/agent/network-get-interfaces"):
			fmt.Fprint(w, `{"data":{"result":[]}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	var err error
	f.client, err = proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: guestPollFixtureHangGuard})
	if err != nil {
		t.Fatal(err)
	}
	m := newTestMonitor(t)
	m.config = &config.Config{}
	m.guestMetadataLimiter = make(map[string]time.Time)
	m.resourceStore = unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	cfg := m.alertManager.GetConfig()
	cfg.Enabled, cfg.ActivationState = true, alerts.ActivationActive
	cfg.GuestDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
	cfg.MetricTimeThresholds = map[string]map[string]int{"guest": {"disk": 0}}
	m.alertManager.UpdateConfig(cfg)
	f.monitor = m
	return f
}

func (f *runtimeDataMountFixture) poll(t *testing.T, collector string) models.VM {
	t.Helper()
	m := f.monitor
	id := makeGuestID("runtime-data", "node", 105)
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "podman-vm", VMID: 105, Status: "running", MaxMem: 1000, Mem: 400, MaxDisk: uint64(10000 * runtimeMountMiB), CPU: .2}
	previous := m.previousGuestContextForInstance("runtime-data").vmsByID[id]
	var vm models.VM
	if collector == "node" {
		vms, _ := m.pollNodeVMsWithClusterResourceBuilder(context.Background(), "runtime-data", "node", []proxmox.VM{{VMID: 105, Name: res.Name, Status: res.Status, MaxMem: res.MaxMem, Mem: res.Mem, MaxDisk: res.MaxDisk, CPU: res.CPU}}, f.client, map[string]models.VM{id: previous}, nil)
		if len(vms) != 1 {
			t.Fatal("node collector lost guest")
		}
		vm = vms[0]
	} else {
		var ok bool
		vm, _, _, _, _, ok = m.buildVMFromClusterResource(context.Background(), "runtime-data", res, f.client, id, nil, &previous)
		if !ok {
			t.Fatal("cluster builder lost guest")
		}
		m.checkGuestAlertsForVM("runtime-data", vm)
	}
	m.state.UpdateVMs([]models.VM{vm})
	m.updateResourceStore(m.currentStateWithScope())
	return vm
}

func TestVMRuntimeDataMountPolling(t *testing.T) {
	for _, fsType := range []string{"ext4", "xfs", "btrfs"} {
		for _, mount := range []string{"/var/lib/containers", "/var/lib/docker"} {
			for _, collector := range []string{"cluster", "node"} {
				t.Run(fsType+mount+"/"+collector, func(t *testing.T) {
					f := newRuntimeDataMountFixture(t, fsType, mount)
					var breachID string
					for phase := 0; phase < 3; phase++ {
						f.phase.Store(int32(phase))
						vm := f.poll(t, collector)
						wantUsed := int64(9020 * runtimeMountMiB)
						wantMountUsage := float64(98)
						if phase == 2 {
							wantUsed = 200 * runtimeMountMiB
							wantMountUsage = 0
						}
						if vm.Disk.Total != 10000*runtimeMountMiB || vm.Disk.Used != wantUsed || vm.Disk.Free != vm.Disk.Total-wantUsed || len(vm.Disks) != 3 || vm.DiskStatusReason != "" || vm.DiskObservation.ObservedAt.IsZero() {
							t.Fatalf("real data disk omitted or alias counted twice: %+v", vm)
						}
						view := f.monitor.currentModeReadState().VMs()[0]
						if view.DiskTotal() != vm.Disk.Total || view.DiskUsed() != wantUsed || !reflect.DeepEqual(previousVMFromView(view).Disks, vm.Disks) {
							t.Fatal("canonical state lost data mount or changed observations")
						}
						front := f.monitor.buildBroadcastFrontendStateFromSnapshot(models.StateSnapshot{VMs: []models.VM{vm}}, f.monitor.mockModeFence.begin())
						wire, err := json.Marshal(front.Resources)
						var served []struct {
							Disk    *models.ResourceMetricFrontend
							Proxmox struct{ Disks []unifiedresources.DiskInfo }
						}
						if err != nil || json.Unmarshal(wire, &served) != nil || len(served) != 1 || served[0].Disk == nil || served[0].Disk.Total == nil || *served[0].Disk.Total != vm.Disk.Total || served[0].Disk.Used == nil || *served[0].Disk.Used != wantUsed || len(served[0].Proxmox.Disks) != 3 {
							t.Fatalf("served disk differs from poll: %s / %v", wire, err)
						}
						for _, disk := range served[0].Proxmox.Disks {
							if disk.Mountpoint == mount && (disk.Device != "/dev/vdb1" || disk.Filesystem != fsType || disk.Usage != wantMountUsage) {
								t.Fatalf("data mount lost its own identity/usage: %+v", disk)
							}
						}
						active := f.monitor.alertManager.GetActiveAlerts()
						if phase < 2 {
							if len(active) != 1 || active[0].Value != 98 {
								t.Fatalf("real near-full mount failed ordinary alerting: %+v", active)
							}
							if phase == 0 {
								breachID = active[0].ID
							} else if active[0].ID != breachID {
								t.Fatal("alias reordering replaced the breach")
							}
						} else if len(active) != 0 {
							t.Fatal("real zero data usage failed to clear the alert")
						}
					}
					if f.fsCalls.Load() != 3 {
						t.Fatalf("unexpected filesystem read count: %d", f.fsCalls.Load())
					}
				})
			}
		}
	}
}
