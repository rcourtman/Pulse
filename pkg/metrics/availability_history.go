package metrics

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	pdb "github.com/rcourtman/pulse-go-rewrite/pkg/db"
	"github.com/rs/zerolog/log"
)

// AvailabilityOutcome is the closed categorical vocabulary retained by the
// history store. It deliberately does not reuse numeric metric rows: averaging
// these values would erase indeterminate and unknown time.
type AvailabilityOutcome string

const (
	AvailabilityReachable     AvailabilityOutcome = "reachable"
	AvailabilityUnreachable   AvailabilityOutcome = "unreachable"
	AvailabilityIndeterminate AvailabilityOutcome = "indeterminate"
)

// AvailabilityExecutionSource identifies where an observation executed
// without retaining the assigned agent's identity.
type AvailabilityExecutionSource string

const (
	AvailabilitySourceLocal         AvailabilityExecutionSource = "local"
	AvailabilitySourceAssignedAgent AvailabilityExecutionSource = "assigned_agent"
)

const (
	availabilityRawRetention    = 48 * time.Hour
	availabilityMinuteRetention = 8 * 24 * time.Hour
	availabilityHourlyRetention = 92 * 24 * time.Hour
	availabilityDailyRetention  = 366 * 24 * time.Hour
	availabilityMaxValidity     = 24 * time.Hour
	availabilityMaxBatchTargets = 200
	availabilityMaxBuckets      = 120
)

// AvailabilityObservation is one accepted scheduled result. TimelineAt is
// server-authored and controls coverage; ObservedAt is evidence metadata only.
type AvailabilityObservation struct {
	ObservationID   string
	TargetID        string
	ConfigRevision  int64
	Outcome         AvailabilityOutcome
	ObservedAt      time.Time
	TimelineAt      time.Time
	IngestedAt      time.Time
	ValidFor        time.Duration
	ExecutionSource AvailabilityExecutionSource
	LatencyMillis   *int64
}

