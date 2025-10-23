package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
)

// HostAgentHandlers manages ingest from the pulse-host-agent.
type HostAgentHandlers struct {
	monitor *monitoring.Monitor
	wsHub   *websocket.Hub
}

// NewHostAgentHandlers constructs a new handler set for host agents.
func NewHostAgentHandlers(m *monitoring.Monitor, hub *websocket.Hub) *HostAgentHandlers {
	return &HostAgentHandlers{monitor: m, wsHub: hub}
}

// SetMonitor updates the monitor reference for host agent handlers.
func (h *HostAgentHandlers) SetMonitor(m *monitoring.Monitor) {
	h.monitor = m
}

// HandleReport ingests host agent reports.
func (h *HostAgentHandlers) HandleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	defer r.Body.Close()

	var report agentshost.Report
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now().UTC()
	}

	tokenRecord := getAPITokenRecordFromRequest(r)

	host, err := h.monitor.ApplyHostReport(report, tokenRecord)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("hostId", host.ID).
		Str("hostname", host.Hostname).
		Str("platform", host.Platform).
		Msg("Host agent report processed")

	go h.wsHub.BroadcastState(h.monitor.GetState().ToFrontend())

	resp := map[string]any{
		"success":   true,
		"hostId":    host.ID,
		"lastSeen":  host.LastSeen,
		"platform":  host.Platform,
		"osName":    host.OSName,
		"osVersion": host.OSVersion,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host agent response")
	}
}
