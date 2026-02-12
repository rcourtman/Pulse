// Package patterns provides failure pattern detection for predictive intelligence.
// It analyzes historical data to identify recurring issues and predict future failures.
package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EventType represents the type of event being tracked
type EventType string

const (
	EventHighMemory   EventType = "high_memory"   // Memory exceeded threshold
	EventHighCPU      EventType = "high_cpu"      // CPU exceeded threshold
	EventDiskFull     EventType = "disk_full"     // Disk space critical
	EventOOM          EventType = "oom"           // Out of memory kill
	EventRestart      EventType = "restart"       // Resource restarted
	EventUnresponsive EventType = "unresponsive"  // Resource became unresponsive
	EventBackupFailed EventType = "backup_failed" // Backup job failed
)

// HistoricalEvent represents a recorded event
type HistoricalEvent struct {
	ID          string        `json:"id"`
	ResourceID  string        `json:"resource_id"`
	EventType   EventType     `json:"event_type"`
	Timestamp   time.Time     `json:"timestamp"`
	Description string        `json:"description,omitempty"`
	Resolved    bool          `json:"resolved"`
	ResolvedAt  time.Time     `json:"resolved_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"` // How long it lasted
}

// Pattern represents a detected recurring pattern
type Pattern struct {
	ResourceID      string        `json:"resource_id"`
	EventType       EventType     `json:"event_type"`
	Occurrences     int           `json:"occurrences"`      // Number of times event occurred
	AverageInterval time.Duration `json:"average_interval"` // Average time between occurrences
	StdDevInterval  time.Duration `json:"stddev_interval"`  // Standard deviation
	LastOccurrence  time.Time     `json:"last_occurrence"`
	NextPredicted   time.Time     `json:"next_predicted"`             // When we expect it to happen again
	Confidence      float64       `json:"confidence"`                 // 0-1, based on consistency
	AverageDuration time.Duration `json:"average_duration,omitempty"` // How long events typically last
}

// FailurePrediction represents a predicted future failure
type FailurePrediction struct {
	ResourceID  string    `json:"resource_id"`
	EventType   EventType `json:"event_type"`
	PredictedAt time.Time `json:"predicted_at"`
	DaysUntil   float64   `json:"days_until"`
	Confidence  float64   `json:"confidence"`
	Basis       string    `json:"basis"` // Human-readable explanation
	Pattern     *Pattern  `json:"pattern,omitempty"`
}

// Detector tracks historical events and detects patterns
type Detector struct {
	mu       sync.RWMutex
	events   []HistoricalEvent
	patterns map[string]*Pattern // resourceID:eventType -> pattern

	// Configuration
	maxEvents       int
	minOccurrences  int           // Minimum occurrences to form a pattern
	patternWindow   time.Duration // How far back to look for patterns
	predictionLimit time.Duration // How far ahead to predict

	// Persistence
	dataDir string
}

// DetectorConfig configures the pattern detector
type DetectorConfig struct {
	MaxEvents       int
	MinOccurrences  int           // Default: 3
	PatternWindow   time.Duration // Default: 90 days
	PredictionLimit time.Duration // Default: 30 days
	DataDir         string
}

// DefaultConfig returns default detector configuration
func DefaultConfig() DetectorConfig {
	return DetectorConfig{
		MaxEvents:       5000,
		MinOccurrences:  3,
		PatternWindow:   90 * 24 * time.Hour,
		PredictionLimit: 30 * 24 * time.Hour,
	}
}

// NewDetector creates a new pattern detector
func NewDetector(cfg DetectorConfig) *Detector {
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 5000
	}
	if cfg.MinOccurrences <= 0 {
		cfg.MinOccurrences = 3
	}
	if cfg.PatternWindow <= 0 {
		cfg.PatternWindow = 90 * 24 * time.Hour
	}
	if cfg.PredictionLimit <= 0 {
		cfg.PredictionLimit = 30 * 24 * time.Hour
	}

	d := &Detector{
		events:          make([]HistoricalEvent, 0),
		patterns:        make(map[string]*Pattern),
		maxEvents:       cfg.MaxEvents,
		minOccurrences:  cfg.MinOccurrences,
		patternWindow:   cfg.PatternWindow,
		predictionLimit: cfg.PredictionLimit,
		dataDir:         cfg.DataDir,
	}

	// Load existing data
	if cfg.DataDir != "" {
		if err := d.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to load pattern history from disk")
		} else if len(d.events) > 0 {
			log.Info().Int("events", len(d.events)).Int("patterns", len(d.patterns)).
				Msg("Loaded pattern history from disk")
		}
	}

	return d
}

// RecordEvent records a new event for pattern analysis
func (d *Detector) RecordEvent(event HistoricalEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	d.events = append(d.events, event)
	d.trimEvents()

	// Recompute pattern for this resource/event type
	key := patternKey(event.ResourceID, event.EventType)
	pattern := d.computePattern(event.ResourceID, event.EventType)
	if pattern == nil {
		delete(d.patterns, key)
	} else {
		d.patterns[key] = pattern
	}

	// Persist asynchronously
	go func() {
		if err := d.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("failed to save pattern history")
		}
	}()
}

