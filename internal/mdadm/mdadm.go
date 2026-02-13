package mdadm

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
)

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Pre-compiled regexes for performance (avoid recompilation on each call)
var (
	mdDeviceRe = regexp.MustCompile(`^(md\d+)\s*:`)
	slotRe     = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.+?)\s+(/dev/.+)$`)
	speedRe    = regexp.MustCompile(`speed=(\S+)`)
)

// CollectArrays discovers and collects status for all mdadm RAID arrays on the system.
// Returns an empty slice if mdadm is not available or no arrays are found.
func CollectArrays(ctx context.Context) ([]host.RAIDArray, error) {
	return collectArrays(ctx, defaultRunCommandOutput)
}

func collectArrays(ctx context.Context, run commandRunner) ([]host.RAIDArray, error) {
	// Check if mdadm is available
	if !isMdadmAvailableWithRunner(ctx, run) {
		return nil, nil
	}

	// Get list of arrays from /proc/mdstat
	devices, err := listArrayDevicesWithRunner(ctx, run)
	if err != nil {
		return nil, fmt.Errorf("list array devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	// Collect detailed info for each array
	var arrays []host.RAIDArray
	for _, device := range devices {
		array, err := collectArrayDetailWithRunner(ctx, device, run)
		if err != nil {
			log.Warn().
				Err(err).
				Str("component", "mdadm").
				Str("device", device).
				Msg("Skipping RAID array after detail collection failure")
			continue
		}
		arrays = append(arrays, array)
	}

	return arrays, nil
}

// isMdadmAvailable checks if mdadm binary is accessible
func isMdadmAvailable(ctx context.Context) bool {
	return isMdadmAvailableWithRunner(ctx, defaultRunCommandOutput)
}

func isMdadmAvailableWithRunner(ctx context.Context, run commandRunner) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := run(ctx, "mdadm", "--version")
	return err == nil
}

// listArrayDevices scans /proc/mdstat to find all md devices
func listArrayDevices(ctx context.Context) ([]string, error) {
	return listArrayDevicesWithRunner(ctx, defaultRunCommandOutput)
}

func listArrayDevicesWithRunner(ctx context.Context, run commandRunner) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	output, err := run(ctx, "cat", "/proc/mdstat")
	if err != nil {
		return nil, fmt.Errorf("read /proc/mdstat: %w", err)
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
	return collectArrayDetailWithRunner(ctx, device, defaultRunCommandOutput)
}

func collectArrayDetailWithRunner(ctx context.Context, device string, run commandRunner) (host.RAIDArray, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := run(ctx, "mdadm", "--detail", device)
	if err != nil {
		return host.RAIDArray{}, fmt.Errorf("mdadm --detail %s: %w", device, err)
	}

	return parseDetailWithContext(ctx, device, string(output))
}

// parseDetail parses the output of mdadm --detail
func parseDetail(device, output string) (host.RAIDArray, error) {
	return parseDetailWithContext(context.Background(), device, output)
}

func parseDetailWithContext(ctx context.Context, device, output string) (host.RAIDArray, error) {
	array := host.RAIDArray{
		Device:  device,
		Devices: []host.RAIDDevice{},
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
				slot, _ := strconv.Atoi(matches[1])
				state := strings.TrimSpace(matches[5])
				devicePath := strings.TrimSpace(matches[6])

				array.Devices = append(array.Devices, host.RAIDDevice{
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

					array.Devices = append(array.Devices, host.RAIDDevice{
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
				array.TotalDevices, _ = strconv.Atoi(value)
			case "Active Devices":
				array.ActiveDevices, _ = strconv.Atoi(value)
			case "Working Devices":
				array.WorkingDevices, _ = strconv.Atoi(value)
			case "Failed Devices":
				array.FailedDevices, _ = strconv.Atoi(value)
			case "Spare Devices":
				array.SpareDevices, _ = strconv.Atoi(value)
			case "UUID":
				array.UUID = value
			case "Rebuild Status":
				// Parse rebuild percentage
				// Format: "50% complete"
				if strings.Contains(value, "%") {
					percentStr := strings.TrimSpace(strings.Split(value, "%")[0])
					array.RebuildPercent, _ = strconv.ParseFloat(percentStr, 64)
				}
			case "Reshape Status":
				// Handle reshape similarly to rebuild
				if strings.Contains(value, "%") {
					percentStr := strings.TrimSpace(strings.Split(value, "%")[0])
					array.RebuildPercent, _ = strconv.ParseFloat(percentStr, 64)
				}
			}
		}
	}

	// Check for rebuild/resync info in /proc/mdstat for speed information
	if array.RebuildPercent > 0 {
		speed := getRebuildSpeedWithContext(ctx, device)
		if speed != "" {
			array.RebuildSpeed = speed
		}
	}

	return array, nil
}

// getRebuildSpeed extracts rebuild speed from /proc/mdstat
func getRebuildSpeed(device string) string {
	return getRebuildSpeedWithContext(context.Background(), device)
}

func getRebuildSpeedWithContext(parentCtx context.Context, device string) string {
	// Remove /dev/ prefix for /proc/mdstat lookup
	deviceName := strings.TrimPrefix(device, "/dev/")

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Second)
	defer cancel()

	output, err := run(ctx, "cat", "/proc/mdstat")
	if err != nil {
		log.Debug().
			Err(err).
			Str("component", "mdadm").
			Str("device", device).
			Msg("Failed to read /proc/mdstat for rebuild speed lookup")
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

func defaultRunCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
