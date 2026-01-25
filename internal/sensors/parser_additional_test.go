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