// RecordFromAlert records an event from an alert
func (d *Detector) RecordFromAlert(resourceID string, alertType string, timestamp time.Time) {
	eventType := mapAlertToEventType(alertType)
	if eventType == "" {
		return // Not a trackable event type
	}

	d.RecordEvent(HistoricalEvent{
		ResourceID:  resourceID,
		EventType:   eventType,
		Timestamp:   timestamp,
		Description: alertType,
	})
}

// GetPredictions returns failure predictions for all tracked resources
func (d *Detector) GetPredictions() []FailurePrediction {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var predictions []FailurePrediction
	now := time.Now()

	for _, pattern := range d.patterns {
		if pattern == nil {
			continue
		}
		// Only predict if pattern has sufficient confidence
		if pattern.Confidence < 0.3 || pattern.Occurrences < d.minOccurrences {
			continue
		}

		// Check if prediction is within our limit
		if pattern.NextPredicted.Before(now) || pattern.NextPredicted.After(now.Add(d.predictionLimit)) {
			continue
		}

		daysUntil := pattern.NextPredicted.Sub(now).Hours() / 24

		predictions = append(predictions, FailurePrediction{
			ResourceID:  pattern.ResourceID,
			EventType:   pattern.EventType,
			PredictedAt: pattern.NextPredicted,
			DaysUntil:   daysUntil,
			Confidence:  pattern.Confidence,
			Basis:       formatPatternBasis(pattern),
			Pattern:     pattern,
		})
	}

	// Sort by days until (soonest first)
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].DaysUntil < predictions[j].DaysUntil
	})

	return predictions
}

// GetPredictionsForResource returns failure predictions for a specific resource
func (d *Detector) GetPredictionsForResource(resourceID string) []FailurePrediction {
	all := d.GetPredictions()
	var result []FailurePrediction
	for _, p := range all {
		if p.ResourceID == resourceID {
			result = append(result, p)
		}
	}
	return result
}

// GetPatterns returns all detected patterns
func (d *Detector) GetPatterns() map[string]*Pattern {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]*Pattern)
	for k, v := range d.patterns {
		if v == nil {
			continue
		}
		result[k] = v
	}
	return result
}

// computePattern analyzes events to find patterns for a resource/event type
func (d *Detector) computePattern(resourceID string, eventType EventType) *Pattern {
	cutoff := time.Now().Add(-d.patternWindow)

	// Get all events for this resource/type within the window
	var events []HistoricalEvent
	for _, e := range d.events {
		if e.ResourceID == resourceID && e.EventType == eventType && e.Timestamp.After(cutoff) {
			events = append(events, e)
		}
	}

	if len(events) < d.minOccurrences {
		return nil
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Calculate intervals between events
	var intervals []time.Duration
	var durations []time.Duration

	for i := 1; i < len(events); i++ {
		interval := events[i].Timestamp.Sub(events[i-1].Timestamp)
		intervals = append(intervals, interval)

		if events[i-1].Duration > 0 {
			durations = append(durations, events[i-1].Duration)
		}
	}

	if len(intervals) == 0 {
		return nil
	}

	// Calculate average and stddev of intervals
	avgInterval := averageDuration(intervals)
	stddevInterval := stddevDuration(intervals, avgInterval)

	// Calculate confidence based on consistency
	// If stddev is low relative to mean, pattern is more reliable
	consistency := 1.0
	if avgInterval > 0 {
		cv := float64(stddevInterval) / float64(avgInterval) // Coefficient of variation
		consistency = 1.0 - math.Min(cv, 1.0)                // Higher consistency = lower CV
	}

	// Adjust confidence based on number of occurrences
	occurrenceBonus := math.Min(float64(len(events))/10.0, 0.3)
	confidence := consistency*0.7 + occurrenceBonus

	// Predict next occurrence
	lastEvent := events[len(events)-1]
	nextPredicted := lastEvent.Timestamp.Add(avgInterval)

	// Calculate average duration if available
	var avgDuration time.Duration
	if len(durations) > 0 {
		avgDuration = averageDuration(durations)
	}

	return &Pattern{
		ResourceID:      resourceID,
		EventType:       eventType,
		Occurrences:     len(events),
		AverageInterval: avgInterval,
		StdDevInterval:  stddevInterval,
		LastOccurrence:  lastEvent.Timestamp,
		NextPredicted:   nextPredicted,
		Confidence:      confidence,
		AverageDuration: avgDuration,
	}
}

// trimEvents removes old events beyond maxEvents
func (d *Detector) trimEvents() {
	cutoff := time.Now().Add(-d.patternWindow)
	kept := d.events[:0]
	for _, e := range d.events {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		}
	}
	d.events = kept

	if len(d.events) > d.maxEvents {
		d.events = d.events[len(d.events)-d.maxEvents:]
	}
}

