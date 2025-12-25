package sensors

import (
	"math"
	"testing"
)

func TestParse_IntelCPU(t *testing.T) {
	// Typical Intel CPU output from sensors -j
	intelJSON := `{
		"coretemp-isa-0000": {
			"Adapter": "ISA adapter",
			"Package id 0": {
				"temp1_input": 45.000,
				"temp1_max": 100.000,
				"temp1_crit": 100.000,
				"temp1_crit_alarm": 0.000
			},
			"Core 0": {
				"temp2_input": 43.000,
				"temp2_max": 100.000,
				"temp2_crit": 100.000,
				"temp2_crit_alarm": 0.000
			},
			"Core 1": {
				"temp3_input": 44.000,
				"temp3_max": 100.000,
				"temp3_crit": 100.000,
				"temp3_crit_alarm": 0.000
			},
			"Core 2": {
				"temp4_input": 42.000,
				"temp4_max": 100.000,
				"temp4_crit": 100.000,
				"temp4_crit_alarm": 0.000
			},
			"Core 3": {
				"temp5_input": 45.000,
				"temp5_max": 100.000,
				"temp5_crit": 100.000,
				"temp5_crit_alarm": 0.000
			}
		}
	}`

	data, err := Parse(intelJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if data.CPUPackage != 45.0 {
		t.Errorf("CPUPackage = %v, want 45.0", data.CPUPackage)
	}

	if data.CPUMax != 45.0 {
		t.Errorf("CPUMax = %v, want 45.0", data.CPUMax)
	}

	if len(data.Cores) != 4 {
		t.Errorf("len(Cores) = %d, want 4", len(data.Cores))
	}

	if data.Cores["Core 0"] != 43.0 {
		t.Errorf("Core 0 = %v, want 43.0", data.Cores["Core 0"])
	}
}

func TestParse_AMDCPU(t *testing.T) {
	// Typical AMD Ryzen output from sensors -j
	amdJSON := `{
		"k10temp-pci-00c3": {
			"Adapter": "PCI adapter",
			"Tctl": {
				"temp1_input": 52.000
			},
			"Tdie": {
				"temp2_input": 52.000
			},
			"Tccd1": {
				"temp3_input": 48.000
			},
			"Tccd2": {
				"temp4_input": 50.000
			}
		}
	}`

	data, err := Parse(amdJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	// Should use Tdie as package temp
	if data.CPUPackage != 52.0 {
		t.Errorf("CPUPackage = %v, want 52.0", data.CPUPackage)
	}

	if data.CPUMax != 52.0 {
		t.Errorf("CPUMax = %v, want 52.0", data.CPUMax)
	}
}

func TestParse_NVMe(t *testing.T) {
	nvmeJSON := `{
		"nvme-pci-0100": {
			"Adapter": "PCI adapter",
			"Composite": {
				"temp1_input": 38.850,
				"temp1_max": 81.850,
				"temp1_min": -273.150,
				"temp1_crit": 84.850,
				"temp1_alarm": 0.000
			},
			"Sensor 1": {
				"temp2_input": 38.850,
				"temp2_max": 65261.850,
				"temp2_min": -273.150
			}
		},
		"nvme-pci-0200": {
			"Adapter": "PCI adapter",
			"Composite": {
				"temp1_input": 42.000
			}
		}
	}`

	data, err := Parse(nvmeJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if len(data.NVMe) != 2 {
		t.Errorf("len(NVMe) = %d, want 2", len(data.NVMe))
	}

	if data.NVMe["nvme0"] != 38.85 {
		t.Errorf("nvme0 = %v, want 38.85", data.NVMe["nvme0"])
	}

	if data.NVMe["nvme1"] != 42.0 {
		t.Errorf("nvme1 = %v, want 42.0", data.NVMe["nvme1"])
	}
}

func TestParse_GPU(t *testing.T) {
	gpuJSON := `{
		"amdgpu-pci-0300": {
			"Adapter": "PCI adapter",
			"edge": {
				"temp1_input": 55.000
			},
			"junction": {
				"temp2_input": 58.000
			},
			"mem": {
				"temp3_input": 52.000
			}
		}
	}`

	data, err := Parse(gpuJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if len(data.GPU) != 3 {
		t.Errorf("len(GPU) = %d, want 3", len(data.GPU))
	}

	// Check edge temp
	edgeKey := "amdgpu-pci-0300_edge"
	if data.GPU[edgeKey] != 55.0 {
		t.Errorf("GPU[%s] = %v, want 55.0", edgeKey, data.GPU[edgeKey])
	}
}

func TestParse_Combined(t *testing.T) {
	// Combined CPU + NVMe + GPU output
	combinedJSON := `{
		"coretemp-isa-0000": {
			"Package id 0": {"temp1_input": 50.000},
			"Core 0": {"temp2_input": 48.000}
		},
		"nvme-pci-0100": {
			"Composite": {"temp1_input": 40.000}
		},
		"amdgpu-pci-0300": {
			"edge": {"temp1_input": 60.000}
		}
	}`

	data, err := Parse(combinedJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if data.CPUPackage != 50.0 {
		t.Errorf("CPUPackage = %v, want 50.0", data.CPUPackage)
	}

	if len(data.NVMe) != 1 {
		t.Errorf("len(NVMe) = %d, want 1", len(data.NVMe))
	}

	if data.NVMe["nvme0"] != 40.0 {
		t.Errorf("nvme0 = %v, want 40.0", data.NVMe["nvme0"])
	}

	if len(data.GPU) != 1 {
		t.Errorf("len(GPU) = %d, want 1", len(data.GPU))
	}
}

func TestParse_Empty(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("Parse() should fail on empty input")
	}

	_, err = Parse("   ")
	if err == nil {
		t.Error("Parse() should fail on whitespace-only input")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse("not valid json")
	if err == nil {
		t.Error("Parse() should fail on invalid JSON")
	}

	_, err = Parse("{incomplete")
	if err == nil {
		t.Error("Parse() should fail on incomplete JSON")
	}
}

func TestParse_EmptyObject(t *testing.T) {
	data, err := Parse("{}")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if data.Available {
		t.Error("Expected Available to be false for empty object")
	}
}

func TestParse_NoRecognizedSensors(t *testing.T) {
	// JSON with unknown sensor types - should still be captured in Other map
	unknownJSON := `{
		"unknown-chip-0000": {
			"Adapter": "Unknown adapter",
			"some_sensor": {"temp1_input": 50.000}
		}
	}`

	data, err := Parse(unknownJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Unknown sensors should now be captured in Other, making data available
	if !data.Available {
		t.Error("Expected Available to be true for sensors in Other map")
	}

	if len(data.Other) != 1 {
		t.Errorf("len(Other) = %d, want 1", len(data.Other))
	}

	// Check that the sensor was captured with normalized key
	expectedKey := "unknown_some_sensor"
	if data.Other[expectedKey] != 50.0 {
		t.Errorf("Other[%s] = %v, want 50.0", expectedKey, data.Other[expectedKey])
	}
}

func TestParse_SuperIOChip(t *testing.T) {
	// NCT6775 SuperIO chip output
	superioJSON := `{
		"nct6775-isa-0290": {
			"Adapter": "ISA adapter",
			"CPUTIN": {
				"temp1_input": 45.000,
				"temp1_max": 80.000,
				"temp1_max_hyst": 75.000
			},
			"SYSTIN": {
				"temp2_input": 35.000
			}
		}
	}`

	data, err := Parse(superioJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if data.CPUPackage != 45.0 {
		t.Errorf("CPUPackage = %v, want 45.0", data.CPUPackage)
	}
}

func TestParse_RaspberryPi(t *testing.T) {
	// Raspberry Pi thermal zone output
	rpiJSON := `{
		"cpu_thermal-virtual-0": {
			"Adapter": "Virtual device",
			"temp1": {
				"temp1_input": 52.000
			}
		}
	}`

	data, err := Parse(rpiJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}
}

func TestIsCPUChip(t *testing.T) {
	tests := []struct {
		chip     string
		expected bool
	}{
		{"coretemp-isa-0000", true},
		{"k10temp-pci-00c3", true},
		{"zenpower-pci-00c3", true},
		{"nct6775-isa-0290", true},
		{"cpu_thermal-virtual-0", true},
		{"acpitz-acpi-0", true},
		{"rp1_adc-isa-0000", true}, // Raspberry Pi RP1 ADC
		{"nvme-pci-0100", false},
		{"amdgpu-pci-0300", false},
		{"unknown-chip", false},
	}

	for _, tc := range tests {
		t.Run(tc.chip, func(t *testing.T) {
			result := isCPUChip(tc.chip)
			if result != tc.expected {
				t.Errorf("isCPUChip(%q) = %v, want %v", tc.chip, result, tc.expected)
			}
		})
	}
}

func TestExtractTempInput(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected float64
		isNaN    bool
	}{
		{
			name:     "float value",
			input:    map[string]interface{}{"temp1_input": 45.5},
			expected: 45.5,
		},
		{
			name:     "int value",
			input:    map[string]interface{}{"temp1_input": 45},
			expected: 45.0,
		},
		{
			name:  "no input field",
			input: map[string]interface{}{"temp1_max": 100.0},
			isNaN: true,
		},
		{
			name:  "empty map",
			input: map[string]interface{}{},
			isNaN: true,
		},
		{
			name:  "wrong suffix",
			input: map[string]interface{}{"temp1_max": 45.0},
			isNaN: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTempInput(tc.input)
			if tc.isNaN {
				if !math.IsNaN(result) {
					t.Errorf("extractTempInput() = %v, want NaN", result)
				}
			} else {
				if result != tc.expected {
					t.Errorf("extractTempInput() = %v, want %v", result, tc.expected)
				}
			}
		})
	}
}

func TestParse_MaxTempCalculation(t *testing.T) {
	// Test that CPUMax is correctly calculated from cores when no package temp
	coresOnlyJSON := `{
		"coretemp-isa-0000": {
			"Core 0": {"temp2_input": 40.000},
			"Core 1": {"temp3_input": 45.000},
			"Core 2": {"temp4_input": 42.000},
			"Core 3": {"temp5_input": 48.000}
		}
	}`

	data, err := Parse(coresOnlyJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should use max core temp as package temp
	if data.CPUPackage != 48.0 {
		t.Errorf("CPUPackage = %v, want 48.0 (max core temp)", data.CPUPackage)
	}

	if data.CPUMax != 48.0 {
		t.Errorf("CPUMax = %v, want 48.0", data.CPUMax)
	}
}

func TestParse_NouveauGPU(t *testing.T) {
	// Nouveau (open-source NVIDIA) driver output
	nouveauJSON := `{
		"nouveau-pci-0100": {
			"Adapter": "PCI adapter",
			"temp1": {
				"temp1_input": 45.000
			}
		}
	}`

	data, err := Parse(nouveauJSON)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Error("Expected Available to be true")
	}

	if len(data.GPU) != 1 {
		t.Errorf("len(GPU) = %d, want 1", len(data.GPU))
	}
}
