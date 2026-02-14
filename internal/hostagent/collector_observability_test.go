package hostagent

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestCollectLocal_LogsStructuredContextOnDeviceCollectionFailure(t *testing.T) {
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
	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "lsblk" {
			return []byte("sda disk\n"), nil
		}
		if name == "smartctl" {
			return nil, errors.New("read failed")
		}
		return nil, errors.New("unexpected command: " + name)
	}

	logOutput := captureSmartctlLogs(t)
	results, err := CollectSMARTLocal(context.Background(), nil)
	if err != nil {
		t.Fatalf("CollectSMARTLocal returned unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("CollectSMARTLocal returned results despite command failure: %#v", results)
	}

	for _, expected := range []string{
		`"component":"smartctl_collector"`,
		`"action":"collect_device_smart_failed"`,
		`"device":"/dev/sda"`,
		`"action":"collect_local_complete"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestListBlockDevicesLinux_LogsStructuredContextForExcludedDevice(t *testing.T) {
	origRun := smartRunCommandOutput
	t.Cleanup(func() {
		smartRunCommandOutput = origRun
	})

	smartRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("sda disk\n"), nil
	}

	logOutput := captureSmartctlLogs(t)
	devices, err := listBlockDevicesLinux(context.Background(), []string{"sda"})
	if err != nil {
		t.Fatalf("listBlockDevicesLinux returned error: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected excluded device to be filtered, got %#v", devices)
	}

	for _, expected := range []string{
		`"component":"smartctl_collector"`,
		`"action":"skip_excluded_device"`,
		`"device":"/dev/sda"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func TestCollectDeviceSMART_LogsStructuredContextWhenDeviceInStandby(t *testing.T) {
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
		return exec.CommandContext(ctx, "sh", "-c", "exit 2").Output()
	}

	logOutput := captureSmartctlLogs(t)
	result, err := collectDeviceSMART(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("collectDeviceSMART returned unexpected error: %v", err)
	}
	if result == nil || !result.Standby {
		t.Fatalf("expected standby result, got %#v", result)
	}

	for _, expected := range []string{
		`"component":"smartctl_collector"`,
		`"action":"device_in_standby"`,
		`"device":"sda"`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Fatalf("expected log output to include %s, got %q", expected, logOutput.String())
		}
	}
}

func captureSmartctlLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	t.Cleanup(func() {
		log.Logger = origLogger
	})

	return &buf
}
