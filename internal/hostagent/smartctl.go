package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const smartctlComponent = "smartctl_collector"
const maxCommandOutputBytes = 1 << 20 // 1 MiB
const smartctlStandbyExitStatus = 3

var (
	errCommandOutputTooLarge = errors.New("command output exceeds size limit")
	errSMARTDataUnavailable  = errors.New("smart data unavailable for device")
	execLookPath             = exec.LookPath
	smartRunCommandOutput    = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return runCommandOutputLimited(ctx, maxCommandOutputBytes, name, args...)
	}
	readDir              = os.ReadDir
	smartctlReadFile     = os.ReadFile
	smartctlEvalSymlinks = filepath.EvalSymlinks

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

type nvmeSmartHealthInformationLogJSON struct {
	Temperature     int   `json:"temperature"`
	AvailableSpare  int   `json:"available_spare"`
	PercentageUsed  int   `json:"percentage_used"`
	PowerOnHours    int64 `json:"power_on_hours"`
	UnsafeShutdowns int64 `json:"unsafe_shutdowns"`
	MediaErrors     int64 `json:"media_errors"`
	PowerCycles     int64 `json:"power_cycles"`
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

// linuxSMARTVirtualPrefixes are device name prefixes for virtual/logical
// devices that cannot provide SMART data.
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

// linuxSMARTVirtualMetadataTokens are vendor/model substrings indicating a
// virtual disk that cannot provide SMART data.
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

// linuxSMARTVirtualSubsystemTokens are lsblk SUBSYSTEMS substrings
// indicating virtual block devices.
var linuxSMARTVirtualSubsystemTokens = []string{
	"drbd",
	"nbd",
	"vmbus",
	"virtio",
	"xen",
	"zfs",
}

// smartctlJSON represents the JSON output from smartctl --json=o.
type smartctlJSON struct {
	Smartctl struct {
		Output []string `json:"output"`
	} `json:"smartctl"`
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
	ATASCTStatus struct {
		Current struct {
			Value int `json:"value"`
		} `json:"current"`
	} `json:"ata_sct_status"`
	NVMeSmartHealthInformationLog *nvmeSmartHealthInformationLogJSON `json:"nvme_smart_health_information_log"`
	PowerMode                     string                             `json:"power_mode"`
}

type smartTextFallback struct {
	Model       string
	Serial      string
	Type        string
	Health      string
	Temperature int
	Standby     bool
}

var (
	smartTextTempAttributeRE = regexp.MustCompile(`^\s*(190|194)\s+\S+.*-\s+(\d{1,3})\b`)
	smartTextCurrentTempRE   = regexp.MustCompile(`(?i)^current temperature:\s*(\d{1,3})\b`)
	smartTextTemperatureRE   = regexp.MustCompile(`(?i)^temperature:\s*(\d{1,3})\b`)
)

type smartctlTarget struct {
	Path       string
	DeviceType string
}

func (t smartctlTarget) displayName() string {
	name := filepath.Base(strings.TrimSpace(t.Path))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = strings.TrimSpace(t.Path)
	}
	if t.DeviceType == "" {
		return name
	}
	return name + " [" + t.DeviceType + "]"
}

