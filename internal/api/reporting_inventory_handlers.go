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

	definition := reporting.DescribeVMInventoryExport()
	format := reporting.ReportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = definition.Format
	}
	if !definition.SupportsFormat(format) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_format", definition.InvalidFormatError(), nil)
		return
	}

	snapshot := emptyReportingEnrichmentSnapshot()
	if h != nil {
		if liveSnapshot, ok := h.getReportingEnrichmentSnapshot(r.Context(), GetOrgID(r.Context())); ok {
			snapshot = liveSnapshot
		}
	}

	generatedAt := time.Now().UTC()
	report, err := generateVMInventoryReport(snapshot, generatedAt)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "generation_failed", "Failed to generate VM inventory export", nil)
		return
	}

	w.Header().Set("Content-Type", report.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", report.Filename))
	_, _ = w.Write(report.Data)
}

// Both downloads and schedules use the same current-state projection and CSV
// contract. Inventory does not use historical metric windows or resource caps.
func generateVMInventoryReport(snapshot reportingEnrichmentSnapshot, generatedAt time.Time) (generatedMultiReport, error) {
	definition := reporting.DescribeVMInventoryExport()
	data, err := reporting.GenerateVMInventoryCSV(reporting.VMInventoryData{
		GeneratedAt: generatedAt,
		Rows:        buildVMInventoryRows(snapshot.Resources),
	})
	return generatedMultiReport{
		Data: data, Format: definition.Format, ContentType: "text/csv; charset=utf-8",
		Filename: definition.AttachmentFilename(generatedAt),
	}, err
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
			Pool:                 view.Pool(),
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
