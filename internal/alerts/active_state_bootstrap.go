package alerts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

const (
	activeStateDegradedMarker               = "active-state-sqlite-degraded"
	activeStateRecoverySchemaVersion        = 1
	activeStatePartialRecoverySchemaVersion = 2
)

type activeStateRecoveryEnvelope struct {
	SchemaVersion    int                      `json:"schemaVersion"`
	RecordedAt       time.Time                `json:"recordedAt"`
	Cause            string                   `json:"cause,omitempty"`
	Alerts           []*Alert                 `json:"alerts"`
	HistoryBoundary  eventlog.JournalBoundary `json:"historyBoundary,omitempty"`
	SnapshotBoundary eventlog.JournalBoundary `json:"snapshotBoundary,omitempty"`
	Incomplete       bool                     `json:"incomplete,omitempty"`
}

// bootstrapActiveState establishes one restart authority. Existing databases
// load their projection; pre-projection databases import the already-decoded
// JSON recovery mirror. A degraded marker reverses that preference exactly
// once so a newer mirror can repair a previously failed SQLite checkpoint.
func (m *Manager) bootstrapActiveState(store *eventlog.Store) (authoritative bool) {
	if m == nil || store == nil {
		return false
	}
	var incorporated *eventlog.JournalBoundary
	reconcile := func(alerts []*Alert, boundary eventlog.JournalBoundary, captured ...eventlog.JournalBoundary) ([]*Alert, error) {
		through := store.JournalBoundary()
		result, err := reconcileActiveRecoveryHistory(alerts, store, boundary, captured...)
		if err == nil {
			incorporated = &through
		}
		return result, err
	}
	defer func() {
		m.recoveryMirrorMu.Lock()
		defer m.recoveryMirrorMu.Unlock()
		if authoritative {
			m.activeRecoveryJournalFloor = nil
			return
		}
		if incorporated != nil {
			// Projection storage may still fail, but the live restore has now
			// incorporated the complete retained prefix. Future snapshots can use
			// their own boundary instead of replaying later live changes forever.
			m.activeRecoveryJournalFloor = nil
			return
		}
		if m.activeRecoveryJournalFloor == nil {
			boundary := m.activeRecoveryHistoryBoundary()
			if boundary.ID == "" {
				boundary = m.activeRecoveryCaptureBoundary()
			}
			if boundary.ID == "" {
				boundary.ID = store.JournalBoundary().ID
				boundary.Sequence = 0
			}
			m.activeRecoveryJournalFloor = &boundary
		}
	}()

	initialized, err := store.ActiveStateInitialized()
	if err != nil {
		m.markActiveStateDegraded(err)
		// Even an unreadable target cannot hide a resolution/acknowledgement
		// retained in the independent journal. Prefer the self-contained marker
		// over the compatibility mirror, and keep the proven floor on partial folds.
		if !m.skipPersistedRestore {
			recovered := m.snapshotActiveAlerts()
			boundary := eventlog.JournalBoundary{}
			captured := m.activeRecoveryCaptureBoundary()
			if marker, selfContained, markerErr := m.loadActiveStateDegradedRecovery(); markerErr == nil && selfContained {
				recovered = marker
				boundary = m.activeRecoveryHistoryBoundary()
				captured = m.activeRecoverySnapshotBoundary()
			}
			recovered, foldErr := reconcile(recovered, boundary, captured)
			if restoreErr := m.restoreActiveAlertSnapshots(recovered, "independent lifecycle journal during SQLite outage", true); restoreErr != nil {
				log.Error().Err(restoreErr).Msg("failed to restore independent journal recovery")
			}
			if foldErr != nil {
				log.Error().Err(foldErr).Msg("SQLite recovery view remains incomplete; retained journal facts folded")
			}
		}
		log.Error().Err(err).Msg("SQLite active alert authority unavailable; retaining journal recovery state")
		return false
	}
	degraded := m.activeStateDegraded()
	checkpointComplete := false
	if initialized {
		checkpointComplete, err = store.ActiveCheckpointComplete()
		if err != nil {
			m.markActiveStateDegraded(err)
			return false
		}
	}
	recoveryFileExists := false
	if _, err := os.Stat(filepath.Join(m.getAlertsDir(), "active-alerts.json")); err == nil {
		recoveryFileExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Warn().Err(err).Msg("could not inspect active alert recovery mirror")
	}

	partialRecovery := degraded && m.activeRecoveryMarkerIncomplete()
	if !m.skipPersistedRestore && (partialRecovery || (initialized && checkpointComplete && !degraded)) {
		snapshots, boundary, err := store.LoadActiveStateAtJournalBoundary()
		if err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("failed to load SQLite active alert authority; keeping recovery mirror state")
			return false
		}
		alerts, err := decodeActiveStateSnapshots(snapshots)
		if err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("SQLite active alert authority is invalid; keeping recovery mirror state")
			return false
		}
		var partial []*Alert
		captured := boundary
		if partialRecovery {
			partial, _, err = m.loadActiveStateDegradedRecovery()
			if err != nil {
				return false
			}
		}
		if partialRecovery && !checkpointComplete {
			// Neither source proves omissions before the first full checkpoint.
			// Fold each at its own cursor: replaying older controls over current
			// SQL rows could roll back a later control whose event was pruned.
			alerts, err = reconcileActiveRecoveryHistory(alerts, store, boundary, boundary)
			if err == nil {
				partial, err = reconcile(partial, m.activeRecoveryHistoryBoundary(), m.activeRecoverySnapshotBoundary())
			}
			if err == nil {
				alerts = mergePartialRecoveryBaseline(alerts, partial)
			}
		} else {
			alerts, err = reconcile(alerts, boundary, captured)
		}
		if err != nil {
			m.markActiveStateDegraded(err)
			return false
		}
		if partialRecovery && checkpointComplete {
			alerts = mergePartialRecoveryObservations(alerts, partial)
		}
		if err := m.restoreActiveAlertSnapshots(alerts, "SQLite active-state projection and journal tail", true); err != nil {
			m.markActiveStateDegraded(err)
			log.Error().Err(err).Msg("failed to restore SQLite active alert authority")
			return false
		}
		if partialRecovery {
			// History retention cannot establish the entire active set. Only the
			// complete projection plus retained journal tail, or the union of all
			// positive sources before the first checkpoint, can replace a partial
			// fallback; its omissions carry no such authority.
			snapshots, err := activeStateSnapshots(alerts)
			if err == nil {
				err = store.ReplaceActiveState(snapshots)
			}
			if err != nil {
				markerErr, mirrorErr := m.writeDegradedActiveState(alerts, err, *incorporated)
				log.Error().Err(errors.Join(err, markerErr, mirrorErr)).Msg("complete active recovery checkpoint remains unsettled")
				return false
			}
			m.activeRecoveryWriteBlock.Store(false)
			if err := m.writeActiveAlertsRecoveryMirror(alerts, *incorporated); err != nil {
				return false
			}
			m.activeRecoveryReadable.Store(true)
			if err := m.clearActiveStateDegraded(); err != nil {
				return false
			}
		}
		m.activeStateAuthoritative.Store(true)
		return true
	}

	if degraded && !m.skipPersistedRestore {
		recovered, selfContained, err := m.loadActiveStateDegradedRecovery()
		if err != nil {
			log.Error().Err(err).Msg("active alert recovery marker is invalid; SQLite authority initialization deferred")
			return false
		}
		if selfContained {
			// The marker may predate a journalled transition if the process stopped
			// between event admission and its recovery checkpoint. Retained canonical
			// lifecycle facts must be folded over that marker before it can replace
			// SQLite's newly recovered active projection.
			recovered, err = reconcile(recovered, m.activeRecoveryHistoryBoundary(), m.activeRecoverySnapshotBoundary())
			if err := m.restoreActiveAlertSnapshots(recovered, "crash-safe SQLite recovery marker and lifecycle journal", true); err != nil {
				log.Error().Err(err).Msg("failed to restore active alerts from crash-safe recovery marker")
				return false
			}
			if err != nil {
				log.Error().Err(err).Msg("active alert marker reconciliation remains partial")
				return false
			}
			// The self-contained marker is the trusted source, so it can also
			// replace a missing or malformed compatibility mirror. Keep the
			// marker until both recovery copies and SQLite agree.
			m.activeRecoveryWriteBlock.Store(false)
			if err := m.writeActiveAlertsRecoveryMirror(recovered, store.JournalBoundary()); err != nil {
				log.Error().Err(err).Msg("failed to repair active alert recovery mirror from crash-safe marker")
				return false
			}
			m.activeRecoveryReadable.Store(true)
		} else if !recoveryFileExists || !m.activeRecoveryReadable.Load() {
			log.Error().Msg("legacy active alert degradation marker has no readable recovery mirror; SQLite authority initialization deferred")
			return false
		} else {
			recovered, err := reconcile(m.snapshotActiveAlerts(), eventlog.JournalBoundary{}, m.activeRecoveryCaptureBoundary())
			if err := m.restoreActiveAlertSnapshots(recovered, "legacy recovery mirror and lifecycle journal", true); err != nil {
				return false
			}
			if err != nil {
				return false
			}
		}
	}

	// A new database, an explicit clean-room manager, or a prior failed
	// checkpoint starts from the in-memory recovery state already loaded by the
	// constructor. Never overwrite a new database from malformed JSON.
	if !m.skipPersistedRestore && recoveryFileExists && !m.activeRecoveryReadable.Load() {
		log.Error().Msg("active alert recovery mirror is unreadable; SQLite authority initialization deferred")
		return false
	}
	if !checkpointComplete && !degraded && !m.skipPersistedRestore {
		// A lifecycle activation does not make the projection a full baseline.
		// Import the same trusted constructor recovery source as an uninitialized
		// database, folding retained facts before establishing that baseline.
		recovered, err := reconcile(m.snapshotActiveAlerts(), eventlog.JournalBoundary{}, m.activeRecoveryCaptureBoundary())
		if restoreErr := m.restoreActiveAlertSnapshots(recovered, "initial active recovery baseline and journal", true); restoreErr != nil {
			m.markActiveStateDegraded(restoreErr)
			return false
		}
		if err != nil {
			m.markActiveStateDegraded(err)
			return false
		}
	}
	boundary := store.JournalBoundary()
	snapshots, err := m.currentActiveStateSnapshots()
	if err != nil {
		m.markActiveStateDegraded(err)
		log.Error().Err(err).Msg("failed to encode active alerts for SQLite authority")
		return false
	}
	// The validated fallback is complete even if its first SQL checkpoint fails.
	// Keep that knowledge separate from storage success: a later isolated firing
	// must not turn a recoverable full fallback into a permanent partial state.
	incorporated = &boundary
	if err := store.ReplaceActiveState(snapshots); err != nil {
		m.markActiveStateDegraded(err)
		markerErr, mirrorErr := m.writeDegradedActiveState(m.snapshotActiveAlerts(), err, boundary)
		log.Error().Err(errors.Join(err, markerErr, mirrorErr)).Msg("failed to initialize SQLite active alert authority")
		return false
	}
	if err := m.clearActiveStateDegraded(); err != nil {
		log.Error().Err(err).Msg("failed to durably clear repaired SQLite active alert degradation marker")
		return false
	}
	m.activeStateAuthoritative.Store(true)
	log.Info().Int("alerts", len(snapshots)).Msg("SQLite active alert authority initialized from recovery state")
	return true
}

