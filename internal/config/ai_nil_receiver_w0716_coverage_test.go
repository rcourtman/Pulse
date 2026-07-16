package config

import "testing"

// Test_w0716_cfg_GetOllamaKeepAlive exercises both the nil-receiver
// default-return branch and the field-read branch of GetOllamaKeepAlive.
func Test_w0716_cfg_GetOllamaKeepAlive(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AIConfig
		want string
	}{
		{name: "nil receiver returns pulse default", cfg: nil, want: DefaultOllamaKeepAlive},
		{name: "populated returns trimmed value", cfg: &AIConfig{OllamaKeepAlive: "  45m  "}, want: "45m"},
		{name: "empty field preserved as empty", cfg: &AIConfig{OllamaKeepAlive: ""}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetOllamaKeepAlive(); got != tt.want {
				t.Fatalf("GetOllamaKeepAlive() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test_w0716_cfg_GetModel covers the nil-receiver arm of GetModel alongside the
// field-read arm.
func Test_w0716_cfg_GetModel(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AIConfig
		want string
	}{
		{name: "nil receiver returns empty", cfg: nil, want: ""},
		{name: "populated returns normalized model", cfg: &AIConfig{Model: "openai:gpt-4o"}, want: "openai:gpt-4o"},
		{name: "retired quickstart normalized away", cfg: &AIConfig{Model: "pulse-hosted"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetModel(); got != tt.want {
				t.Fatalf("GetModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test_w0716_cfg_NormalizePatrolEventTriggerSettings covers the nil-receiver
// arm and each normalization mutation arm (already-consistent, legacy-aggregate
// backfill, and fully-disabled).
func Test_w0716_cfg_NormalizePatrolEventTriggerSettings(t *testing.T) {
	t.Run("nil receiver returns false", func(t *testing.T) {
		var c *AIConfig
		if c.NormalizePatrolEventTriggerSettings() {
			t.Fatal("nil receiver NormalizePatrolEventTriggerSettings() = true, want false")
		}
	})

	t.Run("already consistent returns false without mutation", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   true,
			PatrolAnomalyTriggersEnabled: true,
			PatrolEventTriggersEnabled:   true,
		}
		if cfg.NormalizePatrolEventTriggerSettings() {
			t.Fatal("expected no change for already-consistent config")
		}
		if !cfg.PatrolAlertTriggersEnabled || !cfg.PatrolAnomalyTriggersEnabled || !cfg.PatrolEventTriggersEnabled {
			t.Fatal("consistent config must not be mutated")
		}
	})

	t.Run("legacy aggregate backfills both split flags", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   false,
			PatrolAnomalyTriggersEnabled: false,
			PatrolEventTriggersEnabled:   true,
		}
		if !cfg.NormalizePatrolEventTriggerSettings() {
			t.Fatal("expected change when legacy aggregate forces split flags")
		}
		if !cfg.PatrolAlertTriggersEnabled || !cfg.PatrolAnomalyTriggersEnabled {
			t.Fatalf("expected both split flags enabled after normalize, got alert=%v anomaly=%v",
				cfg.PatrolAlertTriggersEnabled, cfg.PatrolAnomalyTriggersEnabled)
		}
		if !cfg.PatrolEventTriggersEnabled {
			t.Fatal("aggregate flag should remain enabled")
		}
	})

	t.Run("fully disabled stays consistent", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   false,
			PatrolAnomalyTriggersEnabled: false,
			PatrolEventTriggersEnabled:   false,
		}
		if cfg.NormalizePatrolEventTriggerSettings() {
			t.Fatal("expected no change for fully-disabled consistent config")
		}
	})
}

// Test_w0716_cfg_IsPatrolAlertTriggersEnabled covers the AI-disabled gate arm.
func Test_w0716_cfg_IsPatrolAlertTriggersEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AIConfig
		want bool
	}{
		{name: "disabled when AI master switch off", cfg: &AIConfig{Enabled: false, PatrolAlertTriggersEnabled: true}, want: false},
		{name: "enabled when AI on and alert triggers on", cfg: &AIConfig{Enabled: true, PatrolAlertTriggersEnabled: true}, want: true},
		{name: "false when alert triggers off", cfg: &AIConfig{Enabled: true, PatrolAlertTriggersEnabled: false}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsPatrolAlertTriggersEnabled(); got != tt.want {
				t.Fatalf("IsPatrolAlertTriggersEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_w0716_cfg_IsPatrolAnomalyTriggersEnabled covers the AI-disabled gate arm.
func Test_w0716_cfg_IsPatrolAnomalyTriggersEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AIConfig
		want bool
	}{
		{name: "disabled when AI master switch off", cfg: &AIConfig{Enabled: false, PatrolAnomalyTriggersEnabled: true}, want: false},
		{name: "enabled when AI on and anomaly triggers on", cfg: &AIConfig{Enabled: true, PatrolAnomalyTriggersEnabled: true}, want: true},
		{name: "false when anomaly triggers off", cfg: &AIConfig{Enabled: true, PatrolAnomalyTriggersEnabled: false}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsPatrolAnomalyTriggersEnabled(); got != tt.want {
				t.Fatalf("IsPatrolAnomalyTriggersEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_w0716_cfg_patrolEventTriggerPreferences covers the nil-receiver arm of
// the unexported helper, exercised both directly and via the public accessor,
// plus the legacy-aggregate and split-flag pass-through arms.
func Test_w0716_cfg_patrolEventTriggerPreferences(t *testing.T) {
	t.Run("nil receiver returns false false directly", func(t *testing.T) {
		var c *AIConfig
		alert, anomaly := c.patrolEventTriggerPreferences()
		if alert || anomaly {
			t.Fatalf("nil patrolEventTriggerPreferences() = (%v, %v), want (false, false)", alert, anomaly)
		}
	})

	t.Run("nil receiver via public accessor", func(t *testing.T) {
		var c *AIConfig
		settings := c.GetPatrolEventTriggerSettings()
		if settings.AlertTriggersEnabled || settings.AnomalyTriggersEnabled {
			t.Fatalf("nil GetPatrolEventTriggerSettings() = %+v, want both false", settings)
		}
	})

	t.Run("legacy aggregate enables both", func(t *testing.T) {
		cfg := &AIConfig{PatrolEventTriggersEnabled: true}
		alert, anomaly := cfg.patrolEventTriggerPreferences()
		if !alert || !anomaly {
			t.Fatalf("patrolEventTriggerPreferences() = (%v, %v), want (true, true) via legacy aggregate", alert, anomaly)
		}
	})

	t.Run("split flags pass through unchanged", func(t *testing.T) {
		cfg := &AIConfig{PatrolAlertTriggersEnabled: true, PatrolAnomalyTriggersEnabled: false}
		alert, anomaly := cfg.patrolEventTriggerPreferences()
		if !alert || anomaly {
			t.Fatalf("patrolEventTriggerPreferences() = (%v, %v), want (true, false)", alert, anomaly)
		}
	})
}
