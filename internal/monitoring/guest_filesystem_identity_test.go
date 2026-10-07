package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	windowsVolumeC       = "\\\\?\\Volume{c65410ae-abf1-4829-8d8e-81d4b7581949}\\"
	windowsVolumeD       = "\\\\?\\Volume{5743c199-8613-4953-94a2-574d75a27bfc}\\"
	windowsPhysicalDrive = "\\\\.\\PhysicalDrive0"
)

func TestVMFilesystemCapacityIdentity(t *testing.T) {
	windows := func(name, mount string, used uint64) proxmox.VMFileSystem {
		return proxmox.VMFileSystem{Name: name, Mountpoint: mount, Type: "NTFS", Disk: windowsPhysicalDrive, TotalBytes: 1000, UsedBytes: used}
	}
	linux := func(device, mount string) proxmox.VMFileSystem {
		return proxmox.VMFileSystem{Name: device, Mountpoint: mount, Type: "ext4", Disk: device, TotalBytes: 1000, UsedBytes: 400}
	}
	for _, tc := range []struct {
		name        string
		fs          []proxmox.VMFileSystem
		total, used uint64
	}{
		{"equal Windows partitions", []proxmox.VMFileSystem{windows(windowsVolumeC, "C:\\", 100), windows(windowsVolumeD, "D:\\", 900)}, 2000, 1000},
		{"no GUID distinct mount paths", []proxmox.VMFileSystem{windows("", "C:\\", 100), windows("", "D:\\", 900)}, 2000, 1000},
		{"one GUID multiple mount paths", []proxmox.VMFileSystem{windows(windowsVolumeC, "C:\\", 400), windows(strings.ToUpper(windowsVolumeC), "D:\\mount\\", 400)}, 1000, 400},
		{"same mount case slash aliases", []proxmox.VMFileSystem{windows("", "C:\\", 0), windows("", "c:/", 0)}, 1000, 0},
		{"Linux bind mount", []proxmox.VMFileSystem{linux("/dev/vda1", "/"), linux("/dev/vda1", "/bind")}, 1000, 400},
		{"Linux distinct devices", []proxmox.VMFileSystem{linux("/dev/vda1", "/"), linux("/dev/vda2", "/data")}, 2000, 800},
		{"Windows identity does not hide a Linux peer", []proxmox.VMFileSystem{windows(windowsVolumeC, "C:\\", 100), linux(windowsPhysicalDrive, "/data")}, 2000, 500},
		{"different GUID same mount label", []proxmox.VMFileSystem{windows(windowsVolumeC, "Data", 100), windows(windowsVolumeD, "Data", 900)}, 2000, 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, tc.fs)
			if summary.totalBytes != tc.total || summary.usedBytes != tc.used || len(summary.individualDisks) != len(tc.fs) || summary.invalidBytes {
				t.Fatalf("filesystem identity changed capacity or visible mount rows: %+v, want %d/%d and %d rows", summary, tc.total, tc.used, len(tc.fs))
			}
			for i, disk := range summary.individualDisks {
				if disk.Mountpoint != tc.fs[i].Mountpoint || disk.Device != tc.fs[i].Disk || disk.Used != int64(tc.fs[i].UsedBytes) {
					t.Fatalf("capacity key leaked into row metadata: %+v", disk)
				}
			}
		})
	}
}

func TestVMFilesystemDistinctWindowsVolumesRetainOverflowFence(t *testing.T) {
	fs := []proxmox.VMFileSystem{
		{Name: windowsVolumeC, Mountpoint: "C:\\", Type: "NTFS", Disk: windowsPhysicalDrive, TotalBytes: math.MaxInt64},
		{Name: windowsVolumeD, Mountpoint: "D:\\", Type: "NTFS", Disk: windowsPhysicalDrive, TotalBytes: math.MaxInt64},
	}
	summary := (&Monitor{}).summarizeVMFSInfo("fixture", proxmox.ClusterResource{}, fs)
	if !summary.invalidBytes || summary.totalBytes != 0 || len(summary.individualDisks) != 0 {
		t.Fatalf("deduplication concealed an unrepresentable aggregate: %+v", summary)
	}
}

