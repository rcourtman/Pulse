package sensors

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTempInputKeepsUnits(t *testing.T) {
	for _, key := range []string{"fan1_input", "in1_input", "curr1_input", "power1_input", "freq1_input", "energy1_input", "humidity1_input", "temperature_input"} {
		for _, value := range []any{6152.0, 45, "+45.0°C"} {
			require.True(t, math.IsNaN(extractTempInput(map[string]any{key: value})), "%s must not become Celsius", key)
		}
	}
	// The true subfeature, not map iteration order or a displayed label,
	// supplies the value when a feature includes measurements in other units.
	for range 32 {
		require.Equal(t, 34.2, extractTempInput(map[string]any{"fan1_input": 6152.0, "in1_input": 12.1, "temp12_input": 34.2}))
	}
}

func TestParseMixedSensorUnits(t *testing.T) {
	// Inferred lm-sensors JSON based on #3565's human-readable output, with
	// explicit non-temperature controls. This is not an original host export.
	input, err := os.ReadFile("testdata/mixed_sensor_units.json")
	require.NoError(t, err)
	got, err := Parse(string(input))
	require.NoError(t, err)
	require.Equal(t, &TemperatureData{
		CPUPackage: 91, CPUMax: 91, Available: true,
		Cores: map[string]float64{"Core 0": 87},
		NVMe:  map[string]float64{"nvme0": 36.9},
		GPU:   map[string]float64{"amdgpu-pci-0300_edge": 58},
		Fans:  map[string]float64{"applesmc_left_side": 6152, "applesmc_right_side": 5699},
		Other: map[string]float64{"applesmc_tb0t": 34.2, "applesmc_tb1t": 29.2, "applesmc_th1h": 44.5},
	}, got, "compare the complete report, not only selected expected readings")
}

func TestParseNonTemperatureInputsCannotImpersonateTypedReadings(t *testing.T) {
	for _, chip := range []struct{ name, label string }{
		{"coretemp-isa-0000", "Package id 0"},
		{"coretemp-isa-0000", "Core 0"},
		{"k10temp-pci-00c3", "Tctl"},
		{"nvme-pci-0200", "Composite"},
		{"amdgpu-pci-0300", "edge"},
		{"nouveau-pci-0400", "edge"},
		{"applesmc-isa-0300", "TB0T"},
	} {
		for _, key := range []string{"fan1_input", "in1_input", "curr1_input", "power1_input", "freq1_input"} {
			t.Run(chip.name+"/"+chip.label+"/"+key, func(t *testing.T) {
				input, err := json.Marshal(map[string]any{chip.name: map[string]any{chip.label: map[string]any{key: 45.0}}})
				require.NoError(t, err)
				got, err := Parse(string(input))
				require.NoError(t, err)
				require.Zero(t, got.CPUPackage)
				require.Zero(t, got.CPUMax)
				require.Empty(t, got.Cores)
				require.Empty(t, got.NVMe)
				require.Empty(t, got.GPU)
				require.Empty(t, got.Other)
			})
		}
	}
}

func TestParseNonTemperatureOnlyKeepsMissingDataSemantics(t *testing.T) {
	got, err := Parse(`{"board-isa-0000":{"Supply":{"in1_input":12.1},"Power":{"power1_input":45}}}`)
	require.NoError(t, err)
	require.False(t, got.Available, "non-temperature inputs must not invent temperature availability")
	got, err = Parse(`{"board-isa-0000":{"Fan":{"fan1_input":6152}}}`)
	require.NoError(t, err)
	require.True(t, got.Available, "existing fan-only availability stays supported")
	require.Equal(t, map[string]float64{"board_fan": 6152}, got.Fans)
	require.Empty(t, got.Other)
	require.Zero(t, got.CPUPackage, "fan-only output must not invent a CPU source")
}
