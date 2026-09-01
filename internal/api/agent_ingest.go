package api

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
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
	agentRoutePrefix      = "/api/agents/agent/"
	legacyHostRoutePrefix = "/api/agents/host/"
)

var configSigningState struct {
	once sync.Once
	key  ed25519.PrivateKey
	err  error
}

// UnifiedAgentHandlers manages ingest from the runtime-side Unified Agent module of pulse-agent.
type UnifiedAgentHandlers struct {
	baseAgentHandlers

	// serverVersion is echoed on report acknowledgements so agents can spot a
	// server upgrade on their next report instead of their next hourly update
	// check. Empty (the default) omits it from acks.
	serverVersion string
}

// SetServerVersion supplies the running server version to include on report
// acknowledgements. Call once at wiring time; an empty version keeps acks
// version-free.
func (h *UnifiedAgentHandlers) SetServerVersion(version string) {
	h.serverVersion = strings.TrimSpace(version)
}

func trimUnifiedAgentRoutePath(path string) string {
	for _, prefix := range []string{agentRoutePrefix, legacyHostRoutePrefix} {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	return path
}

// NewUnifiedAgentHandlers constructs a new handler set for Pulse Unified Agent ingest.
func NewUnifiedAgentHandlers(mtm *monitoring.MultiTenantMonitor, m *monitoring.Monitor, hub *websocket.Hub) *UnifiedAgentHandlers {
	return &UnifiedAgentHandlers{baseAgentHandlers: newBaseAgentHandlers(mtm, m, hub)}
}

// HandleReport ingests Pulse Unified Agent reports.
func (h *UnifiedAgentHandlers) HandleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	var report agentshost.Report
	// Support gzip-compressed reports from agents (backward compatible with
	// uncompressed), with independent encoded and decoded size limits.
	if !decodeCompressedAgentReport(w, r, 256*1024, 1536*1024, &report) {
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

	// A freshly generated install token is the server's durable record of the
	// operator's command-policy choice. Project that choice onto the stable host
	// identity before returning remote config, otherwise an old false override
	// survives remove/reinstall and immediately shuts down an agent launched
	// with --enable-commands (#1728).
	if err := reconcileInstallTokenCommandPolicy(h.getMonitor(r.Context()), tokenRecord, host); err != nil {
		log.Error().Err(err).
			Str("agent_id", host.ID).
			Str("token_id", tokenID(tokenRecord)).
			Msg("Failed to reconcile install-token command policy")
	}

	log.Debug().
		Str("agentId", host.ID).
		Str("hostname", host.Hostname).
		Str("platform", host.Platform).
		Msg("Agent report processed")

	h.broadcastState(r.Context())

	// Include any server-side config overrides in the response
	serverConfig := h.getMonitor(r.Context()).GetHostAgentConfig(host.ID)
	serverConfig = sanitizeHostAgentConfigForToken(serverConfig, tokenRecord, host)

	resp := map[string]any{
		"success":   true,
		"agentId":   host.ID,
		"lastSeen":  host.LastSeen,
		"platform":  host.Platform,
		"osName":    host.OSName,
		"osVersion": host.OSVersion,
	}
	if h.serverVersion != "" {
		resp["serverVersion"] = h.serverVersion
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

func reconcileInstallTokenCommandPolicy(monitor *monitoring.Monitor, requestRecord *config.APITokenRecord, host models.Host) error {
	if monitor == nil || requestRecord == nil || strings.TrimSpace(requestRecord.ID) == "" || strings.TrimSpace(host.ID) == "" {
		return nil
	}

	cfg := monitor.GetConfig()
	if cfg == nil {
		return nil
	}

	// Authentication stores a clone in the request context. Re-read the live
	// record so concurrent scope changes and an already-consumed intent marker
	// are authoritative.
	var record *config.APITokenRecord
	config.Mu.RLock()
	for index := range cfg.APITokens {
		if cfg.APITokens[index].ID == requestRecord.ID {
			clone := cfg.APITokens[index].Clone()
			record = &clone
			break
		}
	}
	config.Mu.RUnlock()
	if record == nil {
		return nil
	}

	requestedEnabled, hasIntent := agenttokens.ParseCommandPolicyIntent(record)
	if !hasIntent || strings.TrimSpace(record.Metadata[agenttokens.CommandPolicyAppliedAgentIDMetadataKey]) != "" {
		return nil
	}
	if !evaluateAgentExecBinding(record, host.ID, host.Hostname).admit {
		// Intent metadata is useful only on a Pulse-minted install token whose
		// binding policy admits this identity. Hand-created API tokens remain
		// unable to turn themselves into command credentials.
		return nil
	}

	// Scope removal always wins over the original enabled intent. Conversely,
	// adding agent:exec later does not rewrite a deliberately disabled install
	// intent; the explicit token-scope editor and desired policy remain separate
	// controls.
	effectiveEnabled := requestedEnabled && record.HasScope(config.ScopeAgentExec)
	if err := monitor.UpdateHostAgentConfig(host.ID, &effectiveEnabled); err != nil {
		return fmt.Errorf("persist desired command policy: %w", err)
	}

	persistence := monitor.GetConfigPersistence()
	config.Mu.Lock()
	defer config.Mu.Unlock()
	for index := range cfg.APITokens {
		live := &cfg.APITokens[index]
		if live.ID != requestRecord.ID {
			continue
		}
		if strings.TrimSpace(live.Metadata[agenttokens.CommandPolicyAppliedAgentIDMetadataKey]) != "" {
			return nil
		}
		if live.Metadata == nil {
			live.Metadata = make(map[string]string)
		}
		live.Metadata[agenttokens.CommandPolicyAppliedAgentIDMetadataKey] = host.ID
		if persistence != nil {
			if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
				delete(live.Metadata, agenttokens.CommandPolicyAppliedAgentIDMetadataKey)
				return fmt.Errorf("persist install-token command-policy marker: %w", err)
			}
		}
		log.Info().
			Str("agent_id", host.ID).
			Str("hostname", host.Hostname).
			Str("token_id", live.ID).
			Bool("commands_enabled", effectiveEnabled).
			Msg("Applied install-token command policy to host")
		return nil
	}

	return nil
}

// HandleLookup returns agent registration details for installer validation.
func (h *UnifiedAgentHandlers) HandleLookup(w http.ResponseWriter, r *http.Request) {
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

	// Use the live host snapshot (not the global mock snapshot) so agent
	// registrations can still be validated while Pulse is in mock/demo mode.
	hosts := h.getMonitor(r.Context()).GetLiveHostsSnapshot()

	var (
		host  models.Host
		found bool
	)

	if lookupID != "" {
		for _, candidate := range hosts {
			if candidate.ID == lookupID {
				host = candidate
				found = true
				break
			}
		}
	}

	if !found && hostname != "" {
		// First pass: exact hostname match (case-insensitive) when unique.
		var exactMatch *models.Host
		for i := range hosts {
			candidate := &hosts[i]
			if strings.EqualFold(candidate.Hostname, hostname) {
				if exactMatch != nil {
					exactMatch = nil
					break
				}
				exactMatch = candidate
			}
		}
		if exactMatch != nil {
			host = *exactMatch
			found = true
		}

		// Second pass: display-name match, but only when unambiguous.
		if !found {
			var displayMatch *models.Host
			for i := range hosts {
				candidate := &hosts[i]
				if strings.EqualFold(candidate.DisplayName, hostname) {
					if displayMatch != nil {
						displayMatch = nil
						break
					}
					displayMatch = candidate
				}
			}
			if displayMatch != nil {
				host = *displayMatch
				found = true
			}
		}

		// Third pass: short hostname match, but only when unique.
		if !found {
			// Helper to get short hostname (before first dot)
			getShortName := func(h string) string {
				if idx := strings.Index(h, "."); idx != -1 {
					return h[:idx]
				}
				return h
			}

			shortLookup := getShortName(hostname)
			var shortMatch *models.Host
			for i := range hosts {
				candidate := &hosts[i]
				if strings.EqualFold(getShortName(candidate.Hostname), shortLookup) {
					if shortMatch != nil {
						shortMatch = nil
						break
					}
					shortMatch = candidate
				}
			}
			if shortMatch != nil {
				host = *shortMatch
				found = true
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
func (h *UnifiedAgentHandlers) HandleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only DELETE is allowed", nil)
		return
	}

	// Extract agent ID from URL path.
	// Expected format: /api/agents/agent/{agentId} or legacy /api/agents/host/{agentId}
	trimmedPath := trimUnifiedAgentRoutePath(r.URL.Path)
	agentID := strings.TrimSpace(trimmedPath)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	// Remove the agent from state.
	host, err := h.getMonitor(r.Context()).RemoveHostAgent(agentID)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "agent_removal_failed", err.Error(), nil)
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

// HandleAllowReenroll clears the removal block for a host agent to permit future reports.
func (h *UnifiedAgentHandlers) HandleAllowReenroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed", nil)
		return
	}

	trimmedPath := trimUnifiedAgentRoutePath(r.URL.Path)
	trimmedPath = strings.TrimSuffix(trimmedPath, "/allow-reenroll")
	agentID := strings.TrimSpace(trimmedPath)
	if agentID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "missing_agent_id", "agentId is required", nil)
		return
	}

	if err := h.getMonitor(r.Context()).AllowHostAgentReenroll(agentID); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "agent_reenroll_failed", err.Error(), nil)
		return
	}

	h.broadcastState(r.Context())

	if err := utils.WriteJSONResponse(w, map[string]any{
		"success": true,
		"agentId": agentID,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to serialize host-agent allow reenroll response")
	}
}

