package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// MockStateProvider is a mock implementation of ai.StateProvider for API tests
type MockStateProvider struct {
	State models.StateSnapshot
}

func (m *MockStateProvider) GetState() models.StateSnapshot {
	return m.State
}
