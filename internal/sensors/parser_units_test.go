package sensors

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
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
		Native: []NativeSensor{
			nativeTemp("amdgpu-pci-0300", "", "temp1", "edge", 58, NativeSensorStatusOK),
			nativeTemp("applesmc-isa-0300", "ISA adapter", "temp1", "TB0T", 34.2, NativeSensorStatusOK),
			nativeTemp("applesmc-isa-0300", "ISA adapter", "temp2", "TB1T", 29.2, NativeSensorStatusOK),
			nativeTemp("applesmc-isa-0300", "ISA adapter", "temp3", "Th1H", 44.5, NativeSensorStatusOK),
			nativeTemp("applesmc-isa-0300", "ISA adapter", "temp4", "TG1D", -127, NativeSensorStatusInvalid),
			nativeTemp("applesmc-isa-0300", "ISA adapter", "temp5", "TCMc", 0, NativeSensorStatusOK),
			nativeFan("applesmc-isa-0300", "ISA adapter", "fan1", "Left side", 6152, NativeSensorStatusOK),
			nativeFan("applesmc-isa-0300", "ISA adapter", "fan2", "Right side", 5699, NativeSensorStatusOK),
			nativeTemp("coretemp-isa-0000", "", "temp1", "Package id 0", 91, NativeSensorStatusOK),
			nativeTemp("coretemp-isa-0000", "", "temp2", "Core 0", 87, NativeSensorStatusOK),
			nativeTemp("nvme-pci-0200", "", "temp1", "Composite", 36.9, NativeSensorStatusOK),
		},
	}, got, "compare the complete report, not only selected expected readings")
}

func nativeTemp(chip, adapter, channel, label string, value float64, status string) NativeSensor {
	return NativeSensor{ID: chip + "/" + channel, Chip: chip, Adapter: adapter, Channel: channel, Label: label,
		Kind: NativeSensorKindTemperature, Unit: NativeSensorUnitCelsius, Value: &value, Status: status}
}

func nativeFan(chip, adapter, channel, label string, value float64, status string) NativeSensor {
	return NativeSensor{ID: chip + "/" + channel, Chip: chip, Adapter: adapter, Channel: channel, Label: label,
		Kind: NativeSensorKindFan, Unit: NativeSensorUnitRPM, Value: &value, Status: status}
}

func TestParseNativeInventoryKeepsDistinctIdentities(t *testing.T) {
	// Two chips of one type and labels that differ only by case collide in
	// the legacy flat maps; the native inventory keeps every input apart.
	input := `{
		"nvme-pci-0100": {"Composite": {"temp1_input": 41}},
		"nvme-pci-0200": {"Composite": {"temp1_input": 47}},
		"applesmc-isa-0300": {"TC1F": {"temp1_input": 96.8}, "TC1f": {"temp2_input": 95.1}}
	}`
	for range 16 {
		got, err := Parse(input)
		require.NoError(t, err)
		require.Equal(t, []NativeSensor{
			nativeTemp("applesmc-isa-0300", "", "temp1", "TC1F", 96.8, NativeSensorStatusOK),
			nativeTemp("applesmc-isa-0300", "", "temp2", "TC1f", 95.1, NativeSensorStatusOK),
			nativeTemp("nvme-pci-0100", "", "temp1", "Composite", 41, NativeSensorStatusOK),
			nativeTemp("nvme-pci-0200", "", "temp1", "Composite", 47, NativeSensorStatusOK),
		}, got.Native, "inventory order and identity must not depend on map iteration")
		require.Len(t, got.Other, 1, "the legacy map still collapses the case-only pair")
	}
}

