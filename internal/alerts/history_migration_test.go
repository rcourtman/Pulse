package alerts

// The one-time legacy-history migration: JSON entries become
// history_imported events, the files retire to *.imported, and the
// projection serves the imported entries. Either legacy file is a retry
// marker, and retry after a retirement failure must not duplicate events.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

func TestLegacyHistoryImportRetiresJSONAndServesEntries(t *testing.T) {
	m := newTestManager(t)
	m.UpdateConfig(contractTestConfig(m))

	// Seed a legacy occurrence and persist it the way the JSON manager did.
	past := time.Now().Add(-2 * time.Hour)
	legacyAlert := Alert{
		ID:           "legacy-vm::metric-threshold:cpu",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "legacy-vm",
		ResourceName: "Legacy VM",
		Value:        91,
		StartTime:    past,
		LastSeen:     past.Add(30 * time.Minute),
	}
	m.historyManager.AddAlert(legacyAlert)
	if err := m.historyManager.saveHistory(); err != nil {
		t.Fatalf("persist legacy history: %v", err)
	}
	if !m.historyManager.StorageFileExists() {
		t.Fatal("expected the JSON history file on disk before migration")
	}
	historyFile := m.historyManager.historyFile

	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)
	t.Cleanup(func() { m.SetEventLog(nil) })
	m.importLegacyHistoryIntoEventLog(store)

	if m.historyManager.StorageFileExists() {
		t.Fatal("JSON history file still present after import")
	}
	if _, err := os.Stat(historyFile + ".imported"); err != nil {
		t.Fatalf("retired history backup missing: %v", err)
	}

	projected, ok := m.AlertHistoryFromEvents(time.Time{}, 0)
	if !ok {
		t.Fatal("projection unavailable")
	}
	found := false
	for _, entry := range projected {
		if entry.ResourceID == "legacy-vm" {
			found = true
			if !entry.StartTime.Truncate(time.Second).Equal(past.Truncate(time.Second)) {
				t.Errorf("imported entry StartTime = %v, want %v", entry.StartTime, past)
			}
		}
	}
	if !found {
		t.Fatalf("imported legacy entry missing from projection: %+v", summarizeHistory(projected))
	}

	// Second run: the marker file is gone, so nothing imports again.
	before, _ := store.Query(eventlog.Filter{Types: []string{eventlog.TypeHistoryImported}, Limit: 1000})
	m.importLegacyHistoryIntoEventLog(store)
	store.Flush()
	after, _ := store.Query(eventlog.Filter{Types: []string{eventlog.TypeHistoryImported}, Limit: 1000})
	if len(after) != len(before) {
		t.Fatalf("re-running the import duplicated entries: %d -> %d", len(before), len(after))
	}
}

func TestRetiredLegacyHistoryRecoversRecreatedEventDatabase(t *testing.T) {
	dataDir := t.TempDir()
	seed := NewManagerWithDataDir(dataDir)
	past := time.Now().Add(-2 * time.Hour).UTC()
	seed.historyManager.AddAlert(Alert{
		ID:         "legacy-recovery::metric-threshold:cpu",
		Type:       "cpu",
		ResourceID: "legacy-recovery",
		StartTime:  past,
		LastSeen:   past.Add(time.Minute),
	})
	if err := seed.historyManager.saveHistory(); err != nil {
		t.Fatalf("persist legacy history: %v", err)
	}
	seed.EnableEventLog()
	seed.SetEventLog(nil)
	seed.Stop()

	eventsPath := filepath.Join(dataDir, "alerts", "events.db")
	if err := os.Remove(eventsPath); err != nil {
		t.Fatalf("remove event database: %v", err)
	}
	for _, suffix := range []string{"-shm", "-wal"} {
		_ = os.Remove(eventsPath + suffix)
	}

	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	restarted.EnableEventLog()
	entries := restarted.GetAlertHistory(0)
	if len(entries) != 1 || entries[0].ResourceID != "legacy-recovery" {
		t.Fatalf("recreated event database did not recover retired history: %#v", entries)
	}
}