func decodeActiveStateSnapshots(snapshots []eventlog.ActiveStateSnapshot) ([]*Alert, error) {
	alerts := make([]*Alert, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var alert Alert
		if err := json.Unmarshal(snapshot.Snapshot, &alert); err != nil {
			return nil, fmt.Errorf("decode SQLite active alert %q: %w", snapshot.AlertID, err)
		}
		if alert.ID == "" {
			alert.ID = snapshot.AlertID
		}
		if alert.ID == "" {
			return nil, fmt.Errorf("SQLite active alert has no identity")
		}
		alerts = append(alerts, &alert)
	}
	return alerts, nil
}

func (m *Manager) currentActiveStateSnapshots() ([]eventlog.ActiveStateSnapshot, error) {
	m.mu.RLock()
	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		alerts = append(alerts, cloneAlertForOutput(alert))
	}
	m.mu.RUnlock()
	return activeStateSnapshots(alerts)
}

func (m *Manager) activeStateDegradedPath() string {
	return filepath.Join(m.getAlertsDir(), activeStateDegradedMarker)
}

func (m *Manager) activeStateDegraded() bool {
	_, err := os.Stat(m.activeStateDegradedPath())
	return err == nil
}

func (m *Manager) markActiveStateDegraded(cause error) {
	if m == nil {
		return
	}
	m.activeStateAuthoritative.Store(false)
	message := "SQLite active alert state requires recovery from active-alerts.json\n"
	if cause != nil {
		message += cause.Error() + "\n"
	}
	m.recoveryMirrorMu.Lock()
	_, statErr := os.Stat(m.activeStateDegradedPath())
	var err error
	if errors.Is(statErr, os.ErrNotExist) {
		err = m.writeActiveStateDegradedMarker([]byte(message))
	} else if statErr != nil {
		err = fmt.Errorf("inspect existing SQLite active alert degradation marker: %w", statErr)
	}
	m.recoveryMirrorMu.Unlock()
	if err != nil {
		log.Error().Err(err).Msg("failed to persist SQLite active alert degradation marker")
	}
}

