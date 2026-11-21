package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxrecoverymapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/stretchr/testify/require"
)

func TestBackupJobsPayloadExpiresReportedRunningAtReadTime(t *testing.T) {
	observed := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	// The history range ends at the most recent covered poll, even as
	// later requests need to decide whether an open task is still fresh.
	coveredTo := observed.Add(time.Minute)
	deadline := observed.Add(recovery.BackupTaskRunningMaxAge)
	for _, tc := range []struct {
		name, status string
		now          time.Time
		legacy       bool
		wantRunning  bool
	}{
		{"provider active before deadline", "active", deadline.Add(-time.Second), false, true},
		{"provider running at deadline", "running", deadline, false, true},
		{"expired reported running", "running", deadline.Add(time.Nanosecond), false, false},
		{"legacy ingestion at deadline", "running", deadline, true, true},
		{"expired legacy ingestion", "running", deadline.Add(time.Nanosecond), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := observed.Add(-5 * time.Minute)
			task := models.BackupTask{
				ID: "reported-running", Instance: "lab", Node: "pve1", VMID: 100, Type: "vzdump",
				Status: tc.status, StartTime: started, ObservedAt: observed,
			}
			// A modern point can be newly ingested again, but the original
			// observation, not ingestion, decides when its live claim ends.
			ingested := tc.now
			if tc.legacy {
				task.ObservedAt = time.Time{}
				ingested = observed
			}
			points, err := proxmoxrecoverymapper.FromPVEBackupTasksWithEvidence([]models.BackupTask{task}, nil, ingested)
			require.NoError(t, err)
			require.Len(t, points, 1)
			if tc.legacy {
				require.Nil(t, points[0].Evidence.ValidUntil)
			} else {
				require.Equal(t, deadline, *points[0].Evidence.ValidUntil)
			}
			points = append(points,
				backupJobsFixturePoint("sibling-ok", recovery.OutcomeSuccess, started.Add(-time.Hour), 101),
				backupJobsFixturePoint("sibling-failed", recovery.OutcomeFailed, started.Add(-30*time.Minute), 101))

			from := backupJobsHistoryStart(tc.now, time.UTC)
			payload := buildBackupJobsPayload(backupJobsFixtureWorkloads, points, backupJobsFixtureCoverage(from, coveredTo), true, from, tc.now, time.UTC)
			want := BackupJobsCounts{Success: 1, Failed: 1, Unknown: 1}
			if tc.wantRunning {
				want.Unknown, want.Running = 0, 1
			}
			require.Equal(t, coveredTo, payload.Last24h.End, "freshness is not a change to the coverage window")
			require.Equal(t, "complete", payload.Last24h.HistoryState)
			require.Equal(t, want, payload.Last24h.Counts)
			require.Len(t, payload.Days, backupJobsHistoryDays)
			require.Equal(t, want, payload.Days[len(payload.Days)-1].Counts)
			require.Len(t, payload.Last24h.Failures, 1)
			require.Equal(t, "db", payload.Last24h.Failures[0].WorkloadName)
			require.Equal(t, recovery.OutcomeRunning, points[0].Outcome, "leave the raw outcome untouched")
			require.Equal(t, tc.status, points[0].Details["status"])
		})
	}
}
