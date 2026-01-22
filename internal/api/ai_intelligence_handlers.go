package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

const aiIntelligenceUpgradeURL = "https://pulserelay.pro/"

// HandleGetPatterns returns detected failure patterns (GET /api/ai/intelligence/patterns)
func (h *AISettingsHandler) HandleGetPatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"patterns": []interface{}{},
			"message":  "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write patterns response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
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

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"patterns":         result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
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

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"predictions": []interface{}{},
			"message":     "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write predictions response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
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

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"predictions":      result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
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

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"correlations": []interface{}{},
			"message":      "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write correlations response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
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

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"correlations":     result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
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

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"changes": []interface{}{},
			"message": "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write changes response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
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

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"changes":          result,
		"count":            count,
		"hours":            hours,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
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

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"baselines": []interface{}{},
			"message":   "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write baselines response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
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

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"baselines":        result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write baselines response")
	}
}

// HandleGetRemediations returns remediation history (GET /api/ai/intelligence/remediations)
func (h *AISettingsHandler) HandleGetRemediations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	// Check for Pulse Pro license (soft-lock)
	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIAutoFix)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIAutoFix)
	}

	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"remediations":     []interface{}{},
			"message":          "AI is not enabled",
			"license_required": locked,
			"upgrade_url":      aiIntelligenceUpgradeURL,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write remediations response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"remediations":     []interface{}{},
			"message":          "Patrol service not initialized",
			"license_required": locked,
			"upgrade_url":      aiIntelligenceUpgradeURL,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write remediations response")
		}
		return
	}

	remediationLog := patrol.GetRemediationLog()
	if remediationLog == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"remediations":     []interface{}{},
			"message":          "Remediation log not initialized",
			"license_required": locked,
			"upgrade_url":      aiIntelligenceUpgradeURL,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write remediations response")
		}
		return
	}

	resourceID := r.URL.Query().Get("resource_id")
	findingID := r.URL.Query().Get("finding_id")

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	hours := 168
	if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
		if parsed, err := strconv.Atoi(hoursStr); err == nil && parsed > 0 {
			hours = parsed
		}
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var records []ai.RemediationRecord
	switch {
	case findingID != "":
		records = remediationLog.GetForFinding(findingID, limit)
	case resourceID != "":
		records = remediationLog.GetForResource(resourceID, limit)
	default:
		records = remediationLog.GetRecentRemediations(limit, since)
	}

	stats := remediationStatsFromRecords(records)
	if findingID == "" && resourceID == "" {
		stats = remediationLog.GetRecentRemediationStats(since)
	}

	result := make([]map[string]interface{}, 0, len(records))
	for _, rec := range records {
		durationMs := int64(0)
		if rec.Duration > 0 {
			durationMs = rec.Duration.Milliseconds()
		}
		result = append(result, map[string]interface{}{
			"id":            rec.ID,
			"timestamp":     rec.Timestamp,
			"resource_id":   rec.ResourceID,
			"resource_type": rec.ResourceType,
			"resource_name": rec.ResourceName,
			"finding_id":    rec.FindingID,
			"problem":       rec.Problem,
			"action":        rec.Action,
			"output":        rec.Output,
			"outcome":       rec.Outcome,
			"duration_ms":   durationMs,
			"note":          rec.Note,
			"automatic":     rec.Automatic,
		})
	}

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"remediations":     result,
		"count":            count,
		"stats":            stats,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write remediations response")
	}
}

func remediationStatsFromRecords(records []ai.RemediationRecord) map[string]int {
	stats := map[string]int{
		"total":     len(records),
		"resolved":  0,
		"partial":   0,
		"failed":    0,
		"unknown":   0,
		"automatic": 0,
		"manual":    0,
	}

	for _, rec := range records {
		switch rec.Outcome {
		case ai.OutcomeResolved:
			stats["resolved"]++
		case ai.OutcomePartial:
			stats["partial"]++
		case ai.OutcomeFailed:
			stats["failed"]++
		default:
			stats["unknown"]++
		}
		if rec.Automatic {
			stats["automatic"]++
		} else {
			stats["manual"]++
		}
	}

	return stats
}

