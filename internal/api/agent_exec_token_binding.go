package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	agentInstallIssuedViaConfig = "config_agent_install_command"
	agentInstallIssuedViaHosted = "hosted_agent_install_command"
)

func (r *Router) validateAgentExecToken(token string, agentID string, hostname string) bool {
	if r == nil || r.config == nil {
		return false
	}

	requestedID := strings.TrimSpace(agentID)
	requestedHost := strings.TrimSpace(hostname)

	config.Mu.Lock()
	record, ok := r.config.ValidateAPIToken(token)
	if !ok {
		config.Mu.Unlock()
		return false
	}

	tokenID := record.ID
	if !record.HasScope(config.ScopeAgentExec) {
		config.Mu.Unlock()
		log.Warn().
			Str("token_id", tokenID).
			Msg("Agent exec token missing required scope: agent:exec")
		return false
	}

	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])
	if boundID == "" && boundHost == "" && canBindProxmoxAgentInstallExecToken(record, requestedID, requestedHost) {
		issuedVia := strings.TrimSpace(record.Metadata["issued_via"])
		installType := strings.TrimSpace(record.Metadata["install_type"])
		if record.Metadata == nil {
			record.Metadata = make(map[string]string)
		}
		record.Metadata["bound_agent_id"] = requestedID
		record.Metadata["bound_hostname"] = requestedHost
		record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
		tokens := make([]config.APITokenRecord, len(r.config.APITokens))
		copy(tokens, r.config.APITokens)
		config.Mu.Unlock()

		if r.persistence != nil {
			if err := r.persistence.SaveAPITokens(tokens); err != nil {
				log.Warn().
					Err(err).
					Str("token_id", tokenID).
					Str("hostname", requestedHost).
					Msg("Failed to persist first-use Proxmox agent exec token binding")
			}
		}

		log.Info().
			Str("token_id", tokenID).
			Str("hostname", requestedHost).
			Str("issued_via", issuedVia).
			Str("install_type", installType).
			Msg("Bound Proxmox agent install token to first command agent registration")
		return true
	}

	if boundID == "" && boundHost == "" {
		config.Mu.Unlock()
		log.Warn().
			Str("token_id", tokenID).
			Msg("Agent exec token missing binding metadata")
		return false
	}

	if boundHost != "" && strings.EqualFold(boundHost, requestedHost) {
		config.Mu.Unlock()
		return true
	}

	if boundID != "" && boundID == requestedID {
		config.Mu.Unlock()
		return true
	}

	config.Mu.Unlock()
	log.Warn().
		Str("token_id", tokenID).
		Str("bound_id", boundID).
		Str("bound_hostname", boundHost).
		Str("requested_id", requestedID).
		Str("requested_hostname", requestedHost).
		Msg("Agent token mismatch: token is not bound to the registering agent")
	return false
}

func canBindProxmoxAgentInstallExecToken(record *config.APITokenRecord, agentID string, hostname string) bool {
	if record == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(hostname) == "" {
		return false
	}
	if strings.TrimSpace(record.Metadata["bound_agent_id"]) != "" ||
		strings.TrimSpace(record.Metadata["bound_hostname"]) != "" {
		return false
	}

	switch strings.TrimSpace(record.Metadata["install_type"]) {
	case proxmoxInstallTypePVE, proxmoxInstallTypePBS:
	default:
		return false
	}

	switch strings.TrimSpace(record.Metadata["issued_via"]) {
	case agentInstallIssuedViaConfig, agentInstallIssuedViaHosted:
		return true
	default:
		return false
	}
}
