package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rs/zerolog/log"
)

type ConversionHandlers struct {
	recorder *conversion.Recorder
	health   *conversion.PipelineHealth
	config   *conversion.CollectionConfig
	store    *conversion.ConversionStore
}

func NewConversionHandlers(recorder *conversion.Recorder, health *conversion.PipelineHealth, config *conversion.CollectionConfig, store *conversion.ConversionStore) *ConversionHandlers {
	return &ConversionHandlers{
		recorder: recorder,
		health:   health,
		config:   config,
		store:    store,
	}
}

func (h *ConversionHandlers) HandleRecordEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event conversion.ConversionEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		conversion.GetConversionMetrics().RecordInvalid("invalid_request_body")
		writeConversionValidationError(w, "invalid request body")
		return
	}

	if err := event.Validate(); err != nil {
		conversion.GetConversionMetrics().RecordInvalid(conversionValidationReason(err))
		writeConversionValidationError(w, err.Error())
		return
	}

	// Persisted conversion telemetry must always be tenant-aware; use request context when clients omit org_id.
	if strings.TrimSpace(event.OrgID) == "" {
		event.OrgID = GetOrgID(r.Context())
	}

	if h != nil && h.config != nil && !h.config.IsSurfaceEnabled(event.Surface) {
		reason := "surface_disabled"
		if !h.config.IsEnabled() {
			reason = "collection_disabled"
		}
		conversion.GetConversionMetrics().RecordSkipped(reason)
		writeConversionAccepted(w)
		return
	}

	if h != nil && h.recorder != nil {
		if err := h.recorder.Record(event); err != nil {
			// Analytics ingestion is fire-and-forget; do not fail UX if recording fails.
			log.Warn().Err(err).Str("event_type", event.Type).Msg("Failed to record conversion event")
		} else {
			conversion.GetConversionMetrics().RecordEvent(event.Type, event.Surface)
			if h.health != nil {
				h.health.RecordEvent(event.Type)
			}
		}
	}

	writeConversionAccepted(w)
}

// HandleConversionFunnel returns conversion funnel counts for admin reporting.
// GET /api/admin/conversion-funnel?org_id=...&from=...&to=...
func (h *ConversionHandlers) HandleConversionFunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h == nil || h.store == nil {
		http.Error(w, "Conversion store unavailable", http.StatusServiceUnavailable)
		return
	}

	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))

	now := time.Now().UTC()
	to, err := parseOptionalTimeParam(r.URL.Query().Get("to"), now)
	if err != nil {
		http.Error(w, "invalid to", http.StatusBadRequest)
		return
	}
	fromDefault := to.Add(-30 * 24 * time.Hour)
	from, err := parseOptionalTimeParam(r.URL.Query().Get("from"), fromDefault)
	if err != nil {
		http.Error(w, "invalid from", http.StatusBadRequest)
		return
	}
	if from.After(to) {
		http.Error(w, "from must be <= to", http.StatusBadRequest)
		return
	}

	summary, err := h.store.FunnelSummary(orgID, from, to)
	if err != nil {
		http.Error(w, "failed to query funnel", http.StatusInternalServerError)
		return
	}

	writeConversionJSONResponse(w, http.StatusOK, summary, "HandleConversionFunnel")
}

// HandleGetHealth returns conversion pipeline health status.
func (h *ConversionHandlers) HandleGetHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := conversion.HealthStatus{
		Status:              "healthy",
		LastEventAgeSeconds: 0,
		EventsTotal:         0,
		EventsByType:        map[string]int64{},
		StartedAt:           time.Now().UnixMilli(),
	}
	if h != nil && h.health != nil {
		status = h.health.CheckHealth()
	}

	writeConversionJSONResponse(w, http.StatusOK, status, "HandleGetHealth")
}

type conversionStatsBucket struct {
	Type       string `json:"type"`
	Key        string `json:"key"`
	Count      int64  `json:"count"`
	TotalValue int64  `json:"total_value"`
}

type conversionStatsResponse struct {
	WindowStart int64                   `json:"window_start"`
	WindowEnd   int64                   `json:"window_end"`
	Buckets     []conversionStatsBucket `json:"buckets"`
	TotalEvents int64                   `json:"total_events"`
}

