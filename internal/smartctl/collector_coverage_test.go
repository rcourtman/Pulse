package smartctl

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func mustMarshalLSBLK(t *testing.T, devices []lsblkDevice) []byte {
	t.Helper()

	out, err := json.Marshal(lsblkJSON{Blockdevices: devices})
	if err != nil {
		t.Fatalf("marshal lsblk json: %v", err)
	}
	return out
}

func TestListBlockDevices(t *testing.T) {
	origRun := runCommandOutput
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runCommandOutput = origRun; runtimeGOOS = origGOOS })

	runtimeGOOS = "linux"
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "lsblk" {
			return nil, errors.New("unexpected command")
		}
		return mustMarshalLSBLK(t, []lsblkDevice{
			{Name: "sda", Type: "disk", Model: "Samsung SSD 870 EVO", Vendor: "ATA"},
			{Name: "sda1", Type: "part"},
			{Name: "sr0", Type: "rom"},
			{Name: "nvme0n1", Type: "disk", Model: "Samsung SSD 980", Vendor: "Samsung"},
		}), nil
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
	origRun := runCommandOutput
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runCommandOutput = origRun; runtimeGOOS = origGOOS })

	runtimeGOOS = "freebsd"
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "sysctl" {
			return nil, errors.New("unexpected command: " + name)
		}
		return []byte("ada0 da0 nvd0\n"), nil
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
	origRun := runCommandOutput
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runCommandOutput = origRun; runtimeGOOS = origGOOS })

	runtimeGOOS = "freebsd"
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ada0 da0 nvd0\n"), nil
	}

	devices, err := listBlockDevices(context.Background(), []string{"da0"})
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 2 || devices[0] != "/dev/ada0" || devices[1] != "/dev/nvd0" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestListBlockDevicesError(t *testing.T) {
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	if _, err := listBlockDevices(context.Background(), nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDefaultRunCommandOutput(t *testing.T) {
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })
	runCommandOutput = origRun

	if _, err := runCommandOutput(context.Background(), "sh", "-c", "printf ''"); err != nil {
		t.Fatalf("expected default runCommandOutput to work, got %v", err)
	}
}

func TestCollectLocalNoDevices(t *testing.T) {
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return mustMarshalLSBLK(t, nil), nil
		}
		return nil, errors.New("unexpected command")
	}

	result, err := CollectLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectLocal error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for no devices")
	}
}

func TestCollectLocalListDevicesError(t *testing.T) {
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("lsblk failed")
	}

	if _, err := CollectLocal(context.Background(), nil); err == nil {
		t.Fatalf("expected list error")
	}
}

func TestCollectLocalSkipsErrors(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return mustMarshalLSBLK(t, []lsblkDevice{
				{Name: "sda", Type: "disk", Model: "Samsung SSD 870 EVO", Vendor: "ATA"},
				{Name: "sdb", Type: "disk", Model: "Samsung SSD 980", Vendor: "Samsung"},
			}), nil
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

	result, err := CollectLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectLocal error: %v", err)
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

func TestCollectDeviceSMARTStandby(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := runCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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

	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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

	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("{"), nil
	}

	if _, err := collectDeviceSMART(context.Background(), "/dev/sda"); err == nil {
		t.Fatalf("expected json error")
	}
}

func TestCollectDeviceSMARTNVMeTempFallback(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	origNow := timeNow
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
	})

	fixed := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := smartctlJSON{}
		payload.Device.Protocol = "NVMe"
		payload.NVMeSmartHealthInformationLog.Temperature = 55
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
	origRun := runCommandOutput
	origLook := execLookPath
	origNow := timeNow
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
		timeNow = origNow
		runtimeGOOS = origGOOS
	})

	fixed := time.Date(2024, 4, 6, 7, 8, 9, 0, time.UTC)
	timeNow = func() time.Time { return fixed }
	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runtimeGOOS = "freebsd"

	var attempts [][]string
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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
	if len(attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(attempts))
	}
	if len(attempts[0]) >= 2 && attempts[0][0] == "-d" {
		t.Fatalf("expected first attempt without explicit device type, got %v", attempts[0])
	}
	if len(attempts[1]) < 2 || attempts[1][0] != "-d" || attempts[1][1] != "sat" {
		t.Fatalf("expected sat fallback on second attempt, got %v", attempts[1])
	}
}

func TestCollectDeviceSMARTWWN(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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

func TestListBlockDevicesSkipsVirtualLinuxDevices(t *testing.T) {
	origRun := runCommandOutput
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runCommandOutput = origRun; runtimeGOOS = origGOOS })

	runtimeGOOS = "linux"
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "lsblk" {
			return nil, errors.New("unexpected command")
		}
		return mustMarshalLSBLK(t, []lsblkDevice{
			{Name: "sda", Type: "disk", Model: "Samsung SSD 870 EVO", Vendor: "ATA"},
			{Name: "zd0", Type: "disk"},
			{Name: "dm-0", Type: "disk"},
			{Name: "vda", Type: "disk", Tran: "virtio"},
			{Name: "sdb", Type: "disk", Model: "Virtual disk", Vendor: "VMware"},
			{Name: "sdc", Type: "disk", Subsystems: "block:scsi:vmbus:pci"},
		}), nil
	}

	devices, err := listBlockDevices(context.Background(), nil)
	if err != nil {
		t.Fatalf("listBlockDevices error: %v", err)
	}
	if len(devices) != 1 || devices[0] != "/dev/sda" {
		t.Fatalf("unexpected devices after virtual filtering: %#v", devices)
	}
}

func TestCollectDeviceSMARTSkipsUnsupportedPayload(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := smartctlJSON{
			ModelName:    "QEMU HARDDISK",
			SerialNumber: "disk0",
		}
		payload.Device.Protocol = "SCSI"
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected unsupported SMART payload to be skipped, got %#v", result)
	}
}

func TestCollectDeviceSMARTMissingStatusDoesNotFail(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		payload := smartctlJSON{
			ModelName:    "Samsung SSD 870 EVO",
			SerialNumber: "disk1",
		}
		payload.Device.Protocol = "ATA"
		payload.Temperature.Current = 38
		out, _ := json.Marshal(payload)
		return out, nil
	}

	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Health != "" {
		t.Fatalf("expected empty health for SMART data without explicit status, got %q", result.Health)
	}
	if result.Temperature != 38 {
		t.Fatalf("expected temperature 38, got %d", result.Temperature)
	}
}
