package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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

	payload.Type = strings.ToLower(strings.TrimSpace(payload.Type))
	if payload.Type != "pve" && payload.Type != "pbs" {
		http.Error(w, "Type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate hosted tenant agent API token")
		http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		return
	}

	tokenName := fmt.Sprintf("cloud-tenant-agent-%s-%s-%d", orgID, payload.Type, time.Now().UTC().Unix())
	scopes := []string{
		config.ScopeHostReport,
		config.ScopeHostConfigRead,
		config.ScopeHostManage,
		config.ScopeAgentExec,
	}

	record, err := config.NewAPITokenRecord(rawToken, tokenName, scopes)
	if err != nil {
		log.Error().Err(err).Str("token_name", tokenName).Msg("Failed to construct hosted tenant agent token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	// Enforce tenant scoping even when callers omit X-Pulse-Org-ID (cloud agent UX).
	record.OrgID = orgID
	if record.Metadata == nil {
		record.Metadata = map[string]string{}
	}
	record.Metadata["install_type"] = payload.Type
	record.Metadata["issued_via"] = "hosted_agent_install_command"

	// Persist to the global token store. Multi-tenant auth relies on org binding.
	config.Mu.Lock()
	r.config.APITokens = append(r.config.APITokens, *record)
	r.config.SortAPITokens()
	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			// Roll back in-memory addition on persistence failure.
			r.config.APITokens = r.config.APITokens[:len(r.config.APITokens)-1]
			config.Mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist hosted tenant agent token")
			http.Error(w, "Failed to save token to disk: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// If the tenant monitor is already initialized, ensure it sees the new token immediately.
	// If not initialized, future GetMonitor() calls will deep-copy the updated base config.
	if r.mtMonitor != nil {
		if m, ok := r.mtMonitor.PeekMonitor(orgID); ok && m != nil && m.GetConfig() != nil {
			m.GetConfig().APITokens = append(m.GetConfig().APITokens, *record)
			m.GetConfig().SortAPITokens()
		}
	}
	config.Mu.Unlock()

	baseURL := strings.TrimRight(r.resolvePublicURL(req), "/")
	command := fmt.Sprintf(`curl -fsSL %s/install.sh | bash -s -- \
  --url %s \
  --token %s \
  --enable-proxmox \
  --proxmox-type %s`,
		baseURL, baseURL, rawToken, payload.Type)

	writeJSON(w, http.StatusOK, hostedTenantAgentInstallCommandResponse{
		OrgID:   orgID,
		Command: command,
		Token:   rawToken,
	})
}
