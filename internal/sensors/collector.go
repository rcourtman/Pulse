package sensors

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CollectLocal reads sensor data from the local machine using lm-sensors.
// Returns the raw JSON output from `sensors -j` or an error if sensors is not available.
func CollectLocal(ctx context.Context) (string, error) {
	// Check if sensors command exists
	if _, err := exec.LookPath("sensors"); err != nil {
		return "", fmt.Errorf("lm-sensors not installed: %w", err)
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Run sensors -j command
	// sensors exits non-zero when optional subfeatures fail; "|| true" keeps the JSON for parsing
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", "sensors -j 2>/dev/null || true")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute sensors: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || outputStr == "{}" {
		log.Debug().
			Str("component", "sensors_collector").
			Str("action", "collect_local_empty_output").
			Msg("lm-sensors returned empty output, attempting Raspberry Pi thermal fallback")

		// Try Raspberry Pi temperature method as fallback
		cmd = exec.CommandContext(cmdCtx, "cat", "/sys/class/thermal/thermal_zone0/temp")
		rpiOutput, rpiErr := cmd.Output()
		if rpiErr == nil {
			rpiTemp := strings.TrimSpace(string(rpiOutput))
			if rpiTemp != "" {
				log.Debug().
					Str("component", "sensors_collector").
					Str("action", "collect_local_rpi_fallback_success").
					Str("thermal_path", "/sys/class/thermal/thermal_zone0/temp").
					Msg("Collected sensor data from Raspberry Pi thermal fallback")

				// Convert to pseudo-sensors format for compatibility
				// Raspberry Pi reports in millidegrees Celsius
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