func TestLegacyHistoryImportRetryAfterRetirementFailureIsIdempotent(t *testing.T) {
	dataDir := t.TempDir()
	m := NewManagerWithDataDir(dataDir)
	t.Cleanup(m.Stop)
	past := time.Now().Add(-time.Hour)
	m.historyManager.AddAlert(Alert{
		ID:           "legacy-retry::metric-threshold:cpu",
		Type:         "cpu",
		Level:        AlertLevelWarning,
		ResourceID:   "legacy-retry",
		ResourceName: "Legacy Retry VM",
		StartTime:    past,
		LastSeen:     past.Add(10 * time.Minute),
	})
	if err := m.historyManager.saveHistory(); err != nil {
		t.Fatalf("persist legacy history: %v", err)
	}

	// A directory at the retirement destination forces the post-import rename
	// to fail after the database transaction has committed.
	retirementBlocker := m.historyManager.historyFile + ".imported"
	if err := os.Mkdir(retirementBlocker, 0o700); err != nil {
		t.Fatalf("create retirement blocker: %v", err)
	}

	store, err := eventlog.Open(m.alertsDir)
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)

	m.importLegacyHistoryIntoEventLog(store)
	if !m.historyManager.StorageFileExists() {
		t.Fatal("legacy source disappeared despite retirement failure")
	}
	if got := importedHistoryEventCount(t, store); got != 1 {
		t.Fatalf("first import wrote %d events, want 1", got)
	}
	m.SetEventLog(nil)
	m.Stop()

	if err := os.Remove(retirementBlocker); err != nil {
		t.Fatalf("remove retirement blocker: %v", err)
	}

	restarted := NewManagerWithDataDir(dataDir)
	t.Cleanup(restarted.Stop)
	reopenedStore, err := eventlog.Open(restarted.alertsDir)
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetEventLog(reopenedStore)
	restarted.importLegacyHistoryIntoEventLog(reopenedStore)
	if restarted.historyManager.StorageFileExists() {
		t.Fatal("legacy source still present after successful retry")
	}
	if got := importedHistoryEventCount(t, reopenedStore); got != 1 {
		t.Fatalf("retirement retry duplicated import events: got %d, want 1", got)
	}
}

func TestLegacyHistoryImportUsesBackupAsRetryMarker(t *testing.T) {
	m := newTestManager(t)
	past := time.Now().Add(-time.Hour)
	m.historyManager.AddAlert(Alert{
		ID:           "legacy-backup::metric-threshold:memory",
		Type:         "memory",
		Level:        AlertLevelWarning,
		ResourceID:   "legacy-backup",
		ResourceName: "Legacy Backup VM",
		StartTime:    past,
		LastSeen:     past.Add(5 * time.Minute),
	})
	if err := m.historyManager.saveHistory(); err != nil {
		t.Fatalf("persist legacy history: %v", err)
	}
	data, err := os.ReadFile(m.historyManager.historyFile)
	if err != nil {
		t.Fatalf("read legacy history: %v", err)
	}
	if err := os.WriteFile(m.historyManager.backupFile, data, 0o600); err != nil {
		t.Fatalf("seed legacy backup: %v", err)
	}
	if err := os.Rename(m.historyManager.historyFile, m.historyManager.historyFile+".imported"); err != nil {
		t.Fatalf("simulate retired primary: %v", err)
	}
	if !m.historyManager.StorageFileExists() {
		t.Fatal("backup-only legacy source was not recognized")
	}

	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)
	t.Cleanup(func() { m.SetEventLog(nil) })
	m.importLegacyHistoryIntoEventLog(store)

	if m.historyManager.StorageFileExists() {
		t.Fatal("backup-only legacy source still present after import")
	}
	if _, err := os.Stat(m.historyManager.backupFile + ".imported"); err != nil {
		t.Fatalf("retired backup missing: %v", err)
	}
	if got := importedHistoryEventCount(t, store); got != 1 {
		t.Fatalf("backup-only import wrote %d events, want 1", got)
	}
}

