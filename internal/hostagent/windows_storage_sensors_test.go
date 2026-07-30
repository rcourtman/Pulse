package hostagent

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestAgentCollectWindowsTemperatureSensorsMergesNativeStorageAndNVIDIA(t *testing.T) {
	const powerShellPath = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	const nvidiaPath = `C:\Windows\System32\nvidia-smi.exe`

	collector := &mockCollector{
		goos: "windows",
		lookPathFn: func(file string) (string, error) {
			switch file {
			case "powershell.exe":
				return powerShellPath, nil
			case "nvidia-smi":
				return nvidiaPath, nil
			default:
				return "", os.ErrNotExist
			}
		},
		commandCombinedOutputFn: func(_ context.Context, name string, args ...string) (string, error) {
			switch name {
			case powerShellPath:
				if len(args) != 5 ||
					args[0] != "-NoLogo" ||
					args[1] != "-NoProfile" ||
					args[2] != "-NonInteractive" ||
					args[3] != "-Command" ||
					args[4] != windowsStorageTemperatureScript {
					t.Fatalf("unexpected PowerShell args: %#v", args)
				}
				return `[
					{"deviceId":"1","friendlyName":"Samsung SSD 990 PRO","busType":"NVMe","mediaType":"SSD","sizeBytes":2000398934016,"temperature":42},
					{"deviceId":"0","friendlyName":"Archive Disk","busType":"SATA","mediaType":"HDD","sizeBytes":4000787030016,"temperature":35}
				]`, nil
			case nvidiaPath:
				return "0, NVIDIA GeForce RTX 4090, 61, 7, 4096, 24576\r\n", nil
			default:
				t.Fatalf("unexpected command %q", name)
				return "", nil
			}
		},
	}
	agent := &Agent{logger: zerolog.Nop(), collector: collector}

	got := agent.collectTemperatures(context.Background())
	if len(got.SMART) != 2 {
		t.Fatalf("Windows storage disks = %d, want 2: %+v", len(got.SMART), got.SMART)
	}
	if got.SMART[0].Device != "PhysicalDisk0" ||
		got.SMART[0].Model != "Archive Disk" ||
		got.SMART[0].Type != "sata" ||
		got.SMART[0].Temperature != 35 {
		t.Fatalf("unexpected first storage disk: %+v", got.SMART[0])
	}
	if got.SMART[1].Device != "PhysicalDisk1" ||
		got.SMART[1].Model != "Samsung SSD 990 PRO" ||
		got.SMART[1].Type != "nvme" ||
		got.SMART[1].SizeBytes != 2000398934016 ||
		got.SMART[1].Temperature != 42 {
		t.Fatalf("unexpected second storage disk: %+v", got.SMART[1])
	}
	if got.SMART[1].Collection == nil ||
		got.SMART[1].Collection.Temperature.Source != windowsStorageSource ||
		got.SMART[1].Collection.Temperature.State != "available" {
		t.Fatalf("storage temperature provenance = %+v", got.SMART[1].Collection)
	}
	if got.TemperatureCelsius["gpu_nvidia_0"] != 61 {
		t.Fatalf("NVIDIA temperature = %v, want 61", got.TemperatureCelsius["gpu_nvidia_0"])
	}
	if len(got.GPU) != 1 || got.GPU[0].UtilizationPercent == nil || *got.GPU[0].UtilizationPercent != 7 {
		t.Fatalf("typed NVIDIA telemetry = %+v", got.GPU)
	}
}

func TestParseWindowsStorageTemperaturesValidatesAndBoundsReadings(t *testing.T) {
	longName := strings.Repeat("温", 140)
	output := `[
		{"deviceId":" 0 ","friendlyName":"` + longName + `","busType":"USB","mediaType":"SSD","sizeBytes":1000,"temperature":40.6},
		{"deviceId":"0","friendlyName":"duplicate","busType":"NVMe","temperature":50},
		{"deviceId":"bad id/with spaces","friendlyName":"Disk B","busType":"Unknown","mediaType":"SSD","sizeBytes":-1,"temperature":33},
		{"deviceId":"hot","temperature":151},
		{"deviceId":"zero","temperature":0},
		{"deviceId":"missing"}
	]`

	got, err := parseWindowsStorageTemperatures(output)
	if err != nil {
		t.Fatalf("parseWindowsStorageTemperatures returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("parsed disks = %d, want 2: %+v", len(got), got)
	}
	if got[0].Device != "PhysicalDisk0" || got[0].Temperature != 41 || len([]rune(got[0].Model)) != 128 {
		t.Fatalf("bounded first disk = %+v", got[0])
	}
	if got[1].Device != "PhysicalDiskbad_id_with_spaces" ||
		got[1].Type != "ssd" ||
		got[1].SizeBytes != 0 ||
		got[1].Temperature != 33 {
		t.Fatalf("normalized second disk = %+v", got[1])
	}
}

func TestParseWindowsStorageTemperaturesAcceptsSingleObjectAndEmptyOutput(t *testing.T) {
	got, err := parseWindowsStorageTemperatures(
		`{"deviceId":"7","friendlyName":"Single Disk","busType":"SAS","temperature":29}`,
	)
	if err != nil {
		t.Fatalf("parse single object: %v", err)
	}
	if len(got) != 1 || got[0].Device != "PhysicalDisk7" || got[0].Type != "sas" {
		t.Fatalf("single object result = %+v", got)
	}

	for _, output := range []string{"", "null", "\ufeff [] "} {
		got, err := parseWindowsStorageTemperatures(output)
		if err != nil || got != nil {
			t.Fatalf("empty output %q = (%+v, %v), want nil, nil", output, got, err)
		}
	}
}

func TestParseWindowsStorageTemperaturesRejectsMalformedAndOversizedOutput(t *testing.T) {
	if _, err := parseWindowsStorageTemperatures("{"); err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if _, err := parseWindowsStorageTemperatures(strings.Repeat("x", windowsStorageMaxOutputBytes+1)); err == nil {
		t.Fatal("expected oversized output error")
	}
}

func TestAgentCollectWindowsStorageTemperaturesIsBestEffort(t *testing.T) {
	tests := []struct {
		name      string
		lookPath  func(string) (string, error)
		runOutput func(context.Context, string, ...string) (string, error)
	}{
		{
			name: "PowerShell missing",
			lookPath: func(string) (string, error) {
				return "", os.ErrNotExist
			},
		},
		{
			name: "query fails",
			lookPath: func(string) (string, error) {
				return `C:\powershell.exe`, nil
			},
			runOutput: func(context.Context, string, ...string) (string, error) {
				return "", errors.New("storage provider unavailable")
			},
		},
		{
			name: "query returns invalid JSON",
			lookPath: func(string) (string, error) {
				return `C:\powershell.exe`, nil
			},
			runOutput: func(context.Context, string, ...string) (string, error) {
				return "{", nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agent := &Agent{
				logger: zerolog.Nop(),
				collector: &mockCollector{
					goos:                    "windows",
					lookPathFn:              tc.lookPath,
					commandCombinedOutputFn: tc.runOutput,
				},
			}
			got := agent.collectWindowsStorageTemperatures(context.Background())
			if len(got.SMART) != 0 {
				t.Fatalf("best-effort result = %+v, want empty", got)
			}
		})
	}
}
