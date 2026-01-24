// Package ai provides AI-powered infrastructure analysis.
package ai

import (
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/metrics"
	"github.com/rs/zerolog/log"
)

// IncidentCoordinatorConfig configures the incident coordinator
type IncidentCoordinatorConfig struct {
	PreBuffer      time.Duration // History to capture before incident (default: 5 min)
	PostDuration   time.Duration // How long to record after trigger (default: 10 min)
	MaxConcurrent  int           // Maximum concurrent incident recordings (default: 50)
	EnableRecorder bool          // Whether to enable high-frequency recording
}

// DefaultIncidentCoordinatorConfig returns sensible defaults
func DefaultIncidentCoordinatorConfig() IncidentCoordinatorConfig {
	return IncidentCoordinatorConfig{
		PreBuffer:      5 * time.Minute,
		PostDuration:   10 * time.Minute,
		MaxConcurrent:  50,
		EnableRecorder: true,
	}
}

// IncidentCoordinator coordinates incident recording between the metrics.IncidentRecorder
// (for high-frequency data capture) and memory.IncidentStore (for incident timeline tracking).
type IncidentCoordinator struct {
	mu sync.RWMutex

	config IncidentCoordinatorConfig

	// Components
	recorder      *metrics.IncidentRecorder // High-frequency metrics capture
	incidentStore *memory.IncidentStore     // Incident timeline tracking

	// Active incidents - maps alert ID to window ID
	activeIncidents map[string]activeIncident

	// Control
	running bool
}

type activeIncident struct {
	windowID   string
	resourceID string
	startedAt  time.Time
	stopTimer  *time.Timer // Timer to auto-stop recording after post-duration
}

// NewIncidentCoordinator creates a new incident coordinator
func NewIncidentCoordinator(cfg IncidentCoordinatorConfig) *IncidentCoordinator {
	if cfg.PreBuffer <= 0 {
		cfg.PreBuffer = 5 * time.Minute
	}
	if cfg.PostDuration <= 0 {
		cfg.PostDuration = 10 * time.Minute
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 50
	}

	return &IncidentCoordinator{
		config:          cfg,
		activeIncidents: make(map[string]activeIncident),
	}
}

// SetRecorder sets the metrics incident recorder
func (c *IncidentCoordinator) SetRecorder(recorder *metrics.IncidentRecorder) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recorder = recorder
}

// SetIncidentStore sets the incident timeline store
func (c *IncidentCoordinator) SetIncidentStore(store *memory.IncidentStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.incidentStore = store
}

// Start starts the incident coordinator
func (c *IncidentCoordinator) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	log.Info().Msg("Incident coordinator started")
}

// Stop stops the incident coordinator
func (c *IncidentCoordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false

	// Stop all active timers
	for _, inc := range c.activeIncidents {
		if inc.stopTimer != nil {
			inc.stopTimer.Stop()
		}
	}
	c.activeIncidents = make(map[string]activeIncident)

	log.Info().Msg("Incident coordinator stopped")
}

// OnAlertFired is called when an alert fires - starts incident recording
func (c *IncidentCoordinator) OnAlertFired(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	// Check if we already have an active incident for this alert
	if _, exists := c.activeIncidents[alert.ID]; exists {
		log.Debug().
			Str("alert_id", alert.ID).
			Msg("Incident already being recorded for this alert")
		return
	}

	// Check concurrent limit
	if len(c.activeIncidents) >= c.config.MaxConcurrent {
		log.Warn().
			Str("alert_id", alert.ID).
			Int("active_count", len(c.activeIncidents)).
			Msg("Incident coordinator at capacity, skipping new incident")
		return
	}

	// Start high-frequency recording if enabled and recorder available
	var windowID string
	if c.config.EnableRecorder && c.recorder != nil {
		windowID = c.recorder.StartRecording(
			alert.ResourceID,
			alert.ResourceName,
			"", // resourceType - we don't always have this
			"alert",
			alert.ID,
		)
	}

	// Record in incident store
	if c.incidentStore != nil {
		c.incidentStore.RecordAlertFired(alert)
	}

	// Track the active incident
	inc := activeIncident{
		windowID:   windowID,
		resourceID: alert.ResourceID,
		startedAt:  time.Now(),
	}

	c.activeIncidents[alert.ID] = inc

	log.Info().
		Str("alert_id", alert.ID).
		Str("resource_id", alert.ResourceID).
		Str("window_id", windowID).
		Msg("Incident coordinator: Started incident recording")
}

