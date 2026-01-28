package ai

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TriggerReason describes why a patrol was triggered
type TriggerReason string

const (
	TriggerReasonScheduled       TriggerReason = "scheduled"      // Regular interval trigger
	TriggerReasonManual          TriggerReason = "manual"         // User-initiated patrol
	TriggerReasonAlertFired      TriggerReason = "alert_fired"    // New alert triggered
	TriggerReasonAlertCleared    TriggerReason = "alert_cleared"  // Alert was resolved
	TriggerReasonAnomalyDetected TriggerReason = "anomaly"        // Baseline breach detected
	TriggerReasonUserAction      TriggerReason = "user_action"    // User dismissed/snoozed finding
	TriggerReasonConfigChanged   TriggerReason = "config_changed" // System configuration changed
	TriggerReasonStartup         TriggerReason = "startup"        // Service startup
)

// PatrolScope defines the scope of a patrol run
type PatrolScope struct {
	// ResourceIDs limits patrol to specific resources (empty = all resources)
	ResourceIDs []string
	// ResourceTypes limits patrol to specific resource types (empty = all types)
	ResourceTypes []string
	// Depth controls how thorough the patrol should be
	Depth PatrolDepth
	// Reason why this patrol was triggered
	Reason TriggerReason
	// Context provides additional information about the trigger
	Context string
	// Priority indicates relative urgency (higher = more urgent)
	Priority int
	// AlertID is the ID of the alert that triggered this patrol (if applicable)
	AlertID string
	// FindingID is the ID of the finding that triggered this patrol (if applicable)
	FindingID string
}

// PatrolDepth controls how thorough a patrol run should be
type PatrolDepth int

const (
	// PatrolDepthQuick is a fast, focused check on specific resources
	PatrolDepthQuick PatrolDepth = iota
	// PatrolDepthNormal is the standard patrol depth
	PatrolDepthNormal
	// PatrolDepthDeep is a thorough analysis with more context
	PatrolDepthDeep
)

// String returns the depth as a string
func (d PatrolDepth) String() string {
	switch d {
	case PatrolDepthQuick:
		return "quick"
	case PatrolDepthNormal:
		return "normal"
	case PatrolDepthDeep:
		return "deep"
	default:
		return "unknown"
	}
}

// TriggerManager manages patrol triggers with rate limiting and prioritization
type TriggerManager struct {
	mu sync.Mutex

	// Rate limiting per resource
	lastPatrolByResource map[string]time.Time
	minResourceInterval  time.Duration

	// Global rate limiting
	lastGlobalPatrol  time.Time
	minGlobalInterval time.Duration

	// Pending triggers queue (priority queue)
	pendingTriggers    []PatrolScope
	maxPendingTriggers int

	// Adaptive interval management
	baseInterval    time.Duration
	currentInterval time.Duration
	busyThreshold   int // Number of alerts/anomalies to consider "busy"
	recentEvents    []time.Time
	eventWindow     time.Duration

	// Callback to execute patrol
	onTrigger func(ctx context.Context, scope PatrolScope)

	// Stop channel
	stopCh  chan struct{}
	running bool
}

// TriggerManagerConfig configures the trigger manager
type TriggerManagerConfig struct {
	MinResourceInterval time.Duration // Minimum time between patrols for same resource
	MinGlobalInterval   time.Duration // Minimum time between any patrols
	MaxPendingTriggers  int           // Maximum pending triggers to queue
	BaseInterval        time.Duration // Base scheduled interval
	BusyThreshold       int           // Events to trigger "busy" mode
	EventWindow         time.Duration // Window to count events for busy detection
}

// DefaultTriggerManagerConfig returns sensible defaults
func DefaultTriggerManagerConfig() TriggerManagerConfig {
	return TriggerManagerConfig{
		MinResourceInterval: 2 * time.Minute,
		MinGlobalInterval:   30 * time.Second,
		MaxPendingTriggers:  10,
		BaseInterval:        15 * time.Minute,
		BusyThreshold:       5,
		EventWindow:         5 * time.Minute,
	}
}

