package reporting

import "testing"

func TestDescribeReportingCatalog_DefinesCanonicalSurfaces(t *testing.T) {
	catalog := DescribeReportingCatalog()

	if catalog.ID != "advanced_reporting" {
		t.Fatalf("catalog ID = %q, want advanced_reporting", catalog.ID)
	}
	if catalog.PerformanceReport.ID != "performance_reports" {
		t.Fatalf("performance report ID = %q, want performance_reports", catalog.PerformanceReport.ID)
	}
	if catalog.PerformanceReport.MultiResourceMax != 50 {
		t.Fatalf("multi-resource max = %d, want 50", catalog.PerformanceReport.MultiResourceMax)
	}
	if got := len(catalog.PerformanceReport.Ranges); got != 3 {
		t.Fatalf("range count = %d, want 3", got)
	}
	if catalog.VMInventoryExport.ExportEndpoint != "/api/admin/reports/inventory/vms/export" {
		t.Fatalf("vm inventory export endpoint = %q", catalog.VMInventoryExport.ExportEndpoint)
	}
}
