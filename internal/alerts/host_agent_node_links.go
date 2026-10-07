package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// hostAgentNodeLink is a reporting host agent's link to a Proxmox node and the
// usage metrics the agent covers for that machine. The link is the monitor's
// identity decision (automatic or operator-set), so an agent whose hostname
// merely matches a node name in another instance, or one an operator unlinked,
// never makes a node release its alerts, and an agent reporting an FQDN still
// dedups its node.
type hostAgentNodeLink struct {
	agentID   string
	agentName string
	nodeID    string
	cpu       bool
	memory    bool
	disk      bool
	// summaryDiskResourceID is the agent disk resource behind the node's disk
	// metric, kept so a config change can revoke disk ownership at once.
	summaryDiskResourceID string
}

func (l hostAgentNodeLink) ownsAny() bool {
	return l.cpu || l.memory || l.disk
}

// beginHostAgentReport numbers a CheckHost run for one agent. Link changes are
// applied in report order, so a slower, older report can neither re-register
// nor remove ownership after a newer report or after the agent was
// unregistered.
func (m *Manager) beginHostAgentReport(hostID string) uint64 {
	hostID = strings.TrimSpace(hostID)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hostAgentReportSeq[hostID]++
	return m.hostAgentReportSeq[hostID]
}

// applyHostAgentNodeLink records the outcome of a report: the link and the
// usage metrics the agent covers, or no ownership when the link has no node or
// covers nothing. A stale report is ignored, and the link is checked against
// the current config so a report evaluated before a config change cannot
// restore ownership that change revoked.
func (m *Manager) applyHostAgentNodeLink(link hostAgentNodeLink, reportSeq uint64) {
	link.agentID = strings.TrimSpace(link.agentID)
	link.nodeID = strings.TrimSpace(link.nodeID)
	if link.agentID == "" {
		return
	}
	m.mu.Lock()
	if reportSeq <= m.hostAgentLinkApplied[link.agentID] {
		m.mu.Unlock()
		log.Debug().
			Str("hostID", link.agentID).
			Msg("Skipped stale host agent node link update")
		return
	}
	m.hostAgentLinkApplied[link.agentID] = reportSeq
	if link.nodeID != "" {
		link = m.revokeUnsupportedOwnershipNoLock(link)
	}
	previous, existed := m.hostAgentNodeLinks[link.agentID]
	if link.nodeID == "" || !link.ownsAny() {
		delete(m.hostAgentNodeLinks, link.agentID)
	} else {
		m.hostAgentNodeLinks[link.agentID] = link
	}
	m.mu.Unlock()

	switch {
	case link.nodeID != "" && link.ownsAny() && (!existed || previous != link):
		log.Debug().
			Str("hostID", link.agentID).
			Str("linkedNodeID", link.nodeID).
			Bool("cpu", link.cpu).
			Bool("memory", link.memory).
			Bool("disk", link.disk).
			Msg("Updated host agent node link for deduplication")
	case (link.nodeID == "" || !link.ownsAny()) && existed:
		log.Debug().
			Str("hostID", link.agentID).
			Str("linkedNodeID", previous.nodeID).
			Msg("Removed host agent node link from deduplication")
	}
}

// unregisterHostAgentNodeLink removes a host agent from deduplication tracking,
// so its linked node evaluates its own usage alerts again, and marks every
// report already in flight for that agent as stale.
func (m *Manager) unregisterHostAgentNodeLink(hostID string) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return
	}
	m.mu.Lock()
	m.hostAgentLinkApplied[hostID] = m.hostAgentReportSeq[hostID]
	link, existed := m.hostAgentNodeLinks[hostID]
	delete(m.hostAgentNodeLinks, hostID)
	m.mu.Unlock()

	if existed {
		log.Debug().
			Str("hostID", hostID).
			Str("linkedNodeID", link.nodeID).
			Msg("Removed host agent node link from deduplication")
	}
}

// reconcileHostAgentNodeLinksNoLock applies a config change to the recorded
// links straight away instead of waiting for each agent's next report. It only
// revokes ownership the new config no longer supports. Caller holds m.mu.
func (m *Manager) reconcileHostAgentNodeLinksNoLock() {
	for agentID, link := range m.hostAgentNodeLinks {
		updated := m.revokeUnsupportedOwnershipNoLock(link)
		if updated.ownsAny() {
			m.hostAgentNodeLinks[agentID] = updated
		} else {
			delete(m.hostAgentNodeLinks, agentID)
		}
	}
}

// revokeUnsupportedOwnershipNoLock drops the usage metrics the current config
// does not let the agent evaluate. Caller holds m.mu.
func (m *Manager) revokeUnsupportedOwnershipNoLock(link hostAgentNodeLink) hostAgentNodeLink {
	if disableAllAgents, _ := m.alertPolicyTypeSwitchesNoLock("agent"); !m.config.Enabled || disableAllAgents {
		link.cpu, link.memory, link.disk = false, false, false
		return link
	}
	thresholds := m.resolveHostThresholdsNoLock(link.agentID, link.nodeID, "", "")
	if thresholds.Disabled {
		link.cpu, link.memory, link.disk = false, false, false
		return link
	}
	link.cpu = link.cpu && liveHysteresisThreshold(thresholds.CPU)
	link.memory = link.memory && liveHysteresisThreshold(thresholds.Memory)
	link.disk = link.disk && m.hostDiskUsageEnabledNoLock(link.summaryDiskResourceID, thresholds)
	return link
}

