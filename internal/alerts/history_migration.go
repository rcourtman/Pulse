package alerts

// One-time migration of the legacy JSON alert history into the event log
// (docs/ALERT_ENGINE_EVOLUTION.md — the log becomes the sole history
// authority). Either JSON leaf's continued presence is the migration marker:
// import runs while the primary or backup exists, and successful retirement
// renames them to *.imported. The event-log import itself is idempotent, so a
// database commit followed by a rename failure can retry without duplicating
// user history.

import (
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

// importLegacyHistoryIntoEventLog migrates the loaded JSON history entries
// into the event log as history_imported events and retires the JSON files.
// A failed import leaves the files in place so the next startup retries;
// history reads fall back to the in-memory entries either way. A retirement
// failure also retries safely because history_imported events are inserted by
// immutable event identity.
func (m *Manager) importLegacyHistoryIntoEventLog(store *eventlog.Store) (authoritative bool) {
	if m != nil {
		defer func() { m.eventHistoryAuthoritative.Store(authoritative) }()
	}
	if m == nil || store == nil || m.historyManager == nil {
		return false
	}
	hasLegacySource := m.historyManager.StorageFileExists()
	if !hasLegacySource && !m.historyManager.ImportedStorageFileExists() {
		return true
	}
	if err := m.historyManager.StorageLoadError(); err != nil {
		log.Error().Err(err).
			Msg("legacy alert history import deferred; source could not be loaded and remains untouched")
		return false
	}

	entries := m.historyManager.SnapshotEntries()
	events := make([]eventlog.Event, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		exported := cloneAlertForOutput(&entry.Alert)
		if exported == nil || exported.ID == "" {
			log.Error().Int("entry", i).
				Msg("legacy alert history import deferred; entry has no alert identity")
			return false
		}
		snapshot, err := json.Marshal(exported)
		if err != nil {
			log.Error().Err(err).Int("entry", i).
				Msg("legacy alert history import deferred; entry snapshot could not be encoded")
			return false
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
			return false
		}
	}
	if hasLegacySource {
		if err := m.historyManager.RetireStorage(); err != nil {
			log.Error().Err(err).Msg("legacy alert history files could not be retired after import")
			return false
		}
	}
	log.Info().
		Int("entries", len(events)).
		Msg("legacy alert history imported into the event log; JSON history files retired")
	return true
}
