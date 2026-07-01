package server

import (
	"bytes"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestResolveMetricsPortFromEnvPrefersPrefixedOverride(t *testing.T) {
	t.Setenv("PULSE_METRICS_PORT", "0")
	t.Setenv("METRICS_PORT", "9091")

	if got := ResolveMetricsPortFromEnv(nil, 7655); got != 0 {
		t.Fatalf("ResolveMetricsPortFromEnv() = %d, want 0", got)
	}
}

func TestResolveMetricsPortFromEnvFallsBackOnInvalidValue(t *testing.T) {
	t.Setenv("PULSE_METRICS_PORT", "not-a-port")

	var stderr bytes.Buffer
	if got := ResolveMetricsPortFromEnv(&stderr, 9091); got != 9091 {
		t.Fatalf("ResolveMetricsPortFromEnv() = %d, want fallback 9091", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected invalid metrics port warning")
	}
}

func TestMetricsListenAddressDefaultsToLoopback(t *testing.T) {
	if got := metricsListenAddress(&config.Config{}, 9091); got != "127.0.0.1:9091" {
		t.Fatalf("metricsListenAddress() = %q, want loopback default", got)
	}
}

func TestMetricsListenAddressUsesConfiguredAddress(t *testing.T) {
	cfg := &config.Config{MetricsBindAddress: "192.0.2.10"}
	if got := metricsListenAddress(cfg, 9091); got != "192.0.2.10:9091" {
		t.Fatalf("metricsListenAddress() = %q, want configured address", got)
	}
}

func TestMetricsAddressIsLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:9091", "[::1]:9091", "localhost:9091"} {
		if !metricsAddressIsLoopback(addr) {
			t.Fatalf("metricsAddressIsLoopback(%q) = false, want true", addr)
		}
	}
	if metricsAddressIsLoopback("0.0.0.0:9091") {
		t.Fatal("metricsAddressIsLoopback(0.0.0.0:9091) = true, want false")
	}
}
