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

func TestDurableLifecycleFailureSynchronouslyCheckpointsRecoveryMirror(t *testing.T) {
	t.Run("fired while manager lock is held", func(t *testing.T) {
		dataDir := t.TempDir()
		m := NewManagerWithDataDir(dataDir)
		m.EnableEventLog()
		store := m.eventLogStore()
		if store == nil {
			t.Fatal("event log was not enabled")
		}
		store.Close()

		alert := durableRestoreAlert(time.Now().UTC())
		m.mu.Lock()
		m.setActiveAlertNoLock(alert.ID, alert)
		m.recordAlertEvent(eventlog.TypeFired, alert, alert.ID, "threshold", "Alert fired.", nil)
		m.mu.Unlock()

		data, err := os.ReadFile(filepath.Join(dataDir, "alerts", "active-alerts.json"))
		if err != nil {
			t.Fatalf("read recovery mirror: %v", err)
		}
		var recovered []*Alert
		if err := json.Unmarshal(data, &recovered); err != nil {
			t.Fatalf("decode recovery mirror: %v", err)
		}
		if len(recovered) != 1 || recovered[0].ID != alert.ID {
			t.Fatalf("recovery mirror after failed fire = %+v", recovered)
		}
		if _, err := os.Stat(filepath.Join(dataDir, "alerts", activeStateDegradedMarker)); err != nil {
			t.Fatalf("degraded marker missing after failed fire: %v", err)
		}
		markerData, err := os.ReadFile(filepath.Join(dataDir, "alerts", activeStateDegradedMarker))
		if err != nil {
			t.Fatalf("read degraded recovery marker: %v", err)
		}
		var envelope activeStateRecoveryEnvelope
		if err := json.Unmarshal(markerData, &envelope); err != nil {
			t.Fatalf("decode degraded recovery marker: %v", err)
		}
		if envelope.SchemaVersion != activeStateRecoverySchemaVersion || len(envelope.Alerts) != 1 || envelope.Alerts[0].ID != alert.ID {
			t.Fatalf("degraded recovery marker = %+v", envelope)
		}

		m.SetEventLog(nil)
		m.Stop()
		if err := os.WriteFile(
			filepath.Join(dataDir, "alerts", "active-alerts.json"),
			[]byte(`{"truncated":`),
			alertsFilePerm,
		); err != nil {
			t.Fatalf("corrupt recovery mirror to isolate marker recovery: %v", err)
		}
		restarted := NewManagerWithDataDir(dataDir)
		t.Cleanup(restarted.Stop)
		restarted.EnableEventLog()
		restarted.mu.RLock()
		_, restored := testLookupActiveAlert(t, restarted, alert.ID)
		restarted.mu.RUnlock()
		if !restored {
			t.Fatal("self-contained degraded marker did not restore fired alert over malformed JSON mirror")
		}
		repairedMirror, err := os.ReadFile(filepath.Join(dataDir, "alerts", "active-alerts.json"))
		if err != nil {
			t.Fatalf("read repaired recovery mirror: %v", err)
		}
		var repaired []*Alert
		if err := json.Unmarshal(repairedMirror, &repaired); err != nil || len(repaired) != 1 || repaired[0].ID != alert.ID {
			t.Fatalf("repaired recovery mirror = %+v, error = %v", repaired, err)
		}
	})

	t.Run("resolved before restart", func(t *testing.T) {
		dataDir := t.TempDir()
		alert := durableRestoreAlert(time.Now().Add(-2 * time.Hour).UTC())
		writeActiveRecoveryFixture(t, dataDir, []*Alert{alert})

		m := NewManagerWithDataDir(dataDir)
		m.EnableEventLog()
		if err := m.SaveActiveAlerts(); err != nil {
			t.Fatalf("seed active state: %v", err)
		}
		store := m.eventLogStore()
		if store == nil {
			t.Fatal("event log was not enabled")
		}
		store.Close()

		m.clearAlert(alert.ID)
		data, err := os.ReadFile(filepath.Join(dataDir, "alerts", "active-alerts.json"))
		if err != nil {
			t.Fatalf("read recovery mirror: %v", err)
		}
		var recovered []*Alert
		if err := json.Unmarshal(data, &recovered); err != nil {
			t.Fatalf("decode recovery mirror: %v", err)
		}
		if len(recovered) != 0 {
			t.Fatalf("failed resolution remained in recovery mirror: %+v", recovered)
		}
		markerData, err := os.ReadFile(filepath.Join(dataDir, "alerts", activeStateDegradedMarker))
		if err != nil {
			t.Fatalf("read degraded recovery marker: %v", err)
		}
		var envelope activeStateRecoveryEnvelope
		if err := json.Unmarshal(markerData, &envelope); err != nil {
			t.Fatalf("decode degraded recovery marker: %v", err)
		}
		if envelope.Alerts == nil || len(envelope.Alerts) != 0 {
			t.Fatalf("resolved degraded recovery marker = %+v", envelope.Alerts)
		}

		m.SetEventLog(nil)
		m.Stop()
		if err := os.Remove(filepath.Join(dataDir, "alerts", "active-alerts.json")); err != nil {
			t.Fatalf("remove recovery mirror to isolate marker recovery: %v", err)
		}
		restarted := NewManagerWithDataDir(dataDir)
		t.Cleanup(restarted.Stop)
		restarted.EnableEventLog()
		restarted.mu.RLock()
		_, exists := testLookupActiveAlert(t, restarted, alert.ID)
		restarted.mu.RUnlock()
		if exists {
			t.Fatal("failed durable resolution was resurrected after restart")
		}
	})
}

func TestDurableLifecycleFailureSurfacesRecoveryMarkerFailure(t *testing.T) {
	dataDir := t.TempDir()
	m := NewManagerWithDataDir(dataDir)
	t.Cleanup(m.Stop)
	marker := filepath.Join(dataDir, "alerts", activeStateDegradedMarker)
	if err := os.MkdirAll(marker, alertsDirPerm); err != nil {
		t.Fatal(err)
	}

	alert := durableRestoreAlert(time.Now().UTC())
	m.setActiveRecoveryAlert(alert, alert.ID)
	err := m.checkpointActiveRecoveryAfterDurableFailure(os.ErrClosed)
	if err == nil {
		t.Fatal("checkpoint did not surface an unwritable recovery marker")
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "alerts", "active-alerts.json")); statErr != nil {
		t.Fatalf("best-effort recovery mirror was not written after marker failure: %v", statErr)
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
