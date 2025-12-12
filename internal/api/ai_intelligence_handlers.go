package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// HandleGetPatterns returns detected failure patterns (GET /api/ai/intelligence/patterns)
func (h *AISettingsHandler) HandleGetPatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write patterns response")
		}
		return
	}

	detector := patrol.GetPatternDetector()
	if detector == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "Pattern detector not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write patterns response")
		}
		return
	}

	// Get resource filter if provided
	resourceID := r.URL.Query().Get("resource_id")
	
	patterns := detector.GetPatterns()
	var result []map[string]interface{}
	
	for key, pattern := range patterns {
		if resourceID != "" && pattern.ResourceID != resourceID {
			continue
		}
		result = append(result, map[string]interface{}{
			"key":              key,
			"resource_id":      pattern.ResourceID,
			"event_type":       pattern.EventType,
			"occurrences":      pattern.Occurrences,
			"average_interval": pattern.AverageInterval.String(),
			"average_duration": pattern.AverageDuration.String(),
			"last_occurrence":  pattern.LastOccurrence,
			"confidence":       pattern.Confidence,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"patterns": result,
		"count":    len(result),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write patterns response")
	}
}

// HandleGetPredictions returns failure predictions (GET /api/ai/intelligence/predictions)
func (h *AISettingsHandler) HandleGetPredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"predictions": []interface{}{},
			"message":     "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write predictions response")
		}
		return
	}

	detector := patrol.GetPatternDetector()
	if detector == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"predictions": []interface{}{},
			"message":     "Pattern detector not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write predictions response")
		}
		return
	}

	// Get resource filter if provided
	resourceID := r.URL.Query().Get("resource_id")
	
	var predictions []ai.FailurePrediction
	if resourceID != "" {
		predictions = detector.GetPredictionsForResource(resourceID)
	} else {
		predictions = detector.GetPredictions()
	}

	var result []map[string]interface{}
	for _, pred := range predictions {
		isOverdue := pred.DaysUntil < 0
		result = append(result, map[string]interface{}{
			"resource_id":  pred.ResourceID,
			"event_type":   pred.EventType,
			"predicted_at": pred.PredictedAt,
			"days_until":   pred.DaysUntil,
			"confidence":   pred.Confidence,
			"basis":        pred.Basis,
			"is_overdue":   isOverdue,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"predictions": result,
		"count":       len(result),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write predictions response")
	}
}

// HandleGetCorrelations returns detected resource correlations (GET /api/ai/intelligence/correlations)
func (h *AISettingsHandler) HandleGetCorrelations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"correlations": []interface{}{},
			"message":      "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write correlations response")
		}
		return
	}

	detector := patrol.GetCorrelationDetector()
	if detector == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"correlations": []interface{}{},
			"message":      "Correlation detector not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write correlations response")
		}
		return
	}

	// Get resource filter if provided
	resourceID := r.URL.Query().Get("resource_id")
	
	var correlations []*ai.Correlation
	if resourceID != "" {
		correlations = detector.GetCorrelationsForResource(resourceID)
	} else {
		correlations = detector.GetCorrelations()
	}

	var result []map[string]interface{}
	for _, corr := range correlations {
		result = append(result, map[string]interface{}{
			"source_id":     corr.SourceID,
			"source_name":   corr.SourceName,
			"source_type":   corr.SourceType,
			"target_id":     corr.TargetID,
			"target_name":   corr.TargetName,
			"target_type":   corr.TargetType,
			"event_pattern": corr.EventPattern,
			"occurrences":   corr.Occurrences,
			"avg_delay":     corr.AvgDelay.String(),
			"confidence":    corr.Confidence,
			"last_seen":     corr.LastSeen,
			"description":   corr.Description,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"correlations": result,
		"count":        len(result),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write correlations response")
	}
}

// HandleGetRecentChanges returns recent infrastructure changes (GET /api/ai/intelligence/changes)
func (h *AISettingsHandler) HandleGetRecentChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"changes": []interface{}{},
			"message": "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write changes response")
		}
		return
	}

	detector := patrol.GetChangeDetector()
	if detector == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"changes": []interface{}{},
			"message": "Change detector not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write changes response")
		}
		return
	}

	// Get time range - default to 24 hours
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}
	
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	changes := detector.GetRecentChanges(100, since)

	var result []map[string]interface{}
	for _, change := range changes {
		result = append(result, map[string]interface{}{
			"id":            change.ID,
			"resource_id":   change.ResourceID,
			"resource_name": change.ResourceName,
			"resource_type": change.ResourceType,
			"change_type":   change.ChangeType,
			"before":        change.Before,
			"after":         change.After,
			"detected_at":   change.DetectedAt,
			"description":   change.Description,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"changes": result,
		"count":   len(result),
		"hours":   hours,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write changes response")
	}
}

// HandleGetBaselines returns learned resource baselines (GET /api/ai/intelligence/baselines)
func (h *AISettingsHandler) HandleGetBaselines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	patrol := h.aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"baselines": []interface{}{},
			"message":   "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write baselines response")
		}
		return
	}

	store := patrol.GetBaselineStore()
	if store == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"baselines": []interface{}{},
			"message":   "Baseline store not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write baselines response")
		}
		return
	}

	// Get resource filter if provided
	resourceID := r.URL.Query().Get("resource_id")
	
	baselines := store.GetAllBaselines()
	var result []map[string]interface{}
	
	for key, baseline := range baselines {
		if resourceID != "" && baseline.ResourceID != resourceID {
			continue
		}
		result = append(result, map[string]interface{}{
			"key":         key,
			"resource_id": baseline.ResourceID,
			"metric":      baseline.Metric,
			"mean":        baseline.Mean,
			"std_dev":     baseline.StdDev,
			"min":         baseline.Min,
			"max":         baseline.Max,
			"samples":     baseline.Samples,
			"last_update": baseline.LastUpdate,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"baselines": result,
		"count":     len(result),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write baselines response")
	}
}
