package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// UpdateDetectionHandlers manages API endpoints for infrastructure update detection.
// This is separate from UpdateHandlers which handles Pulse self-updates.
type UpdateDetectionHandlers struct {
	monitor   *monitoring.Monitor
	readState unifiedresources.ReadState
}

// NewUpdateDetectionHandlers creates a new update detection handlers group.
func NewUpdateDetectionHandlers(monitor *monitoring.Monitor, readState unifiedresources.ReadState) *UpdateDetectionHandlers {
	return &UpdateDetectionHandlers{monitor: monitor, readState: readState}
}

// ContainerUpdateInfo represents a container with an available update
type ContainerUpdateInfo struct {
	AgentID         string                  `json:"agentId"`
	AgentName       string                  `json:"agentName"`
	ContainerID     string                  `json:"containerId"`
	ContainerName   string                  `json:"containerName"`
	Image           string                  `json:"image"`
	CurrentDigest   string                  `json:"currentDigest,omitempty"`
	LatestDigest    string                  `json:"latestDigest,omitempty"`
	UpdateAvailable bool                    `json:"updateAvailable"`
	LastChecked     int64                   `json:"lastChecked,omitempty"`
	Error           string                  `json:"error,omitempty"`
	ResourceType    InfraUpdateResourceType `json:"resourceType"`
}

type InfraUpdateResourceType string

const infraUpdateResourceTypeDocker InfraUpdateResourceType = "docker"

type infraUpdatesResponse struct {
	Updates []ContainerUpdateInfo `json:"updates"`
	Total   int                   `json:"total"`
}

func emptyInfraUpdatesResponse() infraUpdatesResponse {
	return infraUpdatesResponse{}.normalizeCollections()
}

func (r infraUpdatesResponse) normalizeCollections() infraUpdatesResponse {
	if r.Updates == nil {
		r.Updates = []ContainerUpdateInfo{}
	}
	return r
}

type infraUpdateAgentSummary struct {
	AgentID    string `json:"agentId"`
	AgentName  string `json:"agentName"`
	TotalCount int    `json:"totalCount"`
	Containers int    `json:"containers"`
}

type infraUpdatesSummaryResponse struct {
	Summaries    map[string]infraUpdateAgentSummary `json:"summaries"`
	TotalUpdates int                                `json:"totalUpdates"`
}

func emptyInfraUpdatesSummaryResponse() infraUpdatesSummaryResponse {
	return infraUpdatesSummaryResponse{}.normalizeCollections()
}

func (r infraUpdatesSummaryResponse) normalizeCollections() infraUpdatesSummaryResponse {
	if r.Summaries == nil {
		r.Summaries = map[string]infraUpdateAgentSummary{}
	}
	return r
}

type infraUpdatesForAgentResponse struct {
	Updates []ContainerUpdateInfo `json:"updates"`
	Total   int                   `json:"total"`
	AgentID string                `json:"agentId"`
}

func emptyInfraUpdatesForAgentResponse() infraUpdatesForAgentResponse {
	return infraUpdatesForAgentResponse{}.normalizeCollections()
}

func (r infraUpdatesForAgentResponse) normalizeCollections() infraUpdatesForAgentResponse {
	if r.Updates == nil {
		r.Updates = []ContainerUpdateInfo{}
	}
	return r
}

type triggerInfraUpdateCheckResponse struct {
	Success   bool   `json:"success"`
	CommandID string `json:"commandId"`
	AgentID   string `json:"agentId"`
	Message   string `json:"message"`
}

// HandleGetInfraUpdates returns all tracked infrastructure updates with optional filtering.
// GET /api/infra-updates
//
//	?agentId=<id>        Filter by agent
//	?resourceType=docker Filter by type (currently only docker supported)
func (h *UpdateDetectionHandlers) HandleGetInfraUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
		if err := utils.WriteJSONResponse(w, emptyInfraUpdatesResponse()); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty updates response")
		}
		return
	}

	// Parse query filters
	query := r.URL.Query()
	agentIDFilter := query.Get("agentId")
	resourceTypeFilter := strings.ToLower(query.Get("resourceType"))

	// Collect updates from Docker agents
	updates := h.collectDockerUpdates(agentIDFilter)

	// Filter by resource type if specified
	if resourceTypeFilter != "" && resourceTypeFilter != string(infraUpdateResourceTypeDocker) {
		updates = []ContainerUpdateInfo{} // Only docker supported currently
	}

	response := emptyInfraUpdatesResponse()
	response.Updates = updates
	response.Total = len(updates)

	if err := utils.WriteJSONResponse(w, response.normalizeCollections()); err != nil {
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

	// ResourceID format: docker:<agentId>/<containerId>
	updates := h.collectDockerUpdates("")
	for _, update := range updates {
		dockerResourceID := "docker:" + update.AgentID + "/" + update.ContainerID
		if dockerResourceID == resourceID || update.ContainerID == resourceID {
			if err := utils.WriteJSONResponse(w, update); err != nil {
				log.Error().Err(err).Msg("Failed to serialize update response")
			}
			return
		}
	}

	writeErrorResponse(w, http.StatusNotFound, "not_found", "No update found for resource", nil)
}

