package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	agentInstallIssuedViaConfig = "config_agent_install_command"
	agentInstallIssuedViaHosted = "hosted_agent_install_command"
	agentExecBindingVersionKey  = "agent_exec_binding_version"
	agentExecBindingVersion     = "2"
)

type agentExecMetadataValue struct {
	value   string
	present bool
}

func snapshotAgentExecMetadata(metadata map[string]string, keys ...string) map[string]agentExecMetadataValue {
	snapshot := make(map[string]agentExecMetadataValue, len(keys))
	for _, key := range keys {
		value, present := metadata[key]
		snapshot[key] = agentExecMetadataValue{value: value, present: present}
	}
	return snapshot
}

func restoreAgentExecMetadata(metadata map[string]string, snapshot map[string]agentExecMetadataValue) {
	for key, previous := range snapshot {
		if previous.present {
			metadata[key] = previous.value
		} else {
			delete(metadata, key)
		}
	}
}

func (r *Router) validateAgentExecToken(token string, agentID string, hostname string) bool {
	_, ok := r.admitAgentExecToken(token, agentID, hostname)
	return ok
}

func (r *Router) admitAgentExecToken(token string, agentID string, hostname string) (agentexec.AgentAdmission, bool) {
	if r == nil || r.config == nil {
		return agentexec.AgentAdmission{}, false
	}

	requestedID := strings.TrimSpace(agentID)
	requestedHost := strings.TrimSpace(hostname)

	config.Mu.Lock()
	record, ok := r.config.ValidateAPIToken(token)
	if !ok {
		config.Mu.Unlock()
		// This is the branch a stale-enrollment agent hits: it holds a token
		// from a prior install that this server no longer recognises. It was
		// previously the only rejection path with no log, which made a looping
		// "Invalid token" agent impossible to diagnose without reading source.
		log.Warn().
			Str("agent_id", requestedID).
			Str("hostname", requestedHost).
			Msg("Agent exec token not recognized by this server — re-run the agent installer to re-enroll this agent")
		return agentexec.AgentAdmission{}, false
	}

	tokenID := record.ID
	if !record.HasScope(config.ScopeAgentExec) {
		config.Mu.Unlock()
		log.Warn().
			Str("token_id", tokenID).
			Msg("Agent exec token missing required scope: agent:exec")
		return agentexec.AgentAdmission{}, false
	}
	orgs := record.GetBoundOrgs()
	if len(orgs) > 1 {
		config.Mu.Unlock()
		log.Warn().
			Str("token_id", tokenID).
			Strs("organization_ids", orgs).
			Msg("Agent exec token rejected because command sessions require one organization binding")
		return agentexec.AgentAdmission{}, false
	}
	organizationID := "default"
	if len(orgs) == 1 && strings.TrimSpace(orgs[0]) != "" {
		organizationID = strings.TrimSpace(orgs[0])
	}

	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])
	if boundID == "" && boundHost == "" && canBindAgentInstallExecToken(record, requestedID, requestedHost) {
		issuedVia := strings.TrimSpace(record.Metadata["issued_via"])
		installType := strings.TrimSpace(record.Metadata["install_type"])
		if record.Metadata == nil {
			record.Metadata = make(map[string]string)
		}
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_hostname",
			"bound_at",
			agentExecBindingVersionKey,
		)
		record.Metadata["bound_agent_id"] = requestedID
		record.Metadata["bound_hostname"] = requestedHost
		record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
		record.Metadata[agentExecBindingVersionKey] = agentExecBindingVersion
		if r.persistence != nil {
			if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
				restoreAgentExecMetadata(record.Metadata, previousMetadata)
				config.Mu.Unlock()
				log.Error().
					Err(err).
					Str("token_id", tokenID).
					Str("hostname", requestedHost).
					Msg("Failed to persist first-use agent exec token binding; command registration denied")
				return agentexec.AgentAdmission{}, false
			}
		}
		config.Mu.Unlock()

		log.Info().
			Str("token_id", tokenID).
			Str("hostname", requestedHost).
			Str("issued_via", issuedVia).
			Str("install_type", installType).
			Msg("Bound agent install token to first command agent registration")
		return agentexec.AgentAdmission{
			OrganizationID: organizationID,
			TokenID:        tokenID,
			AgentID:        requestedID,
			Hostname:       requestedHost,
		}, true
	}

	if boundID == "" && boundHost == "" {
		config.Mu.Unlock()
		log.Warn().
			Str("token_id", tokenID).
			Msg("Agent exec token missing binding metadata")
		return agentexec.AgentAdmission{}, false
	}

	// Pre-v6.1.1 deploy tokens could carry a server-synthesized agent ID even
	// though the runtime derives its ID from machine-id. Migrate that
	// hostname-bound legacy record exactly once, then enforce both fields.
	if strings.TrimSpace(record.Metadata[agentExecBindingVersionKey]) != agentExecBindingVersion &&
		boundHost != "" && strings.EqualFold(boundHost, requestedHost) {
		previousID := boundID
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_at",
			agentExecBindingVersionKey,
		)
		record.Metadata["bound_agent_id"] = requestedID
		record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
		record.Metadata[agentExecBindingVersionKey] = agentExecBindingVersion
		if r.persistence != nil {
			if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
				restoreAgentExecMetadata(record.Metadata, previousMetadata)
				config.Mu.Unlock()
				log.Error().
					Err(err).
					Str("token_id", tokenID).
					Msg("Failed to persist legacy agent exec identity migration; command registration denied")
				return agentexec.AgentAdmission{}, false
			}
		}
		config.Mu.Unlock()
		log.Info().
			Str("token_id", tokenID).
			Str("previous_agent_id", previousID).
			Str("agent_id", requestedID).
			Str("hostname", requestedHost).
			Msg("Migrated legacy hostname-bound agent exec token to immutable runtime identity")
		return agentexec.AgentAdmission{
			OrganizationID: organizationID,
			TokenID:        tokenID,
			AgentID:        requestedID,
			Hostname:       requestedHost,
		}, true
	}

	idMatches := boundID == "" || boundID == requestedID
	hostMatches := boundHost == "" || strings.EqualFold(boundHost, requestedHost)
	if idMatches && hostMatches {
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_hostname",
			"bound_at",
			agentExecBindingVersionKey,
		)
		metadataChanged := false
		if boundID == "" && boundHost != "" {
			record.Metadata["bound_agent_id"] = requestedID
			boundID = requestedID
			metadataChanged = true
		}
		if boundHost == "" && boundID != "" {
			record.Metadata["bound_hostname"] = requestedHost
			boundHost = requestedHost
			metadataChanged = true
		}
		if metadataChanged {
			record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
			record.Metadata[agentExecBindingVersionKey] = agentExecBindingVersion
		}
		if metadataChanged && r.persistence != nil {
			if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
				restoreAgentExecMetadata(record.Metadata, previousMetadata)
				config.Mu.Unlock()
				log.Error().
					Err(err).
					Str("token_id", tokenID).
					Msg("Failed to persist migrated agent exec token binding; command registration denied")
				return agentexec.AgentAdmission{}, false
			}
		}
		config.Mu.Unlock()
		return agentexec.AgentAdmission{
			OrganizationID: organizationID,
			TokenID:        tokenID,
			AgentID:        requestedID,
			Hostname:       requestedHost,
		}, true
	}

	config.Mu.Unlock()
	log.Warn().
		Str("token_id", tokenID).
		Str("bound_id", boundID).
		Str("bound_hostname", boundHost).
		Str("requested_id", requestedID).
		Str("requested_hostname", requestedHost).
		Msg("Agent token mismatch: token is not bound to the registering agent")
	return agentexec.AgentAdmission{}, false
}

