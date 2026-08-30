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

func TestLoadConfigAcceptsDirectAgentIDWithoutIdentityFile(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "runner.token")
	if err := os.WriteFile(token, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_URL", "https://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID", "agent-direct-1")
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", "")
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("d", 32))
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := resolveRunnerAgentID(config); err != nil || got != "agent-direct-1" {
		t.Fatalf("resolveRunnerAgentID = %q, %v", got, err)
	}
}

func TestResolveRunnerAgentIDPrefersDirectBoundIdentity(t *testing.T) {
	config := runtimeConfig{AgentID: "agent-direct", AgentIDFile: filepath.Join(t.TempDir(), "missing")}
	if got, err := resolveRunnerAgentID(config); err != nil || got != "agent-direct" {
		t.Fatalf("resolveRunnerAgentID = %q, %v", got, err)
	}
}

func TestResolveRunnerAgentIDFallsBackToPrivateIdentityFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-id")
	if err := os.WriteFile(path, []byte("agent-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveRunnerAgentID(runtimeConfig{AgentIDFile: path}); err != nil || got != "agent-file" {
		t.Fatalf("resolveRunnerAgentID = %q, %v", got, err)
	}
}

func TestResolveRunnerAgentIDRejectsInvalidFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-id")
	if err := os.WriteFile(path, []byte("bad agent\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRunnerAgentID(runtimeConfig{AgentIDFile: path}); err == nil || !strings.Contains(err.Error(), "identity file is invalid") {
		t.Fatalf("resolveRunnerAgentID error = %v", err)
	}
}

func TestLoadConfigRejectsInvalidDirectAgentID(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "runner.token")
	if err := os.WriteFile(token, []byte("secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_URL", "https://pulse.example")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID_FILE", "")
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("e", 32))
	for _, agentID := range []string{"bad agent", "-agent", strings.Repeat("a", 129)} {
		t.Run(agentID, func(t *testing.T) {
			t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID", agentID)
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "valid agent identity") {
				t.Fatalf("loadConfig error = %v", err)
			}
		})
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

func TestLoadConfigPlaintextPolicyIsLoopbackOnlyEvenWhenInsecure(t *testing.T) {
	for _, rawURL := range []string{"http://pulse.example.com:7655", "http://192.168.1.20:7655", "http://agent.localhost:7655"} {
		t.Run(rawURL, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_URL", rawURL)
			t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", filepath.Join(dir, "token"))
			t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
			t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
			t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID", "agent-1")
			t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("f", 32))
			t.Setenv("PULSE_INSECURE", "true")
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "loopback") {
				t.Fatalf("loadConfig(%q) error = %v, want non-loopback plaintext rejection", rawURL, err)
			}
		})
	}
	t.Run("generic insecure HTTPS rejected", func(t *testing.T) {
		dir := t.TempDir()
		token := filepath.Join(dir, "token")
		if err := os.WriteFile(token, []byte("runner-secret\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PULSE_URL", "https://pulse.example")
		t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
		t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
		t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
		t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID", "agent-1")
		t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("f", 32))
		t.Setenv("PULSE_INSECURE", "true")
		if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "generic insecure HTTPS") {
			t.Fatalf("generic insecure HTTPS error = %v", err)
		}
	})

	dir := t.TempDir()
	token := filepath.Join(dir, "token")
	if err := os.WriteFile(token, []byte("runner-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PULSE_URL", "http://127.0.0.1:7655")
	t.Setenv("PULSE_AGENT_RUNNER_TOKEN_FILE", token)
	t.Setenv("PULSE_AGENT_RUNNER_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("PULSE_AGENT_RUNNER_HEALTH_FILE", filepath.Join(dir, "state", "health.json"))
	t.Setenv("PULSE_AGENT_RUNNER_AGENT_ID", "agent-1")
	t.Setenv("PULSE_AGENT_RUNNER_ACTIVATION_NONCE", strings.Repeat("f", 32))
	t.Setenv("PULSE_INSECURE", "true")
	if _, err := loadConfig(); err != nil {
		t.Fatalf("loopback HTTP config rejected: %v", err)
	}
}
