package recovery

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/stretchr/testify/require"
)

func backupHealthPoint(id, instance string, vmid int, outcome Outcome, at time.Time) RecoveryPoint {
	return RecoveryPoint{ID: "pve-task:" + id, Provider: ProviderProxmoxPVE, Kind: KindOther, Outcome: outcome, StartedAt: &at, CompletedAt: &at, Details: map[string]any{"instance": instance, "vmid": vmid, "status": "backup failed"}}
}

func TestBackupHealthDailyResultsRemainClientScopedWithOverlappingVMIDs(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)
	coverage := model.BackupHistoryCoverage{Sources: []model.BackupHistorySource{{Provider: ProviderProxmoxPVE, Instance: "client-a"}}, Intervals: []model.BackupHistoryInterval{{Provider: ProviderProxmoxPVE, Instance: "client-a", Start: from, End: to}}}
	workloads := []model.BackupHealthWorkload{{ID: "a", Name: "Client A", Instance: "client-a", Node: "new-node", VMID: 100}}
	points := []RecoveryPoint{backupHealthPoint("a-ok", "client-a", 100, OutcomeSuccess, from.Add(time.Hour)), backupHealthPoint("a-failed", "client-a", 100, OutcomeFailed, from.Add(25*time.Hour)), backupHealthPoint("b-failed", "client-b", 100, OutcomeFailed, from.Add(2*time.Hour))}
	summary := BuildBackupHealth(workloads, points, coverage, from, to, time.UTC)
	require.Equal(t, 1, summary.Counts.Success)
	require.Equal(t, 1, summary.Counts.Failed)
	require.Len(t, summary.Failures, 1)
	require.Equal(t, "Client A", summary.Failures[0].WorkloadName)
	require.Equal(t, 1, summary.Days[0].Counts.Success)
	require.Equal(t, 1, summary.Days[1].Counts.Failed)
	require.Equal(t, "complete", summary.HistoryState)
	require.NotNil(t, summary.Workloads[0].LastSuccessAt)
}

func TestBackupHealthMissingHistoryAndZeroRecordedAttemptsAreDistinct(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	workloads := []model.BackupHealthWorkload{{ID: "a", Instance: "site", VMID: 100}}
	summary := BuildBackupHealth(workloads, nil, model.BackupHistoryCoverage{}, from, to, time.UTC)
	require.Equal(t, "not_observed", summary.HistoryState)
	require.Equal(t, "not_observed", summary.Days[0].HistoryState)
	require.Equal(t, 1, summary.MissingWorkloads)
	coverage := model.BackupHistoryCoverage{Sources: []model.BackupHistorySource{{Provider: ProviderProxmoxPVE, Instance: "site"}}, Intervals: []model.BackupHistoryInterval{{Provider: ProviderProxmoxPVE, Instance: "site", Start: from, End: to}}}
	summary = BuildBackupHealth(workloads, nil, coverage, from, to, time.UTC)
	require.Equal(t, "complete", summary.HistoryState)
	require.Equal(t, 1, summary.MissingWorkloads)
	require.Zero(t, summary.Counts.Total())
	artifact := backupHealthPoint("artifact", "site", 100, OutcomeSuccess, from.Add(time.Hour))
	artifact.Kind = KindBackup
	summary = BuildBackupHealth(workloads, []RecoveryPoint{artifact}, coverage, from, to, time.UTC)
	require.Zero(t, summary.Counts.Total())
	require.Nil(t, summary.Workloads[0].LastSuccessAt)
	summary = BuildBackupHealth(nil, nil, coverage, from, to, time.UTC)
	require.Equal(t, "not_observed", summary.HistoryState)
	require.Equal(t, "not_observed", summary.Days[0].HistoryState, "an unmatched client scope is not an observed zero-failure day")
}

