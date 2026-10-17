package api

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/require"
)

func inventoryScheduleFixture(t *testing.T) (*ReportingHandlers, *config.ConfigPersistence, context.Context, config.ReportSchedule, func(int)) {
	t.Helper()
	// Build the real inventory and persistence without starting poll goroutines:
	// fixture setup must not replace state underneath a running monitor.
	state := models.NewState()
	monitor := newReportingMonitorForTest(t, state, nil)
	p := config.NewConfigPersistence(t.TempDir())
	setUnexportedField(t, monitor, "configPersist", p)
	h := NewReportingHandlers(newReportingMTMForTest(t, monitor), nil)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	setVMCount := func(count int) {
		vms := make([]models.VM, count)
		for i := range vms {
			vms[i] = models.VM{
				ID: fmt.Sprintf("fixture-node-%d", 100+i), VMID: 100 + i,
				Name: fmt.Sprintf("inventory-vm-%03d", i), Instance: "fixture", Node: "node", Pool: "customer",
				Type: "qemu", Status: "running", CPUs: 4,
				Memory: models.Memory{Total: 4 << 30}, Disk: models.Disk{Total: 100 << 30, Used: 20 << 30},
			}
		}
		state.UpdateVMs(vms)
		monitor.SetResourceStore(adapter)
	}
	setVMCount(61)
	s := validReportSchedulePayload()
	s.ID, s.Kind, s.Format = "delivery", config.ReportScheduleKindVMInventory, "csv"
	s.Scope = config.ReportScheduleScope{}
	s.Cadence = config.ReportScheduleCadence{Type: "weekly", Weekday: "monday", Time: "09:00", Timezone: "UTC"}
	return h, p, context.WithValue(context.Background(), OrgIDContextKey, "default"), s, setVMCount
}

func TestVMInventoryScheduleUsesExistingExportWithoutHistoricalLimit(t *testing.T) {
	h, p, ctx, s, setVMCount := inventoryScheduleFixture(t)
	now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
	prepared, err := h.prepareReportSchedule(ctx, s, now, false)
	require.NoError(t, err)
	result, finished := h.runReportSchedule(ctx, p, prepared, now, true, "")
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	require.Equal(t, "saved", finished.LastDeliveryStatus)
	data, err := os.ReadFile(result.path)
	require.NoError(t, err)
	download := httptest.NewRecorder()
	h.HandleExportVMInventory(download, httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export", nil).WithContext(ctx))
	require.Equal(t, http.StatusOK, download.Code)
	require.Equal(t, download.Body.Bytes(), data)
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 62) // Header plus every VM, including beyond the performance-report cap.
	require.Contains(t, string(data), "customer")
	require.Contains(t, string(data), "inventory-vm-060")
	require.Contains(t, string(data), "4294967296")
	setVMCount(62)
	result, finished = h.runReportSchedule(ctx, p, prepared, now.AddDate(0, 0, 7), false, "")
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	data, err = os.ReadFile(result.path)
	require.NoError(t, err)
	require.Contains(t, string(data), "inventory-vm-061")
}

func TestVMInventoryScheduleRejectsMisleadingFormatAndScope(t *testing.T) {
	h, _, ctx, s, _ := inventoryScheduleFixture(t)
	for _, tc := range []struct {
		name   string
		change func(*config.ReportSchedule)
	}{
		{"PDF", func(s *config.ReportSchedule) { s.Format = "pdf" }},
		{"resources", func(s *config.ReportSchedule) {
			s.Scope.Resources = []config.ReportScheduleResource{{ResourceType: "vm", ResourceID: "vm-1"}}
		}},
		{"tags", func(s *config.ReportSchedule) { s.Scope.Tags = []string{"client"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			incoming := s
			tc.change(&incoming)
			_, err := h.prepareReportSchedule(ctx, incoming, time.Now(), false)
			require.Error(t, err)
		})
	}
	s.Format = ""
	prepared, err := h.prepareReportSchedule(ctx, s, time.Now(), false)
	require.NoError(t, err)
	require.Equal(t, "csv", prepared.Format)
	s.Delivery.Method = "email"
	_, err = h.prepareReportSchedule(ctx, s, time.Now(), false)
	require.ErrorContains(t, err, "Enable email notifications")
	s.Enabled = false
	_, err = h.prepareReportSchedule(ctx, s, time.Now(), false)
	require.NoError(t, err)
}

func TestVMInventoryScheduleRetriesThroughPersistedDeliveryState(t *testing.T) {
	h, p, ctx, s, _ := inventoryScheduleFixture(t)
	cfg, attempts := reportSMTP(t, []int{451, 250}, nil)
	require.NoError(t, p.SaveEmailConfig(cfg))
	s.Delivery = config.ReportScheduleDelivery{Method: "email", Attach: true, SaveToDisk: true}
	require.NoError(t, p.SaveReportScheduleStore(config.ReportScheduleStore{Schedules: []config.ReportSchedule{s}}))
	now := time.Date(2026, 10, 19, 9, 0, 1, 0, time.UTC)
	h.runDueReportSchedulesForOrg(ctx, "default", now)
	first := storedReport(t, p)
	require.Equal(t, "retrying", first.LastRunStatus, first.LastError)
	require.NotNil(t, first.RetryAt)
	restarted := NewReportingHandlers(h.reportMonitorManager(), nil)
	restarted.runDueReportSchedulesForOrg(ctx, "default", first.RetryAt.Add(time.Second))
	finished := storedReport(t, p)
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	require.Equal(t, "vm_inventory", finished.Kind)
	require.Equal(t, "sent_with_attachment", finished.LastDeliveryStatus)
	require.Equal(t, 2, finished.RunAttempts)
	require.Nil(t, finished.RetryAt)
	require.EqualValues(t, 2, attempts.Load())
	restarted.runDueReportSchedulesForOrg(ctx, "default", now.Add(time.Hour))
	require.EqualValues(t, 2, attempts.Load())
}

func TestVMInventoryScheduleUnavailableOrganizationCannotExportDefault(t *testing.T) {
	h, p, ctx, s, _ := inventoryScheduleFixture(t)
	_, failed := h.runReportSchedule(context.WithValue(ctx, OrgIDContextKey, "missing-organization"), p, s, time.Now(), true, "")
	require.Equal(t, "failed", failed.LastRunStatus)
	require.Contains(t, failed.LastError, "VM inventory is unavailable")
	require.Empty(t, failed.LastReportPath)
	h.SetCommercialLicenseResolver(func(context.Context) *licenseService { return newLicenseService() })
	_, failed = h.runReportSchedule(ctx, p, s, time.Now(), true, "")
	require.Equal(t, "failed", failed.LastRunStatus)
	require.Contains(t, failed.LastError, "entitlement required")
	require.Empty(t, failed.LastReportPath)
}
