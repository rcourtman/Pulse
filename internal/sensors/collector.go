package sensors

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
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
		// Try Raspberry Pi temperature method as fallback
		cmd = exec.CommandContext(cmdCtx, "cat", "/sys/class/thermal/thermal_zone0/temp")
		if rpiOutput, rpiErr := cmd.Output(); rpiErr == nil {
			rpiTemp := strings.TrimSpace(string(rpiOutput))
			if rpiTemp != "" {
				if parsed, parseErr := strconv.ParseFloat(rpiTemp, 64); parseErr == nil {
					// Linux thermal_zone values are commonly millidegrees (e.g. 42000).
					// Convert only when magnitude indicates millidegrees to keep degree inputs intact.
					if parsed >= 1000 || parsed <= -1000 {
						parsed = parsed / 1000.0
					}
					rpiTemp = strconv.FormatFloat(parsed, 'f', 3, 64)
				}
				// Convert to pseudo-sensors format for compatibility
				return fmt.Sprintf(`{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":%s}}}`, rpiTemp), nil
			}
		}
		return "", fmt.Errorf("sensors returned empty output")
	}

	return outputStr, nil
}
