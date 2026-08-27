package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func writeActiveRecoveryFixture(t *testing.T, dataDir string, alerts []*Alert) []byte {
	t.Helper()
	data, err := json.Marshal(alerts)
	if err != nil {
		t.Fatal(err)
	}
	alertsDir := filepath.Join(dataDir, "alerts")
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(alertsDir, "active-alerts.json"), data, alertsFilePerm); err != nil {
		t.Fatal(err)
	}
	return data
}

func durableRestoreAlert(startedAt time.Time) *Alert {
	acknowledgedAt := startedAt.Add(time.Hour)
	return &Alert{
		ID:           "durable-resource::metric-threshold:cpu",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "durable-resource",
		ResourceName: "Durable Resource",
		StartTime:    startedAt,
		LastSeen:     startedAt.Add(48 * time.Hour),
		Acknowledged: true,
		AckTime:      &acknowledgedAt,
		AckUser:      "operator",
	}
}

func TestSQLiteAuthorityRestoresLongRunningAlertOverStaleMirror(t *testing.T) {
	dataDir := t.TempDir()
	alert := durableRestoreAlert(time.Now().Add(-72 * time.Hour).UTC())
	writeActiveRecoveryFixture(t, dataDir, []*Alert{alert})

	seed := NewManagerWithDataDir(dataDir)
	seed.EnableEventLog()
	if !seed.activeStateAuthoritative.Load() {
		t.Fatal("SQLite active state did not become authoritative")
	}
	seed.Stop()

	// The recovery mirror is deliberately stale. A healthy initialized SQLite
	// projection must win, including for alerts older than the former 24h cap.
	writeActiveRecoveryFixture(t, dataDir, []*Alert{})
	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	restarted.EnableEventLog()

	restarted.mu.RLock()
	restored, ok := testLookupActiveAlert(t, restarted, alert.ID)
	restarted.mu.RUnlock()
	if !ok {
		t.Fatal("SQLite-authoritative long-running alert disappeared after restart")
	}
	if !restored.Acknowledged || restored.AckUser != "operator" {
		t.Fatalf("restored acknowledgement = %+v", restored)
	}
}

func TestAtomicResolutionBeatsStaleRecoveryMirror(t *testing.T) {
	dataDir := t.TempDir()
	alert := durableRestoreAlert(time.Now().Add(-2 * time.Hour).UTC())
	staleMirror := writeActiveRecoveryFixture(t, dataDir, []*Alert{alert})

	m := NewManagerWithDataDir(dataDir)
	m.EnableEventLog()
	m.mu.Lock()
	m.removeActiveAlertNoLock(alert.ID)
	m.recordAlertEvent(eventlog.TypeResolved, alert, alert.ID, "healthy", "Alert resolved.", nil)
	m.mu.Unlock()
	m.SetEventLog(nil)
	m.Stop()

	// Reintroduce the pre-resolution mirror to simulate a crash before the
	// asynchronous mirror checkpoint. The lifecycle transaction already
	// removed the occurrence from SQLite and must remain decisive.
	if err := os.WriteFile(filepath.Join(dataDir, "alerts", "active-alerts.json"), staleMirror, alertsFilePerm); err != nil {
		t.Fatal(err)
	}
	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	restarted.EnableEventLog()
	restarted.mu.RLock()
	_, ok := testLookupActiveAlert(t, restarted, alert.ID)
	restarted.mu.RUnlock()
	if ok {
		t.Fatal("stale JSON recovery mirror resurrected a resolved alert")
	}
}

func TestDegradedMarkerRepairsSQLiteFromRecoveryMirror(t *testing.T) {
	dataDir := t.TempDir()
	original := durableRestoreAlert(time.Now().Add(-2 * time.Hour).UTC())
	writeActiveRecoveryFixture(t, dataDir, []*Alert{original})

	seed := NewManagerWithDataDir(dataDir)
	seed.EnableEventLog()
	seed.Stop()

	recovered := original.Clone()
	recovered.ID = "recovered-resource::metric-threshold:memory"
	recovered.Type = "memory"
	recovered.ResourceID = "recovered-resource"
	recovered.ResourceName = "Recovered Resource"
	recovered.CanonicalSpecID = ""
	recovered.CanonicalState = ""
	writeActiveRecoveryFixture(t, dataDir, []*Alert{recovered})
	marker := filepath.Join(dataDir, "alerts", activeStateDegradedMarker)
	if err := os.WriteFile(marker, []byte("checkpoint failed\n"), alertsFilePerm); err != nil {
		t.Fatal(err)
	}

	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	restarted.EnableEventLog()
	restarted.mu.RLock()
	_, recoveredOK := testLookupActiveAlert(t, restarted, recovered.ID)
	_, originalOK := testLookupActiveAlert(t, restarted, original.ID)
	restarted.mu.RUnlock()
	if !recoveredOK {
		t.Fatal("degraded SQLite state was not repaired from the recovery mirror")
	}
	if originalOK {
		t.Fatal("repair retained the stale SQLite occurrence")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("degraded marker was not cleared after repair: %v", err)
	}
}

func TestRecreatedEventDatabaseImportsRecoveryMirror(t *testing.T) {
	dataDir := t.TempDir()
	alert := durableRestoreAlert(time.Now().Add(-3 * time.Hour).UTC())
	writeActiveRecoveryFixture(t, dataDir, []*Alert{alert})

	seed := NewManagerWithDataDir(dataDir)
	seed.EnableEventLog()
	seed.Stop()
	eventsPath := filepath.Join(dataDir, "alerts", eventLogDatabaseFileNameForTest)
	if err := os.Remove(eventsPath); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"-shm", "-wal"} {
		_ = os.Remove(eventsPath + suffix)
	}

	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	restarted.EnableEventLog()
	restarted.mu.RLock()
	_, ok := testLookupActiveAlert(t, restarted, alert.ID)
	restarted.mu.RUnlock()
	if !ok {
		t.Fatal("recreated event database did not import the active recovery mirror")
	}
}

func TestHealthySQLiteRecoversUnreadableMirrorDuringConstruction(t *testing.T) {
	dataDir := t.TempDir()
	alert := durableRestoreAlert(time.Now().Add(-4 * time.Hour).UTC())
	writeActiveRecoveryFixture(t, dataDir, []*Alert{alert})

	seed := NewManagerWithDataDir(dataDir, WithDurableAlertStore())
	if !seed.activeStateAuthoritative.Load() {
		t.Fatal("durable constructor did not establish SQLite authority")
	}
	seed.Stop()
	malformed := []byte(`{"truncated":`)
	if err := os.WriteFile(
		filepath.Join(dataDir, "alerts", "active-alerts.json"),
		malformed,
		alertsFilePerm,
	); err != nil {
		t.Fatal(err)
	}

	restarted := NewManagerWithDataDir(dataDir, WithDurableAlertStore())
	t.Cleanup(restarted.Stop)
	if !restarted.activeStateAuthoritative.Load() {
		t.Fatal("healthy SQLite did not recover from an unreadable mirror")
	}
	restarted.mu.RLock()
	_, ok := testLookupActiveAlert(t, restarted, alert.ID)
	restarted.mu.RUnlock()
	if !ok {
		t.Fatal("unreadable recovery mirror hid the SQLite-authoritative alert")
	}
	restarted.Stop()
	preserved, err := os.ReadFile(filepath.Join(dataDir, "alerts", "active-alerts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(malformed) {
		t.Fatalf("unreadable recovery mirror was overwritten: got %q", preserved)
	}
}

const eventLogDatabaseFileNameForTest = "events.db"
