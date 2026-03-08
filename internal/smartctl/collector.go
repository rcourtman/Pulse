// Package smartctl provides S.M.A.R.T. data collection from local disks.
package smartctl

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
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
	timeNow     = time.Now
	runtimeGOOS = runtime.GOOS

	errSMARTDataUnavailable = errors.New("smart data unavailable for device")
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
	} `json:"smart_status"`
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

type lsblkJSON struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Tran       string `json:"tran"`
	Model      string `json:"model"`
	Vendor     string `json:"vendor"`
	Subsystems string `json:"subsystems"`
}

var linuxSMARTVirtualPrefixes = []string{
	"dm-",
	"drbd",
	"loop",
	"md",
	"nbd",
	"pmem",
	"ram",
	"rbd",
	"vd",
	"xvd",
	"zd",
	"zram",
}

var linuxSMARTVirtualMetadataTokens = []string{
	"hyper-v",
	"msft virtual",
	"parallels",
	"qemu",
	"vbox",
	"virtual disk",
	"virtual hd",
	"virtualbox",
	"vmware",
}

var linuxSMARTVirtualSubsystemTokens = []string{
	"drbd",
	"nbd",
	"vmbus",
	"virtio",
	"xen",
	"zfs",
}

// CollectLocal collects S.M.A.R.T. data from all local block devices.
// The diskExclude parameter specifies patterns for devices to skip (e.g., "sda", "/dev/nvme*", "*cache*").
func CollectLocal(ctx context.Context, diskExclude []string) ([]DiskSMART, error) {
	// List block devices
	devices, err := listBlockDevices(ctx, diskExclude)
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
// Devices matching any of the diskExclude patterns are skipped.
func listBlockDevices(ctx context.Context, diskExclude []string) ([]string, error) {
	if runtimeGOOS == "freebsd" {
		return listBlockDevicesFreeBSD(ctx, diskExclude)
	}
	return listBlockDevicesLinux(ctx, diskExclude)
}

// listBlockDevicesLinux uses lsblk to find disks on Linux.
func listBlockDevicesLinux(ctx context.Context, diskExclude []string) ([]string, error) {
	output, err := runCommandOutput(ctx, "lsblk", "-J", "-d", "-o", "NAME,TYPE,TRAN,MODEL,VENDOR,SUBSYSTEMS")
	if err != nil {
		return nil, err
	}

	var data lsblkJSON
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, err
	}

	var devices []string
	for _, disk := range data.Blockdevices {
		if strings.TrimSpace(disk.Name) == "" {
			continue
		}

		devicePath := "/dev/" + disk.Name
		if reason := linuxSMARTSkipReason(disk); reason != "" {
			log.Debug().
				Str("device", devicePath).
				Str("reason", reason).
				Msg("Skipping non-physical device for SMART collection")
			continue
		}
		if matchesDeviceExclude(disk.Name, devicePath, diskExclude) {
			log.Debug().Str("device", devicePath).Msg("Skipping excluded device for SMART collection")
			continue
		}
		devices = append(devices, devicePath)
	}

	return devices, nil
}

func linuxSMARTSkipReason(device lsblkDevice) string {
	if !strings.EqualFold(strings.TrimSpace(device.Type), "disk") {
		return "not a whole disk"
	}

	name := strings.ToLower(strings.TrimSpace(device.Name))
	for _, prefix := range linuxSMARTVirtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return "virtual/logical device prefix"
		}
	}

	transport := strings.ToLower(strings.TrimSpace(device.Tran))
	if transport == "virtio" {
		return "virtio transport"
	}

	subsystems := strings.ToLower(strings.TrimSpace(device.Subsystems))
	for _, token := range linuxSMARTVirtualSubsystemTokens {
		if strings.Contains(subsystems, token) {
			return "virtual/logical subsystem"
		}
	}

	metadata := strings.ToLower(strings.TrimSpace(device.Vendor + " " + device.Model))
	for _, token := range linuxSMARTVirtualMetadataTokens {
		if strings.Contains(metadata, token) {
			return "virtual disk model/vendor signature"
		}
	}

	return ""
}