// saveToDisk persists events and patterns
func (d *Detector) saveToDisk() error {
	if d.dataDir == "" {
		return nil
	}

	d.mu.RLock()
	data := struct {
		Events   []HistoricalEvent   `json:"events"`
		Patterns map[string]*Pattern `json:"patterns"`
	}{
		Events:   d.events,
		Patterns: d.patterns,
	}
	d.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(d.dataDir, "ai_patterns.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads events and patterns
func (d *Detector) loadFromDisk() error {
	if d.dataDir == "" {
		return nil
	}

	path := filepath.Join(d.dataDir, "ai_patterns.json")
	if st, err := os.Stat(path); err == nil {
		const maxOnDiskBytes = 10 << 20 // 10 MiB safety cap
		if st.Size() > maxOnDiskBytes {
			return fmt.Errorf("pattern history file too large (%d bytes)", st.Size())
		}
	}
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data struct {
		Events   []HistoricalEvent   `json:"events"`
		Patterns map[string]*Pattern `json:"patterns"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	d.events = data.Events
	d.patterns = make(map[string]*Pattern, len(data.Patterns))
	for k, v := range data.Patterns {
		d.patterns[k] = v
	}

	d.trimEvents()
	cutoff := time.Now().Add(-d.patternWindow)
	for k, v := range d.patterns {
		if v == nil {
			delete(d.patterns, k)
			continue
		}
		if v.Occurrences < d.minOccurrences || v.LastOccurrence.Before(cutoff) {
			delete(d.patterns, k)
		}
	}

	return nil
}

// FormatForContext formats predictions for AI consumption
func (d *Detector) FormatForContext(resourceID string) string {
	var predictions []FailurePrediction
	if resourceID != "" {
		predictions = d.GetPredictionsForResource(resourceID)
	} else {
		predictions = d.GetPredictions()
	}

	if len(predictions) == 0 {
		return ""
	}

	var result string
	result = "\n## ⏰ Failure Predictions\n"
	result += "Based on historical patterns:\n"

	for _, p := range predictions {
		if len(result) > 2000 { // Limit context size
			result += "\n... and more\n"
			break
		}
		result += "- " + p.Basis + "\n"
	}

	return result
}

// Helper functions

var eventCounter int64

func generateEventID() string {
	eventCounter++
	return time.Now().Format("20060102150405") + "-" + intToStr(int(eventCounter%1000))
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func patternKey(resourceID string, eventType EventType) string {
	return resourceID + ":" + string(eventType)
}

func mapAlertToEventType(alertType string) EventType {
	switch alertType {
	case "memory_warning", "memory_critical":
		return EventHighMemory
	case "cpu_warning", "cpu_critical":
		return EventHighCPU
	case "disk_warning", "disk_critical":
		return EventDiskFull
	case "oom", "out_of_memory":
		return EventOOM
	case "restart", "restarted":
		return EventRestart
	case "unresponsive", "unreachable":
		return EventUnresponsive
	case "backup_failed":
		return EventBackupFailed
	default:
		return ""
	}
}

func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum int64
	for _, d := range durations {
		sum += int64(d)
	}
	return time.Duration(sum / int64(len(durations)))
}

func stddevDuration(durations []time.Duration, mean time.Duration) time.Duration {
	if len(durations) < 2 {
		return 0
	}
	var sumSquares float64
	for _, d := range durations {
		diff := float64(d - mean)
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(durations)-1)
	return time.Duration(math.Sqrt(variance))
}

func formatPatternBasis(p *Pattern) string {
	daysInterval := p.AverageInterval.Hours() / 24
	daysSinceLast := time.Since(p.LastOccurrence).Hours() / 24
	daysUntilNext := p.NextPredicted.Sub(time.Now()).Hours() / 24

	eventName := string(p.EventType)
	switch p.EventType {
	case EventHighMemory:
		eventName = "high memory usage"
	case EventHighCPU:
		eventName = "high CPU usage"
	case EventDiskFull:
		eventName = "disk space critical"
	case EventOOM:
		eventName = "OOM events"
	case EventRestart:
		eventName = "restarts"
	case EventUnresponsive:
		eventName = "unresponsive periods"
	case EventBackupFailed:
		eventName = "backup failures"
	}

	if daysUntilNext < 0 {
		return eventName + " typically occurs every ~" + formatDays(daysInterval) +
			" (last: " + formatDays(daysSinceLast) + " ago, overdue)"
	}

	return eventName + " typically occurs every ~" + formatDays(daysInterval) +
		" (next expected in ~" + formatDays(daysUntilNext) + ")"
}

func formatDays(days float64) string {
	if days < 1 {
		hours := days * 24
		if hours < 1 {
			return "less than an hour"
		}
		return intToStr(int(hours)) + " hours"
	}
	if days < 2 {
		return "1 day"
	}
	return intToStr(int(days)) + " days"
}
