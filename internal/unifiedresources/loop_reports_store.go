package unifiedresources

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ListResourceOperatorStates returns every persisted operator-set
// state row. Order is not guaranteed.
func (s *SQLiteResourceStore) ListResourceOperatorStates() ([]ResourceOperatorState, error) {
	rows, err := s.db.Query(`
		SELECT canonical_id, intentionally_offline, never_auto_remediate,
			maintenance_start_at, maintenance_end_at, maintenance_reason,
			criticality, note, set_at, set_by
		FROM resource_operator_state`)
	if err != nil {
		return nil, fmt.Errorf("query resource operator states: %w", err)
	}
	defer rows.Close()

	var out []ResourceOperatorState
	for rows.Next() {
		var (
			state          ResourceOperatorState
			intentional    int
			neverRemediate int
			startAt, endAt sql.NullTime
			reason         sql.NullString
			criticality    sql.NullString
			note           sql.NullString
			setBy          sql.NullString
		)
		if err := rows.Scan(
			&state.CanonicalID,
			&intentional,
			&neverRemediate,
			&startAt,
			&endAt,
			&reason,
			&criticality,
			&note,
			&state.SetAt,
			&setBy,
		); err != nil {
			return nil, fmt.Errorf("scan resource operator state row: %w", err)
		}
		state.IntentionallyOffline = intentional != 0
		state.NeverAutoRemediate = neverRemediate != 0
		if startAt.Valid {
			t := startAt.Time
			state.MaintenanceStartAt = &t
		}
		if endAt.Valid {
			t := endAt.Time
			state.MaintenanceEndAt = &t
		}
		if reason.Valid {
			state.MaintenanceReason = reason.String
		}
		if criticality.Valid {
			state.Criticality = ResourceCriticality(criticality.String)
		}
		if note.Valid {
			state.Note = note.String
		}
		if setBy.Valid {
			state.SetBy = setBy.String
		}
		out = append(out, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource operator state rows: %w", err)
	}
	return out, nil
}

// ListResourceOperatorStates returns every persisted operator-set state row.
func (m *MemoryStore) ListResourceOperatorStates() ([]ResourceOperatorState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ResourceOperatorState, 0, len(m.resourceOperatorState))
	for _, state := range m.resourceOperatorState {
		out = append(out, state)
	}
	return out, nil
}

// ErrLoopReportNotFound is returned by store methods that target a
// specific report id which does not exist.
var ErrLoopReportNotFound = errors.New("loop_report_not_found")

