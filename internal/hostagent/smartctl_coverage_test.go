package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// mockLsblkJSON builds a JSON response matching the format from
// `lsblk -J -d -o NAME,TYPE,TRAN,MODEL,VENDOR,SUBSYSTEMS`.
func mockLsblkJSON(devices ...lsblkDevice) []byte {
	data := lsblkJSON{Blockdevices: devices}
	out, _ := json.Marshal(data)
	return out
}

type fakeDirEntry struct {
	name string
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return false }
func (e fakeDirEntry) Type() fs.FileMode          { return 0 }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func forceLinuxLSBLKFallback(t *testing.T) {
	t.Helper()

	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
	})

	runtimeGOOS = "linux"
	readDir = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("sysfs unavailable")
	}
	smartctlReadFile = func(string) ([]byte, error) { return nil, fs.ErrNotExist }
	smartctlEvalSymlinks = func(string) (string, error) { return "", fs.ErrNotExist }
}

func TestListBlockDevices(t *testing.T) {
	origRun := smartRunCommandOutput
	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	t.Cleanup(func() { smartRunCommandOutput = origRun; runtimeGOOS = origGOOS })
	t.Cleanup(func() {
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
	})

	runtimeGOOS = "linux"
	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{name: "sda"},
			fakeDirEntry{name: "nvme0n1"},
			fakeDirEntry{name: "zd0"},
		}, nil
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/sys/block/sda/device/vendor":
			return []byte("ATA\n"), nil
		case "/sys/block/sda/device/model":
			return []byte("Samsung SSD 870 EVO\n"), nil
		case "/sys/block/nvme0n1/device/model":
			return []byte("Samsung SSD 980\n"), nil
		default:
			return nil, fs.ErrNotExist
		}
	}
	smartctlEvalSymlinks = func(path string) (string, error) {
		switch path {
		case "/sys/block/sda":
			return "/sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda", nil
		case "/sys/block/sda/device/subsystem":
			return "/sys/bus/scsi", nil
		case "/sys/block/nvme0n1":
			return "/sys/devices/pci0000:00/0000:00:1d.0/nvme/nvme0/nvme0n1", nil
		case "/sys/block/nvme0n1/device/subsystem":
			return "/sys/bus/nvme", nil
		case "/sys/block/zd0":
			return "/sys/devices/virtual/block/zd0", nil
		case "/sys/block/zd0/device/subsystem":
			return "", fs.ErrNotExist
		default:
			return "", fs.ErrNotExist
		}
	}
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("lsblk should not be called when sysfs enumeration succeeds")
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 2 || devices[0] != "/dev/sda" || devices[1] != "/dev/nvme0n1" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestListBlockDevicesFreeBSD(t *testing.T) {
	origRun := smartRunCommandOutput
	origGOOS := runtimeGOOS
	origReadDir := readDir
	t.Cleanup(func() { smartRunCommandOutput = origRun; runtimeGOOS = origGOOS; readDir = origReadDir })

	runtimeGOOS = "freebsd"
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "sysctl" {
			return nil, errors.New("unexpected command: " + name)
		}
		return []byte("ada0 da0 nvd0\n"), nil
	}
	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{name: "ada0"},
			fakeDirEntry{name: "da0"},
			fakeDirEntry{name: "nvd0"},
			fakeDirEntry{name: "ada0p1"},
		}, nil
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 3 || devices[0] != "/dev/ada0" || devices[1] != "/dev/da0" || devices[2] != "/dev/nvd0" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestListBlockDevicesFreeBSDWithExcludes(t *testing.T) {
	origRun := smartRunCommandOutput
	origGOOS := runtimeGOOS
	origReadDir := readDir
	t.Cleanup(func() { smartRunCommandOutput = origRun; runtimeGOOS = origGOOS; readDir = origReadDir })

	runtimeGOOS = "freebsd"
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ada0 da0 nvd0\n"), nil
	}
	readDir = func(string) ([]os.DirEntry, error) {
		return nil, nil
	}

	devices, err := listBlockDevices(context.Background(), []string{"da0"})
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 2 || devices[0] != "/dev/ada0" || devices[1] != "/dev/nvd0" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestListBlockDevicesFreeBSDFallsBackToDevEntries(t *testing.T) {
	origRun := smartRunCommandOutput
	origReadDir := readDir
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		readDir = origReadDir
		runtimeGOOS = origGOOS
	})

	runtimeGOOS = "freebsd"
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("\n"), nil
	}
	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{name: "ada0"},
			fakeDirEntry{name: "da0"},
			fakeDirEntry{name: "nvd0"},
			fakeDirEntry{name: "nda0"},
			fakeDirEntry{name: "ada0p2"},
			fakeDirEntry{name: "pass0"},
		}, nil
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 4 || devices[0] != "/dev/ada0" || devices[1] != "/dev/da0" || devices[2] != "/dev/nda0" || devices[3] != "/dev/nvd0" {
		t.Fatalf("unexpected devices from /dev fallback: %#v", devices)
	}
}

