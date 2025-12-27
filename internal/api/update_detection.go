package api

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/updatedetection"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// UpdateDetectionHandlers manages API endpoints for infrastructure update detection.
// This is separate from UpdateHandlers which handles Pulse self-updates.
type UpdateDetectionHandlers struct {
	manager *updatedetection.Manager
}

// NewUpdateDetectionHandlers creates a new update detection handlers group.
func NewUpdateDetectionHandlers(manager *updatedetection.Manager) *UpdateDetectionHandlers {
	return &UpdateDetectionHandlers{manager: manager}
}

// HandleGetInfraUpdates returns all tracked infrastructure updates with optional filtering.
// GET /api/infra-updates
//
//	?hostId=<id>         Filter by host
//	?resourceType=docker Filter by type
//	?severity=security   Filter by severity
func (h *UpdateDetectionHandlers) HandleGetInfraUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.manager == nil {
		if err := utils.WriteJSONResponse(w, map[string]any{
			"updates": []any{},
			"total":   0,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty updates response")
		}
		return
	}

	// Parse query filters
	query := r.URL.Query()
	filters := updatedetection.UpdateFilters{
		HostID:       query.Get("hostId"),
		ResourceType: query.Get("resourceType"),
	}

	if severity := query.Get("severity"); severity != "" {
		filters.Severity = updatedetection.UpdateSeverity(severity)
	}
	if updateType := query.Get("type"); updateType != "" {
		filters.UpdateType = updatedetection.UpdateType(updateType)
	}

	updates := h.manager.GetUpdates(filters)

	response := map[string]any{
		"updates": updates,
		"total":   len(updates),
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize updates response")
	}
}

// HandleGetInfraUpdateForResource returns the update status for a specific resource.
// GET /api/infra-updates/{resourceId}
func (h *UpdateDetectionHandlers) HandleGetInfraUpdateForResource(w http.ResponseWriter, r *http.Request, resourceID string) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.manager == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No update found for resource", nil)
		return
	}

	update := h.manager.GetUpdatesForResource(resourceID)
	if update == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No update found for resource", nil)
		return
	}

	if err := utils.WriteJSONResponse(w, update); err != nil {
		log.Error().Err(err).Msg("Failed to serialize update response")
	}
}

// HandleGetInfraUpdatesSummary returns aggregated update statistics per host.
// GET /api/infra-updates/summary
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.manager == nil {
		if err := utils.WriteJSONResponse(w, map[string]any{
			"summaries":    map[string]any{},
			"totalUpdates": 0,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty summary response")
		}
		return
	}

	summaries := h.manager.GetSummary()
	totalUpdates := h.manager.GetTotalCount()

	response := map[string]any{
		"summaries":    summaries,
		"totalUpdates": totalUpdates,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize summary response")
	}
}

// HandleTriggerInfraUpdateCheck triggers an update check for a specific resource or host.
// POST /api/infra-updates/check
//
//	{ "hostId": "xxx" }       Check all on host
//	{ "resourceId": "xxx" }   Check specific resource
func (h *UpdateDetectionHandlers) HandleTriggerInfraUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	if h.manager == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "service_unavailable", "Update detection not available", nil)
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var req struct {
		HostID     string `json:"hostId"`
		ResourceID string `json:"resourceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	// For now, return a placeholder response - the actual check will be performed
	// by agents on their next cycle or when we add server-side registry checking
	response := map[string]any{
		"success": true,
		"message": "Update check queued",
	}

	if req.HostID != "" {
		response["hostId"] = req.HostID
	}
	if req.ResourceID != "" {
		response["resourceId"] = req.ResourceID
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize check response")
	}
}

// HandleGetInfraUpdatesForHost returns all updates for a specific host.
// GET /api/infra-updates/host/{hostId}
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesForHost(w http.ResponseWriter, r *http.Request, hostID string) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.manager == nil {
		if err := utils.WriteJSONResponse(w, map[string]any{
			"updates": []any{},
			"total":   0,
			"hostId":  hostID,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty host updates response")
		}
		return
	}

	updates := h.manager.GetUpdatesForHost(hostID)

	response := map[string]any{
		"updates": updates,
		"total":   len(updates),
		"hostId":  hostID,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host updates response")
	}
}
