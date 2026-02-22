package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/baseline"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/unified"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

type proxmoxWorkload interface {
	ID() string
	Name() string
	Status() unifiedresources.ResourceStatus
	Template() bool
	MemoryPercent() float64
	CPUPercent() float64
	DiskPercent() float64
}

func checkWorkloadAnomalies(
	workload proxmoxWorkload,
	resourceID string,
	resourceMetrics map[string]map[string]float64,
	baselineStore *baseline.Store,
	result *[]map[string]interface{},
	resourceInfo map[string]struct{ name, rtype string },
	resourceType string,
) {
	if workload == nil {
		return
	}
	if workload.Template() {
		return
	}
	if workload.Status() != unifiedresources.StatusOnline {
		return
	}

	if _, ok := resourceMetrics[workload.ID()]; !ok {
		if resourceID == "" {
			return
		}
		if workload.ID() != resourceID {
			return
		}
	}

	metrics := map[string]float64{
		"memory": workload.MemoryPercent(),
	}
	if cpu := workload.CPUPercent(); cpu > 0 {
		metrics["cpu"] = cpu
	}
	if disk := workload.DiskPercent(); disk > 0 {
		metrics["disk"] = disk
	}

	anomalies := baselineStore.CheckResourceAnomaliesReadOnly(workload.ID(), metrics)
	for _, anomaly := range anomalies {
		*result = append(*result, map[string]interface{}{
			"resource_id":      anomaly.ResourceID,
			"resource_name":    workload.Name(),
			"resource_type":    resourceType,
			"metric":           anomaly.Metric,
			"current_value":    anomaly.CurrentValue,
			"baseline_mean":    anomaly.BaselineMean,
			"baseline_std_dev": anomaly.BaselineStdDev,
			"z_score":          anomaly.ZScore,
			"severity":         anomaly.Severity,
			"description":      anomaly.Description,
		})
	}

	resourceInfo[workload.ID()] = struct{ name, rtype string }{workload.Name(), resourceType}
}