// AvailabilityLatencySummary contains reachable-only latency evidence.
type AvailabilityLatencySummary struct {
	Average float64 `json:"average"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
}

// AvailabilityHistorySummary is derived from state durations over the exact
// requested window. AvailabilityPercent is absent without determinate time.
type AvailabilityHistorySummary struct {
	ReachableSeconds       float64                     `json:"reachableSeconds"`
	UnreachableSeconds     float64                     `json:"unreachableSeconds"`
	IndeterminateSeconds   float64                     `json:"indeterminateSeconds"`
	UnknownSeconds         float64                     `json:"unknownSeconds"`
	CoveragePercent        float64                     `json:"coveragePercent"`
	AvailabilityPercent    *float64                    `json:"availabilityPercent,omitempty"`
	ReachableLatencyMillis *AvailabilityLatencySummary `json:"reachableLatencyMillis,omitempty"`
}

// AvailabilityHistoryBucket is one chronological fleet-view bucket.
type AvailabilityHistoryBucket struct {
	Start                time.Time                   `json:"start"`
	End                  time.Time                   `json:"end"`
	ReachableSeconds     float64                     `json:"reachableSeconds"`
	UnreachableSeconds   float64                     `json:"unreachableSeconds"`
	IndeterminateSeconds float64                     `json:"indeterminateSeconds"`
	UnknownSeconds       float64                     `json:"unknownSeconds"`
	LatencyMillis        *AvailabilityLatencySummary `json:"latencyMillis,omitempty"`
}

// AvailabilityRevisionBoundary prevents a chart from implying an unchanged
// check across execution-defining configuration edits.
type AvailabilityRevisionBoundary struct {
	Revision int64     `json:"revision"`
	At       time.Time `json:"at"`
}

// AvailabilityHistoryTarget is the source-owned result for one target.
type AvailabilityHistoryTarget struct {
	TargetID           string                         `json:"targetId"`
	Summary            AvailabilityHistorySummary     `json:"summary"`
	Buckets            []AvailabilityHistoryBucket    `json:"buckets"`
	RevisionBoundaries []AvailabilityRevisionBoundary `json:"revisionBoundaries"`
}

type availabilityAggregate struct {
	reachableMillis     int64
	unreachableMillis   int64
	indeterminateMillis int64
	latencyCount        int64
	latencySum          int64
	latencyMin          int64
	latencyMax          int64
}

func (s *Store) initAvailabilityHistorySchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS availability_observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			observation_id TEXT NOT NULL UNIQUE,
			target_id TEXT NOT NULL,
			config_revision INTEGER NOT NULL,
			outcome TEXT NOT NULL CHECK(outcome IN ('reachable', 'unreachable', 'indeterminate')),
			observed_at_ns INTEGER NOT NULL,
			timeline_at_ns INTEGER NOT NULL,
			ingested_at_ns INTEGER NOT NULL,
			valid_until_ns INTEGER NOT NULL,
			execution_source TEXT NOT NULL CHECK(execution_source IN ('local', 'assigned_agent')),
			latency_millis INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_availability_observations_target_timeline
			ON availability_observations(target_id, timeline_at_ns, id);
		CREATE INDEX IF NOT EXISTS idx_availability_observations_retention
			ON availability_observations(timeline_at_ns);

		CREATE TABLE IF NOT EXISTS availability_history_buckets (
			target_id TEXT NOT NULL,
			tier TEXT NOT NULL CHECK(tier IN ('minute', 'hourly', 'daily')),
			bucket_start INTEGER NOT NULL,
			reachable_millis INTEGER NOT NULL DEFAULT 0,
			unreachable_millis INTEGER NOT NULL DEFAULT 0,
			indeterminate_millis INTEGER NOT NULL DEFAULT 0,
			reachable_count INTEGER NOT NULL DEFAULT 0,
			unreachable_count INTEGER NOT NULL DEFAULT 0,
			indeterminate_count INTEGER NOT NULL DEFAULT 0,
			latency_count INTEGER NOT NULL DEFAULT 0,
			latency_sum INTEGER NOT NULL DEFAULT 0,
			latency_min INTEGER,
			latency_max INTEGER,
			PRIMARY KEY(target_id, tier, bucket_start)
		);
		CREATE INDEX IF NOT EXISTS idx_availability_buckets_retention
			ON availability_history_buckets(tier, bucket_start);

		CREATE TABLE IF NOT EXISTS availability_revision_boundaries (
			target_id TEXT NOT NULL,
			revision INTEGER NOT NULL,
			started_at_ns INTEGER NOT NULL,
			PRIMARY KEY(target_id, revision)
		);
		CREATE INDEX IF NOT EXISTS idx_availability_boundaries_target_time
			ON availability_revision_boundaries(target_id, started_at_ns);
	`)
	if err != nil {
		return fmt.Errorf("create availability history schema: %w", err)
	}
	return nil
}

func normalizeAvailabilityObservation(observation AvailabilityObservation) (AvailabilityObservation, error) {
	observation.ObservationID = strings.TrimSpace(observation.ObservationID)
	observation.TargetID = strings.TrimSpace(observation.TargetID)
	if observation.ObservationID == "" || observation.TargetID == "" {
		return AvailabilityObservation{}, fmt.Errorf("availability observation and target ids are required")
	}
	if observation.ConfigRevision <= 0 {
		observation.ConfigRevision = 1
	}
	switch observation.Outcome {
	case AvailabilityReachable, AvailabilityUnreachable, AvailabilityIndeterminate:
	default:
		return AvailabilityObservation{}, fmt.Errorf("unsupported availability outcome %q", observation.Outcome)
	}
	switch observation.ExecutionSource {
	case AvailabilitySourceLocal, AvailabilitySourceAssignedAgent:
	default:
		return AvailabilityObservation{}, fmt.Errorf("unsupported availability execution source %q", observation.ExecutionSource)
	}
	if observation.TimelineAt.IsZero() || observation.IngestedAt.IsZero() {
		return AvailabilityObservation{}, fmt.Errorf("availability timeline and ingestion times are required")
	}
	observation.TimelineAt = observation.TimelineAt.UTC()
	observation.IngestedAt = observation.IngestedAt.UTC()
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = observation.TimelineAt
	} else {
		observation.ObservedAt = observation.ObservedAt.UTC()
	}
	if observation.ValidFor <= 0 {
		return AvailabilityObservation{}, fmt.Errorf("availability validity window must be positive")
	}
	if observation.ValidFor > availabilityMaxValidity {
		observation.ValidFor = availabilityMaxValidity
	}
	if observation.LatencyMillis != nil {
		latency := *observation.LatencyMillis
		if observation.Outcome != AvailabilityReachable || latency < 0 {
			observation.LatencyMillis = nil
		} else if latency > int64(availabilityMaxValidity/time.Millisecond) {
			latency = int64(availabilityMaxValidity / time.Millisecond)
			observation.LatencyMillis = &latency
		}
	}
	return observation, nil
}

