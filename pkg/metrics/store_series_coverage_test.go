package metrics

import (
	"testing"
	"time"
)

func TestMaxTimestampsForTier(t *testing.T) {
	store, err := NewStore(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	base := time.Now().UTC().Truncate(time.Hour)
	store.WriteBatchSync([]WriteMetric{
		{ResourceType: "vm", ResourceID: "vm-1", MetricType: "cpu", Value: 1, Timestamp: base.Add(-2 * time.Hour), Tier: TierHourly},
		{ResourceType: "vm", ResourceID: "vm-1", MetricType: "cpu", Value: 2, Timestamp: base, Tier: TierHourly},
		{ResourceType: " VM ", ResourceID: " vm-2 ", MetricType: " CPU ", Value: 3, Timestamp: base.Add(-time.Hour), Tier: TierHourly},
		{ResourceType: "vm", ResourceID: "vm-1", MetricType: "cpu", Value: 4, Timestamp: base.Add(time.Hour), Tier: TierRaw},
	})

	coverage, err := store.MaxTimestampsForTier(TierHourly)
	if err != nil {
		t.Fatalf("failed to read hourly coverage: %v", err)
	}
	if len(coverage) != 2 {
		t.Fatalf("expected 2 hourly series, got %d: %v", len(coverage), coverage)
	}
	if got := coverage[NormalizedSeriesKey("vm", "vm-1", "cpu")]; !got.Equal(base) {
		t.Fatalf("expected vm-1 hourly coverage at %s, got %s", base, got)
	}
	// Coverage keys are stored normalized, so messy caller identifiers must
	// resolve through NormalizedSeriesKey.
	if got := coverage[NormalizedSeriesKey(" VM ", " vm-2 ", " CPU ")]; !got.Equal(base.Add(-time.Hour)) {
		t.Fatalf("expected normalized vm-2 hourly coverage at %s, got %s", base.Add(-time.Hour), got)
	}

	rawCoverage, err := store.MaxTimestampsForTier(TierRaw)
	if err != nil {
		t.Fatalf("failed to read raw coverage: %v", err)
	}
	if len(rawCoverage) != 1 {
		t.Fatalf("expected 1 raw series, got %d: %v", len(rawCoverage), rawCoverage)
	}
	if got := rawCoverage[NormalizedSeriesKey("vm", "vm-1", "cpu")]; !got.Equal(base.Add(time.Hour)) {
		t.Fatalf("expected vm-1 raw coverage at %s, got %s", base.Add(time.Hour), got)
	}
}
