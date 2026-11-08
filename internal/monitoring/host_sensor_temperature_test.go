package monitoring

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The conversion boundary, rather than the regex variables, owns these
// expectations. Matching remains case-insensitive but anchored, with ASCII
// core/NVMe numbers and nonempty, non-newline GPU suffixes.
func TestHostSensorTemperatureNameGrammar(t *testing.T) {
	at := time.Unix(1794096000, 0).UTC()
	for _, tc := range []struct {
		key    string
		kind   string
		core   int
		device string
	}{
		{key: "CPU_PACKAGE", kind: "package"},
		{key: "CpU_CoRe_0", kind: "core"},
		{key: "cpu_core_002", kind: "core", core: 2},
		{key: "NVME01", kind: "nvme", device: "nvme01"},
		{key: "GPU_EDGE", kind: "edge", device: "gpu0"},
		{key: "gpu_Junction", kind: "junction", device: "gpu0"},
		{key: "gpu_MEM", kind: "mem", device: "gpu0"},
		{key: "gpu_AMDGPU-PCI-0400", kind: "edge", device: "amdgpu-pci-0400"},
		{key: "gpu_0", kind: "edge", device: "0"},
		{key: "gpu_É", kind: "edge", device: "é"},
		{key: ""},
		{key: " cpu_package"},
		{key: "cpu_package "},
		{key: "cpu_core_"},
		{key: "cpu_core_-1"},
		{key: "cpu_core_+1"},
		{key: "cpu_core_1.0"},
		{key: "cpu_core_١"},
		{key: "cpu_core_99999999999999999999999999999999999999"},
		{key: "prefix_cpu_core_1"},
		{key: "cpu_core_1_suffix"},
		{key: "cpu_core_1\n"},
		{key: "nvme"},
		{key: "nvme١"},
		{key: "nvme0n1"},
		{key: "prefix_nvme0"},
		{key: "nvme0\n"},
		{key: "gpu_"},
		{key: "prefix_gpu_edge"},
		{key: "gpu_edge\n"},
		{key: "gpu_ed\nge"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			sensors := models.HostSensorSummary{TemperatureCelsius: map[string]float64{tc.key: 42}}
			var want *models.Temperature
			if tc.kind != "" {
				want = &models.Temperature{
					Available: true, LastUpdate: at,
					Cores: []models.CoreTemp{}, NVMe: []models.NVMeTemp{}, GPU: []models.GPUTemp{},
				}
				switch tc.kind {
				case "package":
					want.HasCPU, want.CPUPackage = true, 42
				case "core":
					want.HasCPU, want.CPUPackage, want.CPUMax = true, 42, 42
					want.Cores = []models.CoreTemp{{Core: tc.core, Temp: 42}}
				case "nvme":
					want.HasNVMe = true
					want.NVMe = []models.NVMeTemp{{Device: tc.device, Temp: 42}}
				default:
					gpu := models.GPUTemp{Device: tc.device}
					switch tc.kind {
					case "edge":
						gpu.Edge = 42
					case "junction":
						gpu.Junction = 42
					case "mem":
						gpu.Mem = 42
					}
					want.HasGPU, want.GPU = true, []models.GPUTemp{gpu}
				}
			}
			if got := convertHostSensorsToTemperature(sensors, at); !reflect.DeepEqual(got, want) {
				t.Fatalf("sensor %q: got %#v, want %#v", tc.key, got, want)
			}
		})
	}
}

func TestHostSensorTemperatureConcurrentConversions(t *testing.T) {
	at := time.Unix(1794096000, 0).UTC()
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"CPU_PACKAGE": 61, "cpu_core_2": 55, "cpu_core_0": 52,
			"NVME2": 42, "nvme11": 43,
			"GPU_EDGE": 60, "gpu_junction": 70, "gpu_mem": 65,
			"cpu_core_-1": 99, "gpu_": 99,
		},
		SMART: []models.HostDiskSMART{
			{Device: "sda", Temperature: 36, Serial: "fixture-serial", WWN: "fixture-wwn", Model: "fixture", Type: "sata"},
			{Device: "sdb", Standby: true},
		},
	}
	want := &models.Temperature{
		Available: true, LastUpdate: at, HasCPU: true, HasGPU: true, HasNVMe: true, HasSMART: true,
		CPUPackage: 61, CPUMax: 55,
		Cores: []models.CoreTemp{{Core: 0, Temp: 52}, {Core: 2, Temp: 55}},
		NVMe:  []models.NVMeTemp{{Device: "nvme11", Temp: 43}, {Device: "nvme2", Temp: 42}},
		GPU:   []models.GPUTemp{{Device: "gpu0", Edge: 60, Junction: 70, Mem: 65}},
		SMART: []models.DiskTemp{{Device: "/dev/sda", Temperature: 36, Serial: "fixture-serial", WWN: "fixture-wwn", Model: "fixture", Type: "sata", LastUpdated: at}},
	}
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 32; j++ {
				got := convertHostSensorsToTemperature(sensors, at)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("concurrent conversion: got %#v, want %#v", got, want)
					return
				}
				// Returned collections must belong to this conversion, not to
				// the input or another reader sharing the compiled matchers.
				got.Cores[0].Temp = -1
				got.NVMe[0].Temp = -1
				got.GPU[0].Edge = -1
				got.SMART[0].Temperature = -1
			}
		}()
	}
	workers.Wait()
}

func hostSensorTemperatureBenchmarks() []struct {
	name    string
	sensors models.HostSensorSummary
} {
	cores := func(count int) models.HostSensorSummary {
		sensors := models.HostSensorSummary{TemperatureCelsius: map[string]float64{"cpu_package": 60}}
		for i := 0; i < count; i++ {
			sensors.TemperatureCelsius[fmt.Sprintf("cpu_core_%d", i)] = 45 + float64(i%10)
		}
		return sensors
	}
	mixed := cores(8)
	for key, value := range map[string]float64{"nvme0": 40, "nvme1": 42, "gpu_edge": 65, "gpu_mem": 62, "gpu_junction": 70} {
		mixed.TemperatureCelsius[key] = value
	}
	mixed.SMART = []models.HostDiskSMART{{Device: "sda", Temperature: 36}}
	return []struct {
		name    string
		sensors models.HostSensorSummary
	}{
		{name: "empty"},
		{name: "unrecognized", sensors: models.HostSensorSummary{TemperatureCelsius: map[string]float64{"ambient": 24}}},
		{name: "package-only", sensors: cores(0)},
		{name: "8-cores", sensors: cores(8)},
		{name: "64-cores", sensors: cores(64)},
		{name: "mixed", sensors: mixed},
		{name: "smart-only", sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "sda", Temperature: 36}}}},
		{name: "standby-only", sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "sda", Standby: true}}}},
	}
}

var hostSensorTemperatureBenchmarkSink *models.Temperature

func BenchmarkHostSensorTemperature(b *testing.B) {
	at := time.Unix(1794096000, 0).UTC()
	for _, fixture := range hostSensorTemperatureBenchmarks() {
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				hostSensorTemperatureBenchmarkSink = convertHostSensorsToTemperature(fixture.sensors, at)
			}
		})
	}
}
