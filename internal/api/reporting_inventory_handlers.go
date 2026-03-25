package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/reporting"
)

// HandleExportVMInventory exports the current VM inventory as spreadsheet-friendly CSV.
func (h *ReportingHandlers) HandleExportVMInventory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	format := reporting.ReportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = reporting.FormatCSV
	}
	if format != reporting.FormatCSV {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_format", "Format must be 'csv'", nil)
		return
	}

	snapshot := emptyReportingEnrichmentSnapshot()
	if h != nil {
		if liveSnapshot, ok := h.getReportingEnrichmentSnapshot(r.Context(), GetOrgID(r.Context())); ok {
			snapshot = liveSnapshot
		}
	}

	data, err := reporting.GenerateVMInventoryCSV(reporting.VMInventoryData{
		GeneratedAt: time.Now().UTC(),
		Rows:        buildVMInventoryRows(snapshot.Resources),
	})
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate VM inventory export", nil)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	filename := fmt.Sprintf("vm-inventory-%s.csv", time.Now().Format("20060102"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	_, _ = w.Write(data)
}

func buildVMInventoryRows(resources []unifiedresources.Resource) []reporting.VMInventoryRow {
	rows := make([]reporting.VMInventoryRow, 0, len(resources))
	for i := range resources {
		resource := &resources[i]
		if resource.Type != unifiedresources.ResourceTypeVM {
			continue
		}
		view := unifiedresources.NewVMView(resource)
		diskAllocated, diskUsed := resolveVMInventoryDiskUsage(view)
		rows = append(rows, reporting.VMInventoryRow{
			ResourceID:           view.ID(),
			Instance:             view.Instance(),
			Node:                 view.Node(),
			VMID:                 view.VMID(),
			Name:                 view.Name(),
			Status:               string(view.Status()),
			CPUCores:             view.CPUs(),
			MemoryAllocatedBytes: view.MemoryTotal(),
			DiskAllocatedBytes:   diskAllocated,
			DiskUsedBytes:        diskUsed,
			DiskStatusReason:     view.DiskStatusReason(),
		})
	}
	return rows
}

func resolveVMInventoryDiskUsage(view unifiedresources.VMView) (allocated int64, used int64) {
	allocated = view.DiskTotal()
	used = view.DiskUsed()
	if allocated > 0 || used > 0 {
		return allocated, used
	}

	for _, disk := range view.Disks() {
		allocated += disk.Total
		used += disk.Used
	}
	return allocated, used
}