// NewTriggerManager creates a new trigger manager
func NewTriggerManager(cfg TriggerManagerConfig) *TriggerManager {
	if cfg.MinResourceInterval <= 0 {
		cfg.MinResourceInterval = 2 * time.Minute
	}
	if cfg.MinGlobalInterval <= 0 {
		cfg.MinGlobalInterval = 30 * time.Second
	}
	if cfg.MaxPendingTriggers <= 0 {
		cfg.MaxPendingTriggers = 10
	}
	if cfg.BaseInterval <= 0 {
		cfg.BaseInterval = 15 * time.Minute
	}
	if cfg.BusyThreshold <= 0 {
		cfg.BusyThreshold = 5
	}
	if cfg.EventWindow <= 0 {
		cfg.EventWindow = 5 * time.Minute
	}

	return &TriggerManager{
		lastPatrolByResource: make(map[string]time.Time),
		minResourceInterval:  cfg.MinResourceInterval,
		minGlobalInterval:    cfg.MinGlobalInterval,
		maxPendingTriggers:   cfg.MaxPendingTriggers,
		baseInterval:         cfg.BaseInterval,
		currentInterval:      cfg.BaseInterval,
		busyThreshold:        cfg.BusyThreshold,
		eventWindow:          cfg.EventWindow,
		recentEvents:         make([]time.Time, 0),
		pendingTriggers:      make([]PatrolScope, 0),
		stopCh:               make(chan struct{}),
	}
}

// SetOnTrigger sets the callback function to execute when a patrol should run
func (tm *TriggerManager) SetOnTrigger(fn func(ctx context.Context, scope PatrolScope)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.onTrigger = fn
}

// Start begins the trigger manager's background processing
func (tm *TriggerManager) Start(ctx context.Context) {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return
	}
	tm.running = true
	tm.stopCh = make(chan struct{})
	tm.mu.Unlock()

	go tm.processLoop(ctx)
	log.Info().Msg("Patrol trigger manager started")
}

// Stop stops the trigger manager
func (tm *TriggerManager) Stop() {
	tm.mu.Lock()
	if !tm.running {
		tm.mu.Unlock()
		return
	}
	tm.running = false
	close(tm.stopCh)
	tm.mu.Unlock()
	log.Info().Msg("Patrol trigger manager stopped")
}

// processLoop processes pending triggers and handles scheduled patrols
func (tm *TriggerManager) processLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.processPendingTriggers(ctx)
		}
	}
}

// processPendingTriggers checks and executes pending triggers
func (tm *TriggerManager) processPendingTriggers(ctx context.Context) {
	tm.mu.Lock()

	if len(tm.pendingTriggers) == 0 || tm.onTrigger == nil {
		tm.mu.Unlock()
		return
	}

	// Compute adaptive global rate limit based on activity level
	// When busy (currentInterval < baseInterval), process triggers faster
	// When quiet (currentInterval > baseInterval), space them out more
	adaptiveGlobalInterval := tm.minGlobalInterval
	if tm.baseInterval > 0 {
		ratio := float64(tm.currentInterval) / float64(tm.baseInterval)
		adaptiveGlobalInterval = time.Duration(float64(tm.minGlobalInterval) * ratio)
		// Clamp to reasonable bounds: 10 seconds to 2 minutes
		if adaptiveGlobalInterval < 10*time.Second {
			adaptiveGlobalInterval = 10 * time.Second
		}
		if adaptiveGlobalInterval > 2*time.Minute {
			adaptiveGlobalInterval = 2 * time.Minute
		}
	}

	// Check global rate limit with adaptive interval
	if time.Since(tm.lastGlobalPatrol) < adaptiveGlobalInterval {
		tm.mu.Unlock()
		return
	}

	// Get highest priority trigger
	highestPriority := -1
	highestIndex := -1
	for i, trigger := range tm.pendingTriggers {
		if trigger.Priority > highestPriority {
			// Check resource rate limit
			if len(trigger.ResourceIDs) > 0 {
				canRun := true
				for _, rid := range trigger.ResourceIDs {
					if lastPatrol, ok := tm.lastPatrolByResource[rid]; ok {
						if time.Since(lastPatrol) < tm.minResourceInterval {
							canRun = false
							break
						}
					}
				}
				if !canRun {
					continue
				}
			}
			highestPriority = trigger.Priority
			highestIndex = i
		}
	}

	if highestIndex < 0 {
		tm.mu.Unlock()
		return
	}

	// Remove trigger from queue
	trigger := tm.pendingTriggers[highestIndex]
	tm.pendingTriggers = append(tm.pendingTriggers[:highestIndex], tm.pendingTriggers[highestIndex+1:]...)

	// Update rate limiting
	tm.lastGlobalPatrol = time.Now()
	for _, rid := range trigger.ResourceIDs {
		tm.lastPatrolByResource[rid] = time.Now()
	}

	callback := tm.onTrigger
	tm.mu.Unlock()

	// Execute callback outside lock
	log.Info().
		Str("reason", string(trigger.Reason)).
		Int("priority", trigger.Priority).
		Str("depth", trigger.Depth.String()).
		Strs("resources", trigger.ResourceIDs).
		Msg("Executing triggered patrol")

	callback(ctx, trigger)
}

