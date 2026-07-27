package api

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	return strings.EqualFold(bound, requested) || unifiedresources.HostnamesEquivalent(bound, requested)
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
	if record == nil {
		return agentExecBindingDecision{}
	}
	requestedID = strings.TrimSpace(requestedID)
	requestedHost = strings.TrimSpace(requestedHost)
	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])

	if boundID == "" && boundHost == "" {
		if canBindAgentInstallExecToken(record, requestedID, requestedHost) {
			return agentExecBindingDecision{admit: true, firstBind: true}
		}
		return agentExecBindingDecision{}
	}

	// Pre-v6.1.1 deploy tokens could carry a server-synthesized agent ID even
	// though the runtime derives its ID from machine-id. Migrate that
	// hostname-bound legacy record exactly once, then enforce identity.
	if strings.TrimSpace(record.Metadata[agentExecBindingVersionKey]) != agentExecBindingVersion &&
		boundHost != "" && agentExecHostnamesMatch(boundHost, requestedHost) {
		return agentExecBindingDecision{admit: true, legacyMigrate: true}
	}

	idMatches := boundID == "" || boundID == requestedID
	hostMatches := boundHost == "" || agentExecHostnamesMatch(boundHost, requestedHost)
	// The runtime agent ID is immutable machine identity while hostnames can
	// be renamed after enrollment, so an exact ID match re-binds a drifted
	// hostname rather than stranding the host: v6.1.1 admitted these agents
	// under an ID-or-hostname rule, and rejecting them afterwards leaves no
	// operator recourse short of reinstalling the agent.
	rebindHostname := boundID != "" && boundID == requestedID && !hostMatches && requestedHost != ""
	if !idMatches || (!hostMatches && !rebindHostname) {
		return agentExecBindingDecision{}
	}
	return agentExecBindingDecision{
		admit:          true,
		rebindHostname: rebindHostname,
		backfillID:     boundID == "" && boundHost != "",
		backfillHost:   boundHost == "" && boundID != "",
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

	case decision.legacyMigrate:
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

	case decision.admit:
		previousHost := boundHost
		previousMetadata := snapshotAgentExecMetadata(
			record.Metadata,
			"bound_agent_id",
			"bound_hostname",
			"bound_at",
			agentExecBindingVersionKey,
		)
		metadataChanged := false
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
		return organizationID == strings.TrimSpace(admission.OrganizationID) &&
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