func aiIntelligenceUpgradeURL() string {
	return pkglicensing.UpgradeURLForFeature(pkglicensing.FeatureAIPatrol)
}

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
			"message":  "Pulse Patrol is not enabled",
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

	locked := aiService == nil

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"patterns":         result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"message":     "Pulse Patrol is not enabled",
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

	locked := aiService == nil

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"predictions":      result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"message":      "Pulse Patrol is not enabled",
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

	locked := aiService == nil

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"correlations":     result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"message": "Pulse Patrol is not enabled",
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

	locked := aiService == nil

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"changes":          result,
		"count":            count,
		"hours":            hours,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"message":   "Pulse Patrol is not enabled",
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

	locked := aiService == nil

	count := len(result)
	if locked {
		result = []map[string]interface{}{}
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"baselines":        result,
		"count":            count,
		"license_required": locked,
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
	locked := aiService == nil || !aiService.HasLicenseFeature(pkglicensing.FeatureAIAutoFix)
	if locked {
		w.Header().Set("X-License-Required", "true")
		w.Header().Set("X-License-Feature", pkglicensing.FeatureAIAutoFix)
	}

	// AI must be enabled to return intelligence data
	if aiService == nil || !aiService.IsEnabled() {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"remediations":     []interface{}{},
			"message":          "Pulse Patrol is not enabled",
			"license_required": locked,
			"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"upgrade_url":      aiIntelligenceUpgradeURL(),
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
		"upgrade_url":      aiIntelligenceUpgradeURL(),
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
			"message":   "Pulse Patrol is not enabled",
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

	// ReadState is the canonical resource read surface for AI intelligence endpoints.
	// If this isn't wired, we can't reliably join baselines to live resources.
	rs := h.readState
	if rs == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"anomalies": []interface{}{},
			"message":   "ReadState not available",
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

	// Check VMs
	for _, vm := range rs.VMs() {
		checkWorkloadAnomalies(vm, resourceID, resourceMetrics, baselineStore, &result, resourceInfo, "vm")
	}

	// Check Containers
	for _, ct := range rs.Containers() {
		checkWorkloadAnomalies(ct, resourceID, resourceMetrics, baselineStore, &result, resourceInfo, "container")
	}

	// Check nodes
	for _, node := range rs.Nodes() {
		if node == nil {
			continue
		}
		nodeID := node.ID()

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
			"cpu":    node.CPUPercent(),
			"memory": node.MemoryPercent(),
		}

		anomalies := baselineStore.CheckResourceAnomaliesReadOnly(nodeID, metrics)
		for _, anomaly := range anomalies {
			result = append(result, map[string]interface{}{
				"resource_id":      anomaly.ResourceID,
				"resource_name":    node.Name(),
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
			"message":             "Pulse Patrol is not enabled",
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
			"message":             "Baseline learning requires Pulse Patrol to be initialized",
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

	locked := aiService == nil

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

// ============================================================================
// Phase 6: AI Intelligence Handlers
// ============================================================================

// HandleGetForecast returns trend forecast for a resource metric (GET /api/ai/forecast)
func (h *AISettingsHandler) HandleGetForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	forecastSvc := h.GetForecastService()
	if forecastSvc == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"forecast": nil,
			"message":  "Forecast service not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write forecast response")
		}
		return
	}

	resourceID := r.URL.Query().Get("resource_id")
	resourceName := r.URL.Query().Get("resource_name")
	metric := r.URL.Query().Get("metric")
	horizonStr := r.URL.Query().Get("horizon_hours")
	thresholdStr := r.URL.Query().Get("threshold")

	if resourceID == "" || metric == "" {
		http.Error(w, "resource_id and metric parameters are required", http.StatusBadRequest)
		return
	}

	// Default horizon to 24 hours
	horizon := 24 * time.Hour
	if horizonStr != "" {
		hours, err := strconv.Atoi(horizonStr)
		if err == nil && hours > 0 {
			horizon = time.Duration(hours) * time.Hour
		}
	}

	// Default threshold to 90%
	threshold := 90.0
	if thresholdStr != "" {
		if t, err := strconv.ParseFloat(thresholdStr, 64); err == nil && t > 0 {
			threshold = t
		}
	}

	forecast, err := forecastSvc.Forecast(resourceID, resourceName, metric, horizon, threshold)
	if err != nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"forecast": nil,
			"error":    err.Error(),
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write forecast error response")
		}
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"forecast": forecast,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write forecast response")
	}
}

// HandleGetForecastOverview returns forecasts for all resources sorted by urgency (GET /api/ai/forecasts/overview)
func (h *AISettingsHandler) HandleGetForecastOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	forecastSvc := h.GetForecastService()
	if forecastSvc == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"forecasts":     []interface{}{},
			"message":       "Forecast service not available",
			"metric":        "",
			"threshold":     0,
			"horizon_hours": 0,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write forecast overview response")
		}
		return
	}

	// Parse query parameters
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "cpu" // Default to CPU
	}

	// Default horizon to 168 hours (7 days)
	horizonStr := r.URL.Query().Get("horizon_hours")
	horizon := 168 * time.Hour
	if horizonStr != "" {
		hours, err := strconv.Atoi(horizonStr)
		if err == nil && hours > 0 {
			horizon = time.Duration(hours) * time.Hour
		}
	}

	// Default threshold to 90%
	thresholdStr := r.URL.Query().Get("threshold")
	threshold := 90.0
	if thresholdStr != "" {
		if t, err := strconv.ParseFloat(thresholdStr, 64); err == nil && t > 0 {
			threshold = t
		}
	}

	overview, err := forecastSvc.ForecastAll(metric, horizon, threshold)
	if err != nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"forecasts":     []interface{}{},
			"error":         err.Error(),
			"metric":        metric,
			"threshold":     threshold,
			"horizon_hours": int(horizon.Hours()),
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write forecast overview error response")
		}
		return
	}

	if err := utils.WriteJSONResponse(w, overview); err != nil {
		log.Error().Err(err).Msg("Failed to write forecast overview response")
	}
}

