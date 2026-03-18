package mock

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var mockConfigEnvKeys = []string{
	"PULSE_MOCK_NODES",
	"PULSE_MOCK_VMS_PER_NODE",
	"PULSE_MOCK_LXCS_PER_NODE",
	"PULSE_MOCK_DOCKER_HOSTS",
	"PULSE_MOCK_DOCKER_CONTAINERS",
	"PULSE_MOCK_GENERIC_HOSTS",
	"PULSE_MOCK_K8S_CLUSTERS",
	"PULSE_MOCK_K8S_NODES",
	"PULSE_MOCK_K8S_PODS",
	"PULSE_MOCK_K8S_DEPLOYMENTS",
	"PULSE_MOCK_RANDOM_METRICS",
	"PULSE_MOCK_STOPPED_PERCENT",
}

func TestLoadMockConfigLogsInvalidOverridesAndKeepsLegacyFallbacks(t *testing.T) {
	resetMockConfigEnv(t)
	t.Setenv("PULSE_MOCK_NODES", "invalid")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "-2")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "sometimes")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "not-a-percent")

	var buf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	defer func() {
		log.Logger = origLogger
	}()

	cfg := LoadMockConfig()

	if cfg.NodeCount != DefaultConfig.NodeCount {
		t.Fatalf("expected NodeCount default %d, got %d", DefaultConfig.NodeCount, cfg.NodeCount)
	}
	if cfg.VMsPerNode != DefaultConfig.VMsPerNode {
		t.Fatalf("expected VMsPerNode default %d, got %d", DefaultConfig.VMsPerNode, cfg.VMsPerNode)
	}
	if cfg.RandomMetrics != DefaultConfig.RandomMetrics {
		t.Fatalf("expected RandomMetrics=%t (default) for invalid legacy override, got %t", DefaultConfig.RandomMetrics, cfg.RandomMetrics)
	}
	if cfg.StoppedPercent != DefaultConfig.StoppedPercent {
		t.Fatalf("expected StoppedPercent default %f, got %f", DefaultConfig.StoppedPercent, cfg.StoppedPercent)
	}

	logOutput := buf.String()
	for _, envKey := range []string{
		`"env":"PULSE_MOCK_NODES"`,
		`"env":"PULSE_MOCK_VMS_PER_NODE"`,
		`"env":"PULSE_MOCK_RANDOM_METRICS"`,
		`"env":"PULSE_MOCK_STOPPED_PERCENT"`,
	} {
		if !strings.Contains(logOutput, envKey) {
			t.Fatalf("expected warning log to include %s, got %q", envKey, logOutput)
		}
	}
}

func TestLoadMockConfigAppliesValidOverrides(t *testing.T) {
	resetMockConfigEnv(t)
	t.Setenv("PULSE_MOCK_NODES", "7")
	t.Setenv("PULSE_MOCK_RANDOM_METRICS", "true")
	t.Setenv("PULSE_MOCK_STOPPED_PERCENT", "25")

	cfg := LoadMockConfig()

	if cfg.NodeCount != 7 {
		t.Fatalf("expected NodeCount=7, got %d", cfg.NodeCount)
	}
	if !cfg.RandomMetrics {
		t.Fatal("expected RandomMetrics=true")
	}
	if cfg.StoppedPercent != 0.25 {
		t.Fatalf("expected StoppedPercent=0.25, got %f", cfg.StoppedPercent)
	}
}

func resetMockConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range mockConfigEnvKeys {
		t.Setenv(key, "")
	}
}
