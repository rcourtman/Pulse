package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

type hostedTenantAgentInstallCommandResponse struct {
	OrgID   string `json:"orgId"`
	Command string `json:"command"`
	Token   string `json:"token"`
}

type HostedTenantAgentInstallCommandOptions struct {
	Config        *config.Config
	Persistence   *config.ConfigPersistence
	MultiTenant   *config.MultiTenantPersistence
	TenantMonitor *monitoring.MultiTenantMonitor
	HostedMode    bool
	OrgID         string
	InstallType   string
	OwnerUserID   string
	BaseURL       string
}

type HostedTenantAgentInstallCommandResult struct {
	OrgID       string
	InstallType string
	Command     string
	Token       string
	TokenID     string
}

var (
	ErrHostedTenantInstallRequiresHostedMode = errors.New("hosted tenant agent install command requires hosted mode")
	ErrHostedTenantInstallMissingOrgID       = errors.New("hosted tenant agent install command requires org id")
	ErrHostedTenantInstallInvalidOrg         = errors.New("hosted tenant agent install command org does not exist")
	ErrHostedTenantInstallMissingBaseURL     = errors.New("hosted tenant agent install command requires base url")
)

func GenerateHostedTenantAgentInstallCommand(opts HostedTenantAgentInstallCommandOptions) (*HostedTenantAgentInstallCommandResult, error) {
	if !opts.HostedMode {
		return nil, ErrHostedTenantInstallRequiresHostedMode
	}
	orgID := strings.TrimSpace(opts.OrgID)
	if orgID == "" {
		return nil, ErrHostedTenantInstallMissingOrgID
	}
	if opts.MultiTenant == nil || !opts.MultiTenant.OrgExists(orgID) {
		return nil, ErrHostedTenantInstallInvalidOrg
	}
	installType, err := normalizeProxmoxInstallType(opts.InstallType)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		return nil, ErrHostedTenantInstallMissingBaseURL
	}

	tokenName := fmt.Sprintf("cloud-tenant-agent-%s-%s-%d", orgID, installType, time.Now().UTC().Unix())
	rawToken, record, err := issueAndPersistAgentInstallToken(opts.Config, opts.Persistence, issueAgentInstallTokenOptions{
		TokenName:   tokenName,
		OrgID:       orgID,
		OwnerUserID: strings.TrimSpace(opts.OwnerUserID),
		Metadata: map[string]string{
			"install_type": installType,
			"issued_via":   "hosted_agent_install_command",
		},
	})
	if err != nil {
		return nil, err
	}

	// If the tenant monitor is already initialized, ensure it sees the new token immediately.
	// If not initialized, future GetMonitor() calls will deep-copy the updated base config.
	config.Mu.Lock()
	if opts.TenantMonitor != nil {
		if m, ok := opts.TenantMonitor.PeekMonitor(orgID); ok && m != nil && m.GetConfig() != nil {
			m.GetConfig().APITokens = append(m.GetConfig().APITokens, *record)
			m.GetConfig().SortAPITokens()
		}
	}
	config.Mu.Unlock()

	command := buildProxmoxAgentInstallCommand(agentInstallCommandOptions{
		BaseURL:            baseURL,
		Token:              rawToken,
		InstallType:        installType,
		IncludeInstallType: true,
	})

	tokenID := ""
	if record != nil {
		tokenID = record.ID
	}
	return &HostedTenantAgentInstallCommandResult{
		OrgID:       orgID,
		InstallType: installType,
		Command:     command,
		Token:       rawToken,
		TokenID:     tokenID,
	}, nil
}

// handleHostedTenantAgentInstallCommand is a hosted-mode-only control-plane endpoint that generates a
// tenant-scoped agent install command by minting an org-bound API token.
func (r *Router) handleHostedTenantAgentInstallCommand(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	orgID := strings.TrimSpace(req.PathValue("id"))
	var payload struct {
		Type string `json:"type"` // "pve" or "pbs"
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	result, err := GenerateHostedTenantAgentInstallCommand(HostedTenantAgentInstallCommandOptions{
		Config:        r.config,
		Persistence:   r.persistence,
		MultiTenant:   r.multiTenant,
		TenantMonitor: r.mtMonitor,
		HostedMode:    r.hostedMode,
		OrgID:         orgID,
		InstallType:   payload.Type,
		OwnerUserID:   apiTokenOwnerUserIDForRequest(r.config, req),
		BaseURL:       r.resolvePublicURL(req),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrHostedTenantInstallRequiresHostedMode):
			http.NotFound(w, req)
		case errors.Is(err, ErrHostedTenantInstallMissingOrgID):
			writeErrorResponse(w, http.StatusBadRequest, "missing_org_id", "Organization ID required", nil)
		case errors.Is(err, ErrHostedTenantInstallInvalidOrg):
			writeErrorResponse(w, http.StatusBadRequest, "invalid_org", "Invalid Organization ID", nil)
		case errors.Is(err, ErrHostedTenantInstallMissingBaseURL):
			writeErrorResponse(w, http.StatusInternalServerError, "missing_base_url", "Failed to resolve Pulse URL", nil)
		case errors.Is(err, errAgentInstallTokenGeneration):
			log.Error().Err(err).Msg("Failed to generate hosted tenant agent API token")
			writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		case errors.Is(err, errAgentInstallTokenRecord):
			log.Error().Err(err).Str("org_id", orgID).Msg("Failed to construct hosted tenant agent token record")
			writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate token", nil)
		case errors.Is(err, errAgentInstallTokenPersist):
			log.Error().Err(err).Msg("Failed to persist hosted tenant agent token")
			writeErrorResponse(w, http.StatusInternalServerError, "token_persist_failed", "Failed to save token to disk", map[string]string{"error": err.Error()})
		case strings.Contains(err.Error(), "Type must be"):
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		default:
			log.Error().Err(err).Msg("Failed to create hosted tenant agent token")
			writeErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate API token", nil)
		}
		return
	}

	writeJSON(w, http.StatusOK, hostedTenantAgentInstallCommandResponse{
		OrgID:   result.OrgID,
		Command: result.Command,
		Token:   result.Token,
	})
}
