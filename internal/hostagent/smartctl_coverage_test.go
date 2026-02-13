package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestListBlockDevices(t *testing.T) {
	origRun := runCommandOutput
	origGOOS := runtimeGOOS
	t.Cleanup(func() { runCommandOutput = origRun; runtimeGOOS = origGOOS })

	runtimeGOOS = "linux"
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "lsblk" {
			return nil, errors.New("unexpected command")
		}
		out := "sda disk\nsda1 part\nsr0 rom\nnvme0n1 disk\n\n"
		return []byte(out), nil
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

func TestDefaultRunCommandOutputRejectsOversizedOutput(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })
	runCommandOutput = origRun

	cmd := fmt.Sprintf("head -c %d /dev/zero", maxCommandOutputBytes+1)
	_, err := runCommandOutput(context.Background(), "sh", "-c", cmd)
	if err == nil {
		t.Fatal("expected oversized output error")
	}
	if !errors.Is(err, errCommandOutputTooLarge) {
		t.Fatalf("expected oversized output error, got %v", err)
	}
}

func TestCollectLocalNoDevices(t *testing.T) {
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return []byte(""), nil
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
	origRun := runCommandOutput
	t.Cleanup(func() { runCommandOutput = origRun })

	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("lsblk failed")
	}

	if _, err := CollectSMARTLocal(context.Background(), nil); err == nil {
		t.Fatalf("expected list error")
	}
}

func TestCollectSMARTLocalSkipsErrors(t *testing.T) {
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
			return []byte("sda disk\nsdb disk\n"), nil
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
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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

func TestCollectDeviceSMARTMissingSmartStatusUsesUnknownHealth(t *testing.T) {
	origRun := runCommandOutput
	origLook := execLookPath
	t.Cleanup(func() {
		runCommandOutput = origRun
		execLookPath = origLook
	})

	execLookPath = func(string) (string, error) { return "smartctl", nil }
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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
