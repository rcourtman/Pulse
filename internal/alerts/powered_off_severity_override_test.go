package alerts

import (
	"testing"
)

// Per-guest powered-off severity must win over the global default in both
// directions, and a guest override that never set a severity must keep
// following the global default instead of being stamped with "warning" by
// override normalization. Refs #1738.
func TestPoweredOffSeverityOverrideResolution(t *testing.T) {
	run := func(t *testing.T, global, override, want AlertLevel) {
		m := newTestManager(t)

		cfg := m.GetConfig()
		cfg.GuestDefaults.PoweredOffSeverity = global
		if cfg.Overrides == nil {
			cfg.Overrides = map[string]ThresholdConfig{}
		}
		cfg.Overrides["vm100"] = ThresholdConfig{PoweredOffSeverity: override}
		m.UpdateConfig(cfg)

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)

		m.mu.RLock()
		alert := testRequireActiveAlert(t, m, "guest-powered-off-vm100")
		m.mu.RUnlock()

		if alert.Level != want {
			t.Fatalf("global=%s override=%s: expected %s, got %s", global, override, want, alert.Level)
		}
	}

	t.Run("override critical beats global warning", func(t *testing.T) {
		run(t, AlertLevelWarning, AlertLevelCritical, AlertLevelCritical)
	})
	t.Run("override warning beats global critical", func(t *testing.T) {
		run(t, AlertLevelCritical, AlertLevelWarning, AlertLevelWarning)
	})
	t.Run("unset override keeps global critical", func(t *testing.T) {
		run(t, AlertLevelCritical, "", AlertLevelCritical)
	})
}

// A per-guest severity override must re-enable powered-off alerts when the
// global default has connectivity alerts disabled, while a per-guest "off"
// (DisableConnectivity=true) must stay off regardless of any severity the
// override also carries. Refs #1738.
func TestPoweredOffOverrideReenablesDisabledDefault(t *testing.T) {
	setup := func(t *testing.T, override ThresholdConfig) *Manager {
		m := newTestManager(t)

		cfg := m.GetConfig()
		cfg.GuestDefaults.DisableConnectivity = true
		cfg.GuestDefaults.PoweredOffSeverity = AlertLevelCritical
		if cfg.Overrides == nil {
			cfg.Overrides = map[string]ThresholdConfig{}
		}
		cfg.Overrides["vm100"] = override
		m.UpdateConfig(cfg)

		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		return m
	}

	t.Run("severity override re-enables offline alerts", func(t *testing.T) {
		m := setup(t, ThresholdConfig{PoweredOffSeverity: AlertLevelWarning})

		m.mu.RLock()
		alert := testRequireActiveAlert(t, m, "guest-powered-off-vm100")
		m.mu.RUnlock()

		if alert.Level != AlertLevelWarning {
			t.Fatalf("expected %s, got %s", AlertLevelWarning, alert.Level)
		}
	})

	t.Run("per-guest off wins over its own stored severity", func(t *testing.T) {
		m := setup(t, ThresholdConfig{DisableConnectivity: true, PoweredOffSeverity: AlertLevelWarning})

		m.mu.RLock()
		_, exists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if exists {
			t.Fatal("expected no powered-off alert for per-guest disabled connectivity")
		}
	})

	t.Run("override without severity keeps disabled default", func(t *testing.T) {
		m := setup(t, ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 90, Clear: 85}})

		m.mu.RLock()
		_, exists := m.activeAlerts["guest-powered-off-vm100"]
		m.mu.RUnlock()

		if exists {
			t.Fatal("expected no powered-off alert when override never touched the offline control")
		}
	})
}
