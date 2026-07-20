package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
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

	smartCollectionConcurrency       = 6
	smartCollectionParallelThreshold = 12
)

// DiskSMART represents S.M.A.R.T. data for a single disk.
type DiskSMART struct {
	Device      string                          `json:"device"`               // Block device name (e.g., sda, nvme0n1)
	Model       string                          `json:"model,omitempty"`      // Disk model
	Serial      string                          `json:"serial,omitempty"`     // Serial number
	WWN         string                          `json:"wwn,omitempty"`        // World Wide Name
	Type        string                          `json:"type,omitempty"`       // Transport type: sata, sas, nvme
	Controller  string                          `json:"controller,omitempty"` // PCI/controller association when reported
	Target      string                          `json:"target,omitempty"`     // HCTL or smartctl controller-member target
	SizeBytes   int64                           `json:"sizeBytes,omitempty"`  // Capacity in bytes (0 when unknown)
	Temperature int                             `json:"temperature"`          // Temperature in Celsius
	Health      string                          `json:"health,omitempty"`     // PASSED, FAILED, UNKNOWN
	Standby     bool                            `json:"standby,omitempty"`    // True if disk was in standby
	Collection  *diskinventory.CollectionStatus `json:"collection,omitempty"`
	Attributes  *SMARTAttributes                `json:"attributes,omitempty"`
	LastUpdated time.Time                       `json:"lastUpdated"` // When this reading was taken
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
	UserCapacity struct {
		Bytes int64 `json:"bytes"`
	} `json:"user_capacity"`
	NVMeTotalCapacity int64 `json:"nvme_total_capacity"`
	SmartStatus       *struct {
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
	smartTextCurrentTempRE   = regexp.MustCompile(`(?i)^current(?: drive)? temperature:\s*(\d{1,3})\b`)
	smartTextTemperatureRE   = regexp.MustCompile(`(?i)^temperature:\s*(\d{1,3})\b`)
	linuxDirectSATDeviceRE   = regexp.MustCompile(`^(sd|hd)[a-z]+$`)
	pciControllerAddressRE   = regexp.MustCompile(`(?i)^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\.[0-7]$`)
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

	type smartOutcome struct {
		smart *DiskSMART
		err   error
	}
	outcomes := make([]smartOutcome, len(targets))
	workerCount := smartCollectionConcurrency
	if len(targets) < smartCollectionParallelThreshold {
		workerCount = 1
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(targets) {
		workerCount = len(targets)
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				outcomes[index].smart, outcomes[index].err = collectSMARTTarget(ctx, targets[index])
			}
		}()
	}
	for index := range targets {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	var results []DiskSMART
	var missed []smartctlTarget
	collected := make(map[string]struct{}, len(targets))
	multiplexed := make(map[string]struct{})
	for _, target := range targets {
		block := canonicalBlockDeviceForScanPath(target.Path)
		if isMultiplexedDeviceType(target.DeviceType) && block != "" {
			multiplexed[block] = struct{}{}
		}
	}
	for index, target := range targets {
		block := canonicalBlockDeviceForScanPath(target.Path)
		smart, err := outcomes[index].smart, outcomes[index].err
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
			missed = append(missed, target)
			continue
		}
		if smart == nil {
			missed = append(missed, target)
			continue
		}
		refineLinuxBlockDeviceIdentity(smart, target)
		// The refine step can rename the device (nvme0 -> nvme0n1), so re-apply
		// exclusions against the canonical name the user actually sees.
		if matchesDeviceExclude(smart.Device, "/dev/"+smart.Device, diskExclude) {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "skip_excluded_device").
				Str("device", smart.Device).
				Msg("Skipping excluded device for SMART collection")
			if block != "" {
				collected[block] = struct{}{}
			}
			continue
		}
		results = append(results, *smart)
		if block != "" {
			collected[block] = struct{}{}
		}
	}

	results = append(results, linuxIdentityOnlyDisks(missed, collected, multiplexed, diskExclude)...)

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
	scanTargets, scanErr := listSMARTTargetsLinuxFromScanOpen(ctx, diskExclude)
	if scanErr != nil {
		log.Debug().
			Str("component", smartctlComponent).
			Err(scanErr).
			Msg("Failed to enumerate Linux SMART targets via smartctl --scan-open, relying on block device discovery")
	}

	// smartctl --scan-open silently omits any device it fails to open at scan
	// time (the failure is only a #-comment in its output), so the scan alone
	// can hide a real disk while listing its neighbours (#1483: a SATA SSD
	// missing while two NVMe controllers were reported). The kernel block
	// device list is the ground truth for which disks exist; scan-open only
	// contributes device-type hints. Union the two.
	devices, devErr := listBlockDevicesLinux(ctx, diskExclude)
	if devErr != nil {
		if len(scanTargets) > 0 {
			log.Debug().
				Str("component", smartctlComponent).
				Err(devErr).
				Msg("Block device discovery failed; using smartctl --scan-open targets only")
			return scanTargets, nil
		}
		if scanErr != nil {
			return nil, scanErr
		}
		return nil, devErr
	}

	return unionSMARTTargets(scanTargets, devices), nil
}

