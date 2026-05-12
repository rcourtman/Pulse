package relay

import "testing"

func TestApplyEnvOverridesUnsetLeavesConfigAlone(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "")
	t.Setenv(EnvRelayServerURL, "")

	cfg := &Config{Enabled: true, ServerURL: "wss://file.example/ws/instance"}
	ApplyEnvOverrides(cfg)

	if !cfg.Enabled {
		t.Fatalf("Enabled = false, want true (file value preserved when env unset)")
	}
	if cfg.ServerURL != "wss://file.example/ws/instance" {
		t.Fatalf("ServerURL = %q, want file value preserved", cfg.ServerURL)
	}
}

func TestApplyEnvOverridesEnabledTrueOverridesFile(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "true")
	t.Setenv(EnvRelayServerURL, "")

	cfg := &Config{Enabled: false, ServerURL: DefaultServerURL}
	ApplyEnvOverrides(cfg)

	if !cfg.Enabled {
		t.Fatalf("Enabled = false, want true (env override true)")
	}
}

func TestApplyEnvOverridesEnabledFalseOverridesFile(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "false")
	t.Setenv(EnvRelayServerURL, "")

	cfg := &Config{Enabled: true, ServerURL: DefaultServerURL}
	ApplyEnvOverrides(cfg)

	if cfg.Enabled {
		t.Fatalf("Enabled = true, want false (env override false should disable a file-enabled relay)")
	}
}

func TestApplyEnvOverridesGarbageBoolIgnored(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "maybe")
	t.Setenv(EnvRelayServerURL, "")

	cfg := &Config{Enabled: true, ServerURL: DefaultServerURL}
	ApplyEnvOverrides(cfg)

	if !cfg.Enabled {
		t.Fatalf("Enabled = false, want true (garbage bool should not override)")
	}
}

func TestApplyEnvOverridesValidServerURLOverridesFile(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "")
	t.Setenv(EnvRelayServerURL, "wss://relay.test.example/ws/instance")

	cfg := &Config{Enabled: true, ServerURL: DefaultServerURL}
	ApplyEnvOverrides(cfg)

	if cfg.ServerURL != "wss://relay.test.example/ws/instance" {
		t.Fatalf("ServerURL = %q, want env override", cfg.ServerURL)
	}
}

func TestApplyEnvOverridesInvalidServerURLKeepsFile(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "")
	t.Setenv(EnvRelayServerURL, "http://wrong-scheme.example/")

	cfg := &Config{Enabled: true, ServerURL: "wss://file.example/ws/instance"}
	ApplyEnvOverrides(cfg)

	if cfg.ServerURL != "wss://file.example/ws/instance" {
		t.Fatalf("ServerURL = %q, want file value preserved (invalid env URL should not override)", cfg.ServerURL)
	}
}

func TestApplyEnvOverridesBothApplyTogether(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "yes")
	t.Setenv(EnvRelayServerURL, "wss://relay.test.example/ws/instance")

	cfg := &Config{Enabled: false, ServerURL: DefaultServerURL}
	ApplyEnvOverrides(cfg)

	if !cfg.Enabled || cfg.ServerURL != "wss://relay.test.example/ws/instance" {
		t.Fatalf("ApplyEnvOverrides did not apply both overrides: %+v", cfg)
	}
}

func TestApplyEnvOverridesNilConfigSafe(t *testing.T) {
	t.Setenv(EnvRelayEnabled, "true")
	t.Setenv(EnvRelayServerURL, "wss://relay.test.example/ws/instance")

	ApplyEnvOverrides(nil) // must not panic
}

func TestParseEnvBoolRecognizedValues(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "True", "yes", "y", "on", " on "}
	for _, v := range truthy {
		got, ok := parseEnvBool(v)
		if !ok || !got {
			t.Errorf("parseEnvBool(%q) = (%v, %v), want (true, true)", v, got, ok)
		}
	}
	falsy := []string{"0", "false", "FALSE", "no", "n", "off"}
	for _, v := range falsy {
		got, ok := parseEnvBool(v)
		if !ok || got {
			t.Errorf("parseEnvBool(%q) = (%v, %v), want (false, true)", v, got, ok)
		}
	}
	unrecognized := []string{"", "  ", "maybe", "2", "enable"}
	for _, v := range unrecognized {
		_, ok := parseEnvBool(v)
		if ok {
			t.Errorf("parseEnvBool(%q) reported ok=true, want false", v)
		}
	}
}
