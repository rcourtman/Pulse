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

const (
	configSignatureTTL    = 15 * time.Minute
	agentAgentRoutePrefix = "/api/agents/agent/"
)

var configSigningState struct {
	once sync.Once
	key  ed25519.PrivateKey
	err  error
}

// HostAgentHandlers manages ingest from the host module of pulse-agent.
type HostAgentHandlers struct {
	baseAgentHandlers
}

func trimHostAgentRoutePath(path string) string {
	return strings.TrimPrefix(path, agentAgentRoutePrefix)
}

// NewHostAgentHandlers constructs a new handler set for host agents.
func NewHostAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) *HostAgentHandlers {
	return &HostAgentHandlers{baseAgentHandlers: newBaseAgentHandlers(mtm, m, hub)}
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

	// Support gzip-compressed reports from agents (backward compatible with uncompressed).
	// Cap decompressed size at 1.5MB (6x compressed limit — generous for legitimate payloads).
	body, err := utils.DecompressBodyIfGzipped(r, 1536*1024)
	if err != nil {
		writeErrorResponse(w, http.StatusUnsupportedMediaType, "unsupported_encoding", err.Error(), nil)
		return
	}
	defer body.Close()

	var report agentshost.Report
	if err := json.NewDecoder(body).Decode(&report); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	if report.Timestamp.IsZero() {
		report.Timestamp = time.Now().UTC()
	}

	tokenRecord := getAPITokenRecordFromRequest(r)
	if enforceAgentLimitForHostReport(w, r.Context(), h.getMonitor(r.Context()), report, tokenRecord) {
		return
	}

	host, err := h.getMonitor(r.Context()).ApplyHostReport(report, tokenRecord)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return
	}

	log.Debug().
		Str("agentId", host.ID).
		Str("hostname", host.Hostname).
		Str("platform", host.Platform).
		Msg("Agent report processed")

	h.broadcastState(r.Context())

	// Include any server-side config overrides in the response
	serverConfig := h.getMonitor(r.Context()).GetHostAgentConfig(host.ID)

	resp := map[string]any{
		"success":   true,
		"agentId":   host.ID,
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
		log.Error().Err(err).Msg("Failed to serialize agent response")
	}
}

// HandleLookup returns agent registration details for installer validation.
func (h *HostAgentHandlers) HandleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	lookupID := strings.TrimSpace(r.URL.Query().Get("id"))
	hostname := strings.TrimSpace(r.URL.Query().Get("hostname"))

	if lookupID == "" && hostname == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_lookup_param", "Provide either id or hostname to look up an agent", nil)
		return
	}

	// Use the live state snapshot (not the global mock snapshot) so agent
	// registrations can still be validated while Pulse is in mock/demo mode.
	snap := h.getMonitor(r.Context()).GetLiveStateSnapshot()

	var (
		host  models.Host
		found bool
	)

	if lookupID != "" {
		for _, candidate := range snap.Hosts {
			if candidate.ID == lookupID {
				host = candidate
				found = true
				break
			}
		}
	}

	if !found && hostname != "" {
		// First pass: exact match (case-insensitive)
		for _, candidate := range snap.Hosts {
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
			for _, candidate := range snap.Hosts {
				if strings.EqualFold(getShortName(candidate.Hostname), shortLookup) {
					host = candidate
					found = true
					break
				}
			}
		}
	}

	if !found {
		writeErrorResponse(w, http.StatusNotFound, "agent_not_found", "Agent has not registered with Pulse yet", nil)
		return
	}

	// Ensure the querying token matches the agent (when applicable).
	if record := getAPITokenRecordFromRequest(r); record != nil && host.TokenID != "" && host.TokenID != record.ID {
		writeErrorResponse(w, http.StatusForbidden, "agent_lookup_forbidden", "Agent does not belong to this API token", nil)
		return
	}

	connected := strings.EqualFold(host.Status, "online") ||
		strings.EqualFold(host.Status, "running") ||
		strings.EqualFold(host.Status, "healthy")

	agentInfo := map[string]any{
		"id":           host.ID,
		"hostname":     host.Hostname,
		"displayName":  host.DisplayName,
		"status":       host.Status,
		"connected":    connected,
		"lastSeen":     host.LastSeen,
		"agentVersion": host.AgentVersion,
	}

	resp := map[string]any{
		"success": true,
		"agent":   agentInfo,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent lookup response")
	}
}

