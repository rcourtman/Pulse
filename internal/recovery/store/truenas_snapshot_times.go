package store

// correctTrueNASSnapshotCollectionTimes removes a former mapper's synthetic
// completion date. That mapper always paired native creation dates with a
// start date; only its collection-time fallback had a completion but no start.
// Keep every artifact ID, reference and row. Retain the old value explicitly as
// legacy collection evidence, never as a native creation or successful backup.
// This is idempotent and runs before any reads or posture backfill on open.
func (s *Store) correctTrueNASSnapshotCollectionTimes() error {
	_, err := s.db.Exec(`
		UPDATE recovery_points
		SET details_json = CASE WHEN json_valid(details_json) THEN
			CASE WHEN json_type(details_json) = 'object' THEN
				json_set(details_json, '$.legacyCollectedAtMs', completed_at_ms)
			ELSE details_json END
			ELSE details_json END,
			completed_at_ms = NULL,
			outcome = 'unknown'
		WHERE provider = 'truenas' AND kind = 'snapshot' AND mode = 'snapshot'
		  AND outcome = 'success' AND started_at_ms IS NULL
		  AND completed_at_ms IS NOT NULL
	`)
	return err
}