func (r *Router) validateAgentExecSession(admission agentexec.AgentAdmission) bool {
	if r == nil || r.config == nil {
		return false
	}
	tokenID := strings.TrimSpace(admission.TokenID)
	requestedID := strings.TrimSpace(admission.AgentID)
	requestedHost := strings.TrimSpace(admission.Hostname)
	if tokenID == "" || requestedID == "" || requestedHost == "" {
		return false
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()
	for index := range r.config.APITokens {
		record := &r.config.APITokens[index]
		if record.ID != tokenID || record.IsExpired() || !record.HasScope(config.ScopeAgentExec) {
			continue
		}
		orgs := record.GetBoundOrgs()
		organizationID := "default"
		if len(orgs) > 1 {
			return false
		}
		if len(orgs) == 1 && strings.TrimSpace(orgs[0]) != "" {
			organizationID = strings.TrimSpace(orgs[0])
		}
		return organizationID == strings.TrimSpace(admission.OrganizationID) &&
			strings.TrimSpace(record.Metadata["bound_agent_id"]) == requestedID &&
			strings.EqualFold(strings.TrimSpace(record.Metadata["bound_hostname"]), requestedHost)
	}
	return false
}

func canBindAgentInstallExecToken(record *config.APITokenRecord, agentID string, hostname string) bool {
	if record == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(hostname) == "" {
		return false
	}
	if strings.TrimSpace(record.Metadata["bound_agent_id"]) != "" ||
		strings.TrimSpace(record.Metadata["bound_hostname"]) != "" {
		return false
	}

	switch strings.TrimSpace(record.Metadata["install_type"]) {
	case proxmoxInstallTypePVE, proxmoxInstallTypePBS, agentInstallTypeHost:
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