// HandleGetInfraUpdatesSummary returns aggregated update statistics per agent.
// GET /api/infra-updates/summary
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
		if err := utils.WriteJSONResponse(w, emptyInfraUpdatesSummaryResponse()); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty summary response")
		}
		return
	}

	updates := h.collectDockerUpdates("")

	// Aggregate by agent
	summaries := make(map[string]infraUpdateAgentSummary)
	for _, update := range updates {
		summary, ok := summaries[update.AgentID]
		if !ok {
			summary = infraUpdateAgentSummary{
				AgentID:   update.AgentID,
				AgentName: update.AgentName,
			}
		}
		summary.TotalCount++
		summary.Containers++
		summaries[update.AgentID] = summary
	}

	response := emptyInfraUpdatesSummaryResponse()
	response.Summaries = summaries
	response.TotalUpdates = len(updates)

	if err := utils.WriteJSONResponse(w, response.normalizeCollections()); err != nil {
		log.Error().Err(err).Msg("Failed to serialize summary response")
	}
}

// HandleTriggerInfraUpdateCheck triggers an update check for a specific resource or agent.
// POST /api/infra-updates/check
//
//	{ "agentId": "xxx" }      Check all on agent
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
		AgentID    string `json:"agentId"`
		ResourceID string `json:"resourceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	// Handle agent-level check
	if req.AgentID != "" {
		commandStatus, err := h.monitor.QueueDockerCheckUpdatesCommand(req.AgentID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "check_updates_failed", err.Error(), nil)
			return
		}

		if err := utils.WriteJSONResponse(w, triggerInfraUpdateCheckResponse{
			Success:   true,
			CommandID: commandStatus.ID,
			AgentID:   req.AgentID,
			Message:   "Update check command queued for agent",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize check response")
		}
		return
	}

	// Handle resource-level check (currently we just check the whole agent)
	if req.ResourceID != "" {
		// Try to find which agent this resource belongs to
		updates := h.collectDockerUpdates("")
		var agentID string
		for _, update := range updates {
			if update.ContainerID == req.ResourceID || ("docker:"+update.AgentID+"/"+update.ContainerID) == req.ResourceID {
				agentID = update.AgentID
				break
			}
		}

		if agentID == "" {
			writeErrorResponse(w, http.StatusNotFound, "not_found", "Resource not found or has no update status", nil)
			return
		}

		commandStatus, err := h.monitor.QueueDockerCheckUpdatesCommand(agentID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "check_updates_failed", err.Error(), nil)
			return
		}

		if err := utils.WriteJSONResponse(w, triggerInfraUpdateCheckResponse{
			Success:   true,
			CommandID: commandStatus.ID,
			AgentID:   agentID,
			Message:   "Update check command queued for agent",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize check response")
		}
		return
	}

	writeErrorResponse(w, http.StatusBadRequest, "missing_params", "Either agentId or resourceId is required", nil)
}

// HandleGetInfraUpdatesForAgent returns all updates for a specific agent.
// GET /api/infra-updates/agent/{agentId}
func (h *UpdateDetectionHandlers) HandleGetInfraUpdatesForAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	if h.monitor == nil {
		response := emptyInfraUpdatesForAgentResponse()
		response.AgentID = agentID
		if err := utils.WriteJSONResponse(w, response.normalizeCollections()); err != nil {
			log.Error().Err(err).Msg("Failed to serialize empty agent updates response")
		}
		return
	}

	updates := h.collectDockerUpdates(agentID)

	response := emptyInfraUpdatesForAgentResponse()
	response.Updates = updates
	response.Total = len(updates)
	response.AgentID = agentID

	if err := utils.WriteJSONResponse(w, response.normalizeCollections()); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent updates response")
	}
}

// collectDockerUpdates gathers update information from Docker agents via ReadState.
func (h *UpdateDetectionHandlers) collectDockerUpdates(agentIDFilter string) []ContainerUpdateInfo {
	if h.readState == nil {
		return nil
	}

	// Build agent ID -> display name map for lookups.
	agentNames := make(map[string]string)
	for _, dh := range h.readState.DockerHosts() {
		agentNames[dh.HostSourceID()] = dh.Name()
	}

	var updates []ContainerUpdateInfo
	for _, c := range h.readState.DockerContainers() {
		agentID := c.HostSourceID()

		// Filter by agent ID if specified.
		if agentIDFilter != "" && agentID != agentIDFilter {
			continue
		}

		us := c.UpdateStatus()
		if us == nil {
			continue
		}

		// Only include containers with updates available or errors.
		if !us.UpdateAvailable && us.Error == "" {
			continue
		}

		update := ContainerUpdateInfo{
			AgentID:         agentID,
			AgentName:       agentNames[agentID],
			ContainerID:     c.ContainerID(),
			ContainerName:   strings.TrimPrefix(c.Name(), "/"),
			Image:           c.Image(),
			UpdateAvailable: us.UpdateAvailable,
			ResourceType:    infraUpdateResourceTypeDocker,
		}

		if us.CurrentDigest != "" {
			update.CurrentDigest = us.CurrentDigest
		}
		if us.LatestDigest != "" {
			update.LatestDigest = us.LatestDigest
		}
		if !us.LastChecked.IsZero() {
			update.LastChecked = us.LastChecked.Unix()
		}
		if us.Error != "" {
			update.Error = us.Error
		}

		updates = append(updates, update)
	}

	return updates
}
