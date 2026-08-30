package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	pkgmetrics "github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

const (
	availabilityHistoryMaxTargets = 200
	availabilityHistoryMaxBuckets = 120
)

var availabilityHistoryRanges = map[string]time.Duration{
	"1h":   time.Hour,
	"6h":   6 * time.Hour,
	"12h":  12 * time.Hour,
	"24h":  24 * time.Hour,
	"7d":   7 * 24 * time.Hour,
	"14d":  14 * 24 * time.Hour,
	"30d":  30 * 24 * time.Hour,
	"90d":  90 * 24 * time.Hour,
	"365d": 365 * 24 * time.Hour,
}

type availabilityHistoryRequest struct {
	TargetIDs []string `json:"targetIds"`
	Range     string   `json:"range,omitempty"`
}

type availabilityHistoryTargetError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type availabilityHistoryTargetResponse struct {
	TargetID           string                                    `json:"targetId"`
	Summary            *pkgmetrics.AvailabilityHistorySummary    `json:"summary,omitempty"`
	Buckets            []pkgmetrics.AvailabilityHistoryBucket    `json:"buckets,omitempty"`
	RevisionBoundaries []pkgmetrics.AvailabilityRevisionBoundary `json:"revisionBoundaries,omitempty"`
	Error              *availabilityHistoryTargetError           `json:"error,omitempty"`
}

type availabilityHistoryResponse struct {
	Start   time.Time                           `json:"start"`
	End     time.Time                           `json:"end"`
	Targets []availabilityHistoryTargetResponse `json:"targets"`
}

