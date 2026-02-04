package reporting

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMultiReportVisualInspection generates actual PDF and CSV files
// for visual inspection. Run with:
//
//	go test ./pkg/reporting/ -run TestMultiReportVisualInspection -v
//
// Output files will be written to the system temp directory.
func TestMultiReportVisualInspection(t *testing.T) {
	outDir := t.TempDir()

	data := createMultiTestReportData()

	// Generate multi-resource PDF
	pdfGen := NewPDFGenerator()
	pdfBytes, err := pdfGen.GenerateMulti(data)
	if err != nil {
		t.Fatalf("Multi-resource PDF generation failed: %v", err)
	}

	pdfPath := filepath.Join(outDir, "multi-resource-report.pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0644); err != nil {
		t.Fatalf("Failed to write PDF: %v", err)
	}
	t.Logf("Multi-resource PDF written to: %s (%d bytes, %d pages estimated)", pdfPath, len(pdfBytes), estimatePages(pdfBytes))

	// Generate multi-resource CSV
	csvGen := NewCSVGenerator()
	csvBytes, err := csvGen.GenerateMulti(data)
	if err != nil {
		t.Fatalf("Multi-resource CSV generation failed: %v", err)
	}

	csvPath := filepath.Join(outDir, "multi-resource-report.csv")
	if err := os.WriteFile(csvPath, csvBytes, 0644); err != nil {
		t.Fatalf("Failed to write CSV: %v", err)
	}
	t.Logf("Multi-resource CSV written to: %s (%d bytes)", csvPath, len(csvBytes))

	// Also generate a single-resource PDF for comparison
	singlePdfBytes, err := pdfGen.Generate(data.Resources[0])
	if err != nil {
		t.Fatalf("Single-resource PDF generation failed: %v", err)
	}
	singlePdfPath := filepath.Join(outDir, "single-resource-report.pdf")
	if err := os.WriteFile(singlePdfPath, singlePdfBytes, 0644); err != nil {
		t.Fatalf("Failed to write single PDF: %v", err)
	}
	t.Logf("Single-resource PDF written to: %s (%d bytes)", singlePdfPath, len(singlePdfBytes))

	// Basic sanity checks
	if string(pdfBytes[:4]) != "%PDF" {
		t.Error("Multi PDF missing magic bytes")
	}
	if len(pdfBytes) < 5000 {
		t.Errorf("Multi PDF seems too small: %d bytes", len(pdfBytes))
	}

	// CSV content checks
	csvStr := string(csvBytes)
	expectedStrings := []string{
		"# Pulse Multi-Resource Metrics Report",
		"# SUMMARY",
		"# DATA",
		"Resource Name",
		"Resource Type",
		"Resource ID",
		"pve1-node1",     // node resource ID
		"web-01",         // VM name
		"db-container-1", // container name
		"CPU Usage",
		"Memory Usage",
	}
	for _, s := range expectedStrings {
		if !contains(csvStr, s) {
			t.Errorf("CSV missing expected content: %q", s)
		}
	}

	// Copy files to a persistent location so the user can inspect them
	persistDir := filepath.Join(os.TempDir(), "pulse-report-visual-test")
	os.MkdirAll(persistDir, 0755)

	copyFile(t, pdfPath, filepath.Join(persistDir, "multi-resource-report.pdf"))
	copyFile(t, csvPath, filepath.Join(persistDir, "multi-resource-report.csv"))
	copyFile(t, singlePdfPath, filepath.Join(persistDir, "single-resource-report.pdf"))

	t.Logf("\n=== FILES FOR VISUAL INSPECTION ===")
	t.Logf("Multi-resource PDF:   %s", filepath.Join(persistDir, "multi-resource-report.pdf"))
	t.Logf("Multi-resource CSV:   %s", filepath.Join(persistDir, "multi-resource-report.csv"))
	t.Logf("Single-resource PDF:  %s", filepath.Join(persistDir, "single-resource-report.pdf"))
	t.Logf("Open with: open %s", persistDir)
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (len(s) >= len(substr)) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Logf("Warning: could not read %s: %v", src, err)
		return
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Logf("Warning: could not write %s: %v", dst, err)
	}
}

