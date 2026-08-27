package alerts

// Parity: the event-log history projection must reproduce the JSON-backed
// history manager's output for the same lifecycle run
// (docs/ALERT_ENGINE_EVOLUTION.md — the event log becomes the sole history
// authority only once this holds). The JSON history characterizes the
// projection: a divergence is a projection bug unless investigation proves
// a history-manager defect, which is then fixed first.

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func newHistoryParityManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory event log: %v", err)
	}
	m.SetEventLog(store)
	t.Cleanup(func() { m.SetEventLog(nil) })
	return m
}

// assertHistoryParity compares the projection against the JSON history on
// the fields the history API exposes.
func assertHistoryParity(t *testing.T, m *Manager) {
	t.Helper()
	legacy := m.GetAlertHistory(0)
	projected, ok := m.AlertHistoryFromEvents(time.Time{}, 0)
	if !ok {
		t.Fatal("projection unavailable despite an enabled event log")
	}
	if len(projected) != len(legacy) {
		t.Fatalf("projection has %d entries, JSON history has %d\nprojected: %+v\nlegacy: %+v",
			len(projected), len(legacy), summarizeHistory(projected), summarizeHistory(legacy))
	}
	for i := range legacy {
		l, p := legacy[i], projected[i]
		if l.ID != p.ID {
			t.Errorf("entry %d: ID diverges: legacy=%q projected=%q", i, l.ID, p.ID)
			continue
		}
		if l.ResourceID != p.ResourceID {
			t.Errorf("entry %d (%s): ResourceID legacy=%q projected=%q", i, l.ID, l.ResourceID, p.ResourceID)
		}
		if l.Level != p.Level {
			t.Errorf("entry %d (%s): Level legacy=%q projected=%q", i, l.ID, l.Level, p.Level)
		}
		if !l.StartTime.Truncate(time.Second).Equal(p.StartTime.Truncate(time.Second)) {
			t.Errorf("entry %d (%s): StartTime legacy=%v projected=%v", i, l.ID, l.StartTime, p.StartTime)
		}
		if l.Acknowledged != p.Acknowledged {
			t.Errorf("entry %d (%s): Acknowledged legacy=%v projected=%v", i, l.ID, l.Acknowledged, p.Acknowledged)
		}
		if l.Value != p.Value {
			t.Errorf("entry %d (%s): Value legacy=%v projected=%v", i, l.ID, l.Value, p.Value)
		}
	}
}

func summarizeHistory(entries []Alert) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.ID+"@"+e.StartTime.Format(time.RFC3339))
	}
	return out
}

func TestHistoryProjectionParityActiveAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-1", 95)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityResolvedAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()
	contractRaiseGuestCPUAlert(t, m, "hp-vm-2", 95)
	// A clearing observation resolves through the real evaluation path.
	m.checkMetric("hp-vm-2", "Contract VM hp-vm-2", "node1", "inst1", "guest", "cpu", 50, cfg.GuestDefaults.CPU, nil)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityAcknowledgedAlert(t *testing.T) {
	m := newHistoryParityManager(t)
	alert := contractRaiseGuestCPUAlert(t, m, "hp-vm-3", 95)
	if err := m.AcknowledgeAlert(alert.ID, "parity-user"); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParityMultipleResources(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()
	contractRaiseGuestCPUAlert(t, m, "hp-vm-4", 92)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-5", 93)
	contractRaiseGuestCPUAlert(t, m, "hp-vm-6", 94)
	m.checkMetric("hp-vm-5", "Contract VM hp-vm-5", "node1", "inst1", "guest", "cpu", 40, cfg.GuestDefaults.CPU, nil)
	assertHistoryParity(t, m)
}

func TestHistoryProjectionParitySecondOccurrence(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()

	contractRaiseGuestCPUAlert(t, m, "hp-vm-7", 95)
	m.checkMetric("hp-vm-7", "Contract VM hp-vm-7", "node1", "inst1", "guest", "cpu", 40, cfg.GuestDefaults.CPU, nil)

	// Age the resolved occurrence past the refire window in both the
	// manager's ledger and the core so the next trigger opens a genuinely
	// new occurrence (a new history row) instead of a reactivation.
	m.resolvedMutex.Lock()
	for key, record := range m.recentlyResolved {
		record.ResolvedTime = record.ResolvedTime.Add(-10 * time.Minute)
		m.recentlyResolved[key] = record
	}
	m.resolvedMutex.Unlock()
	m.mu.Lock()
	m.core.ShiftResolved(-10 * time.Minute)
	m.mu.Unlock()

	contractRaiseGuestCPUAlert(t, m, "hp-vm-7", 97)
	assertHistoryParity(t, m)
}