// unionSMARTTargets returns scanTargets plus an untyped target for every block
// device that no scan target covers. A scan target covers its own path's
// basename and, for an NVMe controller, the namespace it canonicalizes to.
func unionSMARTTargets(scanTargets []smartctlTarget, devices []string) []smartctlTarget {
	covered := make(map[string]struct{}, len(scanTargets)*2)
	for _, target := range scanTargets {
		if name := filepath.Base(strings.TrimSpace(target.Path)); name != "" && name != "." {
			covered[name] = struct{}{}
		}
		if block := canonicalBlockDeviceForScanPath(target.Path); block != "" {
			covered[block] = struct{}{}
		}
	}

	targets := append([]smartctlTarget(nil), scanTargets...)
	for _, device := range devices {
		name := filepath.Base(strings.TrimSpace(device))
		if name == "" || name == "." {
			continue
		}
		if _, ok := covered[name]; ok {
			continue
		}
		covered[name] = struct{}{}
		targets = append(targets, smartctlTarget{Path: device})
	}
	return targets
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
		if fsfilters.IsVirtualBlockDevice(name) {
			log.Debug().
				Str("component", smartctlComponent).
				Str("action", "skip_virtual_device").
				Str("device", path).
				Msg("Skipping non-physical device reported by smartctl --scan-open")
			continue
		}
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

// refineLinuxBlockDeviceIdentity rewrites a freshly collected SMART reading so
// that its device identity and size reflect the underlying block device rather
// than the smartctl scan target. smartctl --scan-open reports NVMe disks by their
// controller char device (/dev/nvme0), but the stable, user-visible identity is
// the namespace block device (/dev/nvme0n1) — the same name Proxmox's disks/list
// and /sys/block expose. It also backfills the capacity from /sys/block, the
// authoritative size source, so the agent no longer depends on a fragile
// filesystem-usage match on the server side.
func refineLinuxBlockDeviceIdentity(smart *DiskSMART, target smartctlTarget) {
	if smart == nil || runtimeGOOS != "linux" {
		return
	}
	block := canonicalBlockDeviceForScanPath(target.Path)
	if block == "" {
		return
	}
	// Disks addressed behind a multiplexing controller (megaraid,7; cciss,1;
	// areca,1/1; ...) all share a single /dev path, so the smartctl scan label is
	// the only thing that disambiguates them and /sys/block describes the array,
	// not the member. Leave those as-is and trust the smartctl-reported capacity.
	if isMultiplexedDeviceType(target.DeviceType) {
		smart.Controller = block
		smart.Target = strings.TrimSpace(target.DeviceType)
		ensureControllerCollectionStatus(smart, "smartctl_scan")
		return
	}
	smart.Device = block
	smart.Controller, smart.Target = linuxBlockDeviceTopology(block)
	ensureControllerCollectionStatus(smart, "sysfs")
	if size := blockDeviceSizeBytes(block); size > 0 {
		smart.SizeBytes = size
	}
}

func ensureControllerCollectionStatus(smart *DiskSMART, source string) {
	if smart.Collection == nil {
		smart.Collection = &diskinventory.CollectionStatus{}
	}
	if smart.Controller != "" || smart.Target != "" {
		smart.Collection.Controller = diskinventory.Available(source)
		return
	}
	smart.Collection.Controller = diskinventory.Missing(source, "controller association was not reported")
}

// linuxBlockDeviceTopology derives a stable controller association and SCSI
// target from the resolved /sys/block device path. The controller prefers the
// PCI address immediately preceding hostN; the target is the terminal H:C:T:L
// segment. Neither value is fabricated when sysfs does not expose it.
func linuxBlockDeviceTopology(block string) (string, string) {
	resolved, err := smartctlEvalSymlinks(filepath.Join("/sys/block", block, "device"))
	if err != nil {
		return "", ""
	}
	parts := strings.Split(filepath.Clean(resolved), string(filepath.Separator))
	controller := ""
	controllerFallback := ""
	target := ""
	for index, part := range parts {
		if pciControllerAddressRE.MatchString(part) {
			controller = part
		}
		if strings.HasPrefix(part, "host") && hasNumericSuffix(part, "host") && index > 0 {
			controllerFallback = parts[index-1]
		}
		if isSCSITargetAddress(part) {
			target = part
		}
	}
	if controller == "" {
		controller = controllerFallback
	}
	return controller, target
}

func isSCSITargetAddress(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" || !isAllDigits(part) {
			return false
		}
	}
	return true
}

