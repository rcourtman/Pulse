package hostagent

import (
	"context"
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/sensors"
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

func TestAgent_collectTemperatures_SkipsUnsupportedOS(t *testing.T) {
	mc := &mockCollector{goos: "windows"}
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
