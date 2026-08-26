package alerts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

const (
	ExternalProbeUnavailableAlertType      = "external-probe-unavailable"
	ExternalProbeUnavailableIncidentCode   = "availability_probe_unavailable"
	ExternalProbeIncidentSource            = "external-probe"
	externalProbeUnavailableStateKey       = "external-probe-unavailable"
	externalProbeUnavailableTrackingPrefix = "external-probe:"
)

type ExternalProbeState string

const (
	// ExternalProbeStateGrace means no result has arrived yet, but the
	// assignment is still within its first-report window. Existing active
	// alerts are intentionally preserved across restart grace.
	ExternalProbeStateGrace ExternalProbeState = "grace"
	// ExternalProbeStateHealthy means at least one enabled assignment has a
	// fresh result, none are stale, and any active probe-specific alert can
	// resolve while newly assigned checks remain in grace.
	ExternalProbeStateHealthy ExternalProbeState = "healthy"
	// ExternalProbeStateUnavailable means one or more assigned checks have
	// stopped producing results after their grace window.
	ExternalProbeStateUnavailable ExternalProbeState = "unavailable"
	// ExternalProbeStateHostOffline means the agent heartbeat is offline. The
	// canonical host-offline lifecycle owns that incident, so the
	// probe-specific alert must not duplicate it.
	ExternalProbeStateHostOffline ExternalProbeState = "host-offline"
)

// ExternalProbeSnapshot is the alert-owned view of one agent's availability
// assignments. Target IDs are operational context for the self-hosted alert
// only; mobile push payloads remain generic.
type ExternalProbeSnapshot struct {
	AgentID        string
	Name           string
	State          ExternalProbeState
	TargetIDs      []string
	StaleTargetIDs []string
}

// SyncExternalProbes reconciles the complete set of licensed, enabled probe
// assignments. Alert identity is keyed only by agent ID, so adding, deleting,
// or reordering targets cannot churn the incident.
func (m *Manager) SyncExternalProbes(snapshots []ExternalProbeSnapshot) {
	if m == nil {
		return
	}

	seen := make(map[string]struct{}, len(snapshots))
	for _, snapshot := range snapshots {
		agentID := strings.TrimSpace(snapshot.AgentID)
		if agentID == "" {
			continue
		}
		snapshot.AgentID = agentID
		seen[agentID] = struct{}{}
		m.checkExternalProbe(snapshot)
	}

	retiredAgentIDs := make(map[string]struct{})
	m.mu.RLock()
	for _, alert := range m.activeAlerts {
		if alert == nil || alert.Type != ExternalProbeUnavailableAlertType {
			continue
		}
		agentID := alertMetadataString(alert, "hostId")
		if _, ok := seen[agentID]; !ok {
			retiredAgentIDs[agentID] = struct{}{}
		}
	}
	m.mu.RUnlock()
	for agentID := range retiredAgentIDs {
		m.clearExternalProbeAlert(agentID)
	}
}

func (m *Manager) checkExternalProbe(snapshot ExternalProbeSnapshot) {
	m.mu.RLock()
	enabled := m.config.Enabled
	disabled := m.config.DisableAllAgentsOffline
	thresholds := m.resolveHostThresholdsNoLock(snapshot.AgentID, "", "", "")
	m.mu.RUnlock()
	if !enabled || disabled || thresholds.Disabled || thresholds.DisableConnectivity {
		m.clearExternalProbeAlert(snapshot.AgentID)
		return
	}

	switch snapshot.State {
	case ExternalProbeStateHealthy, ExternalProbeStateHostOffline:
		m.clearExternalProbeAlert(snapshot.AgentID)
		return
	case ExternalProbeStateGrace:
		return
	case ExternalProbeStateUnavailable:
		// Continue below.
	default:
		return
	}

	resourceID := hostResourceID(snapshot.AgentID)
	alertID := canonicalDiscreteStateStateID(resourceID, externalProbeUnavailableStateKey)

	targetIDs := normalizedExternalProbeTargetIDs(snapshot.TargetIDs)
	staleTargetIDs := normalizedExternalProbeTargetIDs(snapshot.StaleTargetIDs)
	name := strings.TrimSpace(snapshot.Name)
	if name == "" {
		name = snapshot.AgentID
	}

	spec, err := buildCanonicalDiscreteStateSpec(
		resourceID,
		name,
		unifiedresources.ResourceTypeAgent,
		AlertLevelWarning,
		1,
		false,
		externalProbeUnavailableStateKey,
		[]string{string(ExternalProbeStateUnavailable)},
	)
	if err != nil {
		log.Warn().
			Err(err).
			Str("hostID", snapshot.AgentID).
			Msg("Skipping invalid external probe availability spec")
		return
	}

	message := fmt.Sprintf("External probe '%s' stopped reporting availability results", name)
	if len(staleTargetIDs) > 0 && len(staleTargetIDs) < len(targetIDs) {
		message = fmt.Sprintf(
			"External probe '%s' stopped reporting results for %d of %d assigned checks",
			name,
			len(staleTargetIDs),
			len(targetIDs),
		)
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec: spec,
		Evidence: alertspecs.AlertEvidence{
			ObservedAt: m.now(),
			DiscreteState: &alertspecs.DiscreteStateEvidence{
				StateKey: externalProbeUnavailableStateKey,
				Observed: string(ExternalProbeStateUnavailable),
			},
		},
		AlertID:      alertID,
		AlertType:    ExternalProbeUnavailableAlertType,
		ResourceID:   resourceID,
		ResourceName: name,
		Instance:     name,
		Message:      message,
		Metadata: map[string]interface{}{
			"resourceType":      string(unifiedresources.ResourceTypeAgent),
			"hostId":            snapshot.AgentID,
			"incidentCode":      ExternalProbeUnavailableIncidentCode,
			"incidentSource":    ExternalProbeIncidentSource,
			"incidentCategory":  string(unifiedresources.IncidentCategoryAvailability),
			"incidentNativeID":  snapshot.AgentID,
			"assignedTargetIds": targetIDs,
			"staleTargetIds":    staleTargetIDs,
			"assignedTargets":   len(targetIDs),
			"staleTargets":      len(staleTargetIDs),
		},
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     true,
		DispatchAsync: false,
	})
}

func (m *Manager) clearExternalProbeAlert(agentID string) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return
	}
	resourceID := hostResourceID(agentID)
	alertID := canonicalDiscreteStateStateID(resourceID, externalProbeUnavailableStateKey)

	m.mu.Lock()
	// Clear any pending core run alongside the alert.
	m.core.ApplyDiscrete(reducer.DiscreteSignal{ResourceID: resourceID, Key: canonicalDiscreteStateSpecID(resourceID, externalProbeUnavailableStateKey), Matched: false, ObservedAt: m.policyNow()}, reducer.DiscreteRule{})
	exists := m.hasActiveAlertNoLock(alertID)
	m.mu.Unlock()
	if exists {
		m.clearAlert(alertID)
	}
}

func normalizedExternalProbeTargetIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
