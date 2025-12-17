package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
)

func TestBaselineStoreAdapter(t *testing.T) {
	store := baseline.NewStore(baseline.StoreConfig{
		MinSamples: 1,
	})

	err := store.Learn("node:pve1", "node", "cpu", []baseline.MetricPoint{
		{Value: 10, Timestamp: time.Now().Add(-time.Minute)},
		{Value: 10, Timestamp: time.Now()},
	})
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}

	adapter := NewBaselineStoreAdapter(store)
	if adapter == nil {
		t.Fatalf("expected adapter")
	}

	mean, stddev, samples, ok := adapter.GetBaseline("node:pve1", "cpu")
	if !ok {
		t.Fatalf("expected baseline to exist")
	}
	if mean != 10 || stddev != 0 || samples != 2 {
		t.Fatalf("unexpected baseline: mean=%v stddev=%v samples=%d", mean, stddev, samples)
	}

	severity, z, gotMean, gotStd, ok := adapter.CheckAnomaly("node:pve1", "cpu", 11)
	if !ok {
		t.Fatalf("expected anomaly check ok")
	}
	if severity != "critical" || z <= 0 || gotMean != 10 || gotStd != 0 {
		t.Fatalf("unexpected anomaly: severity=%q z=%v mean=%v stddev=%v", severity, z, gotMean, gotStd)
	}
}

func TestBaselineStoreAdapter_NilStore(t *testing.T) {
	adapter := &BaselineStoreAdapter{}
	if _, _, _, ok := adapter.GetBaseline("r", "m"); ok {
		t.Fatalf("expected ok=false")
	}
	if _, _, _, _, ok := adapter.CheckAnomaly("r", "m", 1); ok {
		t.Fatalf("expected ok=false")
	}
}

