package alerts

// The one-time legacy-history migration: JSON entries become
// history_imported events, the files retire to *.imported, and the
// projection serves the imported entries. The file's presence is the
// marker, so a second startup imports nothing.

import (
	"os"
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
