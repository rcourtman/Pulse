package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rs/zerolog/log"
)

type ConversionHandlers struct {
	recorder *conversion.Recorder
	health   *conversion.PipelineHealth
	config   *conversion.CollectionConfig
}

func NewConversionHandlers(recorder *conversion.Recorder, health *conversion.PipelineHealth, config *conversion.CollectionConfig) *ConversionHandlers {
	return &ConversionHandlers{
		recorder: recorder,
		health:   health,
		config:   config,
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(conversionStatsResponse{
		WindowStart: windowStart.UnixMilli(),
		WindowEnd:   windowEnd.UnixMilli(),
		Buckets:     snapshot,
		TotalEvents: totalEvents,
	})
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snapshot)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(snapshot)
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "validation_error",
		"message": message,
	})
}

func writeConversionAccepted(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}