func TestBackupHealthCoverageKeepsConnectionNodeAndProviderSeparate(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	day2, day3, to := from.AddDate(0, 0, 1), from.AddDate(0, 0, 2), from.AddDate(0, 0, 3)
	coverage := model.BackupHistoryCoverage{
		Sources: []model.BackupHistorySource{
			{Provider: ProviderProxmoxPVE, Instance: "a", Node: "node-1"},
			{Provider: ProviderProxmoxPVE, Instance: "a", Node: "node-2", Reason: "collection_failed"},
			{Provider: ProviderProxmoxPVE, Instance: "b", Node: "node-2"},
			{Provider: ProviderProxmoxPBS, Instance: "a", Node: "node-2"},
		},
		Intervals: []model.BackupHistoryInterval{
			{Provider: ProviderProxmoxPVE, Instance: "a", Node: "node-1", Start: from, End: to},
			{Provider: ProviderProxmoxPVE, Instance: "a", Node: "node-2", Start: from, End: day2},
			{Provider: ProviderProxmoxPVE, Instance: "a", Node: "node-2", Start: day3, End: to},
			{Provider: ProviderProxmoxPVE, Instance: "b", Node: "node-2", Start: from, End: day2},
			{Provider: ProviderProxmoxPVE, Instance: "b", Node: "node-2", Start: day2, End: to},
			{Provider: ProviderProxmoxPBS, Instance: "a", Node: "node-2", Start: from, End: to},
		},
	}
	workloads := []model.BackupHealthWorkload{
		{ID: "a-100", Instance: "a", Node: "node-1", VMID: 100},
		{ID: "a-101", Instance: "a", Node: "node-2", VMID: 101},
		{ID: "b-100", Instance: "b", Node: "node-2", VMID: 100},
	}
	summary := BuildBackupHealth(workloads, nil, coverage, from, to, time.UTC)
	require.Equal(t, "partial", summary.HistoryState)
	require.Zero(t, summary.Counts.Total())
	require.Len(t, summary.Workloads, 3)
	for _, row := range summary.Workloads[:2] {
		require.Equal(t, "partial", row.HistoryState)
		require.Equal(t, "Pulse did not continuously observe this source throughout the reporting period. Latest collection: collection failed.", row.HistoryReason)
	}
	require.Equal(t, "complete", summary.Workloads[2].HistoryState, "touching intervals cover connection b")
	require.Equal(t, []model.BackupHealthDay{
		{Date: "2026-10-01", HistoryState: "complete"},
		{Date: "2026-10-02", HistoryState: "partial"},
		{Date: "2026-10-03", HistoryState: "complete"},
	}, summary.Days, "other nodes, connections and providers cannot fill the gap")
}

func TestBackupHealthUnattributedJobsCannotProduceCompleteClientReport(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	coverage := model.BackupHistoryCoverage{Sources: []model.BackupHistorySource{{Provider: ProviderProxmoxPVE, Instance: "site"}}, Intervals: []model.BackupHistoryInterval{{Provider: ProviderProxmoxPVE, Instance: "site", Start: from, End: to}}}
	summary := BuildBackupHealth([]model.BackupHealthWorkload{{ID: "a", Instance: "site", VMID: 100}}, []RecoveryPoint{backupHealthPoint("job", "site", 0, OutcomeFailed, from.Add(time.Hour))}, coverage, from, to, time.UTC)
	require.Equal(t, "partial", summary.HistoryState)
	require.Equal(t, "partial", summary.Days[0].HistoryState)
	require.Empty(t, summary.Failures)
	require.Zero(t, summary.Counts.Total())
	foreignJob := backupHealthPoint("foreign-job", "another-client", 0, OutcomeFailed, from.Add(time.Hour))
	summary = BuildBackupHealth([]model.BackupHealthWorkload{{ID: "a", Instance: "site", VMID: 100}}, []RecoveryPoint{foreignJob}, coverage, from, to, time.UTC)
	require.Equal(t, "complete", summary.HistoryState)
	require.Equal(t, "complete", summary.Days[0].HistoryState)
}

func TestBackupHealthCalendarDaysRespectDSTAndCutoff(t *testing.T) {
	location, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)
	from := time.Date(2026, 10, 24, 0, 0, 0, 0, location)
	to := from.AddDate(0, 0, 3)
	point := backupHealthPoint("cross-cutoff", "site", 100, OutcomeSuccess, to.Add(time.Hour))
	start := to.Add(-time.Hour)
	point.StartedAt = &start
	summary := BuildBackupHealth([]model.BackupHealthWorkload{{ID: "a", Instance: "site", VMID: 100}}, []RecoveryPoint{point}, model.BackupHistoryCoverage{}, from, to, location)
	require.Len(t, summary.Days, 3)
	require.Equal(t, "2026-10-25", summary.Days[1].Date)
	require.Equal(t, 1, summary.Counts.Running)
	require.Zero(t, summary.Counts.Success)
	require.Nil(t, summary.Workloads[0].LastSuccessAt)
}

