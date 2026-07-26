package monitoring

import "time"

// Runtime-tunable polling cadence settings.
//
// Each tenant monitor polls against a detached DeepCopy of the base config,
// so mutating the base config after a system-settings save never reaches a
// running monitor (#1619). These setters push the saved values into live
// monitors directly, shadowing the config fields. The overrides live behind
// runtimePollingMu because polling goroutines read them every cycle while
// the settings API writes them.

// backupPollingEnabledSetting returns the effective EnableBackupPolling value.
func (m *Monitor) backupPollingEnabledSetting() bool {
	if m == nil || m.config == nil {
		return false
	}
	m.runtimePollingMu.RLock()
	override := m.backupPollingEnabledOverride
	m.runtimePollingMu.RUnlock()
	if override != nil {
		return *override
	}
	return m.config.EnableBackupPolling
}

// backupPollingIntervalSetting returns the effective BackupPollingInterval.
// Zero means cycle-based scheduling (BackupPollingCycles).
func (m *Monitor) backupPollingIntervalSetting() time.Duration {
	if m == nil || m.config == nil {
		return 0
	}
	m.runtimePollingMu.RLock()
	override := m.backupPollingIntervalOverride
	m.runtimePollingMu.RUnlock()
	if override != nil {
		return *override
	}
	return m.config.BackupPollingInterval
}

// pmgPollingIntervalSetting returns the effective PMGPollingInterval before
// clamping.
func (m *Monitor) pmgPollingIntervalSetting() time.Duration {
	if m == nil || m.config == nil {
		return 0
	}
	m.runtimePollingMu.RLock()
	override := m.pmgPollingIntervalOverride
	m.runtimePollingMu.RUnlock()
	if override != nil {
		return *override
	}
	return m.config.PMGPollingInterval
}

// SetBackupPollingEnabled toggles backup polling on the live monitor.
// Re-enabling clears the per-instance last-poll timestamps so the next
// polling cycle runs an immediate catch-up poll, matching the behavior of
// a full monitor reload.
func (m *Monitor) SetBackupPollingEnabled(enabled bool) {
	if m == nil {
		return
	}
	m.runtimePollingMu.Lock()
	wasEnabled := m.config != nil && m.config.EnableBackupPolling
	if m.backupPollingEnabledOverride != nil {
		wasEnabled = *m.backupPollingEnabledOverride
	}
	m.backupPollingEnabledOverride = &enabled
	m.runtimePollingMu.Unlock()

	if enabled && !wasEnabled {
		m.resetBackupPollTimestamps()
	}
}

// SetBackupPollingInterval updates the backup polling cadence on the live
// monitor. Lowering the interval (or switching from cycle-based scheduling
// to an interval) clears the per-instance last-poll timestamps so the next
// polling cycle runs an immediate catch-up poll rather than waiting out the
// remainder of the old interval.
func (m *Monitor) SetBackupPollingInterval(interval time.Duration) {
	if m == nil {
		return
	}
	if interval < 0 {
		interval = 0
	}
	m.runtimePollingMu.Lock()
	previous := time.Duration(0)
	if m.config != nil {
		previous = m.config.BackupPollingInterval
	}
	if m.backupPollingIntervalOverride != nil {
		previous = *m.backupPollingIntervalOverride
	}
	m.backupPollingIntervalOverride = &interval
	m.runtimePollingMu.Unlock()

	if interval > 0 && (previous <= 0 || interval < previous) {
		m.resetBackupPollTimestamps()
	}
}

// SetPMGPollingInterval updates the PMG polling cadence on the live monitor.
// The scheduler re-reads the base interval every cycle, so no further action
// is needed.
func (m *Monitor) SetPMGPollingInterval(interval time.Duration) {
	if m == nil || interval <= 0 {
		return
	}
	m.runtimePollingMu.Lock()
	m.pmgPollingIntervalOverride = &interval
	m.runtimePollingMu.Unlock()
}

// resetBackupPollTimestamps clears the per-instance backup poll timestamps
// so the next cycle polls immediately.
func (m *Monitor) resetBackupPollTimestamps() {
	m.mu.Lock()
	m.lastPVEBackupPoll = make(map[string]time.Time)
	m.lastPBSBackupPoll = make(map[string]time.Time)
	m.mu.Unlock()
}
