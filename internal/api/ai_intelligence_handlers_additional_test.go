package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type stubProvider struct{}

func (s *stubProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{}, nil
}

func (s *stubProvider) TestConnection(ctx context.Context) error { return nil }

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func newEnabledAIService(t *testing.T) *ai.Service {
	t.Helper()

	svc := ai.NewService(nil, nil)
	setUnexportedField(t, svc, "cfg", &config.AIConfig{Enabled: true})
	setUnexportedField(t, svc, "provider", &stubProvider{})
	setUnexportedField(t, svc, "patrolService", &ai.PatrolService{})

	return svc
}

func TestHandleGetRecentChanges_NoChangeDetector(t *testing.T) {
	handler := &AISettingsHandler{legacyAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/changes", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRecentChanges(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Change detector not initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetBaselines_NoBaselineStore(t *testing.T) {
	handler := &AISettingsHandler{legacyAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/baselines", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetBaselines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Baseline store not initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetAnomalies_NoBaselineStore(t *testing.T) {
	handler := &AISettingsHandler{legacyAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/anomalies", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetAnomalies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Baseline store not initialized - baselines are still learning" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetLearningStatus_NoBaselineStore(t *testing.T) {
	handler := &AISettingsHandler{legacyAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/learning", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Baseline store not yet initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetRemediations_NoRemediationLog(t *testing.T) {
	handler := &AISettingsHandler{legacyAIService: newEnabledAIService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/remediations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Remediation log not initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetLearningPreferences_NoStore(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/learning/preferences", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetLearningPreferences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Learning store not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetUnifiedFindings_NoStore(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/unified/findings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetUnifiedFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Unified store not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetForecast_NoProvider(t *testing.T) {
	handler := &AISettingsHandler{forecastService: forecast.NewService(forecast.DefaultForecastConfig())}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast?resource_id=res-1&metric=cpu", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == nil {
		t.Fatalf("expected error in response, got %#v", payload)
	}
}

func TestHandleGetForecastOverview_NoProvider(t *testing.T) {
	handler := &AISettingsHandler{forecastService: forecast.NewService(forecast.DefaultForecastConfig())}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecasts/overview", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecastOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == nil {
		t.Fatalf("expected error in response, got %#v", payload)
	}
}

func TestHandleGetProxmoxEvents_WithCorrelator(t *testing.T) {
	handler := &AISettingsHandler{proxmoxCorrelator: proxmox.NewEventCorrelator(proxmox.EventCorrelatorConfig{})}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/events", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleGetProxmoxCorrelations_WithCorrelator(t *testing.T) {
	handler := &AISettingsHandler{proxmoxCorrelator: proxmox.NewEventCorrelator(proxmox.EventCorrelatorConfig{})}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/correlations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleGetRemediationPlans_WithEngine(t *testing.T) {
	handler := &AISettingsHandler{remediationEngine: remediation.NewEngine(remediation.EngineConfig{})}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediationPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleGetRemediationPlan_MissingID(t *testing.T) {
	handler := &AISettingsHandler{remediationEngine: remediation.NewEngine(remediation.EngineConfig{})}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans/plan-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediationPlan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleApproveRemediationPlan_InvalidBody(t *testing.T) {
	handler := &AISettingsHandler{remediationEngine: remediation.NewEngine(remediation.EngineConfig{})}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/remediation/plans/plan-1/approve", strings.NewReader("bad-json"))
	rec := httptest.NewRecorder()

	handler.HandleApproveRemediationPlan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleApproveRemediationPlan_MissingID(t *testing.T) {
	handler := &AISettingsHandler{remediationEngine: remediation.NewEngine(remediation.EngineConfig{})}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/remediation/plans/plan-1/approve", strings.NewReader(`{\"plan_id\":\"\"}`))
	rec := httptest.NewRecorder()

	handler.HandleApproveRemediationPlan(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleGetForecast_NoService(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecast(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Forecast service not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetForecastOverview_NoService(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/forecasts/overview", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetForecastOverview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Forecast service not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetProxmoxEvents_NoCorrelator(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/events", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Proxmox event correlator not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetProxmoxCorrelations_NoCorrelator(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/correlations", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetProxmoxCorrelations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Proxmox event correlator not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetRemediationPlans_NoEngine(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediationPlans(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Remediation engine not available" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleGetRemediationPlan_NoEngine(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans/plan-1", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRemediationPlan(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleApproveRemediationPlan_NoEngine(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/remediation/plans/plan-1/approve", nil)
	rec := httptest.NewRecorder()

	handler.HandleApproveRemediationPlan(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