// WriteAvailabilityObservationSync is the read-your-writes test and seeding
// path. Live monitoring uses the bounded variant below.
func (s *Store) WriteAvailabilityObservationSync(observation AvailabilityObservation) error {
	normalized, err := normalizeAvailabilityObservation(observation)
	if err != nil {
		return err
	}
	s.enqueueAndWait(writeRequest{availability: []AvailabilityObservation{normalized}})
	return nil
}

// WriteAvailabilityObservationBounded keeps a slow history disk from blocking
// polling. Once queued, the observation remains ordered with numeric writes.
func (s *Store) WriteAvailabilityObservationBounded(observation AvailabilityObservation) error {
	normalized, err := normalizeAvailabilityObservation(observation)
	if err != nil {
		return err
	}
	s.boundedEnqueueAndWait(writeRequest{availability: []AvailabilityObservation{normalized}})
	return nil
}

// DeleteAvailabilityTargetHistory removes raw observations, compact buckets,
// and revision boundaries behind the same writer barrier as ingestion.
func (s *Store) DeleteAvailabilityTargetHistory(targetID string) {
	targetID = strings.TrimSpace(targetID)
	if s == nil || targetID == "" || s.stopping.Load() {
		return
	}
	s.enqueueAndWait(writeRequest{availabilityDeletes: []string{targetID}})
}

func (s *Store) writeAvailabilityBatch(observations []AvailabilityObservation) {
	if len(observations) == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin availability history transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	seen := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		normalized, err := normalizeAvailabilityObservation(observation)
		if err != nil {
			log.Warn().Err(err).Msg("Dropping invalid availability observation")
			continue
		}
		if _, exists := seen[normalized.ObservationID]; exists {
			continue
		}
		seen[normalized.ObservationID] = struct{}{}
		if err := insertAvailabilityObservation(tx, normalized); err != nil {
			log.Warn().Err(err).Str("target_id", normalized.TargetID).Msg("Failed to write availability observation")
		}
	}
	if err := tx.Commit(); err != nil {
		log.Error().Err(err).Int("batch_size", len(observations)).Msg("Failed to commit availability history batch")
	}
}

func insertAvailabilityObservation(tx *pdb.InstrumentedTx, observation AvailabilityObservation) error {
	var latency any
	if observation.LatencyMillis != nil {
		latency = *observation.LatencyMillis
	}
	result, err := tx.Exec(`
		INSERT OR IGNORE INTO availability_observations (
			observation_id, target_id, config_revision, outcome, observed_at_ns,
			timeline_at_ns, ingested_at_ns, valid_until_ns, execution_source, latency_millis
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, observation.ObservationID, observation.TargetID, observation.ConfigRevision, string(observation.Outcome),
		observation.ObservedAt.UnixNano(), observation.TimelineAt.UnixNano(), observation.IngestedAt.UnixNano(),
		observation.TimelineAt.Add(observation.ValidFor).UnixNano(), string(observation.ExecutionSource), latency)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return err
	}
	rowID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO availability_revision_boundaries (target_id, revision, started_at_ns)
		VALUES (?, ?, ?)
	`, observation.TargetID, observation.ConfigRevision, observation.TimelineAt.UnixNano()); err != nil {
		return err
	}
	if err := addAvailabilityObservationAggregates(tx, observation); err != nil {
		return err
	}

	var previousOutcome string
	var previousStart, previousValidUntil int64
	err = tx.QueryRow(`
		SELECT outcome, timeline_at_ns, valid_until_ns
		FROM availability_observations
		WHERE target_id = ? AND id < ?
		ORDER BY id DESC LIMIT 1
	`, observation.TargetID, rowID).Scan(&previousOutcome, &previousStart, &previousValidUntil)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	end := observation.TimelineAt.UnixNano()
	if previousValidUntil < end {
		end = previousValidUntil
	}
	if end <= previousStart {
		return nil
	}
	return addAvailabilityDurationAggregates(tx, observation.TargetID, AvailabilityOutcome(previousOutcome), previousStart, end)
}