func linuxBlockDeviceTransport(block string, target smartctlTarget) string {
	for _, candidate := range []string{
		readTrimmedFile(filepath.Join("/sys/block", block, "device", "protocol")),
		readTrimmedFile(filepath.Join("/sys/block", block, "device", "transport")),
	} {
		switch normalized := strings.ToLower(strings.TrimSpace(candidate)); {
		case strings.Contains(normalized, "nvme"):
			return "nvme"
		case strings.Contains(normalized, "sas"):
			return "sas"
		case strings.Contains(normalized, "sata"), strings.Contains(normalized, "ata"):
			return "sata"
		case strings.Contains(normalized, "usb"):
			return "usb"
		}
	}
	if strings.HasPrefix(block, "nvme") {
		return "nvme"
	}
	deviceType := strings.ToLower(strings.TrimSpace(target.DeviceType))
	switch {
	case strings.HasPrefix(deviceType, "nvme"):
		return "nvme"
	case strings.HasPrefix(deviceType, "sat"):
		return "sata"
	default:
		// Preserve the legacy direct sdX fallback when the kernel supplies no
		// transport evidence. Successful smartctl probes still override this
		// with their protocol field before this fallback is needed.
		return "sata"
	}
}

// linuxIdentityOnlyDisks builds identity-only entries for physical disks whose
// SMART probes produced nothing usable. A real disk that refuses SMART must
// still be listed — Proxmox's own disks/list shows it, and a monitoring view
// that silently hides a present disk reads as data loss (#1483: a SATA SSD
// vanished from the UI because its probe yielded no data). Each entry carries
// only the identity /sys/block can prove (name, capacity, model, serial) plus
// health UNKNOWN; no SMART data is fabricated. Multiplexed controller paths
// are skipped: their per-member typed targets describe the real disks, and the
// shared /dev path is the array, not a disk.
func linuxIdentityOnlyDisks(missed []smartctlTarget, collected, multiplexed map[string]struct{}, diskExclude []string) []DiskSMART {
	if runtimeGOOS != "linux" || len(missed) == 0 {
		return nil
	}

	var results []DiskSMART
	seen := make(map[string]struct{}, len(missed))
	for _, target := range missed {
		if isMultiplexedDeviceType(target.DeviceType) {
			continue
		}
		block := canonicalBlockDeviceForScanPath(target.Path)
		if block == "" {
			continue
		}
		if _, ok := collected[block]; ok {
			continue
		}
		if _, ok := multiplexed[block]; ok {
			continue
		}
		if _, ok := seen[block]; ok {
			continue
		}
		seen[block] = struct{}{}

		size := blockDeviceSizeBytes(block)
		if size <= 0 {
			// Zero capacity means no medium (card readers, empty bridges) or
			// no /sys/block entry at all; nothing real to report.
			continue
		}
		if matchesDeviceExclude(block, "/dev/"+block, diskExclude) {
			continue
		}

		serial := readTrimmedFile(filepath.Join("/sys/block", block, "device", "serial"))
		controller, controllerTarget := linuxBlockDeviceTopology(block)
		collection := &diskinventory.CollectionStatus{
			Temperature: diskinventory.Unavailable("smartctl", "SMART probe returned no usable temperature data"),
		}
		if serial != "" {
			collection.Serial = diskinventory.Available("sysfs")
		} else {
			collection.Serial = diskinventory.Missing("sysfs", "disk serial was not reported")
		}
		if controller != "" || controllerTarget != "" {
			collection.Controller = diskinventory.Available("sysfs")
		} else {
			collection.Controller = diskinventory.Missing("sysfs", "controller association was not reported")
		}
		results = append(results, DiskSMART{
			Device:      block,
			Model:       readTrimmedFile(filepath.Join("/sys/block", block, "device", "model")),
			Serial:      serial,
			Type:        linuxBlockDeviceTransport(block, target),
			Controller:  controller,
			Target:      controllerTarget,
			SizeBytes:   size,
			Health:      "UNKNOWN",
			Collection:  collection,
			LastUpdated: timeNow(),
		})
		log.Debug().
			Str("component", smartctlComponent).
			Str("action", "identity_only_disk").
			Str("device", block).
			Int64("sizeBytes", size).
			Msg("Reporting identity-only entry for physical disk without usable SMART data")
	}
	return results
}

