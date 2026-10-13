package models

import (
	"math"
	"strings"
)

// PrimaryCPUTemperatureCelsius is Pulse's one rule for a machine's headline
// CPU temperature: the hottest current CPU reading, taken from the package
// (Intel Package id, AMD Tctl/Tdie), the producer's CPU maximum, or any core.
//
// Every surface that names a single CPU temperature reads this value: node and
// agent metrics history, the unified resource temperature, node temperature
// alerts and the CPU low/record tracking. The node drawer shows the resource
// value instead of recomputing it, so its headline matches the line drawn
// beneath it. The hottest reading wins because it is the one nearest a thermal
// limit: AMD Tctl and Tccd, or a core running above the package, can differ by
// several degrees.
func PrimaryCPUTemperatureCelsius(temperature *Temperature) *float64 {
	if temperature == nil || !temperature.Available {
		return nil
	}
	var best float64
	found := false
	consider := func(value float64) {
		if !usableCPUTemperature(value) {
			return
		}
		if !found || value > best {
			best = value
			found = true
		}
	}
	consider(temperature.CPUPackage)
	consider(temperature.CPUMax)
	for _, core := range temperature.Cores {
		consider(core.Temp)
	}
	if !found {
		return nil
	}
	return &best
}

// HostSensorPrimaryCPUTemperatureCelsius applies PrimaryCPUTemperatureCelsius
// to an agent's flat sensor map: the hottest reading whose key names a CPU
// sensor ("cpu_package", "cpu_core_N"). For the keys the agent reports this
// equals PrimaryCPUTemperatureCelsius of the node temperature built from the
// same map, so a linked node and its agent carry one value.
func HostSensorPrimaryCPUTemperatureCelsius(sensors HostSensorSummary) *float64 {
	var best float64
	found := false
	for key, value := range sensors.TemperatureCelsius {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "cpu") ||
			!usableCPUTemperature(value) {
			continue
		}
		if !found || value > best {
			best = value
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func usableCPUTemperature(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