// CollectSMARTLocal collects S.M.A.R.T. data from all local block devices.
// The diskExclude parameter specifies patterns for devices to skip (e.g., "sda", "/dev/nvme*", "*cache*").
func CollectSMARTLocal(ctx context.Context, diskExclude []string) ([]DiskSMART, error) {
	targets, err := listSMARTTargets(ctx, diskExclude)
	if err != nil {
		log.Debug().Err(err).Msg("failed to list block devices for SMART collection")
		return nil, fmt.Errorf("list block devices for SMART collection: %w", err)
	}

	if len(targets) == 0 {
		return nil, nil
	}

	var results []DiskSMART
	for _, target := range targets {
		smart, err := collectSMARTTarget(ctx, target)
		if err != nil {
			if errors.Is(err, errSMARTDataUnavailable) {
				log.Debug().
					Str("component", smartctlComponent).
					Str("action", "skip_no_smart_data").
					Str("device", target.displayName()).
					Msg("Device returned no usable SMART data, skipping")
			} else {
				log.Debug().
					Str("component", smartctlComponent).
					Str("action", "collect_device_smart_failed").
					Str("device", target.Path).
					Err(err).
					Msg("Failed to collect SMART data for device")
			}
			continue
		}
		if smart != nil {
			results = append(results, *smart)
		}
	}

	log.Debug().
		Str("component", smartctlComponent).
		Str("action", "collect_local_complete").
		Int("devices_discovered", len(targets)).
		Int("devices_collected", len(results)).
		Msg("Completed SMART collection for local devices")

	return results, nil
}

func listSMARTTargets(ctx context.Context, diskExclude []string) ([]smartctlTarget, error) {
	if runtimeGOOS == "linux" {
		return listSMARTTargetsLinux(ctx, diskExclude)
	}

	devices, err := listBlockDevices(ctx, diskExclude)
	if err != nil {
		return nil, err
	}
	return smartctlTargetsFromDevices(devices), nil
}

func smartctlTargetsFromDevices(devices []string) []smartctlTarget {
	if len(devices) == 0 {
		return nil
	}
	targets := make([]smartctlTarget, 0, len(devices))
	for _, device := range devices {
		targets = append(targets, smartctlTarget{Path: device})
	}
	return targets
}

func listSMARTTargetsLinux(ctx context.Context, diskExclude []string) ([]smartctlTarget, error) {
	targets, err := listSMARTTargetsLinuxFromScanOpen(ctx, diskExclude)
	if err == nil && len(targets) > 0 {
		return targets, nil
	}
	if err != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Err(err).
			Msg("Failed to enumerate Linux SMART targets via smartctl --scan-open, falling back to block device discovery")
	}

	devices, fallbackErr := listBlockDevicesLinux(ctx, diskExclude)
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	return smartctlTargetsFromDevices(devices), nil
}

func listSMARTTargetsLinuxFromScanOpen(ctx context.Context, diskExclude []string) ([]smartctlTarget, error) {
	smartctlPath, err := execLookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("look up smartctl binary: %w", err)
	}

	output, err := smartRunCommandOutput(ctx, smartctlPath, "--scan-open")
	if err != nil {
		return nil, err
	}

	return parseSmartctlScanOpenTargets(output, diskExclude), nil
}

func parseSmartctlScanOpenTargets(output []byte, diskExclude []string) []smartctlTarget {
	lines := strings.Split(string(output), "\n")
	targets := make([]smartctlTarget, 0, len(lines))
	typedByPath := make(map[string]bool)
	seen := make(map[string]struct{})

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		path := strings.TrimSpace(fields[0])
		if path == "" || (!strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "-")) {
			continue
		}

		deviceType := ""
		for i := 1; i < len(fields)-1; i++ {
			if fields[i] == "-d" {
				deviceType = strings.TrimSpace(fields[i+1])
				break
			}
		}

		name := filepath.Base(path)
		if matchesDeviceExclude(name, path, diskExclude) {
			continue
		}

		key := path + "\x00" + deviceType
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if deviceType != "" {
			typedByPath[path] = true
		}

		targets = append(targets, smartctlTarget{
			Path:       path,
			DeviceType: deviceType,
		})
	}

	if len(targets) == 0 {
		return nil
	}

	filtered := make([]smartctlTarget, 0, len(targets))
	for _, target := range targets {
		if target.DeviceType == "" && typedByPath[target.Path] {
			continue
		}
		filtered = append(filtered, target)
	}
	return filtered
}

// listBlockDevices returns a list of block devices suitable for SMART queries.
// Devices matching any of the diskExclude patterns are skipped.
func listBlockDevices(ctx context.Context, diskExclude []string) ([]string, error) {
	if runtimeGOOS == "freebsd" {
		return listBlockDevicesFreeBSD(ctx, diskExclude)
	}
	return listBlockDevicesLinux(ctx, diskExclude)
}

