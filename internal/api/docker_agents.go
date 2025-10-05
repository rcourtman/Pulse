package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rs/zerolog/log"
)

// DockerAgentHandlers manages ingest from the external Docker agent.
type DockerAgentHandlers struct {
	monitor *monitoring.Monitor
	wsHub   *websocket.Hub
}

// NewDockerAgentHandlers constructs a new Docker agent handler group.
func NewDockerAgentHandlers(m *monitoring.Monitor, hub *websocket.Hub) *DockerAgentHandlers {
	return &DockerAgentHandlers{monitor: m, wsHub: hub}
}

// HandleReport accepts heartbeat payloads from the Docker agent.
func (h *DockerAgentHandlers) HandleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	defer r.Body.Close()

	var report agentsdocker.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now()
	}

	host, err := h.monitor.ApplyDockerReport(report)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("dockerHost", host.Hostname).
		Int("containers", len(host.Containers)).
		Msg("Docker agent report processed")

	// Broadcast the updated state for near-real-time UI updates
	go h.wsHub.BroadcastState(h.monitor.GetState().ToFrontend())

	response := map[string]any{
		"success":    true,
		"hostId":     host.ID,
		"containers": len(host.Containers),
		"lastSeen":   host.LastSeen,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to serialize docker agent response")
	}
}
