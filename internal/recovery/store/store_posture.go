package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

const protectionRefreshChunkSize = 200

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) BackfillProtectionMetadata(ctx context.Context) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	const maxBackfillRows = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, kind, mode, outcome,
		       started_at_ms, completed_at_ms, updated_at_ms,
		       subject_resource_id, subject_ref_json,
		       repository_resource_id, repository_ref_json, details_json,
		       evidence_id, evidence_json
		FROM recovery_points
		WHERE provider_scope IS NULL OR TRIM(provider_scope) = ''
		   OR (
		       (evidence_json IS NULL OR TRIM(evidence_json) = '')
		       AND COALESCE(completed_at_ms, started_at_ms, 0) > 0
		   )
		LIMIT `+fmt.Sprint(maxBackfillRows))
	if err != nil {
		return err
	}

	type backfillRow struct {
		id            string
		provider      string
		kind          string
		mode          string
		outcome       string
		startedMs     sql.NullInt64
		completedMs   sql.NullInt64
		updatedAtMs   int64
		subjectRID    sql.NullString
		subjectRefRaw sql.NullString
		repoRID       sql.NullString
		repoRefRaw    sql.NullString
		detailsRaw    sql.NullString
		evidenceID    sql.NullString
		evidenceRaw   sql.NullString
	}
	items := make([]backfillRow, 0, 256)
	for rows.Next() {
		var item backfillRow
		if err := rows.Scan(
			&item.id,
			&item.provider,
			&item.kind,
			&item.mode,
			&item.outcome,
			&item.startedMs,
			&item.completedMs,
			&item.updatedAtMs,
			&item.subjectRID,
			&item.subjectRefRaw,
			&item.repoRID,
			&item.repoRefRaw,
			&item.detailsRaw,
			&item.evidenceID,
			&item.evidenceRaw,
		); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if len(items) > 0 {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		stmt, err := tx.PrepareContext(
			ctx,
			`UPDATE recovery_points
			 SET provider_scope = ?,
			     evidence_id = CASE
			       WHEN evidence_id IS NULL OR TRIM(evidence_id) = '' THEN ?
			       ELSE evidence_id
			     END,
			     evidence_json = CASE
			       WHEN evidence_json IS NULL OR TRIM(evidence_json) = '' THEN ?
			       ELSE evidence_json
			     END
			 WHERE id = ?`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, item := range items {
			point := recovery.RecoveryPoint{
				ID:                   strings.TrimSpace(item.id),
				Provider:             recovery.Provider(strings.TrimSpace(item.provider)),
				Kind:                 recovery.Kind(strings.TrimSpace(item.kind)),
				Mode:                 recovery.Mode(strings.TrimSpace(item.mode)),
				Outcome:              recovery.Outcome(strings.TrimSpace(item.outcome)),
				StartedAt:            millisToTimePtr(item.startedMs),
				CompletedAt:          millisToTimePtr(item.completedMs),
				SubjectResourceID:    strings.TrimSpace(item.subjectRID.String),
				RepositoryResourceID: strings.TrimSpace(item.repoRID.String),
			}
			_ = unmarshalJSON(item.subjectRefRaw, &point.SubjectRef)
			_ = unmarshalJSON(item.repoRefRaw, &point.RepositoryRef)
			_ = unmarshalJSON(item.detailsRaw, &point.Details)
			var evidenceID any
			var evidenceJSON any
			if strings.TrimSpace(item.evidenceRaw.String) == "" {
				ingestedAt := time.UnixMilli(item.updatedAtMs).UTC()
				evidence, evidenceErr := recovery.NewRecoveryPointEvidence(
					point,
					"recovery-point-migration",
					ingestedAt,
				)
				if evidenceErr == nil {
					encoded, marshalErr := json.Marshal(evidence)
					if marshalErr != nil {
						return marshalErr
					}
					evidenceID = evidence.ID
					evidenceJSON = string(encoded)
				}
			}
			if _, err := stmt.ExecContext(
				ctx,
				recovery.ProviderScopeForPoint(point),
				evidenceID,
				evidenceJSON,
				point.ID,
			); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	keyRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT subject_key
		FROM recovery_points
		WHERE subject_key IS NOT NULL AND TRIM(subject_key) != ''
		ORDER BY subject_key
		LIMIT `+fmt.Sprint(maxBackfillRows))
	if err != nil {
		return err
	}
	keys := make([]string, 0, 512)
	for keyRows.Next() {
		var key string
		if err := keyRows.Scan(&key); err != nil {
			_ = keyRows.Close()
			return err
		}
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	if err := keyRows.Err(); err != nil {
		_ = keyRows.Close()
		return err
	}
	if err := keyRows.Close(); err != nil {
		return err
	}
	return s.RefreshProtectionPostures(ctx, keys)
}

