package conversion

import (
	"strings"
	"sync"
	"time"
)

const pipelineStaleThreshold = 5 * time.Minute

var knownConversionEventTypes = []string{
	EventPaywallViewed,
	EventTrialStarted,
	EventUpgradeClicked,
	EventCheckoutCompleted,
}

// HealthStatus describes conversion pipeline health and recent activity.
type HealthStatus struct {
	Status              string           `json:"status"`
	LastEventAgeSeconds float64          `json:"last_event_age_seconds"`
	EventsTotal         int64            `json:"events_total"`
	EventsByType        map[string]int64 `json:"events_by_type"`
	StartedAt           int64            `json:"started_at"`
}

// PipelineHealth tracks conversion pipeline activity for health checks.
type PipelineHealth struct {
	mu            sync.RWMutex
	startedAt     time.Time
	lastEventTime time.Time
	eventsTotal   int64
	eventsByType  map[string]int64
}

// NewPipelineHealth creates a health tracker initialized at current time.
func NewPipelineHealth() *PipelineHealth {
	return &PipelineHealth{
		startedAt:    time.Now(),
		eventsByType: make(map[string]int64),
	}
}

// RecordEvent records an event occurrence for health tracking.
func (p *PipelineHealth) RecordEvent(eventType string) {
	if p == nil {
		return
	}

	normalized := strings.TrimSpace(eventType)
	if normalized == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.startedAt.IsZero() {
		p.startedAt = time.Now()
	}
	if p.eventsByType == nil {
		p.eventsByType = make(map[string]int64)
	}

	p.lastEventTime = time.Now()
	p.eventsTotal++
	p.eventsByType[normalized]++
}

// CheckHealth returns the current conversion pipeline health assessment.
func (p *PipelineHealth) CheckHealth() HealthStatus {
	now := time.Now()
	if p == nil {
		return HealthStatus{
			Status:              "healthy",
			LastEventAgeSeconds: 0,
			EventsTotal:         0,
			EventsByType:        map[string]int64{},
			StartedAt:           now.UnixMilli(),
		}
	}

	p.mu.RLock()
	startedAt := p.startedAt
	lastEventTime := p.lastEventTime
	eventsTotal := p.eventsTotal
	eventsByType := make(map[string]int64, len(p.eventsByType))
	for eventType, count := range p.eventsByType {
		eventsByType[eventType] = count
	}
	p.mu.RUnlock()

	if startedAt.IsZero() {
		startedAt = now
	}

	lastEventAge := now.Sub(startedAt).Seconds()
	if !lastEventTime.IsZero() {
		lastEventAge = now.Sub(lastEventTime).Seconds()
	}
	if lastEventAge < 0 {
		lastEventAge = 0
	}

	status := "healthy"
	if eventsTotal == 0 {
		if now.Sub(startedAt) > pipelineStaleThreshold {
			status = "stale"
		}
	} else if now.Sub(lastEventTime) > pipelineStaleThreshold {
		status = "stale"
	} else if !hasSufficientCoverage(eventsByType) {
		status = "degraded"
	}

	return HealthStatus{
		Status:              status,
		LastEventAgeSeconds: lastEventAge,
		EventsTotal:         eventsTotal,
		EventsByType:        eventsByType,
		StartedAt:           startedAt.UnixMilli(),
	}
}

func hasSufficientCoverage(eventsByType map[string]int64) bool {
	if len(knownConversionEventTypes) == 0 {
		return true
	}

	seen := 0
	for _, eventType := range knownConversionEventTypes {
		if eventsByType[eventType] > 0 {
			seen++
		}
	}

	return float64(seen) >= float64(len(knownConversionEventTypes))*0.5
}