func TestListBlockDevicesError(t *testing.T) {
	origRun := smartRunCommandOutput
	forceLinuxLSBLKFallback(t)
	t.Cleanup(func() { smartRunCommandOutput = origRun })
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	if _, err := listBlockDevices(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParseSmartctlScanOpenTargets(t *testing.T) {
	output := []byte(strings.Join([]string{
		`/dev/sda -d sat # /dev/sda, ATA device`,
		`/dev/sda # duplicate plain path should be ignored when typed target exists`,
		`/dev/bsg/sssraid0 -d sssraid,0,1 # controller-backed disk`,
		`/dev/sdb -d megaraid,0 # megaraid slot 0`,
		`/dev/sdb -d megaraid,1 # megaraid slot 1`,
		`/dev/sdc # plain disk`,
		``,
	}, "\n"))

	targets := parseSmartctlScanOpenTargets(output, []string{"sdc"})
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %#v", targets)
	}
	if targets[0].Path != "/dev/sda" || targets[0].DeviceType != "sat" {
		t.Fatalf("unexpected first target: %#v", targets[0])
	}
	if targets[1].Path != "/dev/bsg/sssraid0" || targets[1].DeviceType != "sssraid,0,1" {
		t.Fatalf("unexpected second target: %#v", targets[1])
	}
	if targets[2].DeviceType != "megaraid,0" || targets[3].DeviceType != "megaraid,1" {
		t.Fatalf("expected megaraid targets to be preserved separately, got %#v", targets)
	}
}

func TestCollectSMARTLocalUsesSmartctlScanOpenTargetsOnLinux(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		runtimeGOOS = origGOOS
	})

	runtimeGOOS = "linux"
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	var seenArgs [][]string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		seenArgs = append(seenArgs, append([]string(nil), args...))

		if len(args) == 1 && args[0] == "--scan-open" {
			return []byte("/dev/sda -d megaraid,7 # RAID-backed SSD\n"), nil
		}

		payload := smartctlJSON{
			ModelName:    "RAID SSD",
			SerialNumber: "raid-ssd-1",
		}
		payload.Device.Protocol = "NVMe"
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}
		payload.NVMeSmartHealthInformationLog = &nvmeSmartHealthInformationLogJSON{
			PercentageUsed: 6,
			AvailableSpare: 94,
		}
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %#v", result)
	}
	if result[0].Device != "sda [megaraid,7]" {
		t.Fatalf("unexpected device label: %#v", result[0])
	}
	if result[0].Attributes == nil || result[0].Attributes.PercentageUsed == nil || *result[0].Attributes.PercentageUsed != 6 {
		t.Fatalf("expected percentage_used SMART data, got %#v", result[0].Attributes)
	}

	if len(seenArgs) < 2 {
		t.Fatalf("expected scan and probe calls, got %v", seenArgs)
	}
	probe := seenArgs[1]
	if len(probe) < 3 || probe[0] != "-d" || probe[1] != "megaraid,7" || probe[len(probe)-1] != "/dev/sda" {
		t.Fatalf("expected typed smartctl probe, got %v", probe)
	}
}

