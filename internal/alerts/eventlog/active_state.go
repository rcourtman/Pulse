package eventlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const activeStateInitializedKey = "active_state_initialized"
const activeStateRevisionKey = "active_state_revision"
const activeCheckpointCompleteKey = "active_state_checkpoint_complete"
const activeCheckpointProtocolKey = "active_state_checkpoint_protocol"

// A lifecycle row proves that occurrence, not a complete imported baseline.
// Migrate the legacy authority declaration once, before any new replay writes.
func (s *Store) ensureActiveCheckpointProtocol() error {
	var version string
	err := s.db.QueryRow(`SELECT value FROM alert_store_meta WHERE key = ?`, activeCheckpointProtocolKey).Scan(&version)
	if err == nil {
		if version != "1" {
			return fmt.Errorf("unsupported active checkpoint protocol %q", version)
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO alert_store_meta(key,value)
		SELECT ?, '1' WHERE EXISTS (SELECT 1 FROM alert_store_meta WHERE key = ? AND value = '1')
		AND NOT EXISTS (SELECT 1 FROM alert_store_meta WHERE key = ?)`, activeCheckpointCompleteKey, activeStateInitializedKey, activeCheckpointProtocolKey); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO alert_store_meta(key,value) VALUES (?, '1')`, activeCheckpointProtocolKey); err != nil {
		return err
	}
	return tx.Commit()
}

// ActiveCheckpointComplete requires a full checkpoint, independently of event
// retention. Legacy authority declarations keep their original contract.
func (s *Store) ActiveCheckpointComplete() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("event log is not enabled")
	}
	var complete int
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM alert_store_meta WHERE key = ? AND value = '1')
		OR (NOT EXISTS(SELECT 1 FROM alert_store_meta WHERE key = ?) AND EXISTS(SELECT 1 FROM alert_store_meta WHERE key = ? AND value = '1'))`, activeCheckpointCompleteKey, activeCheckpointProtocolKey, activeStateInitializedKey).Scan(&complete)
	return complete != 0, err
}

// ActiveStateSnapshot is one current alert occurrence projected alongside the
// append-only lifecycle log. The projection is deliberately independent of
// event retention: an incident that remains active for longer than the event
// retention window must still survive a restart.
type ActiveStateSnapshot struct {
	AlertID             string
	OccurrenceStartedAt time.Time
	UpdatedAt           time.Time
	Snapshot            json.RawMessage
}

// ActiveStateInitialized distinguishes an authoritative empty active set from
// a database created before the active-state projection existed.
func (s *Store) ActiveStateInitialized() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("event log is not enabled")
	}
	var value string
	err := s.db.QueryRow(`SELECT value FROM alert_store_meta WHERE key = ?`, activeStateInitializedKey).Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read active alert state marker: %w", err)
	}
	return value == "1", nil
}

// ActiveStateRevision returns the optimistic-concurrency token for the active
// projection. Every lifecycle transaction and full checkpoint advances it.
func (s *Store) ActiveStateRevision() (int64, error) {
	if s == nil {
		return 0, fmt.Errorf("event log is not enabled")
	}
	var value int64
	err := s.db.QueryRow(`SELECT CAST(value AS INTEGER) FROM alert_store_meta WHERE key = ?`, activeStateRevisionKey).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read active alert state revision: %w", err)
	}
	return value, nil
}

// LoadActiveState returns the current durable active-alert projection.
func (s *Store) LoadActiveState() ([]ActiveStateSnapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("event log is not enabled")
	}
	rows, err := s.db.Query(`
		SELECT alert_id, occurrence_started_at, updated_at, snapshot
		FROM alert_active_state
		ORDER BY alert_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("load active alert state: %w", err)
	}
	defer rows.Close()

	result := make([]ActiveStateSnapshot, 0)
	for rows.Next() {
		var alertID, startedAt, updatedAt, snapshot string
		if err := rows.Scan(&alertID, &startedAt, &updatedAt, &snapshot); err != nil {
			return nil, fmt.Errorf("scan active alert state: %w", err)
		}
		item := ActiveStateSnapshot{
			AlertID:  alertID,
			Snapshot: json.RawMessage(snapshot),
		}
		if startedAt != "" {
			item.OccurrenceStartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
			if err != nil {
				return nil, fmt.Errorf("parse active alert %q occurrence time: %w", alertID, err)
			}
		}
		if updatedAt != "" {
			item.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
			if err != nil {
				return nil, fmt.Errorf("parse active alert %q update time: %w", alertID, err)
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load active alert state: %w", err)
	}
	return result, nil
}

// ReplaceActiveState atomically checkpoints the complete current active set.
// Lifecycle events also update this table inside their own event transaction;
// this checkpoint captures mutable fields such as LastSeen and LastNotified.
func (s *Store) ReplaceActiveState(snapshots []ActiveStateSnapshot) error {
	if s == nil {
		return fmt.Errorf("event log is not enabled")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin active alert checkpoint: %w", err)
	}
	if err := replaceActiveStateTx(tx, snapshots); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit active alert checkpoint: %w", err)
	}
	return nil
}