// TriggerPatrol queues a patrol run with the given scope
// Returns true if the trigger was accepted, false if rate limited or queue full
func (tm *TriggerManager) TriggerPatrol(scope PatrolScope) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if queue is full
	if len(tm.pendingTriggers) >= tm.maxPendingTriggers {
		// Replace lowest priority if new trigger is higher priority
		lowestPriority := scope.Priority
		lowestIndex := -1
		for i, t := range tm.pendingTriggers {
			if t.Priority < lowestPriority {
				lowestPriority = t.Priority
				lowestIndex = i
			}
		}
		if lowestIndex < 0 {
			log.Debug().
				Str("reason", string(scope.Reason)).
				Msg("Patrol trigger rejected: queue full and lower priority")
			return false
		}
		// Remove lowest priority trigger
		tm.pendingTriggers = append(tm.pendingTriggers[:lowestIndex], tm.pendingTriggers[lowestIndex+1:]...)
	}

	// Check for duplicate triggers (same reason and resources)
	for i := range tm.pendingTriggers {
		if tm.pendingTriggers[i].Reason == scope.Reason && slicesEqual(tm.pendingTriggers[i].ResourceIDs, scope.ResourceIDs) {
			// Update priority if new trigger is higher priority
			if scope.Priority > tm.pendingTriggers[i].Priority {
				tm.pendingTriggers[i].Priority = scope.Priority
			}
			log.Debug().
				Str("reason", string(scope.Reason)).
				Msg("Patrol trigger merged with existing")
			return true
		}
	}

	// Record event for adaptive interval
	tm.recentEvents = append(tm.recentEvents, time.Now())
	tm.cleanupOldEvents()
	tm.updateAdaptiveInterval()

	// Add to queue
	tm.pendingTriggers = append(tm.pendingTriggers, scope)

	log.Debug().
		Str("reason", string(scope.Reason)).
		Int("queue_size", len(tm.pendingTriggers)).
		Msg("Patrol trigger queued")

	return true
}

// cleanupOldEvents removes events outside the event window
func (tm *TriggerManager) cleanupOldEvents() {
	cutoff := time.Now().Add(-tm.eventWindow)
	kept := make([]time.Time, 0, len(tm.recentEvents))
	for _, t := range tm.recentEvents {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	tm.recentEvents = kept
}

// updateAdaptiveInterval adjusts the scheduled interval based on activity
func (tm *TriggerManager) updateAdaptiveInterval() {
	eventCount := len(tm.recentEvents)

	if eventCount >= tm.busyThreshold {
		// Busy mode: reduce interval to 5 minutes
		tm.currentInterval = 5 * time.Minute
	} else if eventCount == 0 {
		// Quiet mode: extend interval to 30 minutes
		tm.currentInterval = 30 * time.Minute
	} else {
		// Normal mode: use base interval
		tm.currentInterval = tm.baseInterval
	}
}

// GetCurrentInterval returns the current adaptive interval
func (tm *TriggerManager) GetCurrentInterval() time.Duration {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.currentInterval
}

// GetPendingCount returns the number of pending triggers
func (tm *TriggerManager) GetPendingCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.pendingTriggers)
}

// TriggerStatus returns the current status of the trigger manager
type TriggerStatus struct {
	Running         bool          `json:"running"`
	PendingTriggers int           `json:"pending_triggers"`
	CurrentInterval time.Duration `json:"current_interval_ms"`
	RecentEvents    int           `json:"recent_events"`
	IsBusyMode      bool          `json:"is_busy_mode"`
}

