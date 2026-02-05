package reporting

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestReportEngineGenerateMissingStore(t *testing.T) {
	engine := NewReportEngine(EngineConfig{})

	_, _, err := engine.Generate(MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        time.Now().Add(-1 * time.Hour),
		End:          time.Now(),
		Format:       FormatCSV,
	})
	if err == nil {
		t.Fatal("expected error when metrics store is nil")
	}
	if !strings.Contains(err.Error(), "metrics store not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportEngineGenerateUnsupportedFormat(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	_, _, err := engine.Generate(MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
		Start:        time.Now().Add(-1 * time.Hour),
		End:          time.Now(),
		Format:       ReportFormat("xls"),
	})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportEngineQueryMetricsSpecificMetric(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	start := time.Now().Add(-30 * time.Minute)
	points := []metrics.WriteMetric{
		{ResourceType: "node", ResourceID: "node-1", MetricType: "cpu", Value: 10, Timestamp: start, Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-1", MetricType: "cpu", Value: 30, Timestamp: start.Add(5 * time.Minute), Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-1", MetricType: "memory", Value: 50, Timestamp: start, Tier: metrics.TierRaw},
	}
	store.WriteBatchSync(points)

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	data, err := engine.queryMetrics(MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "node-1",
		MetricType:   "cpu",
		Start:        start.Add(-1 * time.Minute),
		End:          start.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("queryMetrics failed: %v", err)
	}

	if data.Title != "node Report: node-1" {
		t.Fatalf("expected default title, got %q", data.Title)
	}

	if len(data.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(data.Metrics))
	}
	if _, ok := data.Metrics["cpu"]; !ok {
		t.Fatal("expected cpu metric data to be present")
	}
	if _, ok := data.Metrics["memory"]; ok {
		t.Fatal("expected memory metric to be filtered out")
	}

	stats, ok := data.Summary.ByMetric["cpu"]
	if !ok {
		t.Fatal("expected cpu summary stats")
	}
	if stats.Count != 2 {
		t.Fatalf("expected cpu count 2, got %d", stats.Count)
	}
	if stats.Min != 10 || stats.Max != 30 {
		t.Fatalf("unexpected cpu min/max: %+v", stats)
	}
	if stats.Avg != 20 {
		t.Fatalf("expected cpu avg 20, got %v", stats.Avg)
	}
	if stats.Current != 30 {
		t.Fatalf("expected cpu current 30, got %v", stats.Current)
	}
}

func TestReportEngineQueryMetricsNoData(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	start := time.Now().Add(-1 * time.Hour)
	data, err := engine.queryMetrics(MetricReportRequest{
		ResourceType: "node",
		ResourceID:   "missing-node",
		Start:        start,
		End:          time.Now(),
	})
	if err != nil {
		t.Fatalf("queryMetrics failed: %v", err)
	}
	if data.TotalPoints != 0 {
		t.Fatalf("expected zero points, got %d", data.TotalPoints)
	}
	if len(data.Metrics) != 0 {
		t.Fatalf("expected no metrics, got %d", len(data.Metrics))
	}
	if data.Title != "node Report: missing-node" {
		t.Fatalf("expected default title, got %q", data.Title)
	}
}

func TestReportEngineGenerateMultiAggregatesData(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	base := time.Now().Add(-15 * time.Minute)
	store.WriteBatchSync([]metrics.WriteMetric{
		{ResourceType: "node", ResourceID: "node-1", MetricType: "cpu", Value: 10, Timestamp: base, Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-2", MetricType: "cpu", Value: 20, Timestamp: base.Add(2 * time.Minute), Tier: metrics.TierRaw},
	})

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	data, contentType, err := engine.GenerateMulti(MultiReportRequest{
		Resources: []MetricReportRequest{
			{ResourceType: "node", ResourceID: "node-1"},
			{ResourceType: "node", ResourceID: "node-2"},
		},
		MetricType: "cpu",
		Start:      base.Add(-1 * time.Minute),
		End:        base.Add(5 * time.Minute),
		Format:     FormatCSV,
	})
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}
	if contentType != "text/csv" {
		t.Fatalf("expected text/csv, got %s", contentType)
	}

	csv := string(data)
	if !strings.Contains(csv, "# Pulse Multi-Resource Metrics Report") {
		t.Fatal("missing multi-resource header")
	}
	if !strings.Contains(csv, "node-1") || !strings.Contains(csv, "node-2") {
		t.Fatal("expected both resource IDs in output")
	}
}

func newTestMetricsStore(t *testing.T) *metrics.Store {
	t.Helper()
	dir := t.TempDir()
	cfg := metrics.DefaultConfig(dir)
	cfg.DBPath = filepath.Join(dir, "metrics.db")
	cfg.WriteBufferSize = 1
	cfg.FlushInterval = 50 * time.Millisecond

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	return store
}
