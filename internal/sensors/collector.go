package sensors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	maxSensorsOutputSizeBytes = 1 << 20
	maxThermalFileReadBytes   = 64
)

var (
	errCommandOutputTooLarge = errors.New("command output exceeds size limit")
	rpiThermalZonePath       = "/sys/class/thermal/thermal_zone0/temp"
)

var rpiThermalZoneTempPath = "/sys/class/thermal/thermal_zone0/temp"

// CollectLocal reads sensor data from the local machine using lm-sensors.
// Returns the raw JSON output from `sensors -j` or an error if sensors is not available.
func CollectLocal(ctx context.Context) (string, error) {
	ctx = normalizeCollectionContext(ctx)

	// Check if sensors command exists
	sensorsPath, err := exec.LookPath("sensors")
	if err != nil {
		return "", fmt.Errorf("lm-sensors not installed: %w", err)
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Run sensors -j command with bounded output capture.
	// sensors exits non-zero when optional subfeatures fail, so non-empty output is still accepted.
	cmd := exec.CommandContext(cmdCtx, sensorsPath, "-j")
	cmd.Stderr = io.Discard
	output, err := runCommandOutputLimited(cmd, maxSensorsOutputSizeBytes)
	if err != nil && errors.Is(err, errCommandOutputTooLarge) {
		return "", fmt.Errorf("failed to execute sensors: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if err != nil && outputStr == "" {
		return "", fmt.Errorf("failed to execute sensors: %w", err)
	}

	if outputStr == "" || outputStr == "{}" {
		log.Debug().
			Str("component", "sensors_collector").
			Str("action", "collect_local_empty_output").
			Msg("lm-sensors returned empty output, attempting Raspberry Pi thermal fallback")

		// Try Raspberry Pi temperature method as fallback
		cmd = exec.CommandContext(cmdCtx, "cat", rpiThermalZonePath)
		rpiOutput, rpiErr := cmd.Output()
		if rpiErr == nil {
			rpiTemp := strings.TrimSpace(string(rpiOutput))
			if rpiTemp != "" {
				parsed, parseErr := strconv.ParseFloat(rpiTemp, 64)
				if parseErr != nil {
					return "", fmt.Errorf("invalid thermal value %q: %w", rpiTemp, parseErr)
				}
				// Linux thermal_zone values are commonly millidegrees (e.g. 42000).
				// Convert only when magnitude indicates millidegrees to keep degree inputs intact.
				if parsed >= 1000 || parsed <= -1000 {
					parsed = parsed / 1000.0
				}
				rpiTemp = strconv.FormatFloat(parsed, 'f', 3, 64)
				// Convert to pseudo-sensors format for compatibility
				return fmt.Sprintf(`{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":%s}}}`, rpiTemp), nil
			}
		} else {
			log.Debug().
				Str("component", "sensors_collector").
				Str("action", "collect_local_rpi_fallback_failed").
				Str("thermal_path", "/sys/class/thermal/thermal_zone0/temp").
				Err(rpiErr).
				Msg("Raspberry Pi thermal fallback failed")
		}
		return "", fmt.Errorf("sensors returned empty output")
	}

	return outputStr, nil
}

func runCommandOutputLimited(cmd *exec.Cmd, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max bytes must be positive")
	}

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

func readRPiThermalMilliDegrees(path string) (int64, error) {
	raw, err := readLimitedTrimmedString(path, maxThermalFileReadBytes)
	if err != nil {
		return 0, err
	}

	if raw == "" {
		return 0, fmt.Errorf("empty thermal value")
	}

	temp, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid thermal value: %w", err)
	}
	if temp < -100000 || temp > 300000 {
		return 0, fmt.Errorf("thermal value out of range")
	}

	return temp, nil
}

func readLimitedTrimmedString(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", fmt.Errorf("file exceeds maximum size of %d bytes", maxBytes)
	}

	return strings.TrimSpace(string(data)), nil
}