// HandleGetStats returns a snapshot of current conversion aggregation window state.
func (h *ConversionHandlers) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := []conversionStatsBucket{}
	windowStart := time.Now()
	windowEnd := windowStart
	var totalEvents int64

	if h != nil && h.recorder != nil {
		buckets := h.recorder.Snapshot()
		snapshot = make([]conversionStatsBucket, 0, len(buckets))
		if len(buckets) > 0 {
			windowStart = buckets[0].WindowStart
			windowEnd = buckets[0].WindowEnd
		}
		for _, bucket := range buckets {
			snapshot = append(snapshot, conversionStatsBucket{
				Type:       string(bucket.Type),
				Key:        bucket.Key,
				Count:      bucket.Count,
				TotalValue: bucket.TotalValue,
			})
			totalEvents += bucket.Count
		}
	}

	writeConversionJSONResponse(w, http.StatusOK, conversionStatsResponse{
		WindowStart: windowStart.UnixMilli(),
		WindowEnd:   windowEnd.UnixMilli(),
		Buckets:     snapshot,
		TotalEvents: totalEvents,
	}, "HandleGetStats")
}

// HandleGetConfig returns runtime conversion collection controls.
func (h *ConversionHandlers) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := conversion.CollectionConfigSnapshot{
		Enabled:          true,
		DisabledSurfaces: []string{},
	}
	if h != nil && h.config != nil {
		snapshot = h.config.GetConfig()
	}

	writeConversionJSONResponse(w, http.StatusOK, snapshot, "HandleGetConfig")
}

// HandleUpdateConfig updates runtime conversion collection controls.
func (h *ConversionHandlers) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var snapshot conversion.CollectionConfigSnapshot
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if h != nil {
		if h.config == nil {
			h.config = conversion.NewCollectionConfig()
		}
		h.config.UpdateConfig(snapshot)
		snapshot = h.config.GetConfig()
	} else {
		cfg := conversion.NewCollectionConfig()
		cfg.UpdateConfig(snapshot)
		snapshot = cfg.GetConfig()
	}

	writeConversionJSONResponse(w, http.StatusOK, snapshot, "HandleUpdateConfig")
}

func conversionValidationReason(err error) string {
	if err == nil {
		return "unknown"
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "type is required"):
		return "missing_type"
	case strings.Contains(msg, "is not supported"):
		return "unsupported_type"
	case strings.Contains(msg, "surface is required"):
		return "missing_surface"
	case strings.Contains(msg, "timestamp is required"):
		return "missing_timestamp"
	case strings.Contains(msg, "idempotency_key is required"):
		return "missing_idempotency_key"
	case strings.Contains(msg, "tenant_mode must be"):
		return "invalid_tenant_mode"
	case strings.Contains(msg, "capability is required"):
		return "missing_capability"
	case strings.Contains(msg, "limit_key is required"):
		return "missing_limit_key"
	default:
		return "validation_error"
	}
}

func writeConversionValidationError(w http.ResponseWriter, message string) {
	writeConversionJSONResponse(w, http.StatusBadRequest, map[string]string{
		"error":   "validation_error",
		"message": message,
	}, "writeConversionValidationError")
}

func writeConversionAccepted(w http.ResponseWriter) {
	writeConversionJSONResponse(w, http.StatusAccepted, map[string]bool{"accepted": true}, "writeConversionAccepted")
}

func writeConversionJSONResponse(w http.ResponseWriter, status int, payload interface{}, operation string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Warn().
			Err(err).
			Str("operation", operation).
			Msg("Failed to encode conversion API response")
	}
}

func parseOptionalTimeParam(raw string, defaultValue time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue.UTC(), nil
	}

	// RFC3339 / RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}

	// Date-only (local midnight is ambiguous; use UTC midnight).
	if t, err := time.ParseInLocation("2006-01-02", raw, time.UTC); err == nil {
		return t.UTC(), nil
	}

	// Unix seconds or milliseconds.
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		// Heuristic: >= 10^12 is likely ms.
		if i >= 1_000_000_000_000 {
			return time.UnixMilli(i).UTC(), nil
		}
		return time.Unix(i, 0).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("unsupported time format")
}
