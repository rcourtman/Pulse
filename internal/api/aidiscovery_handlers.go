package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/aidiscovery"
	"github.com/rs/zerolog/log"
)

// AIDiscoveryHandlers handles AI-powered infrastructure discovery endpoints.
type AIDiscoveryHandlers struct {
	service *aidiscovery.Service
}

// NewAIDiscoveryHandlers creates new AI discovery handlers.
func NewAIDiscoveryHandlers(service *aidiscovery.Service) *AIDiscoveryHandlers {
	return &AIDiscoveryHandlers{
		service: service,
	}
}

// SetService sets the discovery service (used for late initialization after routes are registered).
func (h *AIDiscoveryHandlers) SetService(service *aidiscovery.Service) {
	h.service = service
}

// writeDiscoveryJSON writes a JSON response.
func writeDiscoveryJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeDiscoveryError writes a JSON error response.
func writeDiscoveryError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"error":   true,
		"message": message,
	})
}

// HandleListDiscoveries handles GET /api/aidiscovery
func (h *AIDiscoveryHandlers) HandleListDiscoveries(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	discoveries, err := h.service.ListDiscoveries()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list discoveries")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	// Convert to summaries for list view
	summaries := make([]aidiscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
	})
}

// HandleGetDiscovery handles GET /api/aidiscovery/{type}/{host}/{id}
func (h *AIDiscoveryHandlers) HandleGetDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path: /api/aidiscovery/{type}/{host}/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path: expected /api/aidiscovery/{type}/{host}/{id}")
		return
	}

	resourceType := aidiscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	discovery, err := h.service.GetDiscoveryByResource(resourceType, hostID, resourceID)
	if err != nil {
		log.Error().Err(err).Str("type", string(resourceType)).Str("host", hostID).Str("id", resourceID).Msg("Failed to get discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to get discovery")
		return
	}

	if discovery == nil {
		writeDiscoveryError(w, http.StatusNotFound, "Discovery not found")
		return
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleTriggerDiscovery handles POST /api/aidiscovery/{type}/{host}/{id}
func (h *AIDiscoveryHandlers) HandleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path: expected /api/aidiscovery/{type}/{host}/{id}")
		return
	}

	resourceType := aidiscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	// Parse optional request body for force flag and hostname
	var reqBody struct {
		Force    bool   `json:"force"`
		Hostname string `json:"hostname"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}

	// Build discovery request
	req := aidiscovery.DiscoveryRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		HostID:       hostID,
		Hostname:     reqBody.Hostname,
		Force:        reqBody.Force,
	}

	// If hostname not provided, try to use hostID
	if req.Hostname == "" {
		req.Hostname = hostID
	}

	discovery, err := h.service.DiscoverResource(r.Context(), req)
	if err != nil {
		log.Error().Err(err).
			Str("type", string(resourceType)).
			Str("host", hostID).
			Str("id", resourceID).
			Msg("Failed to trigger discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Discovery failed: "+err.Error())
		return
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleUpdateNotes handles PUT /api/aidiscovery/{type}/{host}/{id}/notes
func (h *AIDiscoveryHandlers) HandleUpdateNotes(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/")
	path = strings.TrimSuffix(path, "/notes")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := aidiscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	// Build the full ID
	id := aidiscovery.MakeResourceID(resourceType, hostID, resourceID)

	// Parse request body
	var req aidiscovery.UpdateNotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.service.UpdateNotes(id, req.UserNotes, req.UserSecrets); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update notes")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to update notes: "+err.Error())
		return
	}

	// Return updated discovery
	discovery, err := h.service.GetDiscovery(id)
	if err != nil {
		writeDiscoveryError(w, http.StatusInternalServerError, "Notes updated but failed to fetch result")
		return
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleDeleteDiscovery handles DELETE /api/aidiscovery/{type}/{host}/{id}
func (h *AIDiscoveryHandlers) HandleDeleteDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := aidiscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	id := aidiscovery.MakeResourceID(resourceType, hostID, resourceID)

	if err := h.service.DeleteDiscovery(id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to delete discovery")
		return
	}

	writeDiscoveryJSON(w, map[string]any{"success": true, "id": id})
}

// HandleGetProgress handles GET /api/aidiscovery/{type}/{host}/{id}/progress
func (h *AIDiscoveryHandlers) HandleGetProgress(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/")
	path = strings.TrimSuffix(path, "/progress")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := aidiscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	id := aidiscovery.MakeResourceID(resourceType, hostID, resourceID)

	progress := h.service.GetProgress(id)
	if progress == nil {
		// Not currently scanning - check if we have a discovery
		discovery, err := h.service.GetDiscovery(id)
		if err == nil && discovery != nil {
			writeDiscoveryJSON(w, map[string]any{
				"status":      "completed",
				"resource_id": id,
				"updated_at":  discovery.UpdatedAt,
			})
			return
		}

		writeDiscoveryJSON(w, map[string]any{
			"status":      "not_started",
			"resource_id": id,
		})
		return
	}

	writeDiscoveryJSON(w, progress)
}

// HandleGetStatus handles GET /api/aidiscovery/status
func (h *AIDiscoveryHandlers) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	writeDiscoveryJSON(w, h.service.GetStatus())
}

// HandleListByType handles GET /api/aidiscovery/type/{type}
func (h *AIDiscoveryHandlers) HandleListByType(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/type/")
	resourceType := aidiscovery.ResourceType(path)

	discoveries, err := h.service.ListDiscoveriesByType(resourceType)
	if err != nil {
		log.Error().Err(err).Str("type", string(resourceType)).Msg("Failed to list discoveries by type")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	summaries := make([]aidiscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
		"type":        resourceType,
	})
}

// HandleListByHost handles GET /api/aidiscovery/host/{host}
func (h *AIDiscoveryHandlers) HandleListByHost(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "AI discovery service not configured")
		return
	}

	// Parse path
	hostID := strings.TrimPrefix(r.URL.Path, "/api/aidiscovery/host/")

	discoveries, err := h.service.ListDiscoveriesByHost(hostID)
	if err != nil {
		log.Error().Err(err).Str("host", hostID).Msg("Failed to list discoveries by host")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	summaries := make([]aidiscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
		"host":        hostID,
	})
}
