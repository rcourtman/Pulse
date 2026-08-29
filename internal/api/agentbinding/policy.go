// Package agentbinding owns the immutable policy for install-token command-channel binding.
package agentbinding

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	VersionKey      = "agent_exec_binding_version"
	Version         = "2"
	IssuedViaConfig = "config_agent_install_command"
	IssuedViaHosted = "hosted_agent_install_command"
)

type Decision struct {
	Admit          bool
	FirstBind      bool
	LegacyMigrate  bool
	RebindHostname bool
	BackfillID     bool
	BackfillHost   bool
}

// EvaluateActionRunner admits only a pre-bound, typed action credential whose
// tenant-independent host identity exactly matches the registering runner.
// Action credentials are never first-use rebound or legacy-migrated.
func EvaluateActionRunner(record *config.APITokenRecord, requestedID, requestedHost string) Decision {
	if record == nil || !record.HasScope(config.ScopeAgentExec) {
		return Decision{}
	}
	if strings.TrimSpace(record.Metadata[agenttokens.RuntimeRoleMetadataKey]) != agenttokens.CredentialKindActionRunner ||
		strings.TrimSpace(record.Metadata[agenttokens.ActionCapabilityMetadataKey]) != agenttokens.ActionCapabilityTypedV1 ||
		strings.TrimSpace(record.Metadata[agenttokens.ActionBindingVersionMetadataKey]) != agenttokens.ActionBindingVersion {
		return Decision{}
	}
	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])
	requestedID = strings.TrimSpace(requestedID)
	requestedHost = strings.TrimSpace(requestedHost)
	if boundID == "" || boundHost == "" || requestedID == "" || requestedHost == "" {
		return Decision{}
	}
	return Decision{Admit: boundID == requestedID && hostnamesMatch(boundHost, requestedHost)}
}

func Evaluate(record *config.APITokenRecord, requestedID, requestedHost string) Decision {
	if record == nil {
		return Decision{}
	}
	requestedID = strings.TrimSpace(requestedID)
	requestedHost = strings.TrimSpace(requestedHost)
	boundID := strings.TrimSpace(record.Metadata["bound_agent_id"])
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])

	if boundID == "" && boundHost == "" {
		if CanBindInstallToken(record, requestedID, requestedHost) {
			return Decision{Admit: true, FirstBind: true}
		}
		return Decision{}
	}
	if canBindAutoRegisteredInstallToken(record, requestedID, requestedHost) {
		return Decision{Admit: true, FirstBind: true}
	}
	if strings.TrimSpace(record.Metadata[VersionKey]) != Version &&
		boundHost != "" && hostnamesMatch(boundHost, requestedHost) {
		return Decision{Admit: true, LegacyMigrate: true}
	}

	idMatches := boundID == "" || boundID == requestedID
	hostMatches := boundHost == "" || hostnamesMatch(boundHost, requestedHost)
	rebindHostname := boundID != "" && boundID == requestedID && !hostMatches && requestedHost != ""
	if !idMatches || (!hostMatches && !rebindHostname) {
		return Decision{}
	}
	return Decision{
		Admit:          true,
		RebindHostname: rebindHostname,
		BackfillID:     boundID == "" && boundHost != "",
		BackfillHost:   boundHost == "" && boundID != "",
	}
}

func CanBindInstallToken(record *config.APITokenRecord, agentID, hostname string) bool {
	if record == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(hostname) == "" {
		return false
	}
	if strings.TrimSpace(record.Metadata["bound_agent_id"]) != "" || strings.TrimSpace(record.Metadata["bound_hostname"]) != "" {
		return false
	}
	if !supportedInstallType(record.Metadata["install_type"]) {
		return false
	}
	return supportedIssuer(record.Metadata["issued_via"])
}

func CanBindAutoRegisteredInstallToken(record *config.APITokenRecord, agentID, hostname string) bool {
	return canBindAutoRegisteredInstallToken(record, agentID, hostname)
}

func HostnamesMatch(bound, requested string) bool { return hostnamesMatch(bound, requested) }

func canBindAutoRegisteredInstallToken(record *config.APITokenRecord, agentID, hostname string) bool {
	if record == nil || strings.TrimSpace(agentID) == "" || strings.TrimSpace(hostname) == "" {
		return false
	}
	if strings.TrimSpace(record.Metadata["bound_agent_id"]) != "" || strings.TrimSpace(record.Metadata[VersionKey]) != "" {
		return false
	}
	boundHost := strings.TrimSpace(record.Metadata["bound_hostname"])
	if boundHost == "" || !hostnamesMatch(boundHost, strings.TrimSpace(hostname)) {
		return false
	}
	return supportedInstallType(record.Metadata["install_type"]) && supportedIssuer(record.Metadata["issued_via"])
}

func supportedInstallType(value string) bool {
	switch strings.TrimSpace(value) {
	case "pve", "pbs", "host":
		return true
	default:
		return false
	}
}

func supportedIssuer(value string) bool {
	switch strings.TrimSpace(value) {
	case IssuedViaConfig, IssuedViaHosted:
		return true
	default:
		return false
	}
}

func hostnamesMatch(bound, requested string) bool {
	return strings.EqualFold(bound, requested) || unifiedresources.HostnamesEquivalent(bound, requested)
}
