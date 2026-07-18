package config

import (
	"encoding/json"
	"testing"
)

func TestPatrolFindingTriggersNotification(t *testing.T) {
	enabled := func() *AIConfig {
		cfg := NewDefaultAIConfig()
		cfg.Enabled = true
		return cfg
	}

	t.Run("nil config never notifies", func(t *testing.T) {
		var cfg *AIConfig
		if cfg.PatrolFindingTriggersNotification("critical") {
			t.Fatal("nil config must not notify")
		}
	})

	t.Run("disabled AI master switch blocks", func(t *testing.T) {
		cfg := NewDefaultAIConfig()
		if cfg.PatrolFindingTriggersNotification("critical") {
			t.Fatal("AI disabled must not notify")
		}
	})

	t.Run("explicit opt-out blocks", func(t *testing.T) {
		cfg := enabled()
		cfg.PatrolFindingNotificationsEnabled = false
		if cfg.PatrolFindingTriggersNotification("critical") {
			t.Fatal("opted-out config must not notify")
		}
	})

	t.Run("default floor accepts warning and critical", func(t *testing.T) {
		cfg := enabled()
		if !cfg.PatrolFindingTriggersNotification("warning") {
			t.Fatal("warning finding must notify at the default floor")
		}
		if !cfg.PatrolFindingTriggersNotification("critical") {
			t.Fatal("critical finding must notify at the default floor")
		}
	})

	t.Run("info and watch never notify", func(t *testing.T) {
		cfg := enabled()
		for _, severity := range []string{"info", "watch", "", "bogus"} {
			if cfg.PatrolFindingTriggersNotification(severity) {
				t.Fatalf("severity %q must not notify", severity)
			}
		}
	})

	t.Run("critical floor drops warning", func(t *testing.T) {
		cfg := enabled()
		cfg.PatrolFindingNotifyMinSeverity = AlertTriggerSeverityCritical
		if cfg.PatrolFindingTriggersNotification("warning") {
			t.Fatal("warning finding must not notify at the critical floor")
		}
		if !cfg.PatrolFindingTriggersNotification("critical") {
			t.Fatal("critical finding must notify at the critical floor")
		}
	})
}

func TestGetPatrolFindingNotifyMinSeverity(t *testing.T) {
	var nilCfg *AIConfig
	if got := nilCfg.GetPatrolFindingNotifyMinSeverity(); got != AlertTriggerSeverityWarning {
		t.Fatalf("nil config floor = %q, want warning", got)
	}
	cfg := &AIConfig{}
	if got := cfg.GetPatrolFindingNotifyMinSeverity(); got != AlertTriggerSeverityWarning {
		t.Fatalf("empty floor = %q, want warning", got)
	}
	cfg.PatrolFindingNotifyMinSeverity = " Critical "
	if got := cfg.GetPatrolFindingNotifyMinSeverity(); got != AlertTriggerSeverityCritical {
		t.Fatalf("normalized floor = %q, want critical", got)
	}
	cfg.PatrolFindingNotifyMinSeverity = "bogus"
	if got := cfg.GetPatrolFindingNotifyMinSeverity(); got != AlertTriggerSeverityWarning {
		t.Fatalf("invalid floor = %q, want warning", got)
	}
}

// Configs saved before the field existed must inherit the enabled default,
// while a persisted explicit opt-out must survive a save/load round trip.
// LoadAIConfig unmarshals over NewDefaultAIConfig, which this mirrors.
func TestPatrolFindingNotificationsPersistence(t *testing.T) {
	t.Run("missing field inherits default on", func(t *testing.T) {
		settings := NewDefaultAIConfig()
		if err := json.Unmarshal([]byte(`{"enabled":true}`), settings); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !settings.PatrolFindingNotificationsEnabled {
			t.Fatal("legacy config without the field must default to enabled")
		}
	})

	t.Run("explicit false survives round trip", func(t *testing.T) {
		saved := NewDefaultAIConfig()
		saved.PatrolFindingNotificationsEnabled = false
		data, err := json.Marshal(saved)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		loaded := NewDefaultAIConfig()
		if err := json.Unmarshal(data, loaded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if loaded.PatrolFindingNotificationsEnabled {
			t.Fatal("explicit opt-out must survive save/load")
		}
	})
}
