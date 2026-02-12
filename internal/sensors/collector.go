package sensors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var rpiThermalZoneTempPath = "/sys/class/thermal/thermal_zone0/temp"

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

	// Run sensors -j command directly to avoid shell dependency in minimal runtimes.
	cmd := exec.CommandContext(cmdCtx, "sensors", "-j")
	output, err := cmd.Output()
	outputStr := strings.TrimSpace(string(output))
	// sensors may exit non-zero even when usable JSON is emitted.
	if err != nil && outputStr == "" {
		return "", fmt.Errorf("failed to execute sensors: %w", err)
	}

	if outputStr == "" || outputStr == "{}" {
		// Try Raspberry Pi temperature method as fallback
		if rpiOutput, rpiErr := os.ReadFile(rpiThermalZoneTempPath); rpiErr == nil {
			rpiTemp := strings.TrimSpace(string(rpiOutput))
			if rpiTemp != "" {
				// Convert to pseudo-sensors format for compatibility
				// Raspberry Pi reports in millidegrees Celsius
				return fmt.Sprintf(`{"cpu_thermal-virtual-0":{"temp1":{"temp1_input":%s}}}`, rpiTemp), nil
			}
		}
		return "", fmt.Errorf("sensors returned empty output")
	}

	return outputStr, nil
}
