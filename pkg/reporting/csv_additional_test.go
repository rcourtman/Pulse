package reporting

import (
	"strings"
	"testing"
	"time"
)

func TestFormatValue(t *testing.T) {
	if got := formatValue(1024, "bytes"); got != "1.00 KiB" {
		t.Fatalf("expected 1.00 KiB, got %q", got)
	}
	if got := formatValue(1.234, "%"); got != "1.23" {
		t.Fatalf("expected 1.23, got %q", got)
	}
}

func TestCSVGenerator_MissingMetricValues(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	data := &ReportData{
		Title:        "Missing Values",
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        now.Add(-time.Minute),
		End:          now.Add(2 * time.Minute),
		GeneratedAt:  now,
		Metrics: map[string][]MetricDataPoint{
			"cpu": {
				{Timestamp: now, Value: 50},
				{Timestamp: now.Add(time.Minute), Value: 60},
			},
			"memory": {
				{Timestamp: now, Value: 70},
			},
		},
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu":    {MetricType: "cpu", Count: 2, Min: 50, Max: 60, Avg: 55, Current: 60},
			"memory": {MetricType: "memory", Count: 1, Min: 70, Max: 70, Avg: 70, Current: 70},
		}},
		TotalPoints: 3,
	}

	gen := NewCSVGenerator()
	out, err := gen.Generate(data)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	lines := strings.Split(string(out), "\n")
	dataStart := false
	rowCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "# DATA") {
			dataStart = true
			continue
		}
		if !dataStart || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "Timestamp") || line == "" {
			continue
		}
		rowCount++
		if strings.HasPrefix(line, now.Add(time.Minute).Format(time.RFC3339)) {
			parts := strings.Split(line, ",")
			if len(parts) < 3 {
				t.Fatalf("unexpected data row: %q", line)
			}
			if parts[len(parts)-1] != "" {
				t.Fatalf("expected missing memory value, got %q", line)
			}
		}
	}
	if rowCount != 2 {
		t.Fatalf("expected 2 data rows, got %d", rowCount)
	}
}