// HandleGetLearningPreferences returns learned user preferences (GET /api/ai/learning/preferences)
func (h *AISettingsHandler) HandleGetLearningPreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	learningStore := h.GetLearningStore()
	if learningStore == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"preferences": nil,
			"message":     "Learning store not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write learning preferences response")
		}
		return
	}

	resourceID := r.URL.Query().Get("resource_id")

	var response map[string]interface{}
	if resourceID != "" {
		// Get preferences for specific resource
		prefs := learningStore.GetResourcePreference(resourceID)
		response = map[string]interface{}{
			"resource_id": resourceID,
			"preferences": prefs,
			"context":     learningStore.FormatForContext(),
		}
	} else {
		// Get overall statistics
		stats := learningStore.GetStatistics()
		response = map[string]interface{}{
			"statistics": stats,
			"context":    learningStore.FormatForContext(),
		}
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write learning preferences response")
	}
}

// HandleGetUnifiedFindings returns unified findings from alerts and AI (GET /api/ai/unified/findings)
func (h *AISettingsHandler) HandleGetUnifiedFindings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store := h.GetUnifiedStore()
	if store == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"findings": []interface{}{},
			"message":  "Unified store not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write unified findings response")
		}
		return
	}

	resourceID := strings.TrimSpace(r.URL.Query().Get("resource_id"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	includeResolved := false
	if includeStr := strings.TrimSpace(r.URL.Query().Get("include_resolved")); includeStr != "" {
		switch strings.ToLower(includeStr) {
		case "1", "true", "yes", "y":
			includeResolved = true
		}
	}

	var findings []*unified.UnifiedFinding
	if includeResolved {
		findings = store.GetAll()
	} else {
		findings = store.GetActive()
	}

	type findingView struct {
		ID             string     `json:"id"`
		Source         string     `json:"source"`
		Severity       string     `json:"severity"`
		Category       string     `json:"category"`
		ResourceID     string     `json:"resource_id"`
		ResourceName   string     `json:"resource_name"`
		ResourceType   string     `json:"resource_type"`
		Node           string     `json:"node,omitempty"`
		Title          string     `json:"title"`
		Description    string     `json:"description"`
		Recommendation string     `json:"recommendation,omitempty"`
		Evidence       string     `json:"evidence,omitempty"`
		AlertID        string     `json:"alert_id,omitempty"`
		AlertType      string     `json:"alert_type,omitempty"`
		Value          float64    `json:"value,omitempty"`
		Threshold      float64    `json:"threshold,omitempty"`
		IsThreshold    bool       `json:"is_threshold,omitempty"`
		AIContext      string     `json:"ai_context,omitempty"`
		RootCauseID    string     `json:"root_cause_id,omitempty"`
		CorrelatedIDs  []string   `json:"correlated_ids,omitempty"`
		RemediationID  string     `json:"remediation_id,omitempty"`
		AIConfidence   float64    `json:"ai_confidence,omitempty"`
		EnhancedByAI   bool       `json:"enhanced_by_ai,omitempty"`
		AIEnhancedAt   *time.Time `json:"ai_enhanced_at,omitempty"`
		// Investigation fields
		InvestigationSessionID string                                 `json:"investigation_session_id,omitempty"`
		InvestigationStatus    string                                 `json:"investigation_status,omitempty"`
		InvestigationOutcome   string                                 `json:"investigation_outcome,omitempty"`
		LastInvestigatedAt     *time.Time                             `json:"last_investigated_at,omitempty"`
		InvestigationAttempts  int                                    `json:"investigation_attempts,omitempty"`
		LoopState              string                                 `json:"loop_state,omitempty"`
		Lifecycle              []unified.UnifiedFindingLifecycleEvent `json:"lifecycle,omitempty"`
		RegressionCount        int                                    `json:"regression_count,omitempty"`
		LastRegressionAt       *time.Time                             `json:"last_regression_at,omitempty"`
		// Timestamps and user feedback
		DetectedAt      time.Time  `json:"detected_at"`
		LastSeenAt      time.Time  `json:"last_seen_at"`
		ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
		AcknowledgedAt  *time.Time `json:"acknowledged_at,omitempty"`
		SnoozedUntil    *time.Time `json:"snoozed_until,omitempty"`
		DismissedReason string     `json:"dismissed_reason,omitempty"`
		UserNote        string     `json:"user_note,omitempty"`
		Suppressed      bool       `json:"suppressed,omitempty"`
		TimesRaised     int        `json:"times_raised,omitempty"`
		Status          string     `json:"status"`
	}

	now := time.Now()
	result := make([]findingView, 0, len(findings))
	activeCount := 0

	for _, f := range findings {
		if f == nil {
			continue
		}

		if resourceID != "" && f.ResourceID != resourceID {
			continue
		}
		if source != "" && string(f.Source) != source {
			continue
		}

		status := "active"
		if f.ResolvedAt != nil {
			status = "resolved"
		} else if f.SnoozedUntil != nil && now.Before(*f.SnoozedUntil) {
			status = "snoozed"
		} else if f.DismissedReason != "" || f.Suppressed {
			status = "dismissed"
		}

		if status == "active" {
			activeCount++
		}

		result = append(result, findingView{
			ID:                     f.ID,
			Source:                 string(f.Source),
			Severity:               string(f.Severity),
			Category:               string(f.Category),
			ResourceID:             f.ResourceID,
			ResourceName:           f.ResourceName,
			ResourceType:           f.ResourceType,
			Node:                   f.Node,
			Title:                  f.Title,
			Description:            f.Description,
			Recommendation:         f.Recommendation,
			Evidence:               f.Evidence,
			AlertID:                f.AlertID,
			AlertType:              f.AlertType,
			Value:                  f.Value,
			Threshold:              f.Threshold,
			IsThreshold:            f.IsThreshold,
			AIContext:              f.AIContext,
			RootCauseID:            f.RootCauseID,
			CorrelatedIDs:          f.CorrelatedIDs,
			RemediationID:          f.RemediationID,
			AIConfidence:           f.AIConfidence,
			EnhancedByAI:           f.EnhancedByAI,
			AIEnhancedAt:           f.AIEnhancedAt,
			InvestigationSessionID: f.InvestigationSessionID,
			InvestigationStatus:    f.InvestigationStatus,
			InvestigationOutcome:   f.InvestigationOutcome,
			LastInvestigatedAt:     f.LastInvestigatedAt,
			InvestigationAttempts:  f.InvestigationAttempts,
			LoopState:              f.LoopState,
			Lifecycle:              f.Lifecycle,
			RegressionCount:        f.RegressionCount,
			LastRegressionAt:       f.LastRegressionAt,
			DetectedAt:             f.DetectedAt,
			LastSeenAt:             f.LastSeenAt,
			ResolvedAt:             f.ResolvedAt,
			AcknowledgedAt:         f.AcknowledgedAt,
			SnoozedUntil:           f.SnoozedUntil,
			DismissedReason:        f.DismissedReason,
			UserNote:               f.UserNote,
			Suppressed:             f.Suppressed,
			TimesRaised:            f.TimesRaised,
			Status:                 status,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"findings":     result,
		"count":        len(result),
		"active_count": activeCount,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write unified findings response")
	}
}

// HandleGetProxmoxEvents returns recent Proxmox events (GET /api/ai/proxmox/events)
func (h *AISettingsHandler) HandleGetProxmoxEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlator := h.GetProxmoxCorrelator()
	if correlator == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"events":  []interface{}{},
			"message": "Proxmox event correlator not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write proxmox events response")
		}
		return
	}

	durationStr := r.URL.Query().Get("duration")
	duration := 30 * time.Minute
	if durationStr != "" {
		if mins, err := strconv.Atoi(durationStr); err == nil && mins > 0 {
			duration = time.Duration(mins) * time.Minute
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	resourceID := r.URL.Query().Get("resource_id")

	var events interface{}
	if resourceID != "" {
		events = correlator.GetEventsForResource(resourceID, limit)
	} else {
		events = correlator.GetRecentEventsWithLimit(duration, limit)
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"events":            events,
		"active_operations": correlator.GetActiveOperations(),
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write proxmox events response")
	}
}

// HandleGetProxmoxCorrelations returns Proxmox event correlations (GET /api/ai/proxmox/correlations)
func (h *AISettingsHandler) HandleGetProxmoxCorrelations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	correlator := h.GetProxmoxCorrelator()
	if correlator == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"correlations": []interface{}{},
			"message":      "Proxmox event correlator not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write proxmox correlations response")
		}
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	resourceID := r.URL.Query().Get("resource_id")

	var correlations interface{}
	if resourceID != "" {
		correlations = correlator.GetCorrelationsForResource(resourceID)
	} else {
		correlations = correlator.GetCorrelations(limit)
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"correlations": correlations,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write proxmox correlations response")
	}
}

