package server

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ResolveMetricsPortFromEnv returns the configured metrics port using the same
// env contract for both the core and enterprise binaries.
func ResolveMetricsPortFromEnv(stderr io.Writer, fallback int) int {
	for _, envName := range []string{"PULSE_METRICS_PORT", "METRICS_PORT"} {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			continue
		}

		port, err := strconv.Atoi(raw)
		if err != nil || port < 0 || port > 65535 {
			if stderr != nil {
				fmt.Fprintf(stderr, "Ignoring invalid %s value %q; using metrics port %d\n", envName, raw, fallback)
			}
			return fallback
		}
		return port
	}

	return fallback
}
