package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newPatrolCostPreviewHandler(t *testing.T, mutate func(cfg *config.AIConfig)) *AISettingsHandler {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	aiCfg := config.NewDefaultAIConfig()
	if mutate != nil {
		mutate(aiCfg)
	}
	if err := persistence.SaveAIConfig(*aiCfg); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	return newTestAISettingsHandler(cfg, persistence, nil)
}

func TestHandleGetPatrolCostPreview_DefaultsToConfiguredPatrolModelAndSchedule(t *testing.T) {
	t.Parallel()
	handler := newPatrolCostPreviewHandler(t, func(cfg *config.AIConfig) {
		cfg.Enabled = true
		cfg.Model = "gemini:gemini-2.5-flash"
		cfg.PatrolIntervalMinutes = 720
		cfg.CostBudgetUSD30d = 20
	})

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/cost-preview", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolCostPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ai.PatrolCostProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Provider != "gemini" || resp.Model != "gemini-2.5-flash" || resp.IntervalMinutes != 720 {
		t.Fatalf("expected configured model and schedule, got %+v", resp)
	}
	if !resp.PricingKnown || !resp.BilledPerToken || resp.Projected30dUSD <= 0 {
		t.Fatalf("expected a priced projection, got %+v", resp)
	}
	if resp.BudgetUSD30d != 20 || resp.PerRunSource != ai.PatrolCostPerRunSourceDefault {
		t.Fatalf("expected configured budget and default per-run source, got %+v", resp)
	}
}

func TestHandleGetPatrolCostPreview_PreviewsPendingSelection(t *testing.T) {
	t.Parallel()
	handler := newPatrolCostPreviewHandler(t, func(cfg *config.AIConfig) {
		cfg.Enabled = true
		cfg.Model = "ollama:qwen3:8b"
	})

	req := newLoopbackRequest(http.MethodGet, "/api/ai/patrol/cost-preview?model=anthropic:claude-sonnet-5&interval_minutes=60", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetPatrolCostPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ai.PatrolCostProjection
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Provider != "anthropic" || resp.Model != "claude-sonnet-5" || resp.IntervalMinutes != 60 {
		t.Fatalf("expected pending selection, got %+v", resp)
	}
	if resp.RecommendedIntervalMinutes != 1440 {
		t.Fatalf("expected the cost model to propose a daily schedule for sonnet on the reference budget, got %d", resp.RecommendedIntervalMinutes)
	}
}

func TestHandleGetPatrolCostPreview_RejectsBadIntervalAndMethod(t *testing.T) {
	t.Parallel()
	handler := newPatrolCostPreviewHandler(t, nil)

	rec := httptest.NewRecorder()
	handler.HandleGetPatrolCostPreview(rec, newLoopbackRequest(http.MethodGet, "/api/ai/patrol/cost-preview?interval_minutes=-5", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative interval, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	handler.HandleGetPatrolCostPreview(rec, newLoopbackRequest(http.MethodPost, "/api/ai/patrol/cost-preview", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
}

func TestHandleGetPatrolModelGuidance_ReturnsRulesWithoutVerifiedMarker(t *testing.T) {
	t.Parallel()
	handler := newPatrolCostPreviewHandler(t, nil)

	rec := httptest.NewRecorder()
	handler.HandleGetPatrolModelGuidance(rec, newLoopbackRequest(http.MethodGet, "/api/ai/patrol/model-guidance", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ai.PatrolModelGuidanceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Rules) == 0 || resp.Verified != nil {
		t.Fatalf("expected static rules and no verified marker, got %+v", resp)
	}
	sawOllama := false
	for _, rule := range resp.Rules {
		if rule.Provider == config.AIProviderOllama && rule.Level == ai.PatrolModelGuidanceRecommended {
			sawOllama = true
		}
	}
	if !sawOllama {
		t.Fatal("expected the Ollama blessing in the guidance rules")
	}
}