// RecordLoopReport persists a new loop report. Reports are immutable
// except for the user_outcome / reviewed_* / review_note fields, which
// are updated via UpdateLoopReportUserOutcome. Tick-vs-tick dedup for
// the (type, scope, window_ended_at) triple is enforced at the
// sentinel layer (mutex + FindLoopReportByWindow) so explicit reruns,
// which intentionally share the triple under a different id suffix,
// can land alongside the original.
func (s *SQLiteResourceStore) RecordLoopReport(report LoopReport) error {
	report = NormalizeLoopReport(report)
	if err := ValidateLoopReport(report); err != nil {
		return err
	}
	evidenceJSON, err := json.Marshal(report.Evidence)
	if err != nil {
		return fmt.Errorf("marshal loop report evidence: %w", err)
	}
	findingIDsJSON, err := json.Marshal(report.LinkedFindingIDs)
	if err != nil {
		return fmt.Errorf("marshal loop report finding ids: %w", err)
	}
	alertIDsJSON, err := json.Marshal(report.LinkedAlertIDs)
	if err != nil {
		return fmt.Errorf("marshal loop report alert ids: %w", err)
	}
	actionIDsJSON, err := json.Marshal(report.LinkedActionIDs)
	if err != nil {
		return fmt.Errorf("marshal loop report action ids: %w", err)
	}
	var windowStart, windowEnd, reviewedAt sql.NullTime
	if report.WindowStartedAt != nil {
		windowStart.Time = *report.WindowStartedAt
		windowStart.Valid = true
	}
	if report.WindowEndedAt != nil {
		windowEnd.Time = *report.WindowEndedAt
		windowEnd.Valid = true
	}
	if report.ReviewedAt != nil {
		reviewedAt.Time = *report.ReviewedAt
		reviewedAt.Valid = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(`
		INSERT INTO loop_reports (
			id, report_type, scope, trigger, goal, status, started_at, completed_at,
			window_started_at, window_ended_at, evidence_json,
			linked_finding_ids_json, linked_alert_ids_json, linked_action_ids_json,
			linked_patrol_run_id, recommendation,
			user_outcome, reviewed_at, reviewed_by, review_note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.ID,
		string(report.Type),
		report.Scope,
		report.Trigger,
		report.Goal,
		string(report.Status),
		report.StartedAt,
		report.CompletedAt,
		windowStart,
		windowEnd,
		string(evidenceJSON),
		string(findingIDsJSON),
		string(alertIDsJSON),
		string(actionIDsJSON),
		report.LinkedPatrolRunID,
		report.Recommendation,
		string(report.UserOutcome),
		reviewedAt,
		report.ReviewedBy,
		report.ReviewNote,
	)
	if err != nil {
		return fmt.Errorf("insert loop report: %w", err)
	}
	return nil
}

// GetLoopReport returns a single report by id.
func (s *SQLiteResourceStore) GetLoopReport(reportID string) (LoopReport, bool, error) {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return LoopReport{}, false, nil
	}
	row := s.db.QueryRow(`
		SELECT id, report_type, scope, trigger, goal, status, started_at, completed_at,
			window_started_at, window_ended_at, evidence_json,
			linked_finding_ids_json, linked_alert_ids_json, linked_action_ids_json,
			linked_patrol_run_id, recommendation,
			user_outcome, reviewed_at, reviewed_by, review_note
		FROM loop_reports
		WHERE id = ?`, reportID)
	report, err := scanLoopReportRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoopReport{}, false, nil
		}
		return LoopReport{}, false, err
	}
	return report, true, nil
}

// FindLoopReportByWindow looks up an existing report for a
// (type, scope, window-end) triple. Used by the sentinel to dedupe
// before writing a new report.
func (s *SQLiteResourceStore) FindLoopReportByWindow(reportType LoopReportType, canonicalID string, windowEndedAt time.Time) (LoopReport, bool, error) {
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID == "" || !IsValidLoopReportType(reportType) || windowEndedAt.IsZero() {
		return LoopReport{}, false, nil
	}
	row := s.db.QueryRow(`
		SELECT id, report_type, scope, trigger, goal, status, started_at, completed_at,
			window_started_at, window_ended_at, evidence_json,
			linked_finding_ids_json, linked_alert_ids_json, linked_action_ids_json,
			linked_patrol_run_id, recommendation,
			user_outcome, reviewed_at, reviewed_by, review_note
		FROM loop_reports
		WHERE report_type = ? AND scope = ? AND window_ended_at = ?
		ORDER BY started_at DESC
		LIMIT 1`, string(reportType), canonicalID, windowEndedAt.UTC())
	report, err := scanLoopReportRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoopReport{}, false, nil
		}
		return LoopReport{}, false, err
	}
	return report, true, nil
}

// ListLoopReportsForResource returns the most recent reports for a
// resource of the given type, newest first. limit=0 returns all rows.
func (s *SQLiteResourceStore) ListLoopReportsForResource(reportType LoopReportType, canonicalID string, limit int) ([]LoopReport, error) {
	canonicalID = CanonicalResourceID(canonicalID)
	if !IsValidLoopReportType(reportType) {
		return nil, nil
	}
	query := `
		SELECT id, report_type, scope, trigger, goal, status, started_at, completed_at,
			window_started_at, window_ended_at, evidence_json,
			linked_finding_ids_json, linked_alert_ids_json, linked_action_ids_json,
			linked_patrol_run_id, recommendation,
			user_outcome, reviewed_at, reviewed_by, review_note
		FROM loop_reports
		WHERE report_type = ? AND scope = ?
		ORDER BY started_at DESC`
	args := []any{string(reportType), canonicalID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query loop reports: %w", err)
	}
	defer rows.Close()

	var out []LoopReport
	for rows.Next() {
		report, err := scanLoopReportRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate loop report rows: %w", err)
	}
	return out, nil
}

// UpdateLoopReportUserOutcome records the operator's review verdict on
// a report. It does not change the underlying status or evidence —
// those are computed by the sentinel and remain immutable.
func (s *SQLiteResourceStore) UpdateLoopReportUserOutcome(reportID string, outcome LoopReportUserOutcome, reviewedBy, note string, reviewedAt time.Time) error {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return fmt.Errorf("%w: id is required", ErrLoopReportInvalid)
	}
	if !IsValidLoopReportUserOutcome(outcome) {
		return fmt.Errorf("%w: unknown user outcome %q", ErrLoopReportInvalid, outcome)
	}
	if reviewedAt.IsZero() {
		reviewedAt = time.Now().UTC()
	} else {
		reviewedAt = reviewedAt.UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		UPDATE loop_reports
		SET user_outcome = ?, reviewed_at = ?, reviewed_by = ?, review_note = ?
		WHERE id = ?`,
		string(outcome),
		reviewedAt,
		strings.TrimSpace(reviewedBy),
		strings.TrimSpace(note),
		reportID,
	)
	if err != nil {
		return fmt.Errorf("update loop report user outcome: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for update loop report: %w", err)
	}
	if affected == 0 {
		return ErrLoopReportNotFound
	}
	return nil
}

// loopReportScanner is the surface RowQuery / Rows share for
// loop-report scanning.
type loopReportScanner interface {
	Scan(dest ...any) error
}

func scanLoopReportRow(scanner loopReportScanner) (LoopReport, error) {
	var (
		r                       LoopReport
		typ, status, outcome    string
		windowStart, windowEnd  sql.NullTime
		reviewedAt              sql.NullTime
		evidenceJSON            string
		findingIDsJSON          string
		alertIDsJSON            string
		actionIDsJSON           string
		linkedPatrolRunID       string
		recommendation, goal    string
		reviewedBy, reviewNote  string
		trigger                 string
	)
	if err := scanner.Scan(
		&r.ID,
		&typ,
		&r.Scope,
		&trigger,
		&goal,
		&status,
		&r.StartedAt,
		&r.CompletedAt,
		&windowStart,
		&windowEnd,
		&evidenceJSON,
		&findingIDsJSON,
		&alertIDsJSON,
		&actionIDsJSON,
		&linkedPatrolRunID,
		&recommendation,
		&outcome,
		&reviewedAt,
		&reviewedBy,
		&reviewNote,
	); err != nil {
		return LoopReport{}, err
	}
	r.Type = LoopReportType(typ)
	r.Trigger = trigger
	r.Goal = goal
	r.Status = LoopReportStatus(status)
	r.UserOutcome = LoopReportUserOutcome(outcome)
	r.LinkedPatrolRunID = linkedPatrolRunID
	r.Recommendation = recommendation
	r.ReviewedBy = reviewedBy
	r.ReviewNote = reviewNote
	if windowStart.Valid {
		t := windowStart.Time.UTC()
		r.WindowStartedAt = &t
	}
	if windowEnd.Valid {
		t := windowEnd.Time.UTC()
		r.WindowEndedAt = &t
	}
	if reviewedAt.Valid {
		t := reviewedAt.Time.UTC()
		r.ReviewedAt = &t
	}
	if evidenceJSON != "" {
		if err := json.Unmarshal([]byte(evidenceJSON), &r.Evidence); err != nil {
			return LoopReport{}, fmt.Errorf("unmarshal loop report evidence: %w", err)
		}
	}
	if findingIDsJSON != "" && findingIDsJSON != "null" {
		if err := json.Unmarshal([]byte(findingIDsJSON), &r.LinkedFindingIDs); err != nil {
			return LoopReport{}, fmt.Errorf("unmarshal loop report finding ids: %w", err)
		}
	}
	if alertIDsJSON != "" && alertIDsJSON != "null" {
		if err := json.Unmarshal([]byte(alertIDsJSON), &r.LinkedAlertIDs); err != nil {
			return LoopReport{}, fmt.Errorf("unmarshal loop report alert ids: %w", err)
		}
	}
	if actionIDsJSON != "" && actionIDsJSON != "null" {
		if err := json.Unmarshal([]byte(actionIDsJSON), &r.LinkedActionIDs); err != nil {
			return LoopReport{}, fmt.Errorf("unmarshal loop report action ids: %w", err)
		}
	}
	r.StartedAt = r.StartedAt.UTC()
	r.CompletedAt = r.CompletedAt.UTC()
	return r, nil
}

// --- MemoryStore implementations ---

// RecordLoopReport stores a loop report in memory. Duplicate (type,
// scope, window-end) triples are rejected with ErrLoopReportInvalid so
// the sentinel's dedupe contract matches the SQLite store.
func (m *MemoryStore) RecordLoopReport(report LoopReport) error {
	report = NormalizeLoopReport(report)
	if err := ValidateLoopReport(report); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loopReports == nil {
		m.loopReports = make(map[string]LoopReport)
	}
	if _, exists := m.loopReports[report.ID]; exists {
		return fmt.Errorf("%w: id %q already exists", ErrLoopReportInvalid, report.ID)
	}
	m.loopReports[report.ID] = report
	return nil
}

// GetLoopReport returns the report by id, if present.
func (m *MemoryStore) GetLoopReport(reportID string) (LoopReport, bool, error) {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return LoopReport{}, false, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	report, ok := m.loopReports[reportID]
	return report, ok, nil
}

// FindLoopReportByWindow scans the in-memory map for a matching report.
func (m *MemoryStore) FindLoopReportByWindow(reportType LoopReportType, canonicalID string, windowEndedAt time.Time) (LoopReport, bool, error) {
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID == "" || !IsValidLoopReportType(reportType) || windowEndedAt.IsZero() {
		return LoopReport{}, false, nil
	}
	target := windowEndedAt.UTC()
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, report := range m.loopReports {
		if report.Type != reportType || report.Scope != canonicalID {
			continue
		}
		if report.WindowEndedAt == nil {
			continue
		}
		if report.WindowEndedAt.Equal(target) {
			return report, true, nil
		}
	}
	return LoopReport{}, false, nil
}

// ListLoopReportsForResource returns matching reports sorted by
// started_at DESC.
func (m *MemoryStore) ListLoopReportsForResource(reportType LoopReportType, canonicalID string, limit int) ([]LoopReport, error) {
	canonicalID = CanonicalResourceID(canonicalID)
	if !IsValidLoopReportType(reportType) {
		return nil, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []LoopReport{}
	for _, report := range m.loopReports {
		if report.Type != reportType || report.Scope != canonicalID {
			continue
		}
		out = append(out, report)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpdateLoopReportUserOutcome records the operator's review verdict.
func (m *MemoryStore) UpdateLoopReportUserOutcome(reportID string, outcome LoopReportUserOutcome, reviewedBy, note string, reviewedAt time.Time) error {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return fmt.Errorf("%w: id is required", ErrLoopReportInvalid)
	}
	if !IsValidLoopReportUserOutcome(outcome) {
		return fmt.Errorf("%w: unknown user outcome %q", ErrLoopReportInvalid, outcome)
	}
	if reviewedAt.IsZero() {
		reviewedAt = time.Now().UTC()
	} else {
		reviewedAt = reviewedAt.UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	report, ok := m.loopReports[reportID]
	if !ok {
		return ErrLoopReportNotFound
	}
	report.UserOutcome = outcome
	report.ReviewedAt = &reviewedAt
	report.ReviewedBy = strings.TrimSpace(reviewedBy)
	report.ReviewNote = strings.TrimSpace(note)
	m.loopReports[reportID] = report
	return nil
}
