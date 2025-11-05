package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/ssh/knownhosts"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/tempproxy"
	"github.com/rs/zerolog/log"
)

const (
	proxyFailureThreshold = 3
	proxyRetryInterval    = 5 * time.Minute
)

type temperatureProxy interface {
	IsAvailable() bool
	GetTemperature(nodeHost string) (string, error)
}

// TemperatureCollector handles SSH-based temperature collection from Proxmox nodes
type TemperatureCollector struct {
	sshUser            string           // SSH user (typically "root" or "pulse-monitor")
	sshKeyPath         string           // Path to SSH private key
	proxyClient        temperatureProxy // Optional: unix socket client for proxy
	useProxy           bool             // Whether to use proxy for temperature collection
	hostKeys           knownhosts.Manager
	proxyMu            sync.Mutex
	proxyFailures      int
	proxyCooldownUntil time.Time
	missingKeyWarned   atomic.Bool
	legacySSHDisabled  atomic.Bool
}

// NewTemperatureCollector creates a new temperature collector
func NewTemperatureCollector(sshUser, sshKeyPath string) *TemperatureCollector {
	tc := &TemperatureCollector{
		sshUser:    sshUser,
		sshKeyPath: sshKeyPath,
	}

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts_sensors")
	if manager, err := knownhosts.NewManager(knownHostsPath); err != nil {
		log.Warn().Err(err).Str("path", knownHostsPath).Msg("Failed to initialize temperature known_hosts manager")
	} else {
		tc.hostKeys = manager
	}

	// Check if proxy is available
	proxyClient := tempproxy.NewClient()
	if proxyClient.IsAvailable() {
		log.Info().Msg("Temperature proxy detected - using secure host-side bridge")
		tc.proxyClient = proxyClient
		tc.useProxy = true
	} else {
		log.Debug().Msg("Temperature proxy not available - using direct SSH")
		tc.useProxy = false
	}

	return tc
}