func listBlockDevicesLinux(ctx context.Context, diskExclude []string) ([]string, error) {
	devices, err := listBlockDevicesLinuxFromSysfs(diskExclude)
	if err == nil {
		return devices, nil
	}
	log.Debug().
		Str("component", smartctlComponent).
		Err(err).
		Msg("sysfs device discovery failed, falling back to lsblk")

	return listBlockDevicesLinuxFromLSBLK(ctx, diskExclude)
}

func listBlockDevicesLinuxFromSysfs(diskExclude []string) ([]string, error) {
	entries, err := readDir("/sys/block")
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}

		devicePath := "/dev/" + name
		if reason := linuxSMARTSkipReasonSysfs(name); reason != "" {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "skip_virtual_device").
				Str("device", devicePath).
				Str("reason", reason).
				Msg("Skipping non-physical device for SMART collection")
			continue
		}
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

func linuxSMARTSkipReasonSysfs(name string) string {
	lowerName := strings.ToLower(name)
	for _, prefix := range linuxSMARTVirtualPrefixes {
		if strings.HasPrefix(lowerName, prefix) {
			return "virtual/logical device prefix"
		}
	}

	blockPath := filepath.Join("/sys/block", name)
	if resolved, err := smartctlEvalSymlinks(blockPath); err == nil && strings.Contains(strings.ToLower(resolved), "/virtual/") {
		return "virtual block device"
	}

	subsystemPath := filepath.Join(blockPath, "device", "subsystem")
	if resolved, err := smartctlEvalSymlinks(subsystemPath); err == nil {
		lowerResolved := strings.ToLower(resolved)
		for _, token := range linuxSMARTVirtualSubsystemTokens {
			if strings.Contains(lowerResolved, token) {
				return "virtual/logical subsystem"
			}
		}
	}

	metadata := strings.ToLower(strings.TrimSpace(
		readTrimmedFile(filepath.Join(blockPath, "device", "vendor")) + " " +
			readTrimmedFile(filepath.Join(blockPath, "device", "model")),
	))
	for _, token := range linuxSMARTVirtualMetadataTokens {
		if strings.Contains(metadata, token) {
			return "virtual disk model/vendor signature"
		}
	}

	return ""
}