// isMultiplexedDeviceType reports whether a smartctl -d type addresses a member
// disk behind a controller (e.g. "megaraid,7"), as opposed to a directly
// attached device ("", "sat", "nvme", "scsi", "sat,auto").
func isMultiplexedDeviceType(deviceType string) bool {
	idx := strings.IndexByte(deviceType, ',')
	if idx < 0 || idx+1 >= len(deviceType) {
		return false
	}
	next := deviceType[idx+1]
	return next >= '0' && next <= '9'
}

// canonicalBlockDeviceForScanPath maps a smartctl scan target to its canonical
// /sys/block device name. NVMe controllers (nvmeN) resolve to their first
// namespace (nvmeNnM); every other device keeps its basename.
func canonicalBlockDeviceForScanPath(scanPath string) string {
	name := path.Base(strings.TrimSpace(scanPath))
	if name == "" || name == "." || name == "/" {
		return ""
	}
	if isNVMeControllerName(name) {
		if ns := firstNVMeNamespace(name); ns != "" {
			return ns
		}
	}
	return name
}

// isNVMeControllerName reports whether name is an NVMe controller char device
// (e.g. "nvme0") rather than a namespace block device (e.g. "nvme0n1").
func isNVMeControllerName(name string) bool {
	return hasNumericSuffix(name, "nvme")
}

// firstNVMeNamespace returns the lowest-numbered namespace block device for an
// NVMe controller (e.g. "nvme0" -> "nvme0n1"), or "" when none is found.
func firstNVMeNamespace(controller string) string {
	entries, err := readDir("/sys/block")
	if err != nil {
		return ""
	}
	prefix := controller + "n"
	best := ""
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name())
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		// Require a pure namespace (nvme0n1), not a partition (nvme0n1p1).
		if suffix := name[len(prefix):]; suffix == "" || !isAllDigits(suffix) {
			continue
		}
		if best == "" || name < best {
			best = name
		}
	}
	return best
}

