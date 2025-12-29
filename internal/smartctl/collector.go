// Package smartctl provides S.M.A.R.T. data collection from local disks.
package smartctl

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	execLookPath     = exec.LookPath
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}
	timeNow = time.Now
)

// DiskSMART represents S.M.A.R.T. data for a single disk.
type DiskSMART struct {
	Device      string    `json:"device"`            // Device path (e.g., /dev/sda)
	Model       string    `json:"model,omitempty"`   // Disk model
	Serial      string    `json:"serial,omitempty"`  // Serial number
	WWN         string    `json:"wwn,omitempty"`     // World Wide Name
	Type        string    `json:"type,omitempty"`    // Transport type: sata, sas, nvme
	Temperature int       `json:"temperature"`       // Temperature in Celsius
	Health      string    `json:"health,omitempty"`  // PASSED, FAILED, UNKNOWN
	Standby     bool      `json:"standby,omitempty"` // True if disk was in standby
	LastUpdated time.Time `json:"lastUpdated"`       // When this reading was taken
}

// smartctlJSON represents the JSON output from smartctl --json.
type smartctlJSON struct {
	Device struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelFamily  string `json:"model_family"`
	ModelName    string `json:"model_name"`
	SerialNumber string `json:"serial_number"`
	WWN          struct {
		NAA uint64 `json:"naa"`
		OUI uint64 `json:"oui"`
		ID  uint64 `json:"id"`
	} `json:"wwn"`
	SmartStatus struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	// NVMe-specific temperature
	NVMeSmartHealthInformationLog struct {
		Temperature int `json:"temperature"`
	} `json:"nvme_smart_health_information_log"`
	PowerMode string `json:"power_mode"`
}

// CollectLocal collects S.M.A.R.T. data from all local block devices.
func CollectLocal(ctx context.Context) ([]DiskSMART, error) {
	// List block devices
	devices, err := listBlockDevices(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to list block devices for SMART collection")
		return nil, err
	}

	if len(devices) == 0 {
		return nil, nil
	}

	var results []DiskSMART
	for _, dev := range devices {
		smart, err := collectDeviceSMART(ctx, dev)
		if err != nil {
			log.Debug().Err(err).Str("device", dev).Msg("Failed to collect SMART data for device")
			continue
		}
		if smart != nil {
			results = append(results, *smart)
		}
	}

	return results, nil
}

// listBlockDevices returns a list of block devices suitable for SMART queries.
func listBlockDevices(ctx context.Context) ([]string, error) {
	// Use lsblk to find disks (not partitions)
	output, err := runCommandOutput(ctx, "lsblk", "-d", "-n", "-o", "NAME,TYPE")
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name, devType := fields[0], fields[1]
		// Only include disk types (not loop, rom, partition)
		if devType == "disk" {
			devices = append(devices, "/dev/"+name)
		}
	}

	return devices, nil
}

// collectDeviceSMART runs smartctl on a single device and parses the result.
func collectDeviceSMART(ctx context.Context, device string) (*DiskSMART, error) {
	// Use timeout to avoid hanging on slow/unresponsive disks
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if smartctl is available
	smartctlPath, err := execLookPath("smartctl")
	if err != nil {
		return nil, err
	}

	// Run smartctl with standby check to avoid waking sleeping drives
	// -n standby: don't check if drive is in standby (return exit code 2)
	// -i: device info
	// -A: attributes (for temperature)
	// --json=o: output original smartctl JSON format
	output, err := runCommandOutput(cmdCtx, smartctlPath, "-n", "standby", "-i", "-A", "-H", "--json=o", device)

	// smartctl returns non-zero exit codes for various conditions
	// Exit code 2 means drive is in standby - that's okay
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			// Check for standby (bit 1 set in exit status)
			if exitCode&2 != 0 {
				return &DiskSMART{
					Device:      filepath.Base(device),
					Standby:     true,
					LastUpdated: timeNow(),
				}, nil
			}
			// Other exit codes might still have valid JSON output
			// Continue parsing if we got output
			if len(output) == 0 {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Parse JSON output
	var smartData smartctlJSON
	if err := json.Unmarshal(output, &smartData); err != nil {
		return nil, err
	}

	result := &DiskSMART{
		Device:      filepath.Base(device),
		Model:       smartData.ModelName,
		Serial:      smartData.SerialNumber,
		Type:        detectDiskType(smartData),
		LastUpdated: timeNow(),
	}

	// Build WWN string if available
	if smartData.WWN.NAA != 0 {
		result.WWN = formatWWN(smartData.WWN.NAA, smartData.WWN.OUI, smartData.WWN.ID)
	}

	// Get temperature (different location for NVMe vs SATA)
	if smartData.Temperature.Current > 0 {
		result.Temperature = smartData.Temperature.Current
	} else if smartData.NVMeSmartHealthInformationLog.Temperature > 0 {
		result.Temperature = smartData.NVMeSmartHealthInformationLog.Temperature
	}

	// Get health status
	if smartData.SmartStatus.Passed {
		result.Health = "PASSED"
	} else {
		result.Health = "FAILED"
	}

	log.Debug().
		Str("device", result.Device).
		Str("model", result.Model).
		Int("temperature", result.Temperature).
		Str("health", result.Health).
		Msg("Collected SMART data")

	return result, nil
}

// detectDiskType determines the disk transport type from smartctl output.
func detectDiskType(data smartctlJSON) string {
	protocol := strings.ToLower(data.Device.Protocol)
	switch {
	case strings.Contains(protocol, "nvme"):
		return "nvme"
	case strings.Contains(protocol, "sas"):
		return "sas"
	case strings.Contains(protocol, "ata"), strings.Contains(protocol, "sata"):
		return "sata"
	default:
		// Try to infer from device type
		devType := strings.ToLower(data.Device.Type)
		if strings.Contains(devType, "nvme") {
			return "nvme"
		}
		return "sata" // default
	}
}

// formatWWN formats WWN components into a standard string.
func formatWWN(naa, oui, id uint64) string {
	// Format as hex string: naa-oui-id
	return strconv.FormatUint(naa, 16) + "-" +
		strconv.FormatUint(oui, 16) + "-" +
		strconv.FormatUint(id, 16)
}