func readTrimmedFile(path string) string {
	data, err := smartctlReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func listBlockDevicesLinuxFromLSBLK(ctx context.Context, diskExclude []string) ([]string, error) {
	output, err := smartRunCommandOutput(ctx, "lsblk", "-J", "-d", "-o", "NAME,TYPE,TRAN,MODEL,VENDOR,SUBSYSTEMS")
	if err != nil {
		return nil, err
	}

	var data lsblkJSON
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("parse lsblk JSON: %w", err)
	}

	var devices []string
	for _, disk := range data.Blockdevices {
		if strings.TrimSpace(disk.Name) == "" {
			continue
		}

		devicePath := "/dev/" + disk.Name
		if reason := linuxSMARTSkipReason(disk); reason != "" {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "skip_virtual_device").
				Str("device", devicePath).
				Str("reason", reason).
				Msg("Skipping non-physical device for SMART collection")
			continue
		}
		if matchesDeviceExclude(disk.Name, devicePath, diskExclude) {
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

// linuxSMARTSkipReason returns a human-readable reason if the device should be
// skipped for SMART collection, or "" if the device is a real physical disk.
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

// listBlockDevicesFreeBSD uses sysctl kern.disks and /dev fallback to find disks on FreeBSD.
func listBlockDevicesFreeBSD(ctx context.Context, diskExclude []string) ([]string, error) {
	names, sysctlErr := freeBSDDiskNamesFromSysctl(ctx)
	if sysctlErr != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Err(sysctlErr).
			Msg("Failed to enumerate FreeBSD disks from kern.disks")
	}

	fallbackNames, fallbackErr := freeBSDDiskNamesFromDev()
	if fallbackErr != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Err(fallbackErr).
			Msg("Failed to enumerate FreeBSD disks from /dev")
	}

	if len(names) == 0 {
		names = fallbackNames
	} else if len(fallbackNames) > 0 {
		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			seen[name] = struct{}{}
		}
		for _, name := range fallbackNames {
			if _, ok := seen[name]; ok {
				continue
			}
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		switch {
		case sysctlErr != nil:
			return nil, sysctlErr
		case fallbackErr != nil:
			return nil, fallbackErr
		default:
			return nil, nil
		}
	}

	var devices []string
	for _, name := range names {
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

func freeBSDDiskNamesFromSysctl(ctx context.Context) ([]string, error) {
	output, err := smartRunCommandOutput(ctx, "sysctl", "-n", "kern.disks")
	if err != nil {
		return nil, fmt.Errorf("run sysctl kern.disks: %w", err)
	}

	var devices []string
	seen := make(map[string]struct{})
	for _, name := range strings.Fields(strings.TrimSpace(string(output))) {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		devices = append(devices, name)
	}

	return devices, nil
}

func freeBSDDiskNamesFromDev() ([]string, error) {
	entries, err := readDir("/dev")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if !isFreeBSDDiskDeviceName(name) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	return names, nil
}

func isFreeBSDDiskDeviceName(name string) bool {
	for _, prefix := range []string{
		"ad",
		"ada",
		"aacd",
		"amrd",
		"da",
		"idad",
		"ipsd",
		"mfid",
		"mfisyspd",
		"mlxd",
		"mmcsd",
		"nda",
		"nvd",
		"nvme",
		"twa",
		"twed",
		"tws",
		"vtbd",
		"xbd",
	} {
		if hasNumericSuffix(name, prefix) {
			return true
		}
	}

	return false
}

func hasNumericSuffix(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return false
	}

	for _, r := range name[len(prefix):] {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
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

		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
			substring := pattern[1 : len(pattern)-1]
			if strings.Contains(name, substring) || strings.Contains(devicePath, substring) {
				return true
			}
			continue
		}

		if strings.HasSuffix(pattern, "*") {
			prefix := pattern[:len(pattern)-1]
			if strings.HasPrefix(name, prefix) || strings.HasPrefix(devicePath, prefix) {
				return true
			}
			continue
		}

		if name == pattern || devicePath == pattern {
			return true
		}
	}

	return false
}

// collectDeviceSMART runs smartctl on a single device and parses the result.
func collectDeviceSMART(ctx context.Context, device string) (*DiskSMART, error) {
	return collectSMARTTarget(ctx, smartctlTarget{Path: device})
}