// HandleGetAnomalies returns current baseline anomalies (GET /api/ai/intelligence/anomalies)
// This compares live metrics against learned baselines to surface deviations.
// Anomalies are deterministic (no LLM) - based on statistical z-score thresholds.
func (h *AISettingsHandler) HandleGetAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"anomalies": []interface{}{},
			"message":   "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write anomalies response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"anomalies": []interface{}{},
			"message":   "Patrol service not initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write anomalies response")
		}
		return
	}

	baselineStore := patrol.GetBaselineStore()
	if baselineStore == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"anomalies": []interface{}{},
			"message":   "Baseline store not initialized - baselines are still learning",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write anomalies response")
		}
		return
	}

	// Get current metrics from state provider
	stateProvider := aiService.GetStateProvider()
	if stateProvider == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"anomalies": []interface{}{},
			"message":   "State provider not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write anomalies response")
		}
		return
	}

	// Get resource filter if provided
	resourceID := r.URL.Query().Get("resource_id")

	// Collect anomalies
	var result []map[string]interface{}

	// Get all baselines and check current metrics
	allBaselines := baselineStore.GetAllBaselines()

	// Group by resource ID
	resourceMetrics := make(map[string]map[string]float64)
	resourceInfo := make(map[string]struct{ name, rtype string })

	for _, baseline := range allBaselines {
		if resourceID != "" && baseline.ResourceID != resourceID {
			continue
		}
		if _, ok := resourceMetrics[baseline.ResourceID]; !ok {
			resourceMetrics[baseline.ResourceID] = make(map[string]float64)
		}
	}

	// Get current state to extract live metrics
	state := stateProvider.GetState()

	// Check VMs
	for _, vm := range state.VMs {
		if vm.Template {
			continue // Skip templates
		}

		// Skip VMs that aren't running - stopped VMs with 0% usage is expected, not an anomaly
		if vm.Status != "running" {
			continue
		}

		// Skip if we don't have baselines for this resource
		if _, ok := resourceMetrics[vm.ID]; !ok {
			if resourceID == "" {
				continue
			}
			if vm.ID != resourceID {
				continue
			}
		}

		metrics := map[string]float64{
			"cpu":    vm.CPU * 100,    // CPU is already 0-1, convert to percentage
			"memory": vm.Memory.Usage, // Memory.Usage is already in percentage
		}
		if vm.Disk.Usage > 0 {
			metrics["disk"] = vm.Disk.Usage
		}

		anomalies := baselineStore.CheckResourceAnomalies(vm.ID, metrics)
		for _, anomaly := range anomalies {
			result = append(result, map[string]interface{}{
				"resource_id":      anomaly.ResourceID,
				"resource_name":    vm.Name,
				"resource_type":    "vm",
				"metric":           anomaly.Metric,
				"current_value":    anomaly.CurrentValue,
				"baseline_mean":    anomaly.BaselineMean,
				"baseline_std_dev": anomaly.BaselineStdDev,
				"z_score":          anomaly.ZScore,
				"severity":         anomaly.Severity,
				"description":      anomaly.Description,
			})
		}

		// Store info for any additional processing
		resourceInfo[vm.ID] = struct{ name, rtype string }{vm.Name, "vm"}
	}

	// Check Containers
	for _, ct := range state.Containers {
		if ct.Template {
			continue // Skip templates
		}

		// Skip containers that aren't running - stopped containers with 0% usage is expected, not an anomaly
		if ct.Status != "running" {
			continue
		}

		// Skip if we don't have baselines for this resource
		if _, ok := resourceMetrics[ct.ID]; !ok {
			if resourceID == "" {
				continue
			}
			if ct.ID != resourceID {
				continue
			}
		}

		metrics := map[string]float64{
			"cpu":    ct.CPU * 100,    // CPU is already 0-1, convert to percentage
			"memory": ct.Memory.Usage, // Memory.Usage is already in percentage
		}
		if ct.Disk.Usage > 0 {
			metrics["disk"] = ct.Disk.Usage
		}

		anomalies := baselineStore.CheckResourceAnomalies(ct.ID, metrics)
		for _, anomaly := range anomalies {
			result = append(result, map[string]interface{}{
				"resource_id":      anomaly.ResourceID,
				"resource_name":    ct.Name,
				"resource_type":    "container",
				"metric":           anomaly.Metric,
				"current_value":    anomaly.CurrentValue,
				"baseline_mean":    anomaly.BaselineMean,
				"baseline_std_dev": anomaly.BaselineStdDev,
				"z_score":          anomaly.ZScore,
				"severity":         anomaly.Severity,
				"description":      anomaly.Description,
			})
		}

		// Store info for any additional processing
		resourceInfo[ct.ID] = struct{ name, rtype string }{ct.Name, "container"}
	}

	// Check nodes
	for _, node := range state.Nodes {
		nodeID := node.ID

		// Skip if we don't have baselines for this resource
		if _, ok := resourceMetrics[nodeID]; !ok {
			if resourceID == "" {
				continue
			}
			if nodeID != resourceID {
				continue
			}
		}

		metrics := map[string]float64{
			"cpu":    node.CPU * 100,    // CPU is already 0-1, convert to percentage
			"memory": node.Memory.Usage, // Memory.Usage is already in percentage
		}

		anomalies := baselineStore.CheckResourceAnomalies(nodeID, metrics)
		for _, anomaly := range anomalies {
			result = append(result, map[string]interface{}{
				"resource_id":      anomaly.ResourceID,
				"resource_name":    node.Name,
				"resource_type":    "node",
				"metric":           anomaly.Metric,
				"current_value":    anomaly.CurrentValue,
				"baseline_mean":    anomaly.BaselineMean,
				"baseline_std_dev": anomaly.BaselineStdDev,
				"z_score":          anomaly.ZScore,
				"severity":         anomaly.Severity,
				"description":      anomaly.Description,
			})
		}
	}

	count := len(result)

	// Count by severity for summary
	severityCounts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}
	for _, anomaly := range result {
		if sev, ok := anomaly["severity"].(string); ok {
			severityCounts[sev]++
		}
	}

	// NOTE: Anomaly detection is FREE (no license required)
	// It's purely deterministic statistical analysis with no LLM costs
	// This provides value to all users and encourages Pro upgrades for patrol

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"anomalies":        result,
		"count":            count,
		"severity_counts":  severityCounts,
		"license_required": false,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write anomalies response")
	}
}

