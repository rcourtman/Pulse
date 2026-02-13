package hostagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Pre-compiled regexes for performance (avoid recompilation on each call)
var (
	mdDeviceRe      = regexp.MustCompile(`^(md\d+)\s*:`)
	mdArrayPathRe   = regexp.MustCompile(`^/dev/md\d+$`)
	slotRe          = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.+?)\s+(/dev/.+)$`)
	speedRe         = regexp.MustCompile(`speed=(\S+)`)
	mdadmLookPath   = exec.LookPath
	mdadmStat       = os.Stat
	openFileForRead = func(name string) (io.ReadCloser, error) { return os.Open(name) }
	readFile        = func(name string) ([]byte, error) {
		file, err := openFileForRead(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		// Enforce the size cap while reading to avoid allocating arbitrarily
		// large buffers before validation.
		return io.ReadAll(io.LimitReader(file, maxMDStatBytes+1))
	}
	resolveMdadmBinary = resolveMdadmPath
	runCommandOutput   = func(ctx context.Context, name string, args ...string) ([]byte, error) {
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
// Returns an empty slice if mdadm is not available or no arrays are found.
func CollectRAIDArrays(ctx context.Context) ([]agentshost.RAIDArray, error) {
	// Check if mdadm is available
	if !isMdadmAvailable(ctx) {
		return nil, nil
	}

	// Get list of arrays from /proc/mdstat
	devices, err := listArrayDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list array devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	// Collect detailed info for each array
	var arrays []agentshost.RAIDArray
	for _, device := range devices {
		array, err := collectArrayDetail(ctx, device)
		if err != nil {
			detailErrs = append(detailErrs, fmt.Errorf("collect detail for %s: %w", device, err))
			continue
		}
		arrays = append(arrays, array)
	}

	// Keep best-effort behavior for partial success, but surface failures when all detail probes fail.
	if len(arrays) == 0 && len(detailErrs) > 0 {
		return nil, fmt.Errorf("collect mdadm array details: %w", errors.Join(detailErrs...))
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

	_, err = runCommandOutput(ctx, mdadmPath, "--version")
	return err == nil
}

// listArrayDevices scans /proc/mdstat to find all md devices
func listArrayDevices(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	output, err := mdadmCommandRunner(ctx, "cat", "/proc/mdstat")
	if err != nil {
		return nil, err
	}

	// Parse /proc/mdstat to find device names
	// Lines like: md0 : active raid1 sdb1[1] sda1[0]
	var devices []string
	for _, line := range strings.Split(string(output), "\n") {
		matches := mdDeviceRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			devices = append(devices, "/dev/"+matches[1])
		}
	}

	return devices, nil
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

	output, err := runCommandOutput(ctx, mdadmPath, "--detail", device)
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

	// Check for rebuild/resync info in /proc/mdstat for speed information
	if array.RebuildPercent > 0 {
		speed := getRebuildSpeed(device)
		if speed != "" {
			array.RebuildSpeed = speed
		}
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

// getRebuildSpeed extracts rebuild speed from /proc/mdstat
func getRebuildSpeed(device string) string {
	// Remove /dev/ prefix for /proc/mdstat lookup
	deviceName := strings.TrimPrefix(device, "/dev/")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	output, err := mdadmCommandRunner(ctx, "cat", "/proc/mdstat")
	if err != nil {
		return ""
	}

	// Look for lines containing rebuild/resync speed
	// Example: [==>..................]  recovery = 12.6% (37043392/293039104) finish=127.5min speed=33440K/sec
	lines := strings.Split(string(output), "\n")
	inSection := false

	for _, line := range lines {
		// Check if this is our device
		if strings.HasPrefix(strings.TrimSpace(line), deviceName) {
			inSection = true
			continue
		}

		// If we're in the right section, look for speed info
		if inSection {
			if strings.Contains(line, "speed=") {
				// Extract speed value using pre-compiled regex
				matches := speedRe.FindStringSubmatch(line)
				if len(matches) > 1 {
					return matches[1]
				}
			}

			// Exit section when we hit a new device or blank line
			if strings.TrimSpace(line) == "" || (strings.HasPrefix(strings.TrimSpace(line), "md") && strings.Contains(line, ":")) {
				break
			}
		}
	}

	return ""
}

// readProcMDStat reads /proc/mdstat directly (without shelling out)
// and bounds input size to avoid unbounded memory growth.
func readProcMDStat() ([]byte, error) {
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