// Synthetic existing QGA wire shapes (including issue #1319's volume GUID and
// PhysicalDrive form), not a reproduction of #2619's native failure. No Pulse
// host agent participates: ordinary QEMU polling must publish every volume.
func TestVMFilesystemWindowsVolumePollingHistory(t *testing.T) {
	var poll, fsCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			fmt.Fprint(w, "{\"data\":{}}")
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprint(w, "{\"data\":{\"status\":\"running\",\"agent\":1,\"maxmem\":1000,\"mem\":400,\"meminfo\":{\"total\":1000,\"available\":600}}}")
		case strings.HasSuffix(r.URL.Path, "/agent/get-fsinfo"):
			fsCalls.Add(1)
			used := 100
			if poll.Load() == 2 {
				used = 0
			}
			row := func(name, mount string, used int) map[string]any {
				return map[string]any{"name": name, "mountpoint": mount, "type": "NTFS", "total-bytes": 1000, "used-bytes": used, "disk": []any{map[string]any{"dev": windowsPhysicalDrive, "serial": "QM00005"}}}
			}
			rows := []any{row(windowsVolumeC, "C:\\", used), row(strings.ToUpper(windowsVolumeC), "C:\\mount\\", used)}
			if poll.Load() == 0 {
				rows = append(rows, row(windowsVolumeD, "D:\\", 900))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"result": rows}})
		case strings.HasSuffix(r.URL.Path, "/agent/network-get-interfaces"):
			fmt.Fprint(w, "{\"data\":{\"result\":[]}}")
		case strings.HasSuffix(r.URL.Path, "/agent/get-osinfo"):
			fmt.Fprint(w, "{\"data\":{\"name\":\"Microsoft Windows Server 2022\",\"version\":\"fixture\"}}")
		case strings.HasSuffix(r.URL.Path, "/agent/info"):
			fmt.Fprint(w, "{\"data\":{\"result\":{\"version\":\"fixture\"}}}")
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
	m.rateTracker = NewRateTracker()
	m.guestMetadataLimiter = make(map[string]time.Time)
	registry := unifiedresources.NewRegistry(nil)
	m.resourceStore = unifiedresources.NewMonitorAdapter(registry)
	res := proxmox.ClusterResource{Type: "qemu", Node: "node", Name: "windows", VMID: 105, Status: "running", MaxMem: 1000, Mem: 400, MaxDisk: 2000}
	id := makeGuestID("volumes", "node", 105)
	var previous *models.VM
	for phase, want := range []struct {
		total, used int64
		usage       float64
		rows        int
	}{{2000, 1000, 50, 3}, {1000, 100, 10, 2}, {1000, 0, 0, 2}} {
		if phase > 0 {
			time.Sleep(1100 * time.Millisecond)
		}
		poll.Store(int32(phase))
		scope := m.mockModeFence.begin()
		vm, _, _, _, _, ok := m.buildVMFromClusterResource(context.Background(), "volumes", res, client, id, nil, previous)
		if !ok || vm.Disk.Total != want.total || vm.Disk.Used != want.used || vm.Disk.Free != want.total-want.used || vm.Disk.Usage != want.usage || len(vm.Disks) != want.rows || vm.DiskStatusReason != "" {
			t.Fatalf("poll %d did not publish complete current volume usage: %+v", phase, vm)
		}
		m.recordGuestMetrics([]models.VM{vm}, nil, time.Now().Add(-time.Second))
		m.updateResourceStore(models.StateSnapshot{VMs: []models.VM{vm}, LastUpdate: vm.LastSeen}, scope)
		views := m.GetUnifiedReadState().VMs()
		if len(views) != 1 || views[0].DiskTotal() != want.total || views[0].DiskUsed() != want.used || len(views[0].Disks()) != want.rows {
			t.Fatalf("poll %d lost complete volume capacity at the canonical read boundary", phase)
		}
		next := previousVMFromView(views[0])
		previous = &next
	}
	points := []float64{50, 10, 0}
	memory := m.metricsHistory.GetGuestMetrics(id, "disk", time.Hour)
	stored := guestHistoryStoredPoints(t, m, "vm", id, "disk")
	if len(memory) != len(points) || len(stored) != len(points) {
		t.Fatalf("volume lifecycle lost History: memory=%+v stored=%+v", memory, stored)
	}
	for i, want := range points {
		if memory[i].Value != want || stored[i].Value != want {
			t.Fatalf("History[%d] = %v/%v, want %v", i, memory[i].Value, stored[i].Value, want)
		}
	}
	if fsCalls.Load() != 3 {
		t.Fatalf("filesystem read was repeated or skipped: %d commands", fsCalls.Load())
	}
}
