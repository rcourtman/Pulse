// Package adapters provides adapter implementations to connect existing stores
// and services to the interfaces required by the new AI intelligence packages.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

// ForecastDataAdapter adapts monitoring.MetricsHistory to the forecast.DataProvider interface.
// This allows the forecast service to access historical metric data.
type ForecastDataAdapter struct {
	history *monitoring.MetricsHistory
}

// NewForecastDataAdapter creates a new adapter for forecast data.
func NewForecastDataAdapter(history *monitoring.MetricsHistory) *ForecastDataAdapter {
	if history == nil {
		return nil
	}
	return &ForecastDataAdapter{history: history}
}

// GetMetricHistory returns historical metric data for forecasting.
// It supports common metrics: cpu, memory, disk, netin, netout.
func (a *ForecastDataAdapter) GetMetricHistory(resourceID, metric string, from, to time.Time) ([]forecast.MetricDataPoint, error) {
	if a.history == nil {
		return nil, nil
	}

	duration := to.Sub(from)

	// Try guest metrics first (VMs and containers)
	points := a.history.GetGuestMetrics(resourceID, metric, duration)
	if len(points) == 0 {
		// Try node metrics if not found as guest
		points = a.history.GetNodeMetrics(resourceID, metric, duration)
	}

	if len(points) == 0 {
		// Try storage metrics
		allStorageMetrics := a.history.GetAllStorageMetrics(resourceID, duration)
		if storagePoints, ok := allStorageMetrics[metric]; ok {
			points = storagePoints
		}
	}

	result := make([]forecast.MetricDataPoint, 0, len(points))
	for _, p := range points {
		// Filter by time window
		if (p.Timestamp.Equal(from) || p.Timestamp.After(from)) &&
			(p.Timestamp.Equal(to) || p.Timestamp.Before(to)) {
			result = append(result, forecast.MetricDataPoint{
				Timestamp: p.Timestamp,
				Value:     p.Value,
			})
		}
	}

	return result, nil
}

// MetricsAdapter provides current metrics for resources.
// It implements metrics.MetricsProvider for the incident recorder.
// Uses ReadState as the sole data source (SRC-03m migration).
type MetricsAdapter struct {
	readState unifiedresources.ReadState
}

// NewMetricsAdapter creates a new adapter for current metrics.
// ReadState is the sole data source for both GetMonitoredResourceIDs and
// GetCurrentMetrics. Returns nil if readState is nil.
func NewMetricsAdapter(readState unifiedresources.ReadState) *MetricsAdapter {
	if readState == nil {
		return nil
	}
	return &MetricsAdapter{readState: readState}
}

// GetMonitoredResourceIDs returns all resource IDs currently being monitored.
// This is used by the incident recorder to maintain pre-incident buffers for all resources.
// Returns both unified IDs and Proxmox source IDs so that pre-incident buffers
// are keyed by both (alert-triggered recordings use source IDs).
func (a *MetricsAdapter) GetMonitoredResourceIDs() []string {
	var ids []string
	for _, vm := range a.readState.VMs() {
		ids = append(ids, vm.ID())
		if sid := vm.SourceID(); sid != "" && sid != vm.ID() {
			ids = append(ids, sid)
		}
	}
	for _, ct := range a.readState.Containers() {
		ids = append(ids, ct.ID())
		if sid := ct.SourceID(); sid != "" && sid != ct.ID() {
			ids = append(ids, sid)
		}
	}
	for _, node := range a.readState.Nodes() {
		ids = append(ids, node.ID())
		if sid := node.SourceID(); sid != "" && sid != node.ID() {
			ids = append(ids, sid)
		}
	}
	return ids
}

// GetCurrentMetrics returns current metrics for a resource.
// Matches by unified ID, Proxmox source ID, VMID string, or name.
// CPU/memory/disk values are normalized to 0-100 percentage scale.
func (a *MetricsAdapter) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	metrics := make(map[string]float64)

	// Check VMs
	for _, vm := range a.readState.VMs() {
		if vm.ID() == resourceID || vm.SourceID() == resourceID || fmt.Sprintf("%d", vm.VMID()) == resourceID {
			metrics["cpu"] = vm.CPUPercent()
			metrics["memory"] = vm.MemoryPercent()
			metrics["disk"] = vm.DiskPercent()
			metrics["netin"] = vm.NetIn()
			metrics["netout"] = vm.NetOut()
			metrics["diskread"] = vm.DiskRead()
			metrics["diskwrite"] = vm.DiskWrite()
			return metrics, nil
		}
	}

	// Check containers
	for _, ct := range a.readState.Containers() {
		if ct.ID() == resourceID || ct.SourceID() == resourceID || fmt.Sprintf("%d", ct.VMID()) == resourceID {
			metrics["cpu"] = ct.CPUPercent()
			metrics["memory"] = ct.MemoryPercent()
			metrics["disk"] = ct.DiskPercent()
			metrics["netin"] = ct.NetIn()
			metrics["netout"] = ct.NetOut()
			metrics["diskread"] = ct.DiskRead()
			metrics["diskwrite"] = ct.DiskWrite()
			return metrics, nil
		}
	}

	// Check nodes
	for _, node := range a.readState.Nodes() {
		if node.ID() == resourceID || node.SourceID() == resourceID || node.Name() == resourceID {
			metrics["cpu"] = node.CPUPercent()
			metrics["memory"] = node.MemoryPercent()
			metrics["disk"] = node.DiskPercent()
			return metrics, nil
		}
	}

	// Check storage
	for _, sp := range a.readState.StoragePools() {
		if sp.ID() == resourceID || sp.SourceID() == resourceID || sp.Name() == resourceID {
			metrics["disk"] = sp.DiskPercent()
			metrics["used"] = float64(sp.DiskUsed())
			metrics["total"] = float64(sp.DiskTotal())
			return metrics, nil
		}
	}

	return metrics, nil
}