func (s *Store) UpsertProtectionProviderObservations(
	ctx context.Context,
	observations []recovery.ProtectionProviderObservation,
) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if len(observations) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.maybePrune(ctx)

	nowMs := time.Now().UTC().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO protection_provider_observations (
			id, provider, source, scope, job_state,
			history_completeness, permissions, verification_expected,
			observed_at_ms, ingested_at_ms, evidence_json,
			created_at_ms, updated_at_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			provider=excluded.provider,
			source=excluded.source,
			scope=excluded.scope,
			job_state=excluded.job_state,
			history_completeness=excluded.history_completeness,
			permissions=excluded.permissions,
			verification_expected=excluded.verification_expected,
			observed_at_ms=excluded.observed_at_ms,
			ingested_at_ms=excluded.ingested_at_ms,
			evidence_json=excluded.evidence_json,
			updated_at_ms=excluded.updated_at_ms
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	type providerScope struct {
		provider recovery.Provider
		scope    string
	}
	affectedScopes := make(map[providerScope]struct{}, len(observations))
	for _, observation := range observations {
		if err := observation.Validate(); err != nil {
			return err
		}
		evidenceJSON, err := json.Marshal(observation.Evidence)
		if err != nil {
			return err
		}
		verificationExpected := 0
		if observation.VerificationExpected {
			verificationExpected = 1
		}
		if _, err := stmt.ExecContext(
			ctx,
			observation.ID,
			string(observation.Provider),
			observation.Source,
			observation.Scope,
			string(observation.JobState),
			string(observation.HistoryCompleteness),
			string(observation.Permissions),
			verificationExpected,
			observation.ObservedAt.UTC().UnixMilli(),
			observation.IngestedAt.UTC().UnixMilli(),
			string(evidenceJSON),
			nowMs,
			nowMs,
		); err != nil {
			return err
		}
		affectedScopes[providerScope{
			provider: observation.Provider,
			scope:    strings.TrimSpace(observation.Scope),
		}] = struct{}{}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	affectedKeys := make(map[string]struct{})
	for scope := range affectedScopes {
		rows, err := s.db.QueryContext(ctx, `
			SELECT DISTINCT subject_key
			FROM recovery_points
			WHERE provider = ? AND provider_scope = ?
			  AND subject_key IS NOT NULL AND TRIM(subject_key) != ''
		`, string(scope.provider), scope.scope)
		if err != nil {
			return err
		}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				_ = rows.Close()
				return err
			}
			if key = strings.TrimSpace(key); key != "" {
				affectedKeys[key] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return s.RefreshProtectionPostures(ctx, sortedStringSet(affectedKeys))
}

func (s *Store) RefreshProtectionPostures(ctx context.Context, subjectKeys []string) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	subjectKeys = normalizeProtectionStrings(subjectKeys)
	if len(subjectKeys) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	observations, err := s.listLatestProtectionProviderObservations(ctx)
	if err != nil {
		return err
	}
	for start := 0; start < len(subjectKeys); start += protectionRefreshChunkSize {
		end := start + protectionRefreshChunkSize
		if end > len(subjectKeys) {
			end = len(subjectKeys)
		}
		chunk := subjectKeys[start:end]
		pointsByKey, err := s.loadProtectionPointsForKeys(ctx, chunk)
		if err != nil {
			return err
		}
		if err := s.writeProtectionPostures(
			ctx,
			chunk,
			pointsByKey,
			observations,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadProtectionPointsForKeys(
	ctx context.Context,
	subjectKeys []string,
) (map[string][]recovery.RecoveryPoint, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(subjectKeys)), ",")
	args := make([]any, len(subjectKeys))
	for i := range subjectKeys {
		args[i] = subjectKeys[i]
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			subject_key, id, provider, kind, mode, outcome,
			started_at_ms, completed_at_ms, verified,
			subject_resource_id, repository_resource_id,
			subject_ref_json, repository_ref_json, details_json,
			provider_scope, evidence_json
		FROM recovery_points
		WHERE subject_key IN (`+placeholders+`)
		ORDER BY subject_key,
		         COALESCE(completed_at_ms, started_at_ms, updated_at_ms) DESC,
		         updated_at_ms DESC,
		         id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]recovery.RecoveryPoint, len(subjectKeys))
	for rows.Next() {
		var (
			subjectKey             string
			point                  recovery.RecoveryPoint
			provider, kind         string
			mode, outcome          string
			startedMs, completedMs sql.NullInt64
			verified               sql.NullInt64
			subjectRID, repoRID    sql.NullString
			subjectRefRaw          sql.NullString
			repoRefRaw, detailsRaw sql.NullString
			providerScope          sql.NullString
			evidenceRaw            sql.NullString
		)
		if err := rows.Scan(
			&subjectKey,
			&point.ID,
			&provider,
			&kind,
			&mode,
			&outcome,
			&startedMs,
			&completedMs,
			&verified,
			&subjectRID,
			&repoRID,
			&subjectRefRaw,
			&repoRefRaw,
			&detailsRaw,
			&providerScope,
			&evidenceRaw,
		); err != nil {
			return nil, err
		}
		point.Provider = recovery.Provider(provider)
		point.Kind = recovery.Kind(kind)
		point.Mode = recovery.Mode(mode)
		point.Outcome = recovery.Outcome(outcome)
		point.StartedAt = millisToTimePtr(startedMs)
		point.CompletedAt = millisToTimePtr(completedMs)
		point.Verified = dbToBoolPtr(verified)
		point.SubjectResourceID = strings.TrimSpace(subjectRID.String)
		point.RepositoryResourceID = strings.TrimSpace(repoRID.String)
		point.ProviderScope = strings.TrimSpace(providerScope.String)
		_ = unmarshalJSON(subjectRefRaw, &point.SubjectRef)
		_ = unmarshalJSON(repoRefRaw, &point.RepositoryRef)
		_ = unmarshalJSON(detailsRaw, &point.Details)
		var evidence operationaltrust.EvidenceEnvelope
		_ = unmarshalJSON(evidenceRaw, &evidence)
		if strings.TrimSpace(evidence.ID) != "" {
			point.Evidence = &evidence
		}
		subjectKey = strings.TrimSpace(subjectKey)
		out[subjectKey] = append(out[subjectKey], point)
	}
	return out, rows.Err()
}

func (s *Store) listLatestProtectionProviderObservations(
	ctx context.Context,
) ([]recovery.ProtectionProviderObservation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id, provider, source, scope, job_state,
			history_completeness, permissions, verification_expected,
			observed_at_ms, ingested_at_ms, evidence_json
		FROM (
			SELECT *,
			       ROW_NUMBER() OVER (
				       PARTITION BY provider, scope
				       ORDER BY observed_at_ms DESC, id DESC
			       ) AS rn
			FROM protection_provider_observations
		)
		WHERE rn = 1
		ORDER BY provider, scope
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]recovery.ProtectionProviderObservation, 0, 16)
	for rows.Next() {
		var (
			observation                      recovery.ProtectionProviderObservation
			provider, jobState               string
			historyCompleteness, permissions string
			verificationExpected             int
			observedAtMs, ingestedAtMs       int64
			evidenceRaw                      string
		)
		if err := rows.Scan(
			&observation.ID,
			&provider,
			&observation.Source,
			&observation.Scope,
			&jobState,
			&historyCompleteness,
			&permissions,
			&verificationExpected,
			&observedAtMs,
			&ingestedAtMs,
			&evidenceRaw,
		); err != nil {
			return nil, err
		}
		observation.Provider = recovery.Provider(provider)
		observation.JobState = recovery.Outcome(jobState)
		observation.HistoryCompleteness =
			recovery.ProtectionHistoryCompleteness(historyCompleteness)
		observation.Permissions = operationaltrust.EvidencePermissions(permissions)
		observation.VerificationExpected = verificationExpected != 0
		observation.ObservedAt = time.UnixMilli(observedAtMs).UTC()
		observation.IngestedAt = time.UnixMilli(ingestedAtMs).UTC()
		if err := json.Unmarshal([]byte(evidenceRaw), &observation.Evidence); err != nil {
			continue
		}
		if err := observation.Validate(); err != nil {
			continue
		}
		out = append(out, observation)
	}
	return out, rows.Err()
}

