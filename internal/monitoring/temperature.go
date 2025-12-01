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
	sshPort            int              // SSH port (default 22)
	proxyClient        temperatureProxy // Optional: unix socket client for proxy
	useProxy           bool             // Whether to use proxy for temperature collection
	hostKeys           knownhosts.Manager
	proxyMu            sync.Mutex
	proxyFailures      int
	proxyCooldownUntil time.Time
	proxyHostStates    map[string]*proxyHostState
	missingKeyWarned   atomic.Bool
}

type proxyHostState struct {
	failures      int
	cooldownUntil time.Time
	lastError     string
}

// ProxyHostDiagnostics describes the proxy transport state for a host.
type ProxyHostDiagnostics struct {
	Host          string
	Failures      int
	CooldownUntil time.Time
	LastError     string
}

// NewTemperatureCollectorWithPort creates a new temperature collector with custom SSH port
func NewTemperatureCollectorWithPort(sshUser, sshKeyPath string, sshPort int) *TemperatureCollector {
	if sshPort <= 0 {
		sshPort = 22 // Default to standard SSH port
	}

	tc := &TemperatureCollector{
		sshUser:         sshUser,
		sshKeyPath:      sshKeyPath,
		sshPort:         sshPort,
		proxyHostStates: make(map[string]*proxyHostState),
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

	// Always keep a proxy client so we can detect the socket later even if it
	// isn't present during startup. Without this, containerized deployments that
	// mount the socket after Pulse starts never re-enable the hardened proxy.
	proxyClient := tempproxy.NewClient()
	tc.proxyClient = proxyClient
	if proxyClient.IsAvailable() {
		log.Info().Msg("Temperature proxy detected - using secure host-side bridge")
		tc.useProxy = true
	} else {
		log.Debug().Msg("Temperature proxy not available yet - falling back to SSH until socket appears")
		tc.useProxy = false
	}

	return tc
}

// CollectTemperature collects temperature data from a node via SSH
func (tc *TemperatureCollector) CollectTemperature(ctx context.Context, nodeHost, nodeName string) (*models.Temperature, error) {
	return tc.CollectTemperatureWithProxy(ctx, nodeHost, nodeName, "", "")
}

// CollectTemperatureWithProxy collects temperature data with optional HTTP proxy configuration
func (tc *TemperatureCollector) CollectTemperatureWithProxy(ctx context.Context, nodeHost, nodeName, proxyURL, proxyToken string) (*models.Temperature, error) {
	// Extract hostname/IP from the host URL (might be https://hostname:8006)
	host := extractHostname(nodeHost)

	var output string
	var err error

	// Try HTTP proxy first if configured for this instance
	if proxyURL != "" && proxyToken != "" {
		httpClient := tempproxy.NewHTTPClient(proxyURL, proxyToken)
		if httpClient.IsAvailable() {
			// Use nodeName for HTTP proxy (sensor-proxy needs short hostname, not IP)
			output, err = httpClient.GetTemperature(nodeName)
			if err != nil {
				log.Debug().
					Str("node", nodeName).
					Str("host", host).
					Str("proxy_url", proxyURL).
					Err(err).
					Msg("Failed to collect temperature data via HTTP proxy")
				// Don't fall back to socket/SSH for HTTP proxy failures
				// If HTTP proxy is configured, it's the intended method
				return &models.Temperature{Available: false}, nil
			}
			// HTTP proxy succeeded
			goto parseOutput
		}
	}

	// Use Unix socket proxy if available (local deployment)
	if tc.isProxyEnabled() {
		if tc.shouldSkipProxyHost(host) {
			log.Debug().
				Str("node", nodeName).
				Str("host", host).
				Msg("Skipping temperature proxy request while host is in cooldown")
			return &models.Temperature{Available: false}, nil
		}

		output, err = tc.proxyClient.GetTemperature(host)
		if err != nil {
			tc.handleProxyFailure(host, err)
			log.Debug().
				Str("node", nodeName).
				Str("host", host).
				Err(err).
				Msg("Failed to collect temperature data via proxy")
			return &models.Temperature{Available: false}, nil
		}
		tc.handleProxySuccess()
		tc.handleProxyHostSuccess(host)
	} else {
		// SECURITY: Block SSH fallback when running in containers (unless dev mode)
		// Container compromise = SSH key compromise = root access to infrastructure
		devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
		isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()

		if isContainer && devModeAllowSSH {
			// Log when dev override is active so operators understand the security posture
			log.Info().
				Str("node", nodeName).
				Msg("Temperature collection using direct SSH (dev mode override active - not for production)")
		}

		if isContainer && !devModeAllowSSH {
			// Warn but allow if key is present (legacy behavior restoration)
			// We don't return here, allowing the code to fall through to the SSH key check
			log.Warn().
				Str("node", nodeName).
				Msg("Temperature collection using direct SSH from container. This is insecure. Recommend using pulse-sensor-proxy.")
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
		// sensors exits non-zero when optional subfeatures fail; "|| true" keeps the JSON for parsing (#600)
		output, err = tc.runSSHCommand(ctx, host, "sensors -j 2>/dev/null || true")
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

parseOutput:
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
		"-p", fmt.Sprintf("%d", tc.sshPort), // Use configured SSH port
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

	// Do not disable globally on single node failure
	log.Warn().
		Str("node", nodeName).
		Str("host", host).
		Err(err).
		Msg("SSH temperature collection failed due to authentication error; check SSH keys")

	return true
}

type smartEntryRaw struct {
	Device         string `json:"device"`
	Serial         string `json:"serial,omitempty"`
	WWN            string `json:"wwn,omitempty"`
	Model          string `json:"model,omitempty"`
	Type           string `json:"type,omitempty"`
	Temperature    *int   `json:"temperature"`
	LastUpdated    string `json:"lastUpdated,omitempty"`
	StandbySkipped bool   `json:"standbySkipped,omitempty"`
}

func normalizeSMARTEntries(raw []smartEntryRaw) []models.DiskTemp {
	if len(raw) == 0 {
		return nil
	}

	normalized := make([]models.DiskTemp, 0, len(raw))
	for _, entry := range raw {
		dev := strings.TrimSpace(entry.Device)
		if dev == "" {
			continue
		}

		var lastUpdated time.Time
		if entry.LastUpdated != "" {
			if parsed, err := time.Parse(time.RFC3339, entry.LastUpdated); err == nil {
				lastUpdated = parsed
			}
		}

		tempVal := 0
		if entry.Temperature != nil {
			tempVal = *entry.Temperature
		}

		normalized = append(normalized, models.DiskTemp{
			Device:         dev,
			Serial:         strings.TrimSpace(entry.Serial),
			WWN:            strings.TrimSpace(entry.WWN),
			Model:          strings.TrimSpace(entry.Model),
			Type:           strings.TrimSpace(entry.Type),
			Temperature:    tempVal,
			LastUpdated:    lastUpdated,
			StandbySkipped: entry.StandbySkipped,
		})
	}

	return normalized
}

// parseSensorsJSON parses the JSON output from the sensor wrapper
func (tc *TemperatureCollector) parseSensorsJSON(jsonStr string) (*models.Temperature, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return nil, fmt.Errorf("empty sensors output")
	}

	// Try to parse as wrapper format first: {sensors: {...}, smart: [...]}
	// Fall back to legacy format for backward compatibility
	var wrapperData struct {
		Sensors map[string]interface{} `json:"sensors"`
		SMART   []smartEntryRaw        `json:"smart"`
	}

	var sensorsData map[string]interface{}
	var smartRaw []smartEntryRaw
	var parsedWrapper bool

	if err := json.Unmarshal([]byte(jsonStr), &wrapperData); err == nil && wrapperData.Sensors != nil {
		// New wrapper format
		sensorsData = wrapperData.Sensors
		smartRaw = wrapperData.SMART
		parsedWrapper = true
	} else {
		// Legacy format: direct sensors -j output
		if err := json.Unmarshal([]byte(jsonStr), &sensorsData); err != nil {
			return nil, fmt.Errorf("failed to parse sensors JSON: %w", err)
		}
		log.Debug().Msg("Parsed legacy sensors format (no SMART data)")
	}

	smartData := normalizeSMARTEntries(smartRaw)
	if parsedWrapper {
		log.Debug().
			Int("smartDisks", len(smartData)).
			Msg("Parsed new wrapper format with SMART data")
	}

	temp := &models.Temperature{
		Cores: []models.CoreTemp{},
		NVMe:  []models.NVMeTemp{},
		SMART: smartData,
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
			strings.Contains(chipLower, "w83627") || // Winbond W83627 SuperIO series
			strings.Contains(chipLower, "f71882") || // Fintek F71882 SuperIO
			strings.Contains(chipLower, "cpu_thermal") || // Raspberry Pi CPU temperature
			strings.Contains(chipLower, "rp1_adc") || // Raspberry Pi RP1 ADC
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

		// Handle GPU temperature sensors
		if strings.Contains(chipLower, "amdgpu") {
			log.Debug().
				Str("chip", chipName).
				Msg("Detected AMD GPU temperature chip")
			tc.parseGPUTemps(chipName, chipMap, temp)
		}

		// Handle NVIDIA GPU temperature sensors (nouveau driver)
		if strings.Contains(chipLower, "nouveau") {
			log.Debug().
				Str("chip", chipName).
				Msg("Detected NVIDIA GPU temperature chip (nouveau)")
			tc.parseNouveauGPUTemps(chipName, chipMap, temp)
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
	temp.HasGPU = len(temp.GPU) > 0
	temp.HasSMART = len(temp.SMART) > 0

	// Available means any temperature data exists (backward compatibility)
	temp.Available = temp.HasCPU || temp.HasNVMe || temp.HasGPU || temp.HasSMART

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
			Bool("hasGPU", temp.HasGPU).
			Bool("hasSMART", temp.HasSMART).
			Float64("cpuPackage", temp.CPUPackage).
			Float64("cpuMax", temp.CPUMax).
			Int("coreCount", len(temp.Cores)).
			Int("nvmeCount", len(temp.NVMe)).
			Int("gpuCount", len(temp.GPU)).
			Int("smartCount", len(temp.SMART)).
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

	// Try "Composite" first (preferred sensor name for NVMe temps)
	for sensorName, sensorData := range chipMap {
		if !strings.Contains(sensorName, "Composite") {
			continue
		}
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}
		if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
			temp.NVMe = append(temp.NVMe, models.NVMeTemp{
				Device: device,
				Temp:   tempVal,
			})
			return
		}
	}

	// Fall back to "Sensor 1" if no valid Composite found
	for sensorName, sensorData := range chipMap {
		if !strings.Contains(sensorName, "Sensor 1") {
			continue
		}
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}
		if tempVal := extractTempInput(sensorMap); !math.IsNaN(tempVal) && tempVal > 0 {
			temp.NVMe = append(temp.NVMe, models.NVMeTemp{
				Device: device,
				Temp:   tempVal,
			})
			return
		}
	}
}

// parseGPUTemps extracts GPU temperature data from a sensor chip
func (tc *TemperatureCollector) parseGPUTemps(chipName string, chipMap map[string]interface{}, temp *models.Temperature) {
	gpuTemp := models.GPUTemp{
		Device: chipName,
	}

	// AMD GPU sensors typically have: edge, junction (hotspot), mem
	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		sensorLower := strings.ToLower(sensorName)
		tempVal := extractTempInput(sensorMap)

		if math.IsNaN(tempVal) || tempVal <= 0 {
			continue
		}

		// Map sensor names to struct fields
		if strings.Contains(sensorLower, "edge") {
			gpuTemp.Edge = tempVal
		} else if strings.Contains(sensorLower, "junction") || strings.Contains(sensorLower, "hotspot") {
			gpuTemp.Junction = tempVal
		} else if strings.Contains(sensorLower, "mem") {
			gpuTemp.Mem = tempVal
		}
	}

	// Only add GPU entry if we got at least one valid temperature
	if gpuTemp.Edge > 0 || gpuTemp.Junction > 0 || gpuTemp.Mem > 0 {
		temp.GPU = append(temp.GPU, gpuTemp)
		log.Debug().
			Str("device", chipName).
			Float64("edge", gpuTemp.Edge).
			Float64("junction", gpuTemp.Junction).
			Float64("mem", gpuTemp.Mem).
			Msg("Parsed GPU temperatures")
	}
}