func TestBackupHealthAmbiguousWorkloadIdentityIsIncomplete(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	coverage := model.BackupHistoryCoverage{Sources: []model.BackupHistorySource{{Provider: ProviderProxmoxPVE, Instance: "site"}}, Intervals: []model.BackupHistoryInterval{{Provider: ProviderProxmoxPVE, Instance: "site", Start: from, End: to}}}
	workloads := []model.BackupHealthWorkload{{ID: "a", Instance: "site", VMID: 100}, {ID: "b", Instance: "site", VMID: 100}}
	summary := BuildBackupHealth(workloads, []RecoveryPoint{backupHealthPoint("ambiguous", "site", 100, OutcomeFailed, from.Add(time.Hour))}, coverage, from, to, time.UTC)
	require.Equal(t, "partial", summary.HistoryState)
	require.Equal(t, "partial", summary.Days[0].HistoryState)
	require.Zero(t, summary.Counts.Total())
	require.Empty(t, summary.Failures)
	for _, row := range summary.Workloads {
		require.Contains(t, row.HistoryReason, "uniquely matched")
	}
}

func TestBackupHealthAttributionGapsKeepScopedReasonsInPointOrder(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 3)
	coverage := model.BackupHistoryCoverage{}
	for _, instance := range []string{"site-a", "site-b"} {
		coverage.Sources = append(coverage.Sources, model.BackupHistorySource{Provider: ProviderProxmoxPVE, Instance: instance})
		coverage.Intervals = append(coverage.Intervals, model.BackupHistoryInterval{Provider: ProviderProxmoxPVE, Instance: instance, Start: from, End: to})
	}
	workloads := []model.BackupHealthWorkload{
		{ID: "a-first", Instance: "site-a", VMID: 100},
		{ID: "a-second", Instance: "site-a", VMID: 100},
		{ID: "a-other", Instance: "site-a", VMID: 101},
		{ID: "b-first", Instance: "site-b", VMID: 100},
	}
	parent := backupHealthPoint("multi", "site-a", 0, OutcomeFailed, from.Add(time.Hour))
	ambiguous := backupHealthPoint("ambiguous", "site-a", 100, OutcomeWarning, from.Add(25*time.Hour))
	exact := backupHealthPoint("exact", "site-a", 100, OutcomeFailed, from.Add(49*time.Hour))
	exact.SubjectResourceID = "a-first"
	qualified := backupHealthPoint("qualified", "site-a", 0, OutcomeFailed, from.Add(50*time.Hour))
	qualified.Details["guestTaskDetailsComplete"] = true
	foreign := backupHealthPoint("foreign", "site-c", 0, OutcomeFailed, from.Add(51*time.Hour))
	old := backupHealthPoint("old", "site-a", 100, OutcomeFailed, from.Add(-time.Hour))

	for _, tc := range []struct {
		name              string
		first, second     RecoveryPoint
		overlappingReason string
	}{
		{"ambiguous point last", parent, ambiguous, "Some backup task identities could not be uniquely matched to workloads."},
		{"multi-guest point last", ambiguous, parent, "Some multi-guest backup jobs could not be fully attributed to workloads."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points := []RecoveryPoint{tc.first, tc.second, exact, qualified, foreign, old}
			summary := BuildBackupHealth(workloads, points, coverage, from, to, time.UTC)
			require.Equal(t, "partial", summary.HistoryState)
			require.Equal(t, []model.BackupHealthDay{
				{Date: "2026-10-01", HistoryState: "partial"},
				{Date: "2026-10-02", HistoryState: "partial"},
				{Date: "2026-10-03", HistoryState: "complete", Counts: model.BackupOutcomeCounts{Failed: 1}},
			}, summary.Days)
			require.Equal(t, model.BackupOutcomeCounts{Failed: 1}, summary.Counts)
			require.Len(t, summary.Failures, 1)
			require.Equal(t, "a-first", summary.Failures[0].WorkloadID)
			require.Equal(t, 3, summary.MissingWorkloads)

			rows := make(map[string]model.BackupHealthWorkloadResult)
			for _, row := range summary.Workloads {
				rows[row.ID] = row
			}
			for _, id := range []string{"a-first", "a-second"} {
				require.Equal(t, "partial", rows[id].HistoryState)
				require.Equal(t, tc.overlappingReason, rows[id].HistoryReason)
			}
			require.Equal(t, "partial", rows["a-other"].HistoryState)
			require.Equal(t, "Some multi-guest backup jobs could not be fully attributed to workloads.", rows["a-other"].HistoryReason)
			require.Equal(t, "complete", rows["b-first"].HistoryState, "a reused VMID on another connection keeps its evidence")
		})
	}
}
