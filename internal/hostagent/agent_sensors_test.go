package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestAgent_collectTemperatures_MapsKeys(t *testing.T) {
	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return "{}", nil },
		sensorsParseFn: func(string) (*sensors.TemperatureData, error) {
			return &sensors.TemperatureData{
				Available:  true,
				CPUPackage: 55.5,
				Cores: map[string]float64{
					"Core 0": 44,
					"Core 1": 45,
				},
				NVMe: map[string]float64{
					"nvme0": 40,
				},
				GPU: map[string]float64{
					"amdgpu-pci-0100": 60,
				},
			}, nil
		},
	}

	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	want := map[string]float64{
		"cpu_package":     55.5,
		"cpu_core_0":      44,
		"cpu_core_1":      45,
		"nvme0":           40,
		"amdgpu-pci-0100": 60,
	}

	if got.TemperatureCelsius == nil {
		t.Fatalf("expected TemperatureCelsius map to be initialised")
	}
	if len(got.TemperatureCelsius) != len(want) {
		t.Fatalf("temperature keys = %d, want %d", len(got.TemperatureCelsius), len(want))
	}
	for k, v := range want {
		if gotVal, ok := got.TemperatureCelsius[k]; !ok || gotVal != v {
			t.Fatalf("TemperatureCelsius[%q] = (%v, %v), want (%v, %v)", k, gotVal, ok, v, true)
		}
	}
}

func TestAgent_collectTemperatures_ArmadaThermalFeedsMachineCPU(t *testing.T) {
	mc := &mockCollector{
		goos: "linux",
		sensorsLocalFn: func(context.Context) (string, error) {
			return `{"armada_thermal-virtual-0":{"temp1":{"temp1_input":57}}}`, nil
		},
		sensorsParseFn: sensors.Parse,
	}
	a := &Agent{logger: zerolog.Nop(), collector: mc}
	got := a.collectTemperatures(context.Background())
	if got.TemperatureCelsius["cpu_package"] != 57 {
		t.Fatalf("CPU package reading = %v, want 57", got.TemperatureCelsius)
	}
}

func TestAgent_collectTemperatures_MergesNVIDIASMITemperatures(t *testing.T) {
	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return "{}", nil },
		sensorsParseFn: func(string) (*sensors.TemperatureData, error) {
			return &sensors.TemperatureData{
				Available:  true,
				CPUPackage: 55.5,
			}, nil
		},
		lookPathFn: func(file string) (string, error) {
			if file != "nvidia-smi" {
				t.Fatalf("look path file = %q, want nvidia-smi", file)
			}
			return "/usr/bin/nvidia-smi", nil
		},
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			if name != "/usr/bin/nvidia-smi" {
				t.Fatalf("command name = %q, want /usr/bin/nvidia-smi", name)
			}
			if len(arg) != 2 || arg[0] != "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total" || arg[1] != "--format=csv,noheader,nounits" {
				t.Fatalf("command args = %#v, want NVIDIA stats query", arg)
			}
			return "0, NVIDIA GeForce RTX 4090, 61, 7, 4096, 24576\n", nil
		},
	}

	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if got.TemperatureCelsius["cpu_package"] != 55.5 {
		t.Fatalf("cpu package temp = %v, want 55.5", got.TemperatureCelsius["cpu_package"])
	}
	if got.TemperatureCelsius["gpu_nvidia_0"] != 61 {
		t.Fatalf("NVIDIA GPU temp = %v, want 61", got.TemperatureCelsius["gpu_nvidia_0"])
	}
	if len(got.GPU) != 1 {
		t.Fatalf("GPU stats = %d, want 1: %+v", len(got.GPU), got.GPU)
	}
	if got.GPU[0].UtilizationPercent == nil || *got.GPU[0].UtilizationPercent != 7 {
		t.Fatalf("GPU utilization = %#v, want 7", got.GPU[0].UtilizationPercent)
	}
	if got.GPU[0].MemoryUsedBytes == nil || *got.GPU[0].MemoryUsedBytes != 4096*1024*1024 {
		t.Fatalf("GPU memory used = %#v, want 4096 MiB", got.GPU[0].MemoryUsedBytes)
	}
}

