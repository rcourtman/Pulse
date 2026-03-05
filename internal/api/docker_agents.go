package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog/log"
)

// DockerAgentHandlers manages ingest from the external Docker agent.
type DockerAgentHandlers struct {
	baseAgentHandlers
	config *config.Config
}

type dockerCommandAckRequest struct {
	AgentID string         `json:"agentId"`
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// errInvalidCommandStatus is returned when an unrecognized command status is provided.
var errInvalidCommandStatus = errors.New("invalid command status")

// normalizeCommandStatus converts a client-provided status string into a canonical
// internal status constant. It accepts multiple aliases for each status:
//   - acknowledged: "", "ack", "acknowledged"
//   - in_progress: "in_progress", "progress"
//   - completed: "success", "completed", "complete"
//   - failed: "fail", "failed", "error"
//
// Returns errInvalidCommandStatus for unrecognized values.
func normalizeCommandStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "", "ack", "acknowledged":
		return monitoring.DockerCommandStatusAcknowledged, nil
	case "in_progress", "progress":
		return monitoring.DockerCommandStatusInProgress, nil
	case "success", "completed", "complete":
		return monitoring.DockerCommandStatusCompleted, nil
	case "fail", "failed", "error":
		return monitoring.DockerCommandStatusFailed, nil
	default:
		return "", errInvalidCommandStatus
	}
}

func trimDockerRuntimePathPrefix(path string) string {
	if !strings.HasPrefix(path, "/api/agents/docker/runtimes/") {
		return ""
	}
	return strings.TrimPrefix(path, "/api/agents/docker/runtimes/")
}

func dockerRuntimeAgentIDFromPath(path string, suffix string) string {
	trimmed := trimDockerRuntimePathPrefix(path)
	if suffix != "" {
		trimmed = strings.TrimSuffix(trimmed, suffix)
	}
	return strings.TrimSpace(trimmed)
}

// NewDockerAgentHandlers constructs a new Docker agent handler group.
func NewDockerAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub, cfg *config.Config) *DockerAgentHandlers {
	return &DockerAgentHandlers{baseAgentHandlers: newBaseAgentHandlers(mtm, m, hub), config: cfg}
}

// HandleReport accepts heartbeat payloads from the Docker agent.
func (h *DockerAgentHandlers) HandleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	// Limit request body to 2MB to prevent memory exhaustion
	// (512KB was too small for users with 100+ containers)
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
	defer r.Body.Close()

	// Support gzip-compressed reports from agents (backward compatible with uncompressed)
	body, err := utils.DecompressBodyIfGzipped(r, 10*1024*1024)
	if err != nil {
		writeErrorResponse(w, http.StatusUnsupportedMediaType, "unsupported_encoding", err.Error(), nil)
		return
	}
	defer body.Close()

	var report agentsdocker.Report
	if err := json.NewDecoder(body).Decode(&report); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	tokenRecord := getAPITokenRecordFromRequest(r)
	if enforceAgentLimitForDockerReport(w, r.Context(), h.getMonitor(r.Context()), report, tokenRecord) {
		return
	}

	host, err := h.getMonitor(r.Context()).ApplyDockerReport(report, tokenRecord)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("dockerHost", host.Hostname).
		Int("containers", len(host.Containers)).
		Msg("Docker agent report processed")

	// Broadcast the updated state for near-real-time UI updates
	h.broadcastState(r.Context())

	response := map[string]any{
		"success":    true,
		"agentId":    host.ID,
		"containers": len(host.Containers),
		"lastSeen":   host.LastSeen,
	}

	if payload, cmd := h.getMonitor(r.Context()).FetchDockerCommandForHost(host.ID); cmd != nil {
		commandResponse := map[string]any{
			"id":   cmd.ID,
			"type": cmd.Type,
		}
		if len(payload) > 0 {
			commandResponse["payload"] = payload
		}
		response["commands"] = []map[string]any{commandResponse}
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker agent response")
	}
}

// HandleDockerHostActions routes docker host management actions based on path and method.
func (h *DockerAgentHandlers) HandleDockerHostActions(w http.ResponseWriter, r *http.Request) {
	// Check if this is an allow reenroll request
	if strings.HasSuffix(r.URL.Path, "/allow-reenroll") && r.Method == http.MethodPost {
		h.HandleAllowReenroll(w, r)
		return
	}

	// Check if this is an unhide request
	if strings.HasSuffix(r.URL.Path, "/unhide") && r.Method == http.MethodPut {
		h.HandleUnhideHost(w, r)
		return
	}

	// Check if this is a pending uninstall request
	if strings.HasSuffix(r.URL.Path, "/pending-uninstall") && r.Method == http.MethodPut {
		h.HandleMarkPendingUninstall(w, r)
		return
	}

	// Check if this is a custom display name update request
	if strings.HasSuffix(r.URL.Path, "/display-name") && r.Method == http.MethodPut {
		h.HandleSetCustomDisplayName(w, r)
		return
	}

	// Check if this is a check updates request
	if strings.HasSuffix(r.URL.Path, "/check-updates") && r.Method == http.MethodPost {
		h.HandleCheckUpdates(w, r)
		return
	}

	// Check if this is an update-all request
	if strings.HasSuffix(r.URL.Path, "/update-all") && r.Method == http.MethodPost {
		h.HandleUpdateAll(w, r)
		return
	}

	// Otherwise, handle as delete/hide request
	if r.Method == http.MethodDelete {
		h.HandleDeleteHost(w, r)
		return
	}

	writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
}

