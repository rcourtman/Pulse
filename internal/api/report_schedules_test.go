package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

func newTestReportingScheduleHandlers(t *testing.T) (*ReportingHandlers, *config.ConfigPersistence) {
	t.Helper()
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	mtm := monitoring.NewMultiTenantMonitor(&config.Config{DataPath: baseDir}, mtp, nil)
	t.Cleanup(mtm.Stop)
	monitor, err := mtm.GetMonitor("default")
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	persistence := monitor.GetConfigPersistence()
	if persistence == nil {
		t.Fatal("monitor config persistence is nil")
	}
	return NewReportingHandlers(mtm, nil), persistence
}

func TestRunReportScheduleRequiresCurrentAdvancedReportingEntitlement(t *testing.T) {
	handler := NewReportingHandlers(nil, nil)
	service := newLicenseService()
	handler.SetCommercialLicenseResolver(func(context.Context) *licenseService { return service })
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	schedule := config.NormalizeReportSchedule(config.ReportSchedule{
		ID:      "report-schedule-entitlement",
		Name:    "Entitlement proof",
		Enabled: true,
	})
	result, updated := handler.runReportSchedule(context.Background(), nil, schedule, now, false, "weekly:proof")
	if result.path != "" || result.email != "" {
		t.Fatalf("unentitled scheduler generated output: %+v", result)
	}
	if updated.LastRunStatus != config.ReportScheduleLastRunFailed || !strings.Contains(updated.LastError, "advanced reporting entitlement required") {
		t.Fatalf("unexpected unentitled result: %+v", updated)
	}
}

func TestPurgeGeneratedReportsAfterDowngradePreservesDefinitionsAndRejectsSymlinks(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewConfigPersistence(baseDir)
	schedule := config.NormalizeReportSchedule(config.ReportSchedule{ID: "report-schedule-purge", Name: "Purge proof"})
	report := generatedMultiReport{Filename: "report.pdf", Format: reporting.FormatPDF, Data: []byte("report")}
	path, err := saveGeneratedReport(persistence, schedule, report)
	if err != nil {
		t.Fatalf("save report: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(filepath.Dir(path), "outside-link.pdf")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := purgeGeneratedReportsAfterDowngrade(persistence); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated report still exists: %v", err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep" {
		t.Fatalf("outside target changed: data=%q err=%v", data, err)
	}
}

func validReportSchedulePayload() config.ReportSchedule {
	return config.ReportSchedule{
		Name:    "Monthly client report",
		Enabled: true,
		Cadence: config.ReportScheduleCadence{
			Type:       config.ReportScheduleCadenceMonthly,
			DayOfMonth: 1,
			Time:       "09:00",
			Timezone:   "UTC",
		},
		Scope: config.ReportScheduleScope{
			Resources: []config.ReportScheduleResource{{ResourceType: "vm", ResourceID: "vm-1", Name: "VM 1"}},
		},
		Format: config.ReportScheduleFormatPDF,
		Delivery: config.ReportScheduleDelivery{
			Method:     config.ReportScheduleDeliveryDisk,
			Attach:     false,
			SaveToDisk: true,
		},
		RetentionCount: 12,
	}
}

func encodeScheduleBody(t *testing.T, schedule config.ReportSchedule) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(schedule)
	if err != nil {
		t.Fatalf("marshal schedule: %v", err)
	}
	return bytes.NewReader(data)
}

