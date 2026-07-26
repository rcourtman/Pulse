package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Regression test for #1619: monitors poll against a detached DeepCopy of the
// base config, so mutating the base config after a settings save must not be
// relied on — the live setters have to reach the running monitor directly.
func TestSetBackupPollingIntervalReachesLiveMonitor(t *testing.T) {
	now := time.Now()
	base := &config.Config{
		EnableBackupPolling:   true,
		BackupPollingInterval: time.Hour,
	}
	m := &Monitor{
		config:            base.DeepCopy(),
		lastPVEBackupPoll: map[string]time.Time{"pve1": now.Add(-30 * time.Minute)},
		lastPBSBackupPoll: map[string]time.Time{"pbs1": now.Add(-30 * time.Minute)},
	}

	last := m.lastPVEBackupPoll["pve1"]
	if should, _, _ := m.shouldRunBackupPoll(last, now); should {
		t.Fatal("expected no poll 30m into a 1h interval")
	}

	// Mutating the base config (what the settings handler used to do) never
	// reaches the monitor's detached copy.
	base.BackupPollingInterval = 10 * time.Minute
	if should, _, _ := m.shouldRunBackupPoll(last, now); should {
		t.Fatal("base config mutation unexpectedly reached the detached monitor config")
	}

	// The live setter must take effect without a monitor reload.
	m.SetBackupPollingInterval(10 * time.Minute)
	if len(m.lastPVEBackupPoll) != 0 || len(m.lastPBSBackupPoll) != 0 {
		t.Fatal("lowering the interval should clear last-poll timestamps for an immediate catch-up poll")
	}
	last = m.lastPVEBackupPoll["pve1"] // zero after reset
	if should, _, _ := m.shouldRunBackupPoll(last, now); !should {
		t.Fatal("expected immediate catch-up poll after lowering the interval")
	}
}

func TestSetBackupPollingIntervalRaisingKeepsTimestamps(t *testing.T) {
	now := time.Now()
	last := now.Add(-5 * time.Minute)
	m := &Monitor{
		config: &config.Config{
			EnableBackupPolling:   true,
			BackupPollingInterval: 10 * time.Minute,
		},
		lastPVEBackupPoll: map[string]time.Time{"pve1": last},
		lastPBSBackupPoll: map[string]time.Time{},
	}

	m.SetBackupPollingInterval(time.Hour)
	if got := m.lastPVEBackupPoll["pve1"]; !got.Equal(last) {
		t.Fatal("raising the interval should keep last-poll timestamps")
	}
	if should, _, _ := m.shouldRunBackupPoll(last, now); should {
		t.Fatal("expected no poll 5m into the raised 1h interval")
	}
}

func TestSetBackupPollingEnabledReachesLiveMonitor(t *testing.T) {
	now := time.Now()
	m := &Monitor{
		config: &config.Config{
			EnableBackupPolling:   true,
			BackupPollingInterval: time.Hour,
		},
		lastPVEBackupPoll: map[string]time.Time{"pve1": now.Add(-30 * time.Minute)},
		lastPBSBackupPoll: map[string]time.Time{},
	}

	m.SetBackupPollingEnabled(false)
	if should, reason, _ := m.shouldRunBackupPoll(time.Time{}, now); should || reason != "backup polling globally disabled" {
		t.Fatalf("expected backup polling disabled, got should=%v reason=%q", should, reason)
	}

	m.SetBackupPollingEnabled(true)
	if len(m.lastPVEBackupPoll) != 0 {
		t.Fatal("re-enabling backup polling should clear last-poll timestamps")
	}
	if should, _, _ := m.shouldRunBackupPoll(m.lastPVEBackupPoll["pve1"], now); !should {
		t.Fatal("expected immediate poll after re-enabling backup polling")
	}
}

func TestSetPBSPollingIntervalReachesLiveMonitor(t *testing.T) {
	base := &config.Config{PBSPollingInterval: 2 * time.Minute}
	m := &Monitor{config: base.DeepCopy()}

	if got := m.baseIntervalForInstanceType(InstanceTypePBS); got != 2*time.Minute {
		t.Fatalf("expected 2m from config, got %v", got)
	}

	m.SetPBSPollingInterval(5 * time.Minute)
	if got := m.baseIntervalForInstanceType(InstanceTypePBS); got != 5*time.Minute {
		t.Fatalf("expected 5m after live update, got %v", got)
	}

	// Clamping still applies to live updates.
	m.SetPBSPollingInterval(2 * time.Hour)
	if got := m.baseIntervalForInstanceType(InstanceTypePBS); got != time.Hour {
		t.Fatalf("expected live update clamped to 1h, got %v", got)
	}
}

func TestSetPMGPollingIntervalReachesLiveMonitor(t *testing.T) {
	base := &config.Config{PMGPollingInterval: 2 * time.Minute}
	m := &Monitor{config: base.DeepCopy()}

	if got := m.baseIntervalForInstanceType(InstanceTypePMG); got != 2*time.Minute {
		t.Fatalf("expected 2m from config, got %v", got)
	}

	m.SetPMGPollingInterval(5 * time.Minute)
	if got := m.baseIntervalForInstanceType(InstanceTypePMG); got != 5*time.Minute {
		t.Fatalf("expected 5m after live update, got %v", got)
	}

	// Clamping still applies to live updates.
	m.SetPMGPollingInterval(2 * time.Hour)
	if got := m.baseIntervalForInstanceType(InstanceTypePMG); got != time.Hour {
		t.Fatalf("expected live update clamped to 1h, got %v", got)
	}
}