func TestAgent_collectTemperatures_UsesNVIDIASMIWhenLMSensorsUnavailable(t *testing.T) {
	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return "", errors.New("no sensors") },
		lookPathFn: func(file string) (string, error) {
			if file != "nvidia-smi" {
				t.Fatalf("look path file = %q, want nvidia-smi", file)
			}
			return "/usr/bin/nvidia-smi", nil
		},
		commandCombinedOutputFn: func(context.Context, string, ...string) (string, error) {
			return "0, NVIDIA RTX A6000, 58, 0, 0, 49152\n", nil
		},
	}

	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if len(got.TemperatureCelsius) != 1 {
		t.Fatalf("temperature keys = %d, want 1; got %v", len(got.TemperatureCelsius), got.TemperatureCelsius)
	}
	if got.TemperatureCelsius["gpu_nvidia_0"] != 58 {
		t.Fatalf("NVIDIA GPU temp = %v, want 58", got.TemperatureCelsius["gpu_nvidia_0"])
	}
	if got.GPU[0].UtilizationPercent == nil || *got.GPU[0].UtilizationPercent != 0 {
		t.Fatalf("GPU utilization = %#v, want 0", got.GPU[0].UtilizationPercent)
	}
	if got.GPU[0].MemoryUsedBytes == nil || *got.GPU[0].MemoryUsedBytes != 0 {
		t.Fatalf("GPU memory used = %#v, want 0", got.GPU[0].MemoryUsedBytes)
	}
}

func TestAgent_collectTemperatures_BestEffortFailuresReturnEmpty(t *testing.T) {
	mc := &mockCollector{goos: "linux"}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	mc.sensorsLocalFn = func(context.Context) (string, error) { return "", errors.New("no sensors") }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on collect error, got %#v", got.TemperatureCelsius)
	}

	mc.sensorsLocalFn = func(context.Context) (string, error) { return "{}", nil }
	mc.sensorsParseFn = func(string) (*sensors.TemperatureData, error) { return nil, errors.New("bad json") }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on parse error, got %#v", got.TemperatureCelsius)
	}

	mc.sensorsParseFn = func(string) (*sensors.TemperatureData, error) { return &sensors.TemperatureData{Available: false}, nil }
	if got := a.collectTemperatures(context.Background()); len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors when unavailable, got %#v", got.TemperatureCelsius)
	}
}

func TestParseNVIDIASMITemperatures(t *testing.T) {
	input := `0, NVIDIA GeForce RTX 4090, 47, 11, 2048, 24576
1, "NVIDIA, RTX A6000", N/A, N/A, N/A, N/A
2, NVIDIA Tesla T4, 58 C, 0 %, 0 MiB, 15360 MiB
0000:65:00.0, NVIDIA H100, 64, 95, 70000, 81920`

	got, err := parseNVIDIASMITemperatures(input)
	if err != nil {
		t.Fatalf("parseNVIDIASMITemperatures returned error: %v", err)
	}

	want := map[string]float64{
		"gpu_nvidia_0":            47,
		"gpu_nvidia_2":            58,
		"gpu_nvidia_0000_65_00_0": 64,
	}
	if len(got) != len(want) {
		t.Fatalf("temperature keys = %d, want %d; got %v", len(got), len(want), got)
	}
	for key, wantTemp := range want {
		if gotTemp := got[key]; gotTemp != wantTemp {
			t.Fatalf("%s = %v, want %v", key, gotTemp, wantTemp)
		}
	}
}

