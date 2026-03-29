package server

import (
	"bytes"
	"testing"
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
