package alerts

// Alert policy resolution: the declarative rule model scoped by
// docs/ALERT_ENGINE_EVOLUTION.md, Phase 3. The persisted AlertConfig keeps
// its shape — per-type default blocks, per-resource overrides, custom
// rules, DisableAll* switches — and this file is the translator: one
// ordered fold from that config to the effective policy for one resource.
//
// The point is a single resolution surface. Before this, every check path
// read its own DisableAll* boolean and picked its own override lookup, and
// the scattered reads drifted (#1738: overrides saved under registry IDs
// silently never applying on legacy-ID paths; connection.go hand-rolling
// its own type→switch mapping). Callers now ask one question — "what is
// the effective alert policy for this resource?" — and the tiers are:
//
//	1. the type's default threshold block
//	2. the type's DisableAll switches (all alerts / offline family)
//	3. custom rules (guest kinds, live-filter scoped, priority order)
//	4. the per-resource override, through the identity-aware lookup for
//	   the kind (guest stable keys, storage aliases, canonical registry
//	   translation)

// alertPolicyQuery identifies one resource asking for its effective
// alert policy.
type alertPolicyQuery struct {
	// TypeKey is the canonical resource type key ("vm", "node", "agent",
	// "storage", "pbs", "pmg", "docker-host", "k8s-node", ...).
	TypeKey    string
	ResourceID string
	// Guest carries the live guest model when the query is for a guest
	// kind: filter-scoped custom rules and the clustered override
	// identity need it. Nil is valid — filter rules then never match and
	// override lookup falls back to the raw resource ID.
	Guest any
	// StorageAliases are the storage resource's alias IDs, honored by the
	// storage override lookup.
	StorageAliases []string
}

// EffectiveAlertPolicy is the resolved policy for one resource.
type EffectiveAlertPolicy struct {
	// AllDisabled reports the type's every-alert switch
	// (DisableAllNodes, DisableAllGuests, ...).
	AllDisabled bool
	// OfflineDisabled reports the type's connectivity/powered-state
	// family switch (DisableAllNodesOffline, ...). Per-resource
	// connectivity opt-outs are Thresholds.DisableConnectivity.
	OfflineDisabled bool
	// Thresholds is the folded threshold block: type defaults, then
	// custom rules, then the per-resource override.
	Thresholds ThresholdConfig
}

// alertPolicyTypeSwitches maps a resource type key to its global disable
// switches. This is the one place the DisableAll* booleans are read on
// behalf of evaluation paths.
func (m *Manager) alertPolicyTypeSwitchesNoLock(typeKey string) (allDisabled, offlineDisabled bool) {
	switch typeKey {
	case "vm", "system-container":
		return m.config.DisableAllGuests, m.config.DisableAllGuestsOffline
	case "app-container":
		return m.config.DisableAllDockerContainers, false
	case "docker-service":
		return m.config.DisableAllDockerServices, false
	case "docker-host":
		return m.config.DisableAllDockerHosts, m.config.DisableAllDockerHostsOffline
	case "node":
		return m.config.DisableAllNodes, m.config.DisableAllNodesOffline
	case "agent":
		return m.config.DisableAllAgents, m.config.DisableAllAgentsOffline
	case "storage":
		return m.config.DisableAllStorage, false
	case "pbs":
		return m.config.DisableAllPBS, m.config.DisableAllPBSOffline
	case "pmg":
		return m.config.DisableAllPMG, m.config.DisableAllPMGOffline
	default:
		if isUnifiedModernPlatformAlertType(typeKey) {
			return m.unifiedPlatformAlertsDisabledNoLock(typeKey), false
		}
		return false, false
	}
}

// customRuleThresholdsNoLock folds the highest-priority matching enabled
// custom rule onto base. Custom rules are guest-scoped: they match on
// live guest state, so a query without a guest never matches one.
func (m *Manager) customRuleThresholdsNoLock(base ThresholdConfig, guest any) ThresholdConfig {
	if guest == nil {
		return base
	}
	var applicable *CustomAlertRule
	highestPriority := -1
	for i := range m.config.CustomRules {
		rule := &m.config.CustomRules[i]
		if !rule.Enabled {
			continue
		}
		if m.evaluateFilterStack(guest, rule.FilterConditions) {
			if rule.Priority > highestPriority {
				applicable = rule
				highestPriority = rule.Priority
			}
		}
	}
	if applicable == nil {
		return base
	}
	return m.applyThresholdOverride(base, applicable.Thresholds)
}

// effectiveAlertPolicyNoLock resolves the alert policy for one resource.
// Callers must hold m.mu.
func (m *Manager) effectiveAlertPolicyNoLock(q alertPolicyQuery) EffectiveAlertPolicy {
	policy := EffectiveAlertPolicy{}
	policy.AllDisabled, policy.OfflineDisabled = m.alertPolicyTypeSwitchesNoLock(q.TypeKey)

	thresholds := m.defaultThresholdsForResourceType(q.TypeKey)
	switch {
	case isGuestThresholdResourceType(q.TypeKey):
		thresholds = m.customRuleThresholdsNoLock(thresholds, q.Guest)
		thresholds = m.resolveGuestThresholdOverride(thresholds, q.Guest, q.ResourceID)
	case q.TypeKey == "storage":
		thresholds = m.resolveStorageThresholdOverride(thresholds, q.ResourceID, q.StorageAliases)
	default:
		thresholds = m.resolveThresholdOverride(thresholds, q.ResourceID)
	}
	policy.Thresholds = thresholds
	return policy
}
