// Package proxmox provides Proxmox-aware event correlation capabilities.
// It correlates infrastructure operations (migrations, backups, HA events) with metric anomalies.
package proxmox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ProxmoxEventType represents types of Proxmox operations
type ProxmoxEventType string

const (
	EventMigrationStart ProxmoxEventType = "migration_start"
	EventMigrationEnd   ProxmoxEventType = "migration_end"
	EventBackupStart    ProxmoxEventType = "backup_start"
	EventBackupEnd      ProxmoxEventType = "backup_end"
	EventSnapshotCreate ProxmoxEventType = "snapshot_create"
	EventSnapshotDelete ProxmoxEventType = "snapshot_delete"
	EventHAFailover     ProxmoxEventType = "ha_failover"
	EventHAMigration    ProxmoxEventType = "ha_migration"
	EventClusterJoin    ProxmoxEventType = "cluster_join"
	EventClusterLeave   ProxmoxEventType = "cluster_leave"
	EventStorageOnline  ProxmoxEventType = "storage_online"
	EventStorageOffline ProxmoxEventType = "storage_offline"
	EventNodeReboot     ProxmoxEventType = "node_reboot"
	EventVMCreate       ProxmoxEventType = "vm_create"
	EventVMDestroy      ProxmoxEventType = "vm_destroy"
	EventVMStart        ProxmoxEventType = "vm_start"
	EventVMStop         ProxmoxEventType = "vm_stop"
)

// ProxmoxEvent represents a Proxmox infrastructure event
type ProxmoxEvent struct {
	ID           string                 `json:"id"`
	Type         ProxmoxEventType       `json:"type"`
	Timestamp    time.Time              `json:"timestamp"`
	Node         string                 `json:"node,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	ResourceName string                 `json:"resource_name,omitempty"`
	ResourceType string                 `json:"resource_type,omitempty"` // vm, container, storage
	TargetNode   string                 `json:"target_node,omitempty"`   // For migrations
	Storage      string                 `json:"storage,omitempty"`       // For backup/snapshot ops
	TaskID       string                 `json:"task_id,omitempty"`       // Proxmox task ID
	Status       string                 `json:"status,omitempty"`        // success, failed, running
	Duration     time.Duration          `json:"duration,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// MetricAnomaly represents a detected metric anomaly
type MetricAnomaly struct {
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name,omitempty"`
	Metric       string    `json:"metric"`
	Value        float64   `json:"value"`
	Baseline     float64   `json:"baseline"`
	Deviation    float64   `json:"deviation"` // How many std devs from baseline
	Timestamp    time.Time `json:"timestamp"`
}

// EventCorrelation represents a detected correlation between a Proxmox event and anomalies
type EventCorrelation struct {
	ID                string          `json:"id"`
	Event             ProxmoxEvent    `json:"event"`
	Anomalies         []MetricAnomaly `json:"anomalies"`
	Explanation       string          `json:"explanation"`
	Confidence        float64         `json:"confidence"`
	ImpactedResources []string        `json:"impacted_resources"`
	CreatedAt         time.Time       `json:"created_at"`
}

// OperationWindow tracks ongoing operations and their expected impact
type OperationWindow struct {
	EventID           string           `json:"event_id"`
	EventType         ProxmoxEventType `json:"event_type"`
	StartTime         time.Time        `json:"start_time"`
	ExpectedEnd       time.Time        `json:"expected_end"`
	AffectedResources []string         `json:"affected_resources"`
	ExpectedMetrics   []string         `json:"expected_metrics"` // Metrics expected to be affected
}

// EventCorrelatorConfig configures the event correlator
type EventCorrelatorConfig struct {
	DataDir           string
	CorrelationWindow time.Duration // How long after event to look for anomalies
	MaxEvents         int           // Maximum events to keep
	MaxCorrelations   int           // Maximum correlations to keep
	RetentionDays     int           // How long to keep data
}

// DefaultEventCorrelatorConfig returns sensible defaults
func DefaultEventCorrelatorConfig() EventCorrelatorConfig {
	return EventCorrelatorConfig{
		CorrelationWindow: 15 * time.Minute,
		MaxEvents:         5000,
		MaxCorrelations:   1000,
		RetentionDays:     30,
	}
}

// EventCorrelator correlates Proxmox events with metric anomalies
type EventCorrelator struct {
	mu sync.RWMutex

	config EventCorrelatorConfig

	// Event storage
	events []ProxmoxEvent

	// Active operation windows
	activeWindows map[string]*OperationWindow

	// Detected correlations
	correlations []EventCorrelation

	// Persistence
	dataDir string
}

// NewEventCorrelator creates a new event correlator
func NewEventCorrelator(cfg EventCorrelatorConfig) *EventCorrelator {
	if cfg.CorrelationWindow <= 0 {
		cfg.CorrelationWindow = 15 * time.Minute
	}
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 5000
	}
	if cfg.MaxCorrelations <= 0 {
		cfg.MaxCorrelations = 1000
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}

	correlator := &EventCorrelator{
		config:        cfg,
		events:        make([]ProxmoxEvent, 0),
		activeWindows: make(map[string]*OperationWindow),
		correlations:  make([]EventCorrelation, 0),
		dataDir:       cfg.DataDir,
	}

	// Load from disk
	if cfg.DataDir != "" {
		if err := correlator.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load Proxmox event data from disk")
		}
	}

	return correlator
}

