package reporting

import (
	"testing"
	"time"
)

func TestPDFGenerator_BackupsSection(t *testing.T) {
	data := createTestReportData()
	data.Resource = &ResourceInfo{
		Name:   "node-1",
		Uptime: 3600,
	}
	data.Backups = []BackupInfo{
		{
			Type:      "vzdump",
			Storage:   "local",
			Timestamp: time.Now(),
			Size:      1024,
			Protected: true,
		},
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

func TestFormatUptime(t *testing.T) {
	if got := formatUptime(59); got != "0m" {
		t.Fatalf("expected 0m, got %q", got)
	}
	if got := formatUptime(3600); got != "1h 0m" {
		t.Fatalf("expected 1h 0m, got %q", got)
	}
	if got := formatUptime(90061); got != "1d 1h 1m" {
		t.Fatalf("expected 1d 1h 1m, got %q", got)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(30 * time.Minute); got != "30 minutes" {
		t.Fatalf("expected 30 minutes, got %q", got)
	}
	if got := formatDuration(90 * time.Minute); got != "1 hour, 30 minutes" {
		t.Fatalf("expected 1 hour, 30 minutes, got %q", got)
	}
	if got := formatDuration(24 * time.Hour); got != "1 day" {
		t.Fatalf("expected 1 day, got %q", got)
	}
	if got := formatDuration(49 * time.Hour); got != "2 days, 1 hour" {
		t.Fatalf("expected 2 days, 1 hour, got %q", got)
	}
}

// Disk temperatures in a report take the disk's alert thresholds, not a fixed
// 50/60C: an NVMe at 63C under a 70C trigger stays uncoloured, a SATA disk at
// 56C past its 55C trigger is red, and a disk without thresholds is plain.
func TestDiskTemperatureSeverityFollowsAlertThresholds(t *testing.T) {
	cases := []struct {
		disk DiskInfo
		want string
	}{
		{DiskInfo{Temperature: 63, TemperatureWarning: 65, TemperatureCritical: 70}, ""},
		{DiskInfo{Temperature: 66, TemperatureWarning: 65, TemperatureCritical: 70}, "warning"},
		{DiskInfo{Temperature: 70, TemperatureWarning: 65, TemperatureCritical: 70}, "critical"},
		{DiskInfo{Temperature: 56, TemperatureWarning: 50, TemperatureCritical: 55}, "critical"},
		{DiskInfo{Temperature: 72, TemperatureWarning: 70, TemperatureCritical: 75}, "warning"},
		{DiskInfo{Temperature: 80}, ""},
		{DiskInfo{Temperature: 0, TemperatureWarning: 50, TemperatureCritical: 55}, ""},
	}
	for _, tc := range cases {
		if got := diskTemperatureSeverity(tc.disk); got != tc.want {
			t.Errorf("diskTemperatureSeverity(%+v) = %q, want %q", tc.disk, got, tc.want)
		}
	}
}