// ReplaceActiveStateIfRevision checkpoints only if no lifecycle transaction
// has changed the projection since expectedRevision was read. A false result
// is a normal retry signal, not a storage failure.
func (s *Store) ReplaceActiveStateIfRevision(snapshots []ActiveStateSnapshot, expectedRevision int64) (bool, error) {
	if s == nil {
		return false, fmt.Errorf("event log is not enabled")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin active alert checkpoint: %w", err)
	}
	current, err := activeStateRevisionTx(tx)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if current != expectedRevision {
		_ = tx.Rollback()
		return false, nil
	}
	if err := replaceActiveStateTx(tx, snapshots); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit active alert checkpoint: %w", err)
	}
	return true, nil
}

func replaceActiveStateTx(tx *sql.Tx, snapshots []ActiveStateSnapshot) error {
	if _, err := tx.Exec(`DELETE FROM alert_active_state`); err != nil {
		return fmt.Errorf("clear active alert checkpoint: %w", err)
	}
	for _, snapshot := range snapshots {
		if err := upsertActiveSnapshot(tx, snapshot); err != nil {
			return err
		}
	}
	if err := markActiveStateInitialized(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO alert_store_meta(key,value) VALUES (?, '1') ON CONFLICT(key) DO UPDATE SET value='1'`, activeCheckpointCompleteKey); err != nil {
		return err
	}
	return incrementActiveStateRevision(tx)
}

func upsertActiveSnapshot(tx *sql.Tx, snapshot ActiveStateSnapshot) error {
	if snapshot.AlertID == "" {
		return fmt.Errorf("active alert snapshot has no alert identity")
	}
	if len(snapshot.Snapshot) == 0 || !json.Valid(snapshot.Snapshot) {
		return fmt.Errorf("active alert %q has an invalid snapshot", snapshot.AlertID)
	}
	startedAt := ""
	if !snapshot.OccurrenceStartedAt.IsZero() {
		startedAt = snapshot.OccurrenceStartedAt.UTC().Format(time.RFC3339Nano)
	}
	updatedAt := snapshot.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := tx.Exec(`
		INSERT INTO alert_active_state (alert_id, occurrence_started_at, updated_at, snapshot)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(alert_id) DO UPDATE SET
			occurrence_started_at = excluded.occurrence_started_at,
			updated_at = excluded.updated_at,
			snapshot = excluded.snapshot
	`, snapshot.AlertID, startedAt, updatedAt.UTC().Format(time.RFC3339Nano), string(snapshot.Snapshot))
	if err != nil {
		return fmt.Errorf("checkpoint active alert %q: %w", snapshot.AlertID, err)
	}
	return nil
}

func markActiveStateInitialized(tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO alert_store_meta (key, value) VALUES (?, '1')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, activeStateInitializedKey)
	if err != nil {
		return fmt.Errorf("mark active alert state initialized: %w", err)
	}
	return nil
}

func activeStateRevisionTx(tx *sql.Tx) (int64, error) {
	var value int64
	err := tx.QueryRow(`SELECT CAST(value AS INTEGER) FROM alert_store_meta WHERE key = ?`, activeStateRevisionKey).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read active alert state revision: %w", err)
	}
	return value, nil
}

func incrementActiveStateRevision(tx *sql.Tx) error {
	_, err := tx.Exec(`
		INSERT INTO alert_store_meta (key, value) VALUES (?, '1')
		ON CONFLICT(key) DO UPDATE SET value = CAST(value AS INTEGER) + 1
	`, activeStateRevisionKey)
	if err != nil {
		return fmt.Errorf("advance active alert state revision: %w", err)
	}
	return nil
}

type activeSnapshotEnvelope struct {
	StartTime    time.Time  `json:"startTime"`
	LastSeen     time.Time  `json:"lastSeen"`
	LastNotified *time.Time `json:"lastNotified"`
}

func activeSnapshotFromEvent(event Event) (ActiveStateSnapshot, error) {
	if event.AlertID == "" {
		return ActiveStateSnapshot{}, fmt.Errorf("active lifecycle event has no alert identity")
	}
	if len(event.Snapshot) == 0 || !json.Valid(event.Snapshot) {
		return ActiveStateSnapshot{}, fmt.Errorf("active lifecycle event %q has an invalid snapshot", event.AlertID)
	}
	var envelope *activeSnapshotEnvelope
	if err := json.Unmarshal(event.Snapshot, &envelope); err != nil {
		return ActiveStateSnapshot{}, fmt.Errorf("decode active lifecycle snapshot %q: %w", event.AlertID, err)
	}
	if envelope == nil {
		return ActiveStateSnapshot{}, fmt.Errorf("active lifecycle snapshot %q must be an object", event.AlertID)
	}
	return ActiveStateSnapshot{
		AlertID:             event.AlertID,
		OccurrenceStartedAt: envelope.StartTime,
		UpdatedAt:           time.Now().UTC(),
		Snapshot:            event.Snapshot,
	}, nil
}

// projectLifecycleActiveState applies the active-set side of a lifecycle
// transition in the same transaction as its immutable event. Matching a
// resolution by occurrence start prevents a delayed old resolution from
// deleting a newer occurrence that reused the same canonical alert ID.
func projectLifecycleActiveState(tx *sql.Tx, event Event) error {
	// Imported/diagnostic fixtures and legacy callers may record a lifecycle
	// label without a state snapshot. Keep the immutable event, but do not make
	// an unsafe active-set guess from an occurrence we cannot identify.
	if len(event.Snapshot) == 0 {
		return nil
	}
	switch event.Type {
	case TypeActiveMoved:
		snapshot, err := activeSnapshotFromEvent(event)
		if err != nil {
			return err
		}
		previous := event.Details["previousAlertId"]
		if previous == "" || previous == event.AlertID {
			return fmt.Errorf("invalid active identity move")
		}
		startedAt := ""
		if !snapshot.OccurrenceStartedAt.IsZero() {
			startedAt = snapshot.OccurrenceStartedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err := tx.Exec(`DELETE FROM alert_active_state WHERE alert_id = ? AND occurrence_started_at = ?`, previous, startedAt); err != nil {
			return err
		}
		if err := upsertActiveSnapshot(tx, snapshot); err != nil {
			return err
		}
		if err := markActiveStateInitialized(tx); err != nil {
			return err
		}
		return incrementActiveStateRevision(tx)
	case TypeFired, TypeRefired, TypeAcknowledged, TypeUnacknowledged, TypeSnoozed, TypeUnsnoozed, TypeEscalated:
		snapshot, err := activeSnapshotFromEvent(event)
		if err != nil {
			return err
		}
		if event.Type != TypeFired && event.Type != TypeRefired {
			var existing string
			startedAt := ""
			if !snapshot.OccurrenceStartedAt.IsZero() {
				startedAt = snapshot.OccurrenceStartedAt.UTC().Format(time.RFC3339Nano)
			}
			err := tx.QueryRow(`SELECT snapshot FROM alert_active_state WHERE alert_id = ? AND occurrence_started_at = ?`, snapshot.AlertID, startedAt).Scan(&existing)
			if err == sql.ErrNoRows {
				return nil
			}
			if err != nil {
				return err
			}
			// User controls mutate their own fields. A delayed control snapshot
			// cannot roll back newer detector observations or recreate a resolved
			// occurrence after its terminal fact committed.
			var current, incoming map[string]json.RawMessage
			if err := json.Unmarshal([]byte(existing), &current); err != nil {
				return err
			}
			if err := json.Unmarshal(snapshot.Snapshot, &incoming); err != nil {
				return err
			}
			if current == nil || incoming == nil {
				return fmt.Errorf("active control snapshot must be an object")
			}
			var priorObservation, nextObservation activeSnapshotEnvelope
			if err := json.Unmarshal([]byte(existing), &priorObservation); err != nil {
				return err
			}
			if err := json.Unmarshal(snapshot.Snapshot, &nextObservation); err != nil {
				return err
			}
			if nextObservation.LastSeen.After(priorObservation.LastSeen) {
				// A control snapshot can carry a genuinely newer observation. Keep
				// that observation while preserving unrelated control state.
				observed := make(map[string]json.RawMessage, len(incoming))
				for key, value := range incoming {
					observed[key] = value
				}
				for _, key := range []string{"acknowledged", "ackTime", "ackUser", "operationalRecord", "latestTransition", "transitions", "lastEscalation", "escalationTimes"} {
					if value, ok := current[key]; ok {
						observed[key] = value
					} else {
						delete(observed, key)
					}
				}
				if priorObservation.LastNotified != nil && (nextObservation.LastNotified == nil || priorObservation.LastNotified.After(*nextObservation.LastNotified)) {
					observed["lastNotified"] = current["lastNotified"]
				}
				current = observed
			}
			keys := []string{}
			switch event.Type {
			case TypeAcknowledged, TypeUnacknowledged:
				keys = []string{"acknowledged", "ackTime", "ackUser"}
			case TypeSnoozed, TypeUnsnoozed:
				keys = []string{"operationalRecord", "latestTransition", "transitions"}
			case TypeEscalated:
				keys = []string{"lastEscalation", "escalationTimes"}
			}
			for _, key := range keys {
				if value, ok := incoming[key]; ok {
					current[key] = value
				} else {
					delete(current, key)
				}
			}
			snapshot.Snapshot, err = json.Marshal(current)
			if err != nil {
				return err
			}
		}
		if err := upsertActiveSnapshot(tx, snapshot); err != nil {
			return err
		}
		if err := markActiveStateInitialized(tx); err != nil {
			return err
		}
		return incrementActiveStateRevision(tx)
	case TypeResolved, TypeActiveRetired:
		snapshot, err := activeSnapshotFromEvent(event)
		if err != nil {
			return err
		}
		startedAt := ""
		if !snapshot.OccurrenceStartedAt.IsZero() {
			startedAt = snapshot.OccurrenceStartedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err := tx.Exec(`
			DELETE FROM alert_active_state
			WHERE alert_id = ? AND occurrence_started_at = ?
		`, snapshot.AlertID, startedAt); err != nil {
			return fmt.Errorf("resolve active alert %q: %w", snapshot.AlertID, err)
		}
		if err := markActiveStateInitialized(tx); err != nil {
			return err
		}
		return incrementActiveStateRevision(tx)
	default:
		return nil
	}
}