// RecordEvent records a Proxmox infrastructure event
func (c *EventCorrelator) RecordEvent(event ProxmoxEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	c.events = append(c.events, event)

	// Create operation window for ongoing operations
	if isOngoingOperation(event.Type) {
		window := c.createOperationWindow(event)
		c.activeWindows[event.ID] = window
	}

	// Update operation window when operation ends
	if isEndOperation(event.Type) {
		c.updateOperationWindow(event)
	}

	// Trim old events
	c.trimEvents()

	log.Debug().
		Str("event_id", event.ID).
		Str("type", string(event.Type)).
		Str("resource", event.ResourceID).
		Msg("Recorded Proxmox event")

	go c.saveIfDirty()
}

// createOperationWindow creates an operation window for an ongoing operation
func (c *EventCorrelator) createOperationWindow(event ProxmoxEvent) *OperationWindow {
	window := &OperationWindow{
		EventID:     event.ID,
		EventType:   event.Type,
		StartTime:   event.Timestamp,
		ExpectedEnd: event.Timestamp.Add(estimateOperationDuration(event.Type)),
	}

	// Determine affected resources based on event type
	switch event.Type {
	case EventMigrationStart:
		window.AffectedResources = []string{event.ResourceID, event.Node, event.TargetNode}
		window.ExpectedMetrics = []string{"cpu", "memory", "network", "io"}
	case EventBackupStart:
		window.AffectedResources = []string{event.ResourceID, event.Storage}
		window.ExpectedMetrics = []string{"io", "disk", "cpu"}
	case EventSnapshotCreate, EventSnapshotDelete:
		window.AffectedResources = []string{event.ResourceID, event.Storage}
		window.ExpectedMetrics = []string{"io", "disk"}
	case EventHAFailover, EventHAMigration:
		window.AffectedResources = []string{event.ResourceID, event.Node, event.TargetNode}
		window.ExpectedMetrics = []string{"cpu", "memory", "network", "io"}
	}

	return window
}

// updateOperationWindow updates when an operation completes
func (c *EventCorrelator) updateOperationWindow(event ProxmoxEvent) {
	// Find matching start event
	for _, e := range c.events {
		if matchesEndEvent(e, event) {
			if window, ok := c.activeWindows[e.ID]; ok {
				window.ExpectedEnd = event.Timestamp
				delete(c.activeWindows, e.ID)
			}
			break
		}
	}
}

// RecordAnomaly records a metric anomaly and checks for correlations
func (c *EventCorrelator) RecordAnomaly(anomaly MetricAnomaly) *EventCorrelation {
	c.mu.Lock()
	defer c.mu.Unlock()

	if anomaly.Timestamp.IsZero() {
		anomaly.Timestamp = time.Now()
	}

	// Check for correlation with recent events
	correlation := c.findCorrelation(anomaly)
	if correlation != nil {
		c.correlations = append(c.correlations, *correlation)
		c.trimCorrelations()

		log.Info().
			Str("correlation_id", correlation.ID).
			Str("event_type", string(correlation.Event.Type)).
			Str("anomaly_resource", anomaly.ResourceID).
			Msg("Detected Proxmox event correlation")

		go c.saveIfDirty()
		return correlation
	}

	return nil
}

