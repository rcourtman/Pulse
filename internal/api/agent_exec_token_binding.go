package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
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

// agentExecHostnamesMatch compares a bound hostname against the hostname an
// agent reports. Short-name vs fully-qualified variants of the same host must
// compare equal here the same way they do everywhere else in the system
// (unifiedresources.HostnamesEquivalent); the case-insensitive exact branch
// keeps IP-literal hostnames comparable, which HostnamesEquivalent rejects.
func agentExecHostnamesMatch(bound, requested string) bool {
	return agentbinding.HostnamesMatch(bound, requested)
}

// agentExecBindingDecision is the single source of truth for whether an
// already-issued agent exec token accepts a registering agent identity, and
// which metadata repair the admission path must persist when it does.
type agentExecBindingDecision struct {
	admit          bool
	firstBind      bool
	legacyMigrate  bool
	rebindHostname bool
	backfillID     bool
	backfillHost   bool
}

// evaluateAgentExecBinding computes the admission decision for a token record
// and a requesting agent identity without mutating the record. Both the
// command-channel admission (admitAgentExecToken) and the agent config gate
// (commandConfigAllowedForToken) consume this decision: v6.1.2 shipped them as
// two divergent policies, so an agent could be told commands were enabled
// while its command channel was rejected, leaving the host permanently on
// "Remote control blocked" with reinstall as the only recourse.
func evaluateAgentExecBinding(record *config.APITokenRecord, requestedID, requestedHost string) agentExecBindingDecision {
	decision := agentbinding.Evaluate(record, requestedID, requestedHost)
	return agentExecBindingDecision{
		admit:          decision.Admit,
		firstBind:      decision.FirstBind,
		legacyMigrate:  decision.LegacyMigrate,
		rebindHostname: decision.RebindHostname,
		backfillID:     decision.BackfillID,
		backfillHost:   decision.BackfillHost,
	}
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
	runtimeRole := strings.TrimSpace(record.Metadata[agenttokens.RuntimeRoleMetadataKey])
	if runtimeRole == agenttokens.CredentialKindMonitoringCollector {
		config.Mu.Unlock()
		log.Warn().Str("token_id", tokenID).Msg("Monitoring collector credential rejected by action listener")
		return agentexec.AgentAdmission{}, false
	}
	if runtimeRole == agenttokens.CredentialKindActionRunner {
		if len(orgs) != 1 {
			config.Mu.Unlock()
			log.Warn().Str("token_id", tokenID).Msg("Action runner credential requires one explicit organization binding")
			return agentexec.AgentAdmission{}, false
		}
		decision := agentbinding.EvaluateActionRunner(record, requestedID, requestedHost)
		if !decision.Admit {
			config.Mu.Unlock()
			log.Warn().Str("token_id", tokenID).Msg("Action runner credential binding or capability mismatch")
			return agentexec.AgentAdmission{}, false
		}
		capability := strings.TrimSpace(record.Metadata[agenttokens.ActionCapabilityMetadataKey])
		config.Mu.Unlock()
		return agentexec.AgentAdmission{
			OrganizationID:   organizationID,
			TokenID:          tokenID,
			AgentID:          requestedID,
			Hostname:         requestedHost,
			RuntimeRole:      agentexec.RuntimeRoleActionRunner,
			ActionCapability: capability,
		}, true
	}
	if runtimeRole != "" && runtimeRole != agenttokens.CredentialKindLegacyFullTrust {
		config.Mu.Unlock()
		log.Warn().Str("token_id", tokenID).Str("runtime_role", runtimeRole).Msg("Agent exec token has unsupported runtime role")
		return agentexec.AgentAdmission{}, false
	}

	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])
	decision := evaluateAgentExecBinding(record, requestedID, requestedHost)

	switch {
	case decision.firstBind:
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
			agenttokens.RuntimeRoleMetadataKey,
		)
		record.Metadata["bound_agent_id"] = requestedID
		// A bound_hostname already written by the Proxmox auto-register
		// bootstrap is authoritative: it is what the still-unconsumed install
		// grant compares against, so an equivalent-but-different spelling
		// reported by the agent must not overwrite it.
		if strings.TrimSpace(record.Metadata["bound_hostname"]) == "" {
			record.Metadata["bound_hostname"] = requestedHost
		}
		record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
		record.Metadata[agentExecBindingVersionKey] = agentExecBindingVersion
		record.Metadata[agenttokens.RuntimeRoleMetadataKey] = agenttokens.CredentialKindLegacyFullTrust
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
			RuntimeRole:    agentexec.RuntimeRoleLegacyFullTrust,
		}, true

	case decision.legacyMigrate:
		previousID := boundID
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_at",
			agentExecBindingVersionKey,
			agenttokens.RuntimeRoleMetadataKey,
		)
		record.Metadata["bound_agent_id"] = requestedID
		record.Metadata["bound_at"] = time.Now().UTC().Format(time.RFC3339)
		record.Metadata[agentExecBindingVersionKey] = agentExecBindingVersion
		record.Metadata[agenttokens.RuntimeRoleMetadataKey] = agenttokens.CredentialKindLegacyFullTrust
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
			RuntimeRole:    agentexec.RuntimeRoleLegacyFullTrust,
		}, true

	case decision.admit:
		previousHost := boundHost
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_hostname",
			"bound_at",
			agentExecBindingVersionKey,
			agenttokens.RuntimeRoleMetadataKey,
		)
		metadataChanged := runtimeRole == ""
		if runtimeRole == "" {
			record.Metadata[agenttokens.RuntimeRoleMetadataKey] = agenttokens.CredentialKindLegacyFullTrust
		}
		if decision.backfillID {
			record.Metadata["bound_agent_id"] = requestedID
			boundID = requestedID
			metadataChanged = true
		}
		if decision.backfillHost || decision.rebindHostname {
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
		if decision.rebindHostname {
			log.Info().
				Str("token_id", tokenID).
				Str("agent_id", requestedID).
				Str("previous_hostname", previousHost).
				Str("hostname", requestedHost).
				Msg("Re-bound agent exec token hostname for matching immutable agent identity")
		}
		return agentexec.AgentAdmission{
			OrganizationID: organizationID,
			TokenID:        tokenID,
			AgentID:        requestedID,
			Hostname:       requestedHost,
			RuntimeRole:    agentexec.RuntimeRoleLegacyFullTrust,
		}, true
	}

	config.Mu.Unlock()
	if boundID == "" && boundHost == "" {
		log.Warn().
			Str("token_id", tokenID).
			Msg("Agent exec token missing binding metadata")
		return agentexec.AgentAdmission{}, false
	}
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
		runtimeRole := strings.TrimSpace(record.Metadata[agenttokens.RuntimeRoleMetadataKey])
		if runtimeRole == agenttokens.CredentialKindActionRunner {
			return len(orgs) == 1 &&
				strings.TrimSpace(admission.RuntimeRole) == agentexec.RuntimeRoleActionRunner &&
				strings.TrimSpace(admission.ActionCapability) == agentexec.ActionCapabilityTypedV1 &&
				organizationID == strings.TrimSpace(admission.OrganizationID) &&
				agentbinding.EvaluateActionRunner(record, requestedID, requestedHost).Admit
		}
		if runtimeRole != agenttokens.CredentialKindLegacyFullTrust {
			return false
		}
		return strings.TrimSpace(admission.RuntimeRole) == agentexec.RuntimeRoleLegacyFullTrust &&
			organizationID == strings.TrimSpace(admission.OrganizationID) &&
			strings.TrimSpace(record.Metadata["bound_agent_id"]) == requestedID &&
			agentExecHostnamesMatch(strings.TrimSpace(record.Metadata["bound_hostname"]), requestedHost)
	}
	return false
}

