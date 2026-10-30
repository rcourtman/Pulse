package sensors

import "testing"

func TestIsTemperatureInput(t *testing.T) {
	for _, key := range []string{"temp1_input", "temp12_input", "temp999_input"} {
		if !IsTemperatureInput(key) {
			t.Errorf("temperature key %q rejected", key)
		}
	}
	for _, key := range []string{"", "fan1_input", "in1_input", "curr1_input", "power1_input", "freq1_input", "energy1_input", "humidity1_input", "temp1_max", "temp1_crit", "temp1_label", "temp_input", "temperature_input", "temp-1_input", "temp１_input", "Temp1_input", "temp1_input_extra", "prefix_temp1_input"} {
		if IsTemperatureInput(key) {
			t.Errorf("non-temperature key %q admitted as Celsius", key)
		}
	}
}