// OnAlertCleared is called when an alert clears - schedules recording stop
func (c *IncidentCoordinator) OnAlertCleared(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	c.mu.Lock()

	inc, exists := c.activeIncidents[alert.ID]
	if !exists {
		c.mu.Unlock()
		return
	}

	// Record resolution in incident store
	if c.incidentStore != nil {
		c.incidentStore.RecordAlertResolved(alert, time.Now())
	}

	// If no recorder or no window, just clean up immediately
	if c.recorder == nil || inc.windowID == "" {
		delete(c.activeIncidents, alert.ID)
		c.mu.Unlock()
		return
	}

	// Schedule stop after post-duration to capture post-incident data
	alertID := alert.ID
	timer := time.AfterFunc(c.config.PostDuration, func() {
		c.stopIncidentRecording(alertID)
	})

	inc.stopTimer = timer
	c.activeIncidents[alert.ID] = inc
	c.mu.Unlock()

	log.Info().
		Str("alert_id", alert.ID).
		Str("window_id", inc.windowID).
		Dur("post_duration", c.config.PostDuration).
		Msg("Incident coordinator: Alert cleared, scheduled recording stop")
}

// stopIncidentRecording stops recording for a specific alert
func (c *IncidentCoordinator) stopIncidentRecording(alertID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	inc, exists := c.activeIncidents[alertID]
	if !exists {
		return
	}

	// Stop the recorder
	if c.recorder != nil && inc.windowID != "" {
		c.recorder.StopRecording(inc.windowID)
	}

	// Clean up
	if inc.stopTimer != nil {
		inc.stopTimer.Stop()
	}
	delete(c.activeIncidents, alertID)

	log.Info().
		Str("alert_id", alertID).
		Str("window_id", inc.windowID).
		Msg("Incident coordinator: Stopped incident recording")
}

// OnAnomalyDetected is called when an anomaly is detected - starts focused recording
func (c *IncidentCoordinator) OnAnomalyDetected(resourceID, resourceType, metric string, severity string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running || !c.config.EnableRecorder || c.recorder == nil {
		return
	}

	// Create a pseudo-alert ID for the anomaly
	anomalyID := "anomaly-" + resourceID + "-" + metric

	// Check if we already have an active incident for this
	if _, exists := c.activeIncidents[anomalyID]; exists {
		return
	}

	// Check concurrent limit
	if len(c.activeIncidents) >= c.config.MaxConcurrent {
		return
	}

	// Start recording
	windowID := c.recorder.StartRecording(
		resourceID,
		"", // name
		resourceType,
		"anomaly",
		anomalyID,
	)

	// Track the active incident
	c.activeIncidents[anomalyID] = activeIncident{
		windowID:   windowID,
		resourceID: resourceID,
		startedAt:  time.Now(),
	}

	// Schedule auto-stop after post-duration (anomalies don't have "clear" events)
	timer := time.AfterFunc(c.config.PostDuration, func() {
		c.stopIncidentRecording(anomalyID)
	})
	c.activeIncidents[anomalyID] = activeIncident{
		windowID:   windowID,
		resourceID: resourceID,
		startedAt:  time.Now(),
		stopTimer:  timer,
	}

	log.Info().
		Str("resource_id", resourceID).
		Str("metric", metric).
		Str("severity", severity).
		Str("window_id", windowID).
		Msg("Incident coordinator: Started anomaly recording")
}

// GetActiveIncidentCount returns the number of active incidents being recorded
func (c *IncidentCoordinator) GetActiveIncidentCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.activeIncidents)
}

// GetRecordingWindowID returns the recording window ID for an alert
func (c *IncidentCoordinator) GetRecordingWindowID(alertID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if inc, exists := c.activeIncidents[alertID]; exists {
		return inc.windowID
	}
	return ""
}
