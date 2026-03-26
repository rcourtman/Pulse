package reporting

import (
	"testing"
	"time"
)

func TestDescribeReportingCatalog_DefinesCanonicalSurfaces(t *testing.T) {
	catalog := DescribeReportingCatalog()

	if catalog.ID != "advanced_reporting" {
		t.Fatalf("catalog ID = %q, want advanced_reporting", catalog.ID)
	}
	if catalog.PerformanceReport.ID != "performance_reports" {
		t.Fatalf("performance report ID = %q, want performance_reports", catalog.PerformanceReport.ID)
	}
	if catalog.LockedState.Title != "Advanced Reporting (Pro)" {
		t.Fatalf("locked state title = %q, want Advanced Reporting (Pro)", catalog.LockedState.Title)
	}
	if catalog.Guidance.Title != "Advanced Insights" {
		t.Fatalf("guidance title = %q, want Advanced Insights", catalog.Guidance.Title)
	}
	if catalog.PerformanceReport.MultiResourceMax != 50 {
		t.Fatalf("multi-resource max = %d, want 50", catalog.PerformanceReport.MultiResourceMax)
	}
	if catalog.PerformanceReport.FilenameDateStyle != FilenameDateStyleUTCYYYYMMDD {
		t.Fatalf("filename date style = %q", catalog.PerformanceReport.FilenameDateStyle)
	}
	if got := len(catalog.PerformanceReport.Ranges); got != 3 {
		t.Fatalf("range count = %d, want 3", got)
	}
	if catalog.VMInventoryExport.ExportEndpoint != "/api/admin/reports/inventory/vms/export" {
		t.Fatalf("vm inventory export endpoint = %q", catalog.VMInventoryExport.ExportEndpoint)
	}
	if catalog.VMInventoryExport.FilenameDateStyle != FilenameDateStyleUTCYYYYMMDD {
		t.Fatalf("inventory filename date style = %q", catalog.VMInventoryExport.FilenameDateStyle)
	}
}

func TestPerformanceReportDefinition_SupportsFormat(t *testing.T) {
	definition := DescribePerformanceReport()

	if !definition.SupportsFormat(FormatPDF) {
		t.Fatalf("expected PDF to be supported")
	}
	if !definition.SupportsFormat(FormatCSV) {
		t.Fatalf("expected CSV to be supported")
	}
	if definition.SupportsFormat(ReportFormat("xlsx")) {
		t.Fatalf("did not expect xlsx to be supported")
	}
}

func TestPerformanceReportDefinition_DefaultRangeDuration(t *testing.T) {
	definition := DescribePerformanceReport()

	if got := definition.DefaultRangeDuration(); got != 24*time.Hour {
		t.Fatalf("default range duration = %s, want %s", got, 24*time.Hour)
	}
}

func TestPerformanceReportDefinition_InvalidFormatError(t *testing.T) {
	definition := DescribePerformanceReport()

	if got := definition.InvalidFormatError(); got != `Format must be "pdf" or "csv"` {
		t.Fatalf("invalid format error = %q", got)
	}
}

func TestPerformanceReportDefinition_AttachmentFilenamesUseUTCDateStamp(t *testing.T) {
	definition := DescribePerformanceReport()
	generatedAt := time.Date(2026, 3, 26, 0, 30, 0, 0, time.FixedZone("late", -7*60*60))

	if got := definition.SingleAttachmentFilename("node-1", generatedAt, FormatPDF); got != "report-node-1-20260326.pdf" {
		t.Fatalf("single attachment filename = %q", got)
	}
	if got := definition.MultiAttachmentFilename(generatedAt, FormatCSV); got != "fleet-report-20260326.csv" {
		t.Fatalf("multi attachment filename = %q", got)
	}
}

func TestVMInventoryExportDefinition_FormatContract(t *testing.T) {
	definition := DescribeVMInventoryExport()
	generatedAt := time.Date(2026, 3, 26, 23, 30, 0, 0, time.FixedZone("early", 2*60*60))

	if !definition.SupportsFormat(FormatCSV) {
		t.Fatalf("expected csv inventory export support")
	}
	if definition.SupportsFormat(FormatPDF) {
		t.Fatalf("did not expect pdf inventory export support")
	}
	if got := definition.InvalidFormatError(); got != `Format must be "csv"` {
		t.Fatalf("invalid inventory format error = %q", got)
	}
	if got := definition.AttachmentFilename(generatedAt); got != "vm-inventory-20260326.csv" {
		t.Fatalf("inventory attachment filename = %q", got)
	}
}
