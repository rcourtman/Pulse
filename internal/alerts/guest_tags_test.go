package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIgnoredGuestPrefixesSkipsAlerts(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.IgnoredGuestPrefixes = []string{"ignore-", "test-"}
	m.config.GuestDefaults = ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	ignoredGuest := models.VM{
		ID:     "qemu/100",
		Name:   "ignore-vm1",
		Status: "running",
		CPU:    0.9, // 90%
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	monitoredGuest := models.VM{
		ID:     "qemu/101",
		Name:   "prod-vm1",
		Status: "running",
		CPU:    0.9, // 90%
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	// Check ignored guest
	m.CheckGuest(ignoredGuest, "node1")
	if len(m.activeAlerts) != 0 {
		t.Errorf("expected 0 active alerts for ignored prefix, got %d", len(m.activeAlerts))
	}

	// Check monitored guest
	m.CheckGuest(monitoredGuest, "node1")
	if len(m.activeAlerts) != 1 {
		t.Errorf("expected 1 active alert for monitored guest, got %d", len(m.activeAlerts))
	}
}

func TestGuestTagBlacklistSkipsAlerts(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.GuestTagBlacklist = []string{"maintenance", "backup"}
	m.config.GuestDefaults = ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	blacklistedGuest := models.VM{
		ID:     "qemu/100",
		Name:   "vm1",
		Status: "running",
		Tags:   []string{"production", "maintenance"}, // Has blacklisted tag
		CPU:    0.9,
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	normalGuest := models.VM{
		ID:     "qemu/101",
		Name:   "vm2",
		Status: "running",
		Tags:   []string{"production"},
		CPU:    0.9,
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	m.CheckGuest(blacklistedGuest, "node1")
	if len(m.activeAlerts) != 0 {
		t.Errorf("expected 0 active alerts for blacklisted guest, got %d", len(m.activeAlerts))
	}

	m.CheckGuest(normalGuest, "node1")
	if len(m.activeAlerts) != 1 {
		t.Errorf("expected 1 active alert for normal guest, got %d", len(m.activeAlerts))
	}
}

func TestGuestTagWhitelistSkipsAlertsIfTagMissing(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.GuestTagWhitelist = []string{"monitor-me", "production"}
	m.config.GuestDefaults = ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	// Guest without whitelisted tag
	unwatchedGuest := models.VM{
		ID:     "qemu/100",
		Name:   "vm1",
		Status: "running",
		Tags:   []string{"staging"},
		CPU:    0.9,
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	// Guest with whitelisted tag
	watchedGuest := models.VM{
		ID:     "qemu/101",
		Name:   "vm2",
		Status: "running",
		Tags:   []string{"staging", "monitor-me"},
		CPU:    0.9,
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	m.CheckGuest(unwatchedGuest, "node1")
	if len(m.activeAlerts) != 0 {
		t.Errorf("expected 0 active alerts for guest missing whitelist tag, got %d", len(m.activeAlerts))
	}

	m.CheckGuest(watchedGuest, "node1")
	if len(m.activeAlerts) != 1 {
		t.Errorf("expected 1 active alert for whitelisted guest, got %d", len(m.activeAlerts))
	}
}

func TestGuestIgnoredPrefixClearsActiveAlerts(t *testing.T) {
	m := newTestManager(t)
	m.ClearActiveAlerts()

	m.mu.Lock()
	m.config.GuestDefaults = ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 80, Clear: 70}}
	m.config.TimeThreshold = 0
	m.config.TimeThresholds = map[string]int{}
	m.mu.Unlock()

	guest := models.VM{
		ID:     "qemu/100",
		Name:   "vm1",
		Status: "running",
		CPU:    0.9,
		Memory: models.Memory{Usage: 50},
		Disk:   models.Disk{Usage: 50},
	}

	// 1. Create an alert
	m.CheckGuest(guest, "node1")
	if len(m.activeAlerts) != 1 {
		t.Fatalf("setup failed: expected 1 alert, got %d", len(m.activeAlerts))
	}

	// 2. Ignore the prefix
	m.mu.Lock()
	m.config.IgnoredGuestPrefixes = []string{"vm"}
	m.mu.Unlock()

	// 3. Check guest again
	m.CheckGuest(guest, "node1")

	if len(m.activeAlerts) != 0 {
		t.Errorf("expected 0 alerts after adding ignored prefix, got %d", len(m.activeAlerts))
	}
}