func TestListBlockDevicesLinuxFallsBackToLSBLKWhenSysfsUnavailable(t *testing.T) {
	origRun := smartRunCommandOutput
	forceLinuxLSBLKFallback(t)
	t.Cleanup(func() { smartRunCommandOutput = origRun })

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "lsblk" {
			return nil, errors.New("unexpected command")
		}
		return mockLsblkJSON(
			lsblkDevice{Name: "sda", Type: "disk", Model: "Samsung SSD 870 EVO", Vendor: "ATA"},
			lsblkDevice{Name: "sr0", Type: "rom"},
		), nil
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 1 || devices[0] != "/dev/sda" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestListBlockDevicesLinuxSysfsSkipsVirtualMetadata(t *testing.T) {
	origRun := smartRunCommandOutput
	origGOOS := runtimeGOOS
	origReadDir := readDir
	origReadFile := smartctlReadFile
	origEvalSymlinks := smartctlEvalSymlinks
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		runtimeGOOS = origGOOS
		readDir = origReadDir
		smartctlReadFile = origReadFile
		smartctlEvalSymlinks = origEvalSymlinks
	})

	runtimeGOOS = "linux"
	readDir = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{
			fakeDirEntry{name: "sda"},
			fakeDirEntry{name: "sdb"},
		}, nil
	}
	smartctlReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/sys/block/sda/device/vendor":
			return []byte("ATA\n"), nil
		case "/sys/block/sda/device/model":
			return []byte("Samsung SSD 870 EVO\n"), nil
		case "/sys/block/sdb/device/vendor":
			return []byte("VMware\n"), nil
		case "/sys/block/sdb/device/model":
			return []byte("Virtual disk\n"), nil
		default:
			return nil, fs.ErrNotExist
		}
	}
	smartctlEvalSymlinks = func(path string) (string, error) {
		switch path {
		case "/sys/block/sda":
			return "/sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda", nil
		case "/sys/block/sda/device/subsystem":
			return "/sys/bus/scsi", nil
		case "/sys/block/sdb":
			return "/sys/devices/pci0000:00/0000:00:18.0/host1/target1:0:0/1:0:0:0/block/sdb", nil
		case "/sys/block/sdb/device/subsystem":
			return "/sys/bus/scsi", nil
		default:
			return "", fs.ErrNotExist
		}
	}
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("lsblk should not be called when sysfs enumeration succeeds")
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 1 || devices[0] != "/dev/sda" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestDefaultRunCommandOutput(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })
	smartRunCommandOutput = origRun

	if _, err := smartRunCommandOutput(context.Background(), "sh", "-c", "printf ''"); err != nil {
		t.Fatalf("expected default smartRunCommandOutput to work, got %v", err)
	}
}

func TestDefaultRunCommandOutputRejectsOversizedOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })
	smartRunCommandOutput = origRun

	cmd := fmt.Sprintf("head -c %d /dev/zero", maxCommandOutputBytes+1)
	_, err := smartRunCommandOutput(context.Background(), "sh", "-c", cmd)
	if err == nil {
		t.Fatal("expected oversized output error")
	}
	if !errors.Is(err, errCommandOutputTooLarge) {
		t.Fatalf("expected oversized output error, got %v", err)
	}
}

func TestCollectLocalNoDevices(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return mockLsblkJSON(), nil // empty device list
		}
		return nil, errors.New("unexpected command")
	}

	result, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for no devices")
	}
}

func TestCollectSMARTLocalListDevicesError(t *testing.T) {
	origRun := smartRunCommandOutput
	origReadDir := readDir
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		readDir = origReadDir
		runtimeGOOS = origGOOS
	})

	runtimeGOOS = "linux"

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("lsblk failed")
	}
	readDir = func(path string) ([]os.DirEntry, error) {
		return nil, errors.New("sysfs unavailable")
	}

	if _, err := CollectSMARTLocal(context.Background(), nil); err == nil {
		t.Fatalf("expected list error")
	}
}

func TestCollectSMARTLocalSkipsErrors(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return mockLsblkJSON(
				lsblkDevice{Name: "sda", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
				lsblkDevice{Name: "sdb", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
			), nil
		}
		if name == "smartctl" {
			device := args[len(args)-1]
			if strings.Contains(device, "sda") {
				return nil, errors.New("read error")
			}
			payload := smartctlJSON{
				ModelName:    "Model",
				SerialNumber: "Serial",
			}
			payload.Device.Protocol = "ATA"
			payload.SmartStatus = &struct {
				Passed bool `json:"passed"`
			}{Passed: true}
			payload.Temperature.Current = 30
			out, _ := json.Marshal(payload)
			return out, nil
		}
		return nil, errors.New("unexpected command")
	}

	result, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Device != "sdb" || !result[0].LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result[0])
	}
}

