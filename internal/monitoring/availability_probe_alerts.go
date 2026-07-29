package monitoring

import (
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

type availabilityProbeAlertAccumulator struct {
	targetIDs      []string
	staleTargetIDs []string
	hasFreshResult bool
}

// syncAvailabilityProbeAlerts projects target-level reporting state into one
// stable alert lifecycle per agent. Host heartbeat loss remains owned by the
// existing host-offline alert; this lifecycle covers a never-started probe or
// availability results that stall while the host path is otherwise alive.
func (m *Monitor) syncAvailabilityProbeAlerts(
	targets []config.AvailabilityTarget,
	statuses map[string]AvailabilityProbeStatus,
	now time.Time,
) {
	if m == nil || m.alertManager == nil {
		return
	}

	byAgent := make(map[string]*availabilityProbeAlertAccumulator)
	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		agentID := m.effectiveProbeAgentID(target)
		if agentID == "" {
			continue
		}
		accumulator := byAgent[agentID]
		if accumulator == nil {
			accumulator = &availabilityProbeAlertAccumulator{}
			byAgent[agentID] = accumulator
		}
		accumulator.targetIDs = append(accumulator.targetIDs, target.ID)

		status := statuses[target.ID]
		switch {
		case availabilityProbeStatusIsStale(status):
			accumulator.staleTargetIDs = append(accumulator.staleTargetIDs, target.ID)
		case !status.LastChecked.IsZero() && strings.TrimSpace(status.ProbeAgentID) == agentID:
			accumulator.hasFreshResult = true
		}
	}

	agentIDs := make([]string, 0, len(byAgent))
	for agentID := range byAgent {
		agentIDs = append(agentIDs, agentID)
	}
	sort.Strings(agentIDs)

	snapshots := make([]alerts.ExternalProbeSnapshot, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		accumulator := byAgent[agentID]
		name, hostKnown, hostHealthy := m.availabilityProbeHostHealth(agentID, now)
		state := alerts.ExternalProbeStateGrace
		switch {
		case hostKnown && !hostHealthy:
			state = alerts.ExternalProbeStateHostOffline
		case len(accumulator.staleTargetIDs) > 0:
			state = alerts.ExternalProbeStateUnavailable
		case accumulator.hasFreshResult:
			state = alerts.ExternalProbeStateHealthy
		}
		snapshots = append(snapshots, alerts.ExternalProbeSnapshot{
			AgentID:        agentID,
			Name:           name,
			State:          state,
			TargetIDs:      accumulator.targetIDs,
			StaleTargetIDs: accumulator.staleTargetIDs,
		})
	}
	m.alertManager.SyncExternalProbes(snapshots)
}

func (m *Monitor) availabilityProbeHostHealth(agentID string, now time.Time) (string, bool, bool) {
	agentID = strings.TrimSpace(agentID)
	if m == nil || agentID == "" {
		return agentID, false, false
	}
	if m.state != nil {
		for _, host := range m.state.GetHosts() {
			if strings.TrimSpace(host.ID) != agentID {
				continue
			}
			return availabilityProbeHostDisplayName(host),
				true,
				!host.LastSeen.IsZero() && now.Sub(host.LastSeen) <= hostAgentHealthWindow(host.IntervalSeconds)
		}
	}
	for _, entry := range m.recentStandaloneHostContinuityEntries() {
		if strings.TrimSpace(entry.HostID) != agentID {
			continue
		}
		host := hostFromContinuityEntry(entry)
		return availabilityProbeHostDisplayName(host),
			true,
			!host.LastSeen.IsZero() && now.Sub(host.LastSeen) <= hostAgentHealthWindow(host.IntervalSeconds)
	}
	return agentID, false, false
}

func availabilityProbeHostDisplayName(host models.Host) string {
	if name := strings.TrimSpace(host.DisplayName); name != "" {
		return name
	}
	if name := strings.TrimSpace(host.Hostname); name != "" {
		return name
	}
	if id := strings.TrimSpace(host.ID); id != "" {
		return id
	}
	return "External Probe"
}

// HasExternalProbeAssignments allows notification routing to recognize the
// canonical host-offline alert for an agent that currently owns enabled Pro
// availability checks.
func (m *Monitor) HasExternalProbeAssignments(agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if m == nil || agentID == "" {
		return false
	}
	for _, target := range m.availabilityTargets() {
		if target.Enabled && m.effectiveProbeAgentID(target) == agentID {
			return true
		}
	}
	return false
}
