package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigUsesDedicatedEnvironmentAndPrivateTokenFile(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "runner.token")
	agentID := filepath.Join(dir, "agent-id")
	if err := os.WriteFile(token, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentID, []byte("agent-1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_URL", "https://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", agentID)
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("a", 32))
	t.Setenv("PULSE_AGENT_RUNNER_HOSTNAME", " Node.Example. ")
	t.Setenv("PULSE_SERVER_FINGERPRINT", "sha256:test")
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.TokenFile != token || config.ServerFingerprint != "sha256:test" || config.Hostname != "node.example" || config.Insecure {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadConfigRejectsInvalidCanonicalHostnameOverride(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "runner.token")
	if err := os.WriteFile(token, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_URL", "https://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", filepath.Join(dir, "agent-id"))
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("b", 32))
	for _, hostname := range []string{"bad host", "-node.example", "node/example", strings.Repeat("a", 64) + ".example"} {
		t.Run(hostname, func(t *testing.T) {
			t.Setenv("PULSE_AGENT_RUNNER_HOSTNAME", hostname)
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "valid hostname") {
				t.Fatalf("loadConfig error = %v", err)
			}
		})
	}
}

func TestNormalizeRunnerHostnameAcceptsIPLiteral(t *testing.T) {
	if got, err := normalizeRunnerHostname(" 2001:DB8::1 "); err != nil || got != "2001:db8::1" {
		t.Fatalf("normalizeRunnerHostname = %q, %v", got, err)
	}
}

func TestLoadConfigRejectsTokenInArgvEquivalentAndInsecureHTTPByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_URL", "http://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", filepath.Join(dir, "missing"))
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", filepath.Join(dir, "agent-id"))
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("c", 32))
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("error = %v", err)
	}
}