var availabilityStorageTiers = []struct {
	tier string
	size time.Duration
}{
	{string(TierMinute), time.Minute},
	{string(TierHourly), time.Hour},
	{string(TierDaily), 24 * time.Hour},
}

func addAvailabilityObservationAggregates(tx *pdb.InstrumentedTx, observation AvailabilityObservation) error {
	for _, tier := range availabilityStorageTiers {
		bucketStart := observation.TimelineAt.Truncate(tier.size).Unix()
		reachableCount, unreachableCount, indeterminateCount := int64(0), int64(0), int64(0)
		switch observation.Outcome {
		case AvailabilityReachable:
			reachableCount = 1
		case AvailabilityUnreachable:
			unreachableCount = 1
		case AvailabilityIndeterminate:
			indeterminateCount = 1
		}
		latencyCount, latencySum := int64(0), int64(0)
		var latencyMin, latencyMax any
		if observation.Outcome == AvailabilityReachable && observation.LatencyMillis != nil {
			latencyCount = 1
			latencySum = *observation.LatencyMillis
			latencyMin = *observation.LatencyMillis
			latencyMax = *observation.LatencyMillis
		}
		_, err := tx.Exec(`
			INSERT INTO availability_history_buckets (
				target_id, tier, bucket_start, reachable_count, unreachable_count,
				indeterminate_count, latency_count, latency_sum, latency_min, latency_max
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(target_id, tier, bucket_start) DO UPDATE SET
				reachable_count = reachable_count + excluded.reachable_count,
				unreachable_count = unreachable_count + excluded.unreachable_count,
				indeterminate_count = indeterminate_count + excluded.indeterminate_count,
				latency_count = latency_count + excluded.latency_count,
				latency_sum = latency_sum + excluded.latency_sum,
				latency_min = CASE WHEN excluded.latency_min IS NULL THEN latency_min
					WHEN latency_min IS NULL THEN excluded.latency_min ELSE MIN(latency_min, excluded.latency_min) END,
				latency_max = CASE WHEN excluded.latency_max IS NULL THEN latency_max
					WHEN latency_max IS NULL THEN excluded.latency_max ELSE MAX(latency_max, excluded.latency_max) END
		`, observation.TargetID, tier.tier, bucketStart, reachableCount, unreachableCount,
			indeterminateCount, latencyCount, latencySum, latencyMin, latencyMax)
		if err != nil {
			return err
		}
	}
	return nil
}

func addAvailabilityDurationAggregates(tx *pdb.InstrumentedTx, targetID string, outcome AvailabilityOutcome, startNS, endNS int64) error {
	for _, tier := range availabilityStorageTiers {
		cursor := time.Unix(0, startNS).UTC()
		end := time.Unix(0, endNS).UTC()
		for cursor.Before(end) {
			bucketStart := cursor.Truncate(tier.size)
			segmentEnd := bucketStart.Add(tier.size)
			if segmentEnd.After(end) {
				segmentEnd = end
			}
			millis := segmentEnd.Sub(cursor).Milliseconds()
			if millis > 0 {
				reachable, unreachable, indeterminate := int64(0), int64(0), int64(0)
				switch outcome {
				case AvailabilityReachable:
					reachable = millis
				case AvailabilityUnreachable:
					unreachable = millis
				case AvailabilityIndeterminate:
					indeterminate = millis
				}
				if _, err := tx.Exec(`
					INSERT INTO availability_history_buckets (
						target_id, tier, bucket_start, reachable_millis, unreachable_millis, indeterminate_millis
					) VALUES (?, ?, ?, ?, ?, ?)
					ON CONFLICT(target_id, tier, bucket_start) DO UPDATE SET
						reachable_millis = reachable_millis + excluded.reachable_millis,
						unreachable_millis = unreachable_millis + excluded.unreachable_millis,
						indeterminate_millis = indeterminate_millis + excluded.indeterminate_millis
				`, targetID, tier.tier, bucketStart.Unix(), reachable, unreachable, indeterminate); err != nil {
					return err
				}
			}
			cursor = segmentEnd
		}
	}
	return nil
}

