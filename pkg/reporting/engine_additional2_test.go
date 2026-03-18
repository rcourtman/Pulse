package reporting

import (
	"strings"
	"testing"
	"time"
)

func TestReportEngineGenerateMultiMissingStore(t *testing.T) {
	engine := NewReportEngine(EngineConfig{})

	_, _, err := engine.GenerateMulti(MultiReportRequest{
		Resources: []MetricReportRequest{{ResourceType: "node", ResourceID: "node-1"}},
		Start:     time.Now().Add(-1 * time.Hour),
		End:       time.Now(),
		Format:    FormatCSV,
	})
	if err == nil {
		t.Fatal("expected error when metrics store is nil")
	}
	if !strings.Contains(err.Error(), "metrics store not initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportEngineGenerateMultiUnsupportedFormat(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	_, _, err := engine.GenerateMulti(MultiReportRequest{
		Resources: []MetricReportRequest{{ResourceType: "node", ResourceID: "node-1"}},
		Start:     time.Now().Add(-1 * time.Hour),
		End:       time.Now(),
		Format:    ReportFormat("xls"),
	})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportEngineGenerateMultiAllResourcesFail(t *testing.T) {
	store := newTestMetricsStore(t)
	store.Close()

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	_, _, err := engine.GenerateMulti(MultiReportRequest{
		Resources: []MetricReportRequest{
			{ResourceType: "node", ResourceID: "node-1"},
			{ResourceType: "node", ResourceID: "node-2"},
		},
		Start:  time.Now().Add(-1 * time.Hour),
		End:    time.Now(),
		Format: FormatCSV,
	})
	if err == nil {
		t.Fatal("expected error when all resources fail")
	}
	if !strings.Contains(err.Error(), "all resources failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
