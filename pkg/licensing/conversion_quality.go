package licensing

import (
	"strings"
	"sync"
	"time"
)

const defaultPipelineStaleThreshold = 5 * time.Minute

var pipelineCoverageEventTypes = []string{
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

// PipelineHealthOption configures pipeline health behavior.
type PipelineHealthOption func(*pipelineHealthOptions)

type pipelineHealthOptions struct {
	now            func() time.Time
	staleThreshold time.Duration
}

// WithPipelineHealthNow sets the clock source used by PipelineHealth.
func WithPipelineHealthNow(now func() time.Time) PipelineHealthOption {
	return func(options *pipelineHealthOptions) {
		options.now = now
	}
}

// WithPipelineHealthStaleThreshold sets the stale threshold used by PipelineHealth.
func WithPipelineHealthStaleThreshold(threshold time.Duration) PipelineHealthOption {
	return func(options *pipelineHealthOptions) {
		options.staleThreshold = threshold
	}
}

// PipelineHealth tracks conversion pipeline activity for health checks.
type PipelineHealth struct {
	mu             sync.RWMutex
	now            func() time.Time
	staleThreshold time.Duration
	startedAt      time.Time
	lastEventTime  time.Time
	eventsTotal    int64
	eventsByType   map[string]int64
}

// NewPipelineHealth creates a health tracker initialized at current time.
func NewPipelineHealth(opts ...PipelineHealthOption) *PipelineHealth {
	options := pipelineHealthOptions{
		now:            time.Now,
		staleThreshold: defaultPipelineStaleThreshold,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&options)
	}
	if options.now == nil {
		options.now = time.Now
	}
	if options.staleThreshold <= 0 {
		options.staleThreshold = defaultPipelineStaleThreshold
	}

	return &PipelineHealth{
		now:            options.now,
		staleThreshold: options.staleThreshold,
		startedAt:      options.now(),
		eventsByType:   make(map[string]int64),
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

	now := p.clockNow()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.startedAt.IsZero() {
		p.startedAt = now
	}
	if p.eventsByType == nil {
		p.eventsByType = make(map[string]int64)
	}

	p.lastEventTime = now
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
	now = p.clockNow()
	staleThreshold := p.staleThreshold
	if staleThreshold <= 0 {
		staleThreshold = defaultPipelineStaleThreshold
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
		if now.Sub(startedAt) > staleThreshold {
			status = "stale"
		}
	} else if now.Sub(lastEventTime) > staleThreshold {
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

// KnownConversionEventTypes returns the event types used by quality coverage checks.
func KnownConversionEventTypes() []string {
	out := make([]string, len(pipelineCoverageEventTypes))
	copy(out, pipelineCoverageEventTypes)
	return out
}

func (p *PipelineHealth) clockNow() time.Time {
	if p == nil || p.now == nil {
		return time.Now()
	}
	return p.now()
}

func hasSufficientCoverage(eventsByType map[string]int64) bool {
	if len(pipelineCoverageEventTypes) == 0 {
		return true
	}

	seen := 0
	for _, eventType := range pipelineCoverageEventTypes {
		if eventsByType[eventType] > 0 {
			seen++
		}
	}

	return float64(seen) >= float64(len(pipelineCoverageEventTypes))*0.5
}
