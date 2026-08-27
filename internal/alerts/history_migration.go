package alerts

// One-time migration of the legacy JSON alert history into the event log
// (docs/ALERT_ENGINE_EVOLUTION.md — the log becomes the sole history
// authority). The JSON file's continued presence is the migration marker:
// import runs when the file exists, and a successful import renames it to
// *.imported, so the migration is idempotent and the original data survives
// as a backup.

import (
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

// importLegacyHistoryIntoEventLog migrates the loaded JSON history entries
// into the event log as history_imported events and retires the JSON files.
// A failed import leaves the files in place so the next startup retries;
// history reads fall back to the in-memory entries either way.
func (m *Manager) importLegacyHistoryIntoEventLog(store *eventlog.Store) {
	if m == nil || store == nil || m.historyManager == nil {
		return
	}
	if !m.historyManager.StorageFileExists() {
		return
	}

	entries := m.historyManager.SnapshotEntries()
	events := make([]eventlog.Event, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		exported := cloneAlertForOutput(&entry.Alert)
		if exported == nil || exported.ID == "" {
			continue
		}
		snapshot, err := json.Marshal(exported)
		if err != nil {
			continue
		}
		occurredAt := entry.Timestamp
		if entry.Alert.LastSeen.After(occurredAt) {
			occurredAt = entry.Alert.LastSeen
		}
		events = append(events, eventlog.Event{
			OccurredAt:   occurredAt,
			Type:         eventlog.TypeHistoryImported,
			AlertID:      exported.ID,
			ResourceID:   exported.ResourceID,
			ResourceName: exported.ResourceName,
			AlertType:    exported.Type,
			Level:        string(exported.Level),
			Message:      "Imported from the legacy alert history file.",
			Snapshot:     snapshot,
		})
	}

	if len(events) > 0 {
		if err := store.ImportEvents(events); err != nil {
			log.Error().Err(err).
				Int("entries", len(events)).
				Msg("legacy alert history import failed; JSON history stays authoritative until the next attempt")
			return
		}
	}
	if err := m.historyManager.RetireStorage(); err != nil {
		log.Error().Err(err).Msg("legacy alert history files could not be retired after import")
		return
	}
	log.Info().
		Int("entries", len(events)).
		Msg("legacy alert history imported into the event log; JSON history files retired")
}
