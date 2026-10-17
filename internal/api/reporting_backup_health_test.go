package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery/model"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
	"github.com/stretchr/testify/require"
)

// Public tests exercise the scoped evidence contract and shared delivery.
// Real private renderer output is tested in pulse-enterprise and the browser.
func installBackupHealthTestRenderer(t *testing.T) {
	t.Helper()
	previous := resolveBackupHealthReportGenerator()
	SetBackupHealthReportGenerator(func(data reporting.BackupHealthData, _ reporting.ReportFormat) ([]byte, string, error) {
		bytes, err := json.Marshal(data)
		return bytes, "application/json", err
	})
	t.Cleanup(func() { SetBackupHealthReportGenerator(previous) })
}
func readBackupHealthTestPayload(t *testing.T, data []byte) reporting.BackupHealthData {
	t.Helper()
	var payload reporting.BackupHealthData
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}

func TestBackupHealthRequiresPrivateRenderer(t *testing.T) {
	previous := resolveBackupHealthReportGenerator()
	SetBackupHealthReportGenerator(nil)
	t.Cleanup(func() { SetBackupHealthReportGenerator(previous) })
	h, _, ctx, s, _ := inventoryScheduleFixture(t)
	s.Kind = config.ReportScheduleKindBackupHealth
	_, err := h.generateBackupHealthReport(ctx, s, time.Now(), true)
	require.ErrorContains(t, err, "rendering is unavailable")
	w := httptest.NewRecorder()
	h.HandleGetReportingCatalog(w, httptest.NewRequest(http.MethodGet, "/", nil))
	var catalog reporting.ReportingCatalog
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &catalog))
	require.False(t, catalog.BackupHealthAvailable)
	require.NotContains(t, w.Body.String(), "backupHealthAvailable", "runtimes without the extension retain the existing catalog shape")
	installBackupHealthTestRenderer(t)
	w = httptest.NewRecorder()
	h.HandleGetReportingCatalog(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &catalog))
	require.True(t, catalog.BackupHealthAvailable)
}

func TestBackupHealthWeeklyWindowUsesSevenCompletedLocalDaysAcrossDST(t *testing.T) {
	installBackupHealthTestRenderer(t)
	h, _, ctx, s, _ := inventoryScheduleFixture(t)
	s.Kind = config.ReportScheduleKindBackupHealth
	s.Cadence.Timezone = "Europe/London"
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	report, err := h.generateBackupHealthReport(ctx, s, now, true)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), report.End)
	require.Equal(t, 169*time.Hour, report.End.Sub(report.Start))
	require.Len(t, readBackupHealthTestPayload(t, report.Data).Summary.Days, 7)
}

func TestBackupHealthScheduleDataQualityCannotBeForgedByClient(t *testing.T) {
	h, p, ctx, s, _ := inventoryScheduleFixture(t)
	s.Kind = config.ReportScheduleKindBackupHealth
	s.Enabled = false
	s.LastReportDataState = "complete"
	s.LastReportDataMessage = "untrusted completeness claim"
	w := httptest.NewRecorder()
	h.HandleCreateReportSchedule(w, httptest.NewRequest(http.MethodPost, "/api/admin/reports/schedules", encodeScheduleBody(t, s)).WithContext(ctx))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := storedReport(t, p)
	require.Empty(t, created.LastReportDataState)
	require.Empty(t, created.LastReportDataMessage)
}

func TestBackupHealthScheduleReportsRetainedFailuresBeyondPerformanceCap(t *testing.T) {
	installBackupHealthTestRenderer(t)
	h, p, ctx, s, _ := inventoryScheduleFixture(t)
	h.recoveryManager = recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := h.recoveryManager.StoreForOrg("default")
	require.NoError(t, err)
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Second)
	at := now.Add(-48 * time.Hour)
	point := recovery.RecoveryPoint{ID: "pve-task:fixture-failed", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal, Outcome: recovery.OutcomeFailed, StartedAt: &at, CompletedAt: &at, Details: map[string]any{"instance": "fixture", "vmid": 100, "status": "ERROR: storage unavailable"}}
	observation := model.BackupHistoryCollection{Provider: recovery.ProviderProxmoxPVE, Instance: "fixture", ObservedAt: now.Add(-8 * 24 * time.Hour), Complete: true, MaxGap: 9 * 24 * time.Hour}
	require.NoError(t, store.RecordBackupHistory(ctx, []recovery.RecoveryPoint{point}, []model.BackupHistoryCollection{observation}))
	observation.ObservedAt = now
	require.NoError(t, store.RecordBackupHistory(ctx, nil, []model.BackupHistoryCollection{observation}))
	s.Kind = config.ReportScheduleKindBackupHealth
	s, err = h.prepareReportSchedule(ctx, s, now, false)
	require.NoError(t, err)
	generated, err := h.generateBackupHealthReport(ctx, s, now, true)
	require.NoError(t, err)
	require.Equal(t, 61, generated.ResourceCount)
	require.Equal(t, "complete", generated.DataState)
	require.Equal(t, 0, generated.End.UTC().Hour())
	require.Equal(t, generated.End.AddDate(0, 0, -7), generated.Start)
	payload := readBackupHealthTestPayload(t, generated.Data)
	require.Equal(t, 1, payload.Summary.Counts.Failed)
	require.Len(t, payload.Summary.Workloads, 61)
	require.Len(t, payload.Summary.Failures, 1)
	require.Contains(t, payload.Summary.Failures[0].Detail, "storage unavailable")
	result, finished := h.runReportSchedule(ctx, p, s, now, true, "")
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	require.Equal(t, "complete", finished.LastReportDataState)
	require.NotEmpty(t, finished.LastReportDataMessage)
	data, err := os.ReadFile(result.path)
	require.NoError(t, err)
	require.Contains(t, string(data), "fixture-failed")
}