// HandleGetRemediationPlans returns remediation plans with status (GET /api/ai/remediation/plans)
// Note: Plans are transient and stored in memory with their executions
func (h *AISettingsHandler) HandleGetRemediationPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := h.GetRemediationEngine()
	if engine == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"plans":   []interface{}{},
			"message": "Remediation engine not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write remediation plans response")
		}
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	plans := engine.ListPlans(limit)

	type stepView struct {
		Order           int    `json:"order"`
		Action          string `json:"action"`
		Command         string `json:"command,omitempty"`
		RollbackCommand string `json:"rollback_command,omitempty"`
		RiskLevel       string `json:"risk_level"`
	}

	type planView struct {
		ID          string     `json:"id"`
		FindingID   string     `json:"finding_id"`
		ResourceID  string     `json:"resource_id"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		Steps       []stepView `json:"steps"`
		RiskLevel   string     `json:"risk_level"`
		Status      string     `json:"status"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	result := make([]planView, 0, len(plans))
	for _, plan := range plans {
		if plan == nil {
			continue
		}

		riskLevel := string(plan.RiskLevel)
		if plan.RiskLevel == remediation.RiskCritical {
			riskLevel = string(remediation.RiskHigh)
		}

		status := "pending"
		if exec := engine.GetLatestExecutionForPlan(plan.ID); exec != nil {
			switch exec.Status {
			case remediation.StatusApproved:
				status = "approved"
			case remediation.StatusRunning:
				status = "executing"
			case remediation.StatusCompleted:
				status = "completed"
			case remediation.StatusFailed:
				status = "failed"
			case remediation.StatusRolledBack:
				status = "rolled_back"
			default:
				status = "pending"
			}
		}

		steps := make([]stepView, 0, len(plan.Steps))
		for _, step := range plan.Steps {
			action := step.Description
			if action == "" {
				action = step.Command
			}
			steps = append(steps, stepView{
				Order:           step.Order,
				Action:          action,
				Command:         step.Command,
				RollbackCommand: step.Rollback,
				RiskLevel:       riskLevel,
			})
		}

		result = append(result, planView{
			ID:          plan.ID,
			FindingID:   plan.FindingID,
			ResourceID:  plan.ResourceID,
			Title:       plan.Title,
			Description: plan.Description,
			Steps:       steps,
			RiskLevel:   riskLevel,
			Status:      status,
			CreatedAt:   plan.CreatedAt,
		})
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"plans": result,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write remediation plans response")
	}
}