// HandleConfig handles GET (fetch config) and PATCH (update config) for agents.
// GET /api/agents/agent/{agentId}/config - Agent fetches its server-side config.
// PATCH /api/agents/agent/{agentId}/config - UI updates agent config (e.g., commandsEnabled).
// Legacy clients may also use /api/agents/host/{agentId}/config.
func (h *UnifiedAgentHandlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from URL path.
	// Expected format: /api/agents/agent/{agentId}/config or legacy /api/agents/host/{agentId}/config
	trimmedPath := trimUnifiedAgentRoutePath(r.URL.Path)
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

func (h *UnifiedAgentHandlers) canReadConfig(record *config.APITokenRecord) bool {
	if record == nil {
		return true
	}
	return record.HasScope(config.ScopeAgentConfigRead) ||
		// Older host/docker-agent tokens may only carry agent:report (legacy
		// host-agent:report). Allow them to keep fetching their own config via
		// token binding — resolveConfigAgent below restricts a report-only token
		// to the host it is bound to.
		record.HasScope(config.ScopeAgentReport) ||
		record.HasScope(config.ScopeAgentManage) ||
		record.HasScope(config.ScopeSettingsWrite)
}

func (h *UnifiedAgentHandlers) resolveConfigAgent(ctx context.Context, agentID string, record *config.APITokenRecord) (models.Host, bool) {
	// Use the live host snapshot so agents can still fetch config while
	// Pulse is running in mock/demo mode.
	monitor := h.getMonitor(ctx)
	hosts := monitor.GetLiveHostsSnapshot()

	if record == nil || record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeAgentManage) {
		for _, candidate := range hosts {
			if candidate.ID == agentID {
				return candidate, true
			}
		}
		// Live state loses agent-reported hosts across monitor reloads and
		// restarts until the next report lands; fall back to the persisted
		// continuity store so config fetches in that window do not 404 (#1570).
		return monitor.MatchHostConfigContinuity(agentID, "")
	}

	for _, candidate := range hosts {
		if candidate.TokenID != "" && candidate.TokenID == record.ID {
			return candidate, true
		}
	}

	return monitor.MatchHostConfigContinuity(agentID, record.ID)
}