func (m *Manager) writeActiveStateDegradedRecovery(alerts []*Alert, cause error, boundaries ...eventlog.JournalBoundary) error {
	envelope := activeStateRecoveryEnvelope{
		SchemaVersion: activeStateRecoverySchemaVersion,
		RecordedAt:    time.Now().UTC(),
		Alerts:        alerts,
	}
	if len(boundaries) > 0 {
		envelope.HistoryBoundary = boundaries[0]
		envelope.SnapshotBoundary = boundaries[0]
	}
	if floor := m.activeRecoveryJournalFloor; floor != nil {
		envelope.Incomplete = true
		envelope.SchemaVersion = activeStatePartialRecoverySchemaVersion
		if envelope.HistoryBoundary.ID == "" || envelope.HistoryBoundary.ID == floor.ID {
			envelope.HistoryBoundary = *floor
			envelope.SnapshotBoundary = *floor
		}
	}
	if envelope.Alerts == nil {
		envelope.Alerts = []*Alert{}
	}
	if cause != nil {
		envelope.Cause = cause.Error()
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal SQLite active alert recovery marker: %w", err)
	}
	return m.writeActiveStateDegradedMarker(data)
}

func (m *Manager) writeActiveStateDegradedMarker(data []byte) error {
	return m.writeActiveRecoveryMetadata(activeStateDegradedMarker, data)
}