func TestReportScheduleHandlersPersistCRUD(t *testing.T) {
	handlers, persistence := newTestReportingScheduleHandlers(t)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/reports/schedules", encodeScheduleBody(t, validReportSchedulePayload())).WithContext(ctx)
	createRec := httptest.NewRecorder()
	handlers.HandleCreateReportSchedule(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d body=%q", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	var created config.ReportSchedule
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created schedule ID is empty")
	}

	stored, err := persistence.LoadReportScheduleStore()
	if err != nil {
		t.Fatalf("LoadReportScheduleStore: %v", err)
	}
	if len(stored.Schedules) != 1 || stored.Schedules[0].ID != created.ID {
		t.Fatalf("stored schedules = %+v, want created schedule", stored.Schedules)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/reports/schedules", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	handlers.HandleListReportSchedules(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%q", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var list reportScheduleListResponse
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Schedules) != 1 || list.Schedules[0].ID != created.ID {
		t.Fatalf("list schedules = %+v, want created schedule", list.Schedules)
	}

	created.Enabled = false
	created.Name = "Updated monthly report"
	updateReq := httptest.NewRequest(http.MethodPut, "/api/admin/reports/schedules/"+created.ID, encodeScheduleBody(t, created)).WithContext(ctx)
	updateReq.SetPathValue("id", created.ID)
	updateRec := httptest.NewRecorder()
	handlers.HandleUpdateReportSchedule(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d body=%q", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}
	var updated config.ReportSchedule
	if err := json.NewDecoder(updateRec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if updated.Enabled || updated.Name != "Updated monthly report" {
		t.Fatalf("updated schedule = %+v, want disabled updated name", updated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/reports/schedules/"+created.ID, nil).WithContext(ctx)
	deleteReq.SetPathValue("id", created.ID)
	deleteRec := httptest.NewRecorder()
	handlers.HandleDeleteReportSchedule(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d body=%q", deleteRec.Code, http.StatusNoContent, deleteRec.Body.String())
	}
	stored, err = persistence.LoadReportScheduleStore()
	if err != nil {
		t.Fatalf("LoadReportScheduleStore after delete: %v", err)
	}
	if len(stored.Schedules) != 0 {
		t.Fatalf("stored schedules after delete = %+v, want empty", stored.Schedules)
	}
}

func TestRunReportSchedulePersistsFailureStatusWhenEngineUnavailable(t *testing.T) {
	original := reporting.GetEngine()
	reporting.SetEngine(nil)
	t.Cleanup(func() { reporting.SetEngine(original) })

	handlers, persistence := newTestReportingScheduleHandlers(t)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	schedule := validReportSchedulePayload()
	schedule.ID = "schedule-run-failure"
	if err := persistence.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{schedule}}); err != nil {
		t.Fatalf("SaveReportScheduleStore: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/reports/schedules/schedule-run-failure/run", nil).WithContext(ctx)
	req.SetPathValue("id", "schedule-run-failure")
	rec := httptest.NewRecorder()
	handlers.HandleRunReportSchedule(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("run status = %d, want %d body=%q", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	stored, err := persistence.LoadReportScheduleStore()
	if err != nil {
		t.Fatalf("LoadReportScheduleStore: %v", err)
	}
	if len(stored.Schedules) != 1 {
		t.Fatalf("stored schedule len = %d, want 1", len(stored.Schedules))
	}
	if stored.Schedules[0].LastRunStatus != config.ReportScheduleLastRunFailed {
		t.Fatalf("last_run_status = %q, want failed", stored.Schedules[0].LastRunStatus)
	}
	if stored.Schedules[0].LastRunAt == nil || stored.Schedules[0].LastError == "" {
		t.Fatalf("run status fields = lastRunAt %v lastError %q, want populated", stored.Schedules[0].LastRunAt, stored.Schedules[0].LastError)
	}
}

func TestSaveGeneratedReportUsesHashedScheduleDirectory(t *testing.T) {
	_, persistence := newTestReportingScheduleHandlers(t)
	schedule := validReportSchedulePayload()
	schedule.ID = "../tenant-owned-schedule"
	report := generatedMultiReport{
		Data:     []byte("report"),
		Filename: "../unsafe-report.pdf",
		Format:   reporting.FormatPDF,
	}

	path, err := saveGeneratedReport(persistence, schedule, report)
	if err != nil {
		t.Fatalf("saveGeneratedReport() error = %v", err)
	}

	wantDir := filepath.Join(
		persistence.DataDir(),
		"reports",
		"generated",
		securityutil.HashedStorageName(schedule.ID),
	)
	if filepath.Dir(path) != wantDir {
		t.Fatalf("report dir = %q, want %q", filepath.Dir(path), wantDir)
	}
	if strings.Contains(path, schedule.ID) {
		t.Fatalf("report path leaked raw schedule ID: %q", path)
	}
	if strings.ContainsAny(filepath.Base(path), `/\`) {
		t.Fatalf("report filename still contains a path separator: %q", filepath.Base(path))
	}
}
