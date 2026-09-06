package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Missing capacity and a measured empty datastore both arrive with Usage=0.
// Only the latter may recover an incident, including after SQLite restore.
func TestPBSDatastoreMissingCapacityRestartCallbacks(t *testing.T) {
	dir := t.TempDir()
	fired, resolved := make(chan string, 16), make(chan string, 16)
	start := func() *Manager {
		m := NewManagerWithDataDir(dir)
		t.Cleanup(m.Stop)
		m.EnableEventLog()
		if !m.activeStateAuthoritative.Load() {
			t.Fatal("SQLite active state is not authoritative")
		}
		m.UpdateConfig(AlertConfig{Enabled: true, ActivationState: ActivationActive,
			StorageDefault: HysteresisThreshold{Trigger: 95, Clear: 90},
			Overrides:      map[string]ThresholdConfig{"pbs-primary/backups": {Usage: &HysteresisThreshold{Trigger: 80, Clear: 70}}},
		})
		disableTestTimeThresholds(m)
		m.SetAlertCallback(func(a *Alert) { fired <- a.ID })
		m.SetResolvedCallback(func(id string) { resolved <- id })
		return m
	}
	storage := models.Storage{ID: "pbs-primary-backups", AliasIDs: []string{"pbs-primary/backups"}, Name: "backups", Instance: "pbs-primary", Type: "pbs", Status: "online", Total: 1000, Used: 850, Free: 150, Usage: 85}
	id := canonicalMetricStateID(storage.ID, "usage")
	observe := func(m *Manager, s models.Storage) {
		for range 5 {
			m.CheckStorage(s)
		}
	}
	requireCallback := func(ch <-chan string) {
		t.Helper()
		select {
		case got := <-ch:
			if got != id {
				t.Fatalf("callback ID = %q, want %q", got, id)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("missing lifecycle callback")
		}
	}
	requireQuiet := func() {
		t.Helper()
		select {
		case got := <-fired:
			t.Fatalf("unexpected firing callback %q", got)
		case got := <-resolved:
			t.Fatalf("unexpected recovery callback %q", got)
		case <-time.After(50 * time.Millisecond):
		}
	}
	m := start()
	observe(m, storage)
	original := *testRequireActiveAlert(t, m, id)
	requireCallback(fired)
	missing := storage
	missing.Usage, missing.Total, missing.Used, missing.Free = 0, 0, 0, 0
	for _, restart := range []bool{false, true} {
		if restart {
			m.Stop()
			m = start()
		}
		observe(m, missing)
		if got := testRequireActiveAlert(t, m, id); !got.StartTime.Equal(original.StartTime) {
			t.Fatal("missing capacity replaced incident identity")
		}
		requireQuiet()
	}
	empty := storage
	empty.Usage, empty.Used, empty.Free = 0, 0, empty.Total
	observe(m, empty)
	if testHasActiveAlert(t, m, id) {
		t.Fatal("confirmed empty datastore did not recover")
	}
	requireCallback(resolved)
	m.Stop()
	m = start()
	observe(m, missing)
	if testHasActiveAlert(t, m, id) {
		t.Fatal("missing capacity resurrected resolved incident")
	}
	requireQuiet()
	observe(m, storage)
	if got := testRequireActiveAlert(t, m, id); !got.StartTime.After(original.StartTime) {
		t.Fatal("refire reused original incident")
	}
	requireCallback(fired)
	requireQuiet()
	for kind, want := range map[string]int{eventlog.TypeFired: 2, eventlog.TypeResolved: 1} {
		if got := len(queryAlertEvents(t, m, eventlog.Filter{Types: []string{kind}})); got != want {
			t.Fatalf("%s events = %d, want %d", kind, got, want)
		}
	}
}
