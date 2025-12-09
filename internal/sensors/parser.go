package sensors

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/rs/zerolog/log"
)

// TemperatureData contains parsed temperature readings from sensors
type TemperatureData struct {
	CPUPackage float64            // Overall CPU package temperature
	CPUMax     float64            // Maximum CPU temperature
	Cores      map[string]float64 // Per-core temperatures (e.g., "Core 0": 45.0)
	NVMe       map[string]float64 // NVMe drive temperatures (e.g., "nvme0": 42.0)
	GPU        map[string]float64 // GPU temperatures (e.g., "amdgpu-pci-0400": 55.0)
	Available  bool               // Whether any temperature data was found
}

// Parse extracts temperature data from sensors -j JSON output
func Parse(jsonStr string) (*TemperatureData, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, fmt.Errorf("empty sensors output")
	}

	var sensorsData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &sensorsData); err != nil {
		return nil, fmt.Errorf("failed to parse sensors JSON: %w", err)
	}

	data := &TemperatureData{
		Cores: make(map[string]float64),
		NVMe:  make(map[string]float64),
		GPU:   make(map[string]float64),
	}

	foundCPUChip := false

	// Parse each sensor chip
	for chipName, chipData := range sensorsData {
		chipMap, ok := chipData.(map[string]interface{})
		if !ok {
			continue
		}

		chipLower := strings.ToLower(chipName)

		// Handle CPU temperature sensors
		if isCPUChip(chipLower) {
			foundCPUChip = true
			parseCPUTemps(chipMap, data)
		}

		// Handle NVMe temperature sensors
		if strings.Contains(chipName, "nvme") {
			parseNVMeTemps(chipName, chipMap, data)
		}

		// Handle GPU temperature sensors
		if strings.Contains(chipLower, "amdgpu") || strings.Contains(chipLower, "nouveau") {
			parseGPUTemps(chipName, chipMap, data)
		}
	}

	// If we got CPU temps, calculate max from cores if package not available
	if data.CPUPackage == 0 && len(data.Cores) > 0 {
		for _, temp := range data.Cores {
			if temp > data.CPUMax {
				data.CPUMax = temp
			}
		}
		// Use max core temp as package temp if not available
		data.CPUPackage = data.CPUMax
	}

	data.Available = foundCPUChip || len(data.NVMe) > 0 || len(data.GPU) > 0

	log.Debug().
		Bool("available", data.Available).
		Float64("cpuPackage", data.CPUPackage).
		Float64("cpuMax", data.CPUMax).
		Int("coreCount", len(data.Cores)).
		Int("nvmeCount", len(data.NVMe)).
		Int("gpuCount", len(data.GPU)).
		Msg("Parsed temperature data")

	return data, nil
}

func isCPUChip(chipLower string) bool {
	cpuChips := []string{
		"coretemp", "k10temp", "zenpower", "k8temp", "acpitz",
		"it87", "nct6687", "nct6775", "nct6776", "nct6779",
		"nct6791", "nct6792", "nct6793", "nct6795", "nct6796",
		"nct6797", "nct6798", "w83627", "f71882",
		"cpu_thermal", "rp1_adc", "rpitemp",
	}

	for _, chip := range cpuChips {
		if strings.Contains(chipLower, chip) {
			return true
		}
	}
	return false
}

