package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

type hostedTenantAgentInstallCommandResponse struct {
	OrgID   string `json:"orgId"`
	Command string `json:"command"`
	Token   string `json:"token"`
}

// handleHostedTenantAgentInstallCommand is a hosted-mode-only control-plane endpoint that generates a
// tenant-scoped agent install command by minting an org-bound API token.
func (r *Router) handleHostedTenantAgentInstallCommand(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !r.hostedMode {
		http.NotFound(w, req)
		return
	}

	orgID := strings.TrimSpace(req.PathValue("id"))
	if orgID == "" {
		http.Error(w, "Organization ID required", http.StatusBadRequest)
		return
	}
	if r.multiTenant == nil || !r.multiTenant.OrgExists(orgID) {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_org", "Invalid Organization ID", nil)
		return
	}

	var payload struct {
		Type string `json:"type"` // "pve" or "pbs"
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	installType, err := normalizeProxmoxInstallType(payload.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tokenName := fmt.Sprintf("cloud-tenant-agent-%s-%s-%d", orgID, installType, time.Now().UTC().Unix())
	rawToken, record, err := issueAndPersistAgentInstallToken(r.config, r.persistence, issueAgentInstallTokenOptions{
		TokenName: tokenName,
		OrgID:     orgID,
		Metadata: map[string]string{
			"install_type": installType,
			"issued_via":   "hosted_agent_install_command",
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, errAgentInstallTokenGeneration):
			log.Error().Err(err).Msg("Failed to generate hosted tenant agent API token")
			http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		case errors.Is(err, errAgentInstallTokenRecord):
			log.Error().Err(err).Str("token_name", tokenName).Msg("Failed to construct hosted tenant agent token record")
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		case errors.Is(err, errAgentInstallTokenPersist):
			log.Error().Err(err).Msg("Failed to persist hosted tenant agent token")
			http.Error(w, "Failed to save token to disk: "+err.Error(), http.StatusInternalServerError)
		default:
			log.Error().Err(err).Msg("Failed to create hosted tenant agent token")
			http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		}
		return
	}

	// If the tenant monitor is already initialized, ensure it sees the new token immediately.
	// If not initialized, future GetMonitor() calls will deep-copy the updated base config.
	config.Mu.Lock()
	if r.mtMonitor != nil {
		if m, ok := r.mtMonitor.PeekMonitor(orgID); ok && m != nil && m.GetConfig() != nil {
			m.GetConfig().APITokens = append(m.GetConfig().APITokens, *record)
			m.GetConfig().SortAPITokens()
		}
	}
	config.Mu.Unlock()

	baseURL := strings.TrimRight(r.resolvePublicURL(req), "/")
	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            baseURL,
		Token:              rawToken,
		InstallType:        installType,
		IncludeInstallType: true,
	})

	writeJSON(w, http.StatusOK, hostedTenantAgentInstallCommandResponse{
		OrgID:   orgID,
		Command: command,
		Token:   rawToken,
	})
}