// HandleGetLearningStatus returns the current state of baseline learning (GET /api/ai/intelligence/learning)
// This is FREE (no license required) and shows users how much the system has learned
func (h *AISettingsHandler) HandleGetLearningStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	aiService := h.GetAIService(r.Context())
	// AI must be enabled to return learning status
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"resources_baselined": 0,
			"total_metrics":       0,
			"status":              "ai_disabled",
			"message":             "AI is not enabled",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write learning status response")
		}
		return
	}

	patrol := aiService.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"resources_baselined": 0,
			"total_metrics":       0,
			"status":              "patrol_not_initialized",
			"message":             "Baseline learning requires AI Patrol to be initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write learning status response")
		}
		return
	}

	baselineStore := patrol.GetBaselineStore()
	if baselineStore == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"resources_baselined": 0,
			"total_metrics":       0,
			"status":              "baseline_store_not_initialized",
			"message":             "Baseline store not yet initialized",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write learning status response")
		}
		return
	}

	// Get all baselines and count metrics
	baselines := baselineStore.GetAllBaselines()
	resourceCount := baselineStore.ResourceCount()

	// Count unique resources and total metrics
	resourceIDs := make(map[string]bool)
	totalMetrics := 0
	metricCounts := make(map[string]int) // cpu, memory, disk counts

	for _, baseline := range baselines {
		resourceIDs[baseline.ResourceID] = true
		totalMetrics++
		metricCounts[baseline.Metric]++
	}

	// Determine status
	status := "learning"
	message := "Actively learning baseline patterns"
	if resourceCount == 0 {
		status = "waiting"
		message = "Waiting for metric data to learn from"
	} else if resourceCount >= 5 {
		status = "active"
		message = "Baselines established and anomaly detection is active"
	}

	locked := aiService == nil || !aiService.HasLicenseFeature(license.FeatureAIPatrol)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", license.FeatureAIPatrol)
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"resources_baselined": resourceCount,
		"total_metrics":       totalMetrics,
		"metric_breakdown":    metricCounts,
		"status":              status,
		"message":             message,
		"license_required":    locked,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write learning status response")
	}
}
