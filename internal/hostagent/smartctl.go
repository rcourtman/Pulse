package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const smartctlComponent = "smartctl_collector"

var (
	errCommandOutputTooLarge = errors.New("command output exceeds size limit")
	execLookPath             = exec.LookPath
	runCommandOutput         = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return runCommandOutputLimited(ctx, maxCommandOutputBytes, name, args...)
	}
	timeNow     = time.Now
	runtimeGOOS = runtime.GOOS
)

// DiskSMART represents S.M.A.R.T. data for a single disk.
type DiskSMART struct {
	Device      string           `json:"device"`            // Device path (e.g., /dev/sda)
	Model       string           `json:"model,omitempty"`   // Disk model
	Serial      string           `json:"serial,omitempty"`  // Serial number
	WWN         string           `json:"wwn,omitempty"`     // World Wide Name
	Type        string           `json:"type,omitempty"`    // Transport type: sata, sas, nvme
	Temperature int              `json:"temperature"`       // Temperature in Celsius
	Health      string           `json:"health,omitempty"`  // PASSED, FAILED, UNKNOWN
	Standby     bool             `json:"standby,omitempty"` // True if disk was in standby
	Attributes  *SMARTAttributes `json:"attributes,omitempty"`
	LastUpdated time.Time        `json:"lastUpdated"` // When this reading was taken
}