func collectSMARTTarget(ctx context.Context, target smartctlTarget) (*DiskSMART, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	smartctlPath, err := execLookPath("smartctl")
	if err != nil {
		return nil, fmt.Errorf("look up smartctl binary: %w", err)
	}

	attempts := smartctlProbeAttempts(target)
	var firstParsed *DiskSMART
	var firstStandby *DiskSMART
	var lastErr error

	for i, args := range attempts {
		output, err := smartRunCommandOutput(cmdCtx, smartctlPath, args...)
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode := exitErr.ExitCode()
				if (exitCode == smartctlStandbyExitStatus || exitCode&2 != 0) && len(output) == 0 {
					standbyResult := &DiskSMART{
						Device:      filepath.Base(target.Path),
						Standby:     true,
						LastUpdated: timeNow(),
					}
					if runtimeGOOS == "freebsd" && i < len(attempts)-1 && target.DeviceType == "" {
						if firstStandby == nil {
							firstStandby = standbyResult
						}
						continue
					}
					log.Debug().
						Str("component", smartctlComponent).
						Str("action", "device_in_standby").
						Str("device", filepath.Base(target.Path)).
						Msg("Skipping SMART collection for standby device")
					return standbyResult, nil
				}
				if len(output) == 0 {
					lastErr = fmt.Errorf("run smartctl for %s: %w", target.Path, err)
					continue
				}
				log.Debug().
					Str("component", smartctlComponent).
					Str("action", "collect_device_smart_nonzero_exit").
					Str("device", filepath.Base(target.Path)).
					Int("exit_code", exitCode).
					Msg("smartctl returned non-zero exit status with JSON output")
			} else {
				lastErr = fmt.Errorf("run smartctl for %s: %w", target.Path, err)
				continue
			}
		}

		result, parseErr := parseSMARTOutput(output, target)
		if parseErr != nil {
			lastErr = parseErr
			continue
		}
		result = enrichFreeBSDSCTTemperature(cmdCtx, smartctlPath, args, target, result)
		if firstParsed == nil {
			firstParsed = result
		}
		if !shouldRetryFreeBSDSMART(target.Path, result, i, len(attempts)) {
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
	}

	if firstParsed != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Str("action", "collect_device_smart_success").
			Str("device", firstParsed.Device).
			Str("type", firstParsed.Type).
			Str("model", firstParsed.Model).
			Int("temperature", firstParsed.Temperature).
			Str("health", firstParsed.Health).
			Msg("collected SMART data")
		return firstParsed, nil
	}
	if firstStandby != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Str("action", "device_in_standby").
			Str("device", filepath.Base(target.Path)).
			Msg("Skipping SMART collection for standby device")
		return firstStandby, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errSMARTDataUnavailable
}

func smartctlProbeAttempts(target smartctlTarget) [][]string {
	device := target.Path
	if target.DeviceType != "" {
		return [][]string{
			smartctlArgs(device, target.DeviceType),
		}
	}

	if runtimeGOOS == "freebsd" {
		deviceTypes := freeBSDSmartctlDeviceTypes(filepath.Base(device))
		if len(deviceTypes) > 0 {
			attempts := make([][]string, 0, len(deviceTypes)+1)
			for _, deviceType := range deviceTypes {
				attempts = append(attempts, smartctlArgs(device, deviceType))
			}
			return append(attempts, smartctlArgs(device, ""))
		}
	}

	return [][]string{
		smartctlArgs(device, ""),
	}
}

func smartctlArgs(device, deviceType string) []string {
	args := []string{}
	if deviceType != "" {
		args = append(args, "-d", deviceType)
	}
	args = append(args, "-n", "standby,"+strconv.Itoa(smartctlStandbyExitStatus), "-i", "-A", "-H", "--json=o", device)
	return args
}

func smartctlArgsWithLog(args []string, logPage string) []string {
	if logPage == "" || len(args) == 0 {
		return append([]string(nil), args...)
	}
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-l" && args[i+1] == logPage {
			return append([]string(nil), args...)
		}
	}

	deviceIndex := len(args) - 1
	withLog := make([]string, 0, len(args)+2)
	withLog = append(withLog, args[:deviceIndex]...)
	withLog = append(withLog, "-l", logPage)
	withLog = append(withLog, args[deviceIndex:]...)
	return withLog
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
	case strings.HasPrefix(device, "nda"), strings.HasPrefix(device, "nvd"), strings.HasPrefix(device, "nvme"):
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
	if result.Standby {
		return true
	}
	return len(freeBSDSmartctlDeviceTypes(filepath.Base(device))) > 0
}

func enrichFreeBSDSCTTemperature(ctx context.Context, smartctlPath string, args []string, target smartctlTarget, current *DiskSMART) *DiskSMART {
	if runtimeGOOS != "freebsd" || current == nil || current.Standby || current.Temperature > 0 {
		return current
	}
	if len(freeBSDSmartctlDeviceTypes(filepath.Base(target.Path))) == 0 {
		return current
	}

	sctArgs := smartctlArgsWithLog(args, "scttempsts")
	if len(sctArgs) == len(args) {
		return current
	}

	output, err := smartRunCommandOutput(ctx, smartctlPath, sctArgs...)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || len(output) == 0 {
			return current
		}
	}

	sctResult, parseErr := parseSMARTOutput(output, target)
	if parseErr != nil || sctResult == nil || sctResult.Temperature <= 0 {
		return current
	}
	return sctResult
}