func estimatePages(pdfBytes []byte) int {
	// Count "/Type /Page" occurrences as rough page estimate
	count := 0
	needle := []byte("/Type /Page\n")
	for i := 0; i <= len(pdfBytes)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if pdfBytes[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			count++
		}
	}
	if count == 0 {
		return -1
	}
	return count
}

// createMultiTestReportData creates rich test data with multiple resources,
// enrichment data, alerts, storage, backups, and disk info.
func createMultiTestReportData() *MultiReportData {
	now := time.Now()
	start := now.Add(-7 * 24 * time.Hour) // 7 days

	return &MultiReportData{
		Title:       "Fleet Performance Report",
		Start:       start,
		End:         now,
		GeneratedAt: now,
		Resources: []*ReportData{
			createNodeReportData("pve1-node1", "pve1", start, now),
			createNodeReportData("pve1-node2", "pve2", start, now),
			createVMReportData("pve1-node1-100", "web-01", start, now),
			createVMReportData("pve1-node1-101", "api-server", start, now),
			createContainerReportData("pve1-node2-200", "db-container-1", start, now),
		},
		TotalPoints: 168 * 3 * 5, // 168 points * 3 metrics * 5 resources
	}
}

func createNodeReportData(id, name string, start, end time.Time) *ReportData {
	cpuPoints := generateMetricPoints(start, end, 168, 15, 85, 45)
	memPoints := generateMetricPoints(start, end, 168, 40, 92, 72)
	diskPoints := generateMetricPoints(start, end, 168, 30, 55, 42)

	cpuTemp := 62.0

	return &ReportData{
		Title:        fmt.Sprintf("Node Report: %s", name),
		ResourceType: "node",
		ResourceID:   id,
		Start:        start,
		End:          end,
		GeneratedAt:  time.Now(),
		Metrics: map[string][]MetricDataPoint{
			"cpu":    cpuPoints,
			"memory": memPoints,
			"disk":   diskPoints,
		},
		Summary:     computeSummary(cpuPoints, memPoints, diskPoints),
		TotalPoints: len(cpuPoints) + len(memPoints) + len(diskPoints),
		Resource: &ResourceInfo{
			Name:          name,
			DisplayName:   name,
			Status:        "online",
			Host:          fmt.Sprintf("https://%s.local:8006", name),
			Instance:      "pve1",
			Uptime:        432000, // 5 days
			KernelVersion: "6.8.12-1-pve",
			PVEVersion:    "8.3.1",
			CPUModel:      "AMD EPYC 7543P 32-Core Processor",
			CPUCores:      32,
			CPUSockets:    1,
			MemoryTotal:   137438953472,  // 128 GiB
			DiskTotal:     1099511627776, // 1 TiB
			LoadAverage:   []float64{2.45, 1.82, 1.64},
			Temperature:   &cpuTemp,
			Tags:          []string{"production", "cluster-a"},
			ClusterName:   "prod-cluster",
			IsCluster:     true,
		},
		Alerts: []AlertInfo{
			{
				Type:      "cpu",
				Level:     "warning",
				Message:   "CPU usage above 80% for 5 minutes",
				Value:     84.5,
				Threshold: 80,
				StartTime: end.Add(-2 * time.Hour),
			},
		},
		Storage: []StorageInfo{
			{
				Name:      "local-zfs",
				Type:      "zfspool",
				Status:    "active",
				Total:     536870912000,
				Used:      268435456000,
				Available: 268435456000,
				UsagePerc: 50.0,
				Content:   "rootdir,images",
			},
			{
				Name:      "local-lvm",
				Type:      "lvmthin",
				Status:    "active",
				Total:     1099511627776,
				Used:      659706976666,
				Available: 439804651110,
				UsagePerc: 60.0,
				Content:   "images,rootdir",
			},
			{
				Name:      "nfs-backups",
				Type:      "nfs",
				Status:    "active",
				Total:     4398046511104,
				Used:      3958241860394,
				Available: 439804651110,
				UsagePerc: 90.1,
				Content:   "backup",
			},
		},
		Disks: []DiskInfo{
			{
				Device:      "/dev/nvme0n1",
				Model:       "Samsung SSD 990 PRO 2TB",
				Serial:      "S6Z2NF0WB12345",
				Type:        "nvme",
				Size:        2000398934016,
				Health:      "PASSED",
				Temperature: 38,
				WearLevel:   95,
			},
			{
				Device:      "/dev/sda",
				Model:       "WDC WD40EFAX-68JH4N1",
				Serial:      "WD-WXB2D8123456",
				Type:        "hdd",
				Size:        4000787030016,
				Health:      "PASSED",
				Temperature: 34,
				WearLevel:   -1,
			},
		},
	}
}

