package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// BackfillIndex populates normalized index columns for rows that predate those columns.
// It is safe to call multiple times.
func (s *Store) BackfillIndex(ctx context.Context) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Keep bounded: if this is slow, it should not block startup.
	const maxBackfillRows = 3000

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, provider, kind, mode, outcome,
			subject_resource_id, repository_resource_id,
			subject_ref_json, repository_ref_json, details_json
		FROM recovery_points
		WHERE
			(subject_label IS NULL OR subject_label = '' OR
			 subject_type IS NULL OR
			 cluster_label IS NULL OR
			 node_host_label IS NULL OR
			 namespace_label IS NULL OR
			 entity_id_label IS NULL OR
			 repository_label IS NULL OR
			 details_summary IS NULL)
		LIMIT `+fmt.Sprint(maxBackfillRows)+`
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id            string
		provider      string
		kind          string
		mode          string
		outcome       string
		subjectRID    sql.NullString
		repositoryRID sql.NullString
		subjectRefRaw sql.NullString
		repoRefRaw    sql.NullString
		detailsRaw    sql.NullString
	}

	items := make([]row, 0, 256)
	for rows.Next() {
		var r row
		if err := rows.Scan(
			&r.id,
			&r.provider,
			&r.kind,
			&r.mode,
			&r.outcome,
			&r.subjectRID,
			&r.repositoryRID,
			&r.subjectRefRaw,
			&r.repoRefRaw,
			&r.detailsRaw,
		); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE recovery_points
		SET
			subject_label = ?,
			subject_type = ?,
			is_workload = ?,
			cluster_label = ?,
			node_host_label = ?,
			namespace_label = ?,
			entity_id_label = ?,
			repository_label = ?,
			details_summary = ?
		WHERE id = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		p := recovery.RecoveryPoint{
			ID:                   strings.TrimSpace(item.id),
			Provider:             recovery.Provider(strings.TrimSpace(item.provider)),
			Kind:                 recovery.Kind(strings.TrimSpace(item.kind)),
			Mode:                 recovery.Mode(strings.TrimSpace(item.mode)),
			Outcome:              recovery.Outcome(strings.TrimSpace(item.outcome)),
			SubjectResourceID:    strings.TrimSpace(item.subjectRID.String),
			RepositoryResourceID: strings.TrimSpace(item.repositoryRID.String),
		}

		var subjectRef recovery.ExternalRef
		_ = unmarshalJSON(item.subjectRefRaw, &subjectRef) // best-effort
		if strings.TrimSpace(subjectRef.Type) != "" {
			p.SubjectRef = &subjectRef
		}

		var repoRef recovery.ExternalRef
		_ = unmarshalJSON(item.repoRefRaw, &repoRef) // best-effort
		if strings.TrimSpace(repoRef.Type) != "" {
			p.RepositoryRef = &repoRef
		}

		var details map[string]any
		_ = unmarshalJSON(item.detailsRaw, &details) // best-effort
		if len(details) > 0 {
			p.Details = details
		}

		idx := recovery.DeriveIndex(p)
		isWorkload := 0
		if idx.IsWorkload {
			isWorkload = 1
		}

		if _, err := stmt.ExecContext(
			ctx,
			strings.TrimSpace(idx.SubjectLabel),
			strings.TrimSpace(idx.SubjectType),
			isWorkload,
			strings.TrimSpace(idx.ClusterLabel),
			strings.TrimSpace(idx.NodeHostLabel),
			strings.TrimSpace(idx.NamespaceLabel),
			strings.TrimSpace(idx.EntityIDLabel),
			strings.TrimSpace(idx.RepositoryLabel),
			strings.TrimSpace(idx.DetailsSummary),
			strings.TrimSpace(item.id),
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
