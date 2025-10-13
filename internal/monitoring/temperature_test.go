package monitoring

import "testing"

func TestParseSensorsJSON_NoTemperatureData(t *testing.T) {
	collector := &TemperatureCollector{}

	// Test with a chip that doesn't match any known CPU or NVMe patterns
	jsonStr := `{
		"unknown-sensor-0": {
			"Adapter": "Unknown interface",
			"temp1": {
				"temp1_label": "temp1"
			}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if temp.Available {
		t.Fatalf("expected temperature to be unavailable when no CPU or NVMe chips are detected")
	}
	if temp.HasCPU {
		t.Fatalf("expected HasCPU to be false when no CPU chip detected")
	}
	if temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be false when no NVMe chip detected")
	}
}

func TestParseSensorsJSON_WithCpuAndNvmeData(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 45.5},
			"Core 0": {"temp2_input": 43.0},
			"Core 1": {"temp3_input": 44.2}
		},
		"nvme-pci-0400": {
			"Composite": {"temp1_input": 38.75}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	if !temp.Available {
		t.Fatalf("expected temperature to be available when readings are present")
	}
	if temp.CPUPackage != 45.5 {
		t.Fatalf("expected cpu package temperature 45.5, got %.2f", temp.CPUPackage)
	}
	if temp.CPUMax <= 0 {
		t.Fatalf("expected cpu max temperature to be greater than zero, got %.2f", temp.CPUMax)
	}
	if len(temp.Cores) != 2 {
		t.Fatalf("expected two core temperatures, got %d", len(temp.Cores))
	}
	if len(temp.NVMe) != 1 {
		t.Fatalf("expected one NVMe temperature, got %d", len(temp.NVMe))
	}
	if temp.NVMe[0].Temp != 38.75 {
		t.Fatalf("expected NVMe temperature 38.75, got %.2f", temp.NVMe[0].Temp)
	}
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when CPU data present")
	}
	if !temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be true when NVMe data present")
	}
}

// TestParseSensorsJSON_NVMeOnly tests that NVMe-only systems don't show "No CPU sensor"
func TestParseSensorsJSON_NVMeOnly(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"nvme-pci-0400": {
			"Composite": {"temp1_input": 42.5}
		},
		"nvme-pci-0500": {
			"Composite": {"temp1_input": 38.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	// available should be true (any temperature data exists)
	if !temp.Available {
		t.Fatalf("expected temperature to be available when NVMe readings are present")
	}
	// hasCPU should be false (no CPU temperature data)
	if temp.HasCPU {
		t.Fatalf("expected HasCPU to be false when only NVMe data present")
	}
	// hasNVMe should be true
	if !temp.HasNVMe {
		t.Fatalf("expected HasNVMe to be true when NVMe data present")
	}
	// Verify NVMe data was parsed correctly
	if len(temp.NVMe) != 2 {
		t.Fatalf("expected two NVMe temperatures, got %d", len(temp.NVMe))
	}
	// Check that both expected temperatures are present (order may vary)
	foundTemps := make(map[float64]bool)
	for _, nvme := range temp.NVMe {
		foundTemps[nvme.Temp] = true
	}
	if !foundTemps[42.5] {
		t.Fatalf("expected to find NVMe temperature 42.5")
	}
	if !foundTemps[38.0] {
		t.Fatalf("expected to find NVMe temperature 38.0")
	}
}

// TestParseSensorsJSON_ZeroTemperature tests that HasCPU is true even when sensor reports 0°C
func TestParseSensorsJSON_ZeroTemperature(t *testing.T) {
	collector := &TemperatureCollector{}

	jsonStr := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 0.0},
			"Core 0": {"temp2_input": 0.0}
		}
	}`

	temp, err := collector.parseSensorsJSON(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error parsing sensors output: %v", err)
	}
	if temp == nil {
		t.Fatalf("expected temperature struct, got nil")
	}
	// hasCPU should be true because coretemp chip was detected, even though values are 0
	if !temp.HasCPU {
		t.Fatalf("expected HasCPU to be true when CPU chip is detected (even with 0°C readings)")
	}
	// available should be true because we have a CPU sensor
	if !temp.Available {
		t.Fatalf("expected temperature to be available when CPU chip is detected")
	}
	// Values should be accepted (not filtered out)
	if temp.CPUPackage != 0.0 {
		t.Fatalf("expected CPUPackage to be 0.0, got %.2f", temp.CPUPackage)
	}
	if len(temp.Cores) != 1 {
		t.Fatalf("expected one core temperature, got %d", len(temp.Cores))
	}
	if temp.Cores[0].Temp != 0.0 {
		t.Fatalf("expected core temperature to be 0.0, got %.2f", temp.Cores[0].Temp)
	}
}
