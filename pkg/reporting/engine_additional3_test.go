package reporting

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestReportEngineGenerateMultiDefaultTitleAndMetricFilter(t *testing.T) {
	store := newTestMetricsStore(t)
	defer store.Close()

	base := time.Now().Add(-10 * time.Minute)
	store.WriteBatchSync([]metrics.WriteMetric{
		{ResourceType: "node", ResourceID: "node-1", MetricType: "cpu", Value: 10, Timestamp: base, Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-1", MetricType: "memory", Value: 50, Timestamp: base, Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-2", MetricType: "cpu", Value: 20, Timestamp: base, Tier: metrics.TierRaw},
		{ResourceType: "node", ResourceID: "node-2", MetricType: "memory", Value: 60, Timestamp: base, Tier: metrics.TierRaw},
	})

	engine := NewReportEngine(EngineConfig{MetricsStore: store})

	data, _, err := engine.GenerateMulti(MultiReportRequest{
		Resources: []MetricReportRequest{
			{ResourceType: "node", ResourceID: "node-1"},
			{ResourceType: "node", ResourceID: "node-2"},
		},
		Start:      base.Add(-1 * time.Minute),
		End:        base.Add(1 * time.Minute),
		Format:     FormatCSV,
		MetricType: "cpu",
	})
	if err != nil {
		t.Fatalf("GenerateMulti failed: %v", err)
	}

	csv := string(data)
	if !strings.Contains(csv, "Fleet Performance Report") {
		t.Fatal("expected default multi-report title")
	}
	if !strings.Contains(csv, "CPU Usage") {
		t.Fatal("expected CPU metric in output")
	}
	if strings.Contains(csv, "Memory Usage") {
		t.Fatal("did not expect Memory metric when filtering on cpu")
	}
}
