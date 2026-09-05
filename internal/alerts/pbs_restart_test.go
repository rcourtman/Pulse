package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise real threshold transitions across disk-backed manager lifetimes,
// rather than restoring manually seeded active-alert fixtures.
func TestPBSMissingMetricsAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	start := func() *Manager {
		m := NewManagerWithDataDir(dataDir)
		t.Cleanup(m.Stop)
		m.EnableEventLog()
		if !m.activeStateAuthoritative.Load() {
			t.Fatal("SQLite active state is not authoritative")
		}
		m.UpdateConfig(AlertConfig{Enabled: true, PBSDefaults: ThresholdConfig{
			CPU:    &HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &HysteresisThreshold{Trigger: 85, Clear: 80},
		}})
		disableTestTimeThresholds(m)
		return m
	}
	assertEvents := func(m *Manager, fired, resolved int) {
		t.Helper()
		for kind, want := range map[string]int{eventlog.TypeFired: fired, eventlog.TypeResolved: resolved} {
			if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{kind}})); got != want {
				t.Fatalf("%s events = %d, want %d", kind, got, want)
			}
		}
	}

	m := start()
	p := models.PBSInstance{ID: "pbs-restart", Name: "backup", Status: "online", CPU: 95, Memory: 95}
	m.CheckPBS(p)
	initial := m.GetActiveAlerts()
	if len(initial) != 2 {
		t.Fatalf("initial incidents = %d, want 2", len(initial))
	}
	assertEvents(m, 2, 0)
	m.Stop()

	m = start()
	p.CPU, p.Memory, p.NodeMetricsUnavailable = 0, 0, true
	for range 5 {
		m.CheckPBS(p)
	}
	restored := m.GetActiveAlerts()
	if len(restored) != len(initial) {
		t.Fatalf("missing metrics after restart retained %d incidents, want 2", len(restored))
	}
	for _, before := range initial {
		found := false
		for _, after := range restored {
			if before.ID == after.ID && before.StartTime.Equal(after.StartTime) {
				found = true
			}
		}
		if !found {
			t.Fatalf("incident identity/start time changed across restart: %s", before.ID)
		}
	}
	assertEvents(m, 2, 0)

	p.NodeMetricsUnavailable = false
	for range 5 {
		m.CheckPBS(p)
	}
	if got := len(m.GetActiveAlerts()); got != 0 {
		t.Fatalf("measured zero retained %d incidents", got)
	}
	assertEvents(m, 2, 2)
	m.Stop()

	m = start()
	if got := len(m.GetActiveAlerts()); got != 0 {
		t.Fatalf("second restart resurrected %d resolved incidents", got)
	}
	assertEvents(m, 2, 2)
	if got := len(m.GetAlertHistory(10)); got != 2 {
		t.Fatalf("durable incident history = %d, want 2", got)
	}
}
