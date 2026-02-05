package reporting

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateObservations_Default(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{Summary: MetricSummary{ByMetric: make(map[string]MetricStats)}}
	obs := g.generateObservations(data)
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if !strings.Contains(obs[0].text, "Insufficient data") {
		t.Fatalf("unexpected observation: %q", obs[0].text)
	}
}

func TestGenerateObservations_MixedSignals(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu":    {Avg: 50, Max: 95},
			"memory": {Avg: 90, Max: 92},
			"disk":   {Avg: 88, Max: 90},
		}},
		Alerts: []AlertInfo{{ResolvedTime: ptrTime(time.Now())}},
		Disks:  []DiskInfo{{Device: "nvme0", WearLevel: 5, Health: "FAILED"}},
		Resource: &ResourceInfo{
			Uptime: 100 * 86400,
		},
	}

	obs := g.generateObservations(data)
	if len(obs) < 5 {
		t.Fatalf("expected multiple observations, got %d", len(obs))
	}
	assertObservationContains(t, obs, "CPU peaked")
	assertObservationContains(t, obs, "Memory consistently high")
	assertObservationContains(t, obs, "Disk at")
	assertObservationContains(t, obs, "alerts were triggered")
	assertObservationContains(t, obs, "CRITICAL: Disk nvme0")
	assertObservationContains(t, obs, "System uptime")
}

func TestGenerateObservations_UnderutilizedCPU(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu": {Avg: 10, Max: 15},
		}},
	}

	obs := g.generateObservations(data)
	assertObservationContains(t, obs, "underutilized")
}

func TestGenerateRecommendations(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu":    {Avg: 40, Max: 95},
			"memory": {Avg: 90, Max: 95},
			"disk":   {Avg: 90, Max: 92},
		}},
		Storage: []StorageInfo{{Name: "local", UsagePerc: 95}},
		Disks:   []DiskInfo{{Device: "sda", WearLevel: 5, Health: "FAILED"}},
		Resource: &ResourceInfo{
			Uptime: 100 * 86400,
		},
	}

	recs := g.generateRecommendations(data, 2, 1)
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	assertStringSliceContains(t, recs, "Replace disk sda")
	assertStringSliceContains(t, recs, "SMART health check failed")
	assertStringSliceContains(t, recs, "critical alerts")
	assertStringSliceContains(t, recs, "adding memory")
	assertStringSliceContains(t, recs, "CPU-intensive")
	assertStringSliceContains(t, recs, "Clean up disk space")
	assertStringSliceContains(t, recs, "Expand storage pool 'local'")
	assertStringSliceContains(t, recs, "Schedule maintenance window")
}

func TestGenerateRecommendations_Underutilized(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"cpu": {Avg: 5, Max: 10},
		}},
	}

	recs := g.generateRecommendations(data, 0, 0)
	assertStringSliceContains(t, recs, "underutilized")
}

func TestGenerateRecommendations_DefaultMessage(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{}

	recs := g.generateRecommendations(data, 0, 0)
	assertStringSliceContains(t, recs, "No immediate action required")
}

func TestGenerateRecommendations_UsageMetric(t *testing.T) {
	g := NewPDFGenerator()
	data := &ReportData{
		Summary: MetricSummary{ByMetric: map[string]MetricStats{
			"usage": {Avg: 90, Max: 95},
		}},
	}

	recs := g.generateRecommendations(data, 0, 0)
	assertStringSliceContains(t, recs, "Clean up disk space")
}

func TestGetStatColor(t *testing.T) {
	if got := getStatColor(95); got != colorDanger {
		t.Fatalf("expected danger color")
	}
	if got := getStatColor(80); got != colorWarning {
		t.Fatalf("expected warning color")
	}
	if got := getStatColor(50); got != colorAccent {
		t.Fatalf("expected accent color")
	}
}

func TestGetAlertCountColor(t *testing.T) {
	if got := getAlertCountColor(1); got != colorDanger {
		t.Fatal("expected danger color when alerts present")
	}
	if got := getAlertCountColor(0); got != colorAccent {
		t.Fatal("expected accent color when no alerts")
	}
}

func TestCalculateTrend(t *testing.T) {
	g := NewPDFGenerator()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	makePoints := func(values []float64) []MetricDataPoint {
		points := make([]MetricDataPoint, len(values))
		for i, v := range values {
			points[i] = MetricDataPoint{Timestamp: base.Add(time.Duration(i) * time.Minute), Value: v}
		}
		return points
	}

	dataUp := &ReportData{Metrics: map[string][]MetricDataPoint{
		"cpu": makePoints([]float64{10, 10, 10, 10, 10, 20, 20, 20, 20, 20}),
	}}
	if trend := g.calculateTrend(dataUp, "cpu"); trend != "(trending up)" {
		t.Fatalf("expected trending up, got %q", trend)
	}

	dataDown := &ReportData{Metrics: map[string][]MetricDataPoint{
		"cpu": makePoints([]float64{20, 20, 20, 20, 20, 10, 10, 10, 10, 10}),
	}}
	if trend := g.calculateTrend(dataDown, "cpu"); trend != "(trending down)" {
		t.Fatalf("expected trending down, got %q", trend)
	}

	dataStable := &ReportData{Metrics: map[string][]MetricDataPoint{
		"cpu": makePoints([]float64{10, 10, 10, 10, 10, 10.1, 10.2, 10.1, 10.2, 10.1}),
	}}
	if trend := g.calculateTrend(dataStable, "cpu"); trend != "(stable)" {
		t.Fatalf("expected stable, got %q", trend)
	}

	dataShort := &ReportData{Metrics: map[string][]MetricDataPoint{
		"cpu": makePoints([]float64{10, 12, 11}),
	}}
	if trend := g.calculateTrend(dataShort, "cpu"); trend != "" {
		t.Fatalf("expected empty trend, got %q", trend)
	}
}

func assertObservationContains(t *testing.T, obs []observation, needle string) {
	t.Helper()
	for _, o := range obs {
		if strings.Contains(o.text, needle) {
			return
		}
	}
	t.Fatalf("expected observation containing %q", needle)
}

func assertStringSliceContains(t *testing.T, items []string, needle string) {
	t.Helper()
	for _, item := range items {
		if strings.Contains(item, needle) {
			return
		}
	}
	t.Fatalf("expected slice to contain %q", needle)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
