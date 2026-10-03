package sensors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	thermalZoneRoot          = "/sys/class/thermal"
	hwmonRoot                = "/sys/class/hwmon"
)

// CollectLocal reads lm-sensors JSON and supplements it with a recognised CPU
// thermal sysfs source when it contains no usable CPU temperature. Other
// lm-sensors readings are retained; sysfs is also used when lm-sensors is
// unavailable or empty.
func CollectLocal(ctx context.Context) (string, error) {
	ctx = normalizeCollectionContext(ctx)

	sensorsPath, err := exec.LookPath("sensors")
	if err == nil {
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// sensors can exit non-zero when optional subfeatures fail, so accept
		// non-empty output even then. Never fall back after an output-limit hit.
		cmd := exec.CommandContext(cmdCtx, sensorsPath, "-j")
		cmd.Stderr = io.Discard
		output, commandErr := runCommandOutputLimited(cmd, maxSensorsOutputSizeBytes)
		if errors.Is(commandErr, errCommandOutputTooLarge) {
			return "", fmt.Errorf("failed to execute sensors: %w", commandErr)
		}
		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" && outputStr != "{}" {
			return addMissingSysfsCPU(ctx, outputStr), nil
		}
		if cmdCtx.Err() != nil {
			return "", fmt.Errorf("failed to execute sensors: %w", cmdCtx.Err())
		}
		log.Debug().Str("component", "sensors_collector").
			Str("action", "collect_local_empty_output").
			Msg("lm-sensors returned no data; trying recognised CPU thermal sysfs sources")
	}

	output, fallbackErr := collectSysfsCPUTemperature(ctx)
	if fallbackErr == nil {
		return output, nil
	}
	if err != nil {
		return "", fmt.Errorf("lm-sensors unavailable and CPU thermal fallback failed: %w", fallbackErr)
	}
	return "", fmt.Errorf("sensors returned empty output and CPU thermal fallback failed: %w", fallbackErr)
}

// addMissingSysfsCPU leaves existing output unchanged unless it is a JSON
// object without a usable CPU reading and an identified sysfs source is
// available. A failed optional lookup must not discard working sensors data.
func addMissingSysfsCPU(ctx context.Context, sensorsJSON string) string {
	var chips map[string]json.RawMessage
	if err := json.Unmarshal([]byte(sensorsJSON), &chips); err != nil || chips == nil {
		return sensorsJSON
	}
	parsed, err := Parse(sensorsJSON)
	if err != nil || parsed.CPUPackage > 0 {
		return sensorsJSON
	}
	fallbackJSON, err := collectSysfsCPUTemperature(ctx)
	if err != nil {
		return sensorsJSON
	}
	var fallback map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fallbackJSON), &fallback); err != nil {
		return sensorsJSON
	}
	for name, value := range fallback {
		if _, exists := chips[name]; exists {
			// A real lm-sensors chip can have the same name as the synthetic
			// fallback. Keep its other readings rather than replacing the chip.
			for suffix := 1; ; suffix++ {
				alternate := fmt.Sprintf("%s-sysfs-%d", name, suffix)
				if _, exists := chips[alternate]; !exists {
					name = alternate
					break
				}
			}
		}
		chips[name] = value
	}
	merged, err := json.Marshal(chips)
	if err != nil || len(merged) > maxSensorsOutputSizeBytes {
		return sensorsJSON
	}
	return string(merged)
}

// collectSysfsCPUTemperature deliberately accepts only identified CPU/SoC
// sensors. thermal_zone0 is not necessarily a CPU on every Linux machine.
func collectSysfsCPUTemperature(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var lastErr error
	for _, source := range []struct {
		root, prefix, label, value string
	}{
		{thermalZoneRoot, "thermal_zone", "type", "temp"},
		{hwmonRoot, "hwmon", "name", "temp1_input"},
	} {
		entries, err := os.ReadDir(source.root)
		if err != nil {
			lastErr = err
			continue
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if !strings.HasPrefix(entry.Name(), source.prefix) {
				continue
			}
			dir := filepath.Join(source.root, entry.Name())
			name, err := readBoundedThermalFile(filepath.Join(dir, source.label))
			if err != nil || !isCPUSysfsThermalName(name) {
				continue
			}
			raw, err := readBoundedThermalFile(filepath.Join(dir, source.value))
			if err != nil {
				lastErr = err
				continue
			}
			millidegrees, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || millidegrees < 1000 || millidegrees >= 150000 {
				lastErr = fmt.Errorf("invalid CPU thermal millidegree value from %s", dir)
				continue
			}
			celsius := float64(millidegrees) / 1000
			return fmt.Sprintf(`{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":%.3f}}}`, celsius), nil
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("no valid CPU thermal sysfs reading: %w", lastErr)
	}
	return "", errors.New("no recognised CPU thermal sysfs source")
}

func isCPUSysfsThermalName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "armada_thermal", "cpu_thermal", "cpu-thermal", "soc_thermal", "soc-thermal", "x86_pkg_temp", "rpitemp":
		return true
	default:
		return false
	}
}

func readBoundedThermalFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	contents, err := io.ReadAll(io.LimitReader(f, maxThermalFileReadBytes+1))
	if err != nil {
		return "", err
	}
	if len(contents) > maxThermalFileReadBytes {
		return "", fmt.Errorf("thermal sysfs value exceeds %d bytes", maxThermalFileReadBytes)
	}
	return strings.TrimSpace(string(contents)), nil
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