func (m *Manager) writeActiveRecoveryMetadata(name string, data []byte) error {
	alertsDir := m.getAlertsDir()
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		return fmt.Errorf("create alerts directory for active recovery metadata: %w", err)
	}
	if info, err := os.Stat(alertsDir); err != nil {
		return err
	} else if info.Mode().Perm() != alertsDirPerm {
		if err := os.Chmod(alertsDir, alertsDirPerm); err != nil {
			return err
		}
	}
	finalPath := filepath.Join(alertsDir, name)
	if info, err := os.Lstat(finalPath); err == nil && info.Mode().IsRegular() && info.Mode().Perm() == alertsFilePerm {
		if existing, err := readLimitedRegularFile(finalPath, int64(len(data))+1); err == nil && bytes.Equal(existing, data) {
			return syncActiveAlertsDirectory(alertsDir)
		}
	}
	tmp, err := os.CreateTemp(alertsDir, ".active-recovery-*.tmp")
	if err != nil {
		return fmt.Errorf("create active recovery metadata temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write active recovery metadata temp file: %w", err)
	}
	if err := tmp.Chmod(alertsFilePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set active recovery metadata permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync active recovery metadata temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close active recovery metadata temp file: %w", err)
	}
	if err := replaceActiveAlertsFile(tmpName, finalPath); err != nil {
		return fmt.Errorf("replace active recovery metadata: %w", err)
	}
	cleanup = false
	if err := syncActiveAlertsDirectory(alertsDir); err != nil {
		return fmt.Errorf("sync alerts directory after active recovery metadata: %w", err)
	}
	return nil
}