// HandleCommandAck processes acknowledgements from docker agents for issued commands.
func (h *DockerAgentHandlers) HandleCommandAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/agents/docker/commands/")
	if !strings.HasSuffix(trimmed, "/ack") {
		writeErrorResponse(w, http.StatusNotFound, "not_found", "Endpoint not found", nil)
		return
	}
	commandID := strings.TrimSuffix(trimmed, "/ack")
	commandID = strings.TrimSuffix(commandID, "/")
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_command_id", "Command ID is required", nil)
		return
	}

	var req dockerCommandAckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	status, err := normalizeCommandStatus(req.Status)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_status", "Invalid command status", nil)
		return
	}

	mon := h.getMonitor(r.Context())
	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	commandStatus, agentID, shouldRemove, err := mon.AcknowledgeDockerHostCommand(commandID, agentID, status, req.Message)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "docker_command_ack_failed", err.Error(), nil)
		return
	}

	// If a container update succeeded, migrate persisted container metadata (custom URLs, notes, tags)
	// so the UI doesn't "lose" it when the container runtime ID changes.
	if status == monitoring.DockerCommandStatusCompleted && commandStatus.Type == monitoring.DockerCommandTypeUpdateContainer && len(req.Payload) > 0 {
		oldContainerID, _ := req.Payload["oldContainerId"].(string)
		newContainerID, _ := req.Payload["newContainerId"].(string)
		oldContainerID = strings.TrimSpace(oldContainerID)
		newContainerID = strings.TrimSpace(newContainerID)
		if oldContainerID != "" && newContainerID != "" && oldContainerID != newContainerID {
			if err := mon.CopyDockerContainerMetadata(agentID, oldContainerID, newContainerID); err != nil {
				log.Warn().
					Err(err).
					Str("dockerAgentID", agentID).
					Str("oldContainerID", oldContainerID).
					Str("newContainerID", newContainerID).
					Msg("Failed to migrate docker container metadata after update")
			}
		}
	}

	if shouldRemove {
		if _, removeErr := mon.RemoveDockerHost(agentID); removeErr != nil {
			log.Error().Err(removeErr).Str("dockerAgentID", agentID).Str("commandID", commandID).Msg("Failed to remove docker host after command completion")
		} else {
			// Clear the removal block since the agent has confirmed it stopped successfully.
			// This allows immediate re-enrollment without waiting for the 24-hour TTL.
			if reenrollErr := mon.AllowDockerHostReenroll(agentID); reenrollErr != nil {
				log.Warn().Err(reenrollErr).Str("dockerAgentID", agentID).Msg("Failed to clear removal block after successful stop")
			}
		}
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
		"command": commandStatus,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker command acknowledgement response")
	}
}

// HandleDeleteHost removes or hides a container runtime from the shared state.
// If query parameter ?hide=true is provided, the host is marked as hidden instead of deleted.
func (h *DockerAgentHandlers) HandleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	// Check if we should hide instead of delete
	hideParam := r.URL.Query().Get("hide")
	shouldHide := strings.ToLower(hideParam) == "true"
	forceParam := strings.ToLower(r.URL.Query().Get("force"))
	force := forceParam == "true" || strings.ToLower(r.URL.Query().Get("mode")) == "force"

	priorHost, hostExists := h.getMonitor(r.Context()).GetDockerHost(agentID)

	if shouldHide {
		if !hostExists {
			writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", "Container runtime not found", nil)
			return
		}
		host, err := h.getMonitor(r.Context()).HideDockerHost(agentID)
		if err != nil {
			writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", err.Error(), nil)
			return
		}

		h.broadcastState(r.Context())

		if err := utils.WriteJSONResponse(w, map[string]any{
			"success": true,
			"agentId": host.ID,
			"message": "Container runtime hidden",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize docker host operation response")
		}
		return
	}

	if !hostExists {
		if force {
			if err := utils.WriteJSONResponse(w, map[string]any{
				"success": true,
				"agentId": agentID,
				"message": "Container runtime already removed",
			}); err != nil {
				log.Error().Err(err).Msg("Failed to serialize docker host operation response")
			}
			return
		}

		writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", "Container runtime not found", nil)
		return
	}

	if !force && strings.EqualFold(priorHost.Status, "online") {
		command, err := h.getMonitor(r.Context()).QueueDockerHostStop(agentID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "docker_command_failed", err.Error(), nil)
			return
		}

		h.broadcastState(r.Context())

		if err := utils.WriteJSONResponse(w, map[string]any{
			"success": true,
			"agentId": agentID,
			"command": command,
			"message": "Stop command queued",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to serialize docker host stop command response")
		}
		return
	}

	host, err := h.getMonitor(r.Context()).RemoveDockerHost(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": host.ID,
		"message": "Container runtime removed",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker host operation response")
	}
}