func parseSMARTOutput(output []byte, target smartctlTarget) (*DiskSMART, error) {
	var smartData smartctlJSON
	if err := json.Unmarshal(output, &smartData); err != nil {
		return parseSMARTTextOutput(string(output), target)
	}

	result := &DiskSMART{
		Device:      target.displayName(),
		Model:       smartData.ModelName,
		Serial:      smartData.SerialNumber,
		Type:        detectDiskType(smartData),
		Standby:     isStandbyPowerMode(smartData.PowerMode),
		LastUpdated: timeNow(),
	}

	if smartData.WWN.NAA != 0 {
		result.WWN = formatWWN(smartData.WWN.NAA, smartData.WWN.OUI, smartData.WWN.ID)
	}

	if smartData.Temperature.Current > 0 {
		result.Temperature = smartData.Temperature.Current
	} else if smartData.NVMeSmartHealthInformationLog != nil && smartData.NVMeSmartHealthInformationLog.Temperature > 0 {
		result.Temperature = smartData.NVMeSmartHealthInformationLog.Temperature
	} else if smartData.ATASCTStatus.Current.Value > 0 {
		result.Temperature = smartData.ATASCTStatus.Current.Value
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

	applySMARTTextFallback(result, parseSMARTTextFallback(strings.Join(smartData.Smartctl.Output, "\n")))
	if result.Health == "" {
		result.Health = "UNKNOWN"
	}

	result.Attributes = parseSMARTAttributes(&smartData, result.Type)
	if result.Health == "UNKNOWN" && result.Temperature == 0 && result.Attributes == nil && !result.Standby {
		return nil, errSMARTDataUnavailable
	}

	return result, nil
}

func parseSMARTTextOutput(text string, target smartctlTarget) (*DiskSMART, error) {
	fallback := parseSMARTTextFallback(text)
	result := &DiskSMART{
		Device:      target.displayName(),
		Model:       fallback.Model,
		Serial:      fallback.Serial,
		Type:        fallback.Type,
		Temperature: fallback.Temperature,
		Health:      fallback.Health,
		Standby:     fallback.Standby,
		LastUpdated: timeNow(),
	}
	if result.Type == "" {
		switch {
		case target.DeviceType == "nvme",
			strings.HasPrefix(filepath.Base(target.Path), "nvme"),
			strings.HasPrefix(filepath.Base(target.Path), "nvd"),
			strings.HasPrefix(filepath.Base(target.Path), "nda"):
			result.Type = "nvme"
		default:
			result.Type = "sata"
		}
	}
	if result.Health == "" {
		result.Health = "UNKNOWN"
	}
	if result.Health == "UNKNOWN" && result.Temperature == 0 && !result.Standby {
		return nil, errSMARTDataUnavailable
	}
	return result, nil
}

func applySMARTTextFallback(result *DiskSMART, fallback smartTextFallback) {
	if result == nil {
		return
	}
	if result.Model == "" && fallback.Model != "" {
		result.Model = fallback.Model
	}
	if result.Serial == "" && fallback.Serial != "" {
		result.Serial = fallback.Serial
	}
	if result.Type == "" && fallback.Type != "" {
		result.Type = fallback.Type
	}
	if result.Health == "" && fallback.Health != "" {
		result.Health = fallback.Health
	}
	if result.Temperature == 0 && fallback.Temperature > 0 {
		result.Temperature = fallback.Temperature
	}
	if !result.Standby && fallback.Standby {
		result.Standby = true
	}
}

func parseSMARTTextFallback(text string) smartTextFallback {
	var fallback smartTextFallback
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "device model:"):
			fallback.Model = strings.TrimSpace(line[len("Device Model:"):])
		case strings.HasPrefix(lower, "model number:"):
			if fallback.Model == "" {
				fallback.Model = strings.TrimSpace(line[len("Model Number:"):])
			}
		case strings.HasPrefix(lower, "product:"):
			if fallback.Model == "" {
				fallback.Model = strings.TrimSpace(line[len("Product:"):])
			}
		case strings.HasPrefix(lower, "serial number:"):
			fallback.Serial = strings.TrimSpace(line[len("Serial Number:"):])
		case strings.Contains(lower, "device is in standby mode"):
			fallback.Standby = true
		case strings.HasPrefix(lower, "smart overall-health self-assessment test result:"):
			fallback.Health = parseSMARTHealthText(line)
		case strings.HasPrefix(lower, "smart health status:"):
			if fallback.Health == "" {
				fallback.Health = parseSMARTHealthText(line)
			}
		case strings.Contains(lower, "transport protocol:") && strings.Contains(lower, "nvme"):
			fallback.Type = "nvme"
		case strings.Contains(lower, "transport protocol:") && strings.Contains(lower, "sas"):
			fallback.Type = "sas"
		case strings.Contains(lower, "sata version is:") || strings.Contains(lower, "ata version is:"):
			if fallback.Type == "" {
				fallback.Type = "sata"
			}
		}

		if fallback.Temperature == 0 {
			if matches := smartTextCurrentTempRE.FindStringSubmatch(line); len(matches) == 2 {
				if temp, err := strconv.Atoi(matches[1]); err == nil && temp > 0 && temp < 150 {
					fallback.Temperature = temp
					continue
				}
			}
			if matches := smartTextTemperatureRE.FindStringSubmatch(line); len(matches) == 2 && strings.Contains(lower, "celsius") {
				if temp, err := strconv.Atoi(matches[1]); err == nil && temp > 0 && temp < 150 {
					fallback.Temperature = temp
					continue
				}
			}
			if matches := smartTextTempAttributeRE.FindStringSubmatch(line); len(matches) == 3 {
				if temp, err := strconv.Atoi(matches[2]); err == nil && temp > 0 && temp < 150 {
					fallback.Temperature = temp
				}
			}
		}
	}
	return fallback
}

