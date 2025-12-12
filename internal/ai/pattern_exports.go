package ai

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/patterns"
)

// PatternDetector is an alias for patterns.Detector
type PatternDetector = patterns.Detector

// PatternDetectorConfig is an alias for patterns.DetectorConfig
type PatternDetectorConfig = patterns.DetectorConfig

// HistoricalEvent is an alias for patterns.HistoricalEvent
type HistoricalEvent = patterns.HistoricalEvent

// FailurePrediction is an alias for patterns.FailurePrediction
type FailurePrediction = patterns.FailurePrediction

// EventType is an alias for patterns.EventType
type EventType = patterns.EventType

// Pattern is an alias for patterns.Pattern
type Pattern = patterns.Pattern

// Event type constants
const (
	EventHighMemory   = patterns.EventHighMemory
	EventHighCPU      = patterns.EventHighCPU
	EventDiskFull     = patterns.EventDiskFull
	EventOOM          = patterns.EventOOM
	EventRestart      = patterns.EventRestart
	EventUnresponsive = patterns.EventUnresponsive
	EventBackupFailed = patterns.EventBackupFailed
)

// NewPatternDetector creates a new pattern detector
func NewPatternDetector(cfg PatternDetectorConfig) *PatternDetector {
	return patterns.NewDetector(cfg)
}

// DefaultPatternConfig returns default pattern detector configuration
func DefaultPatternConfig() PatternDetectorConfig {
	return patterns.DefaultConfig()
}
