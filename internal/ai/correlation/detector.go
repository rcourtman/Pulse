// Package correlation detects relationships between resources.
// It tracks when events on one resource are followed by events on another,
// enabling the AI to understand dependencies and predict cascade failures.
package correlation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EventType represents the type of event being correlated
type EventType string

const (
	EventAlert     EventType = "alert"     // Alert triggered
	EventRestart   EventType = "restart"   // Resource restarted
	EventHighCPU   EventType = "high_cpu"  // CPU spike
	EventHighMem   EventType = "high_mem"  // Memory spike
	EventDiskFull  EventType = "disk_full" // Disk space critical
	EventOffline   EventType = "offline"   // Resource went offline
	EventMigration EventType = "migration" // Resource migrated
)

// Event represents a tracked event for correlation analysis
type Event struct {
	ID           string    `json:"id"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name,omitempty"`
	ResourceType string    `json:"resource_type,omitempty"` // node, vm, container, storage
	EventType    EventType `json:"event_type"`
	Timestamp    time.Time `json:"timestamp"`
	Value        float64   `json:"value,omitempty"` // Metric value if applicable
}

// Correlation represents a detected relationship between two resources
type Correlation struct {
	SourceID     string        `json:"source_id"` // Resource that triggers
	SourceName   string        `json:"source_name"`
	SourceType   string        `json:"source_type"`
	TargetID     string        `json:"target_id"` // Resource that follows
	TargetName   string        `json:"target_name"`
	TargetType   string        `json:"target_type"`
	EventPattern string        `json:"event_pattern"` // e.g., "high_mem -> restart"
	Occurrences  int           `json:"occurrences"`   // Number of times observed
	AvgDelay     time.Duration `json:"avg_delay"`     // Average time between events
	Confidence   float64       `json:"confidence"`    // 0-1 confidence level
	LastSeen     time.Time     `json:"last_seen"`
	Description  string        `json:"description"`
}

// Detector tracks events and detects correlations between resources
type Detector struct {
	mu           sync.RWMutex
	events       []Event
	correlations map[string]*Correlation // key: sourceID:targetID:pattern

	// Configuration
	maxEvents         int
	correlationWindow time.Duration // How long after source event to look for target
	minOccurrences    int           // Minimum co-occurrences to form correlation
	retentionWindow   time.Duration // How long to keep events

	// Persistence
	dataDir string
}

// Config configures the correlation detector
type Config struct {
	MaxEvents         int
	CorrelationWindow time.Duration // Default: 10 minutes
	MinOccurrences    int           // Default: 3
	RetentionWindow   time.Duration // Default: 30 days
	DataDir           string
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		MaxEvents:         10000,
		CorrelationWindow: 10 * time.Minute,
		MinOccurrences:    3,
		RetentionWindow:   30 * 24 * time.Hour,
	}
}

// NewDetector creates a new correlation detector
func NewDetector(cfg Config) *Detector {
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 10000
	}
	if cfg.CorrelationWindow <= 0 {
		cfg.CorrelationWindow = 10 * time.Minute
	}
	if cfg.MinOccurrences <= 0 {
		cfg.MinOccurrences = 3
	}
	if cfg.RetentionWindow <= 0 {
		cfg.RetentionWindow = 30 * 24 * time.Hour
	}

	d := &Detector{
		events:            make([]Event, 0),
		correlations:      make(map[string]*Correlation),
		maxEvents:         cfg.MaxEvents,
		correlationWindow: cfg.CorrelationWindow,
		minOccurrences:    cfg.MinOccurrences,
		retentionWindow:   cfg.RetentionWindow,
		dataDir:           cfg.DataDir,
	}

	// Load existing data
	if cfg.DataDir != "" {
		if err := d.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load correlation data from disk")
		} else if len(d.events) > 0 {
			log.Info().Int("events", len(d.events)).Int("correlations", len(d.correlations)).
				Msg("Loaded correlation data from disk")
		}
	}

	return d
}

// RecordEvent records a new event for correlation analysis
func (d *Detector) RecordEvent(event Event) {
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

	// Check for correlations with recent events on OTHER resources
	d.detectCorrelations(event)

	// Persist asynchronously
	go func() {
		if err := d.saveToDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to save correlation data")
		}
	}()
}