func TestCollectDeviceSMARTLookPathError(t *testing.T) {
	origLook := execLookPath
	t.Cleanup(func() { execLookPath = origLook })
	execLookPath = func(string) (string, error) { return "", errors.New("missing") }

	if _, err := collectDeviceSMART(context.Background(), "/dev/sda"); err == nil {
		t.Fatalf("expected lookpath error")
	}
}

func TestCollectDeviceSMARTOversizedOutput(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("%w (%d bytes)", errCommandOutputTooLarge, maxCommandOutputBytes)
	}

	if _, err := collectDeviceSMART(context.Background(), "/dev/sda"); !errors.Is(err, errCommandOutputTooLarge) {
		t.Fatalf("expected oversized output error, got %v", err)
	}
}

func TestCollectDeviceSMARTStandby(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "sh", "-c", "exit 2").Output()
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.Standby || result.Device != "sda" || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected standby result: %#v", result)
	}
}

func TestCollectDeviceSMARTExitErrorNoOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "sh", "-c", "exit 1").Output()
	}

	if _, err := collectDeviceSMART(context.Background(), "/dev/sda"); err == nil {
		t.Fatalf("expected error for exit code without output")
	}
}

func TestCollectDeviceSMARTExitErrorWithOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := `{"model_name":"Model","serial_number":"Serial","device":{"protocol":"ATA"},"smart_status":{"passed":false},"temperature":{"current":45}}`
		return exec.CommandContext(ctx, "sh", "-c", "echo '"+payload+"'; exit 1").Output()
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Health != "FAILED" || result.Temperature != 45 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCollectDeviceSMARTJSONError(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("{"), nil
	}

	if _, err := collectDeviceSMART(context.Background(), "/dev/sda"); err == nil {
		t.Fatalf("expected json error")
	}
}

func TestCollectDeviceSMARTMissingSmartStatusUsesUnknownHealth(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := `{"model_name":"Model","serial_number":"Serial","device":{"protocol":"ATA"},"temperature":{"current":40}}`
		return []byte(payload), nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Health != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN health, got %#v", result)
	}
}

func TestCollectDeviceSMARTNVMeTempFallback(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := smartctlJSON{}
		payload.Device.Protocol = "NVMe"
		payload.NVMeSmartHealthInformationLog = &nvmeSmartHealthInformationLogJSON{Temperature: 55}
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/nvme0n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Temperature != 55 || result.Type != "nvme" || result.Health != "PASSED" || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCollectDeviceSMARTFreeBSDAdaFallback(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
		runtimeGOOS = origGOOS
	})

	fixed := time.Date(2024, 4, 6, 7, 8, 9, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runtimeGOOS = "freebsd"

	var attempts [][]string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts = append(attempts, append([]string(nil), args...))

		payload := smartctlJSON{}
		payload.Device.Protocol = "ATA"
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}

		if len(args) >= 2 && args[0] == "-d" && args[1] == "sat" {
			payload.Temperature.Current = 37
		}

		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/ada0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Temperature != 37 || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if len(attempts[0]) < 2 || attempts[0][0] != "-d" || attempts[0][1] != "sat" {
		t.Fatalf("expected sat probe on first attempt, got %v", attempts[0])
	}
}

func TestCollectDeviceSMARTFreeBSDFalseStandbyFallsBackToUntypedProbe(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
		runtimeGOOS = origGOOS
	})

	fixed := time.Date(2024, 4, 7, 9, 10, 11, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runtimeGOOS = "freebsd"

	var attempts [][]string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts = append(attempts, append([]string(nil), args...))

		if len(args) >= 2 && args[0] == "-d" && args[1] == "sat" {
			return exec.CommandContext(ctx, "sh", "-c", "exit 3").Output()
		}

		payload := smartctlJSON{}
		payload.Device.Protocol = "ATA"
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}
		payload.Temperature.Current = 41
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/ada0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Standby || result.Temperature != 41 || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attempts))
	}
	if len(attempts[0]) < 2 || attempts[0][0] != "-d" || attempts[0][1] != "sat" {
		t.Fatalf("expected first probe to use -d sat, got %v", attempts[0])
	}
	if len(attempts[1]) < 1 || attempts[1][0] == "-d" {
		t.Fatalf("expected second probe to fall back to untyped smartctl, got %v", attempts[1])
	}
}