func (r *Router) handleAvailabilityHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, 32*1024)
	defer req.Body.Close()
	var body availabilityHistoryRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid availability history request", nil)
		return
	}
	body.TargetIDs = normalizeAvailabilityHistoryIDs(body.TargetIDs)
	if len(body.TargetIDs) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "missing_target_ids", "At least one availability target ID is required", nil)
		return
	}
	if len(body.TargetIDs) > availabilityHistoryMaxTargets {
		writeErrorResponse(w, http.StatusBadRequest, "too_many_target_ids", "Availability history accepts at most 200 unique target IDs", nil)
		return
	}

	rangeName := strings.ToLower(strings.TrimSpace(body.Range))
	if rangeName == "" {
		rangeName = "24h"
	}
	duration, ok := availabilityHistoryRanges[rangeName]
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_range", "Unsupported availability history range", nil)
		return
	}
	maxHistoryDays := freeHistoryDaysDefault
	if r.licenseHandlers != nil {
		if service := r.licenseHandlers.Service(req.Context()); service != nil {
			status := service.Status()
			if status.Valid {
				maxHistoryDays = tierHistoryDaysFromLicensing(status.Tier)
			}
		}
	}
	if duration > time.Duration(maxHistoryDays)*24*time.Hour {
		WriteLicenseRequired(w, featureLongTermMetricsValue, "Extended availability history requires a higher-tier Pulse license")
		return
	}

	configured := make(map[string]config.AvailabilityTarget)
	if mock.IsMockEnabled() {
		for _, response := range mockAvailabilityTargetResponses() {
			target := config.NormalizeAvailabilityTarget(response.AvailabilityTarget)
			configured[target.ID] = target
		}
	} else {
		persistence := r.persistenceForOrg(req.Context())
		if persistence == nil {
			writeErrorResponse(w, http.StatusInternalServerError, "availability_unavailable", "Availability target persistence is unavailable", nil)
			return
		}
		targets, err := persistence.LoadAvailabilityTargets()
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", nil)
			return
		}
		for _, target := range targets {
			target = config.NormalizeAvailabilityTarget(target)
			configured[target.ID] = target
		}
	}

	queryIDs := make([]string, 0, len(body.TargetIDs))
	for _, targetID := range body.TargetIDs {
		if _, exists := configured[targetID]; exists {
			queryIDs = append(queryIDs, targetID)
		}
	}
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-duration)

	var results map[string]pkgmetrics.AvailabilityHistoryTarget
	if mock.IsMockEnabled() {
		results = mockAvailabilityHistory(queryIDs, start, end)
	} else {
		monitor := r.getTenantMonitor(req.Context())
		if monitor == nil || monitor.GetMetricsStore() == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, "availability_history_unavailable", "Availability history is unavailable", nil)
			return
		}
		var err error
		results, err = monitor.GetMetricsStore().QueryAvailabilityHistory(queryIDs, start, end, availabilityHistoryMaxBuckets)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "availability_history_failed", "Failed to load availability history", nil)
			return
		}
	}

	response := availabilityHistoryResponse{Start: start, End: end, Targets: make([]availabilityHistoryTargetResponse, 0, len(body.TargetIDs))}
	for _, targetID := range body.TargetIDs {
		if _, exists := configured[targetID]; !exists {
			response.Targets = append(response.Targets, availabilityHistoryTargetResponse{
				TargetID: targetID,
				Error:    &availabilityHistoryTargetError{Code: "not_found", Message: "Availability target not found"},
			})
			continue
		}
		history := results[targetID]
		response.Targets = append(response.Targets, availabilityHistoryTargetResponse{
			TargetID:           targetID,
			Summary:            &history.Summary,
			Buckets:            history.Buckets,
			RevisionBoundaries: history.RevisionBoundaries,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func mockAvailabilityHistory(targetIDs []string, start, end time.Time) map[string]pkgmetrics.AvailabilityHistoryTarget {
	const bucketCount = 120
	result := make(map[string]pkgmetrics.AvailabilityHistoryTarget, len(targetIDs))
	step := end.Sub(start) / bucketCount
	for targetIndex, targetID := range targetIDs {
		buckets := make([]pkgmetrics.AvailabilityHistoryBucket, 0, bucketCount)
		summary := pkgmetrics.AvailabilityHistorySummary{}
		latencyCount, latencySum := int64(0), float64(0)
		latencyMin, latencyMax := int64(0), int64(0)
		for bucketIndex := 0; bucketIndex < bucketCount; bucketIndex++ {
			bucketStart := start.Add(time.Duration(bucketIndex) * step)
			bucketEnd := bucketStart.Add(step)
			seconds := bucketEnd.Sub(bucketStart).Seconds()
			bucket := pkgmetrics.AvailabilityHistoryBucket{Start: bucketStart, End: bucketEnd}
			pattern := (bucketIndex + targetIndex*7) % 53
			switch {
			case pattern == 0 || pattern == 1:
				bucket.UnknownSeconds = seconds
				summary.UnknownSeconds += seconds
			case pattern == 9:
				bucket.IndeterminateSeconds = seconds
				summary.IndeterminateSeconds += seconds
			case (targetIndex%5 == 0 && pattern >= 30 && pattern <= 33) || pattern == 21:
				bucket.UnreachableSeconds = seconds
				summary.UnreachableSeconds += seconds
			default:
				bucket.ReachableSeconds = seconds
				summary.ReachableSeconds += seconds
				latency := int64(8 + (bucketIndex*3+targetIndex*5)%44)
				bucket.LatencyMillis = &pkgmetrics.AvailabilityLatencySummary{Average: float64(latency), Min: latency, Max: latency}
				latencyCount++
				latencySum += float64(latency)
				if latencyMin == 0 || latency < latencyMin {
					latencyMin = latency
				}
				if latency > latencyMax {
					latencyMax = latency
				}
			}
			buckets = append(buckets, bucket)
		}
		known := summary.ReachableSeconds + summary.UnreachableSeconds + summary.IndeterminateSeconds
		summary.CoveragePercent = math.Round(known*10000/end.Sub(start).Seconds()) / 100
		determinate := summary.ReachableSeconds + summary.UnreachableSeconds
		if determinate > 0 {
			availability := math.Round(summary.ReachableSeconds*10000/determinate) / 100
			summary.AvailabilityPercent = &availability
		}
		if latencyCount > 0 {
			summary.ReachableLatencyMillis = &pkgmetrics.AvailabilityLatencySummary{
				Average: math.Round(latencySum/float64(latencyCount)*100) / 100,
				Min:     latencyMin,
				Max:     latencyMax,
			}
		}
		boundaries := []pkgmetrics.AvailabilityRevisionBoundary{}
		if targetIndex%9 == 0 {
			boundaries = append(boundaries, pkgmetrics.AvailabilityRevisionBoundary{Revision: 2, At: start.Add(end.Sub(start) / 2)})
		}
		result[targetID] = pkgmetrics.AvailabilityHistoryTarget{
			TargetID:           targetID,
			Summary:            summary,
			Buckets:            buckets,
			RevisionBoundaries: boundaries,
		}
	}
	return result
}

func normalizeAvailabilityHistoryIDs(targetIDs []string) []string {
	seen := make(map[string]struct{}, len(targetIDs))
	ids := make([]string, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		targetID = strings.TrimSpace(targetID)
		if targetID == "" {
			continue
		}
		if _, exists := seen[targetID]; exists {
			continue
		}
		seen[targetID] = struct{}{}
		ids = append(ids, targetID)
	}
	return ids
}