// detectCorrelations looks for patterns where this event follows a recent event on another resource
func (d *Detector) detectCorrelations(newEvent Event) {
	cutoff := newEvent.Timestamp.Add(-d.correlationWindow)

	for _, oldEvent := range d.events {
		// Skip same resource
		if oldEvent.ResourceID == newEvent.ResourceID {
			continue
		}
		// Skip events outside the correlation window
		if oldEvent.Timestamp.Before(cutoff) || oldEvent.Timestamp.After(newEvent.Timestamp) {
			continue
		}

		// Found a potential correlation: oldEvent -> newEvent
		key := correlationKey(oldEvent.ResourceID, newEvent.ResourceID, oldEvent.EventType, newEvent.EventType)
		pattern := string(oldEvent.EventType) + " -> " + string(newEvent.EventType)
		delay := newEvent.Timestamp.Sub(oldEvent.Timestamp)

		if existing, ok := d.correlations[key]; ok {
			// Update existing correlation
			existing.Occurrences++
			existing.AvgDelay = (existing.AvgDelay*time.Duration(existing.Occurrences-1) + delay) / time.Duration(existing.Occurrences)
			existing.LastSeen = newEvent.Timestamp
			existing.Confidence = d.calculateConfidence(existing.Occurrences)
			existing.Description = d.formatCorrelationDescription(existing)
		} else {
			// Create new correlation
			d.correlations[key] = &Correlation{
				SourceID:     oldEvent.ResourceID,
				SourceName:   oldEvent.ResourceName,
				SourceType:   oldEvent.ResourceType,
				TargetID:     newEvent.ResourceID,
				TargetName:   newEvent.ResourceName,
				TargetType:   newEvent.ResourceType,
				EventPattern: pattern,
				Occurrences:  1,
				AvgDelay:     delay,
				Confidence:   0.1, // Low initial confidence
				LastSeen:     newEvent.Timestamp,
			}
		}
	}
}

// calculateConfidence returns confidence based on occurrence count
func (d *Detector) calculateConfidence(occurrences int) float64 {
	// Confidence grows with occurrences, capped at 0.95
	if occurrences < d.minOccurrences {
		return float64(occurrences) * 0.1
	}
	// Logarithmic growth after threshold
	confidence := 0.3 + 0.1*float64(occurrences-d.minOccurrences)
	if confidence > 0.95 {
		confidence = 0.95
	}
	return confidence
}

// formatCorrelationDescription creates a human-readable description
func (d *Detector) formatCorrelationDescription(c *Correlation) string {
	sourceName := c.SourceName
	if sourceName == "" {
		sourceName = c.SourceID
	}
	targetName := c.TargetName
	if targetName == "" {
		targetName = c.TargetID
	}

	delayStr := formatDuration(c.AvgDelay)
	sourceEvent := c.EventPattern
	if parts := strings.Split(c.EventPattern, " -> "); len(parts) == 2 {
		sourceEvent = parts[0]
	}

	return "When " + sourceName + " experiences " + sourceEvent +
		", " + targetName + " often follows within " + delayStr
}

// GetCorrelations returns all detected correlations above minimum confidence
func (d *Detector) GetCorrelations() []*Correlation {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*Correlation
	for _, c := range d.correlations {
		if c.Occurrences >= d.minOccurrences && c.Confidence >= 0.3 {
			result = append(result, c)
		}
	}

	// Sort by confidence descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Confidence > result[j].Confidence
	})

	return result
}

// GetCorrelationsForResource returns correlations involving a specific resource
func (d *Detector) GetCorrelationsForResource(resourceID string) []*Correlation {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*Correlation
	for _, c := range d.correlations {
		if (c.SourceID == resourceID || c.TargetID == resourceID) && c.Occurrences >= d.minOccurrences {
			result = append(result, c)
		}
	}

	return result
}

// GetDependencies returns resources that depend on the given resource
// (resources that experience events after this resource has an event)
func (d *Detector) GetDependencies(resourceID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	deps := make(map[string]bool)
	for _, c := range d.correlations {
		if c.SourceID == resourceID && c.Occurrences >= d.minOccurrences {
			deps[c.TargetID] = true
		}
	}

	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result
}

// GetDependsOn returns resources that the given resource depends on
// (resources whose events precede events on this resource)
func (d *Detector) GetDependsOn(resourceID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	deps := make(map[string]bool)
	for _, c := range d.correlations {
		if c.TargetID == resourceID && c.Occurrences >= d.minOccurrences {
			deps[c.SourceID] = true
		}
	}

	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result
}

