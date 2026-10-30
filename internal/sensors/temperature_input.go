package sensors

import "strings"

// IsTemperatureInput identifies an lm-sensors temperature subfeature by its
// native tempN_input key. The displayed feature label can be arbitrary; a
// fan, voltage, current or power input does not acquire Celsius units from it.
func IsTemperatureInput(key string) bool {
	index := strings.TrimSuffix(strings.TrimPrefix(key, "temp"), "_input")
	if index == "" || key != "temp"+index+"_input" {
		return false
	}
	for _, digit := range index {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
