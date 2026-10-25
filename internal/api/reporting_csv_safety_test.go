package api

import (
	"context"
	"encoding/csv"
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

func TestReportingInventoryDownloadAndSavedScheduleEscapeSpreadsheetText(t *testing.T) {
	monitor := newReportingMonitorForTest(t, models.NewState(), []unifiedresources.Resource{{
		ID: "vm-formula", Type: unifiedresources.ResourceTypeVM, Name: "=1+1",
		Status: unifiedresources.StatusOnline,
		Proxmox: &unifiedresources.ProxmoxData{
			Instance: "+1+1", NodeName: "\t@node", Pool: "@SUM(1,1)", VMID: 101,
			CPUs: 2, DiskStatusReason: "\rreason",
		},
	}})
	p := config.NewConfigPersistence(t.TempDir())
	setUnexportedField(t, monitor, "configPersist", p)
	h := NewReportingHandlers(newReportingMTMForTest(t, monitor), nil)
	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	request := httptest.NewRequest(http.MethodGet, "/api/admin/reports/inventory/vms/export?format=csv", nil).WithContext(ctx)
	download := httptest.NewRecorder()
	h.HandleExportVMInventory(download, request)
	require.Equal(t, http.StatusOK, download.Code)
	require.Equal(t, "text/csv; charset=utf-8", download.Header().Get("Content-Type"))
	require.Contains(t, download.Header().Get("Content-Disposition"), "attachment;")
	records, err := csv.NewReader(strings.NewReader(download.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, []string{
		"vm-formula", "'+1+1", "'\t@node", "'@SUM(1,1)", "101", "'=1+1", "online", "2", "0", "0", "0", "'\rreason",
	}, records[1])

	s := validReportSchedulePayload()
	s.ID, s.Kind, s.Format = "safe-csv", config.ReportScheduleKindVMInventory, "csv"
	s.Scope = config.ReportScheduleScope{}
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)
	prepared, err := h.prepareReportSchedule(ctx, s, now, false)
	require.NoError(t, err)
	result, finished := h.runReportSchedule(ctx, p, prepared, now, true, "")
	require.Equal(t, "ok", finished.LastRunStatus, finished.LastError)
	require.Equal(t, "saved", finished.LastDeliveryStatus)
	saved, err := os.ReadFile(result.path)
	require.NoError(t, err)
	require.Equal(t, download.Body.Bytes(), saved, "scheduled files must use the same safe CSV producer as downloads")

	snapshot, ok := h.getReportingEnrichmentSnapshot(ctx, "default")
	require.True(t, ok)
	require.Len(t, snapshot.Resources, 1)
	require.Equal(t, "=1+1", snapshot.Resources[0].Name, "escape only the exported cell, never the monitored name")
}