func TestBackupHealthUnavailableHistoryDoesNotLookLikeZeroFailures(t *testing.T) {
	installBackupHealthTestRenderer(t)
	h, _, ctx, s, _ := inventoryScheduleFixture(t)
	s.Kind = config.ReportScheduleKindBackupHealth
	report, err := h.generateBackupHealthReport(ctx, s, time.Now().UTC(), true)
	require.NoError(t, err)
	require.Equal(t, "unavailable", report.DataState)
	payload := readBackupHealthTestPayload(t, report.Data)
	require.Equal(t, "unavailable", payload.Summary.HistoryState)
	require.Zero(t, payload.Summary.Counts.Total())
	require.Equal(t, "unavailable", payload.Summary.Workloads[0].HistoryState)
	require.Contains(t, payload.HistoryNotice, "no successful backups are assumed")
	h.SetCommercialLicenseResolver(func(context.Context) *licenseService { return newLicenseService() })
	_, failed := h.runReportSchedule(ctx, config.NewConfigPersistence(t.TempDir()), s, time.Now(), true, "")
	require.Equal(t, "failed", failed.LastRunStatus)
	require.Contains(t, failed.LastError, "entitlement required")
}

func TestBackupHealthScopeCannotIncludeAnotherClientsTasks(t *testing.T) {
	h, _, ctx, _, _ := inventoryScheduleFixture(t)
	snapshot, ok := h.getReportingEnrichmentSnapshot(ctx, "default")
	require.True(t, ok)
	require.Greater(t, len(snapshot.Resources), 1)
	snapshot.Resources[1].Tags = []string{"client-b"}
	scope := config.ReportScheduleScope{Resources: []config.ReportScheduleResource{{ResourceType: "vm", ResourceID: snapshot.Resources[0].ID}}, Tags: []string{"client-b"}}
	workloads, err := backupHealthWorkloads(snapshot, scope)
	require.NoError(t, err)
	require.Len(t, workloads, 2)
	workloads, err = backupHealthWorkloads(snapshot, config.ReportScheduleScope{Resources: []config.ReportScheduleResource{{ResourceType: "vm", ResourceID: "other-org-resource"}}})
	require.NoError(t, err)
	require.Empty(t, workloads)
	_, err = backupHealthWorkloads(snapshot, config.ReportScheduleScope{Resources: []config.ReportScheduleResource{{ResourceType: "host", ResourceID: "host"}}})
	require.Error(t, err)
	otherOrg := context.WithValue(ctx, OrgIDContextKey, "unconnected-organization")
	_, err = h.generateBackupHealthReport(otherOrg, config.ReportSchedule{}, time.Now(), true)
	require.ErrorContains(t, err, "scope is unavailable")
	bad := validReportSchedulePayload()
	bad.ID = "backup-scope"
	bad.Kind = config.ReportScheduleKindBackupHealth
	bad.Scope = config.ReportScheduleScope{Tags: []string{"client\nforged"}}
	_, err = h.prepareReportSchedule(ctx, bad, time.Now(), false)
	require.ErrorContains(t, err, "control characters")
	container := snapshot.Resources[0]
	container.ID, container.Type = "container-1", unifiedresources.ResourceTypeSystemContainer
	snapshot.Resources = append(snapshot.Resources, container)
	workloads, err = backupHealthWorkloads(snapshot, config.ReportScheduleScope{Resources: []config.ReportScheduleResource{{ResourceID: container.ID, ResourceType: "system-container"}}})
	require.NoError(t, err)
	require.Len(t, workloads, 1)
	require.Equal(t, "system-container", workloads[0].Type)
}

func TestBackupHealthRetriesKeepWindowAndExposeDataState(t *testing.T) {
	installBackupHealthTestRenderer(t)
	h, p, ctx, s, _ := inventoryScheduleFixture(t)
	s.Kind = config.ReportScheduleKindBackupHealth
	cfg, attempts := reportSMTP(t, []int{451, 250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	s.Delivery = config.ReportScheduleDelivery{Method: "email", Attach: true, SaveToDisk: true}
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	now := time.Now().UTC()
	// Drive an explicit manual attempt so this test does not depend on weekday.
	store := config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}
	_, first, err := h.executeReportSchedule(ctx, p, &store, 0, now, true, "", false)
	require.NoError(t, err)
	require.Equal(t, "retrying", first.LastRunStatus)
	require.Equal(t, "unavailable", first.LastReportDataState)
	require.NotNil(t, first.RetryAt)
	firstData, err := os.ReadFile(first.LastReportPath)
	require.NoError(t, err)
	restarted := NewReportingHandlers(h.reportMonitorManager(), nil)
	restarted.runDueReportSchedulesForOrg(ctx, "default", first.RetryAt.Add(time.Second))
	finished := storedReport(t, p)
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	require.Equal(t, "unavailable", finished.LastReportDataState)
	require.EqualValues(t, 2, attempts.Load())
	secondData, err := os.ReadFile(finished.LastReportPath)
	require.NoError(t, err)
	firstPayload := readBackupHealthTestPayload(t, firstData)
	secondPayload := readBackupHealthTestPayload(t, secondData)
	require.Equal(t, firstPayload.Summary.Start, secondPayload.Summary.Start)
	require.Equal(t, firstPayload.Summary.End, secondPayload.Summary.End)
}
