// Package updatedetection provides unified update detection across all Pulse-managed
// infrastructure types.
//
// The Manager coordinates update detection for Docker containers, receiving update status
// from Docker agents and checking registries on demand. It maintains an in-memory store
// of available updates and provides APIs for querying update status.
package updatedetection

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Manager coordinates update detection across all infrastructure types.
type Manager struct {
	store    *Store
	registry *RegistryChecker
	logger   zerolog.Logger
	mu       sync.RWMutex

	// Configuration
	enabled             bool
	checkInterval       time.Duration
	alertDelayHours     int
	enableDockerUpdates bool
}

// ManagerConfig holds configuration for the update detection manager.
type ManagerConfig struct {
	Enabled             bool          // Master switch for update detection
	CheckInterval       time.Duration // How often to check for updates (default 6h)
	AlertDelayHours     int           // Hours before alerting on a new update (default 24)
	EnableDockerUpdates bool          // Enable Docker image update detection
}

// DefaultManagerConfig returns sensible default configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		Enabled:             true,
		CheckInterval:       6 * time.Hour,
		AlertDelayHours:     24,
		EnableDockerUpdates: true,
	}
}

// NewManager creates a new update detection manager.
func NewManager(cfg ManagerConfig, logger zerolog.Logger) *Manager {
	return &Manager{
		store:               NewStore(),
		registry:            NewRegistryChecker(logger),
		logger:              logger.With().Str("component", "updatedetection").Logger(),
		enabled:             cfg.Enabled,
		checkInterval:       cfg.CheckInterval,
		alertDelayHours:     cfg.AlertDelayHours,
		enableDockerUpdates: cfg.EnableDockerUpdates,
	}
}

// ProcessDockerContainerUpdate processes an update status report from a Docker agent.
// This is called when the agent reports container data with update status.
func (m *Manager) ProcessDockerContainerUpdate(
	hostID string,
	containerID string,
	containerName string,
	image string,
	currentDigest string,
	updateStatus *ContainerUpdateStatus,
) {
	if !m.enabled || !m.enableDockerUpdates {
		return
	}

	if updateStatus == nil {
		// No update status provided by agent - we can still track the container
		// but won't know about updates yet
		return
	}

	if !updateStatus.UpdateAvailable {
		// No update available - remove any existing update entry
		m.store.DeleteUpdatesForResource(containerID)
		return
	}

	// Create or update the update entry
	updateID := "docker:" + hostID + ":" + containerID
	update := &UpdateInfo{
		ID:            updateID,
		ResourceID:    containerID,
		ResourceType:  "docker",
		ResourceName:  containerName,
		HostID:        hostID,
		Type:          UpdateTypeDockerImage,
		CurrentDigest: updateStatus.CurrentDigest,
		LatestDigest:  updateStatus.LatestDigest,
		LastChecked:   updateStatus.LastChecked,
		CurrentVersion: image,
	}

	if updateStatus.Error != "" {
		update.Error = updateStatus.Error
	}

	m.store.UpsertUpdate(update)

	m.logger.Debug().
		Str("container", containerName).
		Str("image", image).
		Str("hostID", hostID).
		Bool("hasUpdate", updateStatus.UpdateAvailable).
		Msg("Processed container update status")
}

// CheckImageUpdate checks a specific image for updates using the registry API.
// This can be called on demand from the server side.
func (m *Manager) CheckImageUpdate(ctx context.Context, image, currentDigest string) (*ImageUpdateInfo, error) {
	if !m.enabled {
		return nil, nil
	}

	return m.registry.CheckImageUpdate(ctx, image, currentDigest)
}

// GetUpdates returns all tracked updates, optionally filtered.
func (m *Manager) GetUpdates(filters UpdateFilters) []*UpdateInfo {
	all := m.store.GetAllUpdates()

	if filters.IsEmpty() {
		return all
	}

	result := make([]*UpdateInfo, 0)
	for _, update := range all {
		if filters.Matches(update) {
			result = append(result, update)
		}
	}
	return result
}

// GetUpdatesForHost returns all updates for a specific host.
func (m *Manager) GetUpdatesForHost(hostID string) []*UpdateInfo {
	return m.store.GetUpdatesForHost(hostID)
}

// GetUpdatesForResource returns the update for a specific resource.
func (m *Manager) GetUpdatesForResource(resourceID string) *UpdateInfo {
	return m.store.GetUpdatesForResource(resourceID)
}

// GetSummary returns aggregated update statistics by host.
func (m *Manager) GetSummary() map[string]*UpdateSummary {
	return m.store.GetSummary()
}

// GetTotalCount returns the total number of tracked updates.
func (m *Manager) GetTotalCount() int {
	return m.store.Count()
}

// DeleteUpdatesForHost removes all updates for a host (called when host is removed).
func (m *Manager) DeleteUpdatesForHost(hostID string) {
	m.store.DeleteUpdatesForHost(hostID)
}

// AddRegistryConfig adds registry authentication configuration.
func (m *Manager) AddRegistryConfig(cfg RegistryConfig) {
	m.registry.AddRegistryConfig(cfg)
}

// CleanupStale removes update entries that haven't been refreshed recently.
// This is called periodically to clean up stale entries from removed containers.
func (m *Manager) CleanupStale(maxAge time.Duration) int {
	all := m.store.GetAllUpdates()
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, update := range all {
		if update.LastChecked.Before(cutoff) {
			m.store.DeleteUpdate(update.ID)
			removed++
		}
	}

	if removed > 0 {
		m.logger.Info().Int("removed", removed).Msg("Cleaned up stale update entries")
	}

	return removed
}

// UpdateFilters allows filtering update queries.
type UpdateFilters struct {
	HostID       string     // Filter by host
	ResourceType string     // Filter by resource type (docker, lxc, vm, etc)
	UpdateType   UpdateType // Filter by update type
	Severity     UpdateSeverity
	HasError     *bool // Filter by error status
}

// IsEmpty returns true if no filters are set.
func (f *UpdateFilters) IsEmpty() bool {
	return f.HostID == "" && f.ResourceType == "" && f.UpdateType == "" && f.Severity == "" && f.HasError == nil
}

// Matches returns true if the update matches all set filters.
func (f *UpdateFilters) Matches(update *UpdateInfo) bool {
	if f.HostID != "" && update.HostID != f.HostID {
		return false
	}
	if f.ResourceType != "" && update.ResourceType != f.ResourceType {
		return false
	}
	if f.UpdateType != "" && update.Type != f.UpdateType {
		return false
	}
	if f.Severity != "" && update.Severity != f.Severity {
		return false
	}
	if f.HasError != nil {
		hasError := update.Error != ""
		if hasError != *f.HasError {
			return false
		}
	}
	return true
}

// Enabled returns whether update detection is enabled.
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled enables or disables update detection.
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// AlertDelayHours returns the configured delay before alerting on updates.
func (m *Manager) AlertDelayHours() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertDelayHours
}

// GetUpdatesReadyForAlert returns updates that have been pending for longer than the alert delay.
func (m *Manager) GetUpdatesReadyForAlert() []*UpdateInfo {
	m.mu.RLock()
	delay := time.Duration(m.alertDelayHours) * time.Hour
	m.mu.RUnlock()

	all := m.store.GetAllUpdates()
	cutoff := time.Now().Add(-delay)
	ready := make([]*UpdateInfo, 0)

	for _, update := range all {
		if update.FirstDetected.Before(cutoff) && update.Error == "" {
			ready = append(ready, update)
		}
	}

	return ready
}