func TestLegacyHistoryImportKeepsMalformedSourceUntouched(t *testing.T) {
	dataDir := t.TempDir()
	alertsDir := filepath.Join(dataDir, "alerts")
	if err := os.MkdirAll(alertsDir, 0o700); err != nil {
		t.Fatalf("create alerts dir: %v", err)
	}
	historyFile := filepath.Join(alertsDir, HistoryFileName)
	malformed := []byte(`{"alert":`)
	if err := os.WriteFile(historyFile, malformed, 0o600); err != nil {
		t.Fatalf("seed malformed history: %v", err)
	}

	m := NewManagerWithDataDir(dataDir)
	t.Cleanup(m.Stop)
	if m.historyManager.StorageLoadError() == nil {
		t.Fatal("malformed legacy source did not retain its load error")
	}
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)
	m.importLegacyHistoryIntoEventLog(store)

	if !m.historyManager.StorageFileExists() {
		t.Fatal("malformed legacy source was retired")
	}
	if got := importedHistoryEventCount(t, store); got != 0 {
		t.Fatalf("malformed legacy source imported %d events, want 0", got)
	}
	if err := m.historyManager.saveHistory(); err == nil {
		t.Fatal("periodic persistence could overwrite a malformed legacy source")
	}
	got, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatalf("read preserved malformed history: %v", err)
	}
	if string(got) != string(malformed) {
		t.Fatalf("malformed legacy source changed: got %q want %q", got, malformed)
	}
}

func TestLegacyHistoryImportKeepsUnidentifiableEntry(t *testing.T) {
	dataDir := t.TempDir()
	alertsDir := filepath.Join(dataDir, "alerts")
	if err := os.MkdirAll(alertsDir, 0o700); err != nil {
		t.Fatalf("create alerts dir: %v", err)
	}
	historyFile := filepath.Join(alertsDir, HistoryFileName)
	payload, err := json.Marshal([]HistoryEntry{{
		Alert: Alert{
			Type:         "cpu",
			ResourceID:   "missing-id",
			ResourceName: "Missing ID VM",
			StartTime:    time.Now().Add(-time.Hour),
			LastSeen:     time.Now(),
		},
		Timestamp: time.Now(),
	}})
	if err != nil {
		t.Fatalf("encode legacy history: %v", err)
	}
	if err := os.WriteFile(historyFile, payload, 0o600); err != nil {
		t.Fatalf("seed legacy history: %v", err)
	}

	m := NewManagerWithDataDir(dataDir)
	t.Cleanup(m.Stop)
	store, err := eventlog.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	m.SetEventLog(store)
	m.importLegacyHistoryIntoEventLog(store)

	if !m.historyManager.StorageFileExists() {
		t.Fatal("unidentifiable legacy entry was retired")
	}
	if got := importedHistoryEventCount(t, store); got != 0 {
		t.Fatalf("unidentifiable legacy entry imported %d events, want 0", got)
	}
}

func importedHistoryEventCount(t *testing.T, store *eventlog.Store) int {
	t.Helper()
	events, err := store.Query(eventlog.Filter{
		Types: []string{eventlog.TypeHistoryImported},
		Limit: 1000,
	})
	if err != nil {
		t.Fatalf("query imported history: %v", err)
	}
	return len(events)
}

func TestClearAlertHistoryTombstonesTheProjection(t *testing.T) {
	m := newHistoryParityManager(t)
	cfg := m.GetConfig()
	contractRaiseGuestCPUAlert(t, m, "clear-vm-1", 95)
	m.checkMetric("clear-vm-1", "Contract VM clear-vm-1", "node1", "inst1", "guest", "cpu", 40, cfg.GuestDefaults.CPU, nil)

	if entries := m.GetAlertHistory(0); len(entries) == 0 {
		t.Fatal("expected history before the clear")
	}
	if err := m.ClearAlertHistory(); err != nil {
		t.Fatalf("clear history: %v", err)
	}
	if entries := m.GetAlertHistory(0); len(entries) != 0 {
		t.Fatalf("expected empty history after the clear, got %d entries: %v", len(entries), summarizeHistory(entries))
	}

	// Lifecycle after the tombstone repopulates normally.
	contractRaiseGuestCPUAlert(t, m, "clear-vm-2", 96)
	entries := m.GetAlertHistory(0)
	if len(entries) != 1 || entries[0].ResourceID != "clear-vm-2" {
		t.Fatalf("expected exactly the post-clear occurrence, got %v", summarizeHistory(entries))
	}
}
