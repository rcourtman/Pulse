package updates

import (
	"context"
	"fmt"
	"time"
)

// MockUpdater simulates update plans for mock/demo environments.
type MockUpdater struct{}

// NewMockUpdater creates an updater that simulates update operations.
func NewMockUpdater() *MockUpdater {
	return &MockUpdater{}
}

func (u *MockUpdater) SupportsApply() bool {
	return true
}

func (u *MockUpdater) GetDeploymentType() string {
	return "mock"
}

func (u *MockUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	return &UpdatePlan{
		CanAutoUpdate: true,
		Instructions: []string{
			"Simulated update flow (mock mode)",
			fmt.Sprintf("Pretend download of %s", request.Version),
			"Emit deterministic progress stages for integration tests",
		},
		Prerequisites: []string{
			"Mock mode enabled",
		},
		EstimatedTime:   "a few seconds",
		RequiresRoot:    false,
		RollbackSupport: true,
	}, nil
}

func (u *MockUpdater) Execute(ctx context.Context, request UpdateRequest, progressCb ProgressCallback) error {
	stages := []UpdateProgress{
		{Stage: "downloading", Progress: 10, Message: "Mock downloading update..."},
		{Stage: "verifying", Progress: 30, Message: "Mock verifying download..."},
		{Stage: "extracting", Progress: 50, Message: "Mock extracting files..."},
		{Stage: "backing-up", Progress: 70, Message: "Mock backing up data..."},
		{Stage: "applying", Progress: 85, Message: "Mock applying update..."},
		{Stage: "completed", Progress: 100, Message: "Mock update complete", IsComplete: true},
	}

	for _, stage := range stages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			progressCb(stage)
			time.Sleep(200 * time.Millisecond)
		}
	}

	return nil
}

func (u *MockUpdater) Rollback(ctx context.Context, eventID string) error {
	// Nothing to rollback in mock mode
	return nil
}
