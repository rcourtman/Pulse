package api

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

const configSignatureTTL = 15 * time.Minute

var configSigningState struct {
	once sync.Once
	key  ed25519.PrivateKey
	err  error
}

// HostAgentHandlers manages ingest from the pulse-host-agent.
type HostAgentHandlers struct {
	mtMonitor     *monitoring.MultiTenantMonitor
	legacyMonitor *monitoring.Monitor
	wsHub         *websocket.Hub
}

// NewHostAgentHandlers constructs a new handler set for host agents.
func NewHostAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) *HostAgentHandlers {
	// If mtm is provided, try to populate legacyMonitor from "default" org if not provided
	if m == nil && mtm != nil {
		if mon, err := mtm.GetMonitor("default"); err == nil {
			m = mon
		}
	}
	return &HostAgentHandlers{mtMonitor: mtm, legacyMonitor: m, wsHub: hub}
}

// SetMonitor updates the monitor reference for host agent handlers.
func (h *HostAgentHandlers) SetMonitor(m *monitoring.Monitor) {
	h.legacyMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor reference
func (h *HostAgentHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	h.mtMonitor = mtm
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			h.legacyMonitor = m
		}
	}
}

// getMonitor helper
func (h *HostAgentHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	orgID := GetOrgID(ctx)
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m
		}
	}
	return h.legacyMonitor
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

	host, err := h.getMonitor(r.Context()).ApplyHostReport(report, tokenRecord)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("hostId", host.ID).
		Str("hostname", host.Hostname).
		Str("platform", host.Platform).
		Msg("Host agent report processed")

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	// Include any server-side config overrides in the response
	serverConfig := h.getMonitor(r.Context()).GetHostAgentConfig(host.ID)

	resp := map[string]any{
		"success":   true,
		"hostId":    host.ID,
		"lastSeen":  host.LastSeen,
		"platform":  host.Platform,
		"osName":    host.OSName,
		"osVersion": host.OSVersion,
	}

	// Only include config if there are actual overrides
	if serverConfig.CommandsEnabled != nil {
		resp["config"] = map[string]any{
			"commandsEnabled": serverConfig.CommandsEnabled,
		}
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

	state := h.getMonitor(r.Context()).GetState()

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
	host, err := h.getMonitor(r.Context()).RemoveHostAgent(hostID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "host_not_found", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"hostId":  host.ID,
		"message": "Host removed",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host removal response")
	}
}

// HandleConfig handles GET (fetch config) and PATCH (update config) for host agents.
// GET /api/agents/host/{hostId}/config - Agent fetches its server-side config
// PATCH /api/agents/host/{hostId}/config - UI updates host config (e.g., commandsEnabled)
func (h *HostAgentHandlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Extract host ID from URL path
	// Expected format: /api/agents/host/{hostId}/config
	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/agents/host/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/config")
	hostID := strings.TrimSpace(trimmedPath)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetConfig(w, r, hostID)
	case http.MethodPatch:
		h.handlePatchConfig(w, r, hostID)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET and PATCH are allowed", nil)
	}
}

func (h *HostAgentHandlers) canReadConfig(record *config.APITokenRecord) bool {
	if record == nil {
		return true
	}
	return record.HasScope(config.ScopeHostConfigRead) ||
		record.HasScope(config.ScopeHostManage) ||
		record.HasScope(config.ScopeSettingsWrite)
}

func (h *HostAgentHandlers) resolveConfigHost(ctx context.Context, hostID string, record *config.APITokenRecord) (models.Host, bool) {
	state := h.getMonitor(ctx).GetState()

	if record == nil || record.HasScope(config.ScopeSettingsWrite) {
		for _, candidate := range state.Hosts {
			if candidate.ID == hostID {
				return candidate, true
			}
		}
		return models.Host{}, false
	}

	for _, candidate := range state.Hosts {
		if candidate.TokenID != "" && candidate.TokenID == record.ID {
			return candidate, true
		}
	}

	return models.Host{}, false
}

