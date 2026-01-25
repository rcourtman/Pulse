package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
)

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	return payload
}

func TestAIIntelligenceHandlers_NoServices(t *testing.T) {
	handler := &AISettingsHandler{}

	t.Run("anomalies-disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/anomalies", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetAnomalies(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Pulse Patrol is not enabled" {
			t.Fatalf("message = %v, want Pulse Patrol is not enabled", payload["message"])
		}
	})

	t.Run("learning-disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/intelligence/learning", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetLearningStatus(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["status"] != "ai_disabled" {
			t.Fatalf("status = %v, want ai_disabled", payload["status"])
		}
	})

	t.Run("learning-preferences-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/learning/preferences", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetLearningPreferences(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Learning store not available" {
			t.Fatalf("message = %v, want Learning store not available", payload["message"])
		}
	})

	t.Run("unified-findings-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/unified/findings", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetUnifiedFindings(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Unified store not available" {
			t.Fatalf("message = %v, want Unified store not available", payload["message"])
		}
	})

	t.Run("proxmox-events-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/events", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetProxmoxEvents(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Proxmox event correlator not available" {
			t.Fatalf("message = %v, want Proxmox event correlator not available", payload["message"])
		}
	})

	t.Run("proxmox-correlations-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/proxmox/correlations", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetProxmoxCorrelations(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Proxmox event correlator not available" {
			t.Fatalf("message = %v, want Proxmox event correlator not available", payload["message"])
		}
	})

	t.Run("remediation-plans-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetRemediationPlans(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Remediation engine not available" {
			t.Fatalf("message = %v, want Remediation engine not available", payload["message"])
		}
	})

	t.Run("remediation-plan-missing-engine", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/remediation/plans/1?plan_id=1", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetRemediationPlan(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
	})

	t.Run("approve-remediation-plan-missing-engine", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/ai/remediation/plans/1/approve", nil)
		rr := httptest.NewRecorder()

		handler.HandleApproveRemediationPlan(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
	})
}

func TestForecastHandlers(t *testing.T) {
	handler := &AISettingsHandler{}

	t.Run("forecast-service-missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetForecast(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["message"] != "Forecast service not available" {
			t.Fatalf("message = %v, want Forecast service not available", payload["message"])
		}
	})

	t.Run("forecast-missing-params", func(t *testing.T) {
		handler.SetForecastService(forecast.NewService(forecast.DefaultForecastConfig()))
		req := httptest.NewRequest(http.MethodGet, "/api/ai/forecast", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetForecast(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forecast-overview-error", func(t *testing.T) {
		handler.SetForecastService(forecast.NewService(forecast.DefaultForecastConfig()))
		req := httptest.NewRequest(http.MethodGet, "/api/ai/forecasts/overview", nil)
		rr := httptest.NewRecorder()

		handler.HandleGetForecastOverview(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		payload := decodeJSON(t, rr)
		if payload["error"] == nil {
			t.Fatalf("expected error in response")
		}
	})
}

func TestAIIntelligenceHandlers_MethodNotAllowed(t *testing.T) {
	handler := &AISettingsHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/intelligence/anomalies", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetAnomalies(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}
