package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
)

// BaselineConfig is an alias for the baseline package config
type BaselineConfig = baseline.StoreConfig

// BaselineStore is an alias for the baseline.Store type
type BaselineStore = baseline.Store

// BaselineMetricPoint is an alias for the baseline.MetricPoint type
type BaselineMetricPoint = baseline.MetricPoint

// DefaultBaselineConfig returns the default baseline configuration
func DefaultBaselineConfig() BaselineConfig {
	return baseline.DefaultConfig()
}

// NewBaselineStore creates a new baseline store
func NewBaselineStore(cfg BaselineConfig) *BaselineStore {
	return baseline.NewStore(cfg)
}
