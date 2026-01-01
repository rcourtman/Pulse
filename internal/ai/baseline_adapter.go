package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
)

// BaselineStoreAdapter adapts baseline.Store to the context.BaselineProvider interface
type BaselineStoreAdapter struct {
	store *baseline.Store
}

// NewBaselineStoreAdapter creates an adapter for baseline.Store
func NewBaselineStoreAdapter(store *baseline.Store) *BaselineStoreAdapter {
	if store == nil {
		return nil
	}
	return &BaselineStoreAdapter{store: store}
}

// CheckAnomaly implements context.BaselineProvider
func (a *BaselineStoreAdapter) CheckAnomaly(resourceID, metric string, value float64) (severity string, zScore float64, mean float64, stddev float64, ok bool) {
	if a.store == nil {
		return "", 0, 0, 0, false
	}

	s, z, b := a.store.CheckAnomaly(resourceID, metric, value)
	if b == nil {
		return "", 0, 0, 0, false
	}

	return string(s), z, b.Mean, b.StdDev, true
}

// GetBaseline implements context.BaselineProvider
func (a *BaselineStoreAdapter) GetBaseline(resourceID, metric string) (mean float64, stddev float64, sampleCount int, ok bool) {
	if a.store == nil {
		return 0, 0, 0, false
	}

	b, exists := a.store.GetBaseline(resourceID, metric)
	if !exists || b == nil {
		return 0, 0, 0, false
	}

	return b.Mean, b.StdDev, b.SampleCount, true
}