// GetStatus returns the current status
func (tm *TriggerManager) GetStatus() TriggerStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.cleanupOldEvents()

	return TriggerStatus{
		Running:         tm.running,
		PendingTriggers: len(tm.pendingTriggers),
		CurrentInterval: tm.currentInterval,
		RecentEvents:    len(tm.recentEvents),
		IsBusyMode:      len(tm.recentEvents) >= tm.busyThreshold,
	}
}

// Helper functions

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	if len(a) == 1 {
		return a[0] == b[0]
	}
	aSorted := append([]string(nil), a...)
	bSorted := append([]string(nil), b...)
	sort.Strings(aSorted)
	sort.Strings(bSorted)
	for i := range aSorted {
		if aSorted[i] != bSorted[i] {
			return false
		}
	}
	return true
}

// AlertTriggeredPatrolScope creates a patrol scope for an alert that fired
func AlertTriggeredPatrolScope(alertID, resourceID, resourceType, alertType string) PatrolScope {
	return PatrolScope{
		ResourceIDs:   []string{resourceID},
		ResourceTypes: []string{resourceType},
		Depth:         PatrolDepthQuick,
		Reason:        TriggerReasonAlertFired,
		Context:       "Alert: " + alertType,
		Priority:      80, // High priority for alerts
		AlertID:       alertID,
	}
}

// AlertClearedPatrolScope creates a patrol scope for an alert that cleared
func AlertClearedPatrolScope(alertID, resourceID, resourceType string) PatrolScope {
	return PatrolScope{
		ResourceIDs:   []string{resourceID},
		ResourceTypes: []string{resourceType},
		Depth:         PatrolDepthQuick,
		Reason:        TriggerReasonAlertCleared,
		Context:       "Verify resolution",
		Priority:      40, // Medium-low priority for cleared alerts
		AlertID:       alertID,
	}
}

// AnomalyDetectedPatrolScope creates a patrol scope for an anomaly detection
func AnomalyDetectedPatrolScope(resourceID, resourceType, metric string, value, baseline float64) PatrolScope {
	return PatrolScope{
		ResourceIDs:   []string{resourceID},
		ResourceTypes: []string{resourceType},
		Depth:         PatrolDepthNormal,
		Reason:        TriggerReasonAnomalyDetected,
		Context:       "Anomaly: " + metric,
		Priority:      60, // Medium-high priority for anomalies
	}
}

// AnomalyTriggeredPatrolScope creates a patrol scope for a significant anomaly event
// This is a simplified version that takes severity as a string for use in callbacks
func AnomalyTriggeredPatrolScope(resourceID, resourceType, metric, severity string) PatrolScope {
	priority := 60 // default
	if severity == "critical" {
		priority = 85
	} else if severity == "high" {
		priority = 70
	}
	return PatrolScope{
		ResourceIDs:   []string{resourceID},
		ResourceTypes: []string{resourceType},
		Depth:         PatrolDepthQuick,
		Reason:        TriggerReasonAnomalyDetected,
		Context:       "Anomaly: " + metric + " (" + severity + ")",
		Priority:      priority,
	}
}

// UserActionPatrolScope creates a patrol scope for a user action on a finding
func UserActionPatrolScope(findingID, resourceID, action string) PatrolScope {
	return PatrolScope{
		ResourceIDs: []string{resourceID},
		Depth:       PatrolDepthQuick,
		Reason:      TriggerReasonUserAction,
		Context:     "User action: " + action,
		Priority:    30, // Lower priority for user actions
		FindingID:   findingID,
	}
}

// ManualPatrolScope creates a patrol scope for a manual patrol request
func ManualPatrolScope(resourceIDs []string, depth PatrolDepth) PatrolScope {
	return PatrolScope{
		ResourceIDs: resourceIDs,
		Depth:       depth,
		Reason:      TriggerReasonManual,
		Context:     "Manual request",
		Priority:    100, // Highest priority for manual requests
	}
}

// ScheduledPatrolScope creates a patrol scope for a scheduled patrol
func ScheduledPatrolScope() PatrolScope {
	return PatrolScope{
		Depth:    PatrolDepthNormal,
		Reason:   TriggerReasonScheduled,
		Context:  "Scheduled patrol",
		Priority: 20, // Low priority for scheduled
	}
}