// parseNouveauGPUTemps extracts NVIDIA GPU temperature data from nouveau driver sensors
func (tc *TemperatureCollector) parseNouveauGPUTemps(chipName string, chipMap map[string]interface{}, temp *models.Temperature) {
	gpuTemp := models.GPUTemp{
		Device: chipName,
	}

	// Nouveau driver typically exposes "GPU core" sensor
	for sensorName, sensorData := range chipMap {
		sensorMap, ok := sensorData.(map[string]interface{})
		if !ok {
			continue
		}

		sensorLower := strings.ToLower(sensorName)
		tempVal := extractTempInput(sensorMap)

		if math.IsNaN(tempVal) || tempVal <= 0 {
			continue
		}

		// Nouveau typically has "GPU core" sensor - map to edge temperature
		if strings.Contains(sensorLower, "gpu") || strings.Contains(sensorLower, "core") {
			gpuTemp.Edge = tempVal
		}
	}

	// Only add GPU entry if we got a valid temperature
	if gpuTemp.Edge > 0 {
		temp.GPU = append(temp.GPU, gpuTemp)
		log.Debug().
			Str("device", chipName).
			Float64("edge", gpuTemp.Edge).
			Msg("Parsed NVIDIA GPU (nouveau) temperature")
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
	return tc.hostKeys.EnsureWithPort(ctx, host, tc.sshPort)
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

func (tc *TemperatureCollector) shouldSkipProxyHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}

	tc.proxyMu.Lock()
	defer tc.proxyMu.Unlock()
	state, ok := tc.proxyHostStates[host]
	if !ok || state == nil {
		return false
	}

	now := time.Now()
	if state.cooldownUntil.IsZero() || now.After(state.cooldownUntil) {
		// Cooldown expired; reset state so we can retry this host.
		state.cooldownUntil = time.Time{}
		state.failures = 0
		if state.cooldownUntil.IsZero() && state.failures == 0 {
			delete(tc.proxyHostStates, host)
		}
		return false
	}

	return true
}