// HandleGetRemediationPlan returns a specific remediation plan (GET /api/ai/remediation/plans/{id})
func (h *AISettingsHandler) HandleGetRemediationPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := h.GetRemediationEngine()
	if engine == nil {
		http.Error(w, "Remediation engine not available", http.StatusServiceUnavailable)
		return
	}

	planID := r.URL.Query().Get("plan_id")
	if planID == "" {
		http.Error(w, "plan_id is required", http.StatusBadRequest)
		return
	}

	plan := engine.GetPlan(planID)
	if plan == nil {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	if err := utils.WriteJSONResponse(w, plan); err != nil {
		log.Error().Err(err).Msg("Failed to write remediation plan response")
	}
}

// HandleApproveRemediationPlan approves a remediation plan (POST /api/ai/remediation/plans/{id}/approve)
func (h *AISettingsHandler) HandleApproveRemediationPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := h.GetRemediationEngine()
	if engine == nil {
		http.Error(w, "Remediation engine not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		PlanID     string `json:"plan_id"`
		ApprovedBy string `json:"approved_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PlanID == "" {
		http.Error(w, "plan_id is required", http.StatusBadRequest)
		return
	}

	if req.ApprovedBy == "" {
		req.ApprovedBy = "api"
	}

	execution, err := engine.ApprovePlan(req.PlanID, req.ApprovedBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success":   true,
		"message":   "Plan approved",
		"execution": execution,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write remediation approval response")
	}
}

// HandleExecuteRemediationPlan executes an approved remediation plan (POST /api/ai/remediation/plans/{id}/execute)
func (h *AISettingsHandler) HandleExecuteRemediationPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := h.GetRemediationEngine()
	if engine == nil {
		http.Error(w, "Remediation engine not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ExecutionID string `json:"execution_id"`
		PlanID      string `json:"plan_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ExecutionID == "" {
		if req.PlanID == "" {
			http.Error(w, "execution_id or plan_id is required", http.StatusBadRequest)
			return
		}
		// Auto-approve the plan if only plan_id is provided
		exec, err := engine.ApprovePlan(req.PlanID, "api")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.ExecutionID = exec.ID
	}

	if err := engine.Execute(r.Context(), req.ExecutionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	execution := engine.GetExecution(req.ExecutionID)

	// Launch background verification if execution completed successfully
	if execution != nil && execution.Status == remediation.StatusCompleted {
		plan := engine.GetPlan(execution.PlanID)
		aiSvc := h.GetAIService(r.Context())
		if plan != nil && plan.FindingID != "" && aiSvc != nil {
			go func() {
				time.Sleep(30 * time.Second)

				patrol := aiSvc.GetPatrolService()
				if patrol == nil {
					log.Warn().Str("findingID", plan.FindingID).Msg("[Remediation] Post-fix verification skipped: no patrol service")
					return
				}

				finding := patrol.GetFindings().Get(plan.FindingID)
				if finding == nil {
					log.Warn().Str("findingID", plan.FindingID).Msg("[Remediation] Post-fix verification skipped: finding not found")
					return
				}

				bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				verified, verifyErr := patrol.VerifyFixResolved(bgCtx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
				if verifyErr != nil {
					log.Error().Err(verifyErr).Str("findingID", plan.FindingID).Msg("[Remediation] Post-fix verification failed with error")
				} else if !verified {
					log.Warn().Str("findingID", plan.FindingID).Msg("[Remediation] Post-fix verification: issue persists")
				} else {
					log.Info().Str("findingID", plan.FindingID).Msg("[Remediation] Post-fix verification: issue resolved")
				}

				// Update execution status based on verification result
				if verifyErr != nil {
					engine.SetExecutionVerification(execution.ID, false, fmt.Sprintf("Verification error: %v", verifyErr))
				} else if !verified {
					engine.SetExecutionVerification(execution.ID, false, "Issue persists after fix")
				} else {
					engine.SetExecutionVerification(execution.ID, true, "Issue resolved")
				}
			}()
		}
	}

	if err := utils.WriteJSONResponse(w, execution); err != nil {
		log.Error().Err(err).Msg("Failed to write remediation execution response")
	}
}

// HandleRollbackRemediationPlan rolls back an executed remediation (POST /api/ai/remediation/plans/{id}/rollback)
func (h *AISettingsHandler) HandleRollbackRemediationPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := h.GetRemediationEngine()
	if engine == nil {
		http.Error(w, "Remediation engine not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ExecutionID string `json:"execution_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ExecutionID == "" {
		http.Error(w, "execution_id is required", http.StatusBadRequest)
		return
	}

	if err := engine.Rollback(r.Context(), req.ExecutionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Rollback initiated",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write remediation rollback response")
	}
}

// HandleGetCircuitBreakerStatus returns the circuit breaker status (GET /api/ai/circuit/status)
func (h *AISettingsHandler) HandleGetCircuitBreakerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	breaker := h.GetCircuitBreaker()
	if breaker == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"status":  "unknown",
			"message": "Circuit breaker not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write circuit breaker status response")
		}
		return
	}

	status := breaker.GetStatus()
	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"state":                status.State,
		"can_patrol":           breaker.CanAllow(), // Use read-only check to avoid state transitions
		"consecutive_failures": status.ConsecutiveFailures,
		"total_successes":      status.TotalSuccesses,
		"total_failures":       status.TotalFailures,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write circuit breaker status response")
	}
}