// SMARTAttributes holds normalized SMART attributes for both SATA and NVMe disks.
// Pointer fields distinguish zero from absent.
type SMARTAttributes struct {
	// Common attributes
	PowerOnHours *int64 `json:"powerOnHours,omitempty"`
	PowerCycles  *int64 `json:"powerCycles,omitempty"`

	// SATA-specific (by ATA attribute ID)
	ReallocatedSectors   *int64 `json:"reallocatedSectors,omitempty"`   // ID 5
	PendingSectors       *int64 `json:"pendingSectors,omitempty"`       // ID 197
	OfflineUncorrectable *int64 `json:"offlineUncorrectable,omitempty"` // ID 198
	UDMACRCErrors        *int64 `json:"udmaCrcErrors,omitempty"`        // ID 199

	// NVMe-specific
	PercentageUsed  *int   `json:"percentageUsed,omitempty"`
	AvailableSpare  *int   `json:"availableSpare,omitempty"`
	MediaErrors     *int64 `json:"mediaErrors,omitempty"`
	UnsafeShutdowns *int64 `json:"unsafeShutdowns,omitempty"`
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
	SmartStatus *struct {
		Passed bool `json:"passed"`
	} `json:"smart_status,omitempty"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	// ATA SMART attributes table
	ATASmartAttributes struct {
		Table []struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Value  int    `json:"value"`
			Worst  int    `json:"worst"`
			Thresh int    `json:"thresh"`
			Raw    struct {
				Value  int64  `json:"value"`
				String string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	// NVMe-specific health information
	NVMeSmartHealthInformationLog struct {
		Temperature     int   `json:"temperature"`
		AvailableSpare  int   `json:"available_spare"`
		PercentageUsed  int   `json:"percentage_used"`
		PowerOnHours    int64 `json:"power_on_hours"`
		UnsafeShutdowns int64 `json:"unsafe_shutdowns"`
		MediaErrors     int64 `json:"media_errors"`
		PowerCycles     int64 `json:"power_cycles"`
	} `json:"nvme_smart_health_information_log"`
	PowerMode string `json:"power_mode"`
}

// CollectSMARTLocal collects S.M.A.R.T. data from all local block devices.
// The diskExclude parameter specifies patterns for devices to skip (e.g., "sda", "/dev/nvme*", "*cache*").
func CollectSMARTLocal(ctx context.Context, diskExclude []string) ([]DiskSMART, error) {
	// List block devices
	devices, err := listBlockDevices(ctx, diskExclude)
	if err != nil {
		log.Debug().Err(err).Msg("failed to list block devices for SMART collection")
		return nil, fmt.Errorf("list block devices for SMART collection: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	var results []DiskSMART
	for _, dev := range devices {
		smart, err := collectDeviceSMART(ctx, dev)
		if err != nil {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "collect_device_smart_failed").
				Str("device", dev).
				Err(err).
				Msg("Failed to collect SMART data for device")
			continue
		}
		if smart != nil {
			results = append(results, *smart)
		}
	}

	log.Debug().
		Str("component", smartctlComponent).
		Str("action", "collect_local_complete").
		Int("devices_discovered", len(devices)).
		Int("devices_collected", len(results)).
		Msg("Completed SMART collection for local devices")

	return results, nil
}

// listBlockDevices returns a list of block devices suitable for SMART queries.
// Devices matching any of the diskExclude patterns are skipped.
func listBlockDevices(ctx context.Context, diskExclude []string) ([]string, error) {
	if runtimeGOOS == "freebsd" {
		devices, err := listBlockDevicesFreeBSD(ctx, diskExclude)
		if err != nil {
			return nil, fmt.Errorf("list FreeBSD block devices: %w", err)
		}
		return devices, nil
	}
	devices, err := listBlockDevicesLinux(ctx, diskExclude)
	if err != nil {
		return nil, fmt.Errorf("list Linux block devices: %w", err)
	}
	return devices, nil
}

// listBlockDevicesLinux uses lsblk to find disks on Linux.
func listBlockDevicesLinux(ctx context.Context, diskExclude []string) ([]string, error) {
	output, err := runCommandOutput(ctx, "lsblk", "-d", "-n", "-o", "NAME,TYPE")
	if err != nil {
		return nil, fmt.Errorf("run lsblk for block devices: %w", err)
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
			devicePath := "/dev/" + name
			if matchesDeviceExclude(name, devicePath, diskExclude) {
				log.Debug().
					Str("component", smartctlComponent).
					Str("action", "skip_excluded_device").
					Str("device", devicePath).
					Msg("Skipping excluded device for SMART collection")
				continue
			}
			devices = append(devices, devicePath)
		}
	}

	return devices, nil
}

// listBlockDevicesFreeBSD uses sysctl kern.disks to find disks on FreeBSD.
func listBlockDevicesFreeBSD(ctx context.Context, diskExclude []string) ([]string, error) {
	output, err := runCommandOutput(ctx, "sysctl", "-n", "kern.disks")
	if err != nil {
		return nil, fmt.Errorf("run sysctl kern.disks: %w", err)
	}

	var devices []string
	for _, name := range strings.Fields(strings.TrimSpace(string(output))) {
		if name == "" {
			continue
		}
		devicePath := "/dev/" + name
		if matchesDeviceExclude(name, devicePath, diskExclude) {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "skip_excluded_device").
				Str("device", devicePath).
				Msg("Skipping excluded device for SMART collection")
			continue
		}
		devices = append(devices, devicePath)
	}

	return devices, nil
}

// matchesDeviceExclude checks if a block device matches any exclusion pattern.
// Patterns can match against the device name (e.g., "sda", "nvme0n1") or the full
// path (e.g., "/dev/sda"). Supports:
//   - Exact match: "sda" matches device named "sda"
//   - Prefix pattern (ending with *): "nvme*" matches "nvme0n1", "nvme1n1", etc.
//   - Contains pattern (surrounded by *): "*cache*" matches any device with "cache" in name
func matchesDeviceExclude(name, devicePath string, excludePatterns []string) bool {
	if len(excludePatterns) == 0 {
		return false
	}

	for _, pattern := range excludePatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Contains pattern: *substring*
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
			substring := pattern[1 : len(pattern)-1]
			if strings.Contains(name, substring) || strings.Contains(devicePath, substring) {
				return true
			}
			continue
		}

		// Prefix pattern: prefix*
		if strings.HasSuffix(pattern, "*") {
			prefix := pattern[:len(pattern)-1]
			if strings.HasPrefix(name, prefix) || strings.HasPrefix(devicePath, prefix) {
				return true
			}
			continue
		}

		// Exact match against name or full path
		if name == pattern || devicePath == pattern {
			return true
		}
	}

	return false
}

// collectDeviceSMART runs smartctl on a single device and parses the result.
func collectDeviceSMART(ctx context.Context, device string) (*DiskSMART, error) {
	// Use timeout to avoid hanging on slow/unresponsive disks
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if smartctl is available
	smartctlPath, err := execLookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("look up smartctl binary: %w", err)
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
				log.Debug().
					Str("component", smartctlComponent).
					Str("action", "device_in_standby").
					Str("device", filepath.Base(device)).
					Msg("Skipping SMART collection for standby device")
				return &DiskSMART{
					Device:      filepath.Base(device),
					Standby:     true,
					LastUpdated: timeNow(),
				}, nil
			}
			// Other exit codes might still have valid JSON output
			// Continue parsing if we got output
			if len(output) == 0 {
				return nil, fmt.Errorf("run smartctl for %s: %w", device, err)
			}
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "collect_device_smart_nonzero_exit").
				Str("device", filepath.Base(device)).
				Int("exit_code", exitCode).
				Msg("smartctl returned non-zero exit status with JSON output")
		} else {
			return nil, fmt.Errorf("run smartctl for %s: %w", device, err)
		}
	}

	// Parse JSON output
	var smartData smartctlJSON
	if err := json.Unmarshal(output, &smartData); err != nil {
		return nil, fmt.Errorf("parse smartctl JSON for %s: %w", device, err)
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

	// Get health status. Some devices/versions omit smart_status entirely.
	if smartData.SmartStatus == nil {
		result.Health = "UNKNOWN"
	} else if smartData.SmartStatus.Passed {
		result.Health = "PASSED"
	} else {
		result.Health = "FAILED"
	}

	// Parse SMART attributes
	result.Attributes = parseSMARTAttributes(&smartData, result.Type)

	log.Debug().
		Str("component", smartctlComponent).
		Str("action", "collect_device_smart_success").
		Str("device", result.Device).
		Str("type", result.Type).
		Str("model", result.Model).
		Int("temperature", result.Temperature).
		Str("health", result.Health).
		Msg("collected SMART data")

	return result, nil
}

// parseSMARTAttributes extracts normalized SMART attributes from smartctl JSON output.
func parseSMARTAttributes(data *smartctlJSON, diskType string) *SMARTAttributes {
	attrs := &SMARTAttributes{}
	hasData := false

	if diskType == "nvme" {
		nvmeLog := &data.NVMeSmartHealthInformationLog
		// NVMe health log fields are always present when the log is available.
		// We use simple heuristics: power_on_hours > 0 means the log was populated.
		if nvmeLog.PowerOnHours > 0 || nvmeLog.PowerCycles > 0 || nvmeLog.AvailableSpare > 0 {
			hasData = true
			poh := nvmeLog.PowerOnHours
			attrs.PowerOnHours = &poh
			pc := nvmeLog.PowerCycles
			attrs.PowerCycles = &pc
			pu := nvmeLog.PercentageUsed
			attrs.PercentageUsed = &pu
			as := nvmeLog.AvailableSpare
			attrs.AvailableSpare = &as
			me := nvmeLog.MediaErrors
			attrs.MediaErrors = &me
			us := nvmeLog.UnsafeShutdowns
			attrs.UnsafeShutdowns = &us
		}
	} else {
		// SATA / SAS — iterate the ATA attributes table
		for _, attr := range data.ATASmartAttributes.Table {
			hasData = true
			raw := attr.Raw.Value
			switch attr.ID {
			case 5: // Reallocated Sector Count
				v := raw
				attrs.ReallocatedSectors = &v
			case 9: // Power-On Hours
				v := raw
				attrs.PowerOnHours = &v
			case 12: // Power Cycle Count
				v := raw
				attrs.PowerCycles = &v
			case 197: // Current Pending Sector Count
				v := raw
				attrs.PendingSectors = &v
			case 198: // Offline Uncorrectable
				v := raw
				attrs.OfflineUncorrectable = &v
			case 199: // UDMA CRC Error Count
				v := raw
				attrs.UDMACRCErrors = &v
			}
		}
	}

	if !hasData {
		return nil
	}
	return attrs
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

func runCommandOutputLimited(ctx context.Context, maxBytes int, name string, args ...string) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max bytes must be positive")
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	output := make([]byte, 0, 4096)
	buf := make([]byte, 32*1024)
	exceeded := false

	for {
		n, readErr := stdout.Read(buf)
		if n > 0 {
			remaining := maxBytes - len(output)
			if remaining > 0 {
				if n <= remaining {
					output = append(output, buf[:n]...)
				} else {
					output = append(output, buf[:remaining]...)
					exceeded = true
				}
			} else {
				exceeded = true
			}

			if exceeded && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = cmd.Wait()
			return output, readErr
		}
	}

	waitErr := cmd.Wait()
	if exceeded {
		return nil, fmt.Errorf("%w (%d bytes)", errCommandOutputTooLarge, maxBytes)
	}
	if waitErr != nil {
		return output, waitErr
	}

	return output, nil
}
