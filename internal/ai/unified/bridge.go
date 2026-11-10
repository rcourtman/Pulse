// Package unified provides a unified alert/finding system.
package unified

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AlertBridge connects the traditional alert system to the unified finding store.
// It listens for alert events and automatically creates/updates unified findings.
type AlertBridge struct {
	mu sync.RWMutex

	store           *UnifiedStore
	alertProvider   AlertProvider
	patrolTrigger   PatrolTriggerFunc
	triggerPolicy   PatrolTriggerPolicyFunc
	sustainedPolicy SustainedTriggerPolicyFunc
	enhancementFunc AIEnhancementFunc

	// Configuration
	autoEnhance          bool          // Whether to automatically request AI enhancement
	enhanceDelay         time.Duration // Delay before triggering AI enhancement
	triggerPatrolOnNew   bool          // Whether to trigger patrol on new alerts
	triggerPatrolOnClear bool          // Whether to trigger patrol on alert resolution

	// Tracking
	pendingEnhancements map[string]*time.Timer
	episodes            map[string]*alertEpisode // alert identifier -> firing episode state
	sustainedTriggers   []time.Time              // sustained triggers inside the last hour, all alerts
	startedAt           time.Time                // when the bridge last started
	now                 func() time.Time
	running             bool
	stopCh              chan struct{}
}

// Bounds on alert_fired Patrol triggers for one alert identifier. A firing
// episode is one stretch of an alert being active: it starts when the alert
// fires (or fires again after resolving) and ends when it resolves. Each
// episode can wake Patrol once, plus once more if it later rises to a level
// the trigger policy accepts after starting below it. A flapping alert starts
// a new episode on every flap, so the count is capped per identifier.
const (
	maxAlertFiredTriggersPerHour = 3
	maxAlertFiredTriggersPerDay  = 6
)

// The sustained trigger covers alerts that are active and have not woken
// Patrol in their current episode: an alert below the operator's minimum
// severity, or one that was already active when Pulse started. Once such an
// alert has stayed active for sustainedAlertDelay the bridge asks the
// sustained policy whether Patrol may look at it. It fires at most once per
// episode, counts toward the per-identifier caps above, and is capped across
// all alerts at maxSustainedTriggersPerHour. It is off until a policy is set.
const (
	sustainedAlertDelay         = 10 * time.Minute
	maxSustainedTriggersPerHour = 4
	sustainedCheckInterval      = 30 * time.Second
)

// alertEpisode tracks one alert identifier's current firing episode and its
// recent alert_fired triggers.
type alertEpisode struct {
	active         bool
	startedAt      time.Time   // the alert's start time for the current episode
	triggeredLevel int         // highest level this episode has already used to wake Patrol
	triggers       []time.Time // alert_fired triggers inside the daily window
	// sustainedDone is set once the episode has woken Patrol or has used its
	// sustained check, so the sustained trigger leaves it alone.
	sustainedDone bool
	// lastEvent is the payload of the latest dispatch. A sustained trigger
	// fires from a timer, long after the dispatch that carried the alert.
	lastEvent PatrolTriggerEvent
}

// AlertProvider provides access to alert data
type AlertProvider interface {
	// GetActiveAlerts returns all currently active alerts
	GetActiveAlerts() []AlertAdapter
	// GetAlert returns a specific alert by ID
	GetAlert(alertID string) AlertAdapter
	// SetAlertCallback sets the callback for new alerts
	SetAlertCallback(cb func(AlertAdapter))
	// SetResolvedCallback sets the callback for resolved alerts
	SetResolvedCallback(cb func(alertID string))
}

// PatrolTriggerEvent carries the firing (or resolving) alert's specifics so a
// scoped patrol can investigate the actual issue (the metric, the value, the
// threshold that was crossed) instead of re-checking the resource generically.
type PatrolTriggerEvent struct {
	ResourceID      string
	ResourceType    string
	Reason          string // "alert_fired" | "alert_cleared"
	AlertType       string // cpu, memory, disk, etc.
	AlertIdentifier string
	AlertLevel      string // warning | critical
	Value           float64
	Threshold       float64
	Message         string
	// Sustained marks an alert_fired event sent because the alert stayed
	// active without waking Patrol, not because it just fired. ActiveSince is
	// when its episode began.
	Sustained   bool
	ActiveSince time.Time
}

