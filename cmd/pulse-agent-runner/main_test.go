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
	t.Setenv("PULSE_SERVER_FINGERPRINT", "sha256:test")
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.TokenFile != token || config.ServerFingerprint != "sha256:test" || config.Insecure {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadConfigRejectsTokenInArgvEquivalentAndInsecureHTTPByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_URL", "http://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", filepath.Join(dir, "missing"))
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", filepath.Join(dir, "agent-id"))
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("error = %v", err)
	}
}
