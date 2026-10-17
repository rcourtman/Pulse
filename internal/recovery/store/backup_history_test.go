package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/stretchr/testify/require"
)

func TestBackupHistorySurvivesRestartAndShrinkingSourceInventory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recovery.db")
	s, err := Open(path)
	require.NoError(t, err)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	point := recovery.RecoveryPoint{ID: "pve-task:site-job", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal, Outcome: recovery.OutcomeFailed, StartedAt: &at, CompletedAt: &at, Details: map[string]any{"instance": "site"}}
	observation := model.BackupHistoryCollection{Provider: recovery.ProviderProxmoxPVE, Instance: "site", ObservedAt: at, Complete: true, MaxGap: 5 * time.Minute}
	require.NoError(t, s.RecordBackupHistory(ctx, []recovery.RecoveryPoint{point}, []model.BackupHistoryCollection{observation}))
	point.Outcome, point.CompletedAt = recovery.OutcomeRunning, nil
	observation.ObservedAt = at.Add(time.Minute)
	require.NoError(t, s.RecordBackupHistory(ctx, []recovery.RecoveryPoint{point}, []model.BackupHistoryCollection{observation}))
	observation.ObservedAt = at.Add(2 * time.Minute)
	require.NoError(t, s.RecordBackupHistory(ctx, nil, []model.BackupHistoryCollection{observation}))
	require.NoError(t, s.Close())
	s, err = Open(path)
	require.NoError(t, err)
	defer s.Close()
	points, total, err := s.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, recovery.OutcomeFailed, points[0].Outcome)
	coverage, err := s.BackupHistoryCoverage(ctx, at, at.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, coverage.Intervals, 1)
	require.Equal(t, at, coverage.Intervals[0].Start)
	require.Equal(t, at.Add(2*time.Minute), coverage.Intervals[0].End)
}

func TestBackupHistoryCoverageDoesNotBridgeFailureDowntimeOrOldObservations(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	require.NoError(t, err)
	defer s.Close()
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond)
	for _, step := range []struct {
		minutes  int
		complete bool
	}{{0, true}, {1, true}, {2, false}, {3, true}, {4, true}, {30, true}, {2, false}} {
		require.NoError(t, s.RecordBackupHistory(ctx, nil, []model.BackupHistoryCollection{{Provider: recovery.ProviderProxmoxPVE, Instance: "site", ObservedAt: at.Add(time.Duration(step.minutes) * time.Minute), Complete: step.complete, MaxGap: 5 * time.Minute, Reason: "collection_failed"}}))
	}
	coverage, err := s.BackupHistoryCoverage(ctx, at, at.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, coverage.Intervals, 3)
	require.True(t, coverage.Sources[0].Complete)
	require.Equal(t, at.Add(30*time.Minute), coverage.Sources[0].LastObservedAt)
	require.Equal(t, at.Add(time.Minute), coverage.Intervals[0].End)
	require.Equal(t, at.Add(3*time.Minute), coverage.Intervals[1].Start)
}

func TestBackupHistoryCoverageCannotCommitBeforeExecutionEvidence(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	require.NoError(t, err)
	defer s.Close()
	_, err = s.db.Exec(`CREATE TRIGGER refuse_task BEFORE INSERT ON recovery_points BEGIN SELECT RAISE(ABORT,'write failed'); END`)
	require.NoError(t, err)
	at := time.Now().UTC()
	err = s.RecordBackupHistory(context.Background(), []recovery.RecoveryPoint{{ID: "pve-task:site-job", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal, Outcome: recovery.OutcomeFailed, StartedAt: &at, Details: map[string]any{"instance": "site"}}}, []model.BackupHistoryCollection{{Provider: recovery.ProviderProxmoxPVE, Instance: "site", ObservedAt: at, Complete: true, MaxGap: time.Minute}})
	require.ErrorContains(t, err, "write failed")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	coverage, err := s.BackupHistoryCoverage(ctx, at.Add(-time.Hour), at)
	require.NoError(t, err)
	require.Empty(t, coverage.Sources)
	require.Empty(t, coverage.Intervals)
}

func TestBackupHistoryIncrementalCollectionRetainsLongRunningJobs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recovery.db")
	s, err := Open(path)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Millisecond)
	start := now.Add(-72 * time.Hour)
	point := recovery.RecoveryPoint{ID: "pve-task:long", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal, Outcome: recovery.OutcomeRunning, StartedAt: &start, Details: map[string]any{"instance": "site"}}
	collection := model.BackupHistoryCollection{Provider: recovery.ProviderProxmoxPVE, Instance: "site", Complete: true, ObservedAt: now.Add(-time.Minute), MaxGap: 5 * time.Minute}
	require.NoError(t, s.RecordBackupHistory(ctx, []recovery.RecoveryPoint{point}, []model.BackupHistoryCollection{collection}))
	require.NoError(t, s.Close())
	s, err = Open(path)
	require.NoError(t, err)
	defer s.Close()
	since, err := s.BackupHistoryCollectionStart(ctx, recovery.ProviderProxmoxPVE, "site", now)
	require.NoError(t, err)
	require.Equal(t, start, since, "restart must still read a backup that began before the overlap window")
	finished := now.Add(-time.Second)
	point.CompletedAt, point.Outcome = &finished, recovery.OutcomeSuccess
	collection.ObservedAt = now
	require.NoError(t, s.RecordBackupHistory(ctx, []recovery.RecoveryPoint{point}, []model.BackupHistoryCollection{collection}))
	since, err = s.BackupHistoryCollectionStart(ctx, recovery.ProviderProxmoxPVE, "site", now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), since)
	collection.ObservedAt, collection.Complete, collection.Reason = now.Add(time.Hour), false, "collection_failed"
	require.NoError(t, s.RecordBackupHistory(ctx, nil, []model.BackupHistoryCollection{collection}))
	since, err = s.BackupHistoryCollectionStart(ctx, recovery.ProviderProxmoxPVE, "site", now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), since, "failed collections cannot advance the read watermark")
}

func TestBackupHistoryCappedBackfillDoesNotBackdateCoverage(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	require.NoError(t, err)
	defer s.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	c := model.BackupHistoryCollection{Provider: recovery.ProviderProxmoxPVE, Instance: "site", ObservedAt: now, Reason: "history_truncated", MaxGap: 5 * time.Minute}
	require.NoError(t, s.RecordBackupHistory(context.Background(), nil, []model.BackupHistoryCollection{c}))
	since, err := s.BackupHistoryCollectionStart(context.Background(), recovery.ProviderProxmoxPVE, "site", now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), since)
	c.Complete, c.ObservedAt = true, now.Add(time.Minute)
	require.NoError(t, s.RecordBackupHistory(context.Background(), nil, []model.BackupHistoryCollection{c}))
	coverage, err := s.BackupHistoryCoverage(context.Background(), now.Add(-30*24*time.Hour), c.ObservedAt)
	require.NoError(t, err)
	require.Len(t, coverage.Intervals, 1)
	require.Equal(t, c.ObservedAt, coverage.Intervals[0].Start)
}