// PatrolTriggerFunc is called to trigger a mini-patrol for a resource
type PatrolTriggerFunc func(event PatrolTriggerEvent)

// PatrolTriggerPolicyFunc reports whether a firing alert of this type and
// level may wake Patrol under the operator's alert-trigger settings. The
// bridge asks before it spends an episode's trigger, so an alert that starts
// below the minimum severity can still wake Patrol when it rises above it.
type PatrolTriggerPolicyFunc func(alertType, alertLevel string) bool

// SustainedTriggerDecision is a sustained policy's answer for one alert.
type SustainedTriggerDecision int

const (
	// SustainedTriggerWait: not now, ask again later. Use it for conditions
	// that can change while the alert stays active, such as Patrol's mode.
	SustainedTriggerWait SustainedTriggerDecision = iota
	// SustainedTriggerSkip: never for this episode, for example because a
	// Patrol run has already looked at it.
	SustainedTriggerSkip
	// SustainedTriggerRun: wake Patrol for this alert now.
	SustainedTriggerRun
)

// SustainedTriggerPolicyFunc decides whether an alert that has stayed active
// since activeSince without waking Patrol may wake it now. It is called
// without the bridge lock held.
type SustainedTriggerPolicyFunc func(event PatrolTriggerEvent, activeSince time.Time) SustainedTriggerDecision

// AIEnhancementFunc is called to request AI enhancement of a finding
type AIEnhancementFunc func(findingID string) error

// BridgeConfig configures the alert bridge
type BridgeConfig struct {
	// AutoEnhance enables automatic AI enhancement of new threshold findings
	AutoEnhance bool
	// EnhanceDelay is the delay before triggering AI enhancement (allows grouping)
	EnhanceDelay time.Duration
	// TriggerPatrolOnNew triggers a mini-patrol when new alerts fire
	TriggerPatrolOnNew bool
	// TriggerPatrolOnClear triggers patrol verification when alerts resolve
	TriggerPatrolOnClear bool
}

// DefaultBridgeConfig returns sensible defaults
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		AutoEnhance:          true,
		EnhanceDelay:         30 * time.Second, // Wait 30s to allow alert grouping
		TriggerPatrolOnNew:   true,
		TriggerPatrolOnClear: true,
	}
}

// NewAlertBridge creates a new alert bridge
func NewAlertBridge(store *UnifiedStore, config BridgeConfig) *AlertBridge {
	return &AlertBridge{
		store:                store,
		autoEnhance:          config.AutoEnhance,
		enhanceDelay:         config.EnhanceDelay,
		triggerPatrolOnNew:   config.TriggerPatrolOnNew,
		triggerPatrolOnClear: config.TriggerPatrolOnClear,
		pendingEnhancements:  make(map[string]*time.Timer),
		episodes:             make(map[string]*alertEpisode),
		now:                  time.Now,
		stopCh:               make(chan struct{}),
	}
}

// SetAlertProvider sets the alert provider and registers callbacks
func (b *AlertBridge) SetAlertProvider(provider AlertProvider) {
	b.mu.Lock()
	b.alertProvider = provider
	b.mu.Unlock()

	// Register callbacks
	provider.SetAlertCallback(func(alert AlertAdapter) {
		b.handleNewAlert(alert)
	})
	provider.SetResolvedCallback(func(alertID string) {
		b.handleAlertResolved(alertID)
	})
}

// SetPatrolTrigger sets the function to call when patrol should be triggered
func (b *AlertBridge) SetPatrolTrigger(fn PatrolTriggerFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.patrolTrigger = fn
}

// SetPatrolTriggerPolicy sets the alert-trigger policy consulted before an
// alert_fired trigger. With no policy every firing level is accepted.
func (b *AlertBridge) SetPatrolTriggerPolicy(fn PatrolTriggerPolicyFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.triggerPolicy = fn
}

// SetSustainedTriggerPolicy turns the sustained trigger on. With no policy
// the bridge wakes Patrol only when an alert fires or rises.
func (b *AlertBridge) SetSustainedTriggerPolicy(fn SustainedTriggerPolicyFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sustainedPolicy = fn
}

// SetAIEnhancement sets the function to call for AI enhancement
func (b *AlertBridge) SetAIEnhancement(fn AIEnhancementFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enhancementFunc = fn
}