func (m *Manager) loadActiveStateDegradedRecovery() ([]*Alert, bool, error) {
	data, err := readLimitedRegularFile(m.activeStateDegradedPath(), maxActiveAlertsFileSizeBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read SQLite active alert recovery marker: %w", err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false, nil
	}
	var envelope activeStateRecoveryEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, false, fmt.Errorf("decode SQLite active alert recovery marker: %w", err)
	}
	if envelope.SchemaVersion != activeStateRecoverySchemaVersion && envelope.SchemaVersion != activeStatePartialRecoverySchemaVersion {
		return nil, false, fmt.Errorf("unsupported SQLite active alert recovery marker schema %d", envelope.SchemaVersion)
	}
	if envelope.SchemaVersion == activeStatePartialRecoverySchemaVersion && !envelope.Incomplete {
		return nil, false, fmt.Errorf("partial active recovery marker has no incomplete-state declaration")
	}
	if envelope.Alerts == nil {
		return nil, false, fmt.Errorf("SQLite active alert recovery marker has no alert snapshot")
	}
	return envelope.Alerts, true, nil
}

func (m *Manager) clearActiveStateDegraded() error {
	if m == nil {
		return nil
	}
	err := os.Remove(m.activeStateDegradedPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove SQLite active alert degradation marker: %w", err)
	}
	if err := syncActiveAlertsDirectory(m.getAlertsDir()); err != nil {
		return fmt.Errorf("sync alerts directory after clearing SQLite degradation marker: %w", err)
	}
	return nil
}

// reconcileActiveRecoveryHistory applies retained facts in durable-ID order to
// a degraded recovery snapshot. A missing occurrence is never a resolution;
// only an explicit matching resolution deletes it. Newer occurrence identities
// in the mirror survive older journal facts, and newer mutable observation and
// notification timestamps survive replay of that occurrence's lifecycle.
func reconcileActiveRecoveryHistory(recovered []*Alert, store *eventlog.Store, boundary eventlog.JournalBoundary, captured ...eventlog.JournalBoundary) ([]*Alert, error) {
	current := make(map[string]*Alert, len(recovered))
	for _, alert := range recovered {
		if alert == nil {
			continue
		}
		clone := alert.Clone()
		backfillCanonicalIdentity(clone)
		clone.ID = exportedAlertID(clone, clone.ID)
		current[clone.ID] = clone
	}
	err := store.WalkRecoverySince(boundary, func(event eventlog.Event) error {
		switch event.Type {
		case eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved, eventlog.TypeActiveRetired, eventlog.TypeActiveMoved, eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged, eventlog.TypeSnoozed, eventlog.TypeUnsnoozed, eventlog.TypeEscalated:
		default:
			return nil
		}
		if len(event.Snapshot) == 0 {
			return nil
		}
		var incoming Alert
		if err := json.Unmarshal(event.Snapshot, &incoming); err != nil {
			return err
		}
		incoming.ID = event.AlertID
		if event.Type == eventlog.TypeActiveMoved {
			previous := event.Details["previousAlertId"]
			if prior := current[previous]; prior != nil && prior.StartTime.Equal(incoming.StartTime) {
				delete(current, previous)
			}
		}
		existing := current[event.AlertID]
		if existing == nil && event.Type != eventlog.TypeFired && event.Type != eventlog.TypeRefired && event.Type != eventlog.TypeActiveMoved {
			return nil // Control facts cannot reactivate a terminal occurrence.
		}
		if existing == nil && len(captured) > 0 {
			origin := event.JournalBoundary()
			if origin.ID == captured[0].ID && origin.Sequence <= captured[0].Sequence {
				// The checkpoint positively omits this occurrence at its capture
				// prefix, even when an older floor still needs reconciliation.
				return nil
			}
		}
		if event.Type == eventlog.TypeResolved || event.Type == eventlog.TypeActiveRetired {
			if existing != nil && existing.StartTime.Equal(incoming.StartTime) {
				delete(current, event.AlertID)
			}
			return nil
		}
		if existing != nil {
			if existing.StartTime.After(incoming.StartTime) {
				return nil
			}
			if existing.StartTime.Equal(incoming.StartTime) {
				// A lifecycle fact owns its transition fields, not a later detector
				// observation. Replaying a conservative floor must not roll back level,
				// fingerprint metadata or other unjournalled presentation updates.
				merged := existing.Clone()
				if incoming.LastSeen.After(existing.LastSeen) {
					merged = incoming.Clone()
					merged.Acknowledged, merged.AckTime, merged.AckUser = existing.Acknowledged, existing.AckTime, existing.AckUser
				}
				switch event.Type {
				case eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged:
					merged.Acknowledged, merged.AckTime, merged.AckUser = incoming.Acknowledged, incoming.AckTime, incoming.AckUser
				case eventlog.TypeSnoozed, eventlog.TypeUnsnoozed:
					merged.OperationalRecord = incoming.OperationalRecord
					merged.LatestTransition, merged.Transitions = incoming.LatestTransition, incoming.Transitions
				case eventlog.TypeEscalated:
					merged.LastEscalation, merged.EscalationTimes = incoming.LastEscalation, incoming.EscalationTimes
				}
				if incoming.LastNotified != nil && (merged.LastNotified == nil || incoming.LastNotified.After(*merged.LastNotified)) {
					merged.LastNotified = incoming.LastNotified
				}
				incoming = *merged
			}
		}
		current[event.AlertID] = incoming.Clone()
		return nil
	})
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*Alert, 0, len(keys))
	for _, key := range keys {
		result = append(result, current[key])
	}
	return result, err
}