// ============================================
// Phase 7: Incident Recording API Handlers
// ============================================

// HandleGetRecentIncidents returns recent incident recording windows (GET /api/ai/incidents)
func (h *AISettingsHandler) HandleGetRecentIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query params
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Get coordinator status
	coordinator := h.GetIncidentCoordinator()
	var activeCount int
	if coordinator != nil {
		activeCount = coordinator.GetActiveIncidentCount()
	}

	// Get incident data from patrol service
	svc := h.GetAIService(r.Context())
	if svc == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"incidents":    []interface{}{},
			"active_count": activeCount,
			"message":      "Pulse Patrol service not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write incidents response")
		}
		return
	}

	patrol := svc.GetPatrolService()
	if patrol == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"incidents":    []interface{}{},
			"active_count": activeCount,
			"message":      "Patrol service not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write incidents response")
		}
		return
	}

	// Get incidents from incident store
	incidentStore := patrol.GetIncidentStore()
	if incidentStore == nil {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"incidents":    []interface{}{},
			"active_count": activeCount,
			"message":      "Incident store not available",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write incidents response")
		}
		return
	}

	// Get the resource ID filter if provided
	resourceID := r.URL.Query().Get("resource_id")

	var incidents interface{}
	if resourceID != "" {
		incidents = incidentStore.ListIncidentsByResource(resourceID, limit)
	} else {
		// No direct method to list all incidents, use FormatForPatrol for now
		// This is a limitation - we may want to add ListRecentIncidents to the store
		incidentSummary := incidentStore.FormatForPatrol(limit)
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"incidents":        []interface{}{},
			"incident_summary": incidentSummary,
			"active_count":     activeCount,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write incidents response")
		}
		return
	}

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"incidents":    incidents,
		"active_count": activeCount,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write incidents response")
	}
}