// Start begins the bridge operation
func (b *AlertBridge) Start() {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.startedAt = b.now()
	b.stopCh = make(chan struct{})
	stopCh := b.stopCh
	b.mu.Unlock()

	// Sync existing alerts
	b.syncExistingAlerts()

	go b.sustainedLoop(stopCh)

	log.Info().Msg("alert bridge started")
}

// Stop stops the bridge
func (b *AlertBridge) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	close(b.stopCh)

	// Cancel pending enhancements
	for _, timer := range b.pendingEnhancements {
		timer.Stop()
	}
	b.pendingEnhancements = make(map[string]*time.Timer)
	b.mu.Unlock()

	log.Info().Msg("alert bridge stopped")
}

// syncExistingAlerts syncs all currently active alerts to the unified store
func (b *AlertBridge) syncExistingAlerts() {
	b.mu.RLock()
	provider := b.alertProvider
	b.mu.RUnlock()

	if provider == nil {
		return
	}

	alerts := provider.GetActiveAlerts()
	for _, alert := range alerts {
		finding, isNew := b.store.AddFromAlert(alert)
		// An alert that was already active when the bridge started is an
		// episode in progress, not a new firing. Record it at its current
		// level so a later re-notification does not wake Patrol, while a rise
		// in severity or the next episode still can. The sustained trigger may
		// still look at it once.
		b.seedEpisode(alert, alertFiredEvent(finding, alert))
		if isNew {
			log.Debug().
				Str("alert_identifier", alert.GetAlertIdentifier()).
				Str("resource", alert.GetResourceName()).
				Msg("Synced existing alert to unified store")
		}
	}
}

// handleNewAlert handles an alert dispatch from the alert system: a new
// firing, a re-firing after a resolve, or a repeat for an alert that is still
// active (for example a severity change).
func (b *AlertBridge) handleNewAlert(alert AlertAdapter) {
	b.mu.RLock()
	running := b.running
	triggerPatrol := b.triggerPatrolOnNew
	patrolFn := b.patrolTrigger
	policyFn := b.triggerPolicy
	autoEnhance := b.autoEnhance
	enhanceFn := b.enhancementFunc
	enhanceDelay := b.enhanceDelay
	b.mu.RUnlock()

	if !running {
		return
	}

	// Add to unified store
	finding, outcome := b.store.RecordAlert(alert)
	alertIdentifier := alert.GetAlertIdentifier()

	switch outcome {
	case AlertRecordCreated:
		log.Info().
			Str("finding_id", finding.ID).
			Str("alert_identifier", alertIdentifier).
			Str("resource", finding.ResourceName).
			Str("category", string(finding.Category)).
			Str("severity", string(finding.Severity)).
			Msg("Created unified finding from new alert")
	case AlertRecordReopened:
		log.Info().
			Str("finding_id", finding.ID).
			Str("alert_identifier", alertIdentifier).
			Str("resource", finding.ResourceName).
			Msg("Reopened unified finding: alert fired again")
	}

	// Trigger mini-patrol for the resource
	if triggerPatrol && patrolFn != nil {
		accepted := policyFn == nil || policyFn(alert.GetAlertType(), alert.GetAlertLevel())
		event := alertFiredEvent(finding, alert)
		switch b.claimFiredTrigger(alert, outcome != AlertRecordUpdated, accepted, event) {
		case firedTriggerClaimed:
			go patrolFn(event)
		case firedTriggerRateLimited:
			log.Info().
				Str("alert_identifier", alertIdentifier).
				Str("resource", finding.ResourceName).
				Int("max_per_hour", maxAlertFiredTriggersPerHour).
				Int("max_per_day", maxAlertFiredTriggersPerDay).
				Msg("Alert bridge: alert fired again but this alert has used its Patrol triggers for now")
		}
	}

	// Schedule AI enhancement
	if outcome == AlertRecordCreated && autoEnhance && enhanceFn != nil {
		b.scheduleEnhancement(finding.ID, enhanceDelay, enhanceFn)
	}
}

type firedTriggerDecision int

const (
	// firedTriggerNone: nothing new to wake Patrol for (same episode at a
	// level it already used, or the trigger policy does not accept the level).
	firedTriggerNone firedTriggerDecision = iota
	firedTriggerClaimed
	firedTriggerRateLimited
)

