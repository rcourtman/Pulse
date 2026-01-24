// Package correlation provides root-cause correlation capabilities.
// This file implements the root-cause correlation engine that identifies
// the underlying cause of related issues across multiple resources.
package correlation

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// RelationshipType describes how resources are related
type RelationshipType string

const (
	RelationshipRunsOn      RelationshipType = "runs_on"      // VM runs on Node
	RelationshipUsesStorage RelationshipType = "uses_storage" // VM uses Storage
	RelationshipUsesNetwork RelationshipType = "uses_network" // Guest uses Network
	RelationshipContains    RelationshipType = "contains"     // Node contains VMs
	RelationshipBackedBy    RelationshipType = "backed_by"    // Storage backed by disks
	RelationshipHosted      RelationshipType = "hosted"       // Container hosted on Docker
	RelationshipDepends     RelationshipType = "depends_on"   // Generic dependency
)

// ResourceRelationship describes a relationship between two resources
type ResourceRelationship struct {
	SourceID     string           `json:"source_id"`
	SourceType   string           `json:"source_type"`
	TargetID     string           `json:"target_id"`
	TargetType   string           `json:"target_type"`
	Relationship RelationshipType `json:"relationship"`
}

// RelatedEvent represents an event on a related resource
type RelatedEvent struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name,omitempty"`
	ResourceType string    `json:"resource_type"`
	EventType    string    `json:"event_type"` // "alert", "anomaly", "metric_spike", etc.
	Metric       string    `json:"metric,omitempty"`
	Value        float64   `json:"value,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Description  string    `json:"description"`
}

// RootCauseAnalysis represents the result of a root-cause analysis
type RootCauseAnalysis struct {
	ID            string         `json:"id"`
	TriggerEvent  RelatedEvent   `json:"trigger_event"`
	RootCause     *RelatedEvent  `json:"root_cause,omitempty"`
	RelatedEvents []RelatedEvent `json:"related_events"`
	CausalChain   []string       `json:"causal_chain"` // Explanation of causation
	Confidence    float64        `json:"confidence"`
	Explanation   string         `json:"explanation"`
	Timestamp     time.Time      `json:"timestamp"`
}

// TopologyProvider provides resource relationship information
type TopologyProvider interface {
	// GetRelationships returns all relationships for a resource
	GetRelationships(resourceID string) []ResourceRelationship
	// GetResourceType returns the type of a resource
	GetResourceType(resourceID string) string
	// GetResourceName returns the name of a resource
	GetResourceName(resourceID string) string
}

// EventProvider provides recent events for resources
type EventProvider interface {
	// GetRecentEvents returns events for a resource within the time window
	GetRecentEvents(resourceID string, window time.Duration) []RelatedEvent
}

// RootCauseEngineConfig configures the root cause engine
type RootCauseEngineConfig struct {
	CorrelationWindow time.Duration // How far to look back for related events
	MaxChainLength    int           // Maximum length of causal chain
	MinConfidence     float64       // Minimum confidence to report
}

// DefaultRootCauseEngineConfig returns sensible defaults
func DefaultRootCauseEngineConfig() RootCauseEngineConfig {
	return RootCauseEngineConfig{
		CorrelationWindow: 5 * time.Minute,
		MaxChainLength:    5,
		MinConfidence:     0.5,
	}
}

// RootCauseEngine identifies root causes across related resources
type RootCauseEngine struct {
	mu sync.RWMutex

	config           RootCauseEngineConfig
	topologyProvider TopologyProvider
	eventProvider    EventProvider

	// Cache of recent analyses
	recentAnalyses []RootCauseAnalysis
	maxAnalyses    int
}

// NewRootCauseEngine creates a new root cause engine
func NewRootCauseEngine(cfg RootCauseEngineConfig) *RootCauseEngine {
	if cfg.CorrelationWindow <= 0 {
		cfg.CorrelationWindow = 5 * time.Minute
	}
	if cfg.MaxChainLength <= 0 {
		cfg.MaxChainLength = 5
	}
	if cfg.MinConfidence <= 0 {
		cfg.MinConfidence = 0.5
	}

	return &RootCauseEngine{
		config:         cfg,
		recentAnalyses: make([]RootCauseAnalysis, 0),
		maxAnalyses:    100,
	}
}

// SetTopologyProvider sets the topology provider
func (e *RootCauseEngine) SetTopologyProvider(provider TopologyProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.topologyProvider = provider
}

// SetEventProvider sets the event provider
func (e *RootCauseEngine) SetEventProvider(provider EventProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventProvider = provider
}

// Analyze performs root-cause analysis for an event
func (e *RootCauseEngine) Analyze(triggerEvent RelatedEvent) *RootCauseAnalysis {
	e.mu.RLock()
	topology := e.topologyProvider
	events := e.eventProvider
	config := e.config
	e.mu.RUnlock()

	if topology == nil || events == nil {
		return nil
	}

	analysis := &RootCauseAnalysis{
		ID:           generateAnalysisID(),
		TriggerEvent: triggerEvent,
		Timestamp:    time.Now(),
	}

	// Get related resources
	relationships := topology.GetRelationships(triggerEvent.ResourceID)

	// Collect events from related resources
	relatedEvents := make([]RelatedEvent, 0)
	for _, rel := range relationships {
		targetID := rel.TargetID
		if targetID == triggerEvent.ResourceID {
			targetID = rel.SourceID
		}

		resourceEvents := events.GetRecentEvents(targetID, config.CorrelationWindow)
		for _, evt := range resourceEvents {
			// Only include events that happened before the trigger
			if evt.Timestamp.Before(triggerEvent.Timestamp) {
				evt.ResourceName = topology.GetResourceName(evt.ResourceID)
				evt.ResourceType = topology.GetResourceType(evt.ResourceID)
				relatedEvents = append(relatedEvents, evt)
			}
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(relatedEvents, func(i, j int) bool {
		return relatedEvents[i].Timestamp.Before(relatedEvents[j].Timestamp)
	})

	analysis.RelatedEvents = relatedEvents

	// Find potential root cause
	if len(relatedEvents) > 0 {
		rootCause := e.identifyRootCause(triggerEvent, relatedEvents, relationships)
		if rootCause != nil {
			analysis.RootCause = rootCause
			analysis.CausalChain = e.buildCausalChain(rootCause, triggerEvent, relatedEvents, relationships)
			analysis.Confidence = e.calculateConfidence(analysis)
			analysis.Explanation = e.generateExplanation(analysis)
		}
	}

	// Store analysis
	e.mu.Lock()
	e.recentAnalyses = append(e.recentAnalyses, *analysis)
	if len(e.recentAnalyses) > e.maxAnalyses {
		e.recentAnalyses = e.recentAnalyses[len(e.recentAnalyses)-e.maxAnalyses:]
	}
	e.mu.Unlock()

	return analysis
}

// identifyRootCause finds the most likely root cause
func (e *RootCauseEngine) identifyRootCause(trigger RelatedEvent, related []RelatedEvent, relationships []ResourceRelationship) *RelatedEvent {
	if len(related) == 0 {
		return nil
	}

	// Score each related event as a potential root cause
	type scoredEvent struct {
		event *RelatedEvent
		score float64
	}

	var candidates []scoredEvent

	for i := range related {
		event := &related[i]
		score := e.scoreAsRootCause(event, trigger, relationships)
		if score > 0 {
			candidates = append(candidates, scoredEvent{event, score})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	return candidates[0].event
}

// scoreAsRootCause scores how likely an event is the root cause
func (e *RootCauseEngine) scoreAsRootCause(candidate *RelatedEvent, trigger RelatedEvent, relationships []ResourceRelationship) float64 {
	score := 0.0

	// Check if there's a direct relationship
	for _, rel := range relationships {
		if rel.SourceID == candidate.ResourceID || rel.TargetID == candidate.ResourceID {
			// Strong relationship types
			switch rel.Relationship {
			case RelationshipRunsOn:
				score += 0.4 // VM problems often caused by host
			case RelationshipUsesStorage:
				score += 0.35 // Storage issues propagate
			case RelationshipBackedBy:
				score += 0.3
			case RelationshipContains:
				score += 0.25
			default:
				score += 0.2
			}
			break
		}
	}

	// Infrastructure resources (nodes, storage) are more likely root causes
	switch candidate.ResourceType {
	case "node":
		score += 0.25
	case "storage":
		score += 0.2
	case "network":
		score += 0.15
	}

	// Events that happened earlier are more likely causes
	timeDiff := trigger.Timestamp.Sub(candidate.Timestamp)
	if timeDiff < 30*time.Second {
		score += 0.2 // Very close in time
	} else if timeDiff < 1*time.Minute {
		score += 0.15
	} else if timeDiff < 2*time.Minute {
		score += 0.1
	}

	// Similar metric types suggest related issues
	if candidate.Metric != "" && trigger.Metric != "" {
		if isRelatedMetric(candidate.Metric, trigger.Metric) {
			score += 0.15
		}
	}

	return score
}

// buildCausalChain builds a chain of causation from root cause to trigger
func (e *RootCauseEngine) buildCausalChain(rootCause *RelatedEvent, trigger RelatedEvent, related []RelatedEvent, relationships []ResourceRelationship) []string {
	chain := make([]string, 0)

	if rootCause == nil {
		return chain
	}

	// Start with root cause
	chain = append(chain, formatEventForChain(rootCause))

	// Find intermediate events that connect root cause to trigger
	// Simple approach: include events between root cause and trigger times
	for _, event := range related {
		if event.ResourceID == rootCause.ResourceID {
			continue
		}
		if event.Timestamp.After(rootCause.Timestamp) && event.Timestamp.Before(trigger.Timestamp) {
			chain = append(chain, formatEventForChain(&event))
			if len(chain) >= e.config.MaxChainLength-1 {
				break
			}
		}
	}

	// End with trigger event
	chain = append(chain, formatEventForChain(&trigger))

	return chain
}

// calculateConfidence calculates confidence in the analysis
func (e *RootCauseEngine) calculateConfidence(analysis *RootCauseAnalysis) float64 {
	if analysis.RootCause == nil {
		return 0
	}

	confidence := 0.5 // Base confidence

	// More related events = higher confidence
	if len(analysis.RelatedEvents) >= 3 {
		confidence += 0.1
	}
	if len(analysis.RelatedEvents) >= 5 {
		confidence += 0.1
	}

	// Longer causal chain can indicate clearer relationship
	if len(analysis.CausalChain) >= 2 && len(analysis.CausalChain) <= 4 {
		confidence += 0.1
	}

	// Time proximity
	timeDiff := analysis.TriggerEvent.Timestamp.Sub(analysis.RootCause.Timestamp)
	if timeDiff < 1*time.Minute {
		confidence += 0.15
	} else if timeDiff < 2*time.Minute {
		confidence += 0.1
	}

	return minFloat(confidence, 0.95)
}

// generateExplanation creates a human-readable explanation
func (e *RootCauseEngine) generateExplanation(analysis *RootCauseAnalysis) string {
	if analysis.RootCause == nil {
		return "No clear root cause identified"
	}

	rootName := analysis.RootCause.ResourceName
	if rootName == "" {
		rootName = analysis.RootCause.ResourceID
	}

	triggerName := analysis.TriggerEvent.ResourceName
	if triggerName == "" {
		triggerName = analysis.TriggerEvent.ResourceID
	}

	explanation := fmt.Sprintf("%s on %s was likely caused by %s on %s",
		analysis.TriggerEvent.Description,
		triggerName,
		analysis.RootCause.Description,
		rootName)

	timeDiff := analysis.TriggerEvent.Timestamp.Sub(analysis.RootCause.Timestamp)
	if timeDiff < 1*time.Minute {
		explanation += " (occurred within 1 minute)"
	} else {
		minutes := int(timeDiff.Minutes())
		explanation += fmt.Sprintf(" (occurred %d minutes earlier)", minutes)
	}

	return explanation
}

// GetRecentAnalyses returns recent root cause analyses
func (e *RootCauseEngine) GetRecentAnalyses(limit int) []RootCauseAnalysis {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.recentAnalyses) {
		limit = len(e.recentAnalyses)
	}

	// Return most recent
	start := len(e.recentAnalyses) - limit
	if start < 0 {
		start = 0
	}

	result := make([]RootCauseAnalysis, limit)
	copy(result, e.recentAnalyses[start:])
	return result
}

// GetAnalysisForResource returns analyses involving a resource
func (e *RootCauseEngine) GetAnalysisForResource(resourceID string) []RootCauseAnalysis {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []RootCauseAnalysis
	for _, analysis := range e.recentAnalyses {
		if analysis.TriggerEvent.ResourceID == resourceID ||
			(analysis.RootCause != nil && analysis.RootCause.ResourceID == resourceID) {
			result = append(result, analysis)
		}
	}

	return result
}

// FormatForContext formats root cause analysis for AI prompt injection
func (e *RootCauseEngine) FormatForContext(resourceID string) string {
	analyses := e.GetAnalysisForResource(resourceID)
	if len(analyses) == 0 {
		return ""
	}

	result := "\n## Root Cause Analysis\n"
	result += "Identified root causes for recent issues:\n\n"

	for _, analysis := range analyses {
		if analysis.Confidence < e.config.MinConfidence {
			continue
		}

		result += fmt.Sprintf("- %s (%.0f%% confidence)\n",
			analysis.Explanation,
			analysis.Confidence*100)

		if len(analysis.CausalChain) > 0 {
			result += "  Chain: " + strings.Join(analysis.CausalChain, " → ") + "\n"
		}
	}

	return result
}

// FormatAnalysisForPatrol formats analyses for patrol context
func (e *RootCauseEngine) FormatAnalysisForPatrol() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Get analyses from last hour
	cutoff := time.Now().Add(-1 * time.Hour)
	var recentAnalyses []RootCauseAnalysis

	for _, analysis := range e.recentAnalyses {
		if analysis.Timestamp.After(cutoff) && analysis.Confidence >= e.config.MinConfidence {
			recentAnalyses = append(recentAnalyses, analysis)
		}
	}

	if len(recentAnalyses) == 0 {
		return ""
	}

	result := "\n## Root Cause Correlations\n"
	result += "Recent issues with identified root causes:\n\n"

	for _, analysis := range recentAnalyses {
		result += "### " + analysis.TriggerEvent.Description + "\n"
		result += "Root cause: " + analysis.Explanation + "\n"
		result += fmt.Sprintf("Confidence: %.0f%%\n", analysis.Confidence*100)

		if len(analysis.CausalChain) > 0 {
			result += "Causal chain:\n"
			for i, step := range analysis.CausalChain {
				result += fmt.Sprintf("  %d. %s\n", i+1, step)
			}
		}
		result += "\n"
	}

	return result
}

// Helper functions

var analysisCounter int64

func generateAnalysisID() string {
	analysisCounter++
	return fmt.Sprintf("rca-%s-%d", time.Now().Format("20060102150405"), analysisCounter%1000)
}

func formatEventForChain(event *RelatedEvent) string {
	name := event.ResourceName
	if name == "" {
		name = event.ResourceID
	}

	if event.Metric != "" && event.Value > 0 {
		return fmt.Sprintf("%s %s (%.1f)", name, event.Metric, event.Value)
	}
	return fmt.Sprintf("%s: %s", name, event.Description)
}

func isRelatedMetric(metric1, metric2 string) bool {
	// Group related metrics
	ioMetrics := map[string]bool{"io": true, "disk": true, "storage": true, "latency": true}
	cpuMetrics := map[string]bool{"cpu": true, "load": true, "iowait": true}
	memMetrics := map[string]bool{"memory": true, "mem": true, "swap": true}
	netMetrics := map[string]bool{"network": true, "net": true, "bandwidth": true}

	m1Lower := strings.ToLower(metric1)
	m2Lower := strings.ToLower(metric2)

	if ioMetrics[m1Lower] && ioMetrics[m2Lower] {
		return true
	}
	if cpuMetrics[m1Lower] && cpuMetrics[m2Lower] {
		return true
	}
	if memMetrics[m1Lower] && memMetrics[m2Lower] {
		return true
	}
	if netMetrics[m1Lower] && netMetrics[m2Lower] {
		return true
	}

	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
