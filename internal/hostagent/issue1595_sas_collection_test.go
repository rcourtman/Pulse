//go:build !windows

package hostagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
)

type issue1595TopologyFixture struct {
	Node        string `json:"node"`
	Instance    string `json:"instance"`
	AgentID     string `json:"agentId"`
	Model       string `json:"model"`
	SizeBytes   int64  `json:"sizeBytes"`
	Controllers []struct {
		ID    string `json:"id"`
		Pool  string `json:"pool"`
		Disks []struct {
			Device         string `json:"device"`
			Target         string `json:"target"`
			Serial         string `json:"serial"`
			ProviderSerial string `json:"providerSerial"`
			Temperature    int    `json:"temperature"`
			ReadBytes      uint64 `json:"readBytes"`
			WriteBytes     uint64 `json:"writeBytes"`
			IOTimeMs       uint64 `json:"ioTimeMs"`
		} `json:"disks"`
	} `json:"controllers"`
}

func loadIssue1595TopologyFixture(t *testing.T) issue1595TopologyFixture {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "issue1595_sas_topology.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture issue1595TopologyFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return fixture
}

func TestIssue1595CollectSMARTLocalPreservesTwentyFourSASDisksAcrossTwoHBAs(t *testing.T) {
	fixture := loadIssue1595TopologyFixture(t)

	type diskFixture struct {
		controller  string
		target      string
		serial      string
		temperature int
	}
	byDevice := make(map[string]diskFixture)
	var scan strings.Builder
	var entries []os.DirEntry
	for _, controller := range fixture.Controllers {
		for _, disk := range controller.Disks {
			byDevice[disk.Device] = diskFixture{
				controller:  controller.ID,
				target:      disk.Target,
				serial:      disk.Serial,
				temperature: disk.Temperature,
			}
			fmt.Fprintf(&scan, "/dev/%s -d scsi # synthetic issue #1595 SAS disk\n", disk.Device)
			entries = append(entries, fakeDirEntry{name: disk.Device})
		}
	}
	if len(byDevice) != 24 {
		t.Fatalf("fixture disk count = %d, want 24", len(byDevice))
	}

	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	origRun := smartRunCommandOutput
	origLookPath := execLookPath
	origConcurrency := smartCollectionConcurrency
	origThreshold := smartCollectionParallelThreshold
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
		smartRunCommandOutput = origRun
		execLookPath = origLookPath
		smartCollectionConcurrency = origConcurrency
		smartCollectionParallelThreshold = origThreshold
	})

	runtimeGOOS = "linux"
	smartCollectionConcurrency = 6
	smartCollectionParallelThreshold = 12
	readDir = func(path string) ([]os.DirEntry, error) {
		if path == "/sys/block" {
			return entries, nil
		}
		return nil, fs.ErrNotExist
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		device := issue1595DeviceFromSysfsPath(path)
		if device == "" {
			return nil, fs.ErrNotExist
		}
		switch filepath.Base(path) {
		case "size":
			return []byte(fmt.Sprintf("%d\n", fixture.SizeBytes/512)), nil
		case "protocol":
			return []byte("SAS\n"), nil
		case "rotational":
			return []byte("1\n"), nil
		case "model":
			return []byte(fixture.Model + "\n"), nil
		default:
			return nil, fs.ErrNotExist
		}
	}
	smartctlEvalSymlinks = func(path string) (string, error) {
		device := issue1595DeviceFromSysfsPath(path)
		disk, ok := byDevice[device]
		if !ok {
			return "", fs.ErrNotExist
		}
		if strings.HasSuffix(path, "/device/subsystem") {
			return "/sys/bus/scsi", nil
		}
		if strings.HasSuffix(path, "/device") || path == "/sys/block/"+device {
			host := strings.Split(disk.target, ":")[0]
			return fmt.Sprintf(
				"/sys/devices/pci0000:00/%s/host%s/target%s/%s/block/%s",
				disk.controller,
				host,
				strings.Join(strings.Split(disk.target, ":")[:3], ":"),
				disk.target,
				device,
			), nil
		}
		return "", fs.ErrNotExist
	}
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var active atomic.Int32
	var maxActive atomic.Int32
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte(scan.String()), nil
		}
		device := strings.TrimPrefix(args[len(args)-1], "/dev/")
		disk, ok := byDevice[device]
		if !ok {
			return nil, fmt.Errorf("unexpected SMART target %q", device)
		}
		current := active.Add(1)
		for {
			previous := maxActive.Load()
			if current <= previous || maxActive.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(15 * time.Millisecond):
		}
		return json.Marshal(map[string]any{
			"device": map[string]any{
				"name":     "/dev/" + device,
				"type":     "scsi",
				"protocol": "SAS",
			},
			"model_name":    fixture.Model,
			"serial_number": disk.serial,
			"user_capacity": map[string]any{"bytes": fixture.SizeBytes},
			"smart_status":  map[string]any{"passed": true},
			"temperature":   map[string]any{"current": disk.temperature},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results, err := CollectSMARTLocal(ctx, nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal() error = %v", err)
	}
	if len(results) != 24 {
		t.Fatalf("collected disk count = %d, want 24", len(results))
	}
	if maxActive.Load() < 2 {
		t.Fatalf("SMART collection did not use bounded concurrency; max active = %d", maxActive.Load())
	}
	for _, result := range results {
		want := byDevice[result.Device]
		if result.Serial != want.serial || result.Type != "sas" || result.Temperature != want.temperature {
			t.Fatalf("disk %s lost SAS SMART identity: %+v", result.Device, result)
		}
		if result.Controller != want.controller || result.Target != want.target {
			t.Fatalf("disk %s topology = %q/%q, want %q/%q", result.Device, result.Controller, result.Target, want.controller, want.target)
		}
		if result.Collection == nil ||
			result.Collection.Serial.State != diskinventory.FieldAvailable ||
			result.Collection.Temperature.State != diskinventory.FieldAvailable ||
			result.Collection.Controller.State != diskinventory.FieldAvailable {
			t.Fatalf("disk %s collection status = %+v", result.Device, result.Collection)
		}
	}
}

func TestIssue1595DiskIOAssociationMarksSharedControllerTargetsUnsupported(t *testing.T) {
	smart := []agentshost.DiskSMART{
		{Device: "sda [megaraid,0]", Controller: "sda", Target: "megaraid,0"},
		{Device: "sda [megaraid,1]", Controller: "sda", Target: "megaraid,1"},
	}
	annotateSMARTWithDiskIO(smart, []agentshost.DiskIO{{Device: "sda", ReadBytes: 100}})

	for _, disk := range smart {
		if disk.IO != nil {
			t.Fatalf("controller member %s received aggregate I/O counters", disk.Target)
		}
		if disk.Collection == nil || disk.Collection.IO.State != diskinventory.FieldUnsupported {
			t.Fatalf("controller member %s I/O status = %+v, want unsupported", disk.Target, disk.Collection)
		}
	}
}

func TestLinuxBlockDeviceTopologyPreservesDirectNVMeAndSATAPCIController(t *testing.T) {
	origEvalSymlinks := smartctlEvalSymlinks
	t.Cleanup(func() { smartctlEvalSymlinks = origEvalSymlinks })

	paths := map[string]string{
		"/sys/block/sda/device":     "/sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0",
		"/sys/block/nvme0n1/device": "/sys/devices/pci0000:00/0000:00:1d.0/nvme/nvme0/nvme0n1",
	}
	smartctlEvalSymlinks = func(path string) (string, error) {
		if resolved, ok := paths[path]; ok {
			return resolved, nil
		}
		return "", fs.ErrNotExist
	}

	if controller, target := linuxBlockDeviceTopology("sda"); controller != "0000:00:17.0" || target != "0:0:0:0" {
		t.Fatalf("SATA topology = %q/%q, want PCI controller and HCTL", controller, target)
	}
	if controller, target := linuxBlockDeviceTopology("nvme0n1"); controller != "0000:00:1d.0" || target != "" {
		t.Fatalf("NVMe topology = %q/%q, want PCI controller without HCTL", controller, target)
	}
}

func issue1595DeviceFromSysfsPath(path string) string {
	const prefix = "/sys/block/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	remainder := strings.TrimPrefix(path, prefix)
	if index := strings.IndexByte(remainder, '/'); index >= 0 {
		remainder = remainder[:index]
	}
	return remainder
}