// PredictCascade predicts what resources might be affected if the given resource has an issue
func (d *Detector) PredictCascade(resourceID string, eventType EventType) []CascadePrediction {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var predictions []CascadePrediction

	for _, c := range d.correlations {
		if c.SourceID == resourceID && c.Occurrences >= d.minOccurrences {
			// Check if the correlation's source event matches the given event type.
			sourceEvent := c.EventPattern
			if parts := strings.Split(c.EventPattern, " -> "); len(parts) == 2 {
				sourceEvent = parts[0]
			}
			if EventType(sourceEvent) == eventType {
				predictions = append(predictions, CascadePrediction{
					ResourceID:   c.TargetID,
					ResourceName: c.TargetName,
					EventPattern: c.EventPattern,
					ExpectedIn:   c.AvgDelay,
					Confidence:   c.Confidence,
				})
			}
		}
	}

	// Sort by confidence
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Confidence > predictions[j].Confidence
	})

	return predictions
}

// CascadePrediction represents a predicted downstream effect
type CascadePrediction struct {
	ResourceID   string        `json:"resource_id"`
	ResourceName string        `json:"resource_name"`
	EventPattern string        `json:"event_pattern"`
	ExpectedIn   time.Duration `json:"expected_in"`
	Confidence   float64       `json:"confidence"`
}

// FormatForContext formats correlations for AI consumption
func (d *Detector) FormatForContext(resourceID string) string {
	var correlations []*Correlation
	if resourceID != "" {
		correlations = d.GetCorrelationsForResource(resourceID)
	} else {
		correlations = d.GetCorrelations()
	}

	if len(correlations) == 0 {
		return ""
	}

	var result string
	result = "\n## Resource Correlations\n"
	result += "Observed relationships between resources:\n"

	for i, c := range correlations {
		if i >= 10 { // Limit to 10 correlations
			result += "\n... and more\n"
			break
		}
		if c.Description != "" {
			result += "- " + c.Description + "\n"
		} else {
			result += "- " + c.EventPattern + " (" + formatConfidence(c.Confidence) + " confidence)\n"
		}
	}

	return result
}

// trimEvents removes old events
func (d *Detector) trimEvents() {
	// Remove events beyond maxEvents
	if len(d.events) > d.maxEvents {
		d.events = d.events[len(d.events)-d.maxEvents:]
	}

	// Remove events older than retention window
	cutoff := time.Now().Add(-d.retentionWindow)
	kept := make([]Event, 0, len(d.events))
	for _, e := range d.events {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		}
	}
	d.events = kept
}

// saveToDisk persists data
func (d *Detector) saveToDisk() error {
	if d.dataDir == "" {
		return nil
	}

	d.mu.RLock()
	data := struct {
		Events       []Event                 `json:"events"`
		Correlations map[string]*Correlation `json:"correlations"`
	}{
		Events:       d.events,
		Correlations: d.correlations,
	}
	d.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(d.dataDir, "ai_correlations.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads data
func (d *Detector) loadFromDisk() error {
	if d.dataDir == "" {
		return nil
	}

	path := filepath.Join(d.dataDir, "ai_correlations.json")
	if st, err := os.Stat(path); err == nil {
		const maxOnDiskBytes = 10 << 20 // 10 MiB safety cap
		if st.Size() > maxOnDiskBytes {
			return fmt.Errorf("correlation history file too large (%d bytes)", st.Size())
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
		Events       []Event                 `json:"events"`
		Correlations map[string]*Correlation `json:"correlations"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	d.events = data.Events
	d.correlations = data.Correlations
	if d.correlations == nil {
		d.correlations = make(map[string]*Correlation)
	}

	d.trimEvents()
	cutoff := time.Now().Add(-d.retentionWindow)
	for k, v := range d.correlations {
		if v == nil || v.LastSeen.Before(cutoff) {
			delete(d.correlations, k)
		}
	}

	return nil
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

func correlationKey(sourceID, targetID string, sourceType, targetType EventType) string {
	return sourceID + ":" + targetID + ":" + string(sourceType) + ":" + string(targetType)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "seconds"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return intToStr(mins) + " minutes"
	}
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour"
	}
	return intToStr(hours) + " hours"
}

func formatConfidence(c float64) string {
	pct := int(c * 100)
	return intToStr(pct) + "%"
}
