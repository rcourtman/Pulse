package alerts

import "reflect"

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
//	5. a guest's pulse-relaxed tag, which lifts its CPU/memory/disk
//	   triggers to a 95/92/95 floor

// alertPolicyQuery identifies one resource asking for its effective
// alert policy.
type alertPolicyQuery struct {
	// TypeKey is the canonical resource type key ("vm", "node", "agent",
	// "storage", "pbs", "pmg", "docker-host", "k8s-node", ...).
	TypeKey    string
	ResourceID string
	// Guest carries the live guest model when the query is for a guest
	// kind: filter-scoped custom rules, the clustered override identity and
	// the pulse-relaxed tag need it. Nil is valid — filter rules then never
	// match, override lookup falls back to the raw resource ID, and no tag
	// relaxes the thresholds.
	Guest any
	// StorageAliases are the storage resource's alias IDs, honored by the
	// storage override lookup.
	StorageAliases []string
	// DiskType is a physical disk's type ("nvme", "sas", "sata"), which
	// picks its disk temperature threshold. DiskTypeUnknown marks a disk
	// whose type was never recorded, such as an alert persisted before it
	// carried one; it is judged against the lowest per-type trigger.
	DiskType        string
	DiskTypeUnknown bool
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
	// custom rules, then the per-resource override, then a guest's
	// pulse-relaxed floor.
	Thresholds ThresholdConfig
	// Plausible higher-priority rules whose current filter input is missing.
	// Each is already folded through resource overrides and pulse-relaxed.
	// Evaluation may use a signal only when every possible policy agrees.
	guestAlternatives []ThresholdConfig
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
	thresholds, _ := m.customRuleThresholdCandidatesNoLock(base, guest)
	return thresholds
}

func (m *Manager) customRuleThresholdCandidatesNoLock(base ThresholdConfig, guest any) (ThresholdConfig, []ThresholdConfig) {
	if guest == nil {
		return base, nil
	}
	applicable := -1
	highestPriority := -1
	unknown := make([]int, 0)
	for i := range m.config.CustomRules {
		rule := &m.config.CustomRules[i]
		if !rule.Enabled || rule.Priority < 0 {
			continue
		}
		switch m.evaluateFilterStackResult(guest, rule.FilterConditions) {
		case guestFilterMatch:
			if rule.Priority > highestPriority {
				applicable = i
				highestPriority = rule.Priority
			}
		case guestFilterUnknown:
			unknown = append(unknown, i)
		}
	}
	selected := base
	if applicable >= 0 {
		selected = m.applyThresholdOverride(base, m.config.CustomRules[applicable].Thresholds)
	}
	var alternatives []ThresholdConfig
	for _, i := range unknown {
		rule := m.config.CustomRules[i]
		// Preserve the existing first-in-config tie break. Lower priority or
		// later tied unknown rules cannot displace an affirmative winner.
		if rule.Priority > highestPriority || (rule.Priority == highestPriority && i < applicable) {
			alternatives = append(alternatives, m.applyThresholdOverride(base, rule.Thresholds))
		}
	}
	return selected, alternatives
}

// guestSignalPolicyKnownNoLock compares the final policy, not just the rule's
// fields: an explicit guest/filesystem override can remove the ambiguity. It
// is deliberately stateless; a cached old match must not authorise a new one
// after a config change, restart, migration or telemetry gap.
func (m *Manager) guestSignalPolicyKnownNoLock(policy EffectiveAlertPolicy, signal string) bool {
	for _, alternative := range policy.guestAlternatives {
		if policy.Thresholds.Disabled != alternative.Disabled {
			return false
		}
		if policy.Thresholds.Disabled {
			continue
		}
		switch signal {
		case "disabled":
		case "powered-off":
			if policy.Thresholds.DisableConnectivity != alternative.DisableConnectivity ||
				NormalizePoweredOffSeverity(policy.Thresholds.PoweredOffSeverity) != NormalizePoweredOffSeverity(alternative.PoweredOffSeverity) {
				return false
			}
		case "backup-age":
			if !m.config.BackupDefaults.Enabled {
				continue
			}
			if !reflect.DeepEqual(m.resolvedBackupAlertConfigNoLock(policy.Thresholds), m.resolvedBackupAlertConfigNoLock(alternative)) {
				return false
			}
		case "snapshot-age":
			if !m.config.SnapshotDefaults.Enabled {
				continue
			}
			if !reflect.DeepEqual(m.resolvedSnapshotAlertConfigNoLock(policy.Thresholds), m.resolvedSnapshotAlertConfigNoLock(alternative)) {
				return false
			}
		default:
			if !reflect.DeepEqual(getThresholdForMetric(policy.Thresholds, signal), getThresholdForMetric(alternative, signal)) {
				return false
			}
		}
	}
	return true
}

func (m *Manager) guestThresholdPolicyNoLock(guest any, guestID string) EffectiveAlertPolicy {
	return m.effectiveAlertPolicyNoLock(alertPolicyQuery{TypeKey: "vm", ResourceID: guestID, Guest: guest})
}

// effectiveAlertPolicyNoLock resolves the alert policy for one resource.
// Callers must hold m.mu.
func (m *Manager) effectiveAlertPolicyNoLock(q alertPolicyQuery) EffectiveAlertPolicy {
	policy := EffectiveAlertPolicy{}
	policy.AllDisabled, policy.OfflineDisabled = m.alertPolicyTypeSwitchesNoLock(q.TypeKey)

	thresholds := m.defaultThresholdsForResourceType(q.TypeKey)
	if q.TypeKey == "truenas-disk" {
		thresholds.Temperature = m.trueNASDiskTemperatureDefaultNoLock(q.DiskType, q.DiskTypeUnknown)
	}
	switch {
	case isGuestThresholdResourceType(q.TypeKey):
		var alternatives []ThresholdConfig
		thresholds, alternatives = m.customRuleThresholdCandidatesNoLock(thresholds, q.Guest)
		// Polling and config-save reevaluation both resolve through here,
		// so a relaxed guest's alert is judged by one threshold. Applied
		// only by CheckGuest, a save re-judged it by the unrelaxed one.
		finish := func(candidate ThresholdConfig) ThresholdConfig {
			candidate = m.resolveGuestThresholdOverride(candidate, q.Guest, q.ResourceID)
			if guest, ok := extractGuestSnapshot(q.Guest); ok && parsePulseTags(guest.Tags).Relaxed {
				candidate = applyRelaxedGuestThresholds(candidate)
			}
			return candidate
		}
		thresholds = finish(thresholds)
		for _, alternative := range alternatives {
			policy.guestAlternatives = append(policy.guestAlternatives, finish(alternative))
		}
	case q.TypeKey == "storage":
		thresholds = m.resolveStorageThresholdOverride(thresholds, q.ResourceID, q.StorageAliases)
	default:
		thresholds = m.resolveThresholdOverride(thresholds, q.ResourceID)
	}
	policy.Thresholds = thresholds
	return policy
}
