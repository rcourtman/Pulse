package reporting

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/stretchr/testify/require"
)

func TestMetricEvidenceRetainsObservedCoverageAcrossRestart(t *testing.T) {
	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 8 * 24 * time.Hour
	store, err := metrics.NewStore(cfg)
	require.NoError(t, err)
	start := time.Now().Add(-6 * time.Hour).Truncate(time.Minute)
	for i, delta := range []time.Duration{0, 2 * time.Minute, time.Hour} {
		store.Write("node", "native-node", "memory", float64(88+i), start.Add(delta))
	}
	require.NoError(t, store.Close())
	// A retained point can carry a mean and separately recorded extrema.
	// Preserve both the peak and its timestamp rather than using its mean.
	db, err := sql.Open("sqlite", cfg.DBPath)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE metrics SET min_value = 80, max_value = 100 WHERE timestamp = ?", start.Add(2*time.Minute).Unix())
	require.NoError(t, err)
	require.NoError(t, db.Close())
	store, err = metrics.NewStore(cfg)
	require.NoError(t, err)
	defer store.Close()
	engine := NewReportEngine(EngineConfig{MetricsStore: store})
	narrator := &stubNarrator{err: errors.New("must not run")}
	req := MetricReportRequest{ResourceID: "canonical-agent", MetricsResourceID: "native-node", ResourceType: "node", Start: start.Add(-18 * time.Hour), End: start.Add(6 * time.Hour), Narrator: narrator}
	evidence, err := engine.MetricEvidenceFor(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "canonical-agent", evidence.ResourceID)
	reading := evidence.Metrics["memory"]
	require.Equal(t, 3, reading.RetainedPoints)
	require.Equal(t, "%", reading.Unit)
	require.Equal(t, 89.0, reading.Mean)
	require.Equal(t, 90.0, reading.Latest)
	require.Equal(t, 80.0, reading.Min)
	require.Equal(t, 100.0, reading.Max)
	require.True(t, reading.MinBucketAt.Equal(start.Add(2*time.Minute)))
	require.True(t, reading.MaxBucketAt.Equal(start.Add(2*time.Minute)))
	require.True(t, reading.FirstAt.Equal(start))
	require.True(t, reading.LastAt.Equal(start.Add(time.Hour)))
	require.Equal(t, (58 * time.Minute).Seconds(), reading.MaxGapSeconds)
	require.Empty(t, narrator.seen.ResourceID, "evidence must not invoke report judgment")

	req.MetricsResourceID = "unobserved-node"
	empty, err := engine.MetricEvidenceFor(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, empty.Metrics)
	require.Empty(t, empty.Metrics)

	require.NoError(t, store.Close())
	_, err = engine.MetricEvidenceFor(context.Background(), req)
	require.Error(t, err, "query errors must not become empty evidence")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = engine.MetricEvidenceFor(ctx, req)
	require.ErrorIs(t, err, context.Canceled)
}