func TestParseNVIDIASMIStats(t *testing.T) {
	input := `0, NVIDIA GeForce RTX 4090, 47, 11, 2048, 24576
1, NVIDIA RTX A6000, 0, 0, 0, 49152`

	got, err := parseNVIDIASMIStats(input)
	if err != nil {
		t.Fatalf("parseNVIDIASMIStats returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GPU stats = %d, want 2: %+v", len(got), got)
	}
	if got[0].ID != "0" || got[0].Name != "NVIDIA GeForce RTX 4090" {
		t.Fatalf("unexpected first GPU identity: %+v", got[0])
	}
	if got[0].TemperatureCelsius == nil || *got[0].TemperatureCelsius != 47 {
		t.Fatalf("first GPU temp = %#v, want 47", got[0].TemperatureCelsius)
	}
	if got[0].UtilizationPercent == nil || *got[0].UtilizationPercent != 11 {
		t.Fatalf("first GPU utilization = %#v, want 11", got[0].UtilizationPercent)
	}
	if got[0].MemoryUsedBytes == nil || *got[0].MemoryUsedBytes != 2048*1024*1024 {
		t.Fatalf("first GPU memory used = %#v, want 2048 MiB", got[0].MemoryUsedBytes)
	}
	if got[1].TemperatureCelsius != nil {
		t.Fatalf("second GPU temp = %#v, want nil for zero temperature", got[1].TemperatureCelsius)
	}
	if got[1].UtilizationPercent == nil || *got[1].UtilizationPercent != 0 {
		t.Fatalf("second GPU utilization = %#v, want 0", got[1].UtilizationPercent)
	}
	if got[1].MemoryUsedBytes == nil || *got[1].MemoryUsedBytes != 0 {
		t.Fatalf("second GPU memory used = %#v, want 0", got[1].MemoryUsedBytes)
	}
}

func TestParseNVIDIASMITemperatures_NoUsableRows(t *testing.T) {
	got, err := parseNVIDIASMITemperatures("0, NVIDIA RTX, N/A\nbad row\n")
	if err != nil {
		t.Fatalf("parseNVIDIASMITemperatures returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil temps, got %v", got)
	}
}

func TestAgent_queryNVIDIASMITemperatures_CommandNotInstalled(t *testing.T) {
	for _, lookPathErr := range []error{os.ErrNotExist, exec.ErrNotFound} {
		mc := &mockCollector{
			lookPathFn: func(string) (string, error) {
				return "", lookPathErr
			},
		}
		a := &Agent{logger: zerolog.Nop(), collector: mc}

		got, err := a.queryNVIDIASMITemperatures(context.Background())
		if err != nil {
			t.Fatalf("queryNVIDIASMITemperatures returned error for %v: %v", lookPathErr, err)
		}
		if got != nil {
			t.Fatalf("expected nil temps for %v, got %v", lookPathErr, got)
		}
	}
}

func TestAgent_collectTemperatures_CollectsNVIDIAOnWindows(t *testing.T) {
	mc := &mockCollector{
		goos: "windows",
		lookPathFn: func(file string) (string, error) {
			if file == "powershell.exe" || file == "powershell" {
				return "", os.ErrNotExist
			}
			if file != "nvidia-smi" {
				t.Fatalf("look path file = %q, want nvidia-smi", file)
			}
			return `C:\Windows\System32\nvidia-smi.exe`, nil
		},
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			if name != `C:\Windows\System32\nvidia-smi.exe` {
				t.Fatalf("command name = %q, want Windows nvidia-smi path", name)
			}
			if len(arg) != 2 || arg[0] != "--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total" || arg[1] != "--format=csv,noheader,nounits" {
				t.Fatalf("command args = %#v, want NVIDIA stats query", arg)
			}
			return "0, NVIDIA GeForce RTX 3070, 57, 31, 2048, 8192\r\n", nil
		},
	}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if got.TemperatureCelsius["gpu_nvidia_0"] != 57 {
		t.Fatalf("Windows NVIDIA GPU temp = %v, want 57", got.TemperatureCelsius["gpu_nvidia_0"])
	}
	if len(got.GPU) != 1 {
		t.Fatalf("Windows GPU stats = %d, want 1: %+v", len(got.GPU), got.GPU)
	}
	if got.GPU[0].UtilizationPercent == nil || *got.GPU[0].UtilizationPercent != 31 {
		t.Fatalf("Windows GPU utilization = %#v, want 31", got.GPU[0].UtilizationPercent)
	}
	if got.GPU[0].MemoryTotalBytes == nil || *got.GPU[0].MemoryTotalBytes != 8192*1024*1024 {
		t.Fatalf("Windows GPU memory total = %#v, want 8192 MiB", got.GPU[0].MemoryTotalBytes)
	}
}