// blockDeviceSizeBytes reads /sys/block/<name>/size, which the kernel always
// reports in 512-byte sectors regardless of the physical block size.
func blockDeviceSizeBytes(name string) int64 {
	if name == "" {
		return 0
	}
	data, err := smartctlReadFile(filepath.Join("/sys/block", name, "size"))
	if err != nil {
		return 0
	}
	sectors, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil || sectors <= 0 {
		return 0
	}
	return sectors * 512
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
						Device:  filepath.Base(target.Path),
						Standby: true,
						Collection: &diskinventory.CollectionStatus{
							Serial:      diskinventory.Unavailable("smartctl", "disk is in standby"),
							Temperature: diskinventory.Unavailable("smartctl", "disk is in standby"),
						},
						LastUpdated: timeNow(),
					}
					if runtimeGOOS == "freebsd" && i < len(attempts)-1 && target.DeviceType == "" {
						if firstStandby == nil {
							firstStandby = standbyResult
						}
						continue
					}
					// -n standby,3 makes a real standby exit 3; exit bit 2 with
					// no output is an open/identify failure, not standby. When
					// another attempt remains (Linux untyped retry), try it
					// before reporting a phantom standby disk.
					if runtimeGOOS == "linux" && i < len(attempts)-1 && exitCode != smartctlStandbyExitStatus {
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
		if !shouldRetrySMARTTarget(target.Path, result, i, len(attempts)) {
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
		deviceTypes := []string{target.DeviceType}
		// A scan-open device type is a hint, not ground truth: smartctl can
		// suggest a type whose full query (-i -A -H) fails or returns no usable
		// data even though untyped auto-detection works (#1483: a SATA SSD
		// dropped after its typed probe yielded nothing). Retry untyped before
		// giving up. Multiplexed controller members are exempt because dropping
		// the -d would re-probe the shared array device, not the member.
		if runtimeGOOS == "linux" && !isMultiplexedDeviceType(target.DeviceType) {
			deviceTypes = append(deviceTypes, "")
			deviceTypes = append(deviceTypes, linuxInferredSmartctlDeviceTypes(device)...)
		}
		return smartctlArgsForDeviceTypes(device, deviceTypes)
	}

	if runtimeGOOS == "linux" {
		deviceTypes := append([]string{""}, linuxInferredSmartctlDeviceTypes(device)...)
		return smartctlArgsForDeviceTypes(device, deviceTypes)
	}

	if runtimeGOOS == "freebsd" {
		deviceTypes := freeBSDSmartctlDeviceTypes(filepath.Base(device))
		if len(deviceTypes) > 0 {
			return smartctlArgsForDeviceTypes(device, append(deviceTypes, ""))
		}
	}

	return [][]string{
		smartctlArgs(device, ""),
	}
}

func smartctlArgsForDeviceTypes(device string, deviceTypes []string) [][]string {
	attempts := make([][]string, 0, len(deviceTypes))
	seen := make(map[string]struct{}, len(deviceTypes))
	for _, deviceType := range deviceTypes {
		deviceType = strings.TrimSpace(deviceType)
		if _, ok := seen[deviceType]; ok {
			continue
		}
		seen[deviceType] = struct{}{}
		attempts = append(attempts, smartctlArgs(device, deviceType))
	}
	return attempts
}

func linuxInferredSmartctlDeviceTypes(device string) []string {
	if runtimeGOOS != "linux" {
		return nil
	}
	name := strings.ToLower(filepath.Base(strings.TrimSpace(device)))
	switch {
	case linuxDirectSATDeviceRE.MatchString(name):
		return []string{"sat", "scsi"}
	default:
		return nil
	}
}

