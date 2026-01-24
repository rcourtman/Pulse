package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/require"
)

// MockStateProvider for initializing PatrolService
type MockStateProvider struct{}

func (m *MockStateProvider) GetState() models.StateSnapshot {
	return models.StateSnapshot{}
}

// MockOrchestrator for testing GetFixedCount integration
type MockOrchestrator struct {
	FixedCount int
}

func (m *MockOrchestrator) InvestigateFinding(ctx context.Context, finding *ai.InvestigationFinding, autonomyLevel string) error {
	return nil
}
func (m *MockOrchestrator) GetInvestigationByFinding(findingID string) *ai.InvestigationSession {
	return nil
}
func (m *MockOrchestrator) GetRunningCount() int {
	return 0
}
func (m *MockOrchestrator) GetFixedCount() int {
	return m.FixedCount
}
func (m *MockOrchestrator) CanStartInvestigation() bool {
	return true
}
func (m *MockOrchestrator) ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error {
	return nil
}

func TestHandleGetPatrolStatus_Integration(t *testing.T) {
	// Setup temporary persistence
	tempDir := t.TempDir()
	// persistence := config.NewConfigPersistence(tempDir) // Unused
	mtPersistence := config.NewMultiTenantPersistence(tempDir)

	// Create handler (this will initialize legacyAIService inside)
	handler := NewAISettingsHandler(mtPersistence, nil, nil)
	// Manually inject legacy persistence since NewAISettingsHandler uses it for default service
	// The constructor initializes legacyAIService if defaultPersistence is provided.
	// We need to match how the constructor works.
	// Since NewAISettingsHandler takes *MultiTenantPersistence, we need to ensure it can get "default" persistence
	// OR we can manually set up the legacy service if the constructor behavior is complex.

	// Better approach: mimic constructor logic by ensuring "default" exists or use a helper?
	// Actually NewAISettingsHandler code:
	// if mtp != nil { if p, err := mtp.GetPersistence("default"); ... defaultPersistence = p }
	// So we need to ensure "default" persistence is available in mtPersistence?
	// config.NewMultiTenantPersistence creates the dir but maybe not the sub-persistence until requested?
	// Let's rely on the fact that GetAIService("default") calls mtPersistence.GetPersistence("default")

	// Let's explicitly setup the handler's internal service for the test context
	svc := handler.GetAIService(context.Background())
	if svc == nil {
		// Fallback: manually set it if constructor didn't pick it up (likely due to empty mtp)
		// But GetAIService creates it if missing for orgID.
		// For "default", it returns legacyAIService.

		// To force a clean service creation we can use a specific tenant ID context
		ctx := context.WithValue(context.Background(), "org_id", "test-org")
		svc = handler.GetAIService(ctx)
		require.NotNil(t, svc, "Service should be created for new tenant")

		// Initialize PatrolService by setting StateProvider
		svc.SetStateProvider(&MockStateProvider{})
		patrol := svc.GetPatrolService()
		require.NotNil(t, patrol, "PatrolService should be initialized")

		// Inject MockOrchestrator with specific FixedCount
		mockOrch := &MockOrchestrator{FixedCount: 42}
		patrol.SetInvestigationOrchestrator(mockOrch)

		// Make request
		req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
		// Inject org_id into context so handler uses the same service
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.HandleGetPatrolStatus(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify FixedCount is propagated
		// Use float64 because JSON unmarshals numbers as floats
		require.Equal(t, 42.0, response["fixed_count"], "fixed_count should match mocked value")

		// Verify other fields exist
		require.Contains(t, response, "running")
		require.Contains(t, response, "healthy")
	} else {
		// If service was found (default case)
		svc.SetStateProvider(&MockStateProvider{})
		patrol := svc.GetPatrolService()
		require.NotNil(t, patrol)

		mockOrch := &MockOrchestrator{FixedCount: 42}
		patrol.SetInvestigationOrchestrator(mockOrch)

		req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
		w := httptest.NewRecorder()

		handler.HandleGetPatrolStatus(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		require.Equal(t, 42.0, response["fixed_count"])
	}
}

// Helper to handle context key if needed (assuming GetOrgID uses a specific key)
// But since we can't import internal/api's internal helpers easily if they are unexported,
// we rely on standard behavior or just use the default path if possible.
// internal/api/utils.go likely has GetOrgID. Ideally we'd test the default path.

func TestHandleGetPatrolStatus_NotInitialized(t *testing.T) {
	// Setup handler with NO state provider -> NO patrol service
	tempDir := t.TempDir()
	mtPersistence := config.NewMultiTenantPersistence(tempDir)
	handler := NewAISettingsHandler(mtPersistence, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetPatrolStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should indicate not enabled/running
	require.Equal(t, false, response["running"])
	require.Equal(t, false, response["enabled"])
}