func (s *Store) deleteAvailabilityTargets(targetIDs []string) {
	tx, err := s.db.Begin()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to begin availability history deletion")
		return
	}
	defer func() { _ = tx.Rollback() }()
	for _, targetID := range targetIDs {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		for _, statement := range []string{
			`DELETE FROM availability_observations WHERE target_id = ?`,
			`DELETE FROM availability_history_buckets WHERE target_id = ?`,
			`DELETE FROM availability_revision_boundaries WHERE target_id = ?`,
		} {
			if _, err := tx.Exec(statement, targetID); err != nil {
				log.Warn().Err(err).Str("target_id", targetID).Msg("Failed to delete availability history")
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		log.Warn().Err(err).Msg("Failed to commit availability history deletion")
	}
}

func (s *Store) runAvailabilityRetention() {
	now := time.Now().UTC()
	if _, err := s.db.Exec(`DELETE FROM availability_observations WHERE timeline_at_ns < ?`,
		now.Add(-s.effectiveRetention(availabilityRawRetention, now)).UnixNano()); err != nil {
		log.Warn().Err(err).Msg("Failed to prune raw availability observations")
	}
	for _, tier := range []struct {
		name      string
		retention time.Duration
	}{
		{string(TierMinute), availabilityMinuteRetention},
		{string(TierHourly), availabilityHourlyRetention},
		{string(TierDaily), availabilityDailyRetention},
	} {
		cutoff := now.Add(-s.effectiveRetention(tier.retention, now)).Unix()
		if _, err := s.db.Exec(`DELETE FROM availability_history_buckets WHERE tier = ? AND bucket_start < ?`, tier.name, cutoff); err != nil {
			log.Warn().Err(err).Str("tier", tier.name).Msg("Failed to prune availability history buckets")
		}
	}
	cutoff := now.Add(-s.effectiveRetention(availabilityDailyRetention, now)).UnixNano()
	if _, err := s.db.Exec(`DELETE FROM availability_revision_boundaries WHERE started_at_ns < ?`, cutoff); err != nil {
		log.Warn().Err(err).Msg("Failed to prune availability revision boundaries")
	}
}

// QueryAvailabilityHistory reads all requested targets with three bounded
// batch queries (buckets, live tails, and revision boundaries), never one
// query per target.
func (s *Store) QueryAvailabilityHistory(targetIDs []string, start, end time.Time, maxBuckets int) (map[string]AvailabilityHistoryTarget, error) {
	ids := normalizeAvailabilityTargetIDs(targetIDs)
	if len(ids) == 0 {
		return map[string]AvailabilityHistoryTarget{}, nil
	}
	if len(ids) > availabilityMaxBatchTargets {
		return nil, fmt.Errorf("availability history supports at most %d target ids", availabilityMaxBatchTargets)
	}
	start, end = start.UTC(), end.UTC()
	if !end.After(start) {
		return nil, fmt.Errorf("availability history end must be after start")
	}
	if maxBuckets <= 0 || maxBuckets > availabilityMaxBuckets {
		maxBuckets = availabilityMaxBuckets
	}

	step := availabilityPresentationStep(end.Sub(start), maxBuckets)
	results := make(map[string]AvailabilityHistoryTarget, len(ids))
	aggregates := make(map[string][]availabilityAggregate, len(ids))
	for _, id := range ids {
		buckets := makeAvailabilityBuckets(start, end, step)
		results[id] = AvailabilityHistoryTarget{TargetID: id, Buckets: buckets, RevisionBoundaries: []AvailabilityRevisionBoundary{}}
		aggregates[id] = make([]availabilityAggregate, len(buckets))
	}

	if err := s.queryAvailabilityBuckets(ids, start, end, step, aggregates); err != nil {
		return nil, err
	}
	if err := s.queryAvailabilityLiveTails(ids, start, end, step, aggregates); err != nil {
		return nil, err
	}
	boundaries, err := s.queryAvailabilityRevisionBoundaries(ids, start, end)
	if err != nil {
		return nil, err
	}

	windowMillis := end.Sub(start).Milliseconds()
	for _, id := range ids {
		target := results[id]
		total := availabilityAggregate{}
		for index := range target.Buckets {
			aggregate := aggregates[id][index]
			bucketMillis := target.Buckets[index].End.Sub(target.Buckets[index].Start).Milliseconds()
			target.Buckets[index] = finalizeAvailabilityBucket(target.Buckets[index], aggregate, bucketMillis)
			mergeAvailabilityAggregate(&total, aggregate)
		}
		target.Summary = finalizeAvailabilitySummary(total, windowMillis)
		target.RevisionBoundaries = boundaries[id]
		results[id] = target
	}
	return results, nil
}

func normalizeAvailabilityTargetIDs(targetIDs []string) []string {
	seen := make(map[string]struct{}, len(targetIDs))
	ids := make([]string, 0, len(targetIDs))
	for _, id := range targetIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func availabilityPresentationStep(window time.Duration, maxBuckets int) time.Duration {
	steps := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 6 * time.Minute, 12 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 4 * 24 * time.Hour}
	minimum := time.Duration(math.Ceil(float64(window) / float64(maxBuckets)))
	for _, step := range steps {
		if step >= minimum {
			return step
		}
	}
	return time.Duration(math.Ceil(float64(minimum)/(float64(24*time.Hour)))) * 24 * time.Hour
}

func makeAvailabilityBuckets(start, end time.Time, step time.Duration) []AvailabilityHistoryBucket {
	count := int(math.Ceil(float64(end.Sub(start)) / float64(step)))
	buckets := make([]AvailabilityHistoryBucket, 0, count)
	for cursor := start; cursor.Before(end); cursor = cursor.Add(step) {
		bucketEnd := cursor.Add(step)
		if bucketEnd.After(end) {
			bucketEnd = end
		}
		buckets = append(buckets, AvailabilityHistoryBucket{Start: cursor, End: bucketEnd})
	}
	return buckets
}

func availabilityPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func (s *Store) queryAvailabilityBuckets(ids []string, start, end time.Time, step time.Duration, aggregates map[string][]availabilityAggregate) error {
	minuteStart := start
	if candidate := end.Add(-7 * 24 * time.Hour); candidate.After(minuteStart) {
		minuteStart = candidate.Truncate(time.Hour)
	}
	hourlyStart := start
	if candidate := end.Add(-90 * 24 * time.Hour); candidate.After(hourlyStart) {
		hourlyStart = candidate.Truncate(24 * time.Hour)
	}

	args := make([]any, 0, len(ids)+9)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args,
		string(TierDaily), start.Unix(), hourlyStart.Unix(),
		string(TierHourly), hourlyStart.Unix(), minuteStart.Unix(),
		string(TierMinute), minuteStart.Unix(), end.Unix(),
	)
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT target_id, bucket_start, reachable_millis, unreachable_millis,
			indeterminate_millis, latency_count, latency_sum,
			COALESCE(latency_min, 0), COALESCE(latency_max, 0)
		FROM availability_history_buckets
		WHERE target_id IN (%s) AND (
			(tier = ? AND bucket_start >= ? AND bucket_start < ?) OR
			(tier = ? AND bucket_start >= ? AND bucket_start < ?) OR
			(tier = ? AND bucket_start >= ? AND bucket_start < ?)
		)
		ORDER BY target_id, bucket_start
	`, availabilityPlaceholders(len(ids))), args...)
	if err != nil {
		return fmt.Errorf("query availability history buckets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var targetID string
		var bucketStart, reachable, unreachable, indeterminate, latencyCount, latencySum, latencyMin, latencyMax int64
		if err := rows.Scan(&targetID, &bucketStart, &reachable, &unreachable, &indeterminate, &latencyCount, &latencySum, &latencyMin, &latencyMax); err != nil {
			return err
		}
		index := int(time.Unix(bucketStart, 0).Sub(start) / step)
		if index < 0 || index >= len(aggregates[targetID]) {
			continue
		}
		mergeAvailabilityAggregate(&aggregates[targetID][index], availabilityAggregate{
			reachableMillis: reachable, unreachableMillis: unreachable, indeterminateMillis: indeterminate,
			latencyCount: latencyCount, latencySum: latencySum, latencyMin: latencyMin, latencyMax: latencyMax,
		})
	}
	return rows.Err()
}

func (s *Store) queryAvailabilityLiveTails(ids []string, start, end time.Time, step time.Duration, aggregates map[string][]availabilityAggregate) error {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.Query(fmt.Sprintf(`
		WITH ranked AS (
			SELECT target_id, outcome, timeline_at_ns, valid_until_ns,
				ROW_NUMBER() OVER (PARTITION BY target_id ORDER BY timeline_at_ns DESC, id DESC) AS rank
			FROM availability_observations WHERE target_id IN (%s)
		)
		SELECT target_id, outcome, timeline_at_ns, valid_until_ns FROM ranked WHERE rank = 1
	`, availabilityPlaceholders(len(ids))), args...)
	if err != nil {
		return fmt.Errorf("query availability live tails: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var targetID, outcome string
		var tailStartNS, validUntilNS int64
		if err := rows.Scan(&targetID, &outcome, &tailStartNS, &validUntilNS); err != nil {
			return err
		}
		tailStart := time.Unix(0, tailStartNS).UTC()
		if tailStart.Before(start) {
			tailStart = start
		}
		tailEnd := time.Unix(0, validUntilNS).UTC()
		if tailEnd.After(end) {
			tailEnd = end
		}
		addAvailabilityRangeToPresentation(aggregates[targetID], start, step, AvailabilityOutcome(outcome), tailStart, tailEnd)
	}
	return rows.Err()
}

func addAvailabilityRangeToPresentation(buckets []availabilityAggregate, start time.Time, step time.Duration, outcome AvailabilityOutcome, cursor, end time.Time) {
	if !end.After(cursor) {
		return
	}
	for cursor.Before(end) {
		index := int(cursor.Sub(start) / step)
		if index < 0 || index >= len(buckets) {
			return
		}
		segmentEnd := start.Add(time.Duration(index+1) * step)
		if segmentEnd.After(end) {
			segmentEnd = end
		}
		millis := segmentEnd.Sub(cursor).Milliseconds()
		switch outcome {
		case AvailabilityReachable:
			buckets[index].reachableMillis += millis
		case AvailabilityUnreachable:
			buckets[index].unreachableMillis += millis
		case AvailabilityIndeterminate:
			buckets[index].indeterminateMillis += millis
		}
		cursor = segmentEnd
	}
}

func (s *Store) queryAvailabilityRevisionBoundaries(ids []string, start, end time.Time) (map[string][]AvailabilityRevisionBoundary, error) {
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, start.UnixNano(), end.UnixNano())
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT target_id, revision, started_at_ns
		FROM availability_revision_boundaries
		WHERE target_id IN (%s) AND started_at_ns >= ? AND started_at_ns < ?
		ORDER BY target_id, started_at_ns
	`, availabilityPlaceholders(len(ids))), args...)
	if err != nil {
		return nil, fmt.Errorf("query availability revision boundaries: %w", err)
	}
	defer rows.Close()
	result := make(map[string][]AvailabilityRevisionBoundary, len(ids))
	for rows.Next() {
		var targetID string
		var revision, at int64
		if err := rows.Scan(&targetID, &revision, &at); err != nil {
			return nil, err
		}
		result[targetID] = append(result[targetID], AvailabilityRevisionBoundary{Revision: revision, At: time.Unix(0, at).UTC()})
	}
	return result, rows.Err()
}

