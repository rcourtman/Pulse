package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/stretchr/testify/require"
)

func backupJobsFixturePoint(id string, outcome recovery.Outcome, at time.Time, vmid int) recovery.RecoveryPoint {
	return recovery.RecoveryPoint{
		ID: "pve-task:" + id, Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther,
		Outcome: outcome, StartedAt: &at, CompletedAt: &at,
		Details: map[string]any{"instance": "lab", "vmid": vmid},
	}
}

func backupJobsFixtureCoverage(from, to time.Time) model.BackupHistoryCoverage {
	return model.BackupHistoryCoverage{
		Sources:   []model.BackupHistorySource{{Provider: recovery.ProviderProxmoxPVE, Instance: "lab", FirstObservedAt: from, LastObservedAt: to, Complete: true}},
		Intervals: []model.BackupHistoryInterval{{Provider: recovery.ProviderProxmoxPVE, Instance: "lab", Start: from, End: to}},
	}
}

var backupJobsFixtureWorkloads = []model.BackupHealthWorkload{
	{ID: "vm-100", Name: "web", Instance: "lab", Type: "vm", VMID: 100},
	{ID: "vm-101", Name: "db", Instance: "lab", Type: "vm", VMID: 101},
}

func TestBackupJobsPayloadReadsRetainedHistoryBeyondTheLiveWindow(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	require.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), from)
	points := []recovery.RecoveryPoint{
		backupJobsFixturePoint("recent-failed", recovery.OutcomeFailed, now.Add(-2*time.Hour), 100),
		backupJobsFixturePoint("recent-ok", recovery.OutcomeSuccess, now.Add(-3*time.Hour), 101),
		// Ten days back: outside the live /api/backups/pve window, inside retention.
		backupJobsFixturePoint("old-ok", recovery.OutcomeSuccess, now.Add(-10*24*time.Hour), 100),
	}

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, points, backupJobsFixtureCoverage(from.Add(-24*time.Hour), now), true, from, now, time.UTC)

	require.Equal(t, "complete", payload.HistoryState)
	require.Len(t, payload.Days, backupJobsHistoryDays)
	require.Equal(t, "2026-10-22", payload.Days[3].Date)
	require.Equal(t, 1, payload.Days[3].Counts.Success)
	require.Equal(t, BackupJobsCounts{Success: 1, Failed: 1}, payload.Days[13].Counts)
	require.Equal(t, BackupJobsCounts{Success: 1, Failed: 1}, payload.Last24h.Counts)
	require.Equal(t, "complete", payload.Last24h.HistoryState)
	require.Len(t, payload.Last24h.Failures, 1)
	require.Equal(t, "web", payload.Last24h.Failures[0].WorkloadName)
	require.Equal(t, "vm-100", payload.Last24h.Failures[0].WorkloadID)
	require.Equal(t, "failed", payload.Last24h.Failures[0].Outcome)
}

func TestBackupJobsPayloadMarksUnobservedDaysInsteadOfClean(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	// Pulse only started collecting three days ago.
	coverage := backupJobsFixtureCoverage(now.Add(-72*time.Hour), now)

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, nil, coverage, true, from, now, time.UTC)

	require.Equal(t, "partial", payload.HistoryState)
	// Days before collection began are not clean history.
	require.Equal(t, "partial", payload.Days[0].HistoryState)
	require.Equal(t, "complete", payload.Days[13].HistoryState)
	require.Zero(t, payload.Days[0].Counts.Success+payload.Days[0].Counts.Failed)
}

func TestBackupJobsPayloadUnavailableHistoryNeverReadsAsClean(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, nil, model.BackupHistoryCoverage{}, false, from, now, time.UTC)

	require.Equal(t, "unavailable", payload.HistoryState)
	require.Equal(t, "unavailable", payload.Last24h.HistoryState)
	for _, day := range payload.Days {
		require.Equal(t, "unavailable", day.HistoryState, day.Date)
	}
	require.Empty(t, payload.Last24h.Failures)
}

func TestBackupJobsHistoryStartFollowsTheViewerZone(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)
	// 23:30 UTC on 1 November is already 2 November in Auckland.
	now := time.Date(2026, 11, 1, 23, 30, 0, 0, time.UTC)
	// London is still on summer time on 19 October.
	require.Equal(t, time.Date(2026, 10, 19, 0, 0, 0, 0, london).UTC(), backupJobsHistoryStart(now, london))
	auckland, err := time.LoadLocation("Pacific/Auckland")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 10, 20, 0, 0, 0, 0, auckland), backupJobsHistoryStart(now, auckland).In(auckland))
}