// findCorrelation looks for events that might explain an anomaly
func (c *EventCorrelator) findCorrelation(anomaly MetricAnomaly) *EventCorrelation {
	cutoff := anomaly.Timestamp.Add(-c.config.CorrelationWindow)

	var bestEvent *ProxmoxEvent
	var bestConfidence float64

	for i := len(c.events) - 1; i >= 0; i-- {
		event := c.events[i]

		// Skip events outside the correlation window
		if event.Timestamp.Before(cutoff) {
			break
		}

		// Check if this event could explain the anomaly
		confidence := c.calculateCorrelationConfidence(event, anomaly)
		if confidence > bestConfidence {
			eventCopy := event
			bestEvent = &eventCopy
			bestConfidence = confidence
		}
	}

	if bestEvent != nil && bestConfidence >= 0.5 {
		correlation := &EventCorrelation{
			ID:                generateCorrelationID(),
			Event:             *bestEvent,
			Anomalies:         []MetricAnomaly{anomaly},
			Explanation:       generateExplanation(*bestEvent, anomaly),
			Confidence:        bestConfidence,
			ImpactedResources: []string{anomaly.ResourceID},
			CreatedAt:         time.Now(),
		}
		return correlation
	}

	return nil
}

// calculateCorrelationConfidence calculates how likely an event caused an anomaly
func (c *EventCorrelator) calculateCorrelationConfidence(event ProxmoxEvent, anomaly MetricAnomaly) float64 {
	var confidence float64

	// Check if event affects the anomaly resource
	directlyAffected := event.ResourceID == anomaly.ResourceID ||
		event.Node == anomaly.ResourceID ||
		event.TargetNode == anomaly.ResourceID ||
		event.Storage == anomaly.ResourceID

	if directlyAffected {
		confidence += 0.4
	}

	// Check if the metric type matches expected impact
	expectedMetrics := getExpectedMetrics(event.Type)
	for _, expected := range expectedMetrics {
		if expected == anomaly.Metric {
			confidence += 0.3
			break
		}
	}

	// Time proximity (closer = higher confidence)
	timeDiff := anomaly.Timestamp.Sub(event.Timestamp)
	if timeDiff < 1*time.Minute {
		confidence += 0.3
	} else if timeDiff < 5*time.Minute {
		confidence += 0.2
	} else if timeDiff < 10*time.Minute {
		confidence += 0.1
	}

	// Check active operation windows
	for _, window := range c.activeWindows {
		if containsString(window.AffectedResources, anomaly.ResourceID) {
			if anomaly.Timestamp.After(window.StartTime) && anomaly.Timestamp.Before(window.ExpectedEnd) {
				confidence += 0.2
				break
			}
		}
	}

	return minFloat(confidence, 1.0)
}

// GetRecentEvents returns recent Proxmox events
func (c *EventCorrelator) GetRecentEvents(duration time.Duration) []ProxmoxEvent {
	return c.GetRecentEventsWithLimit(duration, 0)
}

// GetRecentEventsWithLimit returns recent Proxmox events with an optional limit
func (c *EventCorrelator) GetRecentEventsWithLimit(duration time.Duration, limit int) []ProxmoxEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var result []ProxmoxEvent

	for i := len(c.events) - 1; i >= 0; i-- {
		if c.events[i].Timestamp.Before(cutoff) {
			break
		}
		result = append(result, c.events[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetEventsForResource returns events affecting a specific resource
func (c *EventCorrelator) GetEventsForResource(resourceID string, limit int) []ProxmoxEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []ProxmoxEvent
	for i := len(c.events) - 1; i >= 0 && (limit <= 0 || len(result) < limit); i-- {
		event := c.events[i]
		if event.ResourceID == resourceID || event.Node == resourceID ||
			event.TargetNode == resourceID || event.Storage == resourceID {
			result = append(result, event)
		}
	}

	return result
}

// GetCorrelations returns detected correlations
func (c *EventCorrelator) GetCorrelations(limit int) []EventCorrelation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.correlations) {
		limit = len(c.correlations)
	}

	// Return most recent
	start := len(c.correlations) - limit
	if start < 0 {
		start = 0
	}

	result := make([]EventCorrelation, limit)
	copy(result, c.correlations[start:])
	return result
}

// GetCorrelationsForResource returns correlations involving a resource
func (c *EventCorrelator) GetCorrelationsForResource(resourceID string) []EventCorrelation {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []EventCorrelation
	for _, corr := range c.correlations {
		if containsString(corr.ImpactedResources, resourceID) {
			result = append(result, corr)
		}
	}

	return result
}

// GetActiveOperations returns currently active operations
func (c *EventCorrelator) GetActiveOperations() []OperationWindow {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var result []OperationWindow

	for _, window := range c.activeWindows {
		if window.ExpectedEnd.After(now) {
			result = append(result, *window)
		}
	}

	return result
}

