package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A resolved occurrence followed by a fresh firing outside the refire
// cooldown must produce a second history row, not fold into the first.
// Refs #1497 follow-up: history frozen at the v6.2.0 upgrade date.
func TestHistoryNewOccurrenceAfterResolveAppendsRow(t *testing.T) {
	m := newTestManager(t)

	fire := func() {
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
		m.checkGuestPoweredOff("vm100", "TestVM", "pve-node1", "pve-instance", "VM", false)
	}

	fire()
	if got := len(m.GetAlertHistory(0)); got != 1 {
		t.Fatalf("expected 1 history row after first firing, got %d", got)
	}

	m.clearGuestPoweredOffAlert("vm100", "TestVM")

	// Age the resolved record out of the refire cooldown so the second
	// firing is a genuinely new occurrence, as it is a day later in prod.
	m.resolvedMutex.Lock()
	for key, resolved := range m.recentlyResolved {
		resolved.ResolvedTime = time.Now().Add(-2 * recentlyResolvedRetention)
		m.recentlyResolved[key] = resolved
	}
	m.resolvedMutex.Unlock()
	// The reducer core keeps its own resolved ledger for refire decisions;
	// age it in step with the manager's records.
	m.mu.Lock()
	m.core.ShiftResolved(-2 * reducer.RefireRetention)
	m.mu.Unlock()

	fire()

	history := m.GetAlertHistory(0)
	if len(history) != 2 {
		for i, alert := range history {
			t.Logf("history[%d]: id=%s start=%s record=%+v", i, alert.ID, alert.StartTime, alert.OperationalRecord)
		}
		t.Fatalf("expected 2 history rows after resolve and refire, got %d", len(history))
	}
}

// Metric-path variant: a resolved CPU occurrence followed by a fresh firing
// outside the refire cooldown must append a second history row. Before the
// fix, setActiveAlertNoLock stamped the new occurrence's open record onto
// the previous occurrence's resolved row and the history dedup then merged
// every recurrence into that first row forever. Refs #1497.
func TestHistoryMetricNewOccurrenceAfterResolveAppendsRow(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	guest := func(cpu float64) models.VM {
		return models.VM{
			ID:       "vm-hist-1",
			Name:     "hist-vm",
			Node:     "node1",
			Instance: "inst1",
			Status:   "running",
			CPU:      cpu,
			Memory:   models.Memory{Usage: 10},
			Disk:     models.Disk{Usage: 10},
		}
	}

	m.CheckGuest(guest(1.0), "inst1")
	if got := len(m.GetAlertHistory(0)); got != 1 {
		t.Fatalf("expected 1 history row after first firing, got %d", got)
	}

	m.CheckGuest(guest(0.05), "inst1")

	m.resolvedMutex.Lock()
	if len(m.recentlyResolved) == 0 {
		m.resolvedMutex.Unlock()
		t.Fatal("expected the alert to resolve after CPU dropped")
	}
	for key, resolved := range m.recentlyResolved {
		resolved.ResolvedTime = time.Now().Add(-2 * recentlyResolvedRetention)
		m.recentlyResolved[key] = resolved
	}
	m.resolvedMutex.Unlock()

	// A next-day occurrence is outside the recent-alert suppression window
	// and carries a different observed value.
	m.mu.Lock()
	for key, recent := range m.recentAlerts {
		aged := *recent
		aged.StartTime = aged.StartTime.Add(-24 * time.Hour)
		m.recentAlerts[key] = &aged
	}
	m.mu.Unlock()

	// Age the stored resolution as well so the recurrence-continuity window
	// (flap dedup) does not apply, as it would not a day later.
	m.historyManager.mu.Lock()
	for i := range m.historyManager.history {
		if record := m.historyManager.history[i].Alert.OperationalRecord; record != nil && record.ResolvedAt != nil {
			aged := record.ResolvedAt.Add(-24 * time.Hour)
			record.ResolvedAt = &aged
		}
	}
	m.historyManager.mu.Unlock()

	m.CheckGuest(guest(0.93), "inst1")

	history := m.GetAlertHistory(0)
	if len(history) != 2 {
		for i, alert := range history {
			t.Logf("history[%d]: id=%s start=%s record=%+v", i, alert.ID, alert.StartTime, alert.OperationalRecord)
		}
		t.Fatalf("expected 2 history rows after resolve and refire, got %d", len(history))
	}
}
