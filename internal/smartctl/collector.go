// Package smartctl provides S.M.A.R.T. data collection from local disks.
package smartctl

import (
	"context"
	"encoding/json"
	"errors"
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

var (
	execLookPath     = exec.LookPath
	readDir          = os.ReadDir
	readFile         = os.ReadFile
	evalSymlinks     = filepath.EvalSymlinks
	runCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}
	timeNow     = time.Now
	runtimeGOOS = runtime.GOOS

	errSMARTDataUnavailable = errors.New("smart data unavailable for device")
)

const smartctlStandbyExitStatus = 3

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
	ATASCTStatus struct {
		Current struct {
			Value int `json:"value"`
		} `json:"current"`
	} `json:"ata_sct_status"`
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
	NVMeNamespaces []nvmeNamespaceJSON `json:"nvme_namespaces"`
	PowerMode      string              `json:"power_mode"`
}

type nvmeNamespaceJSON struct {
	EUI64 nvmeEUI64JSON `json:"eui64"`
}

type nvmeEUI64JSON struct {
	OUI   uint64 `json:"oui"`
	ExtID uint64 `json:"ext_id"`
}

type smartTextFallback struct {
	Model       string
	Serial      string
	Type        string
	Health      string
	Temperature int
	Standby     bool
	Attributes  *SMARTAttributes
}

var (
	smartTextTempAttributeRE = regexp.MustCompile(`^\s*(190|194)\s+\S+.*-\s+(\d{1,3})\b`)
	smartTextCurrentTempRE   = regexp.MustCompile(`(?i)^current(?: drive)? temperature:\s*(\d{1,3})\b`)
	smartTextTemperatureRE   = regexp.MustCompile(`(?i)^temperature:\s*(\d{1,3})\b`)
)

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
	targets, err := listSMARTTargets(ctx, diskExclude)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to list block devices for SMART collection")
		return nil, err
	}

	if len(targets) == 0 {
		return nil, nil
	}

	var results []DiskSMART
	for _, target := range targets {
		smart, err := collectSMARTTarget(ctx, target)
		if err != nil {
			log.Debug().Err(err).Str("device", target.displayName()).Msg("Failed to collect SMART data for device")
			continue
		}
		if smart != nil {
			results = append(results, *smart)
		}
	}

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
		log.Debug().Err(err).Msg("Failed to enumerate Linux SMART targets via smartctl --scan-open, falling back to block device discovery")
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
		return nil, err
	}

	output, err := runCommandOutput(ctx, smartctlPath, "--scan-open")
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

// listBlockDevicesLinux uses lsblk to find disks on Linux.
func listBlockDevicesLinux(ctx context.Context, diskExclude []string) ([]string, error) {
	devices, err := listBlockDevicesLinuxFromSysfs(diskExclude)
	if err == nil {
		return devices, nil
	}
	if err != nil {
		log.Debug().Err(err).Msg("Failed to enumerate Linux disks from /sys/block, falling back to lsblk")
	}

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
				Str("device", devicePath).
				Str("reason", reason).
				Msg("Skipping non-physical device for SMART collection")
			continue
		}
		if matchesDeviceExclude(name, devicePath, diskExclude) {
			log.Debug().Str("device", devicePath).Msg("Skipping excluded device for SMART collection")
			continue
		}
		devices = append(devices, devicePath)
	}

	return devices, nil
}

func linuxSMARTSkipReasonSysfs(name string) string {
	for _, prefix := range linuxSMARTVirtualPrefixes {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			return "virtual/logical device prefix"
		}
	}

	blockPath := filepath.Join("/sys/block", name)
	if resolved, err := evalSymlinks(blockPath); err == nil && strings.Contains(strings.ToLower(resolved), "/virtual/") {
		return "virtual block device"
	}

	subsystemPath := filepath.Join(blockPath, "device", "subsystem")
	if resolved, err := evalSymlinks(subsystemPath); err == nil {
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
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func listBlockDevicesLinuxFromLSBLK(ctx context.Context, diskExclude []string) ([]string, error) {
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
	names, sysctlErr := freeBSDDiskNamesFromSysctl(ctx)
	if sysctlErr != nil {
		log.Debug().Err(sysctlErr).Msg("Failed to enumerate FreeBSD disks from kern.disks")
	}

	fallbackNames, fallbackErr := freeBSDDiskNamesFromDev()
	if fallbackErr != nil {
		log.Debug().Err(fallbackErr).Msg("Failed to enumerate FreeBSD disks from /dev")
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
			log.Debug().Str("device", devicePath).Msg("Skipping excluded device for SMART collection")
			continue
		}
		devices = append(devices, devicePath)
	}

	return devices, nil
}