func (s *Store) writeProtectionPostures(
	ctx context.Context,
	subjectKeys []string,
	pointsByKey map[string][]recovery.RecoveryPoint,
	observations []recovery.ProtectionProviderObservation,
) error {
	now := time.Now().UTC()
	nowMs := now.UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	upsert, err := tx.PrepareContext(ctx, `
		INSERT INTO protection_postures (
			subject_key, subject_resource_id, state,
			posture_json, evaluated_at_ms, updated_at_ms
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(subject_key) DO UPDATE SET
			subject_resource_id=excluded.subject_resource_id,
			state=excluded.state,
			posture_json=excluded.posture_json,
			evaluated_at_ms=excluded.evaluated_at_ms,
			updated_at_ms=excluded.updated_at_ms
	`)
	if err != nil {
		return err
	}
	defer upsert.Close()

	for _, key := range subjectKeys {
		points := pointsByKey[key]
		subjectResourceID := ""
		for _, point := range points {
			if value := strings.TrimSpace(point.SubjectResourceID); value != "" {
				subjectResourceID = value
				break
			}
		}
		if subjectResourceID == "" {
			if _, err := tx.ExecContext(
				ctx,
				`DELETE FROM protection_postures WHERE subject_key = ?`,
				key,
			); err != nil {
				return err
			}
			continue
		}
		posture := recovery.BuildProtectionPostureFromPointsAt(
			subjectResourceID,
			points,
			observations,
			recovery.DefaultProtectionPosturePolicy,
			now,
		)
		if err := posture.Validate(); err != nil {
			return err
		}
		postureJSON, err := json.Marshal(posture)
		if err != nil {
			return err
		}
		if _, err := upsert.ExecContext(
			ctx,
			key,
			subjectResourceID,
			string(posture.State),
			string(postureJSON),
			posture.EvaluatedAt.UTC().UnixMilli(),
			nowMs,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListProtectionPostures(
	ctx context.Context,
	query recovery.ProtectionPostureQuery,
) ([]recovery.ProtectionPosture, int, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, 0, err
	}
	if query.State != "" && !query.State.Valid() {
		return nil, 0, fmt.Errorf("invalid protection posture state %q", query.State)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	requestedIDs := normalizeProtectionStrings(query.SubjectResourceIDs)
	if len(requestedIDs) > 0 {
		return s.listRequestedProtectionPostures(ctx, requestedIDs, query.State)
	}

	limit := normalizeLimit(query.Limit)
	page := normalizePage(query.Page)
	offset := (page - 1) * limit
	whereSQL := ""
	args := make([]any, 0, 3)
	if query.State != "" {
		whereSQL = "WHERE state = ?"
		args = append(args, string(query.State))
	}
	var total int
	if err := s.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM protection_postures "+whereSQL,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT posture_json
		FROM protection_postures
		`+whereSQL+`
		ORDER BY
			CASE state
				WHEN 'attention' THEN 0
				WHEN 'unprotected' THEN 1
				WHEN 'unknown' THEN 2
				ELSE 3
			END,
			evaluated_at_ms DESC,
			subject_resource_id
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	postures := make([]recovery.ProtectionPosture, 0, limit)
	for rows.Next() {
		var postureJSON string
		if err := rows.Scan(&postureJSON); err != nil {
			return nil, 0, err
		}
		var posture recovery.ProtectionPosture
		if err := json.Unmarshal([]byte(postureJSON), &posture); err != nil {
			return nil, 0, err
		}
		postures = append(postures, posture)
	}
	return postures, total, rows.Err()
}

func (s *Store) listRequestedProtectionPostures(
	ctx context.Context,
	requestedIDs []string,
	state recovery.ProtectionState,
) ([]recovery.ProtectionPosture, int, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(requestedIDs)), ",")
	args := make([]any, len(requestedIDs))
	for i := range requestedIDs {
		args[i] = requestedIDs[i]
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT subject_key, subject_resource_id
		FROM protection_postures
		WHERE subject_resource_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	subjectKeyByID := make(map[string]string, len(requestedIDs))
	subjectKeys := make([]string, 0, len(requestedIDs))
	for rows.Next() {
		var subjectKey, subjectResourceID string
		if err := rows.Scan(&subjectKey, &subjectResourceID); err != nil {
			return nil, 0, err
		}
		subjectKey = strings.TrimSpace(subjectKey)
		subjectResourceID = strings.TrimSpace(subjectResourceID)
		if subjectKey == "" || subjectResourceID == "" {
			continue
		}
		subjectKeyByID[subjectResourceID] = subjectKey
		subjectKeys = append(subjectKeys, subjectKey)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}

	now := time.Now().UTC()
	pointsByKey := make(map[string][]recovery.RecoveryPoint)
	var observations []recovery.ProtectionProviderObservation
	if len(subjectKeys) > 0 {
		pointsByKey, err = s.loadProtectionPointsForKeys(ctx, normalizeProtectionStrings(subjectKeys))
		if err != nil {
			return nil, 0, err
		}
		observations, err = s.listLatestProtectionProviderObservations(ctx)
		if err != nil {
			return nil, 0, err
		}
	}
	out := make([]recovery.ProtectionPosture, 0, len(requestedIDs))
	for _, subjectResourceID := range requestedIDs {
		subjectKey := subjectKeyByID[subjectResourceID]
		var posture recovery.ProtectionPosture
		if subjectKey == "" {
			posture = recovery.DeriveProtectionPostureAt(
				subjectResourceID,
				nil,
				recovery.DefaultProtectionPosturePolicy,
				now,
			)
		} else {
			posture = recovery.BuildProtectionPostureFromPointsAt(
				subjectResourceID,
				pointsByKey[subjectKey],
				observations,
				recovery.DefaultProtectionPosturePolicy,
				now,
			)
		}
		if state != "" && posture.State != state {
			continue
		}
		out = append(out, posture)
	}
	return out, len(out), nil
}

func normalizeProtectionStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
