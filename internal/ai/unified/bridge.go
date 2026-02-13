// Package unified provides a unified alert/finding system.
package unified

import (
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
	enhancementFunc AIEnhancementFunc

	// Configuration
	autoEnhance          bool          // Whether to automatically request AI enhancement
	enhanceDelay         time.Duration // Delay before triggering AI enhancement
	triggerPatrolOnNew   bool          // Whether to trigger patrol on new alerts
	triggerPatrolOnClear bool          // Whether to trigger patrol on alert resolution

	// Tracking
	pendingEnhancements map[string]*time.Timer
	running             bool
	stopCh              chan struct{}
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

// PatrolTriggerFunc is called to trigger a mini-patrol for a resource
type PatrolTriggerFunc func(resourceID, resourceType, reason, alertType string)

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
	b.stopCh = make(chan struct{})
	b.mu.Unlock()

	// Sync existing alerts
	b.syncExistingAlerts()

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
		_, isNew := b.store.AddFromAlert(alert)
		if isNew {
			log.Debug().
				Str("alert_id", alert.GetAlertID()).
				Str("resource", alert.GetResourceName()).
				Msg("Synced existing alert to unified store")
		}
	}
}

// handleNewAlert handles a new alert from the alert system
func (b *AlertBridge) handleNewAlert(alert AlertAdapter) {
	b.mu.RLock()
	running := b.running
	triggerPatrol := b.triggerPatrolOnNew
	patrolFn := b.patrolTrigger
	autoEnhance := b.autoEnhance
	enhanceFn := b.enhancementFunc
	enhanceDelay := b.enhanceDelay
	b.mu.RUnlock()

	if !running {
		return
	}

	// Add to unified store
	finding, isNew := b.store.AddFromAlert(alert)

	if isNew {
		log.Info().
			Str("finding_id", finding.ID).
			Str("alert_id", alert.GetAlertID()).
			Str("resource", finding.ResourceName).
			Str("category", string(finding.Category)).
			Str("severity", string(finding.Severity)).
			Msg("Created unified finding from new alert")

		// Trigger mini-patrol for the resource
		if triggerPatrol && patrolFn != nil {
			go patrolFn(finding.ResourceID, finding.ResourceType, "alert_fired", finding.AlertType)
		}

		// Schedule AI enhancement
		if autoEnhance && enhanceFn != nil {
			b.scheduleEnhancement(finding.ID, enhanceDelay, enhanceFn)
		}
	}
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

	// Get the finding before resolving
	finding := b.store.GetByAlert(alertID)

	// Resolve the unified finding
	if b.store.ResolveByAlert(alertID) {
		log.Info().
			Str("alert_id", alertID).
			Msg("Resolved unified finding from alert")

		// Cancel any pending enhancement
		b.cancelEnhancement(alertID)

		// Trigger verification patrol
		if triggerPatrol && patrolFn != nil && finding != nil {
			go patrolFn(finding.ResourceID, finding.ResourceType, "alert_cleared", finding.AlertType)
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
	finding := b.store.GetByAlert(alertID)
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
	AlertID      string
	AlertType    string
	AlertLevel   string
	ResourceID   string
	ResourceName string
	Node         string
	Message      string
	Value        float64
	Threshold    float64
	StartTime    time.Time
	LastSeen     time.Time
	Metadata     map[string]interface{}
}

func (a *SimpleAlertAdapter) GetAlertID() string                  { return a.AlertID }
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
