package hostagent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Pre-compiled regexes for performance (avoid recompilation on each call)
var (
	mdDeviceRe    = regexp.MustCompile(`^\s*(md\d+)\s*:`)
	mdArrayPathRe = regexp.MustCompile(`^/dev/md\d+$`)
	mdCountRe     = regexp.MustCompile(`\[(\d+)/(\d+)\]\s+\[([U_]+)\]`)
	mdTokenRe     = regexp.MustCompile(`^([A-Za-z0-9_.!/-]+)\[(\d+)\](.*)$`)
	progressRe    = regexp.MustCompile(`=\s*([0-9]+(?:\.[0-9]+)?)%`)
	slotRe        = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.+?)\s+(/dev/.+)$`)
	speedRe       = regexp.MustCompile(`speed=(\S+)`)
	// /proc/mdstat progress line action.
	// Example: "[==>..................]  check = 12.6% (...) finish=... speed=..."
	mdstatOperationRe = regexp.MustCompile(`(?i)\b(recovery|resync|check|reshape)\b\s*=`)
	mdadmLookPath     = exec.LookPath
	mdadmStat         = os.Stat
	openFileForRead   = func(name string) (io.ReadCloser, error) { return os.Open(name) }
	readFile          = func(name string) ([]byte, error) {
		file, err := openFileForRead(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		// Enforce the size cap while reading to avoid allocating arbitrarily
		// large buffers before validation.
		return io.ReadAll(io.LimitReader(file, maxMDStatBytes+1))
	}
	resolveMdadmBinary    = resolveMdadmPath
	mdadmRunCommandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.Output()
	}
)

const (
	mdstatPath     = "/proc/mdstat"
	maxMDStatBytes = 1 << 20 // 1 MiB hard limit for /proc/mdstat reads.
)

// commonMdadmPaths lists common mdadm locations to avoid relying on PATH order.
var commonMdadmPaths = []string{
	"/usr/sbin/mdadm",
	"/sbin/mdadm",
	"/usr/local/sbin/mdadm",
	"/usr/bin/mdadm",
	"/bin/mdadm",
}

// CollectArrays discovers and collects status for all mdadm RAID arrays on the system.
// Returns an empty slice when no arrays are found. If mdadm is unavailable,
// /proc/mdstat still provides a baseline topology report.
func CollectRAIDArrays(ctx context.Context) ([]agentshost.RAIDArray, error) {
	mdstatArrays, err := listMDStatArrays(ctx)
	if err != nil {
		return nil, fmt.Errorf("list array devices: %w", err)
	}

	if len(mdstatArrays) == 0 {
		return nil, nil
	}

	// /proc/mdstat is the canonical discovery source. mdadm enriches that
	// baseline when available, but a detail probe failure must not hide a
	// kernel-reported array from Pulse.
	if !isMdadmAvailable(ctx) {
		return mdstatArrays, nil
	}

	// Collect detailed info for each array
	arrays := make([]agentshost.RAIDArray, 0, len(mdstatArrays))
	for _, fallback := range mdstatArrays {
		array, err := collectArrayDetail(ctx, fallback.Device)
		if err != nil {
			arrays = append(arrays, fallback)
			continue
		}
		arrays = append(arrays, mergeMDStatFallback(array, fallback))
	}

	return arrays, nil
}

// isMdadmAvailable checks if mdadm binary is accessible
func isMdadmAvailable(ctx context.Context) bool {
	mdadmPath, err := resolveMdadmBinary()
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = mdadmRunCommandOutput(ctx, mdadmPath, "--version")
	return err == nil
}

// listArrayDevices scans /proc/mdstat to find all md devices
func listArrayDevices(_ context.Context) ([]string, error) {
	arrays, err := listMDStatArrays(context.Background())
	if err != nil {
		return nil, err
	}

	devices := make([]string, 0, len(arrays))
	for _, array := range arrays {
		devices = append(devices, array.Device)
	}

	return devices, nil
}

// listMDStatArrays scans /proc/mdstat and returns a baseline array report for
// every md device the kernel currently exposes.
func listMDStatArrays(_ context.Context) ([]agentshost.RAIDArray, error) {
	output, err := readProcMDStat()
	if err != nil {
		return nil, err
	}
	return parseMDStatArrays(string(output)), nil
}

func parseMDStatArrays(output string) []agentshost.RAIDArray {
	lines := strings.Split(output, "\n")
	arrays := make([]agentshost.RAIDArray, 0)

	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		matches := mdDeviceRe.FindStringSubmatch(line)
		if len(matches) <= 1 {
			i++
			continue
		}

		section := []string{line}
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if next == "" || strings.HasPrefix(next, "Personalities") || strings.HasPrefix(next, "unused devices") {
				break
			}
			if mdDeviceRe.MatchString(next) {
				break
			}
			section = append(section, next)
			i++
		}

		arrays = append(arrays, parseMDStatArraySection(matches[1], section))
	}

	return arrays
}

func parseMDStatArraySection(deviceName string, section []string) agentshost.RAIDArray {
	array := agentshost.RAIDArray{
		Device:  "/dev/" + deviceName,
		Devices: []agentshost.RAIDDevice{},
	}
	if len(section) == 0 {
		return array
	}

	header := section[0]
	_, fieldsRaw, ok := strings.Cut(header, ":")
	if !ok {
		return array
	}

	fields := strings.Fields(fieldsRaw)
	if len(fields) > 0 {
		array.State = strings.ToLower(fields[0])
	}

	levelIndex := -1
	for i, field := range fields {
		if isMDStatLevelToken(field) {
			levelIndex = i
			array.Level = strings.ToLower(field)
			break
		}
	}

	deviceStart := 1
	if levelIndex >= 0 {
		deviceStart = levelIndex + 1
	}
	for _, field := range fields[deviceStart:] {
		device, ok := parseMDStatDeviceToken(field, array.State)
		if ok {
			array.Devices = append(array.Devices, device)
		}
	}

	for _, line := range section[1:] {
		if matches := mdCountRe.FindStringSubmatch(line); len(matches) > 3 {
			total, _ := strconv.Atoi(matches[1])
			active, _ := strconv.Atoi(matches[2])
			array.TotalDevices = total
			array.ActiveDevices = active
			if total > active && !strings.Contains(array.State, "degraded") {
				array.State = appendMDStatState(array.State, "degraded")
			}
		}

		if array.Operation == "" {
			if matches := mdstatOperationRe.FindStringSubmatch(line); len(matches) > 1 {
				array.Operation = strings.ToLower(matches[1])
			}
		}
		if array.RebuildSpeed == "" {
			if matches := speedRe.FindStringSubmatch(line); len(matches) > 1 {
				array.RebuildSpeed = matches[1]
			}
		}
		if array.RebuildPercent == 0 {
			if matches := progressRe.FindStringSubmatch(line); len(matches) > 1 {
				array.RebuildPercent, _ = strconv.ParseFloat(matches[1], 64)
			}
		}
	}

	var workingDevices, failedDevices, spareDevices int
	for _, device := range array.Devices {
		state := strings.ToLower(device.State)
		switch {
		case strings.Contains(state, "faulty"):
			failedDevices++
		case strings.Contains(state, "spare"):
			spareDevices++
			workingDevices++
		default:
			workingDevices++
		}
	}
	if array.ActiveDevices == 0 {
		array.ActiveDevices = workingDevices - spareDevices
	}
	if array.TotalDevices == 0 {
		array.TotalDevices = array.ActiveDevices
	}
	if failedDevices == 0 && array.TotalDevices > array.ActiveDevices {
		failedDevices = array.TotalDevices - array.ActiveDevices
	}
	array.WorkingDevices = workingDevices
	array.FailedDevices = failedDevices
	array.SpareDevices = spareDevices

	return array
}

func parseMDStatDeviceToken(token string, arrayState string) (agentshost.RAIDDevice, bool) {
	token = strings.Trim(token, ",")
	matches := mdTokenRe.FindStringSubmatch(token)
	if len(matches) < 4 {
		return agentshost.RAIDDevice{}, false
	}

	slot, err := strconv.Atoi(matches[2])
	if err != nil {
		return agentshost.RAIDDevice{}, false
	}

	state := "active sync"
	flags := strings.ToUpper(matches[3])
	switch {
	case strings.Contains(flags, "F"):
		state = "faulty"
	case strings.Contains(flags, "S"):
		state = "spare"
	case strings.EqualFold(arrayState, "inactive"):
		state = "inactive"
	}

	device := strings.TrimSpace(matches[1])
	if !strings.HasPrefix(device, "/dev/") {
		device = "/dev/" + device
	}

	return agentshost.RAIDDevice{
		Device: device,
		State:  state,
		Slot:   slot,
	}, true
}

func isMDStatLevelToken(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	return strings.HasPrefix(token, "raid") || token == "linear" || token == "multipath"
}

func appendMDStatState(state, suffix string) string {
	state = strings.TrimSpace(state)
	if state == "" {
		return suffix
	}
	return state + ", " + suffix
}

func mergeMDStatFallback(array, fallback agentshost.RAIDArray) agentshost.RAIDArray {
	if strings.TrimSpace(array.Device) == "" {
		array.Device = fallback.Device
	}
	if strings.TrimSpace(array.Level) == "" {
		array.Level = fallback.Level
	}
	if strings.TrimSpace(array.State) == "" {
		array.State = fallback.State
	}
	if array.TotalDevices == 0 {
		array.TotalDevices = fallback.TotalDevices
	}
	if array.ActiveDevices == 0 {
		array.ActiveDevices = fallback.ActiveDevices
	}
	if array.WorkingDevices == 0 {
		array.WorkingDevices = fallback.WorkingDevices
	}
	if array.FailedDevices == 0 {
		array.FailedDevices = fallback.FailedDevices
	}
	if array.SpareDevices == 0 {
		array.SpareDevices = fallback.SpareDevices
	}
	if len(array.Devices) == 0 {
		array.Devices = fallback.Devices
	}
	if array.RebuildPercent == 0 {
		array.RebuildPercent = fallback.RebuildPercent
	}
	if strings.TrimSpace(array.RebuildSpeed) == "" {
		array.RebuildSpeed = fallback.RebuildSpeed
	}
	if strings.TrimSpace(array.Operation) == "" {
		array.Operation = fallback.Operation
	}
	return array
}

// collectArrayDetail runs mdadm --detail on a specific device and parses the output
func collectArrayDetail(ctx context.Context, device string) (host.RAIDArray, error) {
	if !mdArrayPathRe.MatchString(device) {
		return host.RAIDArray{}, fmt.Errorf("invalid md device path %q", device)
	}

	mdadmPath, err := resolveMdadmBinary()
	if err != nil {
		return host.RAIDArray{}, fmt.Errorf("resolve mdadm binary: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := mdadmRunCommandOutput(ctx, mdadmPath, "--detail", device)
	if err != nil {
		return agentshost.RAIDArray{}, fmt.Errorf("mdadm --detail %s: %w", device, err)
	}

	return parseMdadmDetail(device, string(output))
}

// parseMdadmDetail parses the output of mdadm --detail
func parseMdadmDetail(device, output string) (agentshost.RAIDArray, error) {
	array := agentshost.RAIDArray{
		Device:  device,
		Devices: []agentshost.RAIDDevice{},
	}

	lines := strings.Split(output, "\n")
	inDeviceSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check if we're entering the device list section
		if strings.Contains(line, "Number") && strings.Contains(line, "Major") && strings.Contains(line, "Minor") {
			inDeviceSection = true
			continue
		}

		// Parse device entries
		if inDeviceSection {
			matches := slotRe.FindStringSubmatch(line)
			if len(matches) >= 7 {
				slot, err := strconv.Atoi(matches[1])
				if err != nil {
					return host.RAIDArray{}, fmt.Errorf("parse slot for %s from %q: %w", device, matches[1], err)
				}
				state := strings.TrimSpace(matches[5])
				devicePath := strings.TrimSpace(matches[6])

				array.Devices = append(array.Devices, agentshost.RAIDDevice{
					Device: devicePath,
					State:  state,
					Slot:   slot,
				})
				continue
			}

			// Handle spare/faulty devices (different format)
			if strings.Contains(line, "spare") || strings.Contains(line, "faulty") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					state := "spare"
					if strings.Contains(line, "faulty") {
						state = "faulty"
					}
					devicePath := parts[len(parts)-1]

					array.Devices = append(array.Devices, agentshost.RAIDDevice{
						Device: devicePath,
						State:  state,
						Slot:   -1,
					})
				}
			}
			continue
		}

		// Parse key-value pairs
		if strings.Contains(line, ":") {
			// SplitN with n=2 always returns 2 elements when ":" exists
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Name":
				array.Name = value
			case "Raid Level":
				array.Level = strings.ToLower(value)
			case "State":
				array.State = strings.ToLower(value)
			case "Total Devices":
				totalDevices, err := parseIntField(device, key, value)
				if err != nil {
					return host.RAIDArray{}, err
				}
				array.TotalDevices = totalDevices
			case "Active Devices":
				activeDevices, err := parseIntField(device, key, value)
				if err != nil {
					return host.RAIDArray{}, err
				}
				array.ActiveDevices = activeDevices
			case "Working Devices":
				workingDevices, err := parseIntField(device, key, value)
				if err != nil {
					return host.RAIDArray{}, err
				}
				array.WorkingDevices = workingDevices
			case "Failed Devices":
				failedDevices, err := parseIntField(device, key, value)
				if err != nil {
					return host.RAIDArray{}, err
				}
				array.FailedDevices = failedDevices
			case "Spare Devices":
				spareDevices, err := parseIntField(device, key, value)
				if err != nil {
					return host.RAIDArray{}, err
				}
				array.SpareDevices = spareDevices
			case "UUID":
				array.UUID = value
			case "Rebuild Status", "Reshape Status":
				array.RebuildPercent = parsePercentValue(value)
			}
		}
	}

	// /proc/mdstat is the authoritative source for distinguishing a true
	// rebuild from routine maintenance such as a scrub on kernels that do not
	// surface scrub state in mdadm --detail.
	operation, speed := getMdstatProgress(device)
	if operation != "" {
		array.Operation = operation
	}
	if speed != "" {
		array.RebuildSpeed = speed
	}

	return array, nil
}

// parsePercentValue parses a percentage string like "50% complete" and returns the numeric value.
func parsePercentValue(value string) float64 {
	if strings.Contains(value, "%") {
		percentStr := strings.TrimSpace(strings.Split(value, "%")[0])
		result, _ := strconv.ParseFloat(percentStr, 64)
		return result
	}
	return 0
}

// parseIntField parses an integer field from mdadm --detail output.
func parseIntField(device, key, value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("parse %s for %s from %q: %w", key, device, value, err)
	}
	return n, nil
}

// getMdstatProgress extracts the in-progress md operation and speed from
// /proc/mdstat for the named array. It returns empty strings when the array is
// idle, absent, or the progress line is unavailable.
func getMdstatProgress(device string) (operation, speed string) {
	// Remove /dev/ prefix for /proc/mdstat lookup
	deviceName := strings.TrimPrefix(device, "/dev/")

	output, err := readProcMDStat()
	if err != nil {
		return "", ""
	}

	// Example progress line shapes:
	//   [==>..................]  recovery = 12.6% (...) finish=... speed=33440K/sec
	//   [=>...................]  resync = 5.0% (...) finish=... speed=...
	//   [==>..................]  check = 12.6% (...) finish=... speed=...
	lines := strings.Split(string(output), "\n")
	inSection := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), deviceName) {
			inSection = true
			continue
		}

		if !inSection {
			continue
		}

		if strings.TrimSpace(line) == "" || (strings.HasPrefix(strings.TrimSpace(line), "md") && strings.Contains(line, ":")) {
			break
		}

		if operation == "" {
			if matches := mdstatOperationRe.FindStringSubmatch(line); len(matches) > 1 {
				operation = strings.ToLower(matches[1])
			}
		}
		if speed == "" {
			if matches := speedRe.FindStringSubmatch(line); len(matches) > 1 {
				speed = matches[1]
			}
		}
		if operation != "" && speed != "" {
			break
		}
	}

	return operation, speed
}

// readProcMDStat reads /proc/mdstat directly (without shelling out)
// and bounds input size to avoid unbounded memory growth.
var readProcMDStat = func() ([]byte, error) {
	output, err := readFile(mdstatPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", mdstatPath, err)
	}
	if len(output) > maxMDStatBytes {
		return nil, fmt.Errorf("%s exceeds max size (%d bytes)", mdstatPath, maxMDStatBytes)
	}
	return output, nil
}

// resolveMdadmPath resolves a trusted mdadm executable path.
// It prefers known absolute paths before falling back to PATH lookup.
func resolveMdadmPath() (string, error) {
	for _, candidate := range commonMdadmPaths {
		candidate = filepath.Clean(candidate)
		if !filepath.IsAbs(candidate) {
			continue
		}
		if _, err := mdadmStat(candidate); err == nil {
			return candidate, nil
		}
	}

	path, err := mdadmLookPath("mdadm")
	if err != nil {
		return "", fmt.Errorf("mdadm binary not found in PATH or common locations")
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("mdadm path is not absolute: %q", path)
	}
	if _, err := mdadmStat(path); err != nil {
		return "", fmt.Errorf("mdadm path unavailable: %w", err)
	}

	return path, nil
}