// HandleGetIncidentData returns incident data for a specific resource (GET /api/ai/incidents/{resourceID})
func (h *AISettingsHandler) HandleGetIncidentData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract resource ID from URL path
	path := r.URL.Path
	prefix := "/api/ai/incidents/"
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	resourceID := strings.TrimPrefix(path, prefix)
	if resourceID == "" {
		http.Error(w, "resource_id is required", http.StatusBadRequest)
		return
	}
	// URL-decode the resource ID (handles IDs with slashes like "node/pve")
	if decoded, err := url.PathUnescape(resourceID); err == nil {
		resourceID = decoded
	}

	// Get limit from query params
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	// Get incident data from patrol service
	svc := h.GetAIService(r.Context())
	if svc == nil {
		http.Error(w, "Pulse Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	patrol := svc.GetPatrolService()
	if patrol == nil {
		http.Error(w, "Patrol service not available", http.StatusServiceUnavailable)
		return
	}

	// Get incidents from incident store
	incidentStore := patrol.GetIncidentStore()
	if incidentStore == nil {
		http.Error(w, "Incident store not available", http.StatusServiceUnavailable)
		return
	}

	incidents := incidentStore.ListIncidentsByResource(resourceID, limit)

	// Also get formatted context for AI
	formattedContext := incidentStore.FormatForResource(resourceID, limit)

	if err := utils.WriteJSONResponse(w, map[string]interface{}{
		"resource_id":       resourceID,
		"incidents":         incidents,
		"formatted_context": formattedContext,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to write incident data response")
	}
}
