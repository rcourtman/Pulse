package alerts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAlertConfigUnmarshal_LegacyHostAliasesIgnored(t *testing.T) {
	payload := []byte(`{
		"hostDefaults": {
			"cpu": {"trigger": 91, "clear": 86}
		},
		"disableAllHosts": true,
		"disableAllHostsOffline": true,
		"timeThresholds": {"host": 42, "docker": 7, "k8s": 8},
		"metricTimeThresholds": {
			"host": {"cpu": 11},
			"dockerhost": {"cpu": 5},
			"kubernetes-cluster": {"cpu": 9}
		}
	}`)

	var cfg AlertConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cfg.AgentDefaults.CPU != nil {
		t.Fatalf("expected hostDefaults to be ignored without populating agent defaults, got %#v", cfg.AgentDefaults.CPU)
	}
	if cfg.DisableAllAgents {
		t.Fatal("expected disableAllAgents to ignore disableAllHosts")
	}
	if cfg.DisableAllAgentsOffline {
		t.Fatal("expected disableAllAgentsOffline to ignore disableAllHostsOffline")
	}
	if _, exists := cfg.TimeThresholds["agent"]; exists {
		t.Fatalf("did not expect timeThresholds.agent from legacy host key, got %d", cfg.TimeThresholds["agent"])
	}
	if _, exists := cfg.TimeThresholds["host"]; exists {
		t.Fatal("expected legacy timeThresholds.host to be removed")
	}
	if _, exists := cfg.TimeThresholds["docker"]; exists {
		t.Fatal("expected legacy timeThresholds.docker to be removed")
	}
	if _, exists := cfg.TimeThresholds["k8s"]; exists {
		t.Fatal("expected legacy timeThresholds.k8s to be removed")
	}
	if _, exists := cfg.MetricTimeThresholds["agent"]; exists {
		t.Fatal("did not expect metricTimeThresholds.agent from legacy host key")
	}
	if _, exists := cfg.MetricTimeThresholds["host"]; exists {
		t.Fatal("expected legacy metricTimeThresholds.host to be removed")
	}
	if _, exists := cfg.MetricTimeThresholds["dockerhost"]; exists {
		t.Fatal("expected legacy metricTimeThresholds.dockerhost to be removed")
	}
	if _, exists := cfg.MetricTimeThresholds["kubernetes-cluster"]; exists {
		t.Fatal("expected legacy metricTimeThresholds.kubernetes-cluster to be removed")
	}
}

func TestAlertConfigUnmarshal_CanonicalKeysTakePrecedence(t *testing.T) {
	payload := []byte(`{
		"agentDefaults": {
			"cpu": {"trigger": 88, "clear": 83}
		},
		"hostDefaults": {
			"cpu": {"trigger": 99, "clear": 94}
		},
		"disableAllAgents": false,
		"disableAllHosts": true,
		"disableAllAgentsOffline": false,
		"disableAllHostsOffline": true,
		"timeThresholds": {"agent": 7, "host": 42},
		"metricTimeThresholds": {
			"agent": {"cpu": 3},
			"host": {"cpu": 11},
			"docker": {"cpu": 13}
		}
	}`)

	var cfg AlertConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cfg.AgentDefaults.CPU == nil || cfg.AgentDefaults.CPU.Trigger != 88 {
		t.Fatalf("expected canonical agentDefaults to win, got %#v", cfg.AgentDefaults.CPU)
	}
	if cfg.DisableAllAgents {
		t.Fatal("expected disableAllAgents=false from canonical key")
	}
	if cfg.DisableAllAgentsOffline {
		t.Fatal("expected disableAllAgentsOffline=false from canonical key")
	}
	if got := cfg.TimeThresholds["agent"]; got != 7 {
		t.Fatalf("expected canonical timeThresholds.agent=7, got %d", got)
	}
	if _, exists := cfg.TimeThresholds["host"]; exists {
		t.Fatal("did not expect legacy timeThresholds.host to remain")
	}
	if got := cfg.MetricTimeThresholds["agent"]["cpu"]; got != 3 {
		t.Fatalf("expected canonical metricTimeThresholds.agent.cpu=3, got %d", got)
	}
	if _, exists := cfg.MetricTimeThresholds["host"]; exists {
		t.Fatal("did not expect legacy metricTimeThresholds.host to remain")
	}
	if _, exists := cfg.MetricTimeThresholds["docker"]; exists {
		t.Fatal("did not expect legacy metricTimeThresholds.docker to remain")
	}
}

func TestAlertConfigMarshal_UsesCanonicalAgentKeys(t *testing.T) {
	cfg := AlertConfig{
		AgentDefaults: ThresholdConfig{
			CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		DisableAllAgents:        true,
		DisableAllAgentsOffline: true,
		TimeThresholds: map[string]int{
			"agent": 5,
		},
		MetricTimeThresholds: map[string]map[string]int{
			"agent": {"cpu": 2},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	encoded := string(data)
	if !strings.Contains(encoded, `"agentDefaults"`) {
		t.Fatalf("expected canonical agentDefaults in output: %s", encoded)
	}
	if strings.Contains(encoded, `"hostDefaults"`) {
		t.Fatalf("did not expect legacy hostDefaults in output: %s", encoded)
	}
	if !strings.Contains(encoded, `"disableAllAgents"`) {
		t.Fatalf("expected canonical disableAllAgents in output: %s", encoded)
	}
	if strings.Contains(encoded, `"disableAllHosts"`) {
		t.Fatalf("did not expect legacy disableAllHosts in output: %s", encoded)
	}
}
