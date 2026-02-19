// Package adapters provides adapter implementations to connect existing stores
// and services to the interfaces required by the new AI intelligence packages.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
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
type MetricsAdapter struct {
	stateProvider ai.StateProvider
}

// NewMetricsAdapter creates a new adapter for current metrics.
func NewMetricsAdapter(stateProvider ai.StateProvider) *MetricsAdapter {
	return &MetricsAdapter{stateProvider: stateProvider}
}

// GetMonitoredResourceIDs returns all resource IDs currently being monitored.
// This is used by the incident recorder to maintain pre-incident buffers for all resources.
func (a *MetricsAdapter) GetMonitoredResourceIDs() []string {
	if a.stateProvider == nil {
		return nil
	}

	state := a.stateProvider.GetState()
	var ids []string

	// Collect VM IDs
	for _, vm := range state.VMs {
		ids = append(ids, vm.ID)
	}

	// Collect container IDs
	for _, ct := range state.Containers {
		ids = append(ids, ct.ID)
	}

	// Collect node IDs
	for _, node := range state.Nodes {
		ids = append(ids, node.ID)
	}

	return ids
}

// GetCurrentMetrics returns current metrics for a resource.
func (a *MetricsAdapter) GetCurrentMetrics(resourceID string) (map[string]float64, error) {
	if a.stateProvider == nil {
		return nil, nil
	}

	state := a.stateProvider.GetState()

	metrics := make(map[string]float64)

	// Check VMs
	for _, vm := range state.VMs {
		if vm.ID == resourceID || fmt.Sprintf("%d", vm.VMID) == resourceID {
			metrics["cpu"] = vm.CPU
			metrics["memory"] = vm.Memory.Usage
			metrics["disk"] = vm.Disk.Usage
			metrics["netin"] = float64(vm.NetworkIn)
			metrics["netout"] = float64(vm.NetworkOut)
			metrics["diskread"] = float64(vm.DiskRead)
			metrics["diskwrite"] = float64(vm.DiskWrite)
			return metrics, nil
		}
	}

	// Check containers
	for _, ct := range state.Containers {
		if ct.ID == resourceID || fmt.Sprintf("%d", ct.VMID) == resourceID {
			metrics["cpu"] = ct.CPU
			metrics["memory"] = ct.Memory.Usage
			metrics["disk"] = ct.Disk.Usage
			metrics["netin"] = float64(ct.NetworkIn)
			metrics["netout"] = float64(ct.NetworkOut)
			metrics["diskread"] = float64(ct.DiskRead)
			metrics["diskwrite"] = float64(ct.DiskWrite)
			return metrics, nil
		}
	}

	// Check nodes
	for _, node := range state.Nodes {
		if node.ID == resourceID || node.Name == resourceID {
			metrics["cpu"] = node.CPU
			metrics["memory"] = node.Memory.Usage
			metrics["disk"] = node.Disk.Usage
			return metrics, nil
		}
	}

	// Check storage
	for _, storage := range state.Storage {
		if storage.ID == resourceID || storage.Name == resourceID {
			metrics["disk"] = storage.Usage
			metrics["used"] = float64(storage.Used)
			metrics["total"] = float64(storage.Total)
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

	return json.Unmarshal(data, &s.entries)
}

// Verify interfaces are implemented
var (
	_ forecast.DataProvider       = (*ForecastDataAdapter)(nil)
	_ remediation.CommandExecutor = (*CommandExecutorAdapter)(nil)
)