// CommandExecutorAdapter adapts the agent execution system to remediation.CommandExecutor.
// This allows the remediation engine to execute commands on targets.
type CommandExecutorAdapter struct {
	// For now, this is a placeholder. In a full implementation, this would
	// use the agentexec package to run commands on Proxmox nodes/guests.
	// Security note: Command execution should be carefully controlled.
}

// NewCommandExecutorAdapter creates a new adapter for command execution.
func NewCommandExecutorAdapter() *CommandExecutorAdapter {
	return &CommandExecutorAdapter{}
}

// Execute runs a command on a target.
// Currently returns an error since direct command execution is not yet implemented.
// Full implementation would route to appropriate execution backend (SSH, PVE API, etc.)
func (a *CommandExecutorAdapter) Execute(ctx context.Context, target, command string) (string, error) {
	// For safety, command execution is disabled by default.
	// This would need to be implemented with proper safety checks and routing.
	return "", &CommandExecutionDisabledError{
		Target:  target,
		Command: command,
	}
}

// CommandExecutionDisabledError indicates command execution is not enabled.
type CommandExecutionDisabledError struct {
	Target  string
	Command string
}

func (e *CommandExecutionDisabledError) Error() string {
	return "command execution is disabled - commands must be run manually"
}

// IncidentRecorderMCPAdapter adapts metrics.IncidentRecorder to tools.IncidentRecorderProvider
type IncidentRecorderMCPAdapter struct {
	recorder IncidentRecorderSource
}

// IncidentRecorderSource defines what we need from an incident recorder
type IncidentRecorderSource interface {
	GetWindowsForResource(resourceID string, limit int) []*IncidentWindowData
	GetWindow(windowID string) *IncidentWindowData
}

// IncidentWindowData represents incident window data
type IncidentWindowData struct {
	ID           string
	ResourceID   string
	ResourceName string
	ResourceType string
	TriggerType  string
	TriggerID    string
	StartTime    time.Time
	EndTime      *time.Time
	Status       string
	DataPoints   []IncidentDataPointData
	Summary      *IncidentSummaryData
}

// IncidentDataPointData represents a single data point
type IncidentDataPointData struct {
	Timestamp time.Time
	Metrics   map[string]float64
}

// IncidentSummaryData provides summary statistics
type IncidentSummaryData struct {
	Duration   time.Duration
	DataPoints int
	Peaks      map[string]float64
	Lows       map[string]float64
	Averages   map[string]float64
	Changes    map[string]float64
}

// NewIncidentRecorderMCPAdapter creates a new incident recorder adapter
func NewIncidentRecorderMCPAdapter(recorder IncidentRecorderSource) *IncidentRecorderMCPAdapter {
	return &IncidentRecorderMCPAdapter{recorder: recorder}
}

// GetWindowsForResource returns incident windows for a resource
func (a *IncidentRecorderMCPAdapter) GetWindowsForResource(resourceID string, limit int) []*IncidentWindowData {
	if a.recorder == nil {
		return nil
	}
	return a.recorder.GetWindowsForResource(resourceID, limit)
}

// GetWindow returns a specific incident window
func (a *IncidentRecorderMCPAdapter) GetWindow(windowID string) *IncidentWindowData {
	if a.recorder == nil {
		return nil
	}
	return a.recorder.GetWindow(windowID)
}

// EventCorrelatorMCPAdapter adapts proxmox.EventCorrelator to tools.EventCorrelatorProvider
type EventCorrelatorMCPAdapter struct {
	correlator EventCorrelatorSource
}

// EventCorrelatorSource defines what we need from an event correlator
type EventCorrelatorSource interface {
	GetCorrelationsForResource(resourceID string) []EventCorrelationData
	GetEventsForResource(resourceID string, limit int) []ProxmoxEventData
}

// EventCorrelationData represents a correlation
type EventCorrelationData struct {
	ID                string
	Explanation       string
	Confidence        float64
	ImpactedResources []string
	CreatedAt         time.Time
}