func TestParseNativeInventoryStatusAndUnits(t *testing.T) {
	got, err := Parse(`{"board-isa-0a20": {
		"Adapter": "ISA adapter",
		"SYSTIN": {"temp1_input": 31000, "temp1_max": 80},
		"AUXTIN": {"temp2_input": 150.5},
		"Cold": {"temp3_input": -55.5},
		"Probe": {"temp4_input": "nan"},
		"Floor": {"temp5_input": -55},
		"Ceiling": {"temp6_input": "150"},
		"Stopped": {"fan1_input": 0},
		"Broken": {"fan2_input": -1},
		"Vcore": {"in0_input": 1.1},
		"Odd": {"tempx_input": 40, "temp_input": 40, "temp07_input": 40, "fan1_min": 300}
	}}`)
	require.NoError(t, err)
	require.Equal(t, []NativeSensor{
		nativeTemp("board-isa-0a20", "ISA adapter", "temp1", "SYSTIN", 31000, NativeSensorStatusInvalid),
		nativeTemp("board-isa-0a20", "ISA adapter", "temp2", "AUXTIN", 150.5, NativeSensorStatusInvalid),
		nativeTemp("board-isa-0a20", "ISA adapter", "temp3", "Cold", -55.5, NativeSensorStatusInvalid),
		{ID: "board-isa-0a20/temp4", Chip: "board-isa-0a20", Adapter: "ISA adapter", Channel: "temp4", Label: "Probe",
			Kind: NativeSensorKindTemperature, Unit: NativeSensorUnitCelsius, Status: NativeSensorStatusInvalid},
		nativeTemp("board-isa-0a20", "ISA adapter", "temp5", "Floor", -55, NativeSensorStatusOK),
		nativeTemp("board-isa-0a20", "ISA adapter", "temp6", "Ceiling", 150, NativeSensorStatusOK),
		nativeFan("board-isa-0a20", "ISA adapter", "fan1", "Stopped", 0, NativeSensorStatusOK),
		nativeFan("board-isa-0a20", "ISA adapter", "fan2", "Broken", -1, NativeSensorStatusInvalid),
	}, got.Native, "values stay as reported (no millidegree guess), the band is inclusive, implausible values stay visible as invalid, a stopped fan is a reading, non-canonical channels are ignored")
	for _, sensor := range got.Native {
		if sensor.Value != nil {
			require.False(t, math.IsNaN(*sensor.Value) || math.IsInf(*sensor.Value, 0), "inventory values must be JSON encodable")
		}
	}
}

func TestNativeSensorChannelKindAndStatus(t *testing.T) {
	for channel, want := range map[string]string{"temp1": NativeSensorKindTemperature, "temp12": NativeSensorKindTemperature, "fan3": NativeSensorKindFan, "temp0": NativeSensorKindTemperature} {
		kind, ok := NativeSensorChannelKind(channel)
		require.True(t, ok, channel)
		require.Equal(t, want, kind, channel)
	}
	for _, channel := range []string{"", "temp", "fan", "temp01", "fan007", "tempx", "in0", "Temp1", "temp1_input", "temp-1"} {
		_, ok := NativeSensorChannelKind(channel)
		require.False(t, ok, "%q must not be a canonical channel", channel)
	}
	value := func(v float64) *float64 { return &v }
	require.Equal(t, NativeSensorStatusInvalid, NativeSensorStatus(NativeSensorKindTemperature, nil))
	require.Equal(t, NativeSensorStatusInvalid, NativeSensorStatus(NativeSensorKindTemperature, value(math.NaN())))
	require.Equal(t, NativeSensorStatusInvalid, NativeSensorStatus(NativeSensorKindFan, value(math.Inf(1))))
	require.Equal(t, NativeSensorStatusInvalid, NativeSensorStatus("voltage", value(1)))
	require.Equal(t, NativeSensorStatusOK, NativeSensorStatus(NativeSensorKindFan, value(0)))
}

func TestSortAndBoundNativeSensors(t *testing.T) {
	inventory := make([]NativeSensor, 0, MaxNativeSensors+11)
	for i := MaxNativeSensors + 9; i >= 0; i-- {
		inventory = append(inventory, nativeTemp("chip-isa-0000", "", "temp"+strconv.Itoa(i+1), "T", 40, NativeSensorStatusOK))
	}
	inventory = append(inventory, nativeTemp("chip-isa-0000", "", "temp1", "Duplicate", 40, NativeSensorStatusOK))
	got := sortAndBoundNativeSensors(inventory)
	require.Len(t, got, MaxNativeSensors)
	require.Equal(t, "chip-isa-0000/temp1", got[0].ID)
	require.Equal(t, "Duplicate", got[0].Label, "a repeated ID keeps the first label in sort order")
	require.Equal(t, "chip-isa-0000/temp2", got[1].ID, "the repeated ID is dropped")
	require.Equal(t, "chip-isa-0000/temp"+strconv.Itoa(MaxNativeSensors), got[MaxNativeSensors-1].ID, "truncation keeps the lowest channels")
	require.Nil(t, sortAndBoundNativeSensors(nil))
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