// HandleDeleteHost removes an agent from the shared state.
func (h *HostAgentHandlers) HandleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	// Extract agent ID from URL path.
	// Expected format: /api/agents/agent/{agentId}
	trimmedPath := trimHostAgentRoutePath(r.URL.Path)
	agentID := strings.TrimSpace(trimmedPath)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	// Remove the agent from state.
	host, err := h.getMonitor(r.Context()).RemoveHostAgent(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusNotFound, "agent_not_found", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": host.ID,
		"message": "Agent removed",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent removal response")
	}
}

// HandleConfig handles GET (fetch config) and PATCH (update config) for agents.
// GET /api/agents/agent/{agentId}/config - Agent fetches its server-side config.
// PATCH /api/agents/agent/{agentId}/config - UI updates agent config (e.g., commandsEnabled).
func (h *HostAgentHandlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from URL path.
	// Expected format: /api/agents/agent/{agentId}/config
	trimmedPath := trimHostAgentRoutePath(r.URL.Path)
	trimmedPath = strings.TrimSuffix(trimmedPath, "/config")
	agentID := strings.TrimSpace(trimmedPath)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetConfig(w, r, agentID)
	case http.MethodPatch:
		h.handlePatchConfig(w, r, agentID)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET and PATCH are allowed", nil)
	}
}

func (h *HostAgentHandlers) canReadConfig(record *config.APITokenRecord) bool {
	if record == nil {
		return true
	}
	return record.HasScope(config.ScopeAgentConfigRead) ||
		record.HasScope(config.ScopeAgentManage) ||
		record.HasScope(config.ScopeSettingsWrite)
}

func (h *HostAgentHandlers) resolveConfigAgent(ctx context.Context, agentID string, record *config.APITokenRecord) (models.Host, bool) {
	// Use the live state snapshot so agents can still fetch config while
	// Pulse is running in mock/demo mode.
	snap := h.getMonitor(ctx).GetLiveStateSnapshot()

	if record == nil || record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeAgentManage) {
		for _, candidate := range snap.Hosts {
			if candidate.ID == agentID {
				return candidate, true
			}
		}
		return models.Host{}, false
	}

	for _, candidate := range snap.Hosts {
		if candidate.TokenID != "" && candidate.TokenID == record.ID {
			return candidate, true
		}
	}

	return models.Host{}, false
}

