package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
)

const backupHistorySchema = `
 CREATE TABLE IF NOT EXISTS backup_history_sources (
  provider TEXT NOT NULL, instance TEXT NOT NULL, node TEXT NOT NULL,
  first_ms INTEGER NOT NULL, last_ms INTEGER NOT NULL,
  complete INTEGER NOT NULL, reason TEXT NOT NULL, range_start_ms INTEGER NOT NULL,
  PRIMARY KEY(provider, instance, node)
 );
 CREATE TABLE IF NOT EXISTS backup_history_intervals (
  provider TEXT NOT NULL, instance TEXT NOT NULL, node TEXT NOT NULL,
  start_ms INTEGER NOT NULL, end_ms INTEGER NOT NULL,
  PRIMARY KEY(provider, instance, node, start_ms)
 );
 CREATE INDEX IF NOT EXISTS idx_backup_history_interval_end ON backup_history_intervals(end_ms);
`

// RecordBackupHistory commits task evidence before recording collection success.
// A crash or failure between these writes can only reduce declared coverage.
// Neither an in-memory queue nor an inventory reconciliation owns this history.
func (s *Store) RecordBackupHistory(ctx context.Context, points []recovery.RecoveryPoint, collections []model.BackupHistoryCollection) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	s.backupHistoryMu.Lock()
	defer s.backupHistoryMu.Unlock()
	if len(collections) != 1 {
		return fmt.Errorf("backup history requires exactly one source collection")
	}
	for _, c := range collections {
		if c.Provider == "" || strings.TrimSpace(c.Instance) == "" || c.ObservedAt.IsZero() || c.MaxGap <= 0 {
			return fmt.Errorf("invalid backup history collection")
		}
	}
	for _, c := range collections {
		var last int64
		err := s.db.QueryRowContext(ctx, "SELECT last_ms FROM backup_history_sources WHERE provider=? AND instance=? AND node=?", c.Provider, c.Instance, c.Node).Scan(&last)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && c.ObservedAt.UnixMilli() <= last {
			return nil
		}
	}
	// A repeated running observation must not erase a previously committed
	// terminal outcome. Terminal corrections remain eligible on later polls.
	accepted := make([]recovery.RecoveryPoint, 0, len(points))
	for _, point := range points {
		instance, _ := point.Details["instance"].(string)
		if point.Provider != collections[0].Provider || instance != collections[0].Instance {
			return fmt.Errorf("backup task belongs to another source collection")
		}
		if !strings.HasPrefix(point.ID, "pve-task:") && !strings.HasPrefix(point.ID, "pbs-task:") {
			return fmt.Errorf("backup history accepts execution evidence only")
		}
		if point.Kind != recovery.KindOther {
			return fmt.Errorf("backup execution is not a restore artifact")
		}
		if point.CompletedAt == nil {
			var completed sql.NullInt64
			err := s.db.QueryRowContext(ctx, "SELECT completed_at_ms FROM recovery_points WHERE id = ?", point.ID).Scan(&completed)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if completed.Valid {
				continue
			}
		}
		accepted = append(accepted, point)
	}
	if err := s.UpsertPoints(ctx, accepted); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range collections {
		if c.Complete {
			c.Reason = ""
		}
		at := c.ObservedAt.UTC().UnixMilli()
		var first, last, previousRange int64
		var complete bool
		err := tx.QueryRowContext(ctx, `SELECT first_ms,last_ms,complete,range_start_ms FROM backup_history_sources WHERE provider=? AND instance=? AND node=?`, c.Provider, c.Instance, c.Node).Scan(&first, &last, &complete, &previousRange)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && at <= last {
			continue
		}
		if first == 0 {
			first = at
		}
		rangeStart := at
		if c.Complete {
			if complete && last > 0 && at-last <= c.MaxGap.Milliseconds() {
				rangeStart = previousRange
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO backup_history_intervals(provider,instance,node,start_ms,end_ms) VALUES(?,?,?,?,?) ON CONFLICT(provider,instance,node,start_ms) DO UPDATE SET end_ms=excluded.end_ms`, c.Provider, c.Instance, c.Node, rangeStart, at); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO backup_history_sources(provider,instance,node,first_ms,last_ms,complete,reason,range_start_ms) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(provider,instance,node) DO UPDATE SET last_ms=excluded.last_ms,complete=excluded.complete,reason=excluded.reason,range_start_ms=excluded.range_start_ms`, c.Provider, c.Instance, c.Node, first, at, c.Complete, c.Reason, rangeStart); err != nil {
			return err
		}
	}
	cutoff := time.Now().UTC().Add(-s.retention).UnixMilli()
	if _, err := tx.ExecContext(ctx, "DELETE FROM backup_history_intervals WHERE end_ms < ?", cutoff); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) BackupHistoryCoverage(ctx context.Context, from, to time.Time) (model.BackupHistoryCoverage, error) {
	if err := s.ensureInitialized(); err != nil {
		return model.BackupHistoryCoverage{}, err
	}
	return readBackupHistoryCoverage(ctx, s.db, from, to)
}

type backupHistoryReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// BackupHistoryCollectionStart bounds later reads to the durable collection
// watermark, with a day of overlap for delayed task publication. Known pending
// jobs keep their original start in range so a long backup can still finish.
// This never changes observation coverage or backdates a successful collection.
func (s *Store) BackupHistoryCollectionStart(ctx context.Context, provider recovery.Provider, instance string, now time.Time) (time.Time, error) {
	floor := now.UTC().Add(-s.retention)
	if err := s.ensureInitialized(); err != nil {
		return floor, err
	}
	var last int64
	var complete bool
	var reason string
	err := s.db.QueryRowContext(ctx, `SELECT last_ms,complete,reason FROM backup_history_sources WHERE provider=? AND instance=? AND node=''`, provider, instance).Scan(&last, &complete, &reason)
	if err != nil && err != sql.ErrNoRows {
		return floor, err
	}
	if !complete && reason != "history_truncated" {
		var qualified sql.NullInt64
		if err := s.db.QueryRowContext(ctx, `SELECT MAX(end_ms) FROM backup_history_intervals WHERE provider=? AND instance=? AND node=''`, provider, instance).Scan(&qualified); err != nil {
			return floor, err
		}
		last = qualified.Int64
	}
	start := floor
	if last > 0 {
		candidate := time.UnixMilli(last).UTC().Add(-24 * time.Hour)
		if candidate.After(start) {
			start = candidate
		}
	}
	var pending sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT MIN(started_at_ms) FROM recovery_points WHERE provider=? AND kind=? AND id LIKE 'pve-task:%' AND completed_at_ms IS NULL AND started_at_ms>=? AND json_extract(details_json,'$.instance')=?`, provider, recovery.KindOther, floor.UnixMilli(), instance).Scan(&pending)
	if err != nil {
		return floor, err
	}
	if pending.Valid && pending.Int64 < start.UnixMilli() {
		start = time.UnixMilli(pending.Int64).UTC()
	}
	return start, nil
}

func readBackupHistoryCoverage(ctx context.Context, reader backupHistoryReader, from, to time.Time) (model.BackupHistoryCoverage, error) {
	coverage := model.BackupHistoryCoverage{Sources: []model.BackupHistorySource{}, Intervals: []model.BackupHistoryInterval{}}
	rows, err := reader.QueryContext(ctx, `SELECT provider,instance,node,first_ms,last_ms,complete,reason FROM backup_history_sources ORDER BY provider,instance,node`)
	if err != nil {
		return coverage, err
	}
	for rows.Next() {
		var source model.BackupHistorySource
		var first, last int64
		if err := rows.Scan(&source.Provider, &source.Instance, &source.Node, &first, &last, &source.Complete, &source.Reason); err != nil {
			rows.Close()
			return coverage, err
		}
		source.FirstObservedAt, source.LastObservedAt = time.UnixMilli(first).UTC(), time.UnixMilli(last).UTC()
		coverage.Sources = append(coverage.Sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return coverage, err
	}
	rows, err = reader.QueryContext(ctx, `SELECT provider,instance,node,start_ms,end_ms FROM backup_history_intervals WHERE end_ms>=? AND start_ms<=? ORDER BY provider,instance,node,start_ms`, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return coverage, err
	}
	defer rows.Close()
	for rows.Next() {
		var interval model.BackupHistoryInterval
		var start, end int64
		if err := rows.Scan(&interval.Provider, &interval.Instance, &interval.Node, &start, &end); err != nil {
			return coverage, err
		}
		interval.Start, interval.End = time.UnixMilli(start).UTC(), time.UnixMilli(end).UTC()
		coverage.Intervals = append(coverage.Intervals, interval)
	}
	return coverage, rows.Err()
}

// BackupHealthHistory reads outcomes and coverage from one SQLite snapshot.
// A concurrent poll cannot qualify coverage over an older set of task rows.
func (s *Store) BackupHealthHistory(ctx context.Context, from, to time.Time) ([]recovery.RecoveryPoint, model.BackupHistoryCoverage, error) {
	var coverage model.BackupHistoryCoverage
	if err := s.ensureInitialized(); err != nil {
		return nil, coverage, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, coverage, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,outcome,started_at_ms,completed_at_ms,subject_resource_id,details_json,evidence_json FROM recovery_points WHERE provider=? AND kind=? AND id LIKE 'pve-task:%' AND started_at_ms<? AND COALESCE(completed_at_ms,started_at_ms,updated_at_ms)>=? ORDER BY id`, recovery.ProviderProxmoxPVE, recovery.KindOther, to.UnixMilli(), time.Now().UTC().Add(-s.retention).UnixMilli())
	if err != nil {
		return nil, coverage, err
	}
	points := []recovery.RecoveryPoint{}
	for rows.Next() {
		if len(points) >= 250000 {
			rows.Close()
			return nil, coverage, fmt.Errorf("backup execution history exceeds report query limit")
		}
		point := recovery.RecoveryPoint{Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther}
		var start, end sql.NullInt64
		var subject, details, evidence sql.NullString
		if err := rows.Scan(&point.ID, &point.Outcome, &start, &end, &subject, &details, &evidence); err != nil {
			rows.Close()
			return nil, coverage, err
		}
		if start.Valid {
			value := time.UnixMilli(start.Int64).UTC()
			point.StartedAt = &value
		}
		if end.Valid {
			value := time.UnixMilli(end.Int64).UTC()
			point.CompletedAt = &value
		}
		point.SubjectResourceID = subject.String
		if details.Valid {
			if err := json.Unmarshal([]byte(details.String), &point.Details); err != nil {
				rows.Close()
				return nil, coverage, err
			}
		}
		if evidence.Valid {
			point.Evidence = &operationaltrust.EvidenceEnvelope{}
			if err := json.Unmarshal([]byte(evidence.String), point.Evidence); err != nil {
				rows.Close()
				return nil, coverage, err
			}
		}
		points = append(points, point)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, coverage, err
	}
	coverage, err = readBackupHistoryCoverage(ctx, tx, from, to)
	if err != nil {
		return nil, coverage, err
	}
	if err := tx.Commit(); err != nil {
		return nil, coverage, err
	}
	return points, coverage, nil
}