// CollectTemperature collects temperature data from a node via SSH
func (tc *TemperatureCollector) CollectTemperature(ctx context.Context, nodeHost, nodeName string) (*models.Temperature, error) {
	// Extract hostname/IP from the host URL (might be https://hostname:8006)
	host := extractHostname(nodeHost)

	var output string
	var err error

	// Use proxy if available, otherwise fall back to direct SSH
	if tc.isProxyEnabled() {
		output, err = tc.proxyClient.GetTemperature(host)
		if err != nil {
			tc.handleProxyFailure(err)
			log.Debug().
				Str("node", nodeName).
				Str("host", host).
				Err(err).
				Msg("Failed to collect temperature data via proxy")
			return &models.Temperature{Available: false}, nil
		}
		tc.handleProxySuccess()
	} else {
		// SECURITY: Block SSH fallback when running in containers (unless dev mode)
		// Container compromise = SSH key compromise = root access to infrastructure
		devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
		if system.InContainer() && !devModeAllowSSH {
			log.Error().
				Str("node", nodeName).
				Msg("SECURITY BLOCK: SSH temperature collection disabled in containers - deploy pulse-sensor-proxy")
			return &models.Temperature{Available: false}, nil
		}

		if tc.legacySSHDisabled.Load() {
			return &models.Temperature{Available: false}, nil
		}

		if strings.TrimSpace(tc.sshKeyPath) == "" {
			tc.logMissingSSHKey(nil)
			return &models.Temperature{Available: false}, nil
		}

		if _, keyErr := os.Stat(tc.sshKeyPath); keyErr != nil {
			tc.logMissingSSHKey(keyErr)
			return &models.Temperature{Available: false}, nil
		}

		// Direct SSH (legacy method for non-containerized deployments)
		// Try sensors first, fall back to Raspberry Pi method if that fails
		output, err = tc.runSSHCommand(ctx, host, "sensors -j 2>/dev/null")
		if err != nil || strings.TrimSpace(output) == "" {
			if tc.disableLegacySSHOnAuthFailure(err, nodeName, host) {
				return &models.Temperature{Available: false}, nil
			}

			// Try Raspberry Pi temperature method
			output, err = tc.runSSHCommand(ctx, host, "cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null")
			if err == nil && strings.TrimSpace(output) != "" {
				// Parse RPi temperature format
				temp, parseErr := tc.parseRPiTemperature(output)
				if parseErr == nil {
					return temp, nil
				}
			}

			if tc.disableLegacySSHOnAuthFailure(err, nodeName, host) {
				return &models.Temperature{Available: false}, nil
			}

			log.Debug().
				Str("node", nodeName).
				Str("host", host).
				Err(err).
				Msg("Failed to collect temperature data via SSH (tried both lm-sensors and RPi methods)")
			return &models.Temperature{Available: false}, nil
		}
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

	if !temp.Available {
		return temp, nil
	}

	temp.LastUpdate = time.Now()

	return temp, nil
}

// runSSHCommand executes a command on a remote node via SSH
func (tc *TemperatureCollector) runSSHCommand(ctx context.Context, host, command string) (string, error) {
	if strings.TrimSpace(tc.sshKeyPath) != "" {
		if _, err := os.Stat(tc.sshKeyPath); err != nil {
			return "", fmt.Errorf("temperature SSH key unavailable: %w", err)
		}
	}

	if err := tc.ensureHostKey(ctx, host); err != nil {
		return "", err
	}

	// Build SSH command with appropriate options
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=yes",
		"-o", "BatchMode=yes",
		"-o", "LogLevel=ERROR", // Suppress host key warnings that break JSON parsing
		"-o", "ConnectTimeout=5",
	}

	if tc.hostKeys != nil && tc.hostKeys.Path() != "" {
		sshArgs = append(sshArgs,
			"-o", fmt.Sprintf("UserKnownHostsFile=%s", tc.hostKeys.Path()),
			"-o", "GlobalKnownHostsFile=/dev/null",
		)
	}

	// Explicitly use SSH config file if it exists (for ProxyJump configuration)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	sshConfigPath := filepath.Join(homeDir, ".ssh/config")
	if _, err := os.Stat(sshConfigPath); err == nil {
		sshArgs = append(sshArgs, "-F", sshConfigPath)
	}

	// Add key if specified
	if tc.sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", tc.sshKeyPath)
	}

	// Add user@host and command
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", tc.sshUser, host), command)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	output, err := cmd.Output()
	if err != nil {
		// On error, try to get stderr for debugging
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ssh command failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("ssh command failed: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))

	// Strip any leading SSH noise (e.g., "Warning: Permanently added ...") so sensors JSON parses cleanly.
	if idx := strings.Index(outputStr, "{"); idx > 0 {
		outputStr = outputStr[idx:]
	}
	if idx := strings.LastIndex(outputStr, "}"); idx != -1 && idx < len(outputStr)-1 {
		outputStr = outputStr[:idx+1]
	}

	return outputStr, nil
}

func (tc *TemperatureCollector) logMissingSSHKey(cause error) {
	if tc.missingKeyWarned.Load() {
		return
	}
	if tc.missingKeyWarned.CompareAndSwap(false, true) {
		event := log.Debug().
			Str("sshKeyPath", tc.sshKeyPath)
		if cause != nil && !errors.Is(cause, os.ErrNotExist) {
			event = event.Err(cause)
		}
		event.Msg("Temperature SSH key not available; skipping legacy SSH collection")
	}
}

func (tc *TemperatureCollector) disableLegacySSHOnAuthFailure(err error, nodeName, host string) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	authFailure := strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "publickey")

	if !authFailure {
		return false
	}

	if tc.legacySSHDisabled.CompareAndSwap(false, true) {
		log.Warn().
			Str("node", nodeName).
			Str("host", host).
			Err(err).
			Msg("Disabling legacy SSH temperature collection after authentication failure; configure pulse-sensor-proxy or adjust SSH access.")
	}

	return true
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

	foundCPUChip := false

	// Parse each sensor chip
	for chipName, chipData := range sensorsData {
		chipMap, ok := chipData.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle CPU temperature sensors
		chipLower := strings.ToLower(chipName)
		if strings.Contains(chipLower, "coretemp") ||
			strings.Contains(chipLower, "k10temp") ||
			strings.Contains(chipLower, "zenpower") ||
			strings.Contains(chipLower, "k8temp") ||
			strings.Contains(chipLower, "acpitz") ||
			strings.Contains(chipLower, "it87") ||
			strings.Contains(chipLower, "nct6687") || // Nuvoton NCT6687 SuperIO
			strings.Contains(chipLower, "nct6775") || // Nuvoton NCT6775 SuperIO
			strings.Contains(chipLower, "nct6776") || // Nuvoton NCT6776 SuperIO
			strings.Contains(chipLower, "nct6779") || // Nuvoton NCT6779 SuperIO
			strings.Contains(chipLower, "nct6791") || // Nuvoton NCT6791 SuperIO
			strings.Contains(chipLower, "nct6792") || // Nuvoton NCT6792 SuperIO
			strings.Contains(chipLower, "nct6793") || // Nuvoton NCT6793 SuperIO
			strings.Contains(chipLower, "nct6795") || // Nuvoton NCT6795 SuperIO
			strings.Contains(chipLower, "nct6796") || // Nuvoton NCT6796 SuperIO
			strings.Contains(chipLower, "nct6797") || // Nuvoton NCT6797 SuperIO
			strings.Contains(chipLower, "nct6798") || // Nuvoton NCT6798 SuperIO
			strings.Contains(chipLower, "w83627") ||  // Winbond W83627 SuperIO series
			strings.Contains(chipLower, "f71882") ||  // Fintek F71882 SuperIO
			strings.Contains(chipLower, "cpu_thermal") || // Raspberry Pi CPU temperature
			strings.Contains(chipLower, "rpitemp") {
			foundCPUChip = true
			log.Debug().
				Str("chip", chipName).
				Msg("Detected CPU temperature chip")
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

	// Set individual sensor type flags based on chip presence, not value thresholds
	// This prevents false negatives when sensors report 0°C during resets or temporarily
	temp.HasCPU = foundCPUChip
	temp.HasNVMe = len(temp.NVMe) > 0

	// Available means any temperature data exists (backward compatibility)
	temp.Available = temp.HasCPU || temp.HasNVMe

	// Log summary of what was detected
	if !foundCPUChip {
		// List all chip names found for debugging
		chipNames := make([]string, 0, len(sensorsData))
		for chipName := range sensorsData {
			chipNames = append(chipNames, chipName)
		}
		log.Debug().
			Strs("chips", chipNames).
			Msg("No recognized CPU temperature chip found in sensors output")
	} else {
		log.Debug().
			Bool("hasCPU", temp.HasCPU).
			Bool("hasNVMe", temp.HasNVMe).
			Float64("cpuPackage", temp.CPUPackage).
			Float64("cpuMax", temp.CPUMax).
			Int("coreCount", len(temp.Cores)).
			Int("nvmeCount", len(temp.NVMe)).
			Msg("Temperature data parsed successfully")
	}

	return temp, nil
}

// parseCPUTemps extracts CPU temperature data from a sensor chip
func (tc *TemperatureCollector) parseCPUTemps(chipMap map[string]interface{}, temp *models.Temperature) {
	foundPackageTemp := false
	var chipletTemps []float64 // Store AMD Tccd chiplet temps for fallback

	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		sensorNameLower := strings.ToLower(sensorName)

		// Look for Package id (Intel) or Tdie/Tctl (AMD control loop temperature)
		if strings.Contains(sensorName, "Package id") ||
			strings.Contains(sensorName, "Tdie") ||
			strings.Contains(sensorNameLower, "tctl") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				temp.CPUPackage = tempVal
				foundPackageTemp = true
				if tempVal > temp.CPUMax {
					temp.CPUMax = tempVal
				}
				log.Debug().
					Str("sensor", sensorName).
					Float64("temp", tempVal).
					Msg("Found CPU package temperature")
			}
		}

		// Look for AMD chiplet temperatures (Tccd1, Tccd2, etc.) as fallback
		if strings.HasPrefix(sensorName, "Tccd") {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
				chipletTemps = append(chipletTemps, tempVal)
				if tempVal > temp.CPUMax {
					temp.CPUMax = tempVal
				}
				log.Debug().
					Str("sensor", sensorName).
					Float64("temp", tempVal).
					Msg("Found AMD chiplet temperature")
			}
		}

		// Look for SuperIO chip CPU temperature fields (CPUTIN, CPU Temperature, etc.)
		if strings.Contains(sensorNameLower, "cputin") ||
			strings.Contains(sensorNameLower, "cpu temperature") ||
			(strings.Contains(sensorNameLower, "temp") && strings.Contains(sensorNameLower, "cpu")) {
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
				if !foundPackageTemp {
					temp.CPUPackage = tempVal
					foundPackageTemp = true
				}
				if tempVal > temp.CPUMax {
					temp.CPUMax = tempVal
				}
				log.Debug().
					Str("sensor", sensorName).
					Float64("temp", tempVal).
					Msg("Found SuperIO CPU temperature")
			}
		}

		// Look for individual cores
		if strings.HasPrefix(sensorName, "Core ") {
			coreNum := extractCoreNumber(sensorName)
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) {
				temp.Cores = append(temp.Cores, models.CoreTemp{
					Core: coreNum,
					Temp: tempVal,
				})
				if tempVal > temp.CPUMax {
					temp.CPUMax = tempVal
				}
				log.Debug().
					Str("sensor", sensorName).
					Int("core", coreNum).
					Float64("temp", tempVal).
					Msg("Found core temperature")
			}
		}
	}

	// If no package temperature found, use highest chiplet temp (AMD Ryzen)
	if !foundPackageTemp && len(chipletTemps) > 0 {
		for _, chipletTemp := range chipletTemps {
			if chipletTemp > temp.CPUPackage {
				temp.CPUPackage = chipletTemp
			}
		}
		foundPackageTemp = true
		log.Debug().
			Float64("temp", temp.CPUPackage).
			Msg("Using highest chiplet temperature as CPU package temperature")
	}

	// If no package temperature was found (e.g., Raspberry Pi), look for generic temp sensors
	if !foundPackageTemp {
		for sensorName, sensorData := range chipMap {
			sensorMap, ok := sensorData.(map[string]interface{})
			if !ok {
				continue
			}

			// Look for generic temperature sensors (e.g., "temp1" on Raspberry Pi)
			if strings.HasPrefix(sensorName, "temp") || strings.HasPrefix(sensorName, "Temp") {
				if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
					if temp.CPUPackage <= 0 {
						temp.CPUPackage = tempVal
					}
					if tempVal > temp.CPUMax {
						temp.CPUMax = tempVal
					}
					break // Use the first valid generic temp sensor
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
			if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
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
	return math.NaN()
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

// parseRPiTemperature parses Raspberry Pi temperature from /sys/class/thermal/thermal_zone0/temp
// Format: integer representing millidegrees Celsius (e.g., "45678" = 45.678°C)
func (tc *TemperatureCollector) parseRPiTemperature(output string) (*models.Temperature, error) {
	millidegrees := strings.TrimSpace(output)
	if millidegrees == "" {
		return nil, fmt.Errorf("empty RPi temperature output")
	}

	tempMilliC, err := strconv.ParseFloat(millidegrees, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RPi temperature: %w", err)
	}

	// Convert millidegrees to degrees Celsius
	tempC := tempMilliC / 1000.0

	temp := &models.Temperature{
		Available:  true,
		HasCPU:     true,
		CPUPackage: tempC,
		CPUMax:     tempC,
		Cores:      []models.CoreTemp{},
		NVMe:       []models.NVMeTemp{},
		LastUpdate: time.Now(),
	}

	return temp, nil
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

func (tc *TemperatureCollector) ensureHostKey(ctx context.Context, host string) error {
	if tc.hostKeys == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return tc.hostKeys.Ensure(ctx, host)
}

func (tc *TemperatureCollector) isProxyEnabled() bool {
	if tc.proxyClient == nil {
		return false
	}

	tc.proxyMu.Lock()
	restored := false
	if !tc.useProxy {
		now := time.Now()
		if now.After(tc.proxyCooldownUntil) {
			if tc.proxyClient.IsAvailable() {
				tc.useProxy = true
				tc.proxyFailures = 0
				tc.proxyCooldownUntil = time.Time{}
				restored = true
			} else {
				tc.proxyCooldownUntil = now.Add(proxyRetryInterval)
			}
		}
	}
	useProxy := tc.useProxy
	tc.proxyMu.Unlock()

	if restored {
		log.Info().Msg("Temperature proxy connection restored; resuming proxy collection")
	}

	return useProxy
}

func (tc *TemperatureCollector) handleProxySuccess() {
	if tc.proxyClient == nil {
		return
	}
	tc.proxyMu.Lock()
	tc.proxyFailures = 0
	tc.proxyMu.Unlock()
}

func (tc *TemperatureCollector) handleProxyFailure(err error) {
	if tc.proxyClient == nil || !tc.shouldDisableProxy(err) {
		return
	}

	tc.proxyMu.Lock()
	tc.proxyFailures++
	disable := tc.proxyFailures >= proxyFailureThreshold && tc.useProxy
	if disable {
		tc.useProxy = false
		tc.proxyCooldownUntil = time.Now().Add(proxyRetryInterval)
		tc.proxyFailures = 0
	}
	tc.proxyMu.Unlock()

	if disable {
		log.Warn().
			Err(err).
			Dur("cooldown", proxyRetryInterval).
			Msg("Temperature proxy disabled after repeated failures; will retry later")
	}
}

func (tc *TemperatureCollector) shouldDisableProxy(err error) bool {
	var proxyErr *tempproxy.ProxyError
	if errors.As(err, &proxyErr) {
		switch proxyErr.Type {
		case tempproxy.ErrorTypeTransport, tempproxy.ErrorTypeTimeout:
			return true
		default:
			return false
		}
	}
	return true
}