func (h *HostAgentHandlers) signAgentConfig(agentID string, cfg monitoring.HostAgentConfig) (monitoring.HostAgentConfig, error) {
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
		AgentID:         agentID,
		IssuedAt:        issuedAt,
		ExpiresAt:       expiresAt,
		CommandsEnabled: cfg.CommandsEnabled,
		Settings:        cfg.Settings,
	}

	signature, err := remoteconfig.SignConfigPayload(payload, key)
	if err != nil {
		if signatureRequired {
			return cfg, fmt.Errorf("failed to sign agent config payload: %w", err)
		}
		log.Warn().Err(err).Msg("Failed to sign agent config payload")
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
func (h *HostAgentHandlers) handleGetConfig(w http.ResponseWriter, r *http.Request, agentID string) {
	record := getAPITokenRecordFromRequest(r)
	if !h.canReadConfig(record) {
		respondMissingScope(w, config.ScopeAgentConfigRead)
		LogAuditEventForTenant(GetOrgID(r.Context()), "agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("agent_id=%s token_id=%s", agentID, tokenID(record)))
		return
	}

	host, ok := h.resolveConfigAgent(r.Context(), agentID, record)
	if !ok {
		writeErrorResponse(w, http.StatusNotFound, "agent_not_found", "Agent has not registered with Pulse yet", nil)
		LogAuditEventForTenant(GetOrgID(r.Context()), "agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("agent_id=%s token_id=%s", agentID, tokenID(record)))
		return
	}

	agentID = host.ID

	config := h.getMonitor(r.Context()).GetHostAgentConfig(agentID)
	signedConfig, err := h.signAgentConfig(agentID, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to sign agent config payload")
		writeErrorResponse(w, http.StatusInternalServerError, "config_signing_failed", "Failed to sign agent config", nil)
		LogAuditEventForTenant(GetOrgID(r.Context()), "agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("agent_id=%s token_id=%s", agentID, tokenID(record)))
		return
	}

	resp := map[string]any{
		"success": true,
		"agentId": agentID,
		"config":  signedConfig,
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent config response")
		LogAuditEventForTenant(GetOrgID(r.Context()), "agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, false,
			fmt.Sprintf("agent_id=%s token_id=%s", agentID, tokenID(record)))
		return
	}

	LogAuditEventForTenant(GetOrgID(r.Context()), "agent_config_fetch", auth.GetUser(r.Context()), GetClientIP(r), r.URL.Path, true,
		fmt.Sprintf("agent_id=%s token_id=%s", agentID, tokenID(record)))
}

func tokenID(record *config.APITokenRecord) string {
	if record == nil {
		return ""
	}
	return record.ID
}

func (h *HostAgentHandlers) ensureAgentTokenMatch(w http.ResponseWriter, r *http.Request, agentID string) bool {
	record := getAPITokenRecordFromRequest(r)
	if record == nil {
		return true
	}

	if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeWildcard) {
		return true
	}

	// Use the live state snapshot so mock/demo mode doesn't block agent auth checks.
	snap := h.getMonitor(r.Context()).GetLiveStateSnapshot()
	for _, host := range snap.Hosts {
		if host.ID != agentID {
			continue
		}
		if host.TokenID == record.ID {
			return true
		}
		writeErrorResponse(w, http.StatusForbidden, "agent_lookup_forbidden", "Agent does not belong to this API token", nil)
		return false
	}

	writeErrorResponse(w, http.StatusNotFound, "agent_not_found", "Agent has not registered with Pulse yet", nil)
	return false
}

// handlePatchConfig updates the server-side config for an agent.
func (h *HostAgentHandlers) handlePatchConfig(w http.ResponseWriter, r *http.Request, agentID string) {
	if !h.ensureAgentTokenMatch(w, r, agentID) {
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

	if err := h.getMonitor(r.Context()).UpdateHostAgentConfig(agentID, req.CommandsEnabled); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "update_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	log.Info().
		Str("agentId", agentID).
		Interface("commandsEnabled", req.CommandsEnabled).
		Msg("Agent config updated")

	resp := map[string]any{
		"success": true,
		"agentId": agentID,
		"config": map[string]any{
			"commandsEnabled": req.CommandsEnabled,
		},
	}

	if err := utils.WriteJSONResponse(w, resp); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent config update response")
	}
}

// HandleUninstall allows an agent to unregister itself during uninstallation.
// Requires ScopeAgentReport and a valid agentId in the request body.
func (h *HostAgentHandlers) HandleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	log.Info().Str("agentId", agentID).Msg("Received unregistration request from agent uninstaller")

	// Ensure the token can manage this specific agent.
	if !h.ensureAgentTokenMatch(w, r, agentID) {
		return
	}

	// Remove the agent from state.
	_, err := h.getMonitor(r.Context()).RemoveHostAgent(agentID)
	if err != nil {
		// If the agent is not found, we still return success because the goal is reached.
		log.Warn().Err(err).Str("agentId", agentID).Msg("Agent not found during unregistration request")
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
		"message": "Agent unregistered successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent unregistration response")
	}
}

// HandleLink manually links an agent to a specific PVE node.
// This is used when auto-linking can't disambiguate (e.g., multiple nodes with hostname "pve").
func (h *HostAgentHandlers) HandleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		AgentID string `json:"agentId"`
		NodeID  string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_node_id", "Node ID is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).LinkHostAgent(agentID, nodeID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "link_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
		"nodeId":  nodeID,
		"message": "Agent linked to PVE node",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent link response")
	}
}

// HandleUnlink removes the link between an agent and its PVE node.
// The agent continues to report but appears in the Managed Agents table.
func (h *HostAgentHandlers) HandleUnlink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()

	var req struct {
		AgentID string `json:"agentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
		return
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).UnlinkHostAgent(agentID); err != nil {
		writeErrorResponse(w, http.StatusNotFound, "unlink_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
		"message": "Agent unlinked from PVE node",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent unlink response")
	}
}