func freeBSDDiskNamesFromSysctl(ctx context.Context) ([]string, error) {
	output, err := runCommandOutput(ctx, "sysctl", "-n", "kern.disks")
	if err != nil {
		return nil, err
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
	return collectSMARTTarget(ctx, smartctlTarget{Path: device})
}

func collectSMARTTarget(ctx context.Context, target smartctlTarget) (*DiskSMART, error) {
	// Use timeout to avoid hanging on slow/unresponsive disks
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if smartctl is available
	smartctlPath, err := execLookPath("smartctl")
	if err != nil {
		return nil, err
	}

	attempts := smartctlProbeAttempts(target)
	var firstParsed *DiskSMART
	var firstStandby *DiskSMART
	var lastErr error

	for i, args := range attempts {
		output, err := runCommandOutput(cmdCtx, smartctlPath, args...)

		// smartctl returns non-zero exit codes for various conditions.
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == smartctlStandbyExitStatus && len(output) == 0 {
					standbyResult := &DiskSMART{
						Device:      target.displayName(),
						Standby:     true,
						LastUpdated: timeNow(),
					}
					if runtimeGOOS == "freebsd" && i < len(attempts)-1 && target.DeviceType == "" {
						if firstStandby == nil {
							firstStandby = standbyResult
						}
						continue
					}
					return standbyResult, nil
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
	if firstStandby != nil {
		log.Debug().
			Str("device", firstStandby.Device).
			Msg("Collected SMART standby data")
		return firstStandby, nil
	}
	if lastErr != nil {
		if errors.Is(lastErr, errSMARTDataUnavailable) {
			return nil, nil
		}
		return nil, lastErr
	}
	return nil, nil
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

	output, err := runCommandOutput(ctx, smartctlPath, sctArgs...)
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
		LastUpdated: timeNow(),
	}
	result.Standby = isStandbyPowerMode(smartData.PowerMode)

	if smartData.WWN.NAA != 0 {
		result.WWN = formatWWN(smartData.WWN.NAA, smartData.WWN.OUI, smartData.WWN.ID)
	} else if eui64 := firstNVMeEUI64(smartData.NVMeNamespaces); eui64 != "" {
		result.WWN = eui64
	}

	if smartData.Temperature.Current > 0 {
		result.Temperature = smartData.Temperature.Current
	} else if smartData.NVMeSmartHealthInformationLog.Temperature > 0 {
		result.Temperature = smartData.NVMeSmartHealthInformationLog.Temperature
	} else if smartData.ATASCTStatus.Current.Value > 0 {
		result.Temperature = smartData.ATASCTStatus.Current.Value
	} else {
		for _, attr := range smartData.ATASmartAttributes.Table {
			if attr.ID == 194 || attr.ID == 190 {
				temp := parseRawValue(attr.Raw.String, attr.Raw.Value)
				if temp > 0 && temp < 150 {
					result.Temperature = int(temp)
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

	textFallback := parseSMARTTextFallback(strings.Join(smartData.Smartctl.Output, "\n"))
	applySMARTTextFallback(result, textFallback)
	result.Attributes = mergeSMARTAttributes(parseSMARTAttributes(&smartData, result.Type), textFallback.Attributes)
	if result.Health == "" && result.Temperature == 0 && result.Attributes == nil && !result.Standby {
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
		Attributes:  fallback.Attributes,
		LastUpdated: timeNow(),
	}
	if result.Type == "" {
		if target.DeviceType == "nvme" || strings.HasPrefix(filepath.Base(target.Path), "nvme") || strings.HasPrefix(filepath.Base(target.Path), "nvd") || strings.HasPrefix(filepath.Base(target.Path), "nda") {
			result.Type = "nvme"
		} else {
			result.Type = "sata"
		}
	}
	if result.Health == "" && result.Temperature == 0 && !result.Standby {
		return nil, errSMARTDataUnavailable
	}
	return result, nil
}

func firstNVMeEUI64(namespaces []nvmeNamespaceJSON) string {
	for _, namespace := range namespaces {
		if formatted := formatNVMeEUI64(namespace.EUI64.OUI, namespace.EUI64.ExtID); formatted != "" {
			return formatted
		}
	}
	return ""
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
		case strings.Contains(lower, "nvme log"):
			fallback.Type = "nvme"
		case strings.Contains(lower, "transport protocol:") && strings.Contains(lower, "sas"):
			fallback.Type = "sas"
		case strings.Contains(lower, "sata version is:") || strings.Contains(lower, "ata version is:"):
			if fallback.Type == "" {
				fallback.Type = "sata"
			}
		}

		if attrs, ok := parseSMARTTextAttributeLine(line); ok {
			fallback.Attributes = mergeSMARTAttributes(fallback.Attributes, attrs)
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

func parseSMARTTextAttributeLine(line string) (*SMARTAttributes, bool) {
	if attrs, ok := parseNVMeSMARTTextAttributeLine(line); ok {
		return attrs, true
	}
	return parseATASMARTTextAttributeLine(line)
}

func parseNVMeSMARTTextAttributeLine(line string) (*SMARTAttributes, bool) {
	lower := strings.ToLower(strings.TrimSpace(line))
	if !strings.Contains(lower, ":") {
		return nil, false
	}

	value, ok := parseSMARTTextColonNumber(line)
	if !ok {
		return nil, false
	}

	attrs := &SMARTAttributes{}
	switch {
	case strings.HasPrefix(lower, "power on hours"):
		attrs.PowerOnHours = &value
	case strings.HasPrefix(lower, "power cycles"):
		attrs.PowerCycles = &value
	case strings.HasPrefix(lower, "percentage used"):
		v := int(value)
		attrs.PercentageUsed = &v
	case strings.HasPrefix(lower, "available spare"):
		v := int(value)
		attrs.AvailableSpare = &v
	case strings.HasPrefix(lower, "media and data integrity errors"), strings.HasPrefix(lower, "media errors"):
		attrs.MediaErrors = &value
	case strings.HasPrefix(lower, "unsafe shutdowns"):
		attrs.UnsafeShutdowns = &value
	default:
		return nil, false
	}
	return attrs, true
}

func parseATASMARTTextAttributeLine(line string) (*SMARTAttributes, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, false
	}

	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, false
	}

	rawText := fields[len(fields)-1]
	if len(fields) >= 10 {
		rawText = strings.Join(fields[9:], " ")
	}
	rawValue, ok := parseSMARTTextLeadingNumber(rawText)
	if !ok {
		return nil, false
	}
	raw := parseRawValue(rawText, rawValue)

	attrs := &SMARTAttributes{}
	switch id {
	case 5:
		attrs.ReallocatedSectors = &raw
	case 9:
		attrs.PowerOnHours = &raw
	case 12:
		attrs.PowerCycles = &raw
	case 197:
		attrs.PendingSectors = &raw
	case 198:
		attrs.OfflineUncorrectable = &raw
	case 199:
		attrs.UDMACRCErrors = &raw
	default:
		return nil, false
	}
	return attrs, true
}

func parseSMARTTextColonNumber(line string) (int64, bool) {
	idx := strings.Index(line, ":")
	if idx < 0 || idx == len(line)-1 {
		return 0, false
	}
	return parseSMARTTextLeadingNumber(line[idx+1:])
}

func parseSMARTTextLeadingNumber(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}

	var b strings.Builder
	started := false
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			started = true
			b.WriteRune(r)
		case r == ',' && started:
			continue
		default:
			if started {
				value, err := strconv.ParseInt(b.String(), 10, 64)
				return value, err == nil
			}
			if r != ' ' && r != '\t' {
				return 0, false
			}
		}
	}

	if !started {
		return 0, false
	}
	value, err := strconv.ParseInt(b.String(), 10, 64)
	return value, err == nil
}

func mergeSMARTAttributes(primary, fallback *SMARTAttributes) *SMARTAttributes {
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}

	if primary.PowerOnHours == nil {
		primary.PowerOnHours = fallback.PowerOnHours
	}
	if primary.PowerCycles == nil {
		primary.PowerCycles = fallback.PowerCycles
	}
	if primary.ReallocatedSectors == nil {
		primary.ReallocatedSectors = fallback.ReallocatedSectors
	}
	if primary.PendingSectors == nil {
		primary.PendingSectors = fallback.PendingSectors
	}
	if primary.OfflineUncorrectable == nil {
		primary.OfflineUncorrectable = fallback.OfflineUncorrectable
	}
	if primary.UDMACRCErrors == nil {
		primary.UDMACRCErrors = fallback.UDMACRCErrors
	}
	if primary.PercentageUsed == nil {
		primary.PercentageUsed = fallback.PercentageUsed
	}
	if primary.AvailableSpare == nil {
		primary.AvailableSpare = fallback.AvailableSpare
	}
	if primary.MediaErrors == nil {
		primary.MediaErrors = fallback.MediaErrors
	}
	if primary.UnsafeShutdowns == nil {
		primary.UnsafeShutdowns = fallback.UnsafeShutdowns
	}
	return primary
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
	for end < len(s) && ((s[end] >= '0' && s[end] <= '9') || s[end] == ',') {
		end++
	}
	if end == 0 {
		return rawValue
	}
	v, err := strconv.ParseInt(strings.ReplaceAll(s[:end], ",", ""), 10, 64)
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

// formatWWN formats smartctl's WWN components into Linux by-id style hex.
func formatWWN(naa, oui, id uint64) string {
	if naa == 0 {
		return ""
	}

	idHex := strconv.FormatUint(id, 16)
	if len(idHex) < 9 {
		idHex = strings.Repeat("0", 9-len(idHex)) + idHex
	}
	ouiHex := strconv.FormatUint(oui, 16)
	if len(ouiHex) < 6 {
		ouiHex = strings.Repeat("0", 6-len(ouiHex)) + ouiHex
	}
	return "0x" + strconv.FormatUint(naa, 16) + ouiHex + idHex
}

func formatNVMeEUI64(oui, extID uint64) string {
	if oui == 0 && extID == 0 {
		return ""
	}

	ouiHex := strconv.FormatUint(oui, 16)
	if len(ouiHex) < 6 {
		ouiHex = strings.Repeat("0", 6-len(ouiHex)) + ouiHex
	}
	extHex := strconv.FormatUint(extID, 16)
	if len(extHex) < 10 {
		extHex = strings.Repeat("0", 10-len(extHex)) + extHex
	}
	return "eui." + ouiHex + extHex
}