func createVMReportData(id, name string, start, end time.Time) *ReportData {
	cpuPoints := generateMetricPoints(start, end, 168, 5, 95, 22)
	memPoints := generateMetricPoints(start, end, 168, 50, 98, 88)
	diskPoints := generateMetricPoints(start, end, 168, 20, 45, 35)

	backupTime := end.Add(-24 * time.Hour)
	return &ReportData{
		Title:        fmt.Sprintf("VM Report: %s", name),
		ResourceType: "vm",
		ResourceID:   id,
		Start:        start,
		End:          end,
		GeneratedAt:  time.Now(),
		Metrics: map[string][]MetricDataPoint{
			"cpu":    cpuPoints,
			"memory": memPoints,
			"disk":   diskPoints,
		},
		Summary:     computeSummary(cpuPoints, memPoints, diskPoints),
		TotalPoints: len(cpuPoints) + len(memPoints) + len(diskPoints),
		Resource: &ResourceInfo{
			Name:        name,
			DisplayName: name,
			Status:      "running",
			Node:        "pve1",
			Instance:    "pve1",
			Uptime:      864000, // 10 days
			OSName:      "Ubuntu",
			OSVersion:   "24.04",
			IPAddresses: []string{"10.0.1.50", "192.168.1.50"},
			CPUCores:    4,
			MemoryTotal: 8589934592,  // 8 GiB
			DiskTotal:   53687091200, // 50 GiB
			Tags:        []string{"web", "production"},
		},
		Alerts: []AlertInfo{
			{
				Type:      "memory",
				Level:     "critical",
				Message:   "Memory usage above 95% - OOM risk",
				Value:     96.2,
				Threshold: 95,
				StartTime: end.Add(-30 * time.Minute),
			},
			{
				Type:         "cpu",
				Level:        "warning",
				Message:      "CPU usage above 80% sustained",
				Value:        88.4,
				Threshold:    80,
				StartTime:    end.Add(-6 * time.Hour),
				ResolvedTime: timePtr(end.Add(-4 * time.Hour)),
			},
		},
		Backups: []BackupInfo{
			{
				Type:      "vzdump",
				Storage:   "nfs-backups",
				Timestamp: backupTime,
				Size:      4294967296, // 4 GiB
				Protected: true,
				VolID:     "nfs-backups:vzdump/vzdump-qemu-100-2024_01_01-00_00_00.vma.zst",
			},
			{
				Type:      "vzdump",
				Storage:   "nfs-backups",
				Timestamp: backupTime.Add(-24 * time.Hour),
				Size:      4190000000,
				Protected: false,
				VolID:     "nfs-backups:vzdump/vzdump-qemu-100-2023_12_31-00_00_00.vma.zst",
			},
		},
	}
}

