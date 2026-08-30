package auth

import (
	"fmt"
	"slices"
	"strings"
)

const (
	RuntimeRoleMetadataKey         = "runtime_role"
	RuntimeRoleMonitoringCollector = "monitoring-collector"
	RuntimeRoleActionRunner        = "action-runner"
	RuntimeRoleLegacyFullTrust     = "legacy-full-trust"
)

var monitoringCollectorScopeOrder = []string{
	ScopeAgentReport,
	ScopeAgentConfigRead,
	ScopeDockerReport,
	ScopeKubernetesReport,
}

// MonitoringCollectorScopes returns the complete allowed collector scope
// vocabulary. Report and config-read are mandatory; provider report scopes are
// optional because Proxmox-only collectors do not need them.
func MonitoringCollectorScopes() []string {
	return append([]string(nil), monitoringCollectorScopeOrder...)
}

// CanonicalizeRoleScopes reduces a role credential to its canonical authority.
// It never adds a scope. Missing mandatory scopes fail closed because silently
// adding them would be an authority increase and accepting the credential
// would violate the role contract.
func CanonicalizeRoleScopes(role string, scopes []string) ([]string, bool, error) {
	role = strings.TrimSpace(role)
	switch role {
	case RuntimeRoleMonitoringCollector:
		present := scopeSet(scopes)
		for _, required := range []string{ScopeAgentReport, ScopeAgentConfigRead} {
			if _, ok := present[required]; !ok {
				return nil, false, fmt.Errorf("monitoring collector credential requires %s", required)
			}
		}
		canonical := make([]string, 0, len(monitoringCollectorScopeOrder))
		for _, allowed := range monitoringCollectorScopeOrder {
			if _, ok := present[allowed]; ok {
				canonical = append(canonical, allowed)
			}
		}
		return canonical, !slices.Equal(scopes, canonical), nil

	case RuntimeRoleActionRunner:
		present := scopeSet(scopes)
		if _, ok := present[ScopeAgentExec]; !ok {
			return nil, false, fmt.Errorf("action runner credential requires %s", ScopeAgentExec)
		}
		canonical := []string{ScopeAgentExec}
		return canonical, !slices.Equal(scopes, canonical), nil

	case RuntimeRoleLegacyFullTrust:
		present := scopeSet(scopes)
		if _, ok := present[ScopeAgentExec]; !ok {
			return nil, false, fmt.Errorf("legacy full-trust credential requires %s", ScopeAgentExec)
		}
		return append([]string(nil), scopes...), false, nil

	case "":
		return append([]string(nil), scopes...), false, nil

	default:
		return nil, false, fmt.Errorf("unsupported agent credential role %q", role)
	}
}

// ValidateRoleScopes reports whether a role credential already has canonical,
// bounded authority. It is used at admission even though load-time migration
// normally repairs excess authority, providing defense against in-memory
// mutation and stale configuration snapshots.
func ValidateRoleScopes(role string, scopes []string) error {
	_, _, err := CanonicalizeRoleScopes(role, scopes)
	if err != nil {
		return err
	}
	if unexpected := UnexpectedScopes(role, scopes); len(unexpected) > 0 {
		return fmt.Errorf("%s credential has non-canonical scopes", strings.TrimSpace(role))
	}
	return nil
}

// UnexpectedScopes returns scopes that are outside the role's allowlist. It is
// diagnostics-only and never treats missing mandatory scopes as unexpected.
func UnexpectedScopes(role string, scopes []string) []string {
	var allowed map[string]struct{}
	switch strings.TrimSpace(role) {
	case RuntimeRoleMonitoringCollector:
		allowed = scopeSet(monitoringCollectorScopeOrder)
	case RuntimeRoleActionRunner:
		allowed = scopeSet([]string{ScopeAgentExec})
	default:
		return nil
	}
	unexpected := make([]string, 0)
	seen := make(map[string]struct{})
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if _, ok := allowed[scope]; ok {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		unexpected = append(unexpected, scope)
	}
	slices.Sort(unexpected)
	return unexpected
}

func scopeSet(scopes []string) map[string]struct{} {
	result := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		result[strings.TrimSpace(scope)] = struct{}{}
	}
	return result
}
