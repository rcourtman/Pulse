package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// UpdateDetectionHandlers manages API endpoints for infrastructure update detection.
// This is separate from UpdateHandlers which handles Pulse self-updates.
type UpdateDetectionHandlers struct {
	monitor *monitoring.Monitor
}

// NewUpdateDetectionHandlers creates a new update detection handlers group.
func NewUpdateDetectionHandlers(monitor *monitoring.Monitor) *UpdateDetectionHandlers {
	return &UpdateDetectionHandlers{monitor: monitor}
}

// ContainerUpdateInfo represents a container with an available update
type ContainerUpdateInfo struct {
	HostID          string `json:"hostId"`
	HostName        string `json:"hostName"`
	ContainerID     string `json:"containerId"`
	ContainerName   string `json:"containerName"`
	Image           string `json:"image"`
	CurrentDigest   string `json:"currentDigest,omitempty"`
	LatestDigest    string `json:"latestDigest,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	LastChecked     int64  `json:"lastChecked,omitempty"`
	Error           string `json:"error,omitempty"`
	ResourceType    string `json:"resourceType"`
}

// HandleGetInfraUpdates returns all tracked infrastructure updates with optional filtering.
// GET /api/infra-updates
//
//	?hostId=<id>         Filter by host
//	?resourceType=docker Filter by type (currently only docker supported)
func (h *UpdateDetectionHandlers) HandleGetInfraUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
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
	hostIDFilter := query.Get("hostId")
	resourceTypeFilter := strings.ToLower(query.Get("resourceType"))

	// Collect updates from Docker hosts
	updates := h.collectDockerUpdates(hostIDFilter)

	// Filter by resource type if specified
	if resourceTypeFilter != "" && resourceTypeFilter != "docker" {
		updates = []ContainerUpdateInfo{} // Only docker supported currently
	}

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

	if h.monitor == nil {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "No update found for resource", nil)
		return
	}

	// ResourceID format: docker:<hostId>/<containerId>
	updates := h.collectDockerUpdates("")
	for _, update := range updates {
		dockerResourceID := "docker:" + update.HostID + "/" + update.ContainerID
		if dockerResourceID == resourceID || update.ContainerID == resourceID {
			if err := utils.WriteJSONResponse(w, update); err != nil {
				log.Error().Err(err).Msg("Failed to serialize update response")
			}
			return
		}
	}

	writeErrorResponse(w, http.StatusNotFound, "not_found", "No update found for resource", nil)
}

// HandleGetInfraUpdatesSummary returns aggregated update statistics per host.
// GET /api/infra-updates/summary
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
		if err := utils.WriteJSONResponse(w, map[string]any{
			"summaries":    map[string]any{},
			"totalUpdates": 0,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty summary response")
		}
		return
	}

	updates := h.collectDockerUpdates("")

	// Aggregate by host
	summaries := make(map[string]map[string]any)
	for _, update := range updates {
		if _, ok := summaries[update.HostID]; !ok {
			summaries[update.HostID] = map[string]any{
				"hostId":     update.HostID,
				"hostName":   update.HostName,
				"totalCount": 0,
				"containers": 0,
			}
		}
		summaries[update.HostID]["totalCount"] = summaries[update.HostID]["totalCount"].(int) + 1
		summaries[update.HostID]["containers"] = summaries[update.HostID]["containers"].(int) + 1
	}

	response := map[string]any{
		"summaries":    summaries,
		"totalUpdates": len(updates),
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

	if h.monitor == nil {
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

	// Handle host-level check
	if req.HostID != "" {
		commandStatus, err := h.monitor.QueueDockerCheckUpdatesCommand(req.HostID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "check_updates_failed", err.Error(), nil)
			return
		}

		if err := utils.WriteJSONResponse(w, map[string]any{
			"success":   true,
			"commandId": commandStatus.ID,
			"hostId":    req.HostID,
			"message":   "Update check command queued for host",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize check response")
		}
		return
	}

	// Handle resource-level check (currently we just check the whole host)
	if req.ResourceID != "" {
		// Try to find which host this resource belongs to
		updates := h.collectDockerUpdates("")
		var hostID string
		for _, update := range updates {
			if update.ContainerID == req.ResourceID || ("docker:"+update.HostID+"/"+update.ContainerID) == req.ResourceID {
				hostID = update.HostID
				break
			}
		}

		if hostID == "" {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "Resource not found or has no update status", nil)
			return
		}

		commandStatus, err := h.monitor.QueueDockerCheckUpdatesCommand(hostID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "check_updates_failed", err.Error(), nil)
			return
		}

		if err := utils.WriteJSONResponse(w, map[string]any{
			"success":   true,
			"commandId": commandStatus.ID,
			"hostId":    hostID,
			"message":   "Update check command queued for host",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize check response")
		}
		return
	}

	writeErrorResponse(w, http.StatusBadRequest, "missing_params", "Either hostId or resourceId is required", nil)
}

// HandleGetInfraUpdatesForHost returns all updates for a specific host.
// GET /api/infra-updates/host/{hostId}
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesForHost(w http.ResponseWriter, r *http.Request, hostID string) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
		if err := utils.WriteJSONResponse(w, map[string]any{
			"updates": []any{},
			"total":   0,
			"hostId":  hostID,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty host updates response")
		}
		return
	}

	updates := h.collectDockerUpdates(hostID)

	response := map[string]any{
		"updates": updates,
		"total":   len(updates),
		"hostId":  hostID,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host updates response")
	}
}

// collectDockerUpdates gathers update information from Docker hosts
func (h *UpdateDetectionHandlers) collectDockerUpdates(hostIDFilter string) []ContainerUpdateInfo {
	var updates []ContainerUpdateInfo

	state := h.monitor.GetState()

	for _, host := range state.DockerHosts {
		// Filter by host ID if specified
		if hostIDFilter != "" && host.ID != hostIDFilter {
			continue
		}

		for _, container := range host.Containers {
			if container.UpdateStatus == nil {
				continue
			}

			// Only include containers with updates available or errors
			if !container.UpdateStatus.UpdateAvailable && container.UpdateStatus.Error == "" {
				continue
			}

			update := ContainerUpdateInfo{
				HostID:          host.ID,
				HostName:        host.DisplayName,
				ContainerID:     container.ID,
				ContainerName:   strings.TrimPrefix(container.Name, "/"),
				Image:           container.Image,
				UpdateAvailable: container.UpdateStatus.UpdateAvailable,
				ResourceType:    "docker",
			}

			if container.UpdateStatus.CurrentDigest != "" {
				update.CurrentDigest = container.UpdateStatus.CurrentDigest
			}
			if container.UpdateStatus.LatestDigest != "" {
				update.LatestDigest = container.UpdateStatus.LatestDigest
			}
			if !container.UpdateStatus.LastChecked.IsZero() {
				update.LastChecked = container.UpdateStatus.LastChecked.Unix()
			}
			if container.UpdateStatus.Error != "" {
				update.Error = container.UpdateStatus.Error
			}

			updates = append(updates, update)
		}
	}

	return updates
}