func TestAgent_collectTemperatures_SkipsUnsupportedOS(t *testing.T) {
	mc := &mockCollector{goos: "plan9"}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty sensors on unsupported OS, got %#v", got.TemperatureCelsius)
	}
}

func TestAgent_collectTemperatures_DarwinThermalState(t *testing.T) {
	mc := &mockCollector{
		goos: "darwin",
		commandCombinedOutputFn: func(_ context.Context, name string, arg ...string) (string, error) {
			if name != "pmset" {
				t.Fatalf("command name = %q, want pmset", name)
			}
			if len(arg) != 2 || arg[0] != "-g" || arg[1] != "therm" {
				t.Fatalf("command args = %#v, want [-g therm]", arg)
			}
			return `Note: No thermal warning level has been recorded
Note: No performance warning level has been recorded
Note: No CPU power status has been recorded
CPU_Speed_Limit = 100
Scheduler_Limit = 100
`, nil
		},
	}
	a := &Agent{logger: zerolog.Nop(), collector: mc}

	got := a.collectTemperatures(context.Background())
	if len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected no Celsius temperatures on macOS thermal-state collection, got %#v", got.TemperatureCelsius)
	}
	if got.ThermalState == nil {
		t.Fatal("expected macOS thermal state")
	}
	if got.ThermalState.Source != "pmset" {
		t.Fatalf("Source = %q, want pmset", got.ThermalState.Source)
	}
	if got.ThermalState.Pressure != "nominal" {
		t.Fatalf("Pressure = %q, want nominal", got.ThermalState.Pressure)
	}
	if got.ThermalState.ThermalWarningLevel == nil || *got.ThermalState.ThermalWarningLevel != 0 {
		t.Fatalf("ThermalWarningLevel = %#v, want 0", got.ThermalState.ThermalWarningLevel)
	}
	if got.ThermalState.LimitsPercent["cpu_speed_limit"] != 100 {
		t.Fatalf("cpu_speed_limit = %d, want 100", got.ThermalState.LimitsPercent["cpu_speed_limit"])
	}
}

func TestParseDarwinPMSetThermalState_Constrained(t *testing.T) {
	got := parseDarwinPMSetThermalState(`Thermal Warning Level: 1
Performance Warning Level: 2
CPU Power Status: 1
CPU_Speed_Limit = 80
`)
	if got == nil {
		t.Fatal("expected thermal state")
	}
	if got.Pressure != "constrained" {
		t.Fatalf("Pressure = %q, want constrained", got.Pressure)
	}
	if got.ThermalWarningLevel == nil || *got.ThermalWarningLevel != 1 {
		t.Fatalf("ThermalWarningLevel = %#v, want 1", got.ThermalWarningLevel)
	}
	if got.PerformanceWarningLevel == nil || *got.PerformanceWarningLevel != 2 {
		t.Fatalf("PerformanceWarningLevel = %#v, want 2", got.PerformanceWarningLevel)
	}
	if got.CPUPowerStatus == nil || *got.CPUPowerStatus != 1 {
		t.Fatalf("CPUPowerStatus = %#v, want 1", got.CPUPowerStatus)
	}
	if got.LimitsPercent["cpu_speed_limit"] != 80 {
		t.Fatalf("cpu_speed_limit = %d, want 80", got.LimitsPercent["cpu_speed_limit"])
	}
}

