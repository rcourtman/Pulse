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

	// DeployIdentityKey records where a deploy runtime token's agent identity
	// comes from. Enrollment that leaves identity to the agent writes
	// DeployIdentityAgent; the one-time repair of a token enrollment bound to
	// an invented agent-<hostname> writes DeployIdentityRepaired. A token with
	// either value is never repaired again.
	DeployIdentityKey      = "deploy_identity_binding"
	DeployIdentityAgent    = "agent"
	DeployIdentityRepaired = "repaired"
)

type Decision struct {
	Admit          bool
	FirstBind      bool
	LegacyMigrate  bool
	RebindHostname bool
	BackfillID     bool
	BackfillHost   bool
	// RepairDeployIdentity marks a LegacyMigrate that moves a historical deploy
	// placeholder binding; the admission path must persist
	// DeployIdentityRepaired with the new identity so it cannot recur.
	RepairDeployIdentity bool
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
	// A historical deploy placeholder moves to a new identity only through the
	// marked one-time repair, whatever its binding version. It must never fall
	// through to the legacy branch below, which accepts equivalent hostnames
	// and leaves no marker, so a later hostname rebind could recreate the
	// placeholder shape and unlock another move.
	if requestedID != boundID && historicalDeployPlaceholder(record, boundID, boundHost) {
		if requestedID != "" && strings.EqualFold(boundHost, requestedHost) {
			return Decision{Admit: true, LegacyMigrate: true, RepairDeployIdentity: true}
		}
		return Decision{}
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

// historicalDeployPlaceholder reports a runtime token that deploy enrollment
// bound to the agent-<hostname> identity it used to invent. No agent keeps
// that identity past its first acknowledged report, so such a token may move
// once to the registering agent's real identity on exactly the same hostname.
// Tokens issued by current enrollment carry DeployIdentityAgent and a
// repaired token carries DeployIdentityRepaired, so neither qualifies, even if
// a later hostname rebind reproduces the agent-<hostname> shape.
func historicalDeployPlaceholder(record *config.APITokenRecord, boundID, boundHost string) bool {
	return strings.TrimSpace(record.Metadata["deploy_job_id"]) != "" &&
		strings.TrimSpace(record.Metadata[DeployIdentityKey]) == "" &&
		boundHost != "" && boundID == "agent-"+boundHost
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
