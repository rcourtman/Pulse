package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVMInventoryDownloadUnavailableIsNotAnEmptyFleet(t *testing.T) {
	populated, _, _, _, _ := inventoryScheduleFixture(t)
	for _, tc := range []struct {
		name    string
		handler *ReportingHandlers
		orgID   string
	}{
		{name: "nil handler"},
		{name: "missing monitor manager", handler: NewReportingHandlers(nil, nil)},
		{name: "missing organization", handler: populated, orgID: "missing-organization"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil)
			rec := httptest.NewRecorder()
			tc.handler.HandleExportVMInventory(rec, req.WithContext(orgScopeContext(tc.orgID)))

			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			require.Empty(t, rec.Header().Get("Content-Disposition"), "errors must not be offered as report attachments")
			var payload struct {
				Code       string `json:"code"`
				Error      string `json:"error"`
				StatusCode int    `json:"status_code"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
			require.Equal(t, "inventory_unavailable", payload.Code)
			require.Equal(t, vmInventoryUnavailableMessage, payload.Error)
			require.Equal(t, http.StatusServiceUnavailable, payload.StatusCode)
			require.NotContains(t, rec.Body.String(), "Resource ID")
			require.NotContains(t, rec.Body.String(), "inventory-vm-", "never fall back to another organization's inventory")
		})
	}
}

func TestVMInventoryDownloadAvailableEmptyFleetIsStillDownloadable(t *testing.T) {
	h := newReportingHandlersWithInventoryForTest(t, "empty-org")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil)
	rec := httptest.NewRecorder()
	h.HandleExportVMInventory(rec, req.WithContext(orgScopeContext("empty-org")))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "attachment;")
	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 1, "a known empty fleet still has exactly its CSV header")
	require.Equal(t, "Resource ID", rows[0][0])
}

func TestVMInventoryDownloadRecoversWhenItsMonitorBecomesAvailable(t *testing.T) {
	available, _, ctx, _, _ := inventoryScheduleFixture(t)
	h := NewReportingHandlers(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil).WithContext(ctx)
	before := httptest.NewRecorder()
	h.HandleExportVMInventory(before, req)
	require.Equal(t, http.StatusServiceUnavailable, before.Code)

	h.SetMultiTenantMonitor(available.reportMonitorManager())
	after := httptest.NewRecorder()
	h.HandleExportVMInventory(after, req)
	require.Equal(t, http.StatusOK, after.Code)
	require.Contains(t, after.Header().Get("Content-Disposition"), "attachment;")
	rows, err := csv.NewReader(strings.NewReader(after.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 62)
	require.Contains(t, after.Body.String(), "inventory-vm-060")
}