func (m *Manager) activeRecoveryHistoryBoundary() eventlog.JournalBoundary {
	data, err := readLimitedRegularFile(m.activeStateDegradedPath(), maxActiveAlertsFileSizeBytes)
	if err != nil {
		return eventlog.JournalBoundary{}
	}
	var envelope activeStateRecoveryEnvelope
	if json.Unmarshal(data, &envelope) != nil {
		return eventlog.JournalBoundary{}
	}
	return envelope.HistoryBoundary
}

func (m *Manager) activeRecoverySnapshotBoundary() eventlog.JournalBoundary {
	data, err := readLimitedRegularFile(m.activeStateDegradedPath(), maxActiveAlertsFileSizeBytes)
	if err != nil {
		return eventlog.JournalBoundary{}
	}
	var envelope activeStateRecoveryEnvelope
	if json.Unmarshal(data, &envelope) != nil {
		return eventlog.JournalBoundary{}
	}
	return envelope.SnapshotBoundary
}

func (m *Manager) activeRecoveryMarkerIncomplete() bool {
	data, err := readLimitedRegularFile(m.activeStateDegradedPath(), maxActiveAlertsFileSizeBytes)
	if err != nil {
		return false
	}
	var envelope activeStateRecoveryEnvelope
	return json.Unmarshal(data, &envelope) == nil && envelope.Incomplete && envelope.SchemaVersion == activeStatePartialRecoverySchemaVersion
}

func (m *Manager) activeRecoveryIncomplete() bool {
	m.recoveryMirrorMu.Lock()
	defer m.recoveryMirrorMu.Unlock()
	return m.activeRecoveryJournalFloor != nil
}

func mergePartialRecoveryBaseline(canonical, partial []*Alert) []*Alert {
	canonical = mergePartialRecoveryObservations(canonical, partial)
	known := make(map[string]bool, len(canonical))
	for _, alert := range canonical {
		if alert != nil {
			known[alert.ID] = true
		}
	}
	for _, alert := range partial {
		if alert != nil && !known[alert.ID] {
			canonical = append(canonical, alert.Clone())
			known[alert.ID] = true
		}
	}
	return canonical
}

// A partial fallback can refresh observations of proven active occurrences;
// it cannot introduce an occurrence, remove one, or replace canonical controls.
func mergePartialRecoveryObservations(canonical, partial []*Alert) []*Alert {
	byID := make(map[string]*Alert, len(partial))
	for _, alert := range partial {
		if alert != nil {
			byID[alert.ID] = alert
		}
	}
	for _, alert := range canonical {
		observation := byID[alert.ID]
		if observation == nil || !observation.StartTime.Equal(alert.StartTime) {
			continue
		}
		if observation.LastSeen.After(alert.LastSeen) {
			observation = observation.Clone()
			alert.LastSeen, alert.Level, alert.Message, alert.Summary = observation.LastSeen, observation.Level, observation.Message, observation.Summary
			alert.Value, alert.Threshold, alert.Metadata, alert.Correlation = observation.Value, observation.Threshold, observation.Metadata, observation.Correlation
			alert.ResourceName, alert.Node, alert.NodeDisplayName, alert.Instance = observation.ResourceName, observation.Node, observation.NodeDisplayName, observation.Instance
		}
		if observation.LastNotified != nil && (alert.LastNotified == nil || observation.LastNotified.After(*alert.LastNotified)) {
			at := *observation.LastNotified
			alert.LastNotified = &at
		}
	}
	return canonical
}