// FormatForPatrol formats recent events for AI patrol context
func (c *EventCorrelator) FormatForPatrol(duration time.Duration) string {
	events := c.GetRecentEvents(duration)
	if len(events) == 0 {
		return ""
	}

	result := "\n## Recent Proxmox Operations\n"
	result += fmt.Sprintf("Operations in the last %v:\n\n", duration)

	for _, event := range events {
		timestamp := event.Timestamp.Format("15:04:05")
		result += fmt.Sprintf("- %s: %s", timestamp, formatEventType(event.Type))

		if event.ResourceName != "" {
			result += " for " + event.ResourceName
		} else if event.ResourceID != "" {
			result += " for " + event.ResourceID
		}

		if event.Node != "" {
			result += " on " + event.Node
		}

		if event.TargetNode != "" {
			result += " -> " + event.TargetNode
		}

		if event.Storage != "" {
			result += " (storage: " + event.Storage + ")"
		}

		if event.Status != "" {
			result += " [" + event.Status + "]"
		}

		result += "\n"
	}

	// Add active operations
	activeOps := c.GetActiveOperations()
	if len(activeOps) > 0 {
		result += "\n### Currently Active Operations\n"
		for _, op := range activeOps {
			result += fmt.Sprintf("- %s (started %s, expected to complete by %s)\n",
				formatEventType(op.EventType),
				op.StartTime.Format("15:04:05"),
				op.ExpectedEnd.Format("15:04:05"))
		}
	}

	// Add recent correlations
	correlations := c.GetCorrelations(5)
	if len(correlations) > 0 {
		result += "\n### Detected Correlations\n"
		result += "Recent infrastructure events correlated with anomalies:\n"
		for _, corr := range correlations {
			result += "- " + corr.Explanation + "\n"
		}
	}

	return result
}

// FormatForResource formats event context for a specific resource
func (c *EventCorrelator) FormatForResource(resourceID string) string {
	events := c.GetEventsForResource(resourceID, 10)
	correlations := c.GetCorrelationsForResource(resourceID)

	if len(events) == 0 && len(correlations) == 0 {
		return ""
	}

	result := "\n## Proxmox Operations for Resource\n"

	if len(events) > 0 {
		result += "Recent operations:\n"
		for _, event := range events {
			result += fmt.Sprintf("- %s: %s", event.Timestamp.Format("Jan 2 15:04"), formatEventType(event.Type))
			if event.Status != "" {
				result += " [" + event.Status + "]"
			}
			result += "\n"
		}
	}

	if len(correlations) > 0 {
		result += "\nCorrelated events:\n"
		for _, corr := range correlations {
			result += "- " + corr.Explanation + "\n"
		}
	}

	return result
}

// trimEvents removes old events
func (c *EventCorrelator) trimEvents() {
	cutoff := time.Now().AddDate(0, 0, -c.config.RetentionDays)

	kept := make([]ProxmoxEvent, 0, len(c.events))
	for _, event := range c.events {
		if event.Timestamp.After(cutoff) {
			kept = append(kept, event)
		}
	}

	if len(kept) > c.config.MaxEvents {
		kept = kept[len(kept)-c.config.MaxEvents:]
	}

	c.events = kept
}

// trimCorrelations removes old correlations
func (c *EventCorrelator) trimCorrelations() {
	cutoff := time.Now().AddDate(0, 0, -c.config.RetentionDays)

	kept := make([]EventCorrelation, 0, len(c.correlations))
	for _, corr := range c.correlations {
		if corr.CreatedAt.After(cutoff) {
			kept = append(kept, corr)
		}
	}

	if len(kept) > c.config.MaxCorrelations {
		kept = kept[len(kept)-c.config.MaxCorrelations:]
	}

	c.correlations = kept
}

// saveIfDirty saves to disk if there are changes
func (c *EventCorrelator) saveIfDirty() {
	if err := c.saveToDisk(); err != nil {
		log.Warn().Err(err).Msg("Failed to save Proxmox event data")
	}
}