// agentCommandSessionConnected reports whether a live command channel exists
// for a telemetry host. host.TokenID is sticky across token revocation and
// rotation (monitoring keeps the last-seen token on the host record), so a
// token-scoped miss must not be authoritative: fall through to the agent-ID
// and hostname lookups before declaring the channel disconnected. The
// token-first order still lets the canonical enrollment token win when its
// session is live.
func (r *Router) agentCommandSessionConnected(organizationID, tokenID, agentID, hostname string) bool {
	if r == nil || r.agentExecServer == nil {
		return false
	}
	if strings.TrimSpace(tokenID) != "" {
		if _, connected := r.agentExecServer.GetAgentForTokenForOrganization(organizationID, tokenID); connected {
			return true
		}
	}
	if strings.TrimSpace(agentID) != "" && r.agentExecServer.IsAgentConnectedForOrganization(organizationID, agentID) {
		return true
	}
	_, connected := r.agentExecServer.GetAgentForHostForOrganization(organizationID, hostname)
	return connected
}

func canBindAgentInstallExecToken(record *config.APITokenRecord, agentID string, hostname string) bool {
	return agentbinding.CanBindInstallToken(record, agentID, hostname)
}

// canBindAutoRegisteredAgentInstallExecToken reports whether an install token
// whose bound_hostname was populated by the Proxmox auto-register bootstrap may
// take the clean first-use exec bind. The registration path writes
// bound_hostname without bound_agent_id or a binding version, so the record
// looks identical to a pre-v6.1.1 deploy token; without this the first command
// enrollment of a freshly installed Proxmox host is admitted by the
// legacy-migration branch, which exists to repair old records and is not the
// contract this flow should depend on. Hostname equivalence is still required.
func canBindAutoRegisteredAgentInstallExecToken(record *config.APITokenRecord, agentID string, hostname string) bool {
	return agentbinding.CanBindAutoRegisteredInstallToken(record, agentID, hostname)
}
