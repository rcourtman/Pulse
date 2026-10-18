package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestReportingActivityDownloadRequiresCompleteResponse(t *testing.T) {
	testutil.SetMockMode(t, false)
	p := config.NewConfigPersistence(t.TempDir())
	h := NewReportingHandlers(nil, nil)
	h.SetSystemSettingsStore(p)
	r := httptest.NewRequest(http.MethodGet, "/report", nil)
	h.recordReportingDownload(r, config.ReportScheduleKindResources, 12, nil, 12)
	h.recordReportingDownload(r, config.ReportScheduleKindResources, 6, nil, 12)
	h.recordReportingDownload(r, config.ReportScheduleKindResources, 0, errors.New("private-error"), 12)
	h.recordReportingDownload(httptest.NewRequest(http.MethodHead, "/report", nil), config.ReportScheduleKindResources, 12, nil, 12)
	ctx, cancel := context.WithCancel(r.Context())
	cancel()
	h.recordReportingDownload(r.WithContext(ctx), config.ReportScheduleKindResources, 12, nil, 12)
	got, err := p.ReportingActivitySnapshot(time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, got.Counts.Generated[0])
}

func TestReportingActivityRealSchedulerRetriesAndReuse(t *testing.T) {
	testutil.SetMockMode(t, false)
	h, p, _, ctx, s := reportDeliveryFixture(t)
	h.SetSystemSettingsStore(p)
	cfg, attempts := reportSMTP(t, []int{451, 250, 250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
	h.runDueReportSchedulesForOrg(ctx, "default", now)
	first := storedReport(t, p)
	require.Equal(t, "retrying", first.LastRunStatus)
	restarted := NewReportingHandlers(h.reportMonitorManager(), nil)
	restarted.SetSystemSettingsStore(p)
	restarted.runDueReportSchedulesForOrg(ctx, "default", first.RetryAt.Add(time.Second))
	require.Equal(t, "ok", storedReport(t, p).LastRunStatus)
	restarted.runDueReportSchedulesForOrg(ctx, "default", now.Add(time.Hour))
	require.EqualValues(t, 2, attempts.Load())
	got, err := p.ReportingActivitySnapshot(now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 2, got.Counts.Generated[0])
	require.Equal(t, 2, got.Counts.Saved)
	require.Equal(t, 1, got.Counts.DeliveryFailed)
	require.Equal(t, 1, got.Counts.EmailAccepted)
	require.Equal(t, 1, got.Counts.Successful[0])
	require.Zero(t, got.Counts.Repeated[0])
	restarted.runDueReportSchedulesForOrg(ctx, "default", now.AddDate(0, 0, 7))
	got, err = p.ReportingActivitySnapshot(now.AddDate(0, 0, 7))
	require.NoError(t, err)
	require.Equal(t, 2, got.Counts.Successful[0])
	require.Equal(t, 1, got.Counts.Repeated[0])
}

func TestReportingActivityPeriodsIgnoreSameDayRerunsAndIsolateOrganizations(t *testing.T) {
	testutil.SetMockMode(t, false)
	p := config.NewConfigPersistence(t.TempDir())
	h := NewReportingHandlers(nil, nil)
	h.SetSystemSettingsStore(p)
	s := validReportSchedulePayload()
	s.ID = "private-schedule"
	s.LastRunStatus = config.ReportScheduleLastRunOK
	s.Kind = config.ReportScheduleKindResources
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	start := now.AddDate(0, 0, -7)
	period := reportingActivityPeriod(s, start, now, now)
	require.Equal(t, period, reportingActivityPeriod(s, start.Add(time.Minute), now.Add(time.Minute), now))
	for _, org := range []string{"private-org-a", "private-org-a", "private-org-b"} {
		h.recordReportingSuccessfulPeriod(orgScopeContext(org), s, period, now)
	}
	got, err := p.ReportingActivitySnapshot(now)
	require.NoError(t, err)
	require.Equal(t, 2, got.Counts.Successful[0])
	require.Zero(t, got.Counts.Repeated[0])
	s.LastRunStatus = config.ReportScheduleLastRunFailed
	h.recordReportingSuccessfulPeriod(context.Background(), s, reportingActivityDigest("other-period"), now)
	got, err = p.ReportingActivitySnapshot(now)
	require.NoError(t, err)
	require.Equal(t, 2, got.Counts.Successful[0])
}

func TestReportingActivityUnconfirmedDeliveryAndUnreadableTally(t *testing.T) {
	testutil.SetMockMode(t, false)
	for _, unreadable := range []bool{false, true} {
		t.Run(map[bool]string{false: "SMTP uncertainty", true: "unreadable tally"}[unreadable], func(t *testing.T) {
			h, p, _, ctx, s := reportDeliveryFixture(t)
			recorder := config.NewConfigPersistence(t.TempDir())
			if unreadable {
				require.NoError(t, os.Mkdir(filepath.Join(recorder.GetConfigDir(), "reporting_activity.json"), 0700))
			}
			h.SetSystemSettingsStore(recorder)
			replies := []int{0}
			if unreadable {
				replies = []int{250}
			}
			cfg, _ := reportSMTP(t, replies, nil)
			require.NoError(t, p.SaveEmailConfig(cfg))
			require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
			now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
			h.runDueReportSchedulesForOrg(ctx, "default", now)
			stored := storedReport(t, p)
			if unreadable {
				require.Equal(t, "ok", stored.LastRunStatus)
				got, err := recorder.ReportingActivitySnapshot(now)
				require.Error(t, err)
				require.False(t, got.Available)
				return
			}
			require.Equal(t, "unconfirmed", stored.LastRunStatus)
			got, err := recorder.ReportingActivitySnapshot(now)
			require.NoError(t, err)
			require.Equal(t, 1, got.Counts.DeliveryUnconfirmed)
			require.Zero(t, got.Counts.EmailAccepted)
			require.Zero(t, got.Counts.Successful[0])
		})
	}
}

func TestReportingActivityCountsServedHandlersAndExcludesDeniedReads(t *testing.T) {
	testutil.SetMockMode(t, false)
	recorder := config.NewConfigPersistence(t.TempDir())
	h := newReportingOrgScopeFixture(t)
	h.SetSystemSettingsStore(recorder)
	for _, resource := range []string{"node-b", "node-a"} {
		r := httptest.NewRequest(http.MethodGet, "/api/admin/reports/generate?format=csv&resourceType=node&metricType=cpu&resourceId="+resource, nil).WithContext(orgScopeContext("org-b"))
		w := httptest.NewRecorder()
		h.HandleGenerateReport(w, r)
		if resource == "node-b" {
			require.Equal(t, http.StatusOK, w.Code)
		} else {
			require.Equal(t, http.StatusNotFound, w.Code)
		}
	}
	inventory, _, ctx, _, _ := inventoryScheduleFixture(t)
	inventory.SetSystemSettingsStore(recorder)
	w := httptest.NewRecorder()
	inventory.HandleExportVMInventory(w, httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export", nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, w.Code)
	got, err := recorder.ReportingActivitySnapshot(time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, got.Counts.Generated[0])
	require.Equal(t, 1, got.Counts.Generated[1])
	require.Equal(t, [4]int{}, got.Counts.Successful)
}

func TestReportingActivityMockDownloadsAreSuppressed(t *testing.T) {
	testutil.SetMockMode(t, true)
	p := config.NewConfigPersistence(t.TempDir())
	h := NewReportingHandlers(nil, nil)
	h.SetSystemSettingsStore(p)
	h.recordReportingDownload(httptest.NewRequest(http.MethodGet, "/report", nil), config.ReportScheduleKindResources, 12, nil, 12)
	got, err := p.ReportingActivitySnapshot(time.Now())
	require.NoError(t, err)
	require.Zero(t, got.Counts.Generated[0])
}
