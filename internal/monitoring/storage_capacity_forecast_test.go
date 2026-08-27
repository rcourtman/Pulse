package monitoring

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestStorageCapacityTrendUsesDurableHistoryAfterRestart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	config := metrics.DefaultConfig(t.TempDir())
	config.DBPath = filepath.Join(t.TempDir(), "capacity-metrics.db")
	store, err := metrics.NewStore(config)
	if err != nil {
		t.Fatalf("new metrics store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	writes := make([]metrics.WriteMetric, 0, 72)
	for hour := 72; hour >= 1; hour-- {
		writes = append(writes, metrics.WriteMetric{
			ResourceType: "storage",
			ResourceID:   "durable-storage",
			MetricType:   "usage",
			Value:        60 + float64(72-hour)*0.08,
			Timestamp:    now.Add(-time.Duration(hour) * time.Hour),
			Tier:         metrics.TierHourly,
		})
	}
	store.WriteBatchSync(writes)

	// Deliberately omit MetricsHistory: this models the process immediately
	// after restart, when the in-memory ring is empty but SQLite remains.
	monitor := &Monitor{metricsStore: store}
	trend := monitor.storageCapacityTrend(models.Storage{
		ID:     "durable-storage",
		Name:   "archive",
		Status: "active",
		Usage:  65.76,
	}, now)
	if !trend.Ready || trend.Reason != "increasing" {
		t.Fatalf("trend = %+v, want trusted increasing evidence from durable history", trend)
	}
	if trend.CoverageSpan < 48*time.Hour {
		t.Fatalf("coverage = %s, want at least 48h", trend.CoverageSpan)
	}
	if trend.Confidence < 0.8 {
		t.Fatalf("confidence = %.3f, want alert-grade evidence", trend.Confidence)
	}
}