func (h *HostAgentHandlers) signHostConfig(hostID string, cfg monitoring.HostAgentConfig) (monitoring.HostAgentConfig, error) {
	signatureRequired := isConfigSignatureRequired()
	key, err := getConfigSigningKey()
	if err != nil {
		if signatureRequired {
			return cfg, fmt.Errorf("failed to load config signing key: %w", err)
		}
		log.Warn().Err(err).Msg("Failed to load config signing key")
		return cfg, nil
	}
	if len(key) == 0 {
		if signatureRequired {
			return cfg, fmt.Errorf("config signing required but PULSE_AGENT_CONFIG_SIGNING_KEY is not set")
		}
		return cfg, nil
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(configSignatureTTL)

	payload := remoteconfig.SignedConfigPayload{
		HostID:          hostID,
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
		CommandsEnabled: cfg.CommandsEnabled,
		Settings:        cfg.Settings,
	}

	signature, err := remoteconfig.SignConfigPayload(payload, key)
	if err != nil {
		if signatureRequired {
			return cfg, fmt.Errorf("failed to sign host config payload: %w", err)
		}
		log.Warn().Err(err).Msg("Failed to sign host config payload")
		return cfg, nil
	}

	cfg.IssuedAt = &issuedAt
	cfg.ExpiresAt = &expiresAt
	cfg.Signature = signature
	return cfg, nil
}

func getConfigSigningKey() (ed25519.PrivateKey, error) {
	configSigningState.once.Do(func() {
		raw := utils.GetenvTrim("PULSE_AGENT_CONFIG_SIGNING_KEY")
		if raw == "" {
			return
		}
		key, err := remoteconfig.DecodeEd25519PrivateKey(raw)
		if err != nil {
			configSigningState.err = err
			return
		}
		configSigningState.key = key
	})

	return configSigningState.key, configSigningState.err
}

func isConfigSignatureRequired() bool {
	return utils.ParseBool(utils.GetenvTrim("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED"))
}

// handleGetConfig returns the server-side config for an agent to apply.
func (h *HostAgentHandlers) handleGetConfig(w http.ResponseWriter, r *http.Request, hostID string) {
	record := getAPITokenRecordFromRequest(r)
	if !h.canReadConfig(record) {
		respondMissingScope(w, config.ScopeHostConfigRead)
		LogAuditEventForTenant(GetOrgID(r.Context()), "host_agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("host_id=%s token_id=%s", hostID, tokenID(record)))
		return
	}

	host, ok := h.resolveConfigHost(r.Context(), hostID, record)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "host_not_found", "Host has not registered with Pulse yet", nil)
		LogAuditEventForTenant(GetOrgID(r.Context()), "host_agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("host_id=%s token_id=%s", hostID, tokenID(record)))
		return
	}

	hostID = host.ID

	config := h.getMonitor(r.Context()).GetHostAgentConfig(hostID)
	signedConfig, err := h.signHostConfig(hostID, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to sign host config payload")
		writeErrorResponse(w, http.StatusInternalServerError, "config_signing_failed", "Failed to sign host config", nil)
		LogAuditEventForTenant(GetOrgID(r.Context()), "host_agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("host_id=%s token_id=%s", hostID, tokenID(record)))
		return
	}

	resp := map[string]any{
		"success": true,
		"hostId":  hostID,
		"config":  signedConfig,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host config response")
		LogAuditEventForTenant(GetOrgID(r.Context()), "host_agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("host_id=%s token_id=%s", hostID, tokenID(record)))
		return
	}

	LogAuditEventForTenant(GetOrgID(r.Context()), "host_agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("host_id=%s token_id=%s", hostID, tokenID(record)))
}

func tokenID(record *config.APITokenRecord) string {
	if record == nil {
		return ""
	}
	return record.ID
}

func (h *HostAgentHandlers) ensureHostTokenMatch(w http.ResponseWriter, r *http.Request, hostID string) bool {
	record := getAPITokenRecordFromRequest(r)
	if record == nil {
		return true
	}

	if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeWildcard) {
		return true
	}

	state := h.getMonitor(r.Context()).GetState()
	for _, host := range state.Hosts {
		if host.ID != hostID {
			continue
		}
		if host.TokenID == record.ID {
			return true
		}
		writeErrorResponse(w, http.StatusForbidden, "host_lookup_forbidden", "Host does not belong to this API token", nil)
		return false
	}

	writeErrorResponse(w, http.StatusNotFound, "host_not_found", "Host has not registered with Pulse yet", nil)
	return false
}

// handlePatchConfig updates the server-side config for a host agent.
func (h *HostAgentHandlers) handlePatchConfig(w http.ResponseWriter, r *http.Request, hostID string) {
	if !h.ensureHostTokenMatch(w, r, hostID) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		CommandsEnabled *bool `json:"commandsEnabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if err := h.getMonitor(r.Context()).UpdateHostAgentConfig(hostID, req.CommandsEnabled); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	log.Info().
		Str("hostId", hostID).
		Interface("commandsEnabled", req.CommandsEnabled).
		Msg("Host agent config updated")

	resp := map[string]any{
		"success": true,
		"hostId":  hostID,
		"config": map[string]any{
			"commandsEnabled": req.CommandsEnabled,
		},
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host config update response")
	}
}

// HandleUninstall allows an agent to unregister itself during uninstallation.
// Requires ScopeHostReport and a valid hostId in the request body.
func (h *HostAgentHandlers) HandleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		HostID string `json:"hostId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	hostID := strings.TrimSpace(req.HostID)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
		return
	}

	log.Info().Str("hostId", hostID).Msg("Received unregistration request from agent uninstaller")

	// Ensure the token can manage this specific host
	if !h.ensureHostTokenMatch(w, r, hostID) {
		return
	}

	// Remove the host from state
	_, err := h.getMonitor(r.Context()).RemoveHostAgent(hostID)
	if err != nil {
		// If host not found, we still return success because the goal is reached
		log.Warn().Err(err).Str("hostId", hostID).Msg("Host not found during unregistration request")
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"hostId":  hostID,
		"message": "Host unregistered successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host unregistration response")
	}
}

// HandleLink manually links a host agent to a specific PVE node.
// This is used when auto-linking can't disambiguate (e.g., multiple nodes with hostname "pve").
func (h *HostAgentHandlers) HandleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		HostID string `json:"hostId"`
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	hostID := strings.TrimSpace(req.HostID)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
		return
	}

	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_node_id", "Node ID is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).LinkHostAgent(hostID, nodeID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "link_failed", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"hostId":  hostID,
		"nodeId":  nodeID,
		"message": "Host agent linked to PVE node",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host link response")
	}
}

// HandleUnlink removes the link between a host agent and its PVE node.
// The agent continues to report but appears in the Managed Agents table.
func (h *HostAgentHandlers) HandleUnlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		HostID string `json:"hostId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	hostID := strings.TrimSpace(req.HostID)
	if hostID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_host_id", "Host ID is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).UnlinkHostAgent(hostID); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "unlink_failed", err.Error(), nil)
		return
	}

	go h.wsHub.BroadcastState(h.getMonitor(r.Context()).GetState().ToFrontend())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"hostId":  hostID,
		"message": "Host agent unlinked from PVE node",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host unlink response")
	}
}