// SocketProxyAvailable reports whether the unix socket proxy can currently be used.
func (tc *TemperatureCollector) SocketProxyAvailable() bool {
	return tc != nil && tc.isProxyEnabled()
}

// SocketProxyDetected reports whether the proxy socket exists (regardless of cooldown status).
func (tc *TemperatureCollector) SocketProxyDetected() bool {
	if tc == nil || tc.proxyClient == nil {
		return false
	}
	return tc.proxyClient.IsAvailable()
}

func (tc *TemperatureCollector) handleProxySuccess() {
	if tc.proxyClient == nil {
		return
	}
	tc.proxyMu.Lock()
	tc.proxyFailures = 0
	tc.proxyMu.Unlock()
}

func (tc *TemperatureCollector) handleProxyHostSuccess(host string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}
	tc.proxyMu.Lock()
	delete(tc.proxyHostStates, host)
	tc.proxyMu.Unlock()
}

func (tc *TemperatureCollector) handleProxyFailure(host string, err error) {
	if tc.proxyClient == nil {
		return
	}

	if tc.shouldDisableProxy(err) {
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
		return
	}

	tc.handleProxyHostFailure(host, err)
}

func (tc *TemperatureCollector) handleProxyHostFailure(host string, err error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return
	}

	tc.proxyMu.Lock()
	state, ok := tc.proxyHostStates[host]
	if !ok || state == nil {
		state = &proxyHostState{}
		tc.proxyHostStates[host] = state
	}
	state.failures++
	state.lastError = strings.TrimSpace(err.Error())
	trip := state.failures >= proxyFailureThreshold
	if trip {
		state.failures = 0
		state.cooldownUntil = time.Now().Add(proxyRetryInterval)
	}
	tc.proxyMu.Unlock()

	if trip {
		log.Warn().
			Err(err).
			Str("host", host).
			Dur("cooldown", proxyRetryInterval).
			Msg("Temperature proxy host in cooldown after repeated failures")
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

// ProxyHostDiagnostics returns a snapshot of per-host proxy error state.
func (tc *TemperatureCollector) ProxyHostDiagnostics() []ProxyHostDiagnostics {
	if tc == nil {
		return nil
	}

	tc.proxyMu.Lock()
	defer tc.proxyMu.Unlock()

	if len(tc.proxyHostStates) == 0 {
		return nil
	}

	result := make([]ProxyHostDiagnostics, 0, len(tc.proxyHostStates))
	for host, state := range tc.proxyHostStates {
		if state == nil {
			continue
		}
		result = append(result, ProxyHostDiagnostics{
			Host:          host,
			Failures:      state.failures,
			CooldownUntil: state.cooldownUntil,
			LastError:     state.lastError,
		})
	}
	return result
}