// ProxmoxEventData represents a Proxmox event
type ProxmoxEventData struct {
	ID           string
	Type         string
	Timestamp    time.Time
	Node         string
	ResourceID   string
	ResourceName string
	ResourceType string
	Status       string
}

// NewEventCorrelatorMCPAdapter creates a new event correlator adapter
func NewEventCorrelatorMCPAdapter(correlator EventCorrelatorSource) *EventCorrelatorMCPAdapter {
	return &EventCorrelatorMCPAdapter{correlator: correlator}
}

// GetCorrelationsForResource returns correlated events for a resource
func (a *EventCorrelatorMCPAdapter) GetCorrelationsForResource(resourceID string, window time.Duration) []EventCorrelationData {
	if a.correlator == nil {
		return nil
	}
	return a.correlator.GetCorrelationsForResource(resourceID)
}

// KnowledgeStore provides persistent storage for resource notes
type KnowledgeStore struct {
	mu      sync.RWMutex
	entries map[string][]KnowledgeEntry
	dataDir string
}

// KnowledgeEntry represents a stored note
type KnowledgeEntry struct {
	ID         string
	ResourceID string
	Note       string
	Category   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func normalizeKnowledgeResourceID(resourceID string) string {
	return strings.TrimSpace(resourceID)
}

func isUnsupportedKnowledgeResourceID(resourceID string) bool {
	return unifiedresources.IsUnsupportedLegacyResourceIDAlias(resourceID)
}

// NewKnowledgeStore creates a new knowledge store
func NewKnowledgeStore(dataDir string) *KnowledgeStore {
	store := &KnowledgeStore{
		entries: make(map[string][]KnowledgeEntry),
		dataDir: dataDir,
	}
	if dataDir != "" {
		if err := store.loadFromDisk(); err != nil {
			log.Warn().Err(err).Str("data_dir", dataDir).Msg("ai.adapters.NewKnowledgeStore: failed to load knowledge store from disk")
		}
	}
	return store
}

// SaveNote saves a note about a resource
func (s *KnowledgeStore) SaveNote(resourceID, note, category string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resourceID = normalizeKnowledgeResourceID(resourceID)
	if isUnsupportedKnowledgeResourceID(resourceID) {
		return fmt.Errorf("unsupported resource ID %q", resourceID)
	}

	entry := KnowledgeEntry{
		ID:         fmt.Sprintf("note-%d", time.Now().UnixNano()),
		ResourceID: resourceID,
		Note:       note,
		Category:   category,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	s.entries[resourceID] = append(s.entries[resourceID], entry)

	if s.dataDir != "" {
		go s.saveToDisk() // Async save
	}

	return nil
}

// GetKnowledge retrieves notes about a resource
func (s *KnowledgeStore) GetKnowledge(resourceID string, category string) []KnowledgeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resourceID = normalizeKnowledgeResourceID(resourceID)
	if isUnsupportedKnowledgeResourceID(resourceID) {
		log.Warn().
			Str("resource_id", resourceID).
			Msg("ai.adapters.KnowledgeStore.GetKnowledge: ignoring unsupported resource ID")
		return nil
	}

	entries := s.entries[resourceID]
	if category == "" {
		return entries
	}

	// Filter by category
	filtered := make([]KnowledgeEntry, 0)
	for _, e := range entries {
		if e.Category == category {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (s *KnowledgeStore) saveToDisk() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dataDir == "" {
		return
	}

	data, err := json.Marshal(s.entries)
	if err != nil {
		log.Warn().Err(err).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to marshal entries")
		return
	}

	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return
	}

	path := filepath.Join(s.dataDir, "knowledge_store.json")
	// Use atomic write (temp file + fsync + rename) to prevent empty reads after a crash.
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Warn().Err(err).Str("path", tmp).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to open temp store file")
		return
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		log.Warn().Err(err).Str("path", tmp).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to write temp store file")
		return
	}
	if err := f.Sync(); err != nil {
		f.Close()
		log.Warn().Err(err).Str("path", tmp).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to fsync temp store file")
		return
	}
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		log.Warn().Err(err).Str("from", tmp).Str("to", path).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to atomically replace store file")
		if removeErr := os.Remove(tmp); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Warn().Err(removeErr).Str("path", tmp).Msg("ai.adapters.KnowledgeStore.saveToDisk: failed to clean up temp store file")
		}
	}
}

func (s *KnowledgeStore) loadFromDisk() error {
	if s.dataDir == "" {
		return nil
	}

	path := filepath.Join(s.dataDir, "knowledge_store.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &s.entries); err != nil {
		return err
	}
	return nil
}

// Verify interfaces are implemented
var (
	_ forecast.DataProvider       = (*ForecastDataAdapter)(nil)
	_ aicontracts.CommandExecutor = (*CommandExecutorAdapter)(nil)
)
