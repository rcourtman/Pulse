package server

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const defaultMetricsBindAddress = "127.0.0.1"

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

func metricsListenAddress(cfg *config.Config, port int) string {
	bindAddress := defaultMetricsBindAddress
	if cfg != nil && strings.TrimSpace(cfg.MetricsBindAddress) != "" {
		bindAddress = strings.TrimSpace(cfg.MetricsBindAddress)
	}
	return net.JoinHostPort(bindAddress, strconv.Itoa(port))
}

func metricsAddressIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
