package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

	// Limit request body to 256KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
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

// HandleLookup returns host registration details for installer validation.
func (h *HostAgentHandlers) HandleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	id := strings.TrimSpace(r.URL.Query().Get("id"))
	hostname := strings.TrimSpace(r.URL.Query().Get("hostname"))

	if id == "" && hostname == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_lookup_param", "Provide either id or hostname to look up a host", nil)
		return
	}

	state := h.monitor.GetState()

	var (
		host  models.Host
		found bool
	)

	if id != "" {
		for _, candidate := range state.Hosts {
			if candidate.ID == id {
				host = candidate
				found = true
				break
			}
		}
	}

	if !found && hostname != "" {
		// First pass: exact match (case-insensitive)
		for _, candidate := range state.Hosts {
			if strings.EqualFold(candidate.Hostname, hostname) || strings.EqualFold(candidate.DisplayName, hostname) {
				host = candidate
				found = true
				break
			}
		}

		// Second pass: short hostname match (if exact match failed)
		if !found {
			// Helper to get short hostname (before first dot)
			getShortName := func(h string) string {
				if idx := strings.Index(h, "."); idx != -1 {
					return h[:idx]
				}
				return h
			}

			shortLookup := getShortName(hostname)
			for _, candidate := range state.Hosts {
				if strings.EqualFold(getShortName(candidate.Hostname), shortLookup) {
					host = candidate
					found = true
					break
				}
			}
		}
	}

	if !found {
		writeErrorResponse(w, http.StatusNotFound, "host_not_found", "Host has not registered with Pulse yet", nil)
		return
	}

	// Ensure the querying token matches the host (when applicable).
	if record := getAPITokenRecordFromRequest(r); record != nil && host.TokenID != "" && host.TokenID != record.ID {
		writeErrorResponse(w, http.StatusForbidden, "host_lookup_forbidden", "Host does not belong to this API token", nil)
		return
	}

	connected := strings.EqualFold(host.Status, "online") ||
		strings.EqualFold(host.Status, "running") ||
		strings.EqualFold(host.Status, "healthy")

	resp := map[string]any{
		"success": true,
		"host": map[string]any{
			"id":           host.ID,
			"hostname":     host.Hostname,
			"displayName":  host.DisplayName,
			"status":       host.Status,
			"connected":    connected,
			"lastSeen":     host.LastSeen,
			"agentVersion": host.AgentVersion,
		},
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host lookup response")
	}
}

// HandleDeleteHost removes a host from the shared state.
func (h *HostAgentHandlers) HandleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	// Extract host ID from URL path
	// Expected format: /api/agents/host/{hostId}
	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/host/")
	hostID := strings.TrimSpace(trimmedPath)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
		return
	}

	// Remove the host from state
	host, err := h.monitor.RemoveHostAgent(hostID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "host_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.monitor.GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"hostId":  host.ID,
		"message": "Host removed",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host removal response")
	}
}
