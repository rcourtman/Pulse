package reporting

import (
	"testing"
	"time"
)

func TestPDFGenerator_DisksAndAlerts(t *testing.T) {
	data := createTestReportData()
	data.Resource = &ResourceInfo{Name: "node-1", Uptime: 7200}
	data.Disks = []DiskInfo{
		{Device: "sda", Model: "Samsung-SSD", Size: 1 << 30, Health: "PASSED", Temperature: 55, WearLevel: 25},
		{Device: "sdb", Model: "VeryLongModelNameThatWillBeTrimmedInTable", Size: 2 << 30, Health: "FAILED", Temperature: 65, WearLevel: 5},
	}
	data.Alerts = []AlertInfo{
		{Type: "cpu", Level: "critical", Message: "High CPU", Value: 95, Threshold: 90, StartTime: time.Now()},
	}

	gen := NewPDFGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("PDF generation failed: %v", err)
	}
	if len(result) < 4 || string(result[:4]) != "%PDF" {
		t.Fatal("expected PDF output with magic bytes")
	}
}

func TestGetMetricColor(t *testing.T) {
	if got := getMetricColor("cpu"); got != colorSecondary {
		t.Fatal("expected cpu color to be secondary")
	}
	if got := getMetricColor("memory"); got == colorSecondary {
		t.Fatal("expected memory color to differ from secondary")
	}
	if got := getMetricColor("disk"); got != colorAccent {
		t.Fatal("expected disk color to be accent")
	}
	if got := getMetricColor("usage"); got != colorAccent {
		t.Fatal("expected usage color to be accent")
	}
	if got := getMetricColor("unknown"); got != colorSecondary {
		t.Fatal("expected default color to be secondary")
	}
}
