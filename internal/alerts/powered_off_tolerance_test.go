package alerts

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type poweredOffTestClock struct {
	wall time.Time
	tick time.Duration
}

func newPoweredOffTestManager(t *testing.T, start time.Time) (*Manager, *poweredOffTestClock) {
	t.Helper()
	manager := NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	clock := &poweredOffTestClock{wall: start}
	manager.now = func() time.Time { return clock.wall }
	manager.intentClock = func() time.Duration { return clock.tick }
	return manager, clock
}

func installPoweredOffPolicy(t *testing.T, manager *Manager, graceSeconds int, backup *BackupOfflineIntentPolicy) {
	t.Helper()
	document := NewAlertIntentPolicyDocument()
	document.ResourceTypes["guest"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {
			GraceSeconds:  intPointer(graceSeconds),
			BackupOffline: backup,
		},
	}
	if err := manager.LoadIntentPolicies(document); err != nil {
		t.Fatalf("LoadIntentPolicies() error = %v", err)
	}
}

func hasPoweredOffAlert(manager *Manager, resourceID string) bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	_, exists := manager.getActiveAlertNoLock(canonicalPoweredStateStateID(resourceID))
	return exists
}

func TestPoweredOffToleranceUsesElapsedTimeForVMAndLXC(t *testing.T) {
	for _, test := range []struct {
		name         string
		resourceID   string
		resourceType string
	}{
		{name: "vm", resourceID: "vm:101", resourceType: "VM"},
		{name: "lxc", resourceID: "lxc:202", resourceType: "Container"},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
			manager, clock := newPoweredOffTestManager(t, start)
			installPoweredOffPolicy(t, manager, 300, nil)

			for range 20 {
				manager.checkGuestPoweredOff(test.resourceID, test.name, "node-a", "pve-a", test.resourceType, false)
			}
			if hasPoweredOffAlert(manager, test.resourceID) {
				t.Fatal("duplicate reports activated the alert before the duration elapsed")
			}
			manager.mu.RLock()
			_, counted := manager.offlineConfirmations[test.resourceID]
			manager.mu.RUnlock()
			if counted {
				t.Fatal("duration-based powered-off policy must not retain poll confirmations")
			}

			clock.wall, clock.tick = start.Add(299*time.Second), 299*time.Second
			manager.checkGuestPoweredOff(test.resourceID, test.name, "node-a", "pve-a", test.resourceType, false)
			if hasPoweredOffAlert(manager, test.resourceID) {
				t.Fatal("alert activated before the configured duration")
			}

			clock.wall, clock.tick = start.Add(300*time.Second), 300*time.Second
			manager.checkGuestPoweredOff(test.resourceID, test.name, "node-a", "pve-a", test.resourceType, false)
			if !hasPoweredOffAlert(manager, test.resourceID) {
				t.Fatal("alert did not activate at the configured duration")
			}
		})
	}
}

func TestPoweredOffToleranceZeroAndInheritance(t *testing.T) {
	start := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
	manager, _ := newPoweredOffTestManager(t, start)
	document := NewAlertIntentPolicyDocument()
	document.ResourceTypes["guest"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(300)},
	}
	document.Resources["vm:zero"] = map[string]AlertIntentRule{
		string(AlertIntentSignalOffline): {GraceSeconds: intPointer(0)},
	}
	if err := manager.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}

	manager.checkGuestPoweredOff("vm:zero", "zero", "node-a", "pve-a", "VM", false)
	if !hasPoweredOffAlert(manager, "vm:zero") {
		t.Fatal("explicit zero did not activate on the first stopped observation")
	}
	manager.checkGuestPoweredOff("vm:inherit", "inherit", "node-a", "pve-a", "VM", false)
	if hasPoweredOffAlert(manager, "vm:inherit") {
		t.Fatal("inherited guest tolerance was not applied")
	}
}

func TestPoweredOffToleranceIgnoresWallClockChanges(t *testing.T) {
	start := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	manager, clock := newPoweredOffTestManager(t, start)
	installPoweredOffPolicy(t, manager, 300, nil)

	manager.checkGuestPoweredOff("vm:clock", "clock", "node-a", "pve-a", "VM", false)
	clock.wall = start.Add(24 * time.Hour)
	manager.checkGuestPoweredOff("vm:clock", "clock", "node-a", "pve-a", "VM", false)
	if hasPoweredOffAlert(manager, "vm:clock") {
		t.Fatal("forward wall-clock jump activated the alert")
	}

	clock.wall = start.Add(-24 * time.Hour)
	clock.tick = 300 * time.Second
	manager.checkGuestPoweredOff("vm:clock", "clock", "node-a", "pve-a", "VM", false)
	if !hasPoweredOffAlert(manager, "vm:clock") {
		t.Fatal("monotonic elapsed time did not activate after a backward wall-clock jump")
	}
}

