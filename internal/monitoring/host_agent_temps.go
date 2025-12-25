package monitoring

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// getHostAgentTemperature looks for a matching host agent and converts
// its sensor data to the Temperature model used by Proxmox nodes.
// It first tries to match by nodeID using the LinkedNodeID field (preferred for
// duplicate hostname scenarios), then falls back to hostname matching.
// Returns nil if no matching host agent is found or if no temperature data is available.
func (m *Monitor) getHostAgentTemperature(nodeName string) *models.Temperature {
	return m.getHostAgentTemperatureByID("", nodeName)
}

// getHostAgentTemperatureByID looks for a matching host agent by node ID first,
// then falls back to hostname matching. This correctly handles clusters where
// multiple nodes may have the same hostname (e.g., "px1" on different IPs).
func (m *Monitor) getHostAgentTemperatureByID(nodeID, nodeName string) *models.Temperature {
	if m.state == nil {
		return nil
	}

	hosts := m.state.GetHosts()
	if len(hosts) == 0 {
		return nil
	}

	var matchedHost *models.Host

	// First, try to find a host agent that is explicitly linked to this node
	// via LinkedNodeID. This is the most reliable method and handles duplicate
	// hostnames correctly.
	if nodeID != "" {
		for i := range hosts {
			if hosts[i].LinkedNodeID == nodeID {
				matchedHost = &hosts[i]
				log.Debug().
					Str("nodeID", nodeID).
					Str("hostAgentID", hosts[i].ID).
					Str("hostname", hosts[i].Hostname).
					Msg("Matched host agent to node via LinkedNodeID")
				break
			}
		}
	}

	// Fallback: match by hostname if no linked host was found
	// This maintains backwards compatibility for setups where linking hasn't occurred yet
	if matchedHost == nil {
		nodeLower := strings.ToLower(strings.TrimSpace(nodeName))
		for i := range hosts {
			hostnameLower := strings.ToLower(strings.TrimSpace(hosts[i].Hostname))
			if hostnameLower == nodeLower {
				matchedHost = &hosts[i]
				break
			}
		}
	}

	if matchedHost == nil {
		return nil
	}

	// Check if the host agent has temperature data
	if len(matchedHost.Sensors.TemperatureCelsius) == 0 {
		return nil
	}

	// Convert host agent sensor data to Temperature model
	return convertHostSensorsToTemperature(matchedHost.Sensors, matchedHost.LastSeen)
}

// convertHostSensorsToTemperature converts HostSensorSummary to the Temperature model.
// The host agent reports temperatures in a flat map with keys like:
// - "cpu_package" -> CPU package temperature
// - "cpu_core_0", "cpu_core_1", etc. -> individual core temperatures
// - "nvme0", "nvme1", etc. -> NVMe temperatures
// - "gpu_edge", "gpu_junction", etc. -> GPU temperatures
func convertHostSensorsToTemperature(sensors models.HostSensorSummary, lastSeen time.Time) *models.Temperature {
	if len(sensors.TemperatureCelsius) == 0 {
		return nil
	}

	temp := &models.Temperature{
		Available:  true,
		LastUpdate: lastSeen,
		Cores:      []models.CoreTemp{},
		NVMe:       []models.NVMeTemp{},
		GPU:        []models.GPUTemp{},
	}

	var coreTemps []float64
	corePattern := regexp.MustCompile(`^cpu_core_(\d+)$`)
	nvmePattern := regexp.MustCompile(`^(nvme\d+)$`)
	gpuPattern := regexp.MustCompile(`^gpu_(.+)$`)

	for key, value := range sensors.TemperatureCelsius {
		keyLower := strings.ToLower(key)

		// CPU package temperature
		if keyLower == "cpu_package" {
			temp.CPUPackage = value
			temp.HasCPU = true
			continue
		}

		// CPU core temperatures
		if matches := corePattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			coreNum, err := strconv.Atoi(matches[1])
			if err == nil {
				temp.Cores = append(temp.Cores, models.CoreTemp{
					Core: coreNum,
					Temp: value,
				})
				coreTemps = append(coreTemps, value)
				temp.HasCPU = true
			}
			continue
		}

		// NVMe temperatures
		if matches := nvmePattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			temp.NVMe = append(temp.NVMe, models.NVMeTemp{
				Device: matches[1],
				Temp:   value,
			})
			temp.HasNVMe = true
			continue
		}

		// GPU temperatures (gpu_edge, gpu_junction, gpu_mem, or generic gpu_<device>)
		if matches := gpuPattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			gpuKey := matches[1]
			// Handle specific GPU temp types
			if gpuKey == "edge" || gpuKey == "junction" || gpuKey == "mem" {
				// Find or create GPU entry for the default device
				found := false
				for i := range temp.GPU {
					if temp.GPU[i].Device == "gpu0" {
						switch gpuKey {
						case "edge":
							temp.GPU[i].Edge = value
						case "junction":
							temp.GPU[i].Junction = value
						case "mem":
							temp.GPU[i].Mem = value
						}
						found = true
						break
					}
				}
				if !found {
					gpu := models.GPUTemp{Device: "gpu0"}
					switch gpuKey {
					case "edge":
						gpu.Edge = value
					case "junction":
						gpu.Junction = value
					case "mem":
						gpu.Mem = value
					}
					temp.GPU = append(temp.GPU, gpu)
				}
			} else {
				// Generic GPU entry with edge temp
				temp.GPU = append(temp.GPU, models.GPUTemp{
					Device: gpuKey,
					Edge:   value,
				})
			}
			temp.HasGPU = true
			continue
		}
	}

	// Sort cores by core number
	sort.Slice(temp.Cores, func(i, j int) bool {
		return temp.Cores[i].Core < temp.Cores[j].Core
	})

	// Sort NVMe by device name
	sort.Slice(temp.NVMe, func(i, j int) bool {
		return temp.NVMe[i].Device < temp.NVMe[j].Device
	})

	// Calculate CPUMax from core temperatures if package temp wasn't available
	if len(coreTemps) > 0 {
		maxTemp := 0.0
		for _, t := range coreTemps {
			if t > maxTemp {
				maxTemp = t
			}
		}
		temp.CPUMax = maxTemp

		// If no package temp, use max core temp as package
		if temp.CPUPackage == 0 {
			temp.CPUPackage = maxTemp
		}
	}

	// Validate we have at least some data
	if !temp.HasCPU && !temp.HasGPU && !temp.HasNVMe {
		return nil
	}

	log.Debug().
		Str("source", "host-agent").
		Float64("cpuPackage", temp.CPUPackage).
		Float64("cpuMax", temp.CPUMax).
		Int("coreCount", len(temp.Cores)).
		Int("nvmeCount", len(temp.NVMe)).
		Int("gpuCount", len(temp.GPU)).
		Msg("Converted host agent sensors to temperature data")

	return temp
}