// HandleAllowReenroll clears the removal block for a container runtime to permit future reports.
func (h *DockerAgentHandlers) HandleAllowReenroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/allow-reenroll")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).AllowDockerHostReenroll(agentID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "docker_agent_reenroll_failed", err.Error(), nil)
		return
	}

	// Broadcast updated state to ensure the frontend reflects the change
	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker host allow reenroll response")
	}
}

// HandleUnhideHost unhides a previously hidden container runtime.
func (h *DockerAgentHandlers) HandleUnhideHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/unhide")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	host, err := h.getMonitor(r.Context()).UnhideDockerHost(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": host.ID,
		"message": "Container runtime unhidden",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker host unhide response")
	}
}

// HandleMarkPendingUninstall marks a container runtime as pending uninstall.
func (h *DockerAgentHandlers) HandleMarkPendingUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/pending-uninstall")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	host, err := h.getMonitor(r.Context()).MarkDockerHostPendingUninstall(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": host.ID,
		"message": "Container runtime marked as pending uninstall",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker host pending uninstall response")
	}
}

// HandleSetCustomDisplayName updates the custom display name for a container runtime.
func (h *DockerAgentHandlers) HandleSetCustomDisplayName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PUT is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/display-name")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	customName := strings.TrimSpace(req.DisplayName)

	host, err := h.getMonitor(r.Context()).SetDockerHostCustomDisplayName(agentID, customName)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "docker_agent_not_found", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": host.ID,
		"message": "Container runtime custom display name updated",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker host custom display name response")
	}
}

// HandleContainerUpdate triggers a container update on a Docker agent.
// POST /api/agents/docker/containers/{containerId}/update
func (h *DockerAgentHandlers) HandleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	defer r.Body.Close()

	var req struct {
		AgentID       string `json:"agentId"`
		ContainerID   string `json:"containerId"`
		ContainerName string `json:"containerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}
	if req.ContainerID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_container_id", "Container ID is required", nil)
		return
	}

	// Check if Docker update actions are disabled server-wide
	if h.config != nil && h.config.DisableDockerUpdateActions {
		writeErrorResponse(w, http.StatusForbidden, "docker_updates_disabled",
			"Docker container updates are disabled by server configuration. Set PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=false or disable in Settings to enable.", nil)
		return
	}

	// Queue the update command
	commandStatus, err := h.getMonitor(r.Context()).QueueDockerContainerUpdateCommand(agentID, req.ContainerID, req.ContainerName)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "update_command_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"commandId": commandStatus.ID,
		"agentId":   agentID,
		"container": map[string]string{
			"id":   req.ContainerID,
			"name": req.ContainerName,
		},
		"message": "Container update command queued",
		"note":    "The update will be executed on the next agent report cycle",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize container update response")
	}
}

// HandleUpdateAll triggers an update for all containers with updates available on a container runtime.
// POST /api/agents/docker/runtimes/{agentId}/update-all
func (h *DockerAgentHandlers) HandleUpdateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/update-all")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	// Check if Docker update actions are disabled server-wide
	if h.config != nil && h.config.DisableDockerUpdateActions {
		writeErrorResponse(w, http.StatusForbidden, "docker_updates_disabled",
			"Docker container updates are disabled by server configuration. Set PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=false or disable in Settings to enable.", nil)
		return
	}

	commandStatus, err := h.getMonitor(r.Context()).QueueDockerUpdateAllCommand(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "update_all_command_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"commandId": commandStatus.ID,
		"agentId":   agentID,
		"message":   "Update all containers command queued",
		"note":      "The update will be executed on the next agent report cycle",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize update all response")
	}
}

// HandleCheckUpdates triggers an immediate update check for all containers on a container runtime.
// POST /api/agents/docker/runtimes/{agentId}/check-updates
func (h *DockerAgentHandlers) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	agentID := dockerRuntimeAgentIDFromPath(r.URL.Path, "/check-updates")
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "Agent ID is required", nil)
		return
	}

	// Queue the check updates command
	commandStatus, err := h.getMonitor(r.Context()).QueueDockerCheckUpdatesCommand(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "check_updates_command_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success":   true,
		"commandId": commandStatus.ID,
		"agentId":   agentID,
		"message":   "Check for updates command queued",
		"note":      "The agent will clear its registry cache and check for updates on the next report cycle",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize check updates response")
	}
}