func TestParseFreeBSDSysctlTemperatures(t *testing.T) {
	input := `kern.ostype: FreeBSD
kern.osrelease: 13.2-RELEASE
dev.cpu.0.temperature: 45.0C
dev.cpu.1.temperature: 46.0C
dev.cpu.2.temperature: 44.0C
dev.cpu.3.temperature: 47.0C
hw.acpi.thermal.tz0.temperature: 50.0C
hw.acpi.thermal.tz1.temperature: 48.0C
hw.machine: amd64
`

	got := parseFreeBSDSysctlTemperatures(input)
	want := map[string]float64{
		"cpu_core_0": 45.0,
		"cpu_core_1": 46.0,
		"cpu_core_2": 44.0,
		"cpu_core_3": 47.0,
		"acpi_tz0":   50.0,
		"acpi_tz1":   48.0,
	}

	if len(got.TemperatureCelsius) != len(want) {
		t.Fatalf("temperature keys = %d, want %d; got %v", len(got.TemperatureCelsius), len(want), got.TemperatureCelsius)
	}
	for k, v := range want {
		if gotVal, ok := got.TemperatureCelsius[k]; !ok || gotVal != v {
			t.Fatalf("TemperatureCelsius[%q] = (%v, %v), want (%v, %v)", k, gotVal, ok, v, true)
		}
	}
}

func TestParseFreeBSDSysctlTemperatures_Empty(t *testing.T) {
	got := parseFreeBSDSysctlTemperatures("kern.ostype: FreeBSD\nhw.machine: amd64\n")
	if len(got.TemperatureCelsius) != 0 {
		t.Fatalf("expected empty map for no-temperature output, got %v", got.TemperatureCelsius)
	}
}

func TestParseFreeBSDSysctlTemperatures_MalformedLines(t *testing.T) {
	input := `dev.cpu.0.temperature: badvalue
dev.cpu.1.temperature: -5.0C
dev.cpu.2.temperature: 0.0C
dev.cpu.3.temperature: 55.0C
`

	got := parseFreeBSDSysctlTemperatures(input)
	if len(got.TemperatureCelsius) != 1 {
		t.Fatalf("expected 1 valid temperature, got %d: %v", len(got.TemperatureCelsius), got.TemperatureCelsius)
	}
	if got.TemperatureCelsius["cpu_core_3"] != 55.0 {
		t.Fatalf("expected cpu_core_3=55.0, got %v", got.TemperatureCelsius["cpu_core_3"])
	}
}