// alertFiredEvent builds the alert_fired payload for an alert and its finding.
func alertFiredEvent(finding *UnifiedFinding, alert AlertAdapter) PatrolTriggerEvent {
	return PatrolTriggerEvent{
		ResourceID:      finding.ResourceID,
		ResourceType:    finding.ResourceType,
		Reason:          "alert_fired",
		AlertType:       finding.AlertType,
		AlertIdentifier: alert.GetAlertIdentifier(),
		AlertLevel:      alert.GetAlertLevel(),
		Value:           alert.GetValue(),
		Threshold:       alert.GetThreshold(),
		Message:         alert.GetMessage(),
	}
}

func alertLevelRank(level string) int {
	if strings.EqualFold(strings.TrimSpace(level), "critical") {
		return 2
	}
	return 1
}

// seedEpisode records an alert that is already active as an episode that has
// used its trigger at the alert's current level.
func (b *AlertBridge) seedEpisode(alert AlertAdapter, event PatrolTriggerEvent) {
	alertIdentifier := alert.GetAlertIdentifier()
	if alertIdentifier == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	episode := b.episodes[alertIdentifier]
	if episode == nil {
		episode = &alertEpisode{}
		b.episodes[alertIdentifier] = episode
	}
	if episode.active {
		return
	}
	episode.active = true
	episode.startedAt = alert.GetStartTime()
	episode.triggeredLevel = alertLevelRank(alert.GetAlertLevel())
	episode.sustainedDone = false
	episode.lastEvent = event
}

// claimFiredTrigger decides whether this dispatch may wake Patrol and, if so,
// spends the episode's trigger for the alert's level. fresh is true when the
// store saw the alert fire for the first time or fire again after a resolve.
func (b *AlertBridge) claimFiredTrigger(alert AlertAdapter, fresh, policyAccepts bool, event PatrolTriggerEvent) firedTriggerDecision {
	alertIdentifier := alert.GetAlertIdentifier()
	now := b.now()

	b.mu.Lock()
	defer b.mu.Unlock()

	episode := b.episodes[alertIdentifier]
	if episode == nil {
		episode = &alertEpisode{}
		b.episodes[alertIdentifier] = episode
		fresh = true
	}
	// A resolve can be missed or arrive late, because fired and resolved
	// callbacks run on separate goroutines. A later start time is then the
	// only sign that the alert manager began a new occurrence.
	startedAt := alert.GetStartTime()
	if !fresh && (!episode.active || startedAt.After(episode.startedAt)) {
		fresh = true
	}
	if fresh {
		episode.active = true
		episode.startedAt = startedAt
		episode.triggeredLevel = 0
		episode.sustainedDone = false
	}
	episode.lastEvent = event

	level := alertLevelRank(alert.GetAlertLevel())
	if level <= episode.triggeredLevel || !policyAccepts {
		return firedTriggerNone
	}
	// The level is spent whether or not the cap lets the trigger through, so
	// one episode never retries by itself. Either way the sustained trigger
	// has nothing left to do for it.
	episode.triggeredLevel = level
	episode.sustainedDone = true

	if !b.spendAlertTriggerLocked(episode, now) {
		return firedTriggerRateLimited
	}
	return firedTriggerClaimed
}

// spendAlertTriggerLocked applies the per-identifier caps and records the
// trigger when they allow it. Caller holds b.mu.
func (b *AlertBridge) spendAlertTriggerLocked(episode *alertEpisode, now time.Time) bool {
	dayAgo := now.Add(-24 * time.Hour)
	hourAgo := now.Add(-time.Hour)
	kept := episode.triggers[:0]
	lastHour := 0
	for _, at := range episode.triggers {
		if !at.After(dayAgo) {
			continue
		}
		kept = append(kept, at)
		if at.After(hourAgo) {
			lastHour++
		}
	}
	episode.triggers = kept
	if lastHour >= maxAlertFiredTriggersPerHour || len(kept) >= maxAlertFiredTriggersPerDay {
		return false
	}
	episode.triggers = append(episode.triggers, now)
	return true
}

// sustainedLoop runs the sustained check until the bridge stops.
func (b *AlertBridge) sustainedLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(sustainedCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			b.checkSustainedAlerts()
		}
	}
}