func mergeAvailabilityAggregate(target *availabilityAggregate, source availabilityAggregate) {
	target.reachableMillis += source.reachableMillis
	target.unreachableMillis += source.unreachableMillis
	target.indeterminateMillis += source.indeterminateMillis
	if source.latencyCount > 0 {
		if target.latencyCount == 0 || source.latencyMin < target.latencyMin {
			target.latencyMin = source.latencyMin
		}
		if target.latencyCount == 0 || source.latencyMax > target.latencyMax {
			target.latencyMax = source.latencyMax
		}
		target.latencyCount += source.latencyCount
		target.latencySum += source.latencySum
	}
}

func finalizeAvailabilityBucket(bucket AvailabilityHistoryBucket, aggregate availabilityAggregate, bucketMillis int64) AvailabilityHistoryBucket {
	known := aggregate.reachableMillis + aggregate.unreachableMillis + aggregate.indeterminateMillis
	if known > bucketMillis {
		known = bucketMillis
	}
	bucket.ReachableSeconds = millisecondsToSeconds(aggregate.reachableMillis)
	bucket.UnreachableSeconds = millisecondsToSeconds(aggregate.unreachableMillis)
	bucket.IndeterminateSeconds = millisecondsToSeconds(aggregate.indeterminateMillis)
	bucket.UnknownSeconds = millisecondsToSeconds(bucketMillis - known)
	bucket.LatencyMillis = availabilityLatencySummary(aggregate)
	return bucket
}

