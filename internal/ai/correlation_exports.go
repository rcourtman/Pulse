package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/correlation"
)

// CorrelationDetector is an alias for correlation.Detector
type CorrelationDetector = correlation.Detector

// CorrelationConfig is an alias for correlation.Config
type CorrelationConfig = correlation.Config

// CorrelationEvent is an alias for correlation.Event
type CorrelationEvent = correlation.Event

// Correlation is an alias for correlation.Correlation
type Correlation = correlation.Correlation

// CascadePrediction is an alias for correlation.CascadePrediction
type CascadePrediction = correlation.CascadePrediction

// CorrelationEventType is an alias for correlation.EventType
type CorrelationEventType = correlation.EventType

// Event type constants
const (
	CorrelationEventAlert     = correlation.EventAlert
	CorrelationEventRestart   = correlation.EventRestart
	CorrelationEventHighCPU   = correlation.EventHighCPU
	CorrelationEventHighMem   = correlation.EventHighMem
	CorrelationEventDiskFull  = correlation.EventDiskFull
	CorrelationEventOffline   = correlation.EventOffline
	CorrelationEventMigration = correlation.EventMigration
)

// NewCorrelationDetector creates a new correlation detector
func NewCorrelationDetector(cfg CorrelationConfig) *CorrelationDetector {
	return correlation.NewDetector(cfg)
}

// DefaultCorrelationConfig returns default correlation detector configuration
func DefaultCorrelationConfig() CorrelationConfig {
	return correlation.DefaultConfig()
}