// hostDiskUsageEnabledNoLock reports whether the current direct filesystem
// override and host policy still allow summary disk evaluation. Per-type
// defaults remain live after normal config normalization; the next report
// settles their exact trigger. Caller holds m.mu.
func (m *Manager) hostDiskUsageEnabledNoLock(diskResourceID string, thresholds ThresholdConfig) bool {
	if override, ok := m.config.Overrides[diskResourceID]; ok && diskResourceID != "" {
		if override.Disabled {
			return false
		}
		if override.Disk != nil {
			return liveHysteresisThreshold(override.Disk)
		}
	}
	return liveHysteresisThreshold(thresholds.Disk)
}

func liveHysteresisThreshold(threshold *HysteresisThreshold) bool {
	return threshold != nil && threshold.Trigger > 0
}

// hostAgentMetricOpen reports whether an agent resource still holds an alert
// or a pending or firing run for a metric. An agent that cannot evaluate a
// metric this cycle keeps ownership while it has one, so the node does not
// open a duplicate during a missing reading.
func (m *Manager) hostAgentMetricOpen(resourceID, metric string) bool {
	if strings.TrimSpace(resourceID) == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.core.Incident(resourceID, canonicalMetricSpecID(resourceID, metric)); ok {
		return true
	}
	return m.hasActiveAlertNoLock(canonicalMetricStateID(resourceID, metric))
}

// Capture ownership and attribution in one read. Different agents can cover
// different metrics; a handover must name a real owner of the released metric.
type hostAgentUsageOwners struct {
	cpu, memory, disk hostAgentNodeLink
}

func (o hostAgentUsageOwners) forMetric(metric string) hostAgentNodeLink {
	switch metric {
	case "cpu":
		return o.cpu
	case "memory":
		return o.memory
	case "disk":
		return o.disk
	default:
		return hostAgentNodeLink{}
	}
}

func (m *Manager) linkedHostAgentsForNode(nodeID string) hostAgentUsageOwners {
	nodeID = strings.TrimSpace(nodeID)
	var owners hostAgentUsageOwners
	if nodeID == "" {
		return owners
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, link := range m.hostAgentNodeLinks {
		if link.nodeID != nodeID {
			continue
		}
		if link.cpu && (owners.cpu.agentID == "" || link.agentID < owners.cpu.agentID) {
			owners.cpu = link
		}
		if link.memory && (owners.memory.agentID == "" || link.agentID < owners.memory.agentID) {
			owners.memory = link
		}
		if link.disk && (owners.disk.agentID == "" || link.agentID < owners.disk.agentID) {
			owners.disk = link
		}
	}
	return owners
}

// linkedHostAgentForNode returns the host agent coverage for the given Proxmox
// node. A usage metric counts as owned when any link to the node covers it, so
// a second linked agent covering a metric the first does not still keeps the
// node from alerting twice. The agent identity reported is the lowest agent ID
// among the linked agents, so the choice is stable. Coverage comes only from
// links that agent reports keep current; stale evidence such as an old alert's
// metadata never silences a node.
func (m *Manager) linkedHostAgentForNode(nodeID string) (hostAgentNodeLink, bool) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return hostAgentNodeLink{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var found hostAgentNodeLink
	ok := false
	for _, link := range m.hostAgentNodeLinks {
		if link.nodeID != nodeID {
			continue
		}
		if !ok || link.agentID < found.agentID {
			found.agentID = link.agentID
			found.agentName = link.agentName
		}
		found.nodeID = nodeID
		found.cpu = found.cpu || link.cpu
		found.memory = found.memory || link.memory
		found.disk = found.disk || link.disk
		ok = true
	}
	return found, ok
}

// hasHostAgentForNode reports whether a host agent covers at least one of the
// given Proxmox node's usage metrics.
func (m *Manager) hasHostAgentForNode(nodeID string) bool {
	_, ok := m.linkedHostAgentForNode(nodeID)
	return ok
}

// releaseHostUsageMetric stops evaluating an agent usage metric the way a
// disabled threshold does, so a pending run is dropped along with any open
// alert and its explicit intent state.
func (m *Manager) releaseHostUsageMetric(resourceID, resourceName, nodeName, instanceName string, specType unifiedresources.ResourceType, resourceType, metric string) {
	spec, err := buildCanonicalMetricSpec(resourceID, resourceName, specType, metric, nil)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", resourceID).
			Str("metric", metric).
			Msg("Skipping invalid canonical host metric spec; clearing the alert only")
		m.clearAlert(canonicalMetricStateID(resourceID, metric))
		return
	}
	m.releaseCanonicalMetricAlert(spec, resourceName, nodeName, instanceName, resourceType, 0, nil)
}
