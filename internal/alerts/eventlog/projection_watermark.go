package eventlog

// Projection watermarks record, per named consumer, the highest durable event
// id whose lifecycle projection has been fully applied. They live in
// alert_store_meta so the cursor travels with the log it indexes: replay walks
// only ids above the watermark instead of the whole history on every boot.
// The cursor does not carry the projected data — wiping a projection store
// (incident history, unified-resource timelines) without wiping this database
// leaves events at or below the watermark unprojected until read-repair or an
// explicit watermark reset.

import (
	"database/sql"
	"fmt"
)

const projectionWatermarkKeyPrefix = "projection_watermark:"

// ProjectionWatermark returns the durable watermark for a named projection
// consumer, or zero when none has been recorded. A nil store reports zero so
// consumers without an event log replay nothing.
func (s *Store) ProjectionWatermark(name string) int64 {
	if s == nil || name == "" {
		return 0
	}
	var value int64
	err := s.db.QueryRow(`SELECT CAST(value AS INTEGER) FROM alert_store_meta WHERE key = ?`, projectionWatermarkKeyPrefix+name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0
	}
	if err != nil || value < 0 {
		return 0
	}
	return value
}

// SetProjectionWatermark durably records the watermark for a named projection
// consumer. Callers may lower it (including to zero) to force a full replay,
// for example after a projection store has been rebuilt.
func (s *Store) SetProjectionWatermark(name string, eventID int64) error {
	if s == nil {
		return nil
	}
	if name == "" {
		return fmt.Errorf("projection watermark name is required")
	}
	if eventID < 0 {
		return fmt.Errorf("projection watermark id must not be negative")
	}
	_, err := s.db.Exec(`
		INSERT INTO alert_store_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, projectionWatermarkKeyPrefix+name, eventID)
	if err != nil {
		return fmt.Errorf("persist projection watermark %q: %w", name, err)
	}
	return nil
}