func finalizeAvailabilitySummary(aggregate availabilityAggregate, windowMillis int64) AvailabilityHistorySummary {
	known := aggregate.reachableMillis + aggregate.unreachableMillis + aggregate.indeterminateMillis
	if known > windowMillis {
		known = windowMillis
	}
	summary := AvailabilityHistorySummary{
		ReachableSeconds:       millisecondsToSeconds(aggregate.reachableMillis),
		UnreachableSeconds:     millisecondsToSeconds(aggregate.unreachableMillis),
		IndeterminateSeconds:   millisecondsToSeconds(aggregate.indeterminateMillis),
		UnknownSeconds:         millisecondsToSeconds(windowMillis - known),
		ReachableLatencyMillis: availabilityLatencySummary(aggregate),
	}
	if windowMillis > 0 {
		summary.CoveragePercent = roundPercent(float64(known) * 100 / float64(windowMillis))
	}
	determinate := aggregate.reachableMillis + aggregate.unreachableMillis
	if determinate > 0 {
		value := roundPercent(float64(aggregate.reachableMillis) * 100 / float64(determinate))
		summary.AvailabilityPercent = &value
	}
	return summary
}

func availabilityLatencySummary(aggregate availabilityAggregate) *AvailabilityLatencySummary {
	if aggregate.latencyCount <= 0 {
		return nil
	}
	return &AvailabilityLatencySummary{
		Average: math.Round(float64(aggregate.latencySum)/float64(aggregate.latencyCount)*100) / 100,
		Min:     aggregate.latencyMin,
		Max:     aggregate.latencyMax,
	}
}