func smartctlArgs(device, deviceType string) []string {
	args := []string{}
	if deviceType != "" {
		args = append(args, "-d", deviceType)
	}
	// The standby guard exists to avoid spinning up sleeping rotational disks.
	// An SSD has nothing to spin up, and some SATA SSDs answer the guard's
	// CHECK POWER MODE with a bogus standby state that permanently hides their
	// SMART data (#1516), so confirmed non-rotational devices are probed
	// without it. Multiplexed controller members keep the guard: the shared
	// /dev path's rotational flag describes the array device, not the member.
	if isMultiplexedDeviceType(deviceType) || !linuxNonRotationalBlockDevice(device) {
		args = append(args, "-n", "standby,"+strconv.Itoa(smartctlStandbyExitStatus))
	}
	args = append(args, "-i", "-A", "-H", "--json=o", device)
	return args
}

// linuxNonRotationalBlockDevice reports whether the canonical block device
// behind path is positively confirmed non-rotational (SSD) via sysfs. Any
// uncertainty — non-Linux, unresolvable device, unreadable sysfs — returns
// false so the caller keeps the conservative standby guard.
func linuxNonRotationalBlockDevice(device string) bool {
	if runtimeGOOS != "linux" {
		return false
	}
	block := canonicalBlockDeviceForScanPath(device)
	if block == "" {
		return false
	}
	return readTrimmedFile(path.Join("/sys/block", block, "queue", "rotational")) == "0"
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

func shouldRetrySMARTTarget(device string, result *DiskSMART, attemptIndex, attemptCount int) bool {
	if attemptIndex >= attemptCount-1 || result == nil {
		return false
	}
	if result.Temperature > 0 {
		return false
	}
	if result.Standby {
		return true
	}
	switch runtimeGOOS {
	case "freebsd":
		return len(freeBSDSmartctlDeviceTypes(filepath.Base(device))) > 0
	case "linux":
		return true
	default:
		return false
	}
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
		Collection:  &diskinventory.CollectionStatus{},
	}

	if smartData.WWN.NAA != 0 {
		result.WWN = formatWWN(smartData.WWN.NAA, smartData.WWN.OUI, smartData.WWN.ID)
	}

	// Capacity straight from the device smartctl just queried. On Linux this is
	// refined to the authoritative /sys/block value in CollectSMARTLocal; here it
	// is the cross-platform fallback so non-Linux hosts still report a size.
	if smartData.NVMeTotalCapacity > 0 {
		result.SizeBytes = smartData.NVMeTotalCapacity
	} else if smartData.UserCapacity.Bytes > 0 {
		result.SizeBytes = smartData.UserCapacity.Bytes
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

	applySMARTTextFallback(result, parseSMARTTextFallback(strings.Join(smartData.Smartctl.Output, "\n")))
	if result.Serial != "" {
		result.Collection.Serial = diskinventory.Available("smartctl")
	} else {
		result.Collection.Serial = diskinventory.Missing("smartctl", "disk serial was not reported")
	}
	switch {
	case result.Temperature > 0:
		result.Collection.Temperature = diskinventory.Available("smartctl")
	case result.Standby:
		result.Collection.Temperature = diskinventory.Unavailable("smartctl", "disk is in standby")
	default:
		result.Collection.Temperature = diskinventory.Unsupported("smartctl", "device did not expose a temperature reading")
	}
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
		Collection:  &diskinventory.CollectionStatus{},
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
	if result.Serial != "" {
		result.Collection.Serial = diskinventory.Available("smartctl_text")
	} else {
		result.Collection.Serial = diskinventory.Missing("smartctl_text", "disk serial was not reported")
	}
	switch {
	case result.Temperature > 0:
		result.Collection.Temperature = diskinventory.Available("smartctl_text")
	case result.Standby:
		result.Collection.Temperature = diskinventory.Unavailable("smartctl_text", "disk is in standby")
	default:
		result.Collection.Temperature = diskinventory.Unsupported("smartctl_text", "device did not expose a temperature reading")
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