func (h *UnifiedAgentHandlers) signAgentConfig(agentID string, cfg monitoring.HostAgentConfig) (monitoring.HostAgentConfig, error) {
	var err error
	cfg, err = ensureDesiredAgentConfigMetadata(cfg)
	if err != nil {
		return cfg, err
	}

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

func ensureDesiredAgentConfigMetadata(cfg monitoring.HostAgentConfig) (monitoring.HostAgentConfig, error) {
	metadata, err := remoteconfig.BuildDesiredConfigMetadata(cfg.CommandsEnabled, cfg.Settings)
	if err != nil {
		return cfg, fmt.Errorf("failed to build desired config metadata: %w", err)
	}
	cfg.DesiredConfig = &metadata
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
func (h *UnifiedAgentHandlers) handleGetConfig(w http.ResponseWriter, r *http.Request, agentID string) {
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
	config = sanitizeHostAgentConfigForToken(config, record, host)
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

func sanitizeHostAgentConfigForToken(cfg monitoring.HostAgentConfig, record *config.APITokenRecord, host models.Host) monitoring.HostAgentConfig {
	if cfg.CommandsEnabled == nil || !*cfg.CommandsEnabled || commandConfigAllowedForToken(record, host) {
		return cfg
	}

	disabled := false
	cfg.CommandsEnabled = &disabled
	return cfg
}

func commandConfigAllowedForToken(record *config.APITokenRecord, host models.Host) bool {
	if record == nil {
		return true
	}
	if !record.HasScope(config.ScopeAgentExec) {
		log.Warn().
			Str("token_id", record.ID).
			Str("agent_id", host.ID).
			Str("hostname", host.Hostname).
			Msg("Suppressing enabled remote commands for agent: token missing required scope agent:exec")
		return false
	}

	// Mirror the command-channel admission decision exactly. Telling an agent
	// commands are enabled when its channel registration would be rejected
	// strands the host on "Remote control blocked" (the agent reports
	// CommandsEnabled=true forever while no channel can be admitted).
	if !evaluateAgentExecBinding(record, host.ID, host.Hostname).admit {
		// This gate runs before the agent ever attempts command-channel
		// registration, so without a log here a refused binding leaves no
		// trace anywhere: the agent never learns commands were requested and
		// the channel rejection warnings never fire (#1728).
		log.Warn().
			Str("token_id", record.ID).
			Str("agent_id", host.ID).
			Str("hostname", host.Hostname).
			Str("bound_agent_id", strings.TrimSpace(record.Metadata["bound_agent_id"])).
			Str("bound_hostname", strings.TrimSpace(record.Metadata["bound_hostname"])).
			Str("install_type", strings.TrimSpace(record.Metadata["install_type"])).
			Str("issued_via", strings.TrimSpace(record.Metadata["issued_via"])).
			Msg("Suppressing enabled remote commands for agent: exec binding would reject this token and agent identity")
		return false
	}
	return true
}

func (h *UnifiedAgentHandlers) ensureAgentTokenMatch(w http.ResponseWriter, r *http.Request, agentID string) bool {
	record := getAPITokenRecordFromRequest(r)
	if record == nil {
		return true
	}

	if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeWildcard) {
		return true
	}

	// Use the live host snapshot so mock/demo mode doesn't block agent auth checks.
	hosts := h.getMonitor(r.Context()).GetLiveHostsSnapshot()
	for _, host := range hosts {
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
func (h *UnifiedAgentHandlers) handlePatchConfig(w http.ResponseWriter, r *http.Request, agentID string) {
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
func (h *UnifiedAgentHandlers) HandleUninstall(w http.ResponseWriter, r *http.Request) {
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

	monitor := h.getMonitor(r.Context())
	record := getAPITokenRecordFromRequest(r)
	var err error
	if record != nil {
		_, err = monitor.UninstallHostAgent(agentID, record.ID)
	} else {
		// Preserve the existing administrative/session removal surface. Collector
		// self-uninstall always takes the exact-token durable path above.
		_, err = monitor.RemoveHostAgent(agentID)
	}
	if err != nil {
		switch {
		case errors.Is(err, monitoring.ErrHostAgentTokenMismatch):
			writeErrorResponse(w, http.StatusForbidden, "agent_lookup_forbidden", "Agent does not belong to this API token", nil)
		case errors.Is(err, monitoring.ErrHostAgentTokenShared):
			writeErrorResponse(w, http.StatusConflict, "agent_token_shared", "Collector credential is still used by another agent and must be rotated before uninstall", nil)
		case errors.Is(err, monitoring.ErrHostAgentNotFound):
			writeErrorResponse(w, http.StatusNotFound, "agent_not_found", "Agent has not registered with Pulse yet", nil)
		default:
			log.Error().Err(err).Str("agentId", agentID).Msg("Collector uninstall transaction failed")
			writeErrorResponse(w, http.StatusInternalServerError, "agent_uninstall_failed", "Pulse could not durably remove the agent", nil)
		}
		return
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
func (h *UnifiedAgentHandlers) HandleLink(w http.ResponseWriter, r *http.Request) {
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
func (h *UnifiedAgentHandlers) HandleUnlink(w http.ResponseWriter, r *http.Request) {
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