// checkSustainedAlerts wakes Patrol for alerts that have stayed active for
// sustainedAlertDelay without waking it in their current episode, when the
// sustained policy allows and the caps have room. Critical alerts go first,
// then the longest active.
func (b *AlertBridge) checkSustainedAlerts() {
	now := b.now()

	type candidate struct {
		identifier string
		startedAt  time.Time
		event      PatrolTriggerEvent
	}
	b.mu.Lock()
	policyFn := b.sustainedPolicy
	patrolFn := b.patrolTrigger
	if !b.running || !b.triggerPatrolOnNew || policyFn == nil || patrolFn == nil {
		b.mu.Unlock()
		return
	}
	candidates := make([]candidate, 0)
	for identifier, episode := range b.episodes {
		if !episode.active || episode.sustainedDone || episode.lastEvent.AlertIdentifier == "" {
			continue
		}
		// An alert that was already active at start waits the same delay
		// from the start, so Patrol's inventory is loaded before it looks.
		since := episode.startedAt
		if b.startedAt.After(since) {
			since = b.startedAt
		}
		if now.Sub(since) < sustainedAlertDelay {
			continue
		}
		candidates = append(candidates, candidate{identifier: identifier, startedAt: episode.startedAt, event: episode.lastEvent})
	}
	b.mu.Unlock()
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool {
		iLevel, jLevel := alertLevelRank(candidates[i].event.AlertLevel), alertLevelRank(candidates[j].event.AlertLevel)
		if iLevel != jLevel {
			return iLevel > jLevel
		}
		if !candidates[i].startedAt.Equal(candidates[j].startedAt) {
			return candidates[i].startedAt.Before(candidates[j].startedAt)
		}
		return candidates[i].identifier < candidates[j].identifier
	})

	for _, c := range candidates {
		decision := policyFn(c.event, c.startedAt)
		if decision == SustainedTriggerWait {
			continue
		}

		b.mu.Lock()
		episode := b.episodes[c.identifier]
		// The alert may have resolved or started a new episode meanwhile.
		if episode == nil || !episode.active || episode.sustainedDone || !episode.startedAt.Equal(c.startedAt) {
			b.mu.Unlock()
			continue
		}
		if decision == SustainedTriggerSkip {
			episode.sustainedDone = true
			b.mu.Unlock()
			continue
		}
		hourAgo := now.Add(-time.Hour)
		kept := b.sustainedTriggers[:0]
		for _, at := range b.sustainedTriggers {
			if at.After(hourAgo) {
				kept = append(kept, at)
			}
		}
		b.sustainedTriggers = kept
		if len(kept) >= maxSustainedTriggersPerHour {
			// Out of room this hour. The remaining alerts keep waiting.
			b.mu.Unlock()
			return
		}
		episode.sustainedDone = true
		if !b.spendAlertTriggerLocked(episode, now) {
			b.mu.Unlock()
			continue
		}
		b.sustainedTriggers = append(b.sustainedTriggers, now)
		event := episode.lastEvent
		b.mu.Unlock()

		event.Sustained = true
		event.ActiveSince = c.startedAt
		log.Info().
			Str("alert_identifier", c.identifier).
			Str("level", event.AlertLevel).
			Dur("active_for", now.Sub(c.startedAt)).
			Msg("Alert bridge: alert stayed active without a Patrol run, asking Patrol to look")
		go patrolFn(event)
	}
}

// endEpisode marks an alert identifier's firing episode as over.
func (b *AlertBridge) endEpisode(alertIdentifier string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	episode := b.episodes[alertIdentifier]
	if episode == nil {
		return
	}
	episode.active = false
	episode.triggeredLevel = 0
	episode.sustainedDone = false
	// Keep the entry only while it still counts toward the daily cap.
	dayAgo := b.now().Add(-24 * time.Hour)
	for _, at := range episode.triggers {
		if at.After(dayAgo) {
			return
		}
	}
	delete(b.episodes, alertIdentifier)
}

