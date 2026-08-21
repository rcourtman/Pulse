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
