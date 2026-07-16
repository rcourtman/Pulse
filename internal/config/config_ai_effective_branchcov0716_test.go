package config

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
)

// These branch-coverage tests target the effective-evaluation accessors on
// AIConfig: GetEffectiveControlLevel, GetEffectivePatrolAutonomy(WithPolicy),
// IsPatrolFullModeActive, and SetPatrolEventTriggersEnabled. They exercise the
// nil-receiver guards, the entitlement downgrade, every EvaluatePatrolAutopilot
// outcome reachable through the config wrapper, and the nil-receiver no-op of
// the setter. They reuse the same in-package evidence helpers as the sibling
// patrol-autopilot persistence tests.

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

// TestBranchCovGetEffectivePatrolAutonomy covers every reachable branch of
// GetEffectivePatrolAutonomyWithPolicy (which GetEffectivePatrolAutonomy
// delegates to): the nil-receiver contract fallback, the not-requested arm for
// each non-full requested mode, and the full-mode evaluation paths
// (acknowledgement required, legacy boolean ignored, invalid activation digest,
// wrong-org activation, and a genuinely active full-mode activation). It also
// confirms the time-based GetEffectivePatrolAutonomy wrapper agrees with the
// explicit-policy variant.
func TestBranchCovGetEffectivePatrolAutonomy(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)
	actor := configPatrolAutopilotActor("admin", "session:org-a", "org-a")
	acknowledgement, activation := configPatrolAutopilotEvidence(t, "ack-effective-branchcov", actor, policy)

	// An activation with a blank digest trips the activation-digest-invalid
	// arm before any acknowledgement lookup occurs.
	invalidDigestActivation := activation
	invalidDigestActivation.Digest = ""

	tests := []struct {
		name       string
		config     *AIConfig
		orgID      string
		wantLevel  string
		wantCode   string
		wantActive bool
	}{
		{
			name:       "nil receiver returns monitor with acknowledgement-required status",
			config:     nil,
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyMonitor,
			wantCode:   unifiedresources.PatrolAutopilotStatusAcknowledgementRequired,
			wantActive: false,
		},
		{
			name: "monitor requested mode is returned unchanged as not-requested",
			config: &AIConfig{
				PatrolAutonomyLevel: PatrolAutonomyMonitor,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyMonitor,
			wantCode:   unifiedresources.PatrolAutopilotStatusNotRequested,
			wantActive: false,
		},
		{
			name: "approval requested mode is returned unchanged as not-requested",
			config: &AIConfig{
				PatrolAutonomyLevel: PatrolAutonomyApproval,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyApproval,
			wantCode:   unifiedresources.PatrolAutopilotStatusNotRequested,
			wantActive: false,
		},
		{
			name: "assisted requested mode is returned unchanged as not-requested",
			config: &AIConfig{
				PatrolAutonomyLevel: PatrolAutonomyAssisted,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyAssisted,
			wantCode:   unifiedresources.PatrolAutopilotStatusNotRequested,
			wantActive: false,
		},
		{
			name: "full mode without activation requires acknowledgement and falls back to approval",
			config: &AIConfig{
				PatrolAutonomyLevel: PatrolAutonomyFull,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyApproval,
			wantCode:   unifiedresources.PatrolAutopilotStatusAcknowledgementRequired,
			wantActive: false,
		},
		{
			name: "full mode with legacy boolean unlocked but no activation reports legacy-ignored",
			config: &AIConfig{
				PatrolAutonomyLevel:    PatrolAutonomyFull,
				PatrolFullModeUnlocked: true,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyApproval,
			wantCode:   unifiedresources.PatrolAutopilotStatusLegacyBooleanIgnored,
			wantActive: false,
		},
		{
			name: "full mode with invalid activation digest falls back to approval",
			config: &AIConfig{
				PatrolAutonomyLevel:       PatrolAutonomyFull,
				PatrolAutopilotActivation: &invalidDigestActivation,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyApproval,
			wantCode:   unifiedresources.PatrolAutopilotStatusActivationDigestInvalid,
			wantActive: false,
		},
		{
			name: "full mode with activation bound to a different org is rejected as wrong-org",
			config: &AIConfig{
				PatrolAutonomyLevel:             PatrolAutonomyFull,
				PatrolAutopilotAcknowledgements: []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement},
				PatrolAutopilotActivation:       &activation,
			},
			orgID:      "org-b",
			wantLevel:  PatrolAutonomyApproval,
			wantCode:   unifiedresources.PatrolAutopilotStatusWrongOrg,
			wantActive: false,
		},
		{
			name: "full mode with matching activation and acknowledgement is active",
			config: &AIConfig{
				PatrolAutonomyLevel:             PatrolAutonomyFull,
				PatrolAutopilotAcknowledgements: []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement},
				PatrolAutopilotActivation:       &activation,
			},
			orgID:      "org-a",
			wantLevel:  PatrolAutonomyFull,
			wantCode:   unifiedresources.PatrolAutopilotStatusActive,
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, status := tt.config.GetEffectivePatrolAutonomyWithPolicy(tt.orgID, policy)
			assert.Equal(t, tt.wantLevel, level)
			assert.Equal(t, tt.wantCode, status.Code, "status code mismatch")
			assert.Equal(t, tt.wantActive, status.Active, "active flag mismatch")
			assert.Equal(t, policy.CurrentVersion, status.CurrentVersion, "status carries the server policy version")
		})
	}

	// The nil-receiver path is also reachable through the time-based wrapper and
	// must surface the server-owned contract scope/limits rather than an empty
	// status, so callers can render the acknowledgement surface.
	t.Run("nil receiver contract status carries current accepted scope and limits", func(t *testing.T) {
		var cfg *AIConfig
		level, status := cfg.GetEffectivePatrolAutonomyWithPolicy("org-a", policy)
		assert.Equal(t, PatrolAutonomyMonitor, level)
		assert.Equal(t, unifiedresources.PatrolAutopilotStatusAcknowledgementRequired, status.Code)
		assert.False(t, status.Active)
		expectedContract, ok := unifiedresources.PatrolAutopilotContractForVersion(policy.CurrentVersion)
		assert.True(t, ok, "current contract must be registered")
		assert.Equal(t, expectedContract.AcceptedScope, status.AcceptedScope)
		assert.Equal(t, expectedContract.AcceptedLimits, status.AcceptedLimits)
	})

	// GetEffectivePatrolAutonomy (the convenience wrapper) must agree with the
	// explicit-policy variant for the same wall-clock time, exercising the
	// CurrentPatrolAutopilotServerPolicy delegation.
	t.Run("time-based wrapper agrees with explicit policy for monitor mode", func(t *testing.T) {
		cfg := &AIConfig{PatrolAutonomyLevel: PatrolAutonomyMonitor}
		wrapperLevel, wrapperStatus := cfg.GetEffectivePatrolAutonomy("org-a", now)
		policyLevel, policyStatus := cfg.GetEffectivePatrolAutonomyWithPolicy("org-a", policy)
		assert.Equal(t, policyLevel, wrapperLevel)
		assert.Equal(t, PatrolAutonomyMonitor, wrapperLevel)
		assert.Equal(t, policyStatus.Code, wrapperStatus.Code)
		assert.Equal(t, unifiedresources.PatrolAutopilotStatusNotRequested, wrapperStatus.Code)
	})
}

// TestBranchCovIsPatrolFullModeActive covers the conjunction in
// IsPatrolFullModeActive (level == full AND status.Active): the nil-receiver
// path (which never dereferences the receiver thanks to the underlying nil
// guard), a non-full requested mode, a full request without activation, and a
// genuinely active full-mode activation.
func TestBranchCovIsPatrolFullModeActive(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)
	actor := configPatrolAutopilotActor("admin", "session:org-a", "org-a")
	acknowledgement, activation := configPatrolAutopilotEvidence(t, "ack-isfull-branchcov", actor, policy)

	t.Run("nil receiver returns false without panic", func(t *testing.T) {
		var cfg *AIConfig
		assert.False(t, cfg.IsPatrolFullModeActive("org-a", now))
	})

	t.Run("monitor mode is not full", func(t *testing.T) {
		cfg := &AIConfig{PatrolAutonomyLevel: PatrolAutonomyMonitor}
		assert.False(t, cfg.IsPatrolFullModeActive("org-a", now))
	})

	t.Run("full mode without activation is not active", func(t *testing.T) {
		cfg := &AIConfig{PatrolAutonomyLevel: PatrolAutonomyFull}
		assert.False(t, cfg.IsPatrolFullModeActive("org-a", now))
	})

	t.Run("full mode with matching activation is active", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAutonomyLevel:             PatrolAutonomyFull,
			PatrolAutopilotAcknowledgements: []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement},
			PatrolAutopilotActivation:       &activation,
		}
		assert.True(t, cfg.IsPatrolFullModeActive("org-a", now))
	})

	t.Run("full mode with activation bound to a different org is not active", func(t *testing.T) {
		cfg := &AIConfig{
			PatrolAutonomyLevel:             PatrolAutonomyFull,
			PatrolAutopilotAcknowledgements: []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement},
			PatrolAutopilotActivation:       &activation,
		}
		assert.False(t, cfg.IsPatrolFullModeActive("org-b", now))
	})
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