func TestCollectDeviceSMARTFreeBSDUsesSCTTemperatureFallback(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
		runtimeGOOS = origGOOS
	})

	fixed := time.Date(2024, 4, 7, 10, 30, 0, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runtimeGOOS = "freebsd"

	var attempts [][]string
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		attempts = append(attempts, append([]string(nil), args...))

		payload := smartctlJSON{}
		payload.Device.Protocol = "ATA"
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}

		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-l" && args[i+1] == "scttempsts" {
				payload.ATASCTStatus.Current.Value = 43
			}
		}

		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/ada0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Standby || result.Temperature != 43 || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected typed probe plus SCT retry, got %d attempts: %v", len(attempts), attempts)
	}
	if len(attempts[0]) < 2 || attempts[0][0] != "-d" || attempts[0][1] != "sat" {
		t.Fatalf("expected first probe to use -d sat, got %v", attempts[0])
	}
	if !strings.Contains(strings.Join(attempts[1], " "), "-l scttempsts") {
		t.Fatalf("expected second probe to request SCT temperature status, got %v", attempts[1])
	}
}

func TestCollectDeviceSMARTFreeBSDFallsBackToPlainTextSMARTOutput(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	origNow := timeNow
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
		runtimeGOOS = origGOOS
	})

	fixed := time.Date(2024, 4, 7, 10, 30, 0, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runtimeGOOS = "freebsd"

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`
=== START OF INFORMATION SECTION ===
Device Model:     WDC WD40EFRX
Serial Number:    WD-123
SMART overall-health self-assessment test result: PASSED
Current Temperature:                    38 C
`), nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/ada0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Standby || result.Temperature != 38 || result.Health != "PASSED" || !result.LastUpdated.Equal(fixed) {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Model != "WDC WD40EFRX" || result.Serial != "WD-123" {
		t.Fatalf("expected text fallback model/serial, got %#v", result)
	}
}

func TestCollectDeviceSMARTWWN(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := smartctlJSON{}
		payload.WWN.NAA = 5
		payload.WWN.OUI = 0xabc
		payload.WWN.ID = 0x1234
		payload.Device.Protocol = "SAS"
		payload.SmartStatus = &struct {
			Passed bool `json:"passed"`
		}{Passed: true}
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WWN != "5-abc-1234" {
		t.Fatalf("unexpected WWN: %q", result.WWN)
	}
}