// listBlockDevicesFreeBSD uses sysctl kern.disks to find disks on FreeBSD.
func listBlockDevicesFreeBSD(ctx context.Context, diskExclude []string) ([]string, error) {
	output, err := runCommandOutput(ctx, "sysctl", "-n", "kern.disks")
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, name := range strings.Fields(strings.TrimSpace(string(output))) {
		if name == "" {
			continue
		}
		devicePath := "/dev/" + name
		if matchesDeviceExclude(name, devicePath, diskExclude) {
			log.Debug().Str("device", devicePath).Msg("Skipping excluded device for SMART collection")
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
		return nil, err
	}

	attempts := smartctlProbeAttempts(device)
	var firstParsed *DiskSMART
	var lastErr error

	for i, args := range attempts {
		output, err := runCommandOutput(cmdCtx, smartctlPath, args...)

		// smartctl returns non-zero exit codes for various conditions.
		// Exit code 2 means drive is in standby - that's okay.
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode := exitErr.ExitCode()
				if exitCode&2 != 0 {
					return &DiskSMART{
						Device:      filepath.Base(device),
						Standby:     true,
						LastUpdated: timeNow(),
					}, nil
				}
				if len(output) == 0 {
					lastErr = err
					continue
				}
			} else {
				lastErr = err
				continue
			}
		}

		result, parseErr := parseSMARTOutput(output, device)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		if firstParsed == nil {
			firstParsed = result
		}
		if !shouldRetryFreeBSDSMART(device, result, i, len(attempts)) {
			log.Debug().
				Str("device", result.Device).
				Str("model", result.Model).
				Int("temperature", result.Temperature).
				Str("health", result.Health).
				Msg("Collected SMART data")
			return result, nil
		}
	}

	if firstParsed != nil {
		log.Debug().
			Str("device", firstParsed.Device).
			Str("model", firstParsed.Model).
			Int("temperature", firstParsed.Temperature).
			Str("health", firstParsed.Health).
			Msg("Collected SMART data")
		return firstParsed, nil
	}
	if lastErr != nil {
		if errors.Is(lastErr, errSMARTDataUnavailable) {
			return nil, nil
		}
		return nil, lastErr
	}
	return nil, nil
}

func smartctlProbeAttempts(device string) [][]string {
	attempts := [][]string{
		smartctlArgs(device, ""),
	}

	for _, deviceType := range freeBSDSmartctlDeviceTypes(filepath.Base(device)) {
		attempts = append(attempts, smartctlArgs(device, deviceType))
	}

	return attempts
}

func smartctlArgs(device, deviceType string) []string {
	args := []string{}
	if deviceType != "" {
		args = append(args, "-d", deviceType)
	}
	args = append(args, "-n", "standby", "-i", "-A", "-H", "--json=o", device)
	return args
}

func freeBSDSmartctlDeviceTypes(device string) []string {
	if runtimeGOOS != "freebsd" {
		return nil
	}

	switch {
	case strings.HasPrefix(device, "ada"), strings.HasPrefix(device, "ad"):
		return []string{"sat"}
	case strings.HasPrefix(device, "da"):
		return []string{"sat,auto", "scsi"}
	case strings.HasPrefix(device, "nvd"), strings.HasPrefix(device, "nvme"):
		return []string{"nvme"}
	default:
		return nil
	}
}

func shouldRetryFreeBSDSMART(device string, result *DiskSMART, attemptIndex, attemptCount int) bool {
	if runtimeGOOS != "freebsd" || attemptIndex >= attemptCount-1 || result == nil {
		return false
	}
	if result.Temperature > 0 {
		return false
	}
	return len(freeBSDSmartctlDeviceTypes(filepath.Base(device))) > 0
}

func parseSMARTOutput(output []byte, device string) (*DiskSMART, error) {
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

	if smartData.WWN.NAA != 0 {
		result.WWN = formatWWN(smartData.WWN.NAA, smartData.WWN.OUI, smartData.WWN.ID)
	}

	if smartData.Temperature.Current > 0 {
		result.Temperature = smartData.Temperature.Current
	} else if smartData.NVMeSmartHealthInformationLog.Temperature > 0 {
		result.Temperature = smartData.NVMeSmartHealthInformationLog.Temperature
	} else {
		for _, attr := range smartData.ATASmartAttributes.Table {
			if attr.ID == 194 || attr.ID == 190 {
				temp := int(parseRawValue(attr.Raw.String, attr.Raw.Value))
				if temp > 0 && temp < 150 {
					result.Temperature = temp
					break
				}
			}
		}
	}

	if smartData.SmartStatus != nil {
		if smartData.SmartStatus.Passed {
			result.Health = "PASSED"
		} else {
			result.Health = "FAILED"
		}
	}

	result.Attributes = parseSMARTAttributes(&smartData, result.Type)
	if result.Health == "" && result.Temperature == 0 && result.Attributes == nil {
		return nil, errSMARTDataUnavailable
	}

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
			raw := parseRawValue(attr.Raw.String, attr.Raw.Value)
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

// parseRawValue extracts the primary integer from a SMART attribute's raw string.
// Some drives (notably Seagate) pack vendor-specific data in the upper bytes of
// the 48-bit raw value, making raw.value unreliable. For example, Power_On_Hours
// may report raw.value=150323855943 while raw.string="16951 (223 173 0)" where
// only 16951 is the actual hours. Falls back to rawValue if string parsing fails.
func parseRawValue(rawString string, rawValue int64) int64 {
	s := strings.TrimSpace(rawString)
	if s == "" {
		return rawValue
	}
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return rawValue
	}
	v, err := strconv.ParseInt(s[:end], 10, 64)
	if err != nil {
		return rawValue
	}
	return v
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