// saveToDisk persists data
func (c *EventCorrelator) saveToDisk() error {
	if c.dataDir == "" {
		return nil
	}

	c.mu.RLock()
	data := struct {
		Events       []ProxmoxEvent     `json:"events"`
		Correlations []EventCorrelation `json:"correlations"`
	}{
		Events:       c.events,
		Correlations: c.correlations,
	}
	c.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(c.dataDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(c.dataDir, "proxmox_events.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads data
func (c *EventCorrelator) loadFromDisk() error {
	if c.dataDir == "" {
		return nil
	}

	path := filepath.Join(c.dataDir, "proxmox_events.json")
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data struct {
		Events       []ProxmoxEvent     `json:"events"`
		Correlations []EventCorrelation `json:"correlations"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	c.events = data.Events
	c.correlations = data.Correlations

	c.trimEvents()
	c.trimCorrelations()

	return nil
}

// Helper functions

var eventIDCounter, correlationIDCounter int64

func generateEventID() string {
	eventIDCounter++
	return fmt.Sprintf("pve-%s-%d", time.Now().Format("20060102150405"), eventIDCounter%1000)
}

func generateCorrelationID() string {
	correlationIDCounter++
	return fmt.Sprintf("corr-%s-%d", time.Now().Format("20060102150405"), correlationIDCounter%1000)
}

func isOngoingOperation(eventType ProxmoxEventType) bool {
	switch eventType {
	case EventMigrationStart, EventBackupStart, EventSnapshotCreate, EventSnapshotDelete:
		return true
	default:
		return false
	}
}

func isEndOperation(eventType ProxmoxEventType) bool {
	switch eventType {
	case EventMigrationEnd, EventBackupEnd:
		return true
	default:
		return false
	}
}

func matchesEndEvent(start, end ProxmoxEvent) bool {
	if start.ResourceID != end.ResourceID {
		return false
	}

	switch start.Type {
	case EventMigrationStart:
		return end.Type == EventMigrationEnd
	case EventBackupStart:
		return end.Type == EventBackupEnd
	default:
		return false
	}
}

func estimateOperationDuration(eventType ProxmoxEventType) time.Duration {
	switch eventType {
	case EventMigrationStart:
		return 30 * time.Minute
	case EventBackupStart:
		return 2 * time.Hour
	case EventSnapshotCreate, EventSnapshotDelete:
		return 10 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func getExpectedMetrics(eventType ProxmoxEventType) []string {
	switch eventType {
	case EventMigrationStart, EventMigrationEnd:
		return []string{"cpu", "memory", "network", "io"}
	case EventBackupStart, EventBackupEnd:
		return []string{"io", "disk", "cpu"}
	case EventSnapshotCreate, EventSnapshotDelete:
		return []string{"io", "disk"}
	case EventHAFailover, EventHAMigration:
		return []string{"cpu", "memory", "network"}
	default:
		return []string{}
	}
}

func generateExplanation(event ProxmoxEvent, anomaly MetricAnomaly) string {
	eventDesc := formatEventType(event.Type)
	resourceName := event.ResourceName
	if resourceName == "" {
		resourceName = event.ResourceID
	}

	switch event.Type {
	case EventMigrationStart, EventMigrationEnd:
		return fmt.Sprintf("Migration of %s caused %s spike on %s", resourceName, anomaly.Metric, anomaly.ResourceID)
	case EventBackupStart, EventBackupEnd:
		return fmt.Sprintf("Backup of %s saturated storage, causing %s latency on %s", resourceName, anomaly.Metric, anomaly.ResourceID)
	case EventSnapshotCreate, EventSnapshotDelete:
		return fmt.Sprintf("Snapshot operation on %s caused %s spike on %s", resourceName, anomaly.Metric, anomaly.ResourceID)
	case EventHAFailover:
		return fmt.Sprintf("HA failover caused %s disruption on %s", anomaly.Metric, anomaly.ResourceID)
	default:
		return fmt.Sprintf("%s caused %s anomaly on %s", eventDesc, anomaly.Metric, anomaly.ResourceID)
	}
}

func formatEventType(eventType ProxmoxEventType) string {
	switch eventType {
	case EventMigrationStart:
		return "Migration started"
	case EventMigrationEnd:
		return "Migration completed"
	case EventBackupStart:
		return "Backup started"
	case EventBackupEnd:
		return "Backup completed"
	case EventSnapshotCreate:
		return "Snapshot created"
	case EventSnapshotDelete:
		return "Snapshot deleted"
	case EventHAFailover:
		return "HA failover"
	case EventHAMigration:
		return "HA migration"
	case EventClusterJoin:
		return "Node joined cluster"
	case EventClusterLeave:
		return "Node left cluster"
	case EventStorageOnline:
		return "Storage online"
	case EventStorageOffline:
		return "Storage offline"
	case EventNodeReboot:
		return "Node reboot"
	case EventVMCreate:
		return "VM created"
	case EventVMDestroy:
		return "VM destroyed"
	case EventVMStart:
		return "VM started"
	case EventVMStop:
		return "VM stopped"
	default:
		return string(eventType)
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SortEventsByTimestamp sorts events by timestamp (newest first)
func SortEventsByTimestamp(events []ProxmoxEvent) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
}
