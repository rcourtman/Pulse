package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// TemperatureCollector handles SSH-based temperature collection from Proxmox nodes
type TemperatureCollector struct {
	sshUser    string // SSH user (typically "root" or "pulse-monitor")
	sshKeyPath string // Path to SSH private key
}

// NewTemperatureCollector creates a new temperature collector
func NewTemperatureCollector(sshUser, sshKeyPath string) *TemperatureCollector {
	return &TemperatureCollector{
		sshUser:    sshUser,
		sshKeyPath: sshKeyPath,
	}
}

// CollectTemperature collects temperature data from a node via SSH
func (tc *TemperatureCollector) CollectTemperature(ctx context.Context, nodeHost, nodeName string) (*models.Temperature, error) {
	// Extract hostname/IP from the host URL (might be https://hostname:8006)
	host := extractHostname(nodeHost)

	// Try to get sensors JSON output
	output, err := tc.runSSHCommand(ctx, host, "sensors -j 2>/dev/null")
	if err != nil {
		log.Debug().
			Str("node", nodeName).
			Str("host", host).
			Err(err).
			Msg("Failed to collect temperature data via SSH")
		return &models.Temperature{Available: false}, nil
	}

	// Parse sensors JSON output
	temp, err := tc.parseSensorsJSON(output)
	if err != nil {
		log.Debug().
			Str("node", nodeName).
			Err(err).
			Msg("Failed to parse sensors output")
		return &models.Temperature{Available: false}, nil
	}

	temp.Available = true
	temp.LastUpdate = time.Now()

	return temp, nil
}

// runSSHCommand executes a command on a remote node via SSH
func (tc *TemperatureCollector) runSSHCommand(ctx context.Context, host, command string) (string, error) {
	// Build SSH command with appropriate options
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes", // No password prompts
	}

	// Add key if specified
	if tc.sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", tc.sshKeyPath)
	}

	// Add user@host and command
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", tc.sshUser, host), command)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh command failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// parseSensorsJSON parses the JSON output from `sensors -j`
func (tc *TemperatureCollector) parseSensorsJSON(jsonStr string) (*models.Temperature, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, fmt.Errorf("empty sensors output")
	}

	// sensors -j output structure:
	// {
	//   "coretemp-isa-0000": {
	//     "Package id 0": {"temp1_input": 45.0},
	//     "Core 0": {"temp2_input": 43.0},
	//     ...
	//   },
	//   "nvme-pci-0400": {
	//     "Composite": {"temp1_input": 38.9}
	//   }
	// }

	var sensorsData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &sensorsData); err != nil {
		return nil, fmt.Errorf("failed to parse sensors JSON: %w", err)
	}

	temp := &models.Temperature{
		Cores: []models.CoreTemp{},
		NVMe:  []models.NVMeTemp{},
	}

	// Parse each sensor chip
	for chipName, chipData := range sensorsData {
		chipMap, ok := chipData.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle CPU temperature sensors (coretemp, k10temp, etc.)
		if strings.Contains(chipName, "coretemp") || strings.Contains(chipName, "k10temp") {
			tc.parseCPUTemps(chipMap, temp)
		}

		// Handle NVMe temperature sensors
		if strings.Contains(chipName, "nvme") {
			tc.parseNVMeTemps(chipName, chipMap, temp)
		}
	}

	// If we got CPU temps, calculate max from cores if package not available
	if temp.CPUPackage == 0 && len(temp.Cores) > 0 {
		for _, core := range temp.Cores {
			if core.Temp > temp.CPUMax {
				temp.CPUMax = core.Temp
			}
		}
	}

	return temp, nil
}

// parseCPUTemps extracts CPU temperature data from a sensor chip
func (tc *TemperatureCollector) parseCPUTemps(chipMap map[string]interface{}, temp *models.Temperature) {
	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		// Look for Package id (Intel) or Tdie (AMD)
		if strings.Contains(sensorName, "Package id") || strings.Contains(sensorName, "Tdie") {
			if tempVal := extractTempInput(sensorMap); tempVal > 0 {
				temp.CPUPackage = tempVal
			}
		}

		// Look for individual cores
		if strings.HasPrefix(sensorName, "Core ") {
			coreNum := extractCoreNumber(sensorName)
			if tempVal := extractTempInput(sensorMap); tempVal > 0 {
				temp.Cores = append(temp.Cores, models.CoreTemp{
					Core: coreNum,
					Temp: tempVal,
				})
				if tempVal > temp.CPUMax {
					temp.CPUMax = tempVal
				}
			}
		}
	}
}

// parseNVMeTemps extracts NVMe temperature data from a sensor chip
func (tc *TemperatureCollector) parseNVMeTemps(chipName string, chipMap map[string]interface{}, temp *models.Temperature) {
	// Extract device name from chip name (e.g., "nvme-pci-0400" -> "nvme0")
	device := "nvme" + strings.TrimPrefix(chipName, "nvme-pci-")

	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		// Look for Composite temperature (main NVMe temp)
		if strings.Contains(sensorName, "Composite") || strings.Contains(sensorName, "Sensor 1") {
			if tempVal := extractTempInput(sensorMap); tempVal > 0 {
				temp.NVMe = append(temp.NVMe, models.NVMeTemp{
					Device: device,
					Temp:   tempVal,
				})
				break // Only one temp per NVMe device
			}
		}
	}
}

// extractTempInput extracts temperature value from sensor data
func extractTempInput(sensorMap map[string]interface{}) float64 {
	// Look for temp*_input fields
	for key, val := range sensorMap {
		if strings.HasSuffix(key, "_input") {
			switch v := val.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

// extractCoreNumber extracts the core number from a sensor name like "Core 0"
func extractCoreNumber(name string) int {
	parts := strings.Fields(name)
	if len(parts) >= 2 {
		if num, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return num
		}
	}
	return 0
}

// extractHostname extracts hostname/IP from a Proxmox host URL
func extractHostname(hostURL string) string {
	// Remove protocol
	host := strings.TrimPrefix(hostURL, "https://")
	host = strings.TrimPrefix(host, "http://")

	// Remove port
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Remove path
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}
