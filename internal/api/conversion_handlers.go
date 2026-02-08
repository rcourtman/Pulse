package api

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/conversion"
	"github.com/rs/zerolog/log"
)

type ConversionHandlers struct {
	recorder *conversion.Recorder
}

func NewConversionHandlers(recorder *conversion.Recorder) *ConversionHandlers {
	return &ConversionHandlers{recorder: recorder}
}

func (h *ConversionHandlers) HandleRecordEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event conversion.ConversionEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeConversionValidationError(w, "invalid request body")
		return
	}

	if err := event.Validate(); err != nil {
		writeConversionValidationError(w, err.Error())
		return
	}

	if h != nil && h.recorder != nil {
		if err := h.recorder.Record(event); err != nil {
			// Analytics ingestion is fire-and-forget; do not fail UX if recording fails.
			log.Warn().Err(err).Str("event_type", event.Type).Msg("Failed to record conversion event")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}

func writeConversionValidationError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "validation_error",
		"message": message,
	})
}