func TestAgentMixedSensorUnitsReachReportWithoutFalseTemperatures(t *testing.T) {
	input, err := os.ReadFile("../sensors/testdata/mixed_sensor_units.json")
	if err != nil {
		t.Fatal(err)
	}
	mc := &mockCollector{
		goos:           "linux",
		sensorsLocalFn: func(context.Context) (string, error) { return string(input), nil },
		sensorsParseFn: sensors.Parse,
	}
	got := (&Agent{logger: zerolog.Nop(), collector: mc}).collectTemperatures(context.Background())
	want := agentshost.Sensors{
		TemperatureCelsius: map[string]float64{"cpu_package": 91, "cpu_core_0": 87, "nvme0": 36.9, "amdgpu-pci-0300_edge": 58},
		FanRPM:             map[string]float64{"applesmc_left_side": 6152, "applesmc_right_side": 5699},
		Additional:         map[string]float64{"applesmc_tb0t": 34.2, "applesmc_tb1t": 29.2, "applesmc_th1h": 44.5},
		Native: []agentshost.NativeSensor{
			reportNativeSensor("amdgpu-pci-0300", "", "temp1", "edge", agentshost.NativeSensorKindTemperature, 58, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp1", "TB0T", agentshost.NativeSensorKindTemperature, 34.2, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp2", "TB1T", agentshost.NativeSensorKindTemperature, 29.2, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp3", "Th1H", agentshost.NativeSensorKindTemperature, 44.5, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp4", "TG1D", agentshost.NativeSensorKindTemperature, -127, agentshost.NativeSensorStatusInvalid),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp5", "TCMc", agentshost.NativeSensorKindTemperature, 0, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "fan1", "Left side", agentshost.NativeSensorKindFan, 6152, agentshost.NativeSensorStatusOK),
			reportNativeSensor("applesmc-isa-0300", "ISA adapter", "fan2", "Right side", agentshost.NativeSensorKindFan, 5699, agentshost.NativeSensorStatusOK),
			reportNativeSensor("coretemp-isa-0000", "", "temp1", "Package id 0", agentshost.NativeSensorKindTemperature, 91, agentshost.NativeSensorStatusOK),
			reportNativeSensor("coretemp-isa-0000", "", "temp2", "Core 0", agentshost.NativeSensorKindTemperature, 87, agentshost.NativeSensorStatusOK),
			reportNativeSensor("nvme-pci-0200", "", "temp1", "Composite", agentshost.NativeSensorKindTemperature, 36.9, agentshost.NativeSensorStatusOK),
		},
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("full sensor report = %+v, want %+v", got, want)
	}
	if os.Getenv("PULSE_SENSOR_UNITS_FRAME") == "1" {
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		t.Log("SENSOR_UNITS_FRAME=" + string(data))
	}
}

func reportNativeSensor(chip, adapter, channel, label, kind string, value float64, status string) agentshost.NativeSensor {
	unit := agentshost.NativeSensorUnitCelsius
	if kind == agentshost.NativeSensorKindFan {
		unit = agentshost.NativeSensorUnitRPM
	}
	return agentshost.NativeSensor{ID: chip + "/" + channel, Chip: chip, Adapter: adapter, Channel: channel, Label: label,
		Kind: kind, Unit: unit, Value: &value, Status: status}
}

func TestAgentReportsNativeInventoryWithoutLegacyReadings(t *testing.T) {
	// A stopped fan and an unattached input give the legacy maps nothing, so
	// the parser marks the data unavailable, but the inventory still says
	// what the chip exposes.
	mc := &mockCollector{
		goos: "linux",
		sensorsLocalFn: func(context.Context) (string, error) {
			return `{"applesmc-isa-0300":{"Adapter":"ISA adapter","Exhaust":{"fan1_input":0},"TG1D":{"temp1_input":-127}}}`, nil
		},
		sensorsParseFn: sensors.Parse,
		sensorsPowerFn: func(context.Context) (*sensors.PowerData, error) {
			return &sensors.PowerData{PackageWatts: 18, Available: true, Source: "rapl"}, nil
		},
	}
	got := (&Agent{logger: zerolog.Nop(), collector: mc}).collectTemperatures(context.Background())
	want := []agentshost.NativeSensor{
		reportNativeSensor("applesmc-isa-0300", "ISA adapter", "temp1", "TG1D", agentshost.NativeSensorKindTemperature, -127, agentshost.NativeSensorStatusInvalid),
		reportNativeSensor("applesmc-isa-0300", "ISA adapter", "fan1", "Exhaust", agentshost.NativeSensorKindFan, 0, agentshost.NativeSensorStatusOK),
	}
	if !reflect.DeepEqual(got.Native, want) {
		t.Fatalf("native inventory = %+v, want %+v", got.Native, want)
	}
	if len(got.TemperatureCelsius) != 0 || len(got.Additional) != 0 || len(got.FanRPM) != 0 {
		t.Fatalf("legacy maps = %+v, want them empty", got)
	}
	if len(got.PowerWatts) != 0 {
		t.Fatalf("power readings = %+v, want the existing gate to skip power collection", got.PowerWatts)
	}
}
