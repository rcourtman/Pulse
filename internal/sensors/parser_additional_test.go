package sensors

import (
	"math"
	"testing"
)

func TestNormalizeSensorKey(t *testing.T) {
	tests := []struct {
		chipName   string
		sensorName string
		expected   string
	}{
		{"nct6687-isa-0a20", "CPU Fan", "nct6687_cpu_fan"},
		{"amdgpu-pci-0400", "edge-temp", "amdgpu_edge_temp"},
		{"nvme-pci-0100", "Composite", "nvme_composite"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := normalizeSensorKey(tc.chipName, tc.sensorName)
			if got != tc.expected {
				t.Fatalf("normalizeSensorKey(%q, %q) = %q, want %q", tc.chipName, tc.sensorName, got, tc.expected)
			}
		})
	}
}

func TestExtractNumericValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected float64
	}{
		{"float64", 12.5, 12.5},
		{"int", 42, 42},
		{"int64", int64(99), 99},
		{"string", "nope", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractNumericValue(tc.value)
			if got != tc.expected {
				t.Fatalf("extractNumericValue(%v) = %v, want %v", tc.value, got, tc.expected)
			}
		})
	}
}

func TestExtractNVMeCompositeTemp(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		chipMap := map[string]interface{}{
			"Composite": map[string]interface{}{
				"temp1_input": 42.0,
			},
			"Other": map[string]interface{}{
				"temp1_input": 30.0,
			},
		}

		temp, ok := extractNVMeCompositeTemp(chipMap)
		if !ok {
			t.Fatalf("expected composite temp to be found")
		}
		if temp != 42.0 {
			t.Fatalf("temp = %v, want 42.0", temp)
		}
	})

	t.Run("missing", func(t *testing.T) {
		chipMap := map[string]interface{}{
			"Temperature 2": map[string]interface{}{
				"temp1_input": 30.0,
			},
		}

		temp, ok := extractNVMeCompositeTemp(chipMap)
		if ok {
			t.Fatalf("expected composite temp to be missing, got %v", temp)
		}
		if temp != 0 {
			t.Fatalf("temp = %v, want 0", temp)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		chipMap := map[string]interface{}{
			"Composite": map[string]interface{}{
				"temp1_input": -1.0,
			},
		}

		temp, ok := extractNVMeCompositeTemp(chipMap)
		if ok {
			t.Fatalf("expected composite temp to be rejected, got %v", temp)
		}
		if !math.IsNaN(temp) && temp != 0 {
			t.Fatalf("temp = %v, want 0", temp)
		}
	})
}

func TestExtractTempInput_StringTemperatures(t *testing.T) {
	t.Run("preserves normal celsius values", func(t *testing.T) {
		sensorMap := map[string]interface{}{
			"temp1_input": "+45.5°C",
		}

		got := extractTempInput(sensorMap)
		if math.Abs(got-45.5) > 0.0001 {
			t.Fatalf("extractTempInput() = %v, want 45.5", got)
		}
	})

	t.Run("converts millidegree strings", func(t *testing.T) {
		sensorMap := map[string]interface{}{
			"temp1_input": "42000",
		}

		got := extractTempInput(sensorMap)
		if math.Abs(got-42.0) > 0.0001 {
			t.Fatalf("extractTempInput() = %v, want 42.0", got)
		}
	})
}

func TestParse_WithStringTemperatureValues(t *testing.T) {
	input := `{
		"coretemp-isa-0000": {
			"Package id 0": {
				"temp1_input": "+50.0°C"
			},
			"Core 0": {
				"temp2_input": "48.0"
			}
		},
		"nvme-pci-0100": {
			"Composite": {
				"temp1_input": "39000"
			}
		}
	}`

	data, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Fatal("Expected Available to be true")
	}

	if math.Abs(data.CPUPackage-50.0) > 0.0001 {
		t.Fatalf("CPUPackage = %v, want 50.0", data.CPUPackage)
	}

	if math.Abs(data.Cores["Core 0"]-48.0) > 0.0001 {
		t.Fatalf("Core 0 = %v, want 48.0", data.Cores["Core 0"])
	}

	if math.Abs(data.NVMe["nvme0"]-39.0) > 0.0001 {
		t.Fatalf("nvme0 = %v, want 39.0", data.NVMe["nvme0"])
	}
}

func TestExtractTempInput_NumericMillidegrees(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected float64
	}{
		{
			name: "float millidegrees",
			input: map[string]interface{}{
				"temp1_input": 42000.0,
			},
			expected: 42.0,
		},
		{
			name: "int millidegrees",
			input: map[string]interface{}{
				"temp1_input": 55000,
			},
			expected: 55.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTempInput(tc.input)
			if math.Abs(got-tc.expected) > 0.0001 {
				t.Fatalf("extractTempInput(%v) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestParse_RaspberryPiMillidegreeFallbackShape(t *testing.T) {
	input := `{
		"cpu_thermal-virtual-0": {
			"temp1": {
				"temp1_input": 42000
			}
		}
	}`

	data, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if !data.Available {
		t.Fatal("Expected Available to be true")
	}
	if math.Abs(data.CPUPackage-42.0) > 0.0001 {
		t.Fatalf("CPUPackage = %v, want 42.0", data.CPUPackage)
	}
	if math.Abs(data.CPUMax-42.0) > 0.0001 {
		t.Fatalf("CPUMax = %v, want 42.0", data.CPUMax)
	}
}
