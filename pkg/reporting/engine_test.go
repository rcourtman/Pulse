package reporting

import (
	"strings"
	"testing"
	"time"
)

func TestCSVGenerator_Generate(t *testing.T) {
	data := createTestReportData()

	gen := NewCSVGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("CSV generation failed: %v", err)
	}

	csv := string(result)

	// Check header
	if !strings.Contains(csv, "# Pulse Metrics Report") {
		t.Error("Missing report header")
	}
	if !strings.Contains(csv, "Test Report") {
		t.Error("Missing title")
	}
	if !strings.Contains(csv, "node") {
		t.Error("Missing resource type")
	}
	if !strings.Contains(csv, "test-node-1") {
		t.Error("Missing resource ID")
	}

	// Check summary section
	if !strings.Contains(csv, "# SUMMARY") {
		t.Error("Missing summary section")
	}
	if !strings.Contains(csv, "CPU Usage") {
		t.Error("Missing CPU metric in summary")
	}

	// Check data section
	if !strings.Contains(csv, "# DATA") {
		t.Error("Missing data section")
	}
	if !strings.Contains(csv, "Timestamp") {
		t.Error("Missing timestamp column header")
	}
}

func TestPDFGenerator_Generate(t *testing.T) {
	data := createTestReportData()

	gen := NewPDFGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("PDF generation failed: %v", err)
	}

	// Check PDF magic bytes
	if len(result) < 4 {
		t.Fatal("PDF too short")
	}
	if string(result[:4]) != "%PDF" {
		t.Error("Missing PDF magic bytes")
	}

	// Check reasonable size (should be at least a few KB)
	if len(result) < 1000 {
		t.Errorf("PDF seems too small: %d bytes", len(result))
	}
}

func TestPDFGenerator_EmptyData(t *testing.T) {
	data := &ReportData{
		Title:        "Empty Report",
		ResourceType: "node",
		ResourceID:   "empty-node",
		Start:        time.Now().Add(-1 * time.Hour),
		End:          time.Now(),
		GeneratedAt:  time.Now(),
		Metrics:      make(map[string][]MetricDataPoint),
		Summary: MetricSummary{
			ByMetric: make(map[string]MetricStats),
		},
	}

	gen := NewPDFGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("PDF generation failed for empty data: %v", err)
	}

	if string(result[:4]) != "%PDF" {
		t.Error("Missing PDF magic bytes for empty report")
	}
}

func TestCSVGenerator_EmptyData(t *testing.T) {
	data := &ReportData{
		Title:        "Empty Report",
		ResourceType: "node",
		ResourceID:   "empty-node",
		Start:        time.Now().Add(-1 * time.Hour),
		End:          time.Now(),
		GeneratedAt:  time.Now(),
		Metrics:      make(map[string][]MetricDataPoint),
		Summary: MetricSummary{
			ByMetric: make(map[string]MetricStats),
		},
	}

	gen := NewCSVGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("CSV generation failed for empty data: %v", err)
	}

	csv := string(result)
	if !strings.Contains(csv, "# Pulse Metrics Report") {
		t.Error("Missing header in empty report")
	}
}

func TestGetResourceTypeDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"node", "Node"},
		{"vm", "Virtual Machine"},
		{"container", "LXC Container"},
		{"dockerHost", "Docker Host"},
		{"dockerContainer", "Docker Container"},
		{"storage", "Storage"},
		{"unknown", "unknown"},
	}

	for _, tc := range tests {
		result := GetResourceTypeDisplayName(tc.input)
		if result != tc.expected {
			t.Errorf("GetResourceTypeDisplayName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetMetricTypeDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cpu", "CPU Usage"},
		{"memory", "Memory Usage"},
		{"disk", "Disk Usage"},
		{"usage", "Storage Usage"},
		{"used", "Used Space"},
		{"total", "Total Space"},
		{"avail", "Available Space"},
		{"unknown", "unknown"},
	}

	for _, tc := range tests {
		result := GetMetricTypeDisplayName(tc.input)
		if result != tc.expected {
			t.Errorf("GetMetricTypeDisplayName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetMetricUnit(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cpu", "%"},
		{"memory", "%"},
		{"disk", "%"},
		{"usage", "%"},
		{"used", "bytes"},
		{"total", "bytes"},
		{"avail", "bytes"},
		{"unknown", ""},
	}

	for _, tc := range tests {
		result := GetMetricUnit(tc.input)
		if result != tc.expected {
			t.Errorf("GetMetricUnit(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KiB"},
		{1536, "1.50 KiB"},
		{1048576, "1.00 MiB"},
		{1073741824, "1.00 GiB"},
		{1099511627776, "1.00 TiB"},
	}

	for _, tc := range tests {
		result := formatBytes(tc.input)
		if result != tc.expected {
			t.Errorf("formatBytes(%f) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestCSVGenerator_MultipleMetrics(t *testing.T) {
	now := time.Now()
	data := &ReportData{
		Title:        "Multi-Metric Test",
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        now.Add(-1 * time.Hour),
		End:          now,
		GeneratedAt:  now,
		Metrics: map[string][]MetricDataPoint{
			"cpu": {
				{Timestamp: now.Add(-30 * time.Minute), Value: 50.0},
				{Timestamp: now, Value: 60.0},
			},
			"memory": {
				{Timestamp: now.Add(-30 * time.Minute), Value: 70.0},
				{Timestamp: now, Value: 75.0},
			},
			"disk": {
				{Timestamp: now.Add(-30 * time.Minute), Value: 40.0},
				{Timestamp: now, Value: 42.0},
			},
		},
		Summary: MetricSummary{
			ByMetric: map[string]MetricStats{
				"cpu":    {MetricType: "cpu", Count: 2, Min: 50, Max: 60, Avg: 55, Current: 60},
				"memory": {MetricType: "memory", Count: 2, Min: 70, Max: 75, Avg: 72.5, Current: 75},
				"disk":   {MetricType: "disk", Count: 2, Min: 40, Max: 42, Avg: 41, Current: 42},
			},
		},
		TotalPoints: 6,
	}

	gen := NewCSVGenerator()
	result, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("CSV generation failed: %v", err)
	}

	csv := string(result)

	// Check all metrics are present in summary
	if !strings.Contains(csv, "CPU Usage") {
		t.Error("Missing CPU in summary")
	}
	if !strings.Contains(csv, "Memory Usage") {
		t.Error("Missing Memory in summary")
	}
	if !strings.Contains(csv, "Disk Usage") {
		t.Error("Missing Disk in summary")
	}

	// Check data rows
	lines := strings.Split(csv, "\n")
	dataStarted := false
	dataRows := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "# DATA") {
			dataStarted = true
			continue
		}
		if dataStarted && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "Timestamp") && line != "" {
			dataRows++
		}
	}

	// Should have data rows (timestamps may be merged or separate)
	if dataRows == 0 {
		t.Error("No data rows in CSV")
	}
}

// createTestReportData creates sample report data for testing.
func createTestReportData() *ReportData {
	now := time.Now()
	start := now.Add(-1 * time.Hour)

	// Create sample data points
	cpuPoints := make([]MetricDataPoint, 12)
	memPoints := make([]MetricDataPoint, 12)

	for i := 0; i < 12; i++ {
		ts := start.Add(time.Duration(i*5) * time.Minute)
		cpuPoints[i] = MetricDataPoint{
			Timestamp: ts,
			Value:     float64(50 + i*2),
			Min:       float64(48 + i*2),
			Max:       float64(52 + i*2),
		}
		memPoints[i] = MetricDataPoint{
			Timestamp: ts,
			Value:     float64(60 + i),
			Min:       float64(58 + i),
			Max:       float64(62 + i),
		}
	}

	return &ReportData{
		Title:        "Test Report",
		ResourceType: "node",
		ResourceID:   "test-node-1",
		Start:        start,
		End:          now,
		GeneratedAt:  now,
		Metrics: map[string][]MetricDataPoint{
			"cpu":    cpuPoints,
			"memory": memPoints,
		},
		Summary: MetricSummary{
			ByMetric: map[string]MetricStats{
				"cpu": {
					MetricType: "cpu",
					Count:      12,
					Min:        50,
					Max:        72,
					Avg:        61,
					Current:    72,
				},
				"memory": {
					MetricType: "memory",
					Count:      12,
					Min:        60,
					Max:        71,
					Avg:        65.5,
					Current:    71,
				},
			},
		},
		TotalPoints: 24,
	}
}