func millisecondsToSeconds(value int64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Round(float64(value)/10) / 100
}

func roundPercent(value float64) float64 {
	return math.Round(value*100) / 100
}

// AvailabilityHistoryRowCounts is a narrow diagnostic used by contract tests
// to prove idempotency and full target deletion without exposing stored data.
func (s *Store) AvailabilityHistoryRowCounts(targetID string) (raw, buckets, boundaries int64, err error) {
	targetID = strings.TrimSpace(targetID)
	queries := []struct {
		statement string
		value     *int64
	}{
		{`SELECT COUNT(*) FROM availability_observations WHERE target_id = ?`, &raw},
		{`SELECT COUNT(*) FROM availability_history_buckets WHERE target_id = ?`, &buckets},
		{`SELECT COUNT(*) FROM availability_revision_boundaries WHERE target_id = ?`, &boundaries},
	}
	for _, query := range queries {
		if scanErr := s.db.QueryRow(query.statement, targetID).Scan(query.value); scanErr != nil {
			return 0, 0, 0, scanErr
		}
	}
	return raw, buckets, boundaries, nil
}

// SortedAvailabilityHistoryTargets converts the batch map to stable input
// order when callers need a slice response.
func SortedAvailabilityHistoryTargets(results map[string]AvailabilityHistoryTarget, order []string) []AvailabilityHistoryTarget {
	items := make([]AvailabilityHistoryTarget, 0, len(results))
	seen := make(map[string]struct{}, len(results))
	for _, id := range order {
		if item, ok := results[id]; ok {
			items = append(items, item)
			seen[id] = struct{}{}
		}
	}
	remaining := make([]string, 0, len(results)-len(seen))
	for id := range results {
		if _, ok := seen[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	for _, id := range remaining {
		items = append(items, results[id])
	}
	return items
}
