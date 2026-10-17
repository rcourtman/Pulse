package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReportScheduleDeliverySurvivesReload(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "encrypted"}[encrypted], func(t *testing.T) {
			dir := t.TempDir()
			persistence := newPlainReportSchedulePersistence(dir)
			if encrypted {
				persistence = NewConfigPersistence(dir)
			}
			now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
			retry := now.Add(time.Minute)
			schedule := ReportSchedule{
				ID: "delivery", LastRunAt: &now, LastRunStatus: ReportScheduleLastRunRetrying,
				LastError: "Mail server temporarily unavailable", LastDeliveryStatus: "failed",
				LastReportPath: "/owned/reports/report.csv", LastSuccessAt: &now,
				RunAttempts: 1, RetryAt: &retry, RunWindowEnd: &now, RunManual: true,
				LastOccurrenceKey: "weekly:2026-10-17T09:00:00Z:UTC",
			}
			require.NoError(t, persistence.SaveReportScheduleStore(ReportScheduleStore{Schedules: []ReportSchedule{schedule}}))
			loaded, err := persistence.LoadReportScheduleStore()
			require.NoError(t, err)
			require.Equal(t, NormalizeReportSchedule(schedule), loaded.Schedules[0])
		})
	}
}

func TestLegacyReportScheduleHasNoInventedDeliveryOutcome(t *testing.T) {
	store, err := parseReportScheduleStoreJSON([]byte(`[{"id":"old","last_run_status":"ok"}]`))
	require.NoError(t, err)
	require.Equal(t, ReportScheduleLastRunOK, store.Schedules[0].LastRunStatus)
	require.Empty(t, store.Schedules[0].LastDeliveryStatus)
	require.Nil(t, store.Schedules[0].RetryAt)
	require.Zero(t, store.Schedules[0].RunAttempts)
}

func TestVMInventoryScheduleKindSurvivesEncryptedReload(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	s := ReportSchedule{ID: "inventory", Kind: ReportScheduleKindVMInventory, Format: "csv"}
	require.NoError(t, p.SaveReportScheduleStore(ReportScheduleStore{Schedules: []ReportSchedule{s}}))
	loaded, err := p.LoadReportScheduleStore()
	require.NoError(t, err)
	require.Equal(t, ReportScheduleKindVMInventory, loaded.Schedules[0].Kind)
	require.Equal(t, "csv", loaded.Schedules[0].Format)
}