func TestPoweredOffToleranceClearsOnRecoveryAndSuppression(t *testing.T) {
	start := time.Date(2026, 7, 24, 11, 0, 0, 0, time.UTC)
	manager, clock := newPoweredOffTestManager(t, start)
	installPoweredOffPolicy(t, manager, 300, nil)

	manager.checkGuestPoweredOff("vm:flap", "flap", "node-a", "pve-a", "VM", false)
	clock.wall, clock.tick = start.Add(200*time.Second), 200*time.Second
	manager.clearGuestPoweredOffAlert("vm:flap", "flap")
	clock.wall, clock.tick = start.Add(300*time.Second), 300*time.Second
	manager.checkGuestPoweredOff("vm:flap", "flap", "node-a", "pve-a", "VM", false)
	if hasPoweredOffAlert(manager, "vm:flap") {
		t.Fatal("recovery did not reset the powered-off duration")
	}

	if !manager.suppressGuestAlerts("vm:flap") {
		t.Fatal("suppression did not report clearing pending intent state")
	}
	manager.mu.RLock()
	pending := len(manager.intentPending)
	ticks := len(manager.intentRuntimeTicks)
	manager.mu.RUnlock()
	if pending != 0 || ticks != 0 {
		t.Fatalf("suppression left pending state: pending=%d ticks=%d", pending, ticks)
	}
}

func TestPoweredOffToleranceRecoveryNotifiesAfterActivation(t *testing.T) {
	start := time.Date(2026, 7, 24, 11, 30, 0, 0, time.UTC)
	manager, clock := newPoweredOffTestManager(t, start)
	installPoweredOffPolicy(t, manager, 60, nil)
	resolved := make(chan string, 1)
	manager.SetResolvedCallback(func(alertID string) {
		resolved <- alertID
	})

	manager.checkGuestPoweredOff("vm:recover", "recover", "node-a", "pve-a", "VM", false)
	clock.wall, clock.tick = start.Add(60*time.Second), 60*time.Second
	manager.checkGuestPoweredOff("vm:recover", "recover", "node-a", "pve-a", "VM", false)
	if !hasPoweredOffAlert(manager, "vm:recover") {
		t.Fatal("powered-off alert did not activate before recovery")
	}

	manager.clearGuestPoweredOffAlert("vm:recover", "recover")
	select {
	case alertID := <-resolved:
		if alertID == "" {
			t.Fatal("recovery callback omitted the alert identity")
		}
	case <-time.After(time.Second):
		t.Fatal("activated powered-off alert did not emit a recovery callback")
	}
	if hasPoweredOffAlert(manager, "vm:recover") {
		t.Fatal("powered-off alert remained active after recovery")
	}
}

func TestPoweredOffBackupDeferralHasHardCap(t *testing.T) {
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	manager, clock := newPoweredOffTestManager(t, start)
	installPoweredOffPolicy(t, manager, 300, &BackupOfflineIntentPolicy{
		Enabled: true, PostGraceSeconds: 60, MaxDeferralSeconds: 600,
	})
	backup := BackupIntentContext{Active: true, Evidence: "fresh_task"}

	manager.checkGuestPoweredOffWithThresholdsAndIntent("vm:backup", "backup", "node-a", "pve-a", "VM", manager.config.GuestDefaults, false, backup)
	clock.wall, clock.tick = start.Add(599*time.Second), 599*time.Second
	manager.checkGuestPoweredOffWithThresholdsAndIntent("vm:backup", "backup", "node-a", "pve-a", "VM", manager.config.GuestDefaults, false, backup)
	if hasPoweredOffAlert(manager, "vm:backup") {
		t.Fatal("backup deferral activated before its hard cap")
	}

	clock.wall, clock.tick = start.Add(600*time.Second), 600*time.Second
	manager.checkGuestPoweredOffWithThresholdsAndIntent("vm:backup", "backup", "node-a", "pve-a", "VM", manager.config.GuestDefaults, false, backup)
	if !hasPoweredOffAlert(manager, "vm:backup") {
		t.Fatal("backup evidence hid an outage beyond the hard cap")
	}
}

func TestPoweredOffToleranceConcurrentReportsActivateOnce(t *testing.T) {
	start := time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC)
	manager, _ := newPoweredOffTestManager(t, start)
	installPoweredOffPolicy(t, manager, 0, nil)
	manager.config.ActivationState = ActivationActive
	var notifications atomic.Int32
	manager.SetAlertCallback(func(*Alert) {
		notifications.Add(1)
	})

	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			manager.checkGuestPoweredOff("vm:race", "race", "node-a", "pve-a", "VM", false)
		}()
	}
	workers.Wait()

	if !hasPoweredOffAlert(manager, "vm:race") {
		t.Fatal("concurrent reports did not activate the alert")
	}
	if got := notifications.Load(); got != 1 {
		t.Fatalf("notification count = %d, want 1", got)
	}
	history := manager.GetAlertHistory(10)
	if len(history) != 1 {
		t.Fatalf("history entries = %d, want 1", len(history))
	}
}
