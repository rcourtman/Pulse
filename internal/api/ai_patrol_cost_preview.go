package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// HandleGetPatrolCostPreview (GET /api/ai/patrol/cost-preview) projects what
// Patrol will cost per 30 days for a model and schedule, so the settings page
// can show the bill next to the model choice instead of after the budget
// trips (#1789, support 2026-07-20/30).
//
// Query parameters:
//   - model: a provider:model route to preview; defaults to the configured
//     Patrol model.
//   - interval_minutes: schedule to price; defaults to the configured one.
//     0 means manual-only.
func (h *AISettingsHandler) HandleGetPatrolCostPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	modelRoute := strings.TrimSpace(query.Get("model"))
	intervalMinutes := -1
	if raw := strings.TrimSpace(query.Get("interval_minutes")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			http.Error(w, "interval_minutes must be a non-negative integer", http.StatusBadRequest)
			return
		}
		intervalMinutes = parsed
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil {
		projection := ai.ProjectPatrolCostForConfig(nil, modelRoute, intervalMinutes, nil, cost.Summary{}, time.Now())
		if err := utils.WriteJSONResponse(w, projection); err != nil {
			log.Error().Err(err).Msg("Failed to write Patrol cost preview response")
		}
		return
	}

	var runs []ai.PatrolRunRecord
	if patrol := aiService.GetPatrolService(); patrol != nil {
		runs = patrol.GetRunHistory(0)
	}
	projection := ai.ProjectPatrolCostForConfig(
		aiService.GetConfig(),
		modelRoute,
		intervalMinutes,
		runs,
		aiService.GetCostSummary(30),
		time.Now(),
	)
	if err := utils.WriteJSONResponse(w, projection); err != nil {
		log.Error().Err(err).Msg("Failed to write Patrol cost preview response")
	}
}

// HandleGetPatrolModelGuidance (GET /api/ai/patrol/model-guidance) returns
// the recommended / suggested / caution markers the model pickers render,
// plus this install's cached readiness pass when there is one.
func (h *AISettingsHandler) HandleGetPatrolModelGuidance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response := h.GetAIService(r.Context()).PatrolModelGuidance()
	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write Patrol model guidance response")
	}
}