func createContainerReportData(id, name string, start, end time.Time) *ReportData {
	cpuPoints := generateMetricPoints(start, end, 168, 2, 35, 12)
	memPoints := generateMetricPoints(start, end, 168, 30, 65, 48)
	diskPoints := generateMetricPoints(start, end, 168, 10, 25, 18)

	backupTime := end.Add(-48 * time.Hour)
	return &ReportData{
		Title:        fmt.Sprintf("Container Report: %s", name),
		ResourceType: "container",
		ResourceID:   id,
		Start:        start,
		End:          end,
		GeneratedAt:  time.Now(),
		Metrics: map[string][]MetricDataPoint{
			"cpu":    cpuPoints,
			"memory": memPoints,
			"disk":   diskPoints,
		},
		Summary:     computeSummary(cpuPoints, memPoints, diskPoints),
		TotalPoints: len(cpuPoints) + len(memPoints) + len(diskPoints),
		Resource: &ResourceInfo{
			Name:        name,
			DisplayName: name,
			Status:      "running",
			Node:        "pve2",
			Instance:    "pve1",
			Uptime:      2592000, // 30 days
			OSName:      "Debian",
			OSVersion:   "12",
			IPAddresses: []string{"10.0.2.10"},
			CPUCores:    2,
			MemoryTotal: 2147483648,  // 2 GiB
			DiskTotal:   10737418240, // 10 GiB
			Tags:        []string{"database", "staging"},
		},
		Backups: []BackupInfo{
			{
				Type:      "vzdump",
				Storage:   "nfs-backups",
				Timestamp: backupTime,
				Size:      1073741824, // 1 GiB
				Protected: false,
				VolID:     "nfs-backups:vzdump/vzdump-lxc-200-2024_01_01-00_00_00.tar.zst",
			},
		},
	}
}

// generateMetricPoints creates realistic metric data with variation.
func generateMetricPoints(start, end time.Time, count int, minVal, maxVal, baseVal float64) []MetricDataPoint {
	points := make([]MetricDataPoint, count)
	duration := end.Sub(start)
	interval := duration / time.Duration(count)

	for i := 0; i < count; i++ {
		ts := start.Add(time.Duration(i) * interval)

		// Generate realistic variation using sine waves
		hourFraction := float64(i) / float64(count)
		// Daily pattern (higher during "business hours")
		dailyPattern := math.Sin(hourFraction*2*math.Pi) * 0.3
		// Random-ish noise using multiple sine waves
		noise := math.Sin(float64(i)*0.7) * 0.1
		noise += math.Sin(float64(i)*1.3) * 0.05

		val := baseVal + (maxVal-minVal)*(dailyPattern+noise)
		val = math.Max(minVal, math.Min(maxVal, val))

		points[i] = MetricDataPoint{
			Timestamp: ts,
			Value:     val,
			Min:       math.Max(minVal, val-3),
			Max:       math.Min(maxVal, val+3),
		}
	}
	return points
}

// computeSummary calculates summary statistics from metric points.
func computeSummary(cpuPoints, memPoints, diskPoints []MetricDataPoint) MetricSummary {
	return MetricSummary{
		ByMetric: map[string]MetricStats{
			"cpu":    computeStats("cpu", cpuPoints),
			"memory": computeStats("memory", memPoints),
			"disk":   computeStats("disk", diskPoints),
		},
	}
}

func computeStats(metricType string, points []MetricDataPoint) MetricStats {
	if len(points) == 0 {
		return MetricStats{MetricType: metricType}
	}

	stats := MetricStats{
		MetricType: metricType,
		Count:      len(points),
		Min:        points[0].Value,
		Max:        points[0].Value,
		Current:    points[len(points)-1].Value,
	}

	var sum float64
	for _, p := range points {
		sum += p.Value
		if p.Value < stats.Min {
			stats.Min = p.Value
		}
		if p.Value > stats.Max {
			stats.Max = p.Value
		}
	}
	stats.Avg = sum / float64(len(points))
	return stats
}

func timePtr(t time.Time) *time.Time {
	return &t
}
