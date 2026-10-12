package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These branch-coverage tests target the effective-evaluation accessors on
// AIConfig: GetEffectiveControlLevel and SetPatrolEventTriggersEnabled. They
// exercise the nil-receiver guards, the entitlement downgrade, and the
// nil-receiver no-op of the setter.

// TestBranchCovGetEffectiveControlLevel covers both the nil-receiver guard of
// GetEffectiveControlLevel and every branch of the entitlement evaluation it
// delegates to: the read_only default (empty, unknown, and legacy levels), the
// untouched controlled/read_only levels, and the autonomous downgrade that
// fires only when the autonomous entitlement is missing.
func TestBranchCovGetEffectiveControlLevel(t *testing.T) {
	tests := []struct {
		name              string
		config            *AIConfig
		autonomousAllowed bool
		want              string
	}{
		{
			name:              "nil receiver fails closed to read_only",
			config:            nil,
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "nil receiver fails closed regardless of entitlement",
			config:            nil,
			autonomousAllowed: false,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "empty control level normalizes to read_only",
			config:            &AIConfig{ControlLevel: ""},
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "explicit read_only is preserved with entitlement",
			config:            &AIConfig{ControlLevel: ControlLevelReadOnly},
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "explicit read_only preserved without entitlement",
			config:            &AIConfig{ControlLevel: ControlLevelReadOnly},
			autonomousAllowed: false,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "controlled is preserved with entitlement",
			config:            &AIConfig{ControlLevel: ControlLevelControlled},
			autonomousAllowed: true,
			want:              ControlLevelControlled,
		},
		{
			name:              "controlled is preserved even without autonomous entitlement",
			config:            &AIConfig{ControlLevel: ControlLevelControlled},
			autonomousAllowed: false,
			want:              ControlLevelControlled,
		},
		{
			name:              "autonomous preserved when entitlement allows it",
			config:            &AIConfig{ControlLevel: ControlLevelAutonomous},
			autonomousAllowed: true,
			want:              ControlLevelAutonomous,
		},
		{
			name:              "autonomous downgraded to controlled when entitlement missing",
			config:            &AIConfig{ControlLevel: ControlLevelAutonomous},
			autonomousAllowed: false,
			want:              ControlLevelControlled,
		},
		{
			name:              "unknown level fails closed to read_only with entitlement",
			config:            &AIConfig{ControlLevel: "nonsense"},
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "unknown level fails closed to read_only without entitlement",
			config:            &AIConfig{ControlLevel: "nonsense"},
			autonomousAllowed: false,
			want:              ControlLevelReadOnly,
		},
		{
			name:              "legacy suggest level fails closed to read_only",
			config:            &AIConfig{ControlLevel: "suggest"},
			autonomousAllowed: true,
			want:              ControlLevelReadOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetEffectiveControlLevel(tt.autonomousAllowed)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBranchCovSetPatrolEventTriggersEnabled covers the nil-receiver no-op and
// both values of the enabled argument, including the case where the granular
// flags start divergent and are collapsed to a single setting. It also confirms
// the setter writes all three persisted fields (both canonical split flags and
// the legacy aggregate) and that GetPatrolEventTriggerSettings reflects them.
func TestBranchCovSetPatrolEventTriggersEnabled(t *testing.T) {
	t.Run("nil receiver is a no-op without panic", func(t *testing.T) {
		var cfg *AIConfig
		// SetPatrolEventTriggersEnabled is nil-safe and must early-return
		// without mutating the receiver. The receiver stays nil afterwards; we
		// do NOT call the non-nil-safe IsPatrolEventTriggersEnabled getter here
		// (it dereferences c.Enabled and is therefore not symmetric with the
		// setter's nil guard).
		assert.NotPanics(t, func() { cfg.SetPatrolEventTriggersEnabled(true) })
		assert.Nil(t, cfg)
	})

	t.Run("enabling sets all three persisted trigger flags from an empty config", func(t *testing.T) {
		cfg := &AIConfig{}
		cfg.SetPatrolEventTriggersEnabled(true)
		assert.True(t, cfg.PatrolAlertTriggersEnabled)
		assert.True(t, cfg.PatrolAnomalyTriggersEnabled)
		assert.True(t, cfg.PatrolEventTriggersEnabled)
		settings := cfg.GetPatrolEventTriggerSettings()
		assert.True(t, settings.AlertTriggersEnabled)
		assert.True(t, settings.AnomalyTriggersEnabled)
	})

	t.Run("disabling clears all three persisted trigger flags after being enabled", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   true,
			PatrolAnomalyTriggersEnabled: true,
			PatrolEventTriggersEnabled:   true,
		}
		cfg.SetPatrolEventTriggersEnabled(false)
		assert.False(t, cfg.PatrolAlertTriggersEnabled)
		assert.False(t, cfg.PatrolAnomalyTriggersEnabled)
		assert.False(t, cfg.PatrolEventTriggersEnabled)
		settings := cfg.GetPatrolEventTriggerSettings()
		assert.False(t, settings.AlertTriggersEnabled)
		assert.False(t, settings.AnomalyTriggersEnabled)
	})

	t.Run("enable collapses divergent granular flags to both true", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   true,
			PatrolAnomalyTriggersEnabled: false,
			PatrolEventTriggersEnabled:   true,
		}
		cfg.SetPatrolEventTriggersEnabled(true)
		assert.True(t, cfg.PatrolAlertTriggersEnabled)
		assert.True(t, cfg.PatrolAnomalyTriggersEnabled)
		assert.True(t, cfg.PatrolEventTriggersEnabled)
	})

	t.Run("disable collapses divergent granular flags to both false", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAlertTriggersEnabled:   true,
			PatrolAnomalyTriggersEnabled: false,
			PatrolEventTriggersEnabled:   true,
		}
		cfg.SetPatrolEventTriggersEnabled(false)
		assert.False(t, cfg.PatrolAlertTriggersEnabled)
		assert.False(t, cfg.PatrolAnomalyTriggersEnabled)
		assert.False(t, cfg.PatrolEventTriggersEnabled)
	})
}
