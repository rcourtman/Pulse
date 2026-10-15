package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStorageInvestigationHistoryKeepsRequestedWindowAndStoreErrors(t *testing.T) {
	monitor := newChartFallbackTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Second)
	writeRawMetricBatch(t, monitor.metricsStore, "storage", "pool", "usage", []MetricPoint{
		{Timestamp: now.Add(-3 * time.Hour), Value: 44},
	})
	// Sparkline gap filling remains available, but an investigation asking for
	// the last hour must not receive a three-hour-old sample as that history.
	charts := monitor.GetStorageMetricsForChart("pool", time.Hour)
	require.NotEmpty(t, charts["usage"])
	series, _, err := monitor.ReadStorageMetricHistory("pool", time.Hour)
	require.NoError(t, err)
	require.Empty(t, series["usage"])
	series, source, err := monitor.ReadStorageMetricHistory("pool", 7*24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "store", source)
	require.Len(t, series["usage"], 1)
	require.Equal(t, 44.0, series["usage"][0].Value)
	// Even with an available memory sample, a failed retained read is explicit.
	monitor.metricsHistory.AddStorageMetric("pool", "usage", 45, now)
	require.NoError(t, monitor.metricsStore.Close())
	_, _, err = monitor.ReadStorageMetricHistory("pool", time.Hour)
	require.Error(t, err)
}