func parseSMARTHealthText(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "passed"), strings.Contains(lower, "ok"):
		return "PASSED"
	case strings.Contains(lower, "failed"):
		return "FAILED"
	default:
		return ""
	}
}

func isStandbyPowerMode(powerMode string) bool {
	mode := strings.ToLower(strings.TrimSpace(powerMode))
	return strings.Contains(mode, "standby") || strings.Contains(mode, "sleep")
}

// parseSMARTAttributes extracts normalized SMART attributes from smartctl JSON output.
func parseSMARTAttributes(data *smartctlJSON, diskType string) *SMARTAttributes {
	attrs := &SMARTAttributes{}
	hasData := false

	if diskType == "nvme" {
		if data.NVMeSmartHealthInformationLog != nil {
			hasData = true
			nvmeLog := data.NVMeSmartHealthInformationLog
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
		for _, attr := range data.ATASmartAttributes.Table {
			hasData = true
			raw := parseRawValue(attr.Raw.String, attr.Raw.Value)
			switch attr.ID {
			case 5:
				v := raw
				attrs.ReallocatedSectors = &v
			case 9:
				v := raw
				attrs.PowerOnHours = &v
			case 12:
				v := raw
				attrs.PowerCycles = &v
			case 197:
				v := raw
				attrs.PendingSectors = &v
			case 198:
				v := raw
				attrs.OfflineUncorrectable = &v
			case 199:
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
		devType := strings.ToLower(data.Device.Type)
		if strings.Contains(devType, "nvme") {
			return "nvme"
		}
		return "sata"
	}
}

// formatWWN formats WWN components into a standard string.
func formatWWN(naa, oui, id uint64) string {
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
