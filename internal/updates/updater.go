package updates

import (
	"context"
	"fmt"
)

// UpdateRequest represents a request to update
type UpdateRequest struct {
	Version string
	Channel string
	Force   bool
	DryRun  bool
}

// UpdatePlan contains information about how an update will be performed
type UpdatePlan struct {
	CanAutoUpdate   bool     `json:"canAutoUpdate"`
	Instructions    []string `json:"instructions,omitempty"`
	Prerequisites   []string `json:"prerequisites,omitempty"`
	EstimatedTime   string   `json:"estimatedTime,omitempty"`
	RequiresRoot    bool     `json:"requiresRoot"`
	RollbackSupport bool     `json:"rollbackSupport"`
	DownloadURL     string   `json:"downloadUrl,omitempty"`
}

// UpdateProgress represents progress updates during an update
type UpdateProgress struct {
	Stage      string `json:"stage"`
	Progress   int    `json:"progress"` // 0-100
	Message    string `json:"message"`
	IsComplete bool   `json:"isComplete"`
	Error      string `json:"error,omitempty"`
}

// ProgressCallback is called during update execution
type ProgressCallback func(progress UpdateProgress)

// Updater defines the interface for deployment-specific update logic
type Updater interface {
	// SupportsApply returns true if this deployment type supports automated updates
	SupportsApply() bool

	// PrepareUpdate returns a plan describing how the update will be performed
	PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error)

	// Execute performs the update
	Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error

	// Rollback rolls back to a previous version using a backup
	Rollback(ctx context.Context, eventID string) error

	// GetDeploymentType returns the deployment type this updater handles
	GetDeploymentType() string
}

// UpdaterRegistry manages updaters for different deployment types
type UpdaterRegistry struct {
	updaters map[string]Updater
}

// NewUpdaterRegistry creates a new updater registry
func NewUpdaterRegistry() *UpdaterRegistry {
	return &UpdaterRegistry{
		updaters: make(map[string]Updater),
	}
}

// Register registers an updater for a deployment type
func (r *UpdaterRegistry) Register(deploymentType string, updater Updater) {
	r.updaters[deploymentType] = updater
}

// Get returns the updater for a deployment type
func (r *UpdaterRegistry) Get(deploymentType string) (Updater, error) {
	updater, ok := r.updaters[deploymentType]
	if !ok {
		return nil, fmt.Errorf("no updater registered for deployment type: %s", deploymentType)
	}
	return updater, nil
}

// GetAll returns all registered updaters
func (r *UpdaterRegistry) GetAll() map[string]Updater {
	return r.updaters
}