// handleAlertResolved handles an alert being resolved
func (b *AlertBridge) handleAlertResolved(alertID string) {
	b.mu.RLock()
	running := b.running
	triggerPatrol := b.triggerPatrolOnClear
	patrolFn := b.patrolTrigger
	b.mu.RUnlock()

	if !running {
		return
	}

	b.endEpisode(alertID)

	// Get the finding before resolving
	finding := b.store.GetByAlertIdentifier(alertID)

	// Resolve the unified finding
	if b.store.ResolveByAlertIdentifier(alertID) {
		log.Info().
			Str("alert_identifier", alertID).
			Msg("Resolved unified finding from alert")

		// Cancel any pending enhancement
		b.cancelEnhancement(alertID)

		// Trigger verification patrol
		if triggerPatrol && patrolFn != nil && finding != nil {
			go patrolFn(PatrolTriggerEvent{
				ResourceID:      finding.ResourceID,
				ResourceType:    finding.ResourceType,
				Reason:          "alert_cleared",
				AlertType:       finding.AlertType,
				AlertIdentifier: finding.AlertIdentifier,
				Value:           finding.Value,
				Threshold:       finding.Threshold,
				Message:         finding.Description,
			})
		}
	}
}

// scheduleEnhancement schedules an AI enhancement for a finding
func (b *AlertBridge) scheduleEnhancement(findingID string, delay time.Duration, enhanceFn AIEnhancementFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Cancel any existing timer
	if timer, ok := b.pendingEnhancements[findingID]; ok {
		timer.Stop()
	}

	// Schedule new enhancement
	timer := time.AfterFunc(delay, func() {
		b.mu.Lock()
		delete(b.pendingEnhancements, findingID)
		b.mu.Unlock()

		// Check if finding is still active
		finding := b.store.Get(findingID)
		if finding == nil || !finding.IsActive() {
			return
		}

		// Request enhancement
		if err := enhanceFn(findingID); err != nil {
			log.Error().
				Err(err).
				Str("finding_id", findingID).
				Msg("Failed to enhance finding with AI")
		}
	})

	b.pendingEnhancements[findingID] = timer
}

// cancelEnhancement cancels a pending enhancement
func (b *AlertBridge) cancelEnhancement(alertID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Find the finding ID for this alert
	finding := b.store.GetByAlertIdentifier(alertID)
	if finding == nil {
		return
	}

	if timer, ok := b.pendingEnhancements[finding.ID]; ok {
		timer.Stop()
		delete(b.pendingEnhancements, finding.ID)
	}
}

// GetUnifiedStore returns the unified store
func (b *AlertBridge) GetUnifiedStore() *UnifiedStore {
	return b.store
}

// Stats returns bridge statistics
func (b *AlertBridge) Stats() BridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BridgeStats{
		Running:             b.running,
		PendingEnhancements: len(b.pendingEnhancements),
	}
}

// BridgeStats contains bridge statistics
type BridgeStats struct {
	Running             bool `json:"running"`
	PendingEnhancements int  `json:"pending_enhancements"`
}

// SimpleAlertAdapter wraps basic alert data to implement AlertAdapter
type SimpleAlertAdapter struct {
	AlertIdentifier string
	AlertType       string
	AlertLevel      string
	ResourceID      string
	ResourceName    string
	Node            string
	Message         string
	Value           float64
	Threshold       float64
	StartTime       time.Time
	LastSeen        time.Time
	Metadata        map[string]interface{}
}

func (a *SimpleAlertAdapter) GetAlertIdentifier() string          { return a.AlertIdentifier }
func (a *SimpleAlertAdapter) GetAlertType() string                { return a.AlertType }
func (a *SimpleAlertAdapter) GetAlertLevel() string               { return a.AlertLevel }
func (a *SimpleAlertAdapter) GetResourceID() string               { return a.ResourceID }
func (a *SimpleAlertAdapter) GetResourceName() string             { return a.ResourceName }
func (a *SimpleAlertAdapter) GetNode() string                     { return a.Node }
func (a *SimpleAlertAdapter) GetMessage() string                  { return a.Message }
func (a *SimpleAlertAdapter) GetValue() float64                   { return a.Value }
func (a *SimpleAlertAdapter) GetThreshold() float64               { return a.Threshold }
func (a *SimpleAlertAdapter) GetStartTime() time.Time             { return a.StartTime }
func (a *SimpleAlertAdapter) GetLastSeen() time.Time              { return a.LastSeen }
func (a *SimpleAlertAdapter) GetMetadata() map[string]interface{} { return a.Metadata }