func TestListBlockDevicesLinuxWithExcludes(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "lsblk" {
			return nil, errors.New("unexpected command")
		}
		return mockLsblkJSON(
			lsblkDevice{Name: "sda", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
			lsblkDevice{Name: "sdb", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
			lsblkDevice{Name: "sr0", Type: "rom"},
		), nil
	}

	devices, err := listBlockDevicesLinux(context.Background(), []string{"sdb"})
	if err != nil {
		t.Fatalf("listBlockDevicesLinux error: %v", err)
	}
	if len(devices) != 1 || devices[0] != "/dev/sda" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestLinuxSMARTSkipReason(t *testing.T) {
	tests := []struct {
		name   string
		device lsblkDevice
		skip   bool
		reason string
	}{
		{
			name:   "physical SATA disk passes",
			device: lsblkDevice{Name: "sda", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
			skip:   false,
		},
		{
			name:   "physical NVMe disk passes",
			device: lsblkDevice{Name: "nvme0n1", Type: "disk", Tran: "nvme", Subsystems: "block:nvme:pci"},
			skip:   false,
		},
		{
			name:   "ZFS zvol filtered by prefix",
			device: lsblkDevice{Name: "zd0", Type: "disk", Subsystems: "block:zfs"},
			skip:   true,
			reason: "virtual/logical device prefix",
		},
		{
			name:   "device-mapper filtered by prefix",
			device: lsblkDevice{Name: "dm-0", Type: "disk"},
			skip:   true,
			reason: "virtual/logical device prefix",
		},
		{
			name:   "virtio transport filtered",
			device: lsblkDevice{Name: "vda", Type: "disk", Tran: "virtio"},
			skip:   true,
			reason: "virtual/logical device prefix",
		},
		{
			name:   "QEMU model filtered by metadata",
			device: lsblkDevice{Name: "sda", Type: "disk", Tran: "sata", Vendor: "QEMU", Model: "HARDDISK"},
			skip:   true,
			reason: "virtual disk model/vendor signature",
		},
		{
			name:   "VMware model filtered by metadata",
			device: lsblkDevice{Name: "sda", Type: "disk", Model: "VMware Virtual S"},
			skip:   true,
			reason: "virtual disk model/vendor signature",
		},
		{
			name:   "virtio subsystem filtered",
			device: lsblkDevice{Name: "sda", Type: "disk", Subsystems: "block:virtio:pci"},
			skip:   true,
			reason: "virtual/logical subsystem",
		},
		{
			name:   "ZFS subsystem filtered",
			device: lsblkDevice{Name: "sda", Type: "disk", Subsystems: "block:zfs"},
			skip:   true,
			reason: "virtual/logical subsystem",
		},
		{
			name:   "partition type filtered",
			device: lsblkDevice{Name: "sda1", Type: "part"},
			skip:   true,
			reason: "not a whole disk",
		},
		{
			name:   "loop device filtered by prefix",
			device: lsblkDevice{Name: "loop0", Type: "disk"},
			skip:   true,
			reason: "virtual/logical device prefix",
		},
		{
			name:   "md RAID filtered by prefix",
			device: lsblkDevice{Name: "md0", Type: "disk"},
			skip:   true,
			reason: "virtual/logical device prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := linuxSMARTSkipReason(tt.device)
			if tt.skip && reason == "" {
				t.Errorf("expected device to be skipped, but got no reason")
			}
			if !tt.skip && reason != "" {
				t.Errorf("expected device to pass, but got skip reason: %s", reason)
			}
			if tt.skip && reason != tt.reason {
				t.Errorf("expected reason %q, got %q", tt.reason, reason)
			}
		})
	}
}

func TestListBlockDevicesLinuxFiltersVirtualDevices(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return mockLsblkJSON(
			lsblkDevice{Name: "sda", Type: "disk", Tran: "sata", Subsystems: "block:scsi:pci"},
			lsblkDevice{Name: "zd0", Type: "disk", Subsystems: "block:zfs"},
			lsblkDevice{Name: "dm-0", Type: "disk"},
			lsblkDevice{Name: "vda", Type: "disk", Tran: "virtio"},
			lsblkDevice{Name: "nvme0n1", Type: "disk", Tran: "nvme", Subsystems: "block:nvme:pci"},
			lsblkDevice{Name: "sdb", Type: "disk", Vendor: "QEMU", Model: "HARDDISK"},
		), nil
	}

	devices, err := listBlockDevicesLinux(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevicesLinux error: %v", err)
	}
	if len(devices) != 2 || devices[0] != "/dev/sda" || devices[1] != "/dev/nvme0n1" {
		t.Fatalf("expected only physical disks, got: %#v", devices)
	}
}

func TestCollectDeviceSMARTNoDataReturnsNil(t *testing.T) {
	origRun := smartRunCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Device that returns valid JSON but no useful SMART data
		payload := `{"device":{"protocol":"ATA"},"model_name":"Virtual Disk"}`
		return []byte(payload), nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if !errors.Is(err, errSMARTDataUnavailable) {
		t.Fatalf("expected errSMARTDataUnavailable, got err=%v result=%#v", err, result)
	}
	if result != nil {
		t.Fatalf("expected nil result for device with no SMART data, got %#v", result)
	}
}

func TestListBlockDevicesFreeBSDError(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() { smartRunCommandOutput = origRun })

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "sysctl" {
			return nil, errors.New("unexpected command")
		}
		return nil, errors.New("sysctl failed")
	}

	if _, err := listBlockDevicesFreeBSD(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestMatchesDeviceExcludePrefixNoMatchFallsThrough(t *testing.T) {
	matched := matchesDeviceExclude("sda", "/dev/sda", []string{"nvme*", "vda"})
	if matched {
		t.Fatalf("expected no match")
	}
}