// isHostAgentTemperatureRecent checks if the host agent temperature data is recent enough to use.
// We consider data stale if the host hasn't reported in more than 2 minutes.
func isHostAgentTemperatureRecent(lastSeen time.Time) bool {
	const staleDuration = 2 * time.Minute
	return time.Since(lastSeen) < staleDuration
}

// mergeTemperatureData merges host agent temperature with existing/proxy temperature data.
// Host agent data takes priority for CPU temperatures since it's more reliable (no SSH required).
// NVMe/SMART data is merged - host agent NVMe data supplements proxy SMART data.
func mergeTemperatureData(hostAgentTemp, proxyTemp *models.Temperature) *models.Temperature {
	if hostAgentTemp == nil {
		return proxyTemp
	}
	if proxyTemp == nil {
		return hostAgentTemp
	}

	// Start with host agent data as base since it's more reliable
	result := &models.Temperature{
		CPUPackage: hostAgentTemp.CPUPackage,
		CPUMax:     hostAgentTemp.CPUMax,
		CPUMin:     proxyTemp.CPUMin, // Preserve historical min
		CPUMaxRecord: math.Max(hostAgentTemp.CPUPackage, proxyTemp.CPUMaxRecord), // Update historical max
		MinRecorded: proxyTemp.MinRecorded,
		MaxRecorded: proxyTemp.MaxRecorded,
		Cores:       hostAgentTemp.Cores,
		GPU:         hostAgentTemp.GPU,
		NVMe:        hostAgentTemp.NVMe,
		Available:   true,
		HasCPU:      hostAgentTemp.HasCPU,
		HasGPU:      hostAgentTemp.HasGPU,
		HasNVMe:     hostAgentTemp.HasNVMe,
		HasSMART:    proxyTemp.HasSMART, // SMART data only comes from proxy
		LastUpdate:  hostAgentTemp.LastUpdate,
	}

	// Use host agent CPU data if available, fall back to proxy
	if !hostAgentTemp.HasCPU && proxyTemp.HasCPU {
		result.CPUPackage = proxyTemp.CPUPackage
		result.CPUMax = proxyTemp.CPUMax
		result.Cores = proxyTemp.Cores
		result.HasCPU = true
	}

	// Keep proxy SMART data (host agent doesn't have smartctl access currently)
	if proxyTemp.HasSMART {
		result.SMART = proxyTemp.SMART
	}

	// Merge GPU data - prefer host agent if available
	if !hostAgentTemp.HasGPU && proxyTemp.HasGPU {
		result.GPU = proxyTemp.GPU
		result.HasGPU = true
	}

	// Merge NVMe data - prefer host agent if available, fall back to proxy
	if !hostAgentTemp.HasNVMe && proxyTemp.HasNVMe {
		result.NVMe = proxyTemp.NVMe
		result.HasNVMe = true
	}

	// Update historical max if current is higher
	currentTemp := result.CPUPackage
	if currentTemp == 0 && result.CPUMax > 0 {
		currentTemp = result.CPUMax
	}
	if currentTemp > result.CPUMaxRecord {
		result.CPUMaxRecord = currentTemp
		result.MaxRecorded = time.Now()
	}

	return result
}