func parseCPUTemps(chipMap map[string]interface{}, data *TemperatureData) {
	foundPackageTemp := false
	var chipletTemps []float64
	var genericTemp float64 // For chips that only report temp1

	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		sensorNameLower := strings.ToLower(sensorName)

		// Look for Package id (Intel) or Tdie/Tctl (AMD)
		if strings.Contains(sensorName, "Package id") ||
			strings.Contains(sensorName, "Tdie") ||
			strings.Contains(sensorNameLower, "tctl") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				data.CPUPackage = tempVal
				foundPackageTemp = true
				if tempVal > data.CPUMax {
					data.CPUMax = tempVal
				}
			}
		}

		// Capture generic temp1 for chips like cpu_thermal (RPi, ARM SoCs)
		// that don't have labeled sensors
		if sensorNameLower == "temp1" {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
				genericTemp = tempVal
			}
		}

		// Look for AMD chiplet temperatures
		if strings.HasPrefix(sensorName, "Tccd") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
				chipletTemps = append(chipletTemps, tempVal)
				if tempVal > data.CPUMax {
					data.CPUMax = tempVal
				}
			}
		}

		// Look for SuperIO chip CPU temperature fields
		if strings.Contains(sensorNameLower, "cputin") ||
			strings.Contains(sensorNameLower, "cpu temperature") ||
			(strings.Contains(sensorNameLower, "temp") && strings.Contains(sensorNameLower, "cpu")) {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
				if !foundPackageTemp {
					data.CPUPackage = tempVal
					foundPackageTemp = true
				}
				if tempVal > data.CPUMax {
					data.CPUMax = tempVal
				}
			}
		}

		// Look for individual core temperatures
		if strings.Contains(sensorName, "Core ") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				data.Cores[sensorName] = tempVal
				if tempVal > data.CPUMax {
					data.CPUMax = tempVal
				}
			}
		}
	}

	// If no package temp but we have chiplet temps, use highest chiplet
	if !foundPackageTemp && len(chipletTemps) > 0 {
		for _, temp := range chipletTemps {
			if temp > data.CPUPackage {
				data.CPUPackage = temp
			}
		}
	}

	// Fallback: use generic temp1 for chips like cpu_thermal (RPi, ARM SoCs)
	if !foundPackageTemp && data.CPUPackage == 0 && genericTemp > 0 {
		data.CPUPackage = genericTemp
		if genericTemp > data.CPUMax {
			data.CPUMax = genericTemp
		}
	}
}

func parseNVMeTemps(chipName string, chipMap map[string]interface{}, data *TemperatureData) {
	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		// Look for Composite temperature (main NVMe temp)
		if strings.Contains(sensorName, "Composite") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				// Normalize chip name to nvme0, nvme1 format
				// Input format is like "nvme-pci-0200" or "nvme-pci-0300"
				// We extract the device index based on how many NVMe devices we've seen
				normalizedName := fmt.Sprintf("nvme%d", len(data.NVMe))
				data.NVMe[normalizedName] = tempVal
				log.Debug().
					Str("chip", chipName).
					Str("normalizedName", normalizedName).
					Float64("temp", tempVal).
					Msg("Found NVMe temperature")
			}
		}
	}
}

func parseGPUTemps(chipName string, chipMap map[string]interface{}, data *TemperatureData) {
	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		sensorNameLower := strings.ToLower(sensorName)

		// Look for GPU temperature fields
		if strings.Contains(sensorNameLower, "edge") ||
			strings.Contains(sensorNameLower, "junction") ||
			strings.Contains(sensorNameLower, "mem") ||
			strings.Contains(sensorNameLower, "temp1") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				// Use sensor name as key (e.g., "edge", "junction")
				key := fmt.Sprintf("%s_%s", chipName, sensorName)
				data.GPU[key] = tempVal
				log.Debug().
					Str("chip", chipName).
					Str("sensor", sensorName).
					Float64("temp", tempVal).
					Msg("Found GPU temperature")
			}
		}
	}
}

func extractTempInput(sensorMap map[string]interface{}) float64 {
	// Look for temp*_input field (the actual temperature reading)
	for key, value := range sensorMap {
		if strings.HasSuffix(key, "_input") {
			switch v := value.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case string:
				// Raspberry Pi reports in millidegrees as string
				var milliTemp float64
				if _, err := fmt.Sscanf(v, "%f", &milliTemp); err == nil {
					// Convert from millidegrees to degrees
					return milliTemp / 1000.0
				}
			}
		}
	}
	return math.NaN()
}
