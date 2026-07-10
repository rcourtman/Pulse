package updates

import (
	"context"
	"fmt"
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