func TestMockBackupJobHistoryObservesFixtureInstances(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	tasks := []models.BackupTask{{
		ID: "lab-UPID:pve1:1:2:vzdump:100:root@pam:", Node: "pve1", Instance: "lab", Type: "vzdump",
		VMID: 100, Status: "OK", StartTime: now.Add(-time.Hour), EndTime: now.Add(-50 * time.Minute),
	}}

	// A second rated instance without fixture tasks still counts as observed.
	workloads := append([]model.BackupHealthWorkload{{ID: "vm-200", Name: "quiet", Instance: "edge", Type: "vm", VMID: 200}}, backupJobsFixtureWorkloads...)
	points, coverage := mockBackupJobHistory(tasks, workloads, from, now)

	require.Len(t, points, 1)
	require.True(t, strings.HasPrefix(points[0].ID, "pve-task:"))
	require.Len(t, coverage.Sources, 2)
	payload := buildBackupJobsPayload(workloads, points, coverage, true, from, now, time.UTC)
	require.Equal(t, 1, payload.Last24h.Counts.Success)
	require.Equal(t, "complete", payload.HistoryState)
	require.Equal(t, "complete", payload.Days[0].HistoryState)
}

func TestBackupJobsRejectsNonGet(t *testing.T) {
	router := &Router{}
	rec := httptest.NewRecorder()
	router.handleBackupJobs(rec, httptest.NewRequest(http.MethodPost, "/api/backups/jobs", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestBackupJobsRateProxmoxGuestsOnly(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	// A vSphere VM has no PVE instance or VMID.
	mixed := append([]model.BackupHealthWorkload{{ID: "vsphere-vm", Name: "esx-guest", Type: "vm"}}, backupJobsFixtureWorkloads...)

	workloads := pveBackupJobWorkloads(mixed)
	payload := buildBackupJobsPayload(workloads, nil, backupJobsFixtureCoverage(from, now), true, from, now, time.UTC)

	require.Len(t, workloads, 2)
	require.Equal(t, "complete", payload.HistoryState)
	require.Equal(t, "complete", payload.Days[0].HistoryState)
}

func TestBackupJobsHealthyPollLagIsNotPartial(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	// The last poll finished a minute ago, as on any healthy install.
	coverage := backupJobsFixtureCoverage(from.Add(-24*time.Hour), now.Add(-time.Minute))

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, nil, coverage, true, from, now, time.UTC)
	require.Equal(t, "complete", payload.HistoryState)
	require.Equal(t, "complete", payload.Last24h.HistoryState)
	require.Equal(t, "complete", payload.Days[13].HistoryState)

	// Collection that stopped an hour ago is a real gap.
	stale := backupJobsFixtureCoverage(from.Add(-24*time.Hour), now.Add(-time.Hour))
	payload = buildBackupJobsPayload(backupJobsFixtureWorkloads, nil, stale, true, from, now, time.UTC)
	require.Equal(t, "partial", payload.Last24h.HistoryState)
	require.Equal(t, "partial", payload.Days[13].HistoryState)
}

func TestBackupJobsIgnoreAbandonedOlderTasks(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	started := now.Add(-20 * 24 * time.Hour)
	abandoned := recovery.RecoveryPoint{
		ID: "pve-task:abandoned", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther,
		Outcome: recovery.OutcomeRunning, StartedAt: &started,
		Details: map[string]any{"instance": "lab", "vmid": 100},
	}

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, []recovery.RecoveryPoint{abandoned}, backupJobsFixtureCoverage(from.Add(-24*time.Hour), now), true, from, now, time.UTC)

	require.Zero(t, payload.Last24h.Counts.Unknown+payload.Last24h.Counts.Running)
	require.Zero(t, payload.Days[0].Counts.Unknown+payload.Days[0].Counts.Running)
}

func TestBackupJobsDaysNameEachFailedGuestOnce(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	from := backupJobsHistoryStart(now, time.UTC)
	points := []recovery.RecoveryPoint{
		backupJobsFixturePoint("retry", recovery.OutcomeFailed, now.Add(-1*time.Hour), 100),
		backupJobsFixturePoint("first", recovery.OutcomeFailed, now.Add(-3*time.Hour), 100),
		backupJobsFixturePoint("older", recovery.OutcomeFailed, now.Add(-10*24*time.Hour), 100),
		backupJobsFixturePoint("clean", recovery.OutcomeSuccess, now.Add(-5*24*time.Hour), 100),
	}

	payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, points, backupJobsFixtureCoverage(from.Add(-24*time.Hour), now), true, from, now, time.UTC)

	require.Equal(t, 2, payload.Days[13].Counts.Failed)
	require.Len(t, payload.Days[13].Failures, 1)
	require.Equal(t, "web", payload.Days[13].Failures[0].WorkloadName)
	require.Equal(t, now.Add(-1*time.Hour), payload.Days[13].Failures[0].At)
	require.Len(t, payload.Days[3].Failures, 1)
	// A clean day is an empty list, never JSON null.
	require.NotNil(t, payload.Days[8].Failures)
	require.Empty(t, payload.Days[8].Failures)
}
