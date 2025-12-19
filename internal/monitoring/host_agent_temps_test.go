package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestConvertHostSensorsToTemperature_Empty(t *testing.T) {
	sensors := models.HostSensorSummary{}
	result := convertHostSensorsToTemperature(sensors, time.Now())
	if result != nil {
		t.Error("expected nil for empty sensors")
	}
}

func TestConvertHostSensorsToTemperature_CPUOnly(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 55.0,
			"cpu_core_0":  50.0,
			"cpu_core_1":  52.0,
		},
	}
	now := time.Now()
	result := convertHostSensorsToTemperature(sensors, now)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Available {
		t.Error("expected Available to be true")
	}
	if !result.HasCPU {
		t.Error("expected HasCPU to be true")
	}
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage 55.0, got %f", result.CPUPackage)
	}
	if len(result.Cores) != 2 {
		t.Errorf("expected 2 cores, got %d", len(result.Cores))
	}
	// Cores should be sorted
	if result.Cores[0].Core != 0 || result.Cores[1].Core != 1 {
		t.Error("cores not sorted correctly")
	}
	if result.CPUMax != 52.0 { // Max of core temps
		t.Errorf("expected CPUMax 52.0, got %f", result.CPUMax)
	}
}

func TestConvertHostSensorsToTemperature_NVMe(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"nvme0":       40.0,
			"nvme1":       42.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasNVMe {
		t.Error("expected HasNVMe to be true")
	}
	if len(result.NVMe) != 2 {
		t.Errorf("expected 2 NVMe devices, got %d", len(result.NVMe))
	}
	// NVMe should be sorted
	if result.NVMe[0].Device != "nvme0" || result.NVMe[1].Device != "nvme1" {
		t.Error("NVMe devices not sorted correctly")
	}
}

func TestConvertHostSensorsToTemperature_GPU(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"gpu_edge":    60.0,
			"gpu_junction": 65.0,
			"gpu_mem":     55.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU, got %d", len(result.GPU))
	}
	if result.GPU[0].Device != "gpu0" {
		t.Errorf("expected device 'gpu0', got %q", result.GPU[0].Device)
	}
	if result.GPU[0].Edge != 60.0 {
		t.Errorf("expected Edge 60.0, got %f", result.GPU[0].Edge)
	}
	if result.GPU[0].Junction != 65.0 {
		t.Errorf("expected Junction 65.0, got %f", result.GPU[0].Junction)
	}
}

func TestConvertHostSensorsToTemperature_GenericGPU(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_package": 45.0,
			"gpu_nvidia":  70.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU, got %d", len(result.GPU))
	}
	if result.GPU[0].Device != "nvidia" {
		t.Errorf("expected device 'nvidia', got %q", result.GPU[0].Device)
	}
	if result.GPU[0].Edge != 70.0 {
		t.Errorf("expected Edge 70.0, got %f", result.GPU[0].Edge)
	}
}

func TestConvertHostSensorsToTemperature_NoPackageUsesMaxCore(t *testing.T) {
	sensors := models.HostSensorSummary{
		TemperatureCelsius: map[string]float64{
			"cpu_core_0": 50.0,
			"cpu_core_1": 55.0,
		},
	}
	result := convertHostSensorsToTemperature(sensors, time.Now())

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// When no package temp, CPUPackage should use max core temp
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage to be max core temp 55.0, got %f", result.CPUPackage)
	}
}

func TestIsHostAgentTemperatureRecent(t *testing.T) {
	tests := []struct {
		name     string
		lastSeen time.Time
		expected bool
	}{
		{
			name:     "recent - just now",
			lastSeen: time.Now(),
			expected: true,
		},
		{
			name:     "recent - 1 minute ago",
			lastSeen: time.Now().Add(-1 * time.Minute),
			expected: true,
		},
		{
			name:     "stale - 3 minutes ago",
			lastSeen: time.Now().Add(-3 * time.Minute),
			expected: false,
		},
		{
			name:     "stale - 1 hour ago",
			lastSeen: time.Now().Add(-1 * time.Hour),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHostAgentTemperatureRecent(tt.lastSeen)
			if result != tt.expected {
				t.Errorf("isHostAgentTemperatureRecent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMergeTemperatureData_NilInputs(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		result := mergeTemperatureData(nil, nil)
		if result != nil {
			t.Error("expected nil for both nil inputs")
		}
	})

	t.Run("host nil", func(t *testing.T) {
		proxy := &models.Temperature{CPUPackage: 50.0}
		result := mergeTemperatureData(nil, proxy)
		if result != proxy {
			t.Error("expected proxy when host is nil")
		}
	})

	t.Run("proxy nil", func(t *testing.T) {
		host := &models.Temperature{CPUPackage: 55.0}
		result := mergeTemperatureData(host, nil)
		if result != host {
			t.Error("expected host when proxy is nil")
		}
	})
}

func TestMergeTemperatureData_Merge(t *testing.T) {
	hostTemp := &models.Temperature{
		CPUPackage: 55.0,
		CPUMax:     55.0,
		HasCPU:     true,
		Cores: []models.CoreTemp{
			{Core: 0, Temp: 50.0},
			{Core: 1, Temp: 55.0},
		},
		LastUpdate: time.Now(),
	}

	proxyTemp := &models.Temperature{
		CPUPackage:   52.0,
		CPUMin:       30.0,
		CPUMaxRecord: 60.0,
		HasCPU:       true,
		HasSMART:     true,
		SMART: []models.DiskTemp{
			{Device: "/dev/sda", Temperature: 35},
		},
	}

	result := mergeTemperatureData(hostTemp, proxyTemp)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Host agent CPU takes priority
	if result.CPUPackage != 55.0 {
		t.Errorf("expected CPUPackage 55.0 from host, got %f", result.CPUPackage)
	}

	// Proxy historical data preserved
	if result.CPUMin != 30.0 {
		t.Errorf("expected CPUMin 30.0 from proxy, got %f", result.CPUMin)
	}

	// SMART data from proxy preserved
	if !result.HasSMART {
		t.Error("expected HasSMART to be true")
	}
	if len(result.SMART) != 1 {
		t.Errorf("expected 1 SMART disk, got %d", len(result.SMART))
	}
}

func TestMergeTemperatureData_FallbackToProxy(t *testing.T) {
	// Host has no CPU data, should fall back to proxy
	hostTemp := &models.Temperature{
		HasGPU: true,
		GPU: []models.GPUTemp{
			{Device: "gpu0", Edge: 70.0},
		},
		LastUpdate: time.Now(),
	}

	proxyTemp := &models.Temperature{
		CPUPackage: 52.0,
		CPUMax:     52.0,
		HasCPU:     true,
		Cores: []models.CoreTemp{
			{Core: 0, Temp: 48.0},
		},
	}

	result := mergeTemperatureData(hostTemp, proxyTemp)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should fall back to proxy CPU data
	if result.CPUPackage != 52.0 {
		t.Errorf("expected CPUPackage 52.0 from proxy fallback, got %f", result.CPUPackage)
	}
	if len(result.Cores) != 1 {
		t.Errorf("expected 1 core from proxy, got %d", len(result.Cores))
	}

	// Host GPU data should be present
	if !result.HasGPU {
		t.Error("expected HasGPU to be true")
	}
	if len(result.GPU) != 1 {
		t.Errorf("expected 1 GPU from host, got %d", len(result.GPU))
	}
}
