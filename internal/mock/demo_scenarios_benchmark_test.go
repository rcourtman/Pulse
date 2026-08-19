package mock

import (
	"testing"
	"time"
)

func BenchmarkBuildDefaultDemoFixtureGraph(b *testing.B) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		graph := buildFixtureGraph(cfg, now)
		if len(graph.State.Nodes) != DefaultConfig.NodeCount {
			b.Fatalf("node count = %d, want %d", len(graph.State.Nodes), DefaultConfig.NodeCount)
		}
	}
}

func BenchmarkUpdateDefaultDemoFixtureGraph(b *testing.B) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, now)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		graph.UpdateMetrics(cfg, now.Add(time.Duration(i+1)*cfg.UpdateInterval))
	}
}

func BenchmarkDefaultDemoUnifiedResourceSnapshot(b *testing.B) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false
	now := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, now)
	resources, _ := graph.UnifiedResourceSnapshot()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resources, _ = graph.UnifiedResourceSnapshot()
		if len(resources) <= 500 {
			b.Fatalf("resource count = %d, want a fleet-scale snapshot", len(resources))
		}
	}
	b.ReportMetric(float64(len(resources)), "resources")
}
