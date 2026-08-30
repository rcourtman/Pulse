package monitoring

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentupdate"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agentbinding"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	"github.com/rcourtman/pulse-go-rewrite/internal/remoteconfig"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	unraidstatus "github.com/rcourtman/pulse-go-rewrite/internal/unraid"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const hostContinuityRetention = 72 * time.Hour

const maxRetiredHostReportStreams = 8

const (
	maxHostZFSDatasetsPerPool  = 128
	maxHostZFSDatasetNameBytes = 512
	maxHostZFSMountpointBytes  = 1024
)

func normalizeAgentMemory(total, used, free, cache int64, usage float64, swapTotal, swapUsed int64) models.Memory {
	if total <= 0 {
		return models.Memory{}
	}
	hasReportedUsed := used > 0

	unavailable := func() models.Memory {
		memory := models.UnavailableMemory(total)
		memory.SwapTotal = max(0, swapTotal)
		memory.SwapUsed = max(0, swapUsed)
		return memory
	}

	usage = safeFloat(usage)
	if used < 0 || free < 0 || cache < 0 || used > total || usage < 0 || usage > 100 {
		return unavailable()
	}

	if used == 0 {
		switch {
		case usage > 0:
			used = int64(float64(total) * usage / 100)
		case cache > 0 && cache <= total && free <= total-cache:
			used = total - free - cache
		case free == total:
			// A completely idle host is a valid zero-usage measurement.
		default:
			return unavailable()
		}
	}

	if cache > total-used {
		cache = total - used
	}
	if free > total-used-cache {
		free = total - used - cache
	}
	// Byte counters are canonical when the agent reported them. Recompute the
	// percentage so a stale percentage cannot diverge display, history, and
	// threshold evaluation. Preserve a percentage-only report to avoid losing
	// precision when deriving its byte approximation.
	if hasReportedUsed || usage == 0 {
		usage = safePercentage(float64(used), float64(total))
	}

	return models.Memory{
		Total:     total,
		Used:      used,
		Free:      free,
		Cache:     cache,
		Usage:     usage,
		SwapTotal: max(0, swapTotal),
		SwapUsed:  max(0, swapUsed),
	}
}

type hostReportOrder struct {
	ObservedAt       time.Time
	LastReceivedAt   time.Time
	StreamID         string
	Sequence         uint64
	RetiredStreamIDs []string
}

type hostReportApplyLock struct {
	mu   sync.Mutex
	refs int
}

func hostReportOrderFromContinuity(entry config.HostContinuityEntry) hostReportOrder {
	lastReceivedAt := entry.ReportLastReceivedAt.UTC()
	if lastReceivedAt.IsZero() {
		lastReceivedAt = entry.LastSeen.UTC()
	}
	return hostReportOrder{
		ObservedAt:       entry.ReportObservedAt.UTC(),
		LastReceivedAt:   lastReceivedAt,
		StreamID:         strings.TrimSpace(entry.ReportStreamID),
		Sequence:         entry.ReportSequence,
		RetiredStreamIDs: append([]string(nil), entry.RetiredReportStreamIDs...),
	}
}

func containsHostReportStream(streams []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, stream := range streams {
		if strings.TrimSpace(stream) == target {
			return true
		}
	}
	return false
}

func retireHostReportStream(streams []string, streamID string) []string {
	streamID = strings.TrimSpace(streamID)
	if streamID == "" || containsHostReportStream(streams, streamID) {
		return streams
	}
	streams = append(streams, streamID)
	if len(streams) > maxRetiredHostReportStreams {
		streams = append([]string(nil), streams[len(streams)-maxRetiredHostReportStreams:]...)
	}
	return streams
}

func legacyHostReportReorderWindow(intervalSeconds int) time.Duration {
	window := time.Duration(intervalSeconds) * time.Second / 2
	if window < 2*time.Second {
		return 2 * time.Second
	}
	if window > 15*time.Second {
		return 15 * time.Second
	}
	return window
}

// lockHostReportApplication serializes the complete state transition for one
// host while allowing different hosts to ingest concurrently. Reserving a
// sequence number without holding this lock through the subsequent state write
// would still allow two HTTP handlers to commit reports in reverse order.
func (m *Monitor) lockHostReportApplication(hostID string) func() {
	hostID = strings.TrimSpace(hostID)

	m.hostReportApplyLocksMu.Lock()
	if m.hostReportApplyLocks == nil {
		m.hostReportApplyLocks = make(map[string]*hostReportApplyLock)
	}
	applyLock := m.hostReportApplyLocks[hostID]
	if applyLock == nil {
		applyLock = &hostReportApplyLock{}
		m.hostReportApplyLocks[hostID] = applyLock
	}
	applyLock.refs++
	m.hostReportApplyLocksMu.Unlock()

	applyLock.mu.Lock()
	return func() {
		applyLock.mu.Unlock()
		m.hostReportApplyLocksMu.Lock()
		applyLock.refs--
		if applyLock.refs == 0 && m.hostReportApplyLocks[hostID] == applyLock {
			delete(m.hostReportApplyLocks, hostID)
		}
		m.hostReportApplyLocksMu.Unlock()
	}
}

// reserveHostReportOrder separates source-authored ordering from transport
// activity. Receipt time always records authenticated contact, while only a
// non-retired, increasing report stream may replace telemetry and generate
// metrics, history, or alerts.
func (m *Monitor) reserveHostReportOrder(hostID string, report agentshost.Report, receivedAt time.Time) (hostReportOrder, bool) {
	hostID = strings.TrimSpace(hostID)
	authoredAt := report.Timestamp.UTC()
	if authoredAt.IsZero() {
		authoredAt = receivedAt.UTC()
	}

	m.hostReportOrderMu.Lock()
	defer m.hostReportOrderMu.Unlock()

	if m.hostReportOrders == nil {
		m.hostReportOrders = make(map[string]hostReportOrder)
	}
	order, exists := m.hostReportOrders[hostID]
	if !exists && m.hostContinuityStore != nil {
		if entry, ok := m.hostContinuityStore.Get(hostID); ok {
			order = hostReportOrderFromContinuity(entry)
		}
	}

	streamID, sequence, sequenced := agentshost.ParseReportSequenceID(report.SequenceID)
	accepted := true
	switch {
	case sequenced && containsHostReportStream(order.RetiredStreamIDs, streamID):
		accepted = false
	case sequenced && order.StreamID == streamID && sequence <= order.Sequence:
		accepted = false
	case sequenced && order.StreamID == streamID:
		order.Sequence = sequence
		order.ObservedAt = authoredAt
	case sequenced:
		order.RetiredStreamIDs = retireHostReportStream(order.RetiredStreamIDs, order.StreamID)
		order.StreamID = streamID
		order.Sequence = sequence
		order.ObservedAt = authoredAt
	default:
		// Older agents have no process stream. Reject backwards timestamps only
		// when they arrive in the reconnect burst; after a normal report
		// interval, accept a clock epoch reset so a corrected host clock cannot
		// freeze telemetry indefinitely.
		if !order.ObservedAt.IsZero() &&
			authoredAt.Before(order.ObservedAt) &&
			!order.LastReceivedAt.IsZero() &&
			receivedAt.Sub(order.LastReceivedAt) <= legacyHostReportReorderWindow(report.Agent.IntervalSeconds) {
			accepted = false
		} else {
			order.RetiredStreamIDs = retireHostReportStream(order.RetiredStreamIDs, order.StreamID)
			order.StreamID = ""
			order.Sequence = 0
			order.ObservedAt = authoredAt
		}
	}

	if receivedAt.After(order.LastReceivedAt) {
		order.LastReceivedAt = receivedAt.UTC()
	}
	m.hostReportOrders[hostID] = order
	return order, accepted
}

func (m *Monitor) RemoveDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}
	hostID = m.canonicalDockerHostID(hostID)

	host, removed := m.state.RemoveDockerHost(hostID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("dockerHostID", hostID).Msg("Docker host not present in state during removal; proceeding to clear alerts")
		}
		host = models.DockerHost{
			ID:          hostID,
			Hostname:    hostID,
			DisplayName: hostID,
		}
	}

	// Revoke the API token associated with this Docker host
	if host.TokenID != "" {
		tokenRemoved, err := m.revokeAPIToken(host.TokenID)
		if err != nil {
			log.Warn().Err(err).Str("tokenID", host.TokenID).Msg("API token revocation rolled back after Docker host removal")
		} else if tokenRemoved != nil {
			log.Info().Str("tokenID", host.TokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed Docker host")
		}
	}

	// Track removal to prevent resurrection from cached reports.
	m.finalizeDockerHostRemoval(host, time.Now().UTC())

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Bool("removed", removed).
		Msg("Docker host removed and alerts cleared")

	return host, nil
}

func (m *Monitor) finalizeDockerHostRemoval(host models.DockerHost, removedAt time.Time) {
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return
	}

	m.mu.Lock()
	if m.removedDockerHosts == nil {
		m.removedDockerHosts = make(map[string]time.Time)
	}
	m.removedDockerHosts[hostID] = removedAt
	// Unbind the token so it can be reused with a different agent if needed.
	if host.TokenID != "" {
		delete(m.dockerTokenBindings, host.TokenID)
		log.Debug().
			Str("tokenID", host.TokenID).
			Str("dockerHostID", hostID).
			Msg("Unbound Docker / Podman module token from removed host")
	}
	if cmd, ok := m.dockerCommands[hostID]; ok {
		delete(m.dockerCommandIndex, cmd.status.ID)
	}
	delete(m.dockerCommands, hostID)
	m.clearDockerHostIdentityTrackingLocked(hostID)
	m.mu.Unlock()

	m.state.AddRemovedDockerHost(models.RemovedDockerHost{
		ID:          hostID,
		Hostname:    host.Hostname,
		DisplayName: host.DisplayName,
		Platform:    host.OS,
		RemovedAt:   removedAt,
	})

	m.state.RemoveConnectionHealth(dockerConnectionPrefix + hostID)
	if m.alertManager != nil {
		m.alertManager.HandleDockerHostRemoved(host)
		m.SyncAlertState()
	}
}

func dockerHostBelongsToHostAgent(
	dockerHost models.DockerHost,
	hostID string,
	host models.Host,
	continuity config.HostContinuityEntry,
) bool {
	hostAliases := uniqueNonEmptyStrings(
		hostID,
		host.ID,
		host.MachineID,
		continuity.HostID,
		continuity.ReportHostID,
		continuity.AgentReportedID,
		continuity.MachineID,
	)
	for _, dockerAlias := range []string{dockerHost.ID, dockerHost.AgentID} {
		for _, hostAlias := range hostAliases {
			if hostAgentIdentifiersMatch(dockerAlias, hostAlias) {
				return true
			}
		}
	}

	dockerMachineID := strings.TrimSpace(dockerHost.MachineID)
	hostMachineID := firstNonEmpty(continuity.MachineID, host.MachineID)
	return hostAgentIdentifiersMatch(dockerMachineID, hostMachineID)
}

func (m *Monitor) removeDockerHostsForHostAgent(
	hostID string,
	host models.Host,
	continuity config.HostContinuityEntry,
	removedAt time.Time,
) int {
	removedCount := 0
	for _, dockerHost := range m.state.GetDockerHosts() {
		if !dockerHostBelongsToHostAgent(dockerHost, hostID, host, continuity) {
			continue
		}
		removedDockerHost, removed := m.state.RemoveDockerHost(dockerHost.ID)
		if !removed {
			continue
		}
		m.finalizeDockerHostRemoval(removedDockerHost, removedAt)
		removedCount++
	}
	return removedCount
}

// RemoveHostAgent removes a host agent from monitoring state and clears related data.
func (m *Monitor) RemoveHostAgent(hostID string) (models.Host, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.Host{}, fmt.Errorf("host id is required")
	}

	m.hostAgentLifecycleMu.Lock()
	defer m.hostAgentLifecycleMu.Unlock()

	continuity, hasContinuity := config.HostContinuityEntry{}, false
	if m.hostContinuityStore != nil {
		continuity, hasContinuity = m.hostContinuityStore.Get(hostID)
	}

	host, removed := m.state.RemoveHost(hostID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("hostID", hostID).Msg("host not present in state during removal")
		}
		if hasContinuity {
			host = hostFromContinuityEntry(continuity)
		} else {
			host = models.Host{
				ID:       hostID,
				Hostname: hostID,
			}
		}
	}

	removedAt := time.Now().UTC()
	if !continuity.RemovedAt.IsZero() {
		removedAt = continuity.RemovedAt.UTC()
	}
	tombstone := removedHostContinuityEntry(hostID, host, continuity, removedAt)
	if m.hostContinuityStore != nil {
		if err := m.hostContinuityStore.Upsert(tombstone); err != nil {
			if removed {
				m.state.UpsertHost(host)
			}
			return models.Host{}, fmt.Errorf("persist host agent removal tombstone: %w", err)
		}
	}

	removedEntry := removedHostAgentFromContinuity(tombstone)
	m.mu.Lock()
	if m.removedHostAgents == nil {
		m.removedHostAgents = make(map[string]time.Time)
	}
	m.removedHostAgents[hostID] = removedAt
	m.clearHostAgentIdentityTrackingLocked(hostID)
	m.mu.Unlock()
	m.state.AddRemovedHostAgent(removedEntry)

	removedDockerHosts := m.removeDockerHostsForHostAgent(hostID, host, tombstone, removedAt)

	tokenID := strings.TrimSpace(host.TokenID)
	hostname := strings.TrimSpace(host.Hostname)

	tokenStillUsed := false
	if tokenID != "" && m.state != nil {
		readState := m.snapshotBackedUnifiedReadState()
		for _, other := range readState.Hosts() {
			if other == nil {
				continue
			}
			if strings.TrimSpace(other.TokenID()) == tokenID {
				tokenStillUsed = true
				break
			}
		}
		if !tokenStillUsed {
			for _, other := range readState.DockerHosts() {
				if other == nil {
					continue
				}
				if strings.TrimSpace(other.TokenID()) == tokenID {
					tokenStillUsed = true
					break
				}
			}
		}
	}

	var tokenRemoved *config.APITokenRecord
	if tokenID != "" && !tokenStillUsed {
		var err error
		tokenRemoved, err = m.revokeAPIToken(tokenID)
		if err != nil {
			log.Warn().Err(err).Str("tokenID", tokenID).Msg("API token revocation rolled back after host agent removal")
		} else if tokenRemoved != nil {
			log.Info().Str("tokenID", tokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed host agent")
		}
	} else if tokenID != "" && tokenStillUsed {
		log.Info().
			Str("tokenID", tokenID).
			Str("hostID", hostID).
			Msg("API token still used by other agents; skipping revocation during host removal")
	}

	if tokenID != "" {
		m.mu.Lock()
		if m.hostTokenBindings == nil {
			m.hostTokenBindings = make(map[string]string)
		}

		if _, exists := m.hostTokenBindings[tokenID]; exists {
			delete(m.hostTokenBindings, tokenID)
		}

		if hostname != "" {
			key := hostTokenBindingKey(tokenID, hostname)
			delete(m.hostTokenBindings, key)
		}

		prefix := tokenID + ":"
		for key, boundID := range m.hostTokenBindings {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			if strings.TrimSpace(boundID) == hostID {
				delete(m.hostTokenBindings, key)
			}
		}

		if tokenRemoved != nil {
			for key := range m.hostTokenBindings {
				if strings.HasPrefix(key, prefix) {
					delete(m.hostTokenBindings, key)
				}
			}
		}
		m.mu.Unlock()

		log.Debug().
			Str("tokenID", tokenID).
			Str("hostID", hostID).
			Bool("revoked", tokenRemoved != nil).
			Msg("Unbound host agent token bindings after host removal")
	}

	m.state.RemoveConnectionHealth(hostConnectionPrefix + hostID)

	// Clear LinkedAgentID from any nodes that were linked to this host agent
	unlinkedCount := m.state.UnlinkNodesFromHostAgent(hostID)
	m.clearAgentLXCFilesystems(hostID)
	if unlinkedCount > 0 {
		log.Info().
			Str("hostID", hostID).
			Int("unlinkedNodes", unlinkedCount).
			Msg("Cleared host agent links from PVE nodes")
	}

	log.Info().
		Str("host", host.Hostname).
		Str("hostID", hostID).
		Bool("removed", removed).
		Int("removedDockerHosts", removedDockerHosts).
		Msg("Host agent removed from monitoring")

	if m.alertManager != nil {
		m.alertManager.HandleHostRemoved(host)
	}
	m.hostReportOrderMu.Lock()
	delete(m.hostReportOrders, hostID)
	m.hostReportOrderMu.Unlock()
	m.refreshUnifiedResourceStoreAfterAgentStateChange()

	return host, nil
}

func removedHostContinuityEntry(
	hostID string,
	host models.Host,
	continuity config.HostContinuityEntry,
	removedAt time.Time,
) config.HostContinuityEntry {
	continuity.HostID = strings.TrimSpace(hostID)
	continuity.Hostname = firstNonEmpty(continuity.Hostname, host.Hostname)
	continuity.DisplayName = firstNonEmpty(continuity.DisplayName, host.DisplayName)
	continuity.MachineID = firstNonEmpty(continuity.MachineID, host.MachineID)
	continuity.TokenID = firstNonEmpty(continuity.TokenID, host.TokenID)
	continuity.DeniedTokenIDs = uniqueNonEmptyStrings(
		append(continuity.DeniedTokenIDs, continuity.TokenID, host.TokenID)...,
	)
	continuity.AgentVersion = firstNonEmpty(continuity.AgentVersion, host.AgentVersion)
	continuity.Platform = firstNonEmpty(continuity.Platform, host.Platform, host.OSName)
	continuity.LinkedNodeID = firstNonEmpty(continuity.LinkedNodeID, host.LinkedNodeID)
	continuity.LinkedVMID = firstNonEmpty(continuity.LinkedVMID, host.LinkedVMID)
	continuity.LinkedContainerID = firstNonEmpty(continuity.LinkedContainerID, host.LinkedContainerID)
	continuity.IsLegacy = continuity.IsLegacy || host.IsLegacy
	if continuity.LastSeen.IsZero() {
		continuity.LastSeen = host.LastSeen.UTC()
	}
	continuity.RemovedAt = removedAt.UTC()
	return continuity
}

func removedHostAgentFromContinuity(entry config.HostContinuityEntry) models.RemovedHostAgent {
	return models.RemovedHostAgent{
		ID:                strings.TrimSpace(entry.HostID),
		ReportHostID:      strings.TrimSpace(entry.ReportHostID),
		AgentReportedID:   strings.TrimSpace(entry.AgentReportedID),
		Hostname:          strings.TrimSpace(entry.Hostname),
		DisplayName:       strings.TrimSpace(entry.DisplayName),
		Platform:          strings.TrimSpace(entry.Platform),
		MachineID:         strings.TrimSpace(entry.MachineID),
		TokenID:           strings.TrimSpace(entry.TokenID),
		LinkedVMID:        strings.TrimSpace(entry.LinkedVMID),
		LinkedContainerID: strings.TrimSpace(entry.LinkedContainerID),
		RemovedAt:         entry.RemovedAt.UTC(),
	}
}

// AllowHostAgentReenroll removes a host agent ID from the removal blocklist so it can report again.
func (m *Monitor) AllowHostAgentReenroll(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("host id is required")
	}

	m.hostAgentLifecycleMu.Lock()
	defer m.hostAgentLifecycleMu.Unlock()
	_, err := m.allowHostAgentReenrollLocked(hostID, "", true)
	return err
}

func (m *Monitor) allowHostAgentReenrollLocked(
	hostID string,
	replacementTokenID string,
	clearDeniedTokens bool,
) (bool, error) {
	hasDurableStore := m.hostContinuityStore != nil
	existsOnDisk := false
	if hasDurableStore {
		cleared, err := m.hostContinuityStore.ClearRemoval(hostID, replacementTokenID, clearDeniedTokens)
		if err != nil {
			return false, fmt.Errorf("persist host agent re-enrollment allowance: %w", err)
		}
		existsOnDisk = cleared
	}

	m.mu.Lock()
	if m.removedHostAgents == nil {
		m.removedHostAgents = make(map[string]time.Time)
	}
	_, existsInMemory := m.removedHostAgents[hostID]
	delete(m.removedHostAgents, hostID)
	m.mu.Unlock()

	existsInState := false
	for _, entry := range m.state.GetRemovedHostAgents() {
		if strings.TrimSpace(entry.ID) == hostID {
			existsInState = true
			break
		}
	}

	// A removal block can live in the durable store, the legacy in-memory
	// map, or persisted monitor state written by releases that predate the
	// durable store. Clearing must honor whichever still holds the block:
	// trusting only the durable store leaves a state-only block permanently
	// un-clearable after an upgrade, so every fresh re-enroll reads as
	// "already consumed" and the host can never report again (#1772).
	transitioned := existsOnDisk || existsInMemory || existsInState
	if !transitioned {
		log.Info().
			Str("hostID", hostID).
			Msg("allow re-enroll requested but host agent was not blocked; ignoring")
		return false, nil
	}

	m.state.RemoveRemovedHostAgent(hostID)

	log.Info().
		Str("hostID", hostID).
		Msg("Host agent removal block cleared; host may report again")

	return transitioned, nil
}

func (m *Monitor) lookupRemovedHostAgent(identifier, hostname, machineID, tokenID string) (models.RemovedHostAgent, bool) {
	identifier = strings.TrimSpace(identifier)
	machineID = sanitizeDockerHostSuffix(machineID)
	tokenID = strings.TrimSpace(tokenID)

	for _, entry := range m.state.GetRemovedHostAgents() {
		if removedHostAgentMatchesReport(entry, identifier, hostname, machineID, tokenID) {
			return entry, true
		}
	}

	m.mu.RLock()
	removedAt, wasRemoved := m.removedHostAgents[identifier]
	m.mu.RUnlock()
	if wasRemoved {
		return models.RemovedHostAgent{
			ID:        identifier,
			RemovedAt: removedAt,
		}, true
	}

	return models.RemovedHostAgent{}, false
}

func removedHostAgentMatchesReport(entry models.RemovedHostAgent, identifier, hostname, machineID, tokenID string) bool {
	for _, persistedID := range []string{entry.ID, entry.ReportHostID, entry.AgentReportedID} {
		if hostAgentIdentifiersMatch(persistedID, identifier) {
			return true
		}
	}

	entryMachineID := sanitizeDockerHostSuffix(entry.MachineID)
	entryTokenID := strings.TrimSpace(entry.TokenID)
	if entryTokenID != "" || tokenID != "" {
		if entryTokenID == "" || tokenID == "" || entryTokenID != tokenID {
			return false
		}
		if entryMachineID != "" && machineID != "" && entryMachineID == machineID {
			return true
		}
		if !hostAgentHostnamesMatch(entry.Hostname, hostname) {
			return false
		}
		return entryMachineID == "" || machineID == "" || entryMachineID == machineID
	}

	return entryMachineID != "" &&
		machineID != "" &&
		entryMachineID == machineID &&
		hostAgentHostnamesMatch(entry.Hostname, hostname)
}

func removedHostAgentAllowsFreshReenroll(
	entry models.RemovedHostAgent,
	identifier string,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	if tokenRecord == nil ||
		tokenRecord.CreatedAt.IsZero() ||
		!tokenRecord.CreatedAt.After(entry.RemovedAt) {
		return false
	}

	oldTokenID := strings.TrimSpace(entry.TokenID)
	newTokenID := strings.TrimSpace(tokenRecord.ID)
	if newTokenID == "" || (oldTokenID != "" && oldTokenID == newTokenID) {
		return false
	}

	hostname := strings.TrimSpace(report.Host.Hostname)
	if strings.TrimSpace(entry.Hostname) != "" &&
		hostname != "" &&
		!hostAgentHostnamesMatch(entry.Hostname, hostname) {
		return false
	}

	entryMachineID := sanitizeDockerHostSuffix(entry.MachineID)
	reportMachineID := sanitizeDockerHostSuffix(report.Host.MachineID)
	if entryMachineID != "" && reportMachineID != "" && !hostAgentIdentifiersMatch(entryMachineID, reportMachineID) {
		return false
	}

	persistedIDs := []string{
		entry.ID,
		entry.ReportHostID,
		entry.AgentReportedID,
		entry.MachineID,
	}
	reportedIDs := []string{
		identifier,
		report.Host.ID,
		report.Host.MachineID,
		report.Agent.ID,
	}
	for _, persistedID := range persistedIDs {
		for _, reportedID := range reportedIDs {
			if hostAgentIdentifiersMatch(persistedID, reportedID) {
				return true
			}
		}
	}
	return entryMachineID != "" &&
		reportMachineID != "" &&
		hostAgentIdentifiersMatch(entryMachineID, reportMachineID)
}

func hostAgentIdentifiersMatch(left, right string) bool {
	left = sanitizeDockerHostSuffix(left)
	right = sanitizeDockerHostSuffix(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

// LinkHostAgent manually links a host agent to a specific PVE node.
// This is used when auto-linking can't disambiguate (e.g., multiple nodes with hostname "pve").
// After linking, the host agent's temperature/sensor data will appear on the correct node.
func (m *Monitor) LinkHostAgent(hostID, nodeID string) error {
	hostID = strings.TrimSpace(hostID)
	nodeID = strings.TrimSpace(nodeID)
	if hostID == "" {
		return fmt.Errorf("host id is required")
	}
	if nodeID == "" {
		return fmt.Errorf("node id is required")
	}

	if err := m.state.LinkHostAgentToNode(hostID, nodeID); err != nil {
		return fmt.Errorf("link host agent %q to node %q: %w", hostID, nodeID, err)
	}

	log.Info().
		Str("hostID", hostID).
		Str("nodeID", nodeID).
		Msg("Manually linked host agent to PVE node")

	return nil
}

// UnlinkHostAgent removes the link between a host agent and its PVE node.
// The agent will continue to report but will appear in the Managed Agents table
// instead of being merged with the PVE node in the Dashboard.
func (m *Monitor) UnlinkHostAgent(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("host id is required")
	}

	if !m.state.UnlinkHostAgent(hostID) {
		return fmt.Errorf("host not found or not linked to a node")
	}

	log.Info().
		Str("hostID", hostID).
		Msg("Unlinked host agent from PVE node")

	return nil
}

// HostAgentConfig represents server-side configuration for a host agent.
type HostAgentConfig struct {
	CommandsEnabled *bool                               `json:"commandsEnabled,omitempty"` // nil = use agent default
	Settings        map[string]interface{}              `json:"settings,omitempty"`        // Merged profile settings
	DesiredConfig   *remoteconfig.DesiredConfigMetadata `json:"desiredConfig,omitempty"`
	IssuedAt        *time.Time                          `json:"issuedAt,omitempty"`
	ExpiresAt       *time.Time                          `json:"expiresAt,omitempty"`
	Signature       string                              `json:"signature,omitempty"`
}

// GetHostAgentConfig returns the server-side configuration for a host agent.
// The agent can poll this to apply remote config overrides.
// Uses in-memory caching to avoid disk I/O on every agent report (refs #1094).
func (m *Monitor) GetHostAgentConfig(hostID string) HostAgentConfig {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return HostAgentConfig{}
	}

	cfg := HostAgentConfig{}

	// 1. Load Host Metadata (CommandsEnabled) - this is already in-memory
	if m.hostMetadataStore != nil {
		if meta := m.hostMetadataStore.Get(hostID); meta != nil {
			cfg.CommandsEnabled = meta.CommandsEnabled
		}
	}

	// 2. Load Profile Configuration from cache
	if m.persistence != nil {
		profiles, assignments := m.getAgentProfileCache()

		var profileID string
		for _, a := range assignments {
			if a.AgentID == hostID {
				profileID = a.ProfileID
				break
			}
		}

		if profileID != "" {
			for _, p := range profiles {
				if p.ID == profileID {
					cfg.Settings = p.MergedConfig(profiles)
					break
				}
			}
		}
	}

	// 3. Externally probed availability targets assigned to this agent. Each
	// agent only ever sees its own assignments because the payload is built
	// from this hostID.
	if targets := m.availabilityProbeTargetsForAgent(hostID); len(targets) > 0 {
		if cfg.Settings == nil {
			cfg.Settings = map[string]interface{}{}
		}
		cfg.Settings["availabilityTargets"] = targets
	}

	return attachDesiredConfigMetadata(cfg)
}

func attachDesiredConfigMetadata(cfg HostAgentConfig) HostAgentConfig {
	metadata, err := remoteconfig.BuildDesiredConfigMetadata(cfg.CommandsEnabled, cfg.Settings)
	if err != nil {
		log.Warn().Err(err).Msg("failed to build host agent desired config metadata")
		return cfg
	}
	cfg.DesiredConfig = &metadata
	return cfg
}

// getAgentProfileCache returns cached profiles and assignments, refreshing if stale.
func (m *Monitor) getAgentProfileCache() ([]models.AgentProfile, []models.AgentProfileAssignment) {
	now := time.Now()

	// Fast path: check if cache is valid
	m.agentProfileCacheMu.RLock()
	cache := m.agentProfileCache
	if cache != nil && now.Sub(cache.loadedAt) < agentProfileCacheTTL {
		profiles := cache.profiles
		assignments := cache.assignments
		m.agentProfileCacheMu.RUnlock()
		return profiles, assignments
	}
	m.agentProfileCacheMu.RUnlock()

	// Slow path: reload from disk
	m.agentProfileCacheMu.Lock()
	defer m.agentProfileCacheMu.Unlock()

	// Double-check after acquiring write lock
	if m.agentProfileCache != nil && now.Sub(m.agentProfileCache.loadedAt) < agentProfileCacheTTL {
		return m.agentProfileCache.profiles, m.agentProfileCache.assignments
	}

	var profiles []models.AgentProfile
	var assignments []models.AgentProfileAssignment

	if loadedAssignments, err := m.persistence.LoadAgentProfileAssignments(); err != nil {
		log.Warn().Err(err).Msg("failed to load agent profile assignments for cache")
	} else {
		assignments = loadedAssignments
	}

	if loadedProfiles, err := m.persistence.LoadAgentProfiles(); err != nil {
		log.Warn().Err(err).Msg("failed to load agent profiles for cache")
	} else {
		profiles = loadedProfiles
	}

	m.agentProfileCache = &agentProfileCacheEntry{
		profiles:    profiles,
		assignments: assignments,
		loadedAt:    now,
	}

	return profiles, assignments
}

// InvalidateAgentProfileCache clears the agent profile cache, forcing a reload on next access.
// Call this when profiles or assignments are modified.
func (m *Monitor) InvalidateAgentProfileCache() {
	m.agentProfileCacheMu.Lock()
	m.agentProfileCache = nil
	m.agentProfileCacheMu.Unlock()
}

// UpdateHostAgentConfig updates the server-side configuration for a host agent.
// This allows the UI to remotely enable/disable features on agents.
func (m *Monitor) UpdateHostAgentConfig(hostID string, commandsEnabled *bool) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("host id is required")
	}

	if m.hostMetadataStore == nil {
		return fmt.Errorf("host metadata store not initialized")
	}

	// Get existing metadata or create new
	meta := m.hostMetadataStore.Get(hostID)
	if meta == nil {
		meta = &config.HostMetadata{ID: hostID}
	}

	meta.CommandsEnabled = commandsEnabled

	if err := m.hostMetadataStore.Set(hostID, meta); err != nil {
		return fmt.Errorf("failed to save host config: %w", err)
	}

	log.Info().
		Str("hostId", hostID).
		Interface("commandsEnabled", commandsEnabled).
		Msg("Host agent config updated")

	return nil
}

// HideDockerHost marks a docker host as hidden without removing it from state.
// Hidden hosts will not be shown in the frontend but will continue to accept updates.
func (m *Monitor) HideDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}
	hostID = m.canonicalDockerHostID(hostID)

	host, ok := m.state.SetDockerHostHidden(hostID, true)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host hidden from view")

	return host, nil
}

// UnhideDockerHost marks a docker host as visible again.
func (m *Monitor) UnhideDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}
	hostID = m.canonicalDockerHostID(hostID)

	host, ok := m.state.SetDockerHostHidden(hostID, false)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	// Clear removal tracking if it was marked as removed
	m.mu.Lock()
	delete(m.removedDockerHosts, hostID)
	m.mu.Unlock()

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host unhidden")

	return host, nil
}

// MarkDockerHostPendingUninstall marks a docker host as pending uninstall.
// This is used when the user has run the uninstall command and is waiting for the host to go offline.
func (m *Monitor) MarkDockerHostPendingUninstall(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}
	hostID = m.canonicalDockerHostID(hostID)

	host, ok := m.state.SetDockerHostPendingUninstall(hostID, true)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Msg("Docker host marked as pending uninstall")

	return host, nil
}

// SetDockerHostCustomDisplayName updates the custom display name for a docker host.
func (m *Monitor) SetDockerHostCustomDisplayName(hostID string, customName string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}
	hostID = m.canonicalDockerHostID(hostID)

	customName = strings.TrimSpace(customName)

	// Persist to Docker metadata store first without discarding independently
	// authored URL or notes metadata.
	hostMeta := m.dockerMetadataStore.GetHostMetadata(hostID)
	if hostMeta == nil {
		hostMeta = &config.DockerHostMetadata{}
	}
	hostMeta.CustomDisplayName = customName
	if err := m.dockerMetadataStore.SetHostMetadata(hostID, hostMeta); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("failed to persist Docker host metadata")
		return models.DockerHost{}, fmt.Errorf("failed to persist custom display name: %w", err)
	}

	// Update in-memory state
	host, ok := m.state.SetDockerHostCustomDisplayName(hostID, customName)
	if !ok {
		return models.DockerHost{}, fmt.Errorf("docker host %q not found", hostID)
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Str("customDisplayName", customName).
		Msg("Docker host custom display name updated")

	return host, nil
}

// lookupRemovedDockerHost reports whether any of the presented Docker host
// identities is blocked by a deliberate removal, returning the blocked entry's
// ID and removal time. It consults the in-memory map first and the persisted
// store second: the map resets on restart while the persisted entry keeps
// blocking (#1581), so memory absence alone must not admit a blocked host.
func (m *Monitor) lookupRemovedDockerHost(ids []string) (string, time.Time, bool) {
	m.mu.RLock()
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if ts, ok := m.removedDockerHosts[id]; ok {
			m.mu.RUnlock()
			return id, ts, true
		}
	}
	m.mu.RUnlock()

	for _, entry := range m.state.GetRemovedDockerHosts() {
		entryID := strings.TrimSpace(entry.ID)
		if entryID == "" {
			continue
		}
		for _, id := range ids {
			if strings.TrimSpace(id) == entryID {
				return entryID, entry.RemovedAt, true
			}
		}
	}
	return "", time.Time{}, false
}

// AllowDockerHostReenroll removes a host ID from the removal blocklist so it can report again.
func (m *Monitor) AllowDockerHostReenroll(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("docker host id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, resolvedHostID, found := m.resolveDockerCommandHostLocked(hostID); found {
		hostID = resolvedHostID
	}

	// The in-memory map resets on restart while the persisted entry keeps
	// blocking reports, so the persisted store must be checked and cleared
	// independently of memory presence (#1581).
	_, existsInMemory := m.removedDockerHosts[hostID]
	existsInState := false
	for _, entry := range m.state.GetRemovedDockerHosts() {
		if strings.TrimSpace(entry.ID) == hostID {
			existsInState = true
			break
		}
	}

	if !existsInMemory && !existsInState {
		event := log.Info().
			Str("dockerHostID", hostID)
		if host, found := m.stateDockerHostByIDLocked(hostID); found {
			event = event.Str("dockerHost", host.Hostname)
		}
		event.Msg("allow re-enroll requested but host was not blocked; ignoring")
		return nil
	}

	delete(m.removedDockerHosts, hostID)
	if cmd, exists := m.dockerCommands[hostID]; exists {
		delete(m.dockerCommandIndex, cmd.status.ID)
		delete(m.dockerCommands, hostID)
	}
	m.state.SetDockerHostCommand(hostID, nil)
	m.state.RemoveRemovedDockerHost(hostID)

	log.Info().
		Str("dockerHostID", hostID).
		Msg("Docker host removal block cleared; host may report again")

	return nil
}

// GetDockerHost retrieves a docker host by identifier if present in state.
func (m *Monitor) GetDockerHost(hostID string) (models.DockerHost, bool) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, false
	}
	hostID = m.canonicalDockerHostID(hostID)

	hosts := m.state.GetDockerHosts()
	for _, host := range hosts {
		if host.ID == hostID {
			return host, true
		}
	}
	return models.DockerHost{}, false
}

// GetDockerHosts returns a point-in-time snapshot of all Docker hosts Pulse knows about.
func (m *Monitor) GetDockerHosts() []models.DockerHost {
	if m == nil || m.state == nil {
		return nil
	}
	return m.state.GetDockerHosts()
}

func (m *Monitor) canonicalDockerHostID(hostID string) string {
	hostID = normalizeDockerHostID(hostID)
	if hostID == "" {
		return ""
	}

	if _, resolvedHostID, found := m.resolveDockerHostView(hostID); found {
		return resolvedHostID
	}

	return hostID
}

func (m *Monitor) resolveDockerHostView(hostID string) (*unifiedresources.DockerHostView, string, bool) {
	hostID = normalizeDockerHostID(hostID)
	if hostID == "" {
		return nil, "", false
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil, "", false
	}

	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}
		candidateID := normalizeDockerHostID(host.ID())
		sourceID := normalizeDockerHostID(host.HostSourceID())
		if hostID != candidateID && hostID != sourceID {
			continue
		}
		if sourceID == "" {
			sourceID = candidateID
		}
		if sourceID == "" {
			return nil, "", false
		}
		return host, sourceID, true
	}

	return nil, "", false
}

func (m *Monitor) snapshotBackedUnifiedReadState() unifiedresources.ReadState {
	if m == nil || m.state == nil {
		return nil
	}

	registry := unifiedresources.NewRegistry(nil)
	thresholds := m.resourceStaleThresholds()
	registry.IngestSnapshotWithStaleThresholds(m.state.GetSnapshot(), thresholds)
	return unifiedresources.NewMonitorAdapterWithStaleThresholds(registry, thresholds)
}

func (m *Monitor) hostContinuitySince(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.Add(-hostContinuityRetention)
}

func hostTokenBindingKey(tokenID, hostname string) string {
	tokenID = strings.TrimSpace(tokenID)
	hostname = strings.TrimSpace(hostname)
	if tokenID == "" || hostname == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", tokenID, hostname)
}

func lookupHostTokenBinding(bindings map[string]string, tokenID, hostname string) string {
	if len(bindings) == 0 {
		return ""
	}

	bindingKey := hostTokenBindingKey(tokenID, hostname)
	if bindingKey == "" {
		return ""
	}
	if boundID := strings.TrimSpace(bindings[bindingKey]); boundID != "" {
		return boundID
	}

	prefix := strings.TrimSpace(tokenID) + ":"
	for key, boundID := range bindings {
		boundID = strings.TrimSpace(boundID)
		if boundID == "" || !strings.HasPrefix(key, prefix) {
			continue
		}
		boundHostname := strings.TrimSpace(strings.TrimPrefix(key, prefix))
		if hostAgentHostnamesMatch(boundHostname, hostname) {
			return boundID
		}
	}
	return ""
}

func hostAgentHostnamesMatch(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return strings.EqualFold(left, right) || unifiedresources.HostnamesEquivalent(left, right)
}

func (m *Monitor) matchPersistedHostContinuity(
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) (config.HostContinuityEntry, bool) {
	if m == nil || m.hostContinuityStore == nil {
		return config.HostContinuityEntry{}, false
	}

	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}

	return m.hostContinuityStore.Match(
		report.Host.ID,
		report.Host.MachineID,
		report.Agent.ID,
		report.Host.Hostname,
		tokenID,
		m.hostContinuitySince(time.Now().UTC()),
	)
}

func hostContinuityDeniesToken(
	entry config.HostContinuityEntry,
	tokenRecord *config.APITokenRecord,
) bool {
	if tokenRecord == nil {
		return false
	}
	tokenID := strings.TrimSpace(tokenRecord.ID)
	if tokenID == "" {
		return false
	}
	for _, deniedTokenID := range entry.DeniedTokenIDs {
		if strings.TrimSpace(deniedTokenID) == tokenID {
			return true
		}
	}
	return false
}

// MatchHostConfigContinuity resolves a host identity for agent config fetches
// from the persisted continuity store. Live state loses agent-reported hosts
// across monitor reloads and restarts until the next report lands, and config
// fetches in that window 404ed with a valid token (#1570). Mirrors the live
// resolution semantics: a report-scoped token resolves by its binding, a
// manage-scoped (or absent) token resolves by host ID.
func (m *Monitor) MatchHostConfigContinuity(agentID, tokenID string) (models.Host, bool) {
	if m == nil || m.hostContinuityStore == nil {
		return models.Host{}, false
	}
	agentID = strings.TrimSpace(agentID)
	tokenID = strings.TrimSpace(tokenID)

	// RecentEntries is sorted newest-first, so the first match wins.
	for _, entry := range m.hostContinuityStore.RecentEntries(m.hostContinuitySince(time.Now().UTC())) {
		if tokenID != "" {
			if strings.TrimSpace(entry.TokenID) != tokenID {
				continue
			}
		} else if agentID == "" ||
			(!hostAgentIdentifiersMatch(entry.HostID, agentID) &&
				!hostAgentIdentifiersMatch(entry.ReportHostID, agentID) &&
				!hostAgentIdentifiersMatch(entry.AgentReportedID, agentID) &&
				!hostAgentIdentifiersMatch(entry.MachineID, agentID)) {
			continue
		}
		return models.Host{
			ID:          entry.HostID,
			Hostname:    entry.Hostname,
			DisplayName: entry.DisplayName,
			MachineID:   entry.MachineID,
			TokenID:     entry.TokenID,
			Platform:    entry.Platform,
			IsLegacy:    entry.IsLegacy,
		}, true
	}

	return models.Host{}, false
}

func (m *Monitor) persistHostContinuity(host models.Host, report agentshost.Report, order hostReportOrder) {
	if m == nil || m.hostContinuityStore == nil {
		return
	}

	entry := config.HostContinuityEntry{
		HostID:               strings.TrimSpace(host.ID),
		ReportHostID:         strings.TrimSpace(report.Host.ID),
		AgentReportedID:      strings.TrimSpace(report.Agent.ID),
		Hostname:             strings.TrimSpace(host.Hostname),
		DisplayName:          strings.TrimSpace(host.DisplayName),
		MachineID:            strings.TrimSpace(host.MachineID),
		TokenID:              strings.TrimSpace(host.TokenID),
		AgentVersion:         strings.TrimSpace(host.AgentVersion),
		Platform:             strings.TrimSpace(host.Platform),
		LinkedNodeID:         strings.TrimSpace(host.LinkedNodeID),
		LinkedVMID:           strings.TrimSpace(host.LinkedVMID),
		LinkedContainerID:    strings.TrimSpace(host.LinkedContainerID),
		IsLegacy:             host.IsLegacy,
		LastSeen:             host.LastSeen.UTC(),
		IntervalSeconds:      host.IntervalSeconds,
		ReportObservedAt:     order.ObservedAt.UTC(),
		ReportLastReceivedAt: order.LastReceivedAt.UTC(),
		ReportStreamID:       strings.TrimSpace(order.StreamID),
		ReportSequence:       order.Sequence,
		RetiredReportStreamIDs: append(
			[]string(nil),
			order.RetiredStreamIDs...,
		),
	}
	if previous, ok := m.hostContinuityStore.Get(entry.HostID); ok {
		entry.DeniedTokenIDs = append([]string(nil), previous.DeniedTokenIDs...)
	}
	if err := m.hostContinuityStore.Upsert(entry); err != nil {
		log.Warn().
			Err(err).
			Str("hostID", host.ID).
			Msg("failed to persist host continuity state")
	}
}

func (m *Monitor) hostByID(hostID string) (models.Host, bool) {
	for _, host := range m.state.GetHosts() {
		if host.ID == hostID {
			return host, true
		}
	}
	return models.Host{}, false
}

func (m *Monitor) applyRejectedHostReportLiveness(
	hostID string,
	report agentshost.Report,
	receivedAt time.Time,
	order hostReportOrder,
) models.Host {
	host, exists := m.hostByID(hostID)
	if !exists {
		if m.hostContinuityStore != nil {
			if entry, ok := m.hostContinuityStore.Get(hostID); ok {
				host = hostFromContinuityEntry(entry)
			}
		}
		if host.ID == "" {
			host = models.Host{
				ID:           hostID,
				Hostname:     strings.TrimSpace(report.Host.Hostname),
				DisplayName:  strings.TrimSpace(report.Host.DisplayName),
				Platform:     platformsupport.NormalizeAgentReportedPlatform(report.Host.Platform),
				AgentVersion: strings.TrimSpace(report.Agent.Version),
				MachineID:    strings.TrimSpace(report.Host.MachineID),
			}
		}
		if host.IntervalSeconds <= 0 {
			host.IntervalSeconds = report.Agent.IntervalSeconds
		}
		if host.LastSeen.IsZero() ||
			receivedAt.Sub(host.LastSeen) > hostAgentHealthWindow(host.IntervalSeconds) {
			host.Status = "offline"
		}
		m.state.UpsertHost(host)
	}

	// An authenticated arrival proves the transport is reachable, but a report
	// rejected by the authored-state watermark must not extend Host.LastSeen,
	// clear an offline alert, or renew the accepted telemetry lease.
	m.state.SetConnectionHealth(hostConnectionPrefix+host.ID, true)
	m.persistHostContinuity(host, report, order)
	m.refreshUnifiedResourceStoreAfterAgentStateChange()

	log.Debug().
		Str("hostID", host.ID).
		Str("sequenceId", report.SequenceID).
		Time("reportTimestamp", report.Timestamp).
		Msg("Ignored stale or duplicate host report state while refreshing receipt-time liveness")
	return host
}

// HostReportMatchesKnownIdentity returns true when a host report targets either
// the live host snapshot or a recent persisted standalone-host continuity
// record.
func (m *Monitor) HostReportMatchesKnownIdentity(
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	if m == nil {
		return false
	}

	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	if pkglicensing.HostReportTargetsExistingHosts(m.GetLiveHostsSnapshot(), report, tokenID) {
		return true
	}
	_, ok := m.matchPersistedHostContinuity(report, tokenRecord)
	return ok
}

func hostFromContinuityEntry(entry config.HostContinuityEntry) models.Host {
	return models.Host{
		ID:                strings.TrimSpace(entry.HostID),
		Hostname:          strings.TrimSpace(entry.Hostname),
		DisplayName:       strings.TrimSpace(entry.DisplayName),
		Status:            "online",
		LastSeen:          entry.LastSeen,
		AgentVersion:      strings.TrimSpace(entry.AgentVersion),
		MachineID:         strings.TrimSpace(entry.MachineID),
		TokenID:           strings.TrimSpace(entry.TokenID),
		Platform:          platformsupport.NormalizeAgentReportedPlatform(entry.Platform),
		IsLegacy:          entry.IsLegacy,
		IntervalSeconds:   entry.IntervalSeconds,
		LinkedNodeID:      strings.TrimSpace(entry.LinkedNodeID),
		LinkedVMID:        strings.TrimSpace(entry.LinkedVMID),
		LinkedContainerID: strings.TrimSpace(entry.LinkedContainerID),
	}
}

// mockDiscardedHostReportAck echoes the reporting agent's own identity back to
// it without touching monitor state, so the ingest response keeps its usual
// shape for a report mock mode deliberately dropped.
func mockDiscardedHostReportAck(report agentshost.Report) models.Host {
	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		hostname = strings.TrimSpace(report.Agent.Hostname)
	}
	return models.Host{
		ID:           strings.TrimSpace(report.Agent.ID),
		Hostname:     hostname,
		DisplayName:  strings.TrimSpace(report.Host.DisplayName),
		Platform:     strings.TrimSpace(report.Host.Platform),
		OSName:       strings.TrimSpace(report.Host.OSName),
		OSVersion:    strings.TrimSpace(report.Host.OSVersion),
		AgentVersion: strings.TrimSpace(report.Agent.Version),
		LastSeen:     report.Timestamp,
	}
}

func (m *Monitor) recentStandaloneHostContinuityEntries() []config.HostContinuityEntry {
	if m == nil || m.hostContinuityStore == nil {
		return nil
	}
	// Mock mode is a clean room. Continuity entries are persisted to disk from
	// real agent reports and outlive the mock toggle, and every consumer of
	// this list injects them after the read path has already substituted the
	// mock snapshot: standalone host continuity lands on Machines, host-offline
	// alerts fire, and availability probe alerts resolve display names. A real
	// machine that reported before mock mode was enabled would therefore
	// resurface by its real hostname inside an otherwise synthetic view.
	// Unlike the connections ledger there is no real-polling exception here,
	// because agent ingestion is not gated on PULSE_MOCK_KEEP_REAL_POLLING and
	// the unified read state is mock-substituted wholesale either way, so
	// injecting real hosts would only graft them onto mock data.
	if mock.IsMockEnabled() {
		return nil
	}
	return m.hostContinuityStore.RecentEntries(m.hostContinuitySince(time.Now().UTC()))
}

// RebuildTokenBindings reconstructs agent-to-token binding maps from the current
// state of Docker hosts and host agents. This should be called after API tokens
// are reloaded from disk to ensure bindings remain consistent with the new token set.
// It preserves bindings for tokens that still exist and removes orphaned entries.
func (m *Monitor) RebuildTokenBindings() {
	if m == nil || m.config == nil {
		return
	}
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return
	}

	// Build a set of valid token IDs from the current config
	validTokens := make(map[string]struct{})
	for _, token := range m.config.APITokens {
		if token.ID != "" {
			validTokens[token.ID] = struct{}{}
		}
	}

	// Rebuild Docker token bindings
	newDockerBindings := make(map[string]string)
	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}
		tokenID := strings.TrimSpace(host.TokenID())
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		agentID := dockerHostStableID(host)
		if agentID == "" {
			agentID = strings.TrimSpace(host.AgentID())
		}
		if agentID != "" {
			newDockerBindings[tokenID] = agentID
		}
	}

	// Rebuild Host agent token bindings
	newHostBindings := make(map[string]string)
	for _, host := range readState.Hosts() {
		if host == nil {
			continue
		}
		tokenID := strings.TrimSpace(host.TokenID())
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		hostname := strings.TrimSpace(host.Hostname())
		agentID := strings.TrimSpace(host.AgentID())
		if hostname == "" || agentID == "" {
			continue
		}
		newHostBindings[hostTokenBindingKey(tokenID, hostname)] = agentID
	}

	// Log what changed
	m.mu.Lock()
	defer m.mu.Unlock()
	oldDockerCount := len(m.dockerTokenBindings)
	oldHostCount := len(m.hostTokenBindings)
	m.dockerTokenBindings = newDockerBindings
	m.hostTokenBindings = newHostBindings

	log.Info().
		Int("dockerBindings", len(newDockerBindings)).
		Int("hostBindings", len(newHostBindings)).
		Int("previousDockerBindings", oldDockerCount).
		Int("previousHostBindings", oldHostCount).
		Int("validTokens", len(validTokens)).
		Msg("Rebuilt agent token bindings after API token reload")
}

// ClearUnauthenticatedAgents removes all host agents and docker hosts from the state.
// This should be called when security is first configured to clear any agents that
// connected during the brief unauthenticated window before credentials were set up.
// This prevents stale/unauthorized agent data from appearing in the UI.
func (m *Monitor) ClearUnauthenticatedAgents() (int, int) {
	if m == nil || m.state == nil {
		return 0, 0
	}

	// Clear all hosts
	hostCount := m.state.ClearAllHosts()

	// Clear all docker hosts
	dockerCount := m.state.ClearAllDockerHosts()

	// Clear any token bindings since the tokens used by the old agents are invalid
	m.mu.Lock()
	m.dockerTokenBindings = make(map[string]string)
	m.hostTokenBindings = make(map[string]string)
	m.dockerIdentityFlaps = make(map[string]*identityFlapTracker)
	m.hostIdentityFlaps = make(map[string]*identityFlapTracker)
	m.mu.Unlock()

	if hostCount > 0 || dockerCount > 0 {
		log.Info().
			Int("hostsCleared", hostCount).
			Int("dockerHostsCleared", dockerCount).
			Msg("Cleared unauthenticated agents after security setup")
	}

	return hostCount, dockerCount
}

// QueueDockerHostStop queues a stop command for the specified docker host.
func (m *Monitor) QueueDockerHostStop(hostID string) (models.DockerHostCommandStatus, error) {
	return m.queueDockerStopCommand(hostID)
}

// FetchDockerCommandForHost retrieves the next command payload (if any) for the host.
func (m *Monitor) FetchDockerCommandForHost(hostID string) (map[string]any, *models.DockerHostCommandStatus) {
	return m.getDockerCommandPayload(hostID)
}

// AcknowledgeDockerHostCommand updates the lifecycle status for a docker host command.
func (m *Monitor) AcknowledgeDockerHostCommand(commandID, hostID, status, message string) (models.DockerHostCommandStatus, string, bool, error) {
	return m.acknowledgeDockerCommand(commandID, hostID, status, message)
}

// supersedeStaleDockerHostDuplicates reaps leftover Docker host records for
// the same physical machine after an agent re-enrolls under a fresh identity.
// Wiping the agent state dir regenerates the agent ID, and a fresh install
// command mints a fresh token, so identity resolution correctly refuses to
// adopt the old record (a foreign token must never take over a live host,
// #1008) and creates a new one — leaving the old record behind forever with a
// stale agent version, stale containers, and stale image digests (#1586,
// #1564). A record qualifies as a corpse only when it stopped reporting
// before the superseding token was even minted: generating a fresh install
// command for the same machine is explicit replace intent (the same rule the
// removal block uses for re-enrollment, #1581), while a record that is still
// reporting keeps advancing LastSeen and is never touched. Unlike a
// user-initiated removal this does not set the resurrection block and does
// not revoke the orphaned token.
func (m *Monitor) supersedeStaleDockerHostDuplicates(current models.DockerHost, tokenRecord *config.APITokenRecord, hosts []*unifiedresources.DockerHostView) {
	if tokenRecord == nil || tokenRecord.CreatedAt.IsZero() {
		return
	}
	machineID := strings.TrimSpace(current.MachineID)
	hostname := strings.TrimSpace(current.Hostname)
	if machineID == "" || hostname == "" {
		return
	}

	for _, stale := range hosts {
		if stale == nil {
			continue
		}
		staleID := dockerHostStableID(stale)
		if staleID == "" || staleID == strings.TrimSpace(current.ID) {
			continue
		}
		if strings.TrimSpace(stale.MachineID()) != machineID {
			continue
		}
		if !unifiedresources.HostnamesEquivalent(stale.Hostname(), hostname) {
			continue
		}
		if !stale.LastSeen().Before(tokenRecord.CreatedAt) {
			continue
		}

		removed, ok := m.state.RemoveDockerHost(staleID)
		if !ok {
			continue
		}
		m.state.RemoveConnectionHealth(dockerConnectionPrefix + staleID)

		m.mu.Lock()
		if removed.TokenID != "" && removed.TokenID != current.TokenID {
			delete(m.dockerTokenBindings, removed.TokenID)
		}
		if cmd, ok := m.dockerCommands[staleID]; ok {
			delete(m.dockerCommandIndex, cmd.status.ID)
		}
		delete(m.dockerCommands, staleID)
		m.clearDockerHostIdentityTrackingLocked(staleID)
		m.mu.Unlock()

		if m.alertManager != nil {
			m.alertManager.HandleDockerHostRemoved(removed)
			m.SyncAlertState()
		}

		log.Info().
			Str("dockerHost", removed.Hostname).
			Str("staleDockerHostID", staleID).
			Str("replacementDockerHostID", current.ID).
			Msg("Superseded stale Docker host record after agent re-enrollment")
	}
}

// ApplyDockerReport ingests a Docker / Podman module report into the shared state.
func (m *Monitor) ApplyDockerReport(report agentsdocker.Report, tokenRecord *config.APITokenRecord) (models.DockerHost, error) {
	if mock.IsMockEnabled() {
		// See ApplyHostReport: mock mode drops real push-based reports rather
		// than mixing real containers into the fixture fabric.
		return models.DockerHost{
			ID:          strings.TrimSpace(report.Agent.ID),
			Hostname:    strings.TrimSpace(report.Host.Hostname),
			DisplayName: strings.TrimSpace(report.Host.Name),
			LastSeen:    report.Timestamp,
		}, nil
	}

	readState := m.snapshotBackedUnifiedReadState()
	var dockerHosts []*unifiedresources.DockerHostView
	if readState != nil {
		dockerHosts = readState.DockerHosts()
	}
	identifier, legacyIDs, previous, hasPrevious := resolveDockerHostIdentifier(report, tokenRecord, dockerHosts)
	if strings.TrimSpace(identifier) == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing agent identifier")
	}

	// Check if this host was deliberately removed - reject report to prevent
	// resurrection. Both stores must be consulted: the in-memory map resets on
	// restart while the persisted entry keeps blocking (#1581), so a restart
	// must not silently lift a deliberate block.
	blockedID, removedAt, wasRemoved := m.lookupRemovedDockerHost(append([]string{identifier}, legacyIDs...))

	if wasRemoved && tokenRecord != nil &&
		!tokenRecord.CreatedAt.IsZero() && tokenRecord.CreatedAt.After(removedAt) {
		// A token minted after the host was removed means the user generated a
		// fresh install command for this machine: that is explicit re-enroll
		// intent, so clear the block instead of rejecting until the TTL
		// expires or Allow reconnect is clicked. A still-running old agent
		// keeps presenting its pre-removal token and stays blocked. This
		// mirrors the host-agent rule from #1581.
		if err := m.AllowDockerHostReenroll(blockedID); err != nil {
			log.Warn().
				Err(err).
				Str("dockerHostID", blockedID).
				Msg("Failed to clear Docker host removal block for fresh re-enroll token; report remains blocked")
		} else {
			log.Info().
				Str("dockerHostID", blockedID).
				Time("removedAt", removedAt).
				Time("tokenCreatedAt", tokenRecord.CreatedAt).
				Msg("Cleared Docker host removal block: report presented a token created after removal")
			wasRemoved = false
		}
	}

	if wasRemoved {
		log.Info().
			Str("dockerHostID", identifier).
			Time("removedAt", removedAt).
			Msg("Rejecting report from deliberately removed Docker host")
		return models.DockerHost{}, fmt.Errorf("docker host %q had monitoring stopped at %v and cannot report again. Use Allow reconnect in Settings -> Infrastructure or rerun the installer with a docker:manage token to clear this block", identifier, removedAt.Format(time.RFC3339))
	}

	// Enforce token uniqueness: each token can only be bound to one Docker host identity.
	if tokenRecord != nil && tokenRecord.ID != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		agentID, tokenBindingAliases := resolveDockerTokenBindingIdentity(identifier, report, previous, hasPrevious)

		m.mu.Lock()
		if boundAgentID, exists := m.dockerTokenBindings[tokenID]; exists {
			if !dockerTokenBindingMatches(boundAgentID, tokenBindingAliases) {
				m.mu.Unlock()
				// Find the conflicting host to provide helpful error message
				conflictingHostname := "unknown"
				for _, host := range dockerHosts {
					if host == nil {
						continue
					}
					hostSourceID := strings.TrimSpace(host.HostSourceID())
					if host.AgentID() == boundAgentID || hostSourceID == boundAgentID || host.ID() == boundAgentID {
						conflictingHostname = strings.TrimSpace(host.Name())
						if conflictingHostname == "" {
							conflictingHostname = strings.TrimSpace(host.Hostname())
						}
						break
					}
				}
				tokenHint := tokenHintFromRecord(tokenRecord)
				if tokenHint != "" {
					tokenHint = " (" + tokenHint + ")"
				}
				log.Warn().
					Str("tokenID", tokenID).
					Str("tokenHint", tokenHint).
					Str("reportingAgentID", agentID).
					Str("boundAgentID", boundAgentID).
					Str("conflictingHost", conflictingHostname).
					Msg("Rejecting Docker report: token already bound to different agent")
				return models.DockerHost{}, fmt.Errorf("API token%s is already in use by agent %q (host: %s). Each Docker / Podman module must use a unique API token. Generate a new token for this agent", tokenHint, boundAgentID, conflictingHostname)
			}
			if boundAgentID != agentID {
				m.dockerTokenBindings[tokenID] = agentID
			}
		} else {
			// First time seeing this token - bind it to this agent
			m.dockerTokenBindings[tokenID] = agentID
			log.Debug().
				Str("tokenID", tokenID).
				Str("agentID", agentID).
				Str("hostname", report.Host.Hostname).
				Msg("Bound Docker / Podman module token to host identity")
		}
		m.mu.Unlock()
	}

	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing hostname")
	}

	receivedAt := time.Now()
	observedAt := receivedAt.UTC()

	agentID := strings.TrimSpace(report.Agent.ID)
	if agentID == "" {
		agentID = identifier
	}

	displayName := strings.TrimSpace(report.Host.Name)
	if displayName == "" {
		displayName = hostname
	}

	runtime := strings.ToLower(strings.TrimSpace(report.Host.Runtime))
	switch runtime {
	case "", "auto", "default":
		runtime = "docker"
	case "docker", "podman":
		// supported runtimes
	default:
		runtime = "docker"
	}

	runtimeVersion := strings.TrimSpace(report.Host.RuntimeVersion)
	dockerVersion := strings.TrimSpace(report.Host.DockerVersion)
	if runtimeVersion == "" {
		runtimeVersion = dockerVersion
	}
	if dockerVersion == "" {
		dockerVersion = runtimeVersion
	}

	containers := make([]models.DockerContainer, 0, len(report.Containers))
	for _, payload := range report.Containers {
		container := models.DockerContainer{
			ID:                 payload.ID,
			Name:               payload.Name,
			Image:              payload.Image,
			ImageDigest:        payload.ImageDigest,
			State:              payload.State,
			Status:             payload.Status,
			Health:             payload.Health,
			HealthcheckTargets: append([]string(nil), payload.HealthcheckTargets...),
			CPUPercent:         safeFloat(payload.CPUPercent),
			MemoryUsage:        payload.MemoryUsageBytes,
			MemoryLimit:        payload.MemoryLimitBytes,
			MemoryPercent:      safeFloat(payload.MemoryPercent),
			UptimeSeconds:      payload.UptimeSeconds,
			RestartCount:       payload.RestartCount,
			ExitCode:           payload.ExitCode,
			OOMKilled:          cloneReportBoolPtr(payload.OOMKilled),
			CreatedAt:          payload.CreatedAt,
			StartedAt:          payload.StartedAt,
			FinishedAt:         payload.FinishedAt,
			NetworkRXBytes:     payload.NetworkRXBytes,
			NetworkTXBytes:     payload.NetworkTXBytes,
		}
		container.CPUCapacityPercent = models.DockerContainerCPUCapacityPercent(container, report.Host.TotalCPU)

		// Copy update status if provided by agent
		if payload.UpdateStatus != nil {
			container.UpdateStatus = &models.DockerContainerUpdateStatus{
				UpdateAvailable: payload.UpdateStatus.UpdateAvailable,
				CurrentDigest:   payload.UpdateStatus.CurrentDigest,
				LatestDigest:    payload.UpdateStatus.LatestDigest,
				LastChecked:     payload.UpdateStatus.LastChecked,
				Error:           payload.UpdateStatus.Error,
			}
		}

		if len(payload.Ports) > 0 {
			ports := make([]models.DockerContainerPort, len(payload.Ports))
			for i, port := range payload.Ports {
				ports[i] = models.DockerContainerPort{
					PrivatePort: port.PrivatePort,
					PublicPort:  port.PublicPort,
					Protocol:    port.Protocol,
					IP:          port.IP,
				}
			}
			container.Ports = ports
		}

		if len(payload.Labels) > 0 {
			labels := make(map[string]string, len(payload.Labels))
			for k, v := range payload.Labels {
				labels[k] = v
			}
			container.Labels = labels
		}

		if len(payload.Networks) > 0 {
			networks := make([]models.DockerContainerNetworkLink, len(payload.Networks))
			for i, net := range payload.Networks {
				networks[i] = models.DockerContainerNetworkLink{
					Name: net.Name,
					IPv4: net.IPv4,
					IPv6: net.IPv6,
				}
			}
			container.Networks = networks
		}

		container.WritableLayerBytes = payload.WritableLayerBytes
		container.RootFilesystemBytes = payload.RootFilesystemBytes

		if payload.BlockIO != nil {
			container.BlockIO = &models.DockerContainerBlockIO{
				ReadBytes:  payload.BlockIO.ReadBytes,
				WriteBytes: payload.BlockIO.WriteBytes,
			}
		}

		containerIdentifier := payload.ID
		if strings.TrimSpace(containerIdentifier) == "" {
			containerIdentifier = payload.Name
		}
		if strings.TrimSpace(containerIdentifier) != "" {
			metrics := models.IOMetrics{
				NetworkIn:  clampToInt64(payload.NetworkRXBytes),
				NetworkOut: clampToInt64(payload.NetworkTXBytes),
				Timestamp:  receivedAt,
			}
			if payload.BlockIO != nil {
				metrics.DiskRead = clampToInt64(payload.BlockIO.ReadBytes)
				metrics.DiskWrite = clampToInt64(payload.BlockIO.WriteBytes)
			}

			readRate, writeRate, netInRate, netOutRate := m.rateTracker.CalculateRates(
				fmt.Sprintf("docker:%s:%s", identifier, containerIdentifier),
				metrics,
			)

			if container.BlockIO != nil && readRate >= 0 {
				value := readRate
				container.BlockIO.ReadRateBytesPerSecond = &value
			}
			if container.BlockIO != nil && writeRate >= 0 {
				value := writeRate
				container.BlockIO.WriteRateBytesPerSecond = &value
			}
			if netInRate >= 0 {
				container.NetInRate = netInRate
			}
			if netOutRate >= 0 {
				container.NetOutRate = netOutRate
			}
		}

		if len(payload.Mounts) > 0 {
			mounts := make([]models.DockerContainerMount, len(payload.Mounts))
			for i, mount := range payload.Mounts {
				mounts[i] = models.DockerContainerMount{
					Type:        mount.Type,
					Source:      mount.Source,
					Destination: mount.Destination,
					Mode:        mount.Mode,
					RW:          mount.RW,
					Propagation: mount.Propagation,
					Name:        mount.Name,
					Driver:      mount.Driver,
				}
			}
			container.Mounts = mounts
		}

		containers = append(containers, container)
	}

	images := convertDockerImages(report.Images)
	volumes := convertDockerVolumes(report.Volumes)
	networks := convertDockerNetworks(report.Networks)
	services := convertDockerServices(report.Services)
	tasks := convertDockerTasks(report.Tasks)
	nodes := convertDockerNodes(report.Nodes)
	secrets := convertDockerSecrets(report.Secrets)
	configs := convertDockerConfigs(report.Configs)
	storageUsage := convertDockerStorageUsage(report.StorageUsage)
	swarmInfo := convertDockerSwarmInfo(report.Host.Swarm)
	security := deriveDockerHostSecurity(report.Host.Security, runtime)

	loadAverage := make([]float64, 0, len(report.Host.LoadAverage))
	if len(report.Host.LoadAverage) > 0 {
		loadAverage = append(loadAverage, report.Host.LoadAverage...)
	}

	memory := normalizeAgentMemory(
		report.Host.Memory.TotalBytes,
		report.Host.Memory.UsedBytes,
		report.Host.Memory.FreeBytes,
		report.Host.Memory.CacheBytes,
		report.Host.Memory.Usage,
		report.Host.Memory.SwapTotal,
		report.Host.Memory.SwapUsed,
	)
	// Fallback: if gopsutil's memory reading failed but Docker's TotalMemoryBytes
	// is valid (possibly already a fallback from the agent), use that for Total.
	// This handles Docker-in-LXC scenarios where both Docker and gopsutil may
	// fail to read memory stats, but the agent fix provides a valid fallback.
	if memory.Total <= 0 && report.Host.TotalMemoryBytes > 0 {
		memory = models.UnavailableMemory(report.Host.TotalMemoryBytes)
	}

	disks := make([]models.Disk, 0, len(report.Host.Disks))
	for _, disk := range report.Host.Disks {
		// Filter virtual/system filesystems (same as ApplyHostReport) to avoid
		// inflated disk totals from tmpfs, overlayfs, etc.
		if shouldSkip, _ := fsfilters.ShouldSkipFilesystem(disk.Type, disk.Mountpoint, uint64(disk.TotalBytes), uint64(disk.UsedBytes)); shouldSkip {
			continue
		}
		disks = append(disks, models.Disk{
			Total:      disk.TotalBytes,
			Used:       disk.UsedBytes,
			Free:       disk.FreeBytes,
			Usage:      safeFloat(disk.Usage),
			Mountpoint: disk.Mountpoint,
			Type:       disk.Type,
			Device:     disk.Device,
		})
	}

	networkIfaces := make([]models.HostNetworkInterface, 0, len(report.Host.Network))
	for _, iface := range report.Host.Network {
		addresses := append([]string(nil), iface.Addresses...)
		networkIfaces = append(networkIfaces, models.HostNetworkInterface{
			Name:      iface.Name,
			MAC:       iface.MAC,
			Addresses: addresses,
			RXBytes:   iface.RXBytes,
			TXBytes:   iface.TXBytes,
			SpeedMbps: iface.SpeedMbps,
		})
	}

	agentVersion := normalizeAgentVersion(report.Agent.Version)
	if agentVersion == "" && hasPrevious {
		agentVersion = normalizeAgentVersion(previous.AgentVersion())
	}

	// Detect distinct machines flapping under one identity (e.g. cloned VMs
	// sharing /etc/machine-id, #1584) so the UI can warn instead of silently
	// letting the reports overwrite each other.
	identityConflict := m.trackDockerHostIdentity(identifier, hostname, strings.TrimSpace(report.Host.MachineID), receivedAt)
	if identityConflict != nil {
		log.Warn().
			Str("dockerHostID", identifier).
			Strs("hostnames", identityConflict.Hostnames).
			Strs("machineIDs", identityConflict.MachineIDs).
			Msg("Multiple machines appear to report under one Docker host identity (cloned VMs sharing /etc/machine-id?)")
	}

	host := models.DockerHost{
		ID:                identifier,
		AgentID:           agentID,
		Hostname:          hostname,
		DisplayName:       displayName,
		MachineID:         strings.TrimSpace(report.Host.MachineID),
		OS:                report.Host.OS,
		KernelVersion:     report.Host.KernelVersion,
		Architecture:      report.Host.Architecture,
		Runtime:           runtime,
		RuntimeVersion:    runtimeVersion,
		DockerVersion:     dockerVersion,
		CollectionMode:    strings.TrimSpace(report.Host.CollectionMode),
		CPUs:              report.Host.TotalCPU,
		TotalMemoryBytes:  report.Host.TotalMemoryBytes,
		UptimeSeconds:     report.Host.UptimeSeconds,
		CPUUsage:          safeFloat(report.Host.CPUUsagePercent),
		LoadAverage:       loadAverage,
		Memory:            memory,
		Disks:             disks,
		NetworkInterfaces: networkIfaces,
		Status:            "online",
		LastSeen:          observedAt,
		IntervalSeconds:   report.Agent.IntervalSeconds,
		AgentVersion:      agentVersion,
		Containers:        containers,
		Images:            images,
		Volumes:           volumes,
		Networks:          networks,
		Services:          services,
		Tasks:             tasks,
		Nodes:             nodes,
		Secrets:           secrets,
		Configs:           configs,
		StorageUsage:      storageUsage,
		Swarm:             swarmInfo,
		Security:          security,
		IsLegacy:          isLegacyAgent(report.Agent.Type),
		IdentityConflict:  identityConflict,
	}

	if hasPrevious {
		m.migrateDockerContainerMetadataForRenamedContainers(identifier, previous.Containers(), host.Containers)
		m.migrateDockerContainerMetadataForRecreatedContainers(identifier, previous.Containers(), host.Containers)
	}
	m.migrateCurrentDockerContainerMetadataToStableIdentities(identifier, host.Containers)

	if tokenRecord != nil {
		host.TokenID = tokenRecord.ID
		host.TokenName = tokenRecord.Name
		host.TokenHint = tokenHintFromRecord(tokenRecord)
		if tokenRecord.LastUsedAt != nil {
			t := tokenRecord.LastUsedAt.UTC()
			host.TokenLastUsedAt = &t
		} else {
			t := time.Now().UTC()
			host.TokenLastUsedAt = &t
		}
	} else if hasPrevious {
		host.TokenID = previous.TokenID()
		host.TokenName = previous.TokenName()
		host.TokenHint = previous.TokenHint()
		host.TokenLastUsedAt = previous.TokenLastUsedAt()
	}

	// Load custom display name from metadata store if not already set
	if host.CustomDisplayName == "" {
		if hostMeta := m.dockerMetadataStore.GetHostMetadata(identifier); hostMeta != nil {
			host.CustomDisplayName = hostMeta.CustomDisplayName
		}
	}

	m.state.UpsertDockerHost(host)
	m.state.SetConnectionHealth(dockerConnectionPrefix+host.ID, true)

	m.supersedeStaleDockerHostDuplicates(host, tokenRecord, dockerHosts)

	// Check if the host was previously hidden and is now visible again
	if hasPrevious && previous.Hidden() && !host.Hidden {
		log.Info().
			Str("dockerHost", host.Hostname).
			Str("dockerHostID", host.ID).
			Msg("Docker host auto-unhidden after receiving report")
	}

	// Check if the host was pending uninstall - if so, log a warning that uninstall failed and clear the flag
	if hasPrevious && previous.PendingUninstall() {
		log.Warn().
			Str("dockerHost", host.Hostname).
			Str("dockerHostID", host.ID).
			Msg("Docker host reporting again after pending uninstall - uninstall may have failed")

		// Clear the pending uninstall flag since the host is clearly still active
		m.state.SetDockerHostPendingUninstall(host.ID, false)
	}

	if m.alertManager != nil {
		m.alertManager.CheckDockerHost(host)
	}

	// Record Docker HOST and CONTAINER metrics for sparkline charts unless the
	// canonical mock sampler owns history continuity.
	if !shouldSkipNativeMockStateMetricWrites() {
		now := time.Now()
		hostMetricKey := fmt.Sprintf("dockerHost:%s", host.ID)

		// Record host Disk usage (use first disk or calculate total)
		var hostDiskPercent float64
		if len(host.Disks) > 0 {
			hostDiskPercent = host.Disks[0].Usage
		}

		if m.metricsHistory != nil {
			m.metricsHistory.AddGuestMetric(hostMetricKey, "cpu", host.CPUUsage, now)
			if host.Memory.HasKnownUsage() {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "memory", host.Memory.Usage, now)
			}
			m.metricsHistory.AddGuestMetric(hostMetricKey, "disk", hostDiskPercent, now)
		}

		if m.metricsStore != nil {
			m.metricsStore.Write("dockerHost", host.ID, "cpu", host.CPUUsage, now)
			if host.Memory.HasKnownUsage() {
				m.metricsStore.Write("dockerHost", host.ID, "memory", host.Memory.Usage, now)
			}
			m.metricsStore.Write("dockerHost", host.ID, "disk", hostDiskPercent, now)
		}

		// Use a prefixed key (docker:containerID) to distinguish from Proxmox containers.
		for _, container := range containers {
			if container.ID == "" {
				continue
			}
			metricKey := fmt.Sprintf("docker:%s", container.ID)

			var diskPercent float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				diskPercent = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				if diskPercent > 100 {
					diskPercent = 100
				}
			}

			var diskReadRate float64
			var diskWriteRate float64
			if container.BlockIO != nil {
				if container.BlockIO.ReadRateBytesPerSecond != nil {
					diskReadRate = *container.BlockIO.ReadRateBytesPerSecond
				}
				if container.BlockIO.WriteRateBytesPerSecond != nil {
					diskWriteRate = *container.BlockIO.WriteRateBytesPerSecond
				}
			}

			if m.metricsHistory != nil {
				m.metricsHistory.AddGuestMetric(metricKey, "cpu", models.DockerContainerCPUCapacityPercent(container, host.CPUs), now)
				m.metricsHistory.AddGuestMetric(metricKey, "memory", container.MemoryPercent, now)
				m.metricsHistory.AddGuestMetric(metricKey, "disk", diskPercent, now)
				if container.NetInRate >= 0 {
					m.metricsHistory.AddGuestMetric(metricKey, "netin", container.NetInRate, now)
				}
				if container.NetOutRate >= 0 {
					m.metricsHistory.AddGuestMetric(metricKey, "netout", container.NetOutRate, now)
				}
				if diskReadRate >= 0 {
					m.metricsHistory.AddGuestMetric(metricKey, "diskread", diskReadRate, now)
				}
				if diskWriteRate >= 0 {
					m.metricsHistory.AddGuestMetric(metricKey, "diskwrite", diskWriteRate, now)
				}
			}

			if m.metricsStore != nil {
				m.metricsStore.Write("dockerContainer", container.ID, "cpu", models.DockerContainerCPUCapacityPercent(container, host.CPUs), now)
				m.metricsStore.Write("dockerContainer", container.ID, "memory", container.MemoryPercent, now)
				m.metricsStore.Write("dockerContainer", container.ID, "disk", diskPercent, now)
				if container.NetInRate >= 0 {
					m.metricsStore.Write("dockerContainer", container.ID, "netin", container.NetInRate, now)
				}
				if container.NetOutRate >= 0 {
					m.metricsStore.Write("dockerContainer", container.ID, "netout", container.NetOutRate, now)
				}
				if diskReadRate >= 0 {
					m.metricsStore.Write("dockerContainer", container.ID, "diskread", diskReadRate, now)
				}
				if diskWriteRate >= 0 {
					m.metricsStore.Write("dockerContainer", container.ID, "diskwrite", diskWriteRate, now)
				}
			}
		}
	}

	log.Debug().
		Str("dockerHost", host.Hostname).
		Int("containers", len(containers)).
		Msg("Docker host report processed")

	m.refreshUnifiedResourceStoreAfterAgentStateChange()

	return host, nil
}

const dockerAuthorizationPluginBlockReasonFormat = "Pulse blocks Docker daemon-mutating commands while Docker authorization plugins are configured (%s) because advisory GO-2026-4887 does not yet provide a fixed Docker Go module line."

func deriveDockerHostSecurity(raw *agentsdocker.HostSecurityInfo, runtime string) *models.DockerHostSecurity {
	authzPlugins := normalizedSecurityStrings(nil)
	if raw != nil {
		authzPlugins = normalizedSecurityStrings(raw.AuthorizationPlugins)
	}
	if len(authzPlugins) == 0 {
		return nil
	}

	security := &models.DockerHostSecurity{
		AuthorizationPlugins: authzPlugins,
	}
	if strings.EqualFold(strings.TrimSpace(runtime), "docker") {
		security.MutatingCommandsBlocked = true
		security.MutatingCommandsBlockedReason = fmt.Sprintf(
			dockerAuthorizationPluginBlockReasonFormat,
			strings.Join(authzPlugins, ", "),
		)
	}
	return security
}

func normalizedSecurityStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// staleHostIdentityForReenrollment recognizes explicit reinstall intent before
// a new token is bound. A stopped agent may send a final in-flight report just
// after the replacement token is minted, so allow one of its own health windows
// of overlap. The supported install issuer, stable machine/hostname, report-IP,
// and identity-conflict checks prevent an arbitrary token or visible clone from
// taking over a live host.
func staleHostIdentityForReenrollment(report agentshost.Report, tokenRecord *config.APITokenRecord, hosts []models.Host) string {
	if tokenRecord == nil || tokenRecord.CreatedAt.IsZero() {
		return ""
	}
	machineID := strings.TrimSpace(report.Host.MachineID)
	hostname := strings.TrimSpace(report.Host.Hostname)
	reportIP := strings.TrimSpace(report.Host.ReportIP)
	tokenID := strings.TrimSpace(tokenRecord.ID)
	if machineID == "" || hostname == "" || tokenID == "" {
		return ""
	}
	if !agentbinding.CanBindInstallToken(tokenRecord, machineID, hostname) {
		return ""
	}

	var candidateID string
	for _, candidate := range hosts {
		if strings.TrimSpace(candidate.MachineID) != machineID ||
			!unifiedresources.HostnamesEquivalent(candidate.Hostname, hostname) ||
			strings.TrimSpace(candidate.TokenID) == tokenID ||
			candidate.IdentityConflict != nil {
			continue
		}
		candidateReportIP := strings.TrimSpace(candidate.ReportIP)
		if reportIP != "" && candidateReportIP != "" && !strings.EqualFold(reportIP, candidateReportIP) {
			continue
		}
		if candidate.LastSeen.After(tokenRecord.CreatedAt.Add(hostAgentHealthWindow(candidate.IntervalSeconds))) {
			continue
		}
		matchedID := strings.TrimSpace(candidate.ID)
		if matchedID == "" {
			continue
		}
		if candidateID != "" && candidateID != matchedID {
			return ""
		}
		candidateID = matchedID
	}
	return candidateID
}

func (m *Monitor) retireSupersededHostTokenBindings(hostID, currentTokenID string) {
	hostID = strings.TrimSpace(hostID)
	currentTokenID = strings.TrimSpace(currentTokenID)
	if hostID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, boundID := range m.hostTokenBindings {
		if strings.TrimSpace(boundID) != hostID {
			continue
		}
		if key == currentTokenID || strings.HasPrefix(key, currentTokenID+":") {
			continue
		}
		delete(m.hostTokenBindings, key)
	}
}

// hostRenameHealSource returns the host record that keeps a renamed machine's
// identity forked: same host ID, same reporting token, same machine ID, a
// different hostname, and no report for several health windows. A cloned VM
// sharing a machine ID (#1584) keeps reporting under the old hostname, so it
// never matches; only a machine that stopped reporting under its previous
// hostname does (#1667).
func hostRenameHealSource(hosts []models.Host, id, tokenID, machineID, hostname string, now time.Time) (models.Host, bool) {
	id = strings.TrimSpace(id)
	tokenID = strings.TrimSpace(tokenID)
	machineID = strings.TrimSpace(machineID)
	if id == "" || tokenID == "" || machineID == "" {
		return models.Host{}, false
	}
	for _, candidate := range hosts {
		if strings.TrimSpace(candidate.ID) != id {
			continue
		}
		if strings.TrimSpace(candidate.TokenID) != tokenID ||
			strings.TrimSpace(candidate.MachineID) != machineID ||
			hostAgentHostnamesMatch(candidate.Hostname, hostname) {
			return models.Host{}, false
		}
		if candidate.LastSeen.IsZero() ||
			now.Sub(candidate.LastSeen) > 3*hostAgentHealthWindow(candidate.IntervalSeconds) {
			return candidate, true
		}
		return models.Host{}, false
	}
	return models.Host{}, false
}

// removeSupersededHostRecord removes a host record that a live identity has
// superseded, along with its bindings, continuity, and tracking state. This is
// an internal supersession, not a user removal: no tombstone, no token
// revocation.
func (m *Monitor) removeSupersededHostRecord(staleID, replacementID, reason string) bool {
	staleID = strings.TrimSpace(staleID)
	if staleID == "" {
		return false
	}
	removed, ok := m.state.RemoveHost(staleID)
	if !ok {
		return false
	}
	m.state.RemoveConnectionHealth(hostConnectionPrefix + staleID)
	m.state.UnlinkNodesFromHostAgent(staleID)
	m.clearAgentLXCFilesystems(staleID)

	m.mu.Lock()
	for key, boundID := range m.hostTokenBindings {
		if strings.TrimSpace(boundID) == staleID {
			delete(m.hostTokenBindings, key)
		}
	}
	m.clearHostAgentIdentityTrackingLocked(staleID)
	m.mu.Unlock()

	m.hostReportOrderMu.Lock()
	delete(m.hostReportOrders, staleID)
	m.hostReportOrderMu.Unlock()

	if m.hostContinuityStore != nil {
		if err := m.hostContinuityStore.Delete(staleID); err != nil {
			log.Warn().Err(err).Str("staleHostID", staleID).Msg("Failed to remove superseded host continuity entry")
		}
	}
	if m.alertManager != nil {
		m.alertManager.HandleHostRemoved(removed)
		m.SyncAlertState()
	}

	log.Info().
		Str("host", removed.Hostname).
		Str("staleHostID", staleID).
		Str("replacementHostID", replacementID).
		Msg(reason)
	return true
}

// supersedeStaleHostAgentDuplicates removes older generations left behind by
// installs completed before this cleanup existed. This is an internal
// supersession, not a user removal: it deliberately creates no tombstone and
// does not revoke either token.
func (m *Monitor) supersedeStaleHostAgentDuplicates(current models.Host, tokenRecord *config.APITokenRecord, hosts []models.Host) {
	if tokenRecord == nil || tokenRecord.CreatedAt.IsZero() {
		return
	}
	machineID := strings.TrimSpace(current.MachineID)
	hostname := strings.TrimSpace(current.Hostname)
	if machineID == "" || hostname == "" {
		return
	}

	for _, stale := range hosts {
		staleID := strings.TrimSpace(stale.ID)
		if staleID == "" ||
			strings.TrimSpace(stale.MachineID) != machineID ||
			!unifiedresources.HostnamesEquivalent(stale.Hostname, hostname) ||
			!stale.LastSeen.Before(tokenRecord.CreatedAt) {
			continue
		}

		if staleID == strings.TrimSpace(current.ID) {
			// The fresh install reused the physical host's stable ID. Remove
			// bindings for its pre-install token so the stopped old agent
			// cannot later overwrite the replacement through that same ID.
			currentTokenID := strings.TrimSpace(current.TokenID)
			m.mu.Lock()
			for key, boundID := range m.hostTokenBindings {
				if strings.TrimSpace(boundID) != staleID {
					continue
				}
				if key == currentTokenID || strings.HasPrefix(key, currentTokenID+":") {
					continue
				}
				delete(m.hostTokenBindings, key)
			}
			m.mu.Unlock()
			continue
		}

		m.removeSupersededHostRecord(staleID, current.ID, "Superseded stale host agent record after re-enrollment")
	}
}

// ApplyHostReport ingests a host agent report into the shared state.
func (m *Monitor) ApplyHostReport(report agentshost.Report, tokenRecord *config.APITokenRecord) (models.Host, error) {
	if mock.IsMockEnabled() {
		// Mock mode suspends real pull-based collection outright (PVE/PBS/PMG
		// clients are never built), and push-based agent reports get the same
		// treatment. Ingesting them would land a real machine in state, where
		// it raises alerts, persists host continuity, records metrics, and
		// feeds the online/offline sweep, all beside fixture data. The report
		// is acknowledged rather than rejected so a real agent does not read a
		// demo server as an outage and retry-storm it.
		return mockDiscardedHostReportAck(report), nil
	}

	m.hostAgentLifecycleMu.RLock()
	defer m.hostAgentLifecycleMu.RUnlock()

	receivedAt := time.Now()
	observedAt := receivedAt.UTC()

	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		return models.Host{}, fmt.Errorf("host report missing hostname")
	}

	baseIdentifier := strings.TrimSpace(report.Host.ID)
	if baseIdentifier != "" {
		baseIdentifier = sanitizeDockerHostSuffix(baseIdentifier)
	}
	if baseIdentifier == "" {
		if machine := sanitizeDockerHostSuffix(report.Host.MachineID); machine != "" {
			baseIdentifier = machine
		}
	}
	if baseIdentifier == "" {
		if agentID := sanitizeDockerHostSuffix(report.Agent.ID); agentID != "" {
			baseIdentifier = agentID
		}
	}
	if baseIdentifier == "" {
		if hostName := sanitizeDockerHostSuffix(hostname); hostName != "" {
			baseIdentifier = hostName
		}
	}
	if baseIdentifier == "" {
		seedParts := uniqueNonEmptyStrings(
			report.Host.MachineID,
			report.Agent.ID,
			report.Host.Hostname,
		)
		if len(seedParts) == 0 {
			seedParts = []string{hostname}
		}
		seed := strings.Join(seedParts, "|")
		sum := sha1.Sum([]byte(seed))
		baseIdentifier = fmt.Sprintf("agent-%s", hex.EncodeToString(sum[:6]))
	}
	if persisted, ok := m.matchPersistedHostContinuity(report, tokenRecord); ok {
		if hostContinuityDeniesToken(persisted, tokenRecord) {
			return models.Host{}, fmt.Errorf(
				"API token %q was explicitly detached from host agent %q during removal and cannot report for that identity",
				strings.TrimSpace(tokenRecord.ID),
				strings.TrimSpace(persisted.HostID),
			)
		}
		baseIdentifier = strings.TrimSpace(persisted.HostID)
	}

	readState := m.snapshotBackedUnifiedReadState()
	var existingHosts []*unifiedresources.HostView
	if readState != nil {
		existingHosts = readState.Hosts()
	}
	existingHostModels := m.state.GetHosts()

	identifier := baseIdentifier
	if tokenRecord != nil && strings.TrimSpace(tokenRecord.ID) != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		bindingKey := hostTokenBindingKey(tokenID, hostname)

		m.mu.Lock()
		if m.hostTokenBindings == nil {
			m.hostTokenBindings = make(map[string]string)
		}
		boundID := lookupHostTokenBinding(m.hostTokenBindings, tokenID, hostname)
		if boundID != "" {
			m.hostTokenBindings[bindingKey] = boundID
		}
		m.mu.Unlock()

		// If we already have a binding for this token+hostname, use it to keep host IDs stable
		// even if another colliding host disappears later.
		if boundID != "" {
			identifier = boundID
			// A machine renamed while its old record still existed was forked
			// onto a suffixed identity to stay safe against cloned VMs
			// sharing a machine ID (#1584). Once the pre-rename record has
			// clearly stopped reporting, the collision is proven to be a
			// rename, not a clone, so heal back to the base identity that
			// workload modules still report under (#1667).
			if boundID != baseIdentifier {
				if stale, ok := hostRenameHealSource(existingHostModels, baseIdentifier, tokenID, strings.TrimSpace(report.Host.MachineID), hostname, receivedAt); ok {
					m.removeSupersededHostRecord(stale.ID, baseIdentifier, "Superseded stale pre-rename host record to heal forked identity")
					m.removeSupersededHostRecord(boundID, baseIdentifier, "Retired forked host identity after rename heal")
					m.mu.Lock()
					m.hostTokenBindings[bindingKey] = baseIdentifier
					m.mu.Unlock()
					identifier = baseIdentifier
				}
			}
		} else {
			bindingID := baseIdentifier
			reusedStaleID := staleHostIdentityForReenrollment(report, tokenRecord, existingHostModels)
			if reusedStaleID != "" {
				bindingID = reusedStaleID
			}
			for _, candidate := range existingHosts {
				if candidate == nil || candidate.AgentID() != bindingID {
					continue
				}
				if reusedStaleID == bindingID {
					break
				}
				if hostAgentHostnamesMatch(candidate.Hostname(), hostname) && strings.TrimSpace(candidate.TokenID()) == tokenID {
					break
				}

				// A colliding record that stopped reporting under its old
				// hostname is a rename, not a clone: supersede it and keep
				// the base identity instead of forking (#1667).
				if stale, ok := hostRenameHealSource(existingHostModels, bindingID, tokenID, strings.TrimSpace(report.Host.MachineID), hostname, receivedAt); ok {
					m.removeSupersededHostRecord(stale.ID, bindingID, "Superseded stale pre-rename host record instead of forking identity")
					break
				}

				seed := strings.Join([]string{tokenID, hostname, bindingID}, "|")
				sum := sha1.Sum([]byte(seed))
				suffix := hex.EncodeToString(sum[:4])

				base := bindingID
				if base == "" {
					base = "agent"
				}
				if len(base) > 40 {
					base = base[:40]
				}
				bindingID = fmt.Sprintf("%s-%s", base, suffix)
				break
			}

			m.mu.Lock()
			if m.hostTokenBindings == nil {
				m.hostTokenBindings = make(map[string]string)
			}
			if existing := lookupHostTokenBinding(m.hostTokenBindings, tokenID, hostname); existing != "" {
				m.hostTokenBindings[bindingKey] = existing
				identifier = existing
			} else {
				m.hostTokenBindings[bindingKey] = bindingID
				log.Debug().
					Str("tokenID", tokenID).
					Str("hostID", bindingID).
					Str("hostname", hostname).
					Msg("Bound host agent token to hostname")
				identifier = bindingID
			}
			m.mu.Unlock()
			if reusedStaleID != "" {
				m.retireSupersededHostTokenBindings(reusedStaleID, tokenID)
			}
		}
	}

	tokenID := ""
	if tokenRecord != nil {
		tokenID = strings.TrimSpace(tokenRecord.ID)
	}
	blocked, wasRemoved := m.lookupRemovedHostAgent(identifier, hostname, report.Host.MachineID, tokenID)
	if wasRemoved && removedHostAgentAllowsFreshReenroll(blocked, identifier, report, tokenRecord) {
		// A token minted after the host was removed means the user generated a
		// fresh install command for this machine: that is explicit re-enroll
		// intent, so clear the block instead of rejecting until the TTL
		// expires. A still-running old agent keeps presenting its pre-removal
		// token and stays blocked (#1581).
		cleared, err := m.allowHostAgentReenrollLocked(blocked.ID, tokenID, false)
		if err == nil && cleared {
			identifier = strings.TrimSpace(blocked.ID)
			if tokenID != "" {
				m.mu.Lock()
				if m.hostTokenBindings == nil {
					m.hostTokenBindings = make(map[string]string)
				}
				m.hostTokenBindings[hostTokenBindingKey(tokenID, hostname)] = identifier
				m.mu.Unlock()
			}
			log.Info().
				Str("hostID", identifier).
				Str("blockedID", blocked.ID).
				Time("removedAt", blocked.RemovedAt).
				Time("tokenCreatedAt", tokenRecord.CreatedAt).
				Msg("Cleared host agent removal block: report presented a token created after removal")
			wasRemoved = false
		} else if err != nil {
			log.Warn().
				Err(err).
				Str("blockedID", blocked.ID).
				Msg("Failed to persist host agent re-enrollment allowance; report remains blocked")
		} else {
			log.Info().
				Str("blockedID", blocked.ID).
				Msg("Host agent re-enrollment transition was already consumed; report remains blocked")
		}
	}
	if wasRemoved {
		log.Info().
			Str("hostID", identifier).
			Time("removedAt", blocked.RemovedAt).
			Msg("Rejecting report from deliberately removed host agent")
		return models.Host{}, fmt.Errorf("host agent %q had monitoring stopped at %v and cannot report again. Re-enroll by reinstalling the agent with a newly generated API token, or wait for the block to clear 24 hours after removal", identifier, blocked.RemovedAt.Format(time.RFC3339))
	}

	unlockHostReport := m.lockHostReportApplication(identifier)
	defer unlockHostReport()

	// Identity resolution happens before taking the per-host lock. Refresh the
	// shared read model after waiting so token inheritance and previous-state
	// comparisons use the latest accepted report, not the pre-wait snapshot.
	readState = m.snapshotBackedUnifiedReadState()
	existingHosts = nil
	if readState != nil {
		existingHosts = readState.Hosts()
	}
	existingHostModels = m.state.GetHosts()

	reportOrder, accepted := m.reserveHostReportOrder(identifier, report, receivedAt)
	if !accepted {
		return m.applyRejectedHostReportLiveness(identifier, report, receivedAt, reportOrder), nil
	}

	var previous *unifiedresources.HostView
	var hasPrevious bool
	for _, candidate := range existingHosts {
		if candidate != nil && candidate.AgentID() == identifier {
			previous = candidate
			hasPrevious = true
			break
		}
	}

	displayName := strings.TrimSpace(report.Host.DisplayName)
	if displayName == "" {
		displayName = hostname
	}

	memory := normalizeAgentMemory(
		report.Metrics.Memory.TotalBytes,
		report.Metrics.Memory.UsedBytes,
		report.Metrics.Memory.FreeBytes,
		report.Metrics.Memory.CacheBytes,
		report.Metrics.Memory.Usage,
		report.Metrics.Memory.SwapTotal,
		report.Metrics.Memory.SwapUsed,
	)

	disks := make([]models.Disk, 0, len(report.Disks))
	hostZFSPools := hostZFSPoolsFromAgentDisks(report.Disks)
	for _, disk := range report.Disks {
		// Filter virtual/system filesystems and read-only filesystems to avoid cluttering
		// the UI with tmpfs, devtmpfs, /dev, /run, /sys, docker overlay mounts, snap mounts,
		// immutable OS images, etc. (issues #505, #690, #790).
		if shouldSkip, _ := fsfilters.ShouldSkipFilesystem(disk.Type, disk.Mountpoint, uint64(disk.TotalBytes), uint64(disk.UsedBytes)); shouldSkip {
			continue
		}

		usage := safeFloat(disk.Usage)
		if usage <= 0 && disk.TotalBytes > 0 {
			usage = safePercentage(float64(disk.UsedBytes), float64(disk.TotalBytes))
		}
		disks = append(disks, models.Disk{
			Total:      disk.TotalBytes,
			Used:       disk.UsedBytes,
			Free:       disk.FreeBytes,
			Usage:      usage,
			Mountpoint: disk.Mountpoint,
			Type:       disk.Type,
			Device:     disk.Device,
		})
	}

	diskIO := make([]models.DiskIO, 0, len(report.DiskIO))
	for _, io := range report.DiskIO {
		diskIO = append(diskIO, models.DiskIO{
			Device:     io.Device,
			ReadBytes:  io.ReadBytes,
			WriteBytes: io.WriteBytes,
			ReadOps:    io.ReadOps,
			WriteOps:   io.WriteOps,
			ReadTime:   io.ReadTime,
			WriteTime:  io.WriteTime,
			IOTime:     io.IOTime,
		})
	}

	network := make([]models.HostNetworkInterface, 0, len(report.Network))
	for _, nic := range report.Network {
		network = append(network, models.HostNetworkInterface{
			Name:      nic.Name,
			MAC:       nic.MAC,
			Addresses: append([]string(nil), nic.Addresses...),
			RXBytes:   nic.RXBytes,
			TXBytes:   nic.TXBytes,
			SpeedMbps: nic.SpeedMbps,
		})
	}

	raid := make([]models.HostRAIDArray, 0, len(report.RAID))
	for _, array := range report.RAID {
		devices := make([]models.HostRAIDDevice, 0, len(array.Devices))
		for _, dev := range array.Devices {
			devices = append(devices, models.HostRAIDDevice{
				Device: dev.Device,
				State:  dev.State,
				Slot:   dev.Slot,
			})
		}
		operation := strings.TrimSpace(array.Operation)
		rebuildPercent := safeFloat(array.RebuildPercent)
		rebuildSpeed := strings.TrimSpace(array.RebuildSpeed)
		if operation == "" {
			rebuildPercent = 0
			rebuildSpeed = ""
		}
		raid = append(raid, models.HostRAIDArray{
			Device:         array.Device,
			Name:           array.Name,
			Level:          array.Level,
			State:          array.State,
			TotalDevices:   array.TotalDevices,
			ActiveDevices:  array.ActiveDevices,
			WorkingDevices: array.WorkingDevices,
			FailedDevices:  array.FailedDevices,
			SpareDevices:   array.SpareDevices,
			UUID:           array.UUID,
			Devices:        devices,
			RebuildPercent: rebuildPercent,
			RebuildSpeed:   rebuildSpeed,
			Operation:      operation,
		})
	}

	// Convert Ceph data from agent report
	var cephData *models.HostCephCluster
	if report.Ceph != nil {
		cephData = convertAgentCephToModels(report.Ceph)
	}

	var unraidData *models.HostUnraidStorage
	if report.Unraid != nil {
		disks := make([]models.HostUnraidDisk, 0, len(report.Unraid.Disks))
		for _, disk := range report.Unraid.Disks {
			device := strings.TrimSpace(disk.Device)
			rawStatus := strings.TrimSpace(disk.RawStatus)
			status := strings.TrimSpace(disk.Status)
			if status == "" {
				status = normalizeLegacyUnraidDiskStatus(rawStatus, device)
			}
			if isLegacyUnraidEmptySlot(disk, status) {
				continue
			}
			disks = append(disks, models.HostUnraidDisk{
				Name:        strings.TrimSpace(disk.Name),
				Device:      device,
				Role:        strings.TrimSpace(disk.Role),
				Status:      status,
				RawStatus:   rawStatus,
				Model:       strings.TrimSpace(disk.Model),
				Serial:      strings.TrimSpace(disk.Serial),
				Filesystem:  strings.TrimSpace(disk.Filesystem),
				Transport:   strings.TrimSpace(disk.Transport),
				SizeBytes:   disk.SizeBytes,
				UsedBytes:   disk.UsedBytes,
				FreeBytes:   disk.FreeBytes,
				Temperature: disk.Temperature,
				SpunDown:    disk.SpunDown,
				ReadCount:   disk.ReadCount,
				WriteCount:  disk.WriteCount,
				ErrorCount:  disk.ErrorCount,
				Slot:        disk.Slot,
			})
		}
		syncAction := strings.TrimSpace(report.Unraid.SyncAction)
		syncProgress := safeFloat(report.Unraid.SyncProgress)
		if syncAction == "" {
			syncProgress = 0
		}
		numProtected, numDisabled, numInvalid, numMissing := reconcileReportedUnraidCounts(report.Unraid, disks)
		unraidData = &models.HostUnraidStorage{
			ArrayStarted: report.Unraid.ArrayStarted,
			ArrayState:   strings.TrimSpace(report.Unraid.ArrayState),
			SyncAction:   syncAction,
			SyncProgress: syncProgress,
			SyncErrors:   report.Unraid.SyncErrors,
			NumProtected: numProtected,
			NumDisabled:  numDisabled,
			NumInvalid:   numInvalid,
			NumMissing:   numMissing,
			Disks:        disks,
		}
	}

	var previousHostModel *models.Host
	for i := range existingHostModels {
		if existingHostModels[i].ID == identifier {
			previousHostModel = &existingHostModels[i]
			break
		}
	}
	libvirtData := m.normalizeAgentLibvirtInventory(
		report.Libvirt,
		identifier,
		previousHostModel,
		observedAt,
	)
	xcpngData := normalizeAgentXCPNGInventory(report.XCPNG, previousHostModel, observedAt)

	agentUpdate := mergeAgentUpdateStatus(
		previousHostAgentUpdate(m.state.GetHosts(), identifier),
		convertAgentUpdateStatus(report.Agent.Update),
		strings.TrimSpace(report.Agent.UpdatedFrom),
		observedAt,
	)

	// Detect distinct machines flapping under one identity (e.g. template
	// deployments still sharing /etc/machine-id, the host-agent shape of
	// #1584) so the UI can warn instead of silently letting the reports
	// overwrite each other.
	identityConflict := m.trackHostAgentIdentity(identifier, hostname, strings.TrimSpace(report.Host.ReportIP), receivedAt)
	if identityConflict != nil {
		log.Warn().
			Str("hostID", identifier).
			Strs("hostnames", identityConflict.Hostnames).
			Strs("reportIPs", identityConflict.ReportIPs).
			Msg("Multiple machines appear to report under one host agent identity (cloned machines sharing /etc/machine-id?)")
	}

	host := models.Host{
		ID:                identifier,
		Hostname:          hostname,
		DisplayName:       displayName,
		Platform:          platformsupport.NormalizeAgentReportedPlatform(report.Host.Platform),
		OSName:            strings.TrimSpace(report.Host.OSName),
		OSVersion:         strings.TrimSpace(report.Host.OSVersion),
		KernelVersion:     strings.TrimSpace(report.Host.KernelVersion),
		Architecture:      strings.TrimSpace(report.Host.Architecture),
		CPUCount:          report.Host.CPUCount,
		CPUUsage:          safeFloat(report.Metrics.CPUUsagePercent),
		LoadAverage:       append([]float64(nil), report.Host.LoadAverage...),
		Memory:            memory,
		Disks:             disks,
		ZFSPools:          hostZFSPools,
		DiskIO:            diskIO,
		NetworkInterfaces: network,
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: cloneStringFloatMap(report.Sensors.TemperatureCelsius),
			FanRPM:             cloneStringFloatMap(report.Sensors.FanRPM),
			PowerWatts:         cloneStringFloatMap(report.Sensors.PowerWatts),
			Additional:         cloneStringFloatMap(report.Sensors.Additional),
			Custom:             convertAgentCustomSensorsToModels(report.Sensors.Custom),
			GPU:                convertAgentGPUToModels(report.Sensors.GPU),
			ThermalState:       convertAgentThermalStateToModels(report.Sensors.ThermalState),
			SMART:              convertAgentSMARTToModels(report.Sensors.SMART),
		},
		RAID:                    raid,
		Unraid:                  unraidData,
		Ceph:                    cephData,
		Libvirt:                 libvirtData,
		XCPNG:                   xcpngData,
		Status:                  "online",
		UptimeSeconds:           report.Host.UptimeSeconds,
		IntervalSeconds:         report.Agent.IntervalSeconds,
		LastSeen:                observedAt,
		AgentVersion:            strings.TrimSpace(report.Agent.Version),
		MachineID:               strings.TrimSpace(report.Host.MachineID),
		CommandsEnabled:         report.Agent.CommandsEnabled,
		OperationReceiptVersion: report.Agent.OperationReceiptVersion,
		ReportIP:                strings.TrimSpace(report.Host.ReportIP),
		Tags:                    append([]string(nil), report.Tags...),
		DiskExclude:             append([]string(nil), report.Agent.DiskExclude...),
		AppliedConfig:           convertAgentConfigFingerprint(report.Agent.AppliedConfig),
		AgentUpdate:             agentUpdate,
		AgentModules:            convertAgentModuleStatuses(report.Agent.Modules),
		AgentPrivilege:          convertAgentPrivilegeStatus(report.Agent.Privilege),
		PackageUpdates:          convertHostPackageUpdateStatus(report.Host.PackageUpdates, observedAt),
		StorageCleanup:          convertHostStorageCleanupStatus(report.Host.StorageCleanup, observedAt),
		IsLegacy:                isLegacyAgent(report.Agent.Type),
		IdentityConflict:        identityConflict,
	}

	// Normalize vendor-managed internal RAID arrays out of host state so they do
	// not surface as customer-facing degraded storage in APIs, resources, or alerts.
	host.RAID = storagehealth.FilterVendorManagedSystemRAIDArrays(host, host.RAID)

	if len(host.LoadAverage) == 0 {
		host.LoadAverage = nil
	}
	if len(host.Disks) == 0 {
		host.Disks = nil
	}
	if len(host.ZFSPools) == 0 {
		host.ZFSPools = nil
	}
	if len(host.DiskIO) == 0 {
		host.DiskIO = nil
	}
	if len(host.NetworkInterfaces) == 0 {
		host.NetworkInterfaces = nil
	}
	if len(host.RAID) == 0 {
		host.RAID = nil
	}

	if tokenRecord != nil {
		host.TokenID = tokenRecord.ID
		host.TokenName = tokenRecord.Name
		host.TokenHint = tokenHintFromRecord(tokenRecord)
		if tokenRecord.LastUsedAt != nil {
			t := tokenRecord.LastUsedAt.UTC()
			host.TokenLastUsedAt = &t
		} else {
			now := time.Now().UTC()
			host.TokenLastUsedAt = &now
		}
	} else if hasPrevious {
		host.TokenID = previous.TokenID()
		host.TokenName = previous.TokenName()
		host.TokenHint = previous.TokenHint()
		host.TokenLastUsedAt = previous.TokenLastUsedAt()
	}

	// Link host agent to matching PVE node/VM/container by hostname
	// This prevents duplication when users install agents on PVE cluster nodes
	linkedNodeID, linkedVMID, linkedContainerID := m.findLinkedProxmoxEntityWithHints(
		hostname,
		report.Host.ReportIP,
		report.Network,
	)
	if linkedNodeID != "" {
		host.LinkedNodeID = linkedNodeID
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("linkedNodeId", linkedNodeID).
			Msg("Linked host agent to PVE node")
	}
	if linkedVMID != "" {
		host.LinkedVMID = linkedVMID
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("linkedVmId", linkedVMID).
			Msg("Linked host agent to VM")
	}
	if linkedContainerID != "" {
		host.LinkedContainerID = linkedContainerID
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("linkedContainerId", linkedContainerID).
			Msg("Linked host agent to container")
	}

	// Compute I/O rates from cumulative counters before adding to state.
	// Network and disk bytes from the agent are cumulative totals since boot;
	// the RateTracker converts them to bytes/second, just like VMs and containers.
	now := receivedAt

	var totalRXBytes, totalTXBytes uint64
	for _, nic := range host.NetworkInterfaces {
		totalRXBytes += nic.RXBytes
		totalTXBytes += nic.TXBytes
	}
	var totalDiskReadBytes, totalDiskWriteBytes uint64
	var totalDiskBusyMs uint64
	for _, d := range host.DiskIO {
		totalDiskReadBytes += d.ReadBytes
		totalDiskWriteBytes += d.WriteBytes
		totalDiskBusyMs += d.IOTime
	}

	hostRateKey := fmt.Sprintf("agent:%s", host.ID)
	currentMetrics := IOMetrics{
		DiskRead:   int64(totalDiskReadBytes),
		DiskWrite:  int64(totalDiskWriteBytes),
		DiskBusy:   int64(totalDiskBusyMs),
		NetworkIn:  int64(totalRXBytes),
		NetworkOut: int64(totalTXBytes),
		Timestamp:  now,
	}
	diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(hostRateKey, currentMetrics)

	// Store computed rates on the host model so they flow through to unified resources
	if netInRate >= 0 {
		host.NetInRate = netInRate
	}
	if netOutRate >= 0 {
		host.NetOutRate = netOutRate
	}
	if diskReadRate >= 0 {
		host.DiskReadRate = diskReadRate
	}
	if diskWriteRate >= 0 {
		host.DiskWriteRate = diskWriteRate
	}

	m.state.UpsertHost(host)
	m.state.SetConnectionHealth(hostConnectionPrefix+host.ID, true)
	m.supersedeStaleHostAgentDuplicates(host, tokenRecord, existingHostModels)

	// Update the linked PVE node to point back to this host agent
	if host.LinkedNodeID != "" {
		m.linkNodeToHostAgent(host.LinkedNodeID, host.ID)
		m.applyAgentLXCFilesystems(
			host.LinkedNodeID,
			host.ID,
			report.ProxmoxLXC,
			receivedAt,
			report.Agent.IntervalSeconds,
		)
	}

	// If host reports Ceph data, also update the global CephClusters state
	if report.Ceph != nil {
		cephCluster := convertAgentCephToGlobalCluster(report.Ceph, hostname, identifier, observedAt)
		storedCephCluster := m.state.UpsertCephCluster(cephCluster)
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("fsid", storedCephCluster.FSID).
			Str("source", storedCephCluster.Source).
			Str("health", storedCephCluster.Health).
			Int("osds", storedCephCluster.NumOSDs).
			Msg("Updated Ceph cluster from host agent")

		// #1341: the agent-reported Ceph cluster used to land in state but
		// never get evaluated for pool alerts. Only the Proxmox-API polling
		// path ran checkCephPoolStorage, so overrides set against
		// agent-prefixed pool IDs (e.g. agent:hostname-ceph-pool-foo) were
		// silently dormant. Run the alert check here so agent-sourced pools
		// actually fire.
		m.checkCephPoolStorage(storedCephCluster)
	}

	if m.alertManager != nil {
		m.alertManager.CheckHost(host)
	}

	// Record host-agent metrics for sparkline charts.
	hostMetricKey := fmt.Sprintf("agent:%s", host.ID)

	var hostDiskPercent float64
	if len(host.Disks) > 0 {
		hostDiskPercent = host.Disks[0].Usage
	}
	hostTemperature := hostPrimaryTemperatureCelsius(host.Sensors)
	hostGPUUtilization, hostGPUMemory, hostGPUTemperature := hostGPUMetrics(host.Sensors)

	if !shouldSkipNativeMockStateMetricWrites() {
		if m.metricsHistory != nil {
			m.metricsHistory.AddGuestMetric(hostMetricKey, "cpu", host.CPUUsage, now)
			if host.Memory.HasKnownUsage() {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "memory", host.Memory.Usage, now)
			}
			m.metricsHistory.AddGuestMetric(hostMetricKey, "disk", hostDiskPercent, now)
			if hostTemperature != nil {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "temperature", *hostTemperature, now)
			}
			if hostGPUUtilization != nil {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "gpu", *hostGPUUtilization, now)
			}
			if hostGPUMemory != nil {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "gpu_memory", *hostGPUMemory, now)
			}
			if hostGPUTemperature != nil {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "gpu_temperature", *hostGPUTemperature, now)
			}

			if netInRate >= 0 {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "netin", netInRate, now)
			}
			if netOutRate >= 0 {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "netout", netOutRate, now)
			}
			if diskReadRate >= 0 {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "diskread", diskReadRate, now)
			}
			if diskWriteRate >= 0 {
				m.metricsHistory.AddGuestMetric(hostMetricKey, "diskwrite", diskWriteRate, now)
			}
		}

		m.writeHostPhysicalDiskIOMetrics(host, now)

		if m.metricsStore != nil {
			m.metricsStore.Write("agent", host.ID, "cpu", host.CPUUsage, now)
			if host.Memory.HasKnownUsage() {
				m.metricsStore.Write("agent", host.ID, "memory", host.Memory.Usage, now)
			}
			m.metricsStore.Write("agent", host.ID, "disk", hostDiskPercent, now)
			if hostTemperature != nil {
				m.metricsStore.Write("agent", host.ID, "temperature", *hostTemperature, now)
			}
			if hostGPUUtilization != nil {
				m.metricsStore.Write("agent", host.ID, "gpu", *hostGPUUtilization, now)
			}
			if hostGPUMemory != nil {
				m.metricsStore.Write("agent", host.ID, "gpu_memory", *hostGPUMemory, now)
			}
			if hostGPUTemperature != nil {
				m.metricsStore.Write("agent", host.ID, "gpu_temperature", *hostGPUTemperature, now)
			}
			m.writeHostSMARTMetrics(host, now)
			if netInRate >= 0 {
				m.metricsStore.Write("agent", host.ID, "netin", netInRate, now)
			}
			if netOutRate >= 0 {
				m.metricsStore.Write("agent", host.ID, "netout", netOutRate, now)
			}
			if diskReadRate >= 0 {
				m.metricsStore.Write("agent", host.ID, "diskread", diskReadRate, now)
			}
			if diskWriteRate >= 0 {
				m.metricsStore.Write("agent", host.ID, "diskwrite", diskWriteRate, now)
			}
		}
		m.writeLibvirtDomainMetrics(host, now)
	}

	// Store cluster peer sensor data if present and evict stale entries
	m.applyClusterSensors(report.ClusterSensors, observedAt)
	// Availability results are ingested only once the host identity is
	// committed, because ownership is checked against that host ID.
	m.ApplyProbeAvailabilityResults(host.ID, probeAvailabilityResultsFromReport(report.AvailabilityResults))
	m.persistHostContinuity(host, report, reportOrder)
	m.refreshUnifiedResourceStoreAfterAgentStateChange()

	return host, nil
}

func hostZFSPoolsFromAgentDisks(disks []agentshost.Disk) []models.HostZFSPool {
	pools := make([]models.HostZFSPool, 0)
	for _, disk := range disks {
		name := strings.TrimSpace(disk.Device)
		if name == "" ||
			!strings.EqualFold(strings.TrimSpace(disk.Type), "zfs") ||
			len(disk.ZFSDatasets) == 0 {
			continue
		}
		datasets := make([]models.ZFSDataset, 0, len(disk.ZFSDatasets))
		for _, dataset := range disk.ZFSDatasets {
			if len(datasets) >= maxHostZFSDatasetsPerPool {
				break
			}
			datasetName := strings.TrimSpace(dataset.Name)
			datasetType := strings.ToLower(strings.TrimSpace(dataset.Type))
			if datasetType == "" {
				datasetType = "filesystem"
			}
			mountpoint := strings.TrimSpace(dataset.Mountpoint)
			if datasetName == "" ||
				!strings.HasPrefix(datasetName, name+"/") ||
				(datasetType != "filesystem" && datasetType != "volume") ||
				len(datasetName) > maxHostZFSDatasetNameBytes ||
				len(mountpoint) > maxHostZFSMountpointBytes ||
				dataset.UsedBytes < 0 ||
				dataset.AvailableBytes < 0 ||
				dataset.ReferencedBytes < 0 {
				continue
			}
			datasets = append(datasets, models.ZFSDataset{
				Name:            datasetName,
				Type:            datasetType,
				Mountpoint:      mountpoint,
				UsedBytes:       dataset.UsedBytes,
				AvailableBytes:  dataset.AvailableBytes,
				ReferencedBytes: dataset.ReferencedBytes,
			})
		}
		if len(datasets) > 0 {
			pools = append(pools, models.HostZFSPool{Name: name, Datasets: datasets})
		}
	}
	return pools
}

func convertHostPackageUpdateStatus(status *agentshost.PackageUpdateStatus, observedAt time.Time) *models.HostPackageUpdateStatus {
	if status == nil {
		return nil
	}
	packages := make([]models.HostPackageUpdate, len(status.Packages))
	for i, pkg := range status.Packages {
		packages[i] = models.HostPackageUpdate{
			Name:             strings.TrimSpace(pkg.Name),
			InstalledVersion: strings.TrimSpace(pkg.InstalledVersion),
			AvailableVersion: strings.TrimSpace(pkg.AvailableVersion),
		}
	}
	return &models.HostPackageUpdateStatus{
		Supported:      status.Supported,
		Manager:        strings.TrimSpace(status.Manager),
		InventoryHash:  strings.TrimSpace(status.InventoryHash),
		PendingCount:   max(0, status.PendingCount),
		Packages:       packages,
		CheckedAt:      status.CheckedAt.UTC(),
		ObservedAt:     observedAt.UTC(),
		RebootRequired: status.RebootRequired,
		Error:          strings.TrimSpace(status.Error),
	}
}

func convertHostStorageCleanupStatus(status *agentshost.StorageCleanupStatus, observedAt time.Time) *models.HostStorageCleanupStatus {
	if status == nil {
		return nil
	}
	return &models.HostStorageCleanupStatus{
		Supported:        status.Supported,
		Provider:         strings.TrimSpace(status.Provider),
		Fingerprint:      strings.TrimSpace(status.Fingerprint),
		ReclaimableBytes: status.ReclaimableBytes,
		CheckedAt:        status.CheckedAt.UTC(),
		ObservedAt:       observedAt.UTC(),
		Error:            strings.TrimSpace(status.Error),
	}
}

func convertAgentConfigFingerprint(value *agentshost.ConfigFingerprint) *models.AgentConfigFingerprint {
	if value == nil {
		return nil
	}
	return &models.AgentConfigFingerprint{
		Version: strings.TrimSpace(value.Version),
		Hash:    strings.TrimSpace(value.Hash),
	}
}

func convertAgentUpdateStatus(value *agentshost.UpdateStatus) *models.AgentUpdateStatus {
	if value == nil {
		return nil
	}
	return &models.AgentUpdateStatus{
		State:            strings.TrimSpace(value.State),
		AutoUpdate:       value.AutoUpdate,
		UpdatedFrom:      strings.TrimSpace(value.UpdatedFrom),
		AvailableVersion: strings.TrimSpace(value.AvailableVersion),
		LastCheckedAt:    cloneAgentStatusTime(value.LastCheckedAt),
		LastAttemptAt:    cloneAgentStatusTime(value.LastAttemptAt),
		LastSuccessAt:    cloneAgentStatusTime(value.LastSuccessAt),
		LastError:        strings.TrimSpace(value.LastError),
	}
}

func convertAgentModuleStatuses(values []agentshost.ModuleStatus) []models.AgentModuleStatus {
	if len(values) == 0 {
		return nil
	}
	result := make([]models.AgentModuleStatus, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" {
			continue
		}
		result = append(result, models.AgentModuleStatus{
			Name:      name,
			Enabled:   value.Enabled,
			State:     strings.TrimSpace(value.State),
			LastError: strings.TrimSpace(value.LastError),
			UpdatedAt: value.UpdatedAt.UTC(),
		})
	}
	return result
}

// convertAgentPrivilegeStatus copies the agent-authored privilege profile
// into model state. It is informational only: nothing downstream may treat a
// non-root agent as unhealthy on this evidence alone.
func convertAgentPrivilegeStatus(value *agentshost.PrivilegeStatus) *models.AgentPrivilegeStatus {
	if value == nil {
		return nil
	}
	return &models.AgentPrivilegeStatus{
		RunningAsRoot:    value.RunningAsRoot,
		ServiceUser:      strings.TrimSpace(value.ServiceUser),
		CommandAuthority: strings.TrimSpace(value.CommandAuthority),
		TypedHelper:      value.TypedHelper,
		SmartctlHelper:   value.SmartctlHelper,
		PctHelper:        value.PctHelper,
	}
}

func previousHostAgentUpdate(hosts []models.Host, identifier string) *models.AgentUpdateStatus {
	for i := range hosts {
		if hosts[i].ID == identifier {
			return cloneModelAgentUpdateStatus(hosts[i].AgentUpdate)
		}
	}
	return nil
}

func mergeAgentUpdateStatus(previous, reported *models.AgentUpdateStatus, updatedFrom string, observedAt time.Time) *models.AgentUpdateStatus {
	if reported == nil {
		reported = cloneModelAgentUpdateStatus(previous)
	}
	if reported == nil && updatedFrom == "" {
		return nil
	}
	if reported == nil {
		reported = &models.AgentUpdateStatus{State: agentupdate.UpdateStateIdle, AutoUpdate: true}
	}
	if previous != nil {
		if reported.LastSuccessAt == nil {
			reported.LastSuccessAt = cloneAgentStatusTime(previous.LastSuccessAt)
		}
		if reported.UpdatedFrom == "" {
			reported.UpdatedFrom = previous.UpdatedFrom
		}
	}
	if updatedFrom != "" {
		reported.UpdatedFrom = updatedFrom
		succeededAt := observedAt.UTC()
		reported.LastSuccessAt = &succeededAt
	}
	return reported
}

func cloneModelAgentUpdateStatus(value *models.AgentUpdateStatus) *models.AgentUpdateStatus {
	if value == nil {
		return nil
	}
	copy := *value
	copy.LastCheckedAt = cloneAgentStatusTime(value.LastCheckedAt)
	copy.LastAttemptAt = cloneAgentStatusTime(value.LastAttemptAt)
	copy.LastSuccessAt = cloneAgentStatusTime(value.LastSuccessAt)
	return &copy
}

func cloneAgentStatusTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func hostPrimaryTemperatureCelsius(sensors models.HostSensorSummary) *float64 {
	if len(sensors.TemperatureCelsius) == 0 {
		return nil
	}
	if value, ok := sensors.TemperatureCelsius["cpu_package"]; ok && value > 0 {
		return &value
	}

	var best float64
	found := false
	for key, value := range sensors.TemperatureCelsius {
		if value <= 0 || !strings.HasPrefix(strings.ToLower(key), "cpu") {
			continue
		}
		if !found || value > best {
			best = value
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func hostGPUMetrics(sensors models.HostSensorSummary) (
	utilizationPercent *float64,
	memoryPercent *float64,
	temperatureCelsius *float64,
) {
	for _, gpu := range sensors.GPU {
		if gpu.UtilizationPercent != nil {
			value := *gpu.UtilizationPercent
			if value >= 0 && value <= 100 &&
				(utilizationPercent == nil || value > *utilizationPercent) {
				copy := value
				utilizationPercent = &copy
			}
		}
		if gpu.MemoryUsedBytes != nil && gpu.MemoryTotalBytes != nil &&
			*gpu.MemoryUsedBytes >= 0 && *gpu.MemoryTotalBytes > 0 {
			value := float64(*gpu.MemoryUsedBytes) / float64(*gpu.MemoryTotalBytes) * 100
			if value <= 100 && (memoryPercent == nil || value > *memoryPercent) {
				copy := value
				memoryPercent = &copy
			}
		}
		if gpu.TemperatureCelsius != nil {
			value := *gpu.TemperatureCelsius
			if value > 0 && value <= 150 &&
				(temperatureCelsius == nil || value > *temperatureCelsius) {
				copy := value
				temperatureCelsius = &copy
			}
		}
	}
	return utilizationPercent, memoryPercent, temperatureCelsius
}

func nodePrimaryTemperatureCelsius(temperature *models.Temperature) *float64 {
	if temperature == nil || !temperature.Available {
		return nil
	}
	if temperature.CPUMax > 0 {
		value := temperature.CPUMax
		return &value
	}
	if temperature.CPUPackage > 0 {
		value := temperature.CPUPackage
		return &value
	}

	var best float64
	found := false
	for _, core := range temperature.Cores {
		if core.Temp <= 0 {
			continue
		}
		if !found || core.Temp > best {
			best = core.Temp
			found = true
		}
	}
	if !found {
		return nil
	}
	return &best
}

func (m *Monitor) writeHostSMARTMetrics(host models.Host, now time.Time) {
	if shouldSkipNativeMockStateMetricWrites() || m.metricsStore == nil {
		return
	}

	for _, disk := range host.Sensors.SMART {
		resourceID := unifiedresources.HostSMARTDiskSourceID(host, disk)
		if resourceID == "" {
			continue
		}

		if disk.Temperature > 0 {
			m.metricsStore.Write("disk", resourceID, "smart_temp", float64(disk.Temperature), now)
		}

		attrs := disk.Attributes
		if attrs == nil {
			continue
		}

		if attrs.PowerOnHours != nil {
			m.metricsStore.Write("disk", resourceID, "smart_power_on_hours", float64(*attrs.PowerOnHours), now)
		}
		if attrs.PowerCycles != nil {
			m.metricsStore.Write("disk", resourceID, "smart_power_cycles", float64(*attrs.PowerCycles), now)
		}
		if attrs.ReallocatedSectors != nil {
			m.metricsStore.Write("disk", resourceID, "smart_reallocated_sectors", float64(*attrs.ReallocatedSectors), now)
		}
		if attrs.PendingSectors != nil {
			m.metricsStore.Write("disk", resourceID, "smart_pending_sectors", float64(*attrs.PendingSectors), now)
		}
		if attrs.OfflineUncorrectable != nil {
			m.metricsStore.Write("disk", resourceID, "smart_offline_uncorrectable", float64(*attrs.OfflineUncorrectable), now)
		}
		if attrs.UDMACRCErrors != nil {
			m.metricsStore.Write("disk", resourceID, "smart_crc_errors", float64(*attrs.UDMACRCErrors), now)
		}
		if attrs.PercentageUsed != nil {
			m.metricsStore.Write("disk", resourceID, "smart_percentage_used", float64(*attrs.PercentageUsed), now)
		}
		if attrs.AvailableSpare != nil {
			m.metricsStore.Write("disk", resourceID, "smart_available_spare", float64(*attrs.AvailableSpare), now)
		}
		if attrs.MediaErrors != nil {
			m.metricsStore.Write("disk", resourceID, "smart_media_errors", float64(*attrs.MediaErrors), now)
		}
		if attrs.UnsafeShutdowns != nil {
			m.metricsStore.Write("disk", resourceID, "smart_unsafe_shutdowns", float64(*attrs.UnsafeShutdowns), now)
		}
	}
}

func normalizeHostDiskDevice(device string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(device), "/dev/"))
}

func normalizeLegacyUnraidDiskStatus(rawStatus, device string) string {
	status := strings.ToUpper(strings.TrimSpace(rawStatus))
	switch {
	case status == "":
		if strings.TrimSpace(device) != "" {
			return "online"
		}
		return ""
	case strings.Contains(status, "DISK_OK") || status == "OK":
		return "online"
	case strings.Contains(status, "DISK_DSBL") || strings.Contains(status, "DISABLED"):
		return "disabled"
	case strings.Contains(status, "DISK_NP") || strings.Contains(status, "MISSING") || strings.Contains(status, "NOT_INSTALLED"):
		return "missing"
	case strings.Contains(status, "DISK_INVALID") || strings.Contains(status, "INVALID"):
		return "invalid"
	case strings.Contains(status, "DISK_WRONG") || strings.Contains(status, "WRONG"):
		return "wrong"
	case strings.Contains(status, "DISK_ERROR") || strings.Contains(status, "ERROR"):
		return "error"
	default:
		return strings.ToLower(status)
	}
}

func isLegacyUnraidEmptySlot(disk agentshost.UnraidDisk, normalizedStatus string) bool {
	rawStatus := strings.ToUpper(strings.TrimSpace(disk.RawStatus))
	status := strings.ToLower(strings.TrimSpace(normalizedStatus))
	if unraidstatus.IsExplicitMissingMember(rawStatus) {
		return false
	}
	if !strings.Contains(rawStatus, "DISK_NP") && status != "missing" {
		return false
	}
	return strings.TrimSpace(disk.Device) == "" &&
		!unraidstatus.HasMeaningfulIdentity(disk.Model, disk.Serial) &&
		strings.TrimSpace(disk.Filesystem) == "" &&
		disk.SizeBytes == 0
}

// reconcileReportedUnraidCounts makes the filtered per-disk topology
// authoritative when an agent supplied structured statuses. This also repairs
// stale aggregate counters from older agents at the server ingestion boundary.
func reconcileReportedUnraidCounts(storage *agentshost.UnraidStorage, disks []models.HostUnraidDisk) (protected, disabled, invalid, missing int) {
	protected = storage.NumProtected
	disabled = storage.NumDisabled
	invalid = storage.NumInvalid
	missing = storage.NumMissing

	hasStructuredStatus := false
	hasStructuredParity := false
	structuredProtected, structuredDisabled, structuredInvalid, structuredMissing := 0, 0, 0, 0
	for _, disk := range disks {
		status := strings.ToLower(strings.TrimSpace(disk.Status))
		if status == "" {
			continue
		}
		hasStructuredStatus = true
		if strings.EqualFold(strings.TrimSpace(disk.Role), "parity") {
			hasStructuredParity = true
			if status == "online" {
				structuredProtected++
			}
		}
		switch status {
		case "disabled":
			structuredDisabled++
		case "invalid":
			structuredInvalid++
		case "missing":
			structuredMissing++
		}
	}
	if hasStructuredStatus {
		disabled = structuredDisabled
		invalid = structuredInvalid
		missing = structuredMissing
	}
	if hasStructuredParity {
		protected = structuredProtected
	}
	return protected, disabled, invalid, missing
}

type proxmoxDiskMatch struct {
	device   string
	metricID string
}

func hostDiskIOMetricResourceID(host models.Host, io models.DiskIO, proxmoxDisks []proxmoxDiskMatch) string {
	device := normalizeHostDiskDevice(io.Device)
	if device == "" {
		return ""
	}

	smartMetricID := ""
	for _, disk := range host.Sensors.SMART {
		if disk.Standby {
			continue
		}
		if strings.EqualFold(normalizeHostDiskDevice(disk.Device), device) {
			candidate := unifiedresources.HostSMARTDiskSourceID(host, disk)
			if smartMetricID != "" && smartMetricID != candidate {
				// Multiple controller members share this kernel block path.
				// The counter belongs to the aggregate device, not any member.
				return ""
			}
			smartMetricID = candidate
		}
	}
	if smartMetricID != "" {
		return smartMetricID
	}

	for _, pd := range proxmoxDisks {
		if pd.device == "" || pd.metricID == "" {
			continue
		}
		if strings.EqualFold(pd.device, device) {
			return pd.metricID
		}
	}

	return fmt.Sprintf("%s:%s", strings.TrimSpace(host.ID), device)
}

func (m *Monitor) writeHostPhysicalDiskIOMetrics(host models.Host, now time.Time) {
	if shouldSkipNativeMockStateMetricWrites() || (m.metricsHistory == nil && m.metricsStore == nil) {
		return
	}
	if len(host.DiskIO) == 0 {
		return
	}

	var proxmoxDisks []proxmoxDiskMatch
	if host.LinkedNodeID != "" {
		proxmoxDisks = m.proxmoxPhysicalDiskMatchesForLinkedNode(host.LinkedNodeID)
	}

	seenResourceIDs := make(map[string]struct{}, len(host.DiskIO))
	for _, io := range host.DiskIO {
		resourceID := hostDiskIOMetricResourceID(host, io, proxmoxDisks)
		if resourceID == "" {
			continue
		}
		if _, seen := seenResourceIDs[resourceID]; seen {
			continue
		}
		seenResourceIDs[resourceID] = struct{}{}

		trackerKey := fmt.Sprintf("disk:%s:%s", host.ID, normalizeHostDiskDevice(io.Device))
		current := IOMetrics{
			DiskRead:  int64(io.ReadBytes),
			DiskWrite: int64(io.WriteBytes),
			DiskBusy:  int64(io.IOTime),
			Timestamp: now,
		}
		readRate, writeRate, busyPct, _, _ := m.rateTracker.CalculateRatesWithBusy(trackerKey, current)

		if readRate >= 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddDiskMetric(resourceID, "diskread", readRate, now)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("disk", resourceID, "diskread", readRate, now)
			}
		}
		if writeRate >= 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddDiskMetric(resourceID, "diskwrite", writeRate, now)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("disk", resourceID, "diskwrite", writeRate, now)
			}
		}
		if busyPct >= 0 {
			if m.metricsHistory != nil {
				m.metricsHistory.AddDiskMetric(resourceID, "disk", busyPct, now)
			}
			if m.metricsStore != nil {
				m.metricsStore.Write("disk", resourceID, "disk", busyPct, now)
			}
		}
	}
}

func (m *Monitor) proxmoxPhysicalDiskMatchesForLinkedNode(linkedNodeID string) []proxmoxDiskMatch {
	if linkedNodeID == "" {
		return nil
	}
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil
	}
	var matches []proxmoxDiskMatch
	for _, disk := range readState.PhysicalDisks() {
		instance := strings.TrimSpace(disk.Instance())
		node := strings.TrimSpace(disk.Node())
		var diskLinkedID string
		if instance == "" {
			diskLinkedID = node
		} else {
			diskLinkedID = fmt.Sprintf("%s-%s", instance, node)
		}
		if diskLinkedID != linkedNodeID {
			continue
		}
		metricID := strings.TrimSpace(disk.MetricResourceID())
		if metricID == "" {
			continue
		}
		matches = append(matches, proxmoxDiskMatch{
			device:   normalizeHostDiskDevice(disk.DevPath()),
			metricID: metricID,
		})
	}
	return matches
}

// applyClusterSensors stores temperature data collected from Proxmox cluster
// siblings via SSH. Each entry is keyed by lowercase node name so that
// getHostAgentTemperatureByID can use it as a fallback.
func (m *Monitor) applyClusterSensors(entries []agentshost.ClusterNodeSensors, reportTime time.Time) {
	// Fast path: nothing to add and cache is empty — skip lock
	if len(entries) == 0 {
		m.clusterSensorsMu.RLock()
		empty := len(m.clusterSensorsCache) == 0
		m.clusterSensorsMu.RUnlock()
		if empty {
			return
		}
	}

	m.clusterSensorsMu.Lock()
	defer m.clusterSensorsMu.Unlock()

	for _, entry := range entries {
		nodeName := strings.ToLower(strings.TrimSpace(entry.NodeName))
		if nodeName == "" {
			continue
		}
		if len(entry.Sensors.TemperatureCelsius) == 0 {
			continue
		}

		m.clusterSensorsCache[nodeName] = clusterSensorsCacheEntry{
			sensors: models.HostSensorSummary{
				TemperatureCelsius: cloneStringFloatMap(entry.Sensors.TemperatureCelsius),
				FanRPM:             cloneStringFloatMap(entry.Sensors.FanRPM),
				PowerWatts:         cloneStringFloatMap(entry.Sensors.PowerWatts),
				Additional:         cloneStringFloatMap(entry.Sensors.Additional),
				ThermalState:       convertAgentThermalStateToModels(entry.Sensors.ThermalState),
			},
			updatedAt: reportTime,
		}
	}

	// Evict stale entries to prevent unbounded cache growth.
	// Cluster sizes are small (3-16 nodes) so this is cheap.
	const staleThreshold = 5 * time.Minute
	now := time.Now()
	for key, entry := range m.clusterSensorsCache {
		if now.Sub(entry.updatedAt) > staleThreshold {
			delete(m.clusterSensorsCache, key)
		}
	}
}

// findLinkedProxmoxEntity searches for a PVE node, VM, or container with a matching hostname.
// Returns the IDs of matched entities (empty string if no match).
// When multiple entities match the same hostname (e.g., two PVE instances both have a node
// named "pve"), this function returns empty strings to avoid incorrect linking. Users should
// manually link agents to nodes via the UI in such cases.
func (m *Monitor) findLinkedProxmoxEntity(hostname string) (nodeID, vmID, containerID string) {
	return m.findLinkedProxmoxEntityWithHints(hostname, "", nil)
}

func collectReportedHostIPs(
	reportIP string,
	network []agentshost.NetworkInterface,
) map[string]struct{} {
	ips := make(map[string]struct{})
	if normalized := unifiedresources.NormalizeIP(reportIP); normalized != "" {
		ips[normalized] = struct{}{}
	}

	for _, nic := range network {
		for _, address := range nic.Addresses {
			if normalized := unifiedresources.NormalizeIP(address); normalized != "" {
				ips[normalized] = struct{}{}
			}
		}
	}

	return ips
}

func collectNodeNetworkIPs(network []unifiedresources.NetworkInterface) map[string]struct{} {
	ips := make(map[string]struct{})
	for _, nic := range network {
		for _, address := range nic.Addresses {
			if normalized := unifiedresources.NormalizeIP(address); normalized != "" {
				ips[normalized] = struct{}{}
			}
		}
	}
	return ips
}

// uniqueNodeNetworkMatchesReportedHints accepts only provider-observed node
// addresses that belong to one current Proxmox node. This disambiguates the
// common FQDN-endpoint/short-hostname case without guessing when separate
// sites reuse an RFC1918 address or every node exposes the same bridge IP.
func uniqueNodeNetworkMatchesReportedHints(
	network []unifiedresources.NetworkInterface,
	reportedIPs map[string]struct{},
	ownerCounts map[string]int,
) bool {
	for ip := range collectNodeNetworkIPs(network) {
		if ownerCounts[ip] != 1 {
			continue
		}
		if _, reported := reportedIPs[ip]; reported {
			return true
		}
	}
	return false
}

func endpointHostMatchesReportedHints(
	endpointHost string,
	reportedHostname string,
	reportedIPs map[string]struct{},
) bool {
	normalizedEndpointHost := strings.TrimSpace(strings.ToLower(endpointHost))
	if normalizedEndpointHost == "" {
		return false
	}

	if normalizedReportedIP := unifiedresources.NormalizeIP(normalizedEndpointHost); normalizedReportedIP != "" {
		_, ok := reportedIPs[normalizedReportedIP]
		return ok
	}

	normalizedReportedHostname := strings.TrimSpace(strings.ToLower(reportedHostname))
	return normalizedReportedHostname != "" && normalizedEndpointHost == normalizedReportedHostname
}

func (m *Monitor) findLinkedProxmoxEntityWithHints(
	hostname string,
	reportIP string,
	network []agentshost.NetworkInterface,
) (nodeID, vmID, containerID string) {
	if hostname == "" {
		return "", "", ""
	}

	// Normalize hostname for comparison (lowercase, strip domain)
	normalizedHostname := strings.ToLower(hostname)
	shortHostname := normalizedHostname
	if idx := strings.Index(normalizedHostname, "."); idx > 0 {
		shortHostname = normalizedHostname[:idx]
	}

	matchHostname := func(name string) bool {
		normalized := strings.ToLower(name)
		if normalized == normalizedHostname || normalized == shortHostname {
			return true
		}
		// Also check short version of the candidate
		if idx := strings.Index(normalized, "."); idx > 0 {
			if normalized[:idx] == shortHostname {
				return true
			}
		}
		return false
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return "", "", ""
	}
	nodes := readState.Nodes()

	type linkedEntityMatch struct {
		id       string
		instance string
	}

	reportedIPs := collectReportedHostIPs(reportIP, network)
	nodeIPOwnerCounts := make(map[string]int)
	for _, node := range nodes {
		for ip := range collectNodeNetworkIPs(node.NetworkInterfaces()) {
			nodeIPOwnerCounts[ip]++
		}
	}

	// First, try to match the configured PVE node endpoint or a unique address
	// from the provider-observed node network inventory against the host report.
	// Both are stronger than node-name matching and can disambiguate nodes that
	// share the same short hostname but have different management addresses.
	var endpointMatchedNodes []linkedEntityMatch
	for _, node := range nodes {
		if endpointHostMatchesReportedHints(extractHostname(node.HostURL()), hostname, reportedIPs) ||
			uniqueNodeNetworkMatchesReportedHints(node.NetworkInterfaces(), reportedIPs, nodeIPOwnerCounts) {
			endpointMatchedNodes = append(endpointMatchedNodes, linkedEntityMatch{
				id:       node.SourceID(),
				instance: node.Instance(),
			})
		}
	}
	if len(endpointMatchedNodes) == 1 {
		return endpointMatchedNodes[0].id, "", ""
	}
	if len(endpointMatchedNodes) > 1 {
		log.Warn().
			Str("hostname", hostname).
			Str("reportIP", strings.TrimSpace(reportIP)).
			Int("matchCount", len(endpointMatchedNodes)).
			Msg("Multiple PVE node endpoints match host report hints - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	// Check PVE nodes first - but detect ambiguity when multiple nodes match
	var matchingNodes []linkedEntityMatch
	for _, node := range nodes {
		if matchHostname(node.Name()) {
			matchingNodes = append(matchingNodes, linkedEntityMatch{
				id:       node.SourceID(),
				instance: node.Instance(),
			})
		}
	}
	if len(matchingNodes) == 1 {
		return matchingNodes[0].id, "", ""
	}
	if len(matchingNodes) > 1 {
		// Multiple nodes with the same hostname - can't auto-link, would cause data mixing
		log.Warn().
			Str("hostname", hostname).
			Int("matchCount", len(matchingNodes)).
			Strs("instances", func() []string {
				instances := make([]string, len(matchingNodes))
				for i, n := range matchingNodes {
					instances[i] = n.instance
				}
				return instances
			}()).
			Msg("Multiple PVE nodes match hostname - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	// Check VMs - same pattern for ambiguity detection
	var matchingVMs []linkedEntityMatch
	for _, vm := range readState.VMs() {
		if matchHostname(vm.Name()) {
			matchingVMs = append(matchingVMs, linkedEntityMatch{
				id: vm.SourceID(),
			})
		}
	}
	if len(matchingVMs) == 1 {
		return "", matchingVMs[0].id, ""
	}
	if len(matchingVMs) > 1 {
		log.Warn().
			Str("hostname", hostname).
			Int("matchCount", len(matchingVMs)).
			Msg("Multiple VMs match hostname - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	// Check containers - same pattern
	var matchingCTs []linkedEntityMatch
	for _, ct := range readState.Containers() {
		if matchHostname(ct.Name()) {
			matchingCTs = append(matchingCTs, linkedEntityMatch{
				id: ct.SourceID(),
			})
		}
	}
	if len(matchingCTs) == 1 {
		return "", "", matchingCTs[0].id
	}
	if len(matchingCTs) > 1 {
		log.Warn().
			Str("hostname", hostname).
			Int("matchCount", len(matchingCTs)).
			Msg("Multiple containers match hostname - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	return "", "", ""
}

// linkNodeToHostAgent updates a PVE node to link to its host agent.
func (m *Monitor) linkNodeToHostAgent(nodeID, hostAgentID string) {
	m.state.LinkNodeToHostAgent(nodeID, hostAgentID)
}

const (
	removedDockerHostsTTL = 24 * time.Hour // Clean up removed hosts tracking after 24 hours
	removedHostAgentsTTL  = 24 * time.Hour // Clean up removed host agent tracking after 24 hours
)

// recoverFromPanic recovers from panics in monitoring goroutines and logs them.
// This prevents a panic in one component from crashing the entire monitoring system.
func recoverFromPanic(goroutineName string) {
	if r := recover(); r != nil {
		log.Error().
			Str("goroutine", goroutineName).
			Interface("panic", r).
			Stack().
			Msg("Recovered from panic in monitoring goroutine")
	}
}

// cleanupRemovedDockerHosts expires removed Docker host blocks older than 24
// hours. Persisted entries are swept by their own RemovedAt because the
// in-memory map resets on restart (see cleanupRemovedHostAgents, #1581).
func (m *Monitor) cleanupRemovedDockerHosts(now time.Time) {
	expired := make(map[string]time.Time)

	for _, entry := range m.state.GetRemovedDockerHosts() {
		if now.Sub(entry.RemovedAt) > removedDockerHostsTTL {
			expired[entry.ID] = entry.RemovedAt
		}
	}

	m.mu.Lock()
	for hostID, removedAt := range m.removedDockerHosts {
		if now.Sub(removedAt) > removedDockerHostsTTL {
			expired[hostID] = removedAt
		}
	}
	m.mu.Unlock()

	// Remove from state and map without holding both locks
	for hostID, removedAt := range expired {
		m.state.RemoveRemovedDockerHost(hostID)

		m.mu.Lock()
		delete(m.removedDockerHosts, hostID)
		m.mu.Unlock()

		log.Debug().
			Str("dockerHostID", hostID).
			Time("removedAt", removedAt).
			Msg("Cleaned up old removed Docker host entry")
	}
}

// hydrateRemovedHostAgents restores durable removal tombstones before the
// monitor accepts reports. Expired tombstones are deleted eagerly; if that
// delete cannot be persisted, the block is retained in memory to fail closed.
func (m *Monitor) hydrateRemovedHostAgents(now time.Time) {
	if m == nil || m.hostContinuityStore == nil || m.state == nil {
		return
	}

	for _, continuity := range m.hostContinuityStore.RemovedEntries() {
		if now.Sub(continuity.RemovedAt) > removedHostAgentsTTL {
			if err := m.hostContinuityStore.Delete(continuity.HostID); err == nil {
				continue
			} else {
				log.Warn().
					Err(err).
					Str("hostID", continuity.HostID).
					Msg("Failed to expire durable host-agent removal tombstone; retaining block")
			}
		}

		entry := removedHostAgentFromContinuity(continuity)
		m.state.AddRemovedHostAgent(entry)
		m.mu.Lock()
		if m.removedHostAgents == nil {
			m.removedHostAgents = make(map[string]time.Time)
		}
		m.removedHostAgents[entry.ID] = entry.RemovedAt
		m.mu.Unlock()
	}
}

// cleanupRemovedHostAgents expires removal tombstones older than 24 hours.
// Durable state is deleted before memory so a persistence failure retains the
// deny boundary across this process and the next restart.
func (m *Monitor) cleanupRemovedHostAgents(now time.Time) {
	m.hostAgentLifecycleMu.Lock()
	defer m.hostAgentLifecycleMu.Unlock()

	expired := make(map[string]time.Time)

	for _, entry := range m.state.GetRemovedHostAgents() {
		if now.Sub(entry.RemovedAt) > removedHostAgentsTTL {
			expired[entry.ID] = entry.RemovedAt
		}
	}

	m.mu.Lock()
	for hostID, removedAt := range m.removedHostAgents {
		if now.Sub(removedAt) > removedHostAgentsTTL {
			expired[hostID] = removedAt
		}
	}
	m.mu.Unlock()

	for hostID, removedAt := range expired {
		if m.hostContinuityStore != nil {
			if continuity, ok := m.hostContinuityStore.Get(hostID); ok && !continuity.RemovedAt.IsZero() {
				if err := m.hostContinuityStore.Delete(hostID); err != nil {
					log.Warn().
						Err(err).
						Str("hostID", hostID).
						Msg("Failed to expire durable host-agent removal tombstone; retaining block")
					continue
				}
			}
		}

		m.state.RemoveRemovedHostAgent(hostID)

		m.mu.Lock()
		delete(m.removedHostAgents, hostID)
		m.mu.Unlock()

		log.Debug().
			Str("hostID", hostID).
			Time("removedAt", removedAt).
			Msg("Cleaned up old removed host agent entry")
	}
}

// cleanupGuestMetadataCache removes stale guest metadata cache and limiter entries.
// Entries older than 2x the cache TTL (10 minutes) are removed to prevent unbounded growth
// when VMs are deleted or moved.
func (m *Monitor) cleanupGuestMetadataCache(now time.Time) {
	const maxAge = 2 * guestMetadataCacheTTL // 10 minutes

	m.guestMetadataMu.Lock()
	for key, entry := range m.guestMetadataCache {
		if now.Sub(entry.fetchedAt) > maxAge {
			delete(m.guestMetadataCache, key)
			log.Debug().
				Str("key", key).
				Time("fetchedAt", entry.fetchedAt).
				Msg("Cleaned up stale guest metadata cache entry")
		}
	}
	m.guestMetadataMu.Unlock()

	m.guestMetadataLimiterMu.Lock()
	defer m.guestMetadataLimiterMu.Unlock()
	for key, nextAllowed := range m.guestMetadataLimiter {
		// Keep near-term limiter state; remove long-idle keys.
		if now.Sub(nextAllowed) > maxAge {
			delete(m.guestMetadataLimiter, key)
			log.Debug().
				Str("key", key).
				Time("nextAllowed", nextAllowed).
				Msg("Cleaned up stale guest metadata limiter entry")
		}
	}
}

// cleanupTrackingMaps removes stale entries from various tracking maps to prevent unbounded memory growth.
// This cleans up auth tracking, polling timestamps, and circuit breaker state for resources
// that haven't been accessed in over 24 hours.
func (m *Monitor) cleanupTrackingMaps(now time.Time) {
	const staleThreshold = 24 * time.Hour
	cutoff := now.Add(-staleThreshold)
	cleaned := 0
	activeKeys := m.activeSchedulerKeys()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up auth tracking maps - entries older than 24 hours
	for nodeID, ts := range m.lastAuthAttempt {
		if ts.Before(cutoff) {
			delete(m.lastAuthAttempt, nodeID)
			delete(m.authFailures, nodeID)
			cleaned++
		}
	}

	// Clean up last cluster check timestamps
	for instanceID, ts := range m.lastClusterCheck {
		if ts.Before(cutoff) {
			delete(m.lastClusterCheck, instanceID)
			cleaned++
		}
	}

	// Clean up last physical disk poll timestamps
	for instanceID, ts := range m.lastPhysicalDiskPoll {
		if ts.Before(cutoff) {
			delete(m.lastPhysicalDiskPoll, instanceID)
			cleaned++
		}
	}

	// Clean up last PVE backup poll timestamps
	for instanceID, ts := range m.lastPVEBackupPoll {
		if ts.Before(cutoff) {
			delete(m.lastPVEBackupPoll, instanceID)
			delete(m.pveBackupInventoryReady, instanceID)
			delete(m.pveBackupTemplateSubjects, instanceID)
			cleaned++
		}
	}

	// Clean up last PBS backup poll timestamps
	for instanceID, ts := range m.lastPBSBackupPoll {
		if ts.Before(cutoff) {
			delete(m.lastPBSBackupPoll, instanceID)
			cleaned++
		}
	}

	// Only clean up circuit breakers for inactive keys that have been idle
	// for longer than the stale threshold
	for key, breaker := range m.circuitBreakers {
		if _, active := activeKeys[key]; !active {
			// Key is not in active clients - check if breaker is stale
			if breaker != nil {
				_, _, _, _, lastTransition := breaker.stateDetails()
				if now.Sub(lastTransition) > staleThreshold {
					delete(m.circuitBreakers, key)
					delete(m.failureCounts, key)
					delete(m.lastOutcome, key)
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		log.Debug().
			Int("entriesCleaned", cleaned).
			Msg("Cleaned stale entries from monitor tracking maps")
	}
}

// cleanupDiagnosticSnapshots removes stale diagnostic snapshots.
// Snapshots older than 1 hour are removed to prevent unbounded growth
// when nodes/VMs are deleted or reconfigured.
func (m *Monitor) cleanupDiagnosticSnapshots(now time.Time) {
	const maxAge = 1 * time.Hour

	m.diagMu.Lock()
	defer m.diagMu.Unlock()

	for key, snapshot := range m.nodeSnapshots {
		if now.Sub(snapshot.RetrievedAt) > maxAge {
			delete(m.nodeSnapshots, key)
			log.Debug().
				Str("key", key).
				Time("retrievedAt", snapshot.RetrievedAt).
				Msg("Cleaned up stale node snapshot")
		}
	}

	for key, snapshot := range m.guestSnapshots {
		if now.Sub(snapshot.RetrievedAt) > maxAge {
			delete(m.guestSnapshots, key)
			log.Debug().
				Str("key", key).
				Time("retrievedAt", snapshot.RetrievedAt).
				Msg("Cleaned up stale guest snapshot")
		}
	}
}

// cleanupRRDCache removes stale RRD memory cache entries.
// Entries older than their short-lived TTL windows are removed to prevent
// unbounded growth when nodes or guests disappear from the poll set.
func (m *Monitor) cleanupRRDCache(now time.Time) {
	const maxAge = 2 * nodeRRDCacheTTL // 1 minute

	m.rrdCacheMu.Lock()
	defer m.rrdCacheMu.Unlock()

	for key, entry := range m.nodeRRDMemCache {
		if now.Sub(entry.fetchedAt) > maxAge {
			delete(m.nodeRRDMemCache, key)
			log.Debug().
				Str("node", key).
				Time("fetchedAt", entry.fetchedAt).
				Msg("Cleaned up stale RRD cache entry")
		}
	}

	for key, entry := range m.vmAgentMemCache {
		if now.Sub(entry.fetchedAt) > vmAgentMemCleanupMaxAge {
			delete(m.vmAgentMemCache, key)
		}
	}
}

// cleanupMetricsHistory removes stale entries from the metrics history.
// This prevents unbounded memory growth when containers/VMs are deleted.
func (m *Monitor) cleanupMetricsHistory() {
	if m.metricsHistory != nil {
		m.metricsHistory.Cleanup()
	}
}

// cleanupRateTracker removes stale entries from the rate tracker.
// Entries older than 24 hours are removed to prevent unbounded memory growth.
func (m *Monitor) cleanupRateTracker(now time.Time) {
	const staleThreshold = 24 * time.Hour
	cutoff := now.Add(-staleThreshold)

	if m.rateTracker != nil {
		if removed := m.rateTracker.Cleanup(cutoff); removed > 0 {
			log.Debug().
				Int("entriesRemoved", removed).
				Msg("Cleaned up stale rate tracker entries")
		}
	}
}

// evaluateDockerAgents updates health for Docker hosts based on last report time.
func (m *Monitor) evaluateDockerAgents(now time.Time) {
	hosts := m.state.GetDockerHosts()
	for _, host := range hosts {
		interval := host.IntervalSeconds
		if interval <= 0 {
			interval = int(dockerMinimumHealthWindow / time.Second)
		}

		window := time.Duration(interval) * time.Second * dockerOfflineGraceMultiplier
		if window < dockerMinimumHealthWindow {
			window = dockerMinimumHealthWindow
		} else if window > dockerMaximumHealthWindow {
			window = dockerMaximumHealthWindow
		}

		healthy := !host.LastSeen.IsZero() && now.Sub(host.LastSeen) <= window
		key := dockerConnectionPrefix + host.ID
		m.state.SetConnectionHealth(key, healthy)
		hostCopy := host
		if healthy {
			hostCopy.Status = "online"
			m.state.SetDockerHostStatus(host.ID, "online")
			if m.alertManager != nil {
				m.alertManager.HandleDockerHostOnline(hostCopy)
			}
		} else {
			hostCopy.Status = "offline"
			m.state.SetDockerHostStatus(host.ID, "offline")
			if m.alertManager != nil {
				m.alertManager.HandleDockerHostOffline(hostCopy)
			}
		}
	}
}

func hostAgentHealthWindow(interval int) time.Duration {
	if interval <= 0 {
		interval = int(hostMinimumHealthWindow / time.Second)
	}
	window := time.Duration(interval) * time.Second * hostOfflineGraceMultiplier
	if window < hostMinimumHealthWindow {
		return hostMinimumHealthWindow
	}
	if window > hostMaximumHealthWindow {
		return hostMaximumHealthWindow
	}
	return window
}

// evaluateHostAgents updates health for host agents based on last report time.
func (m *Monitor) evaluateHostAgents(now time.Time) {
	hosts := m.state.GetHosts()
	nodes := m.NodesSnapshot()
	liveHostIDs := make(map[string]struct{}, len(hosts))
	resourceRefreshNeeded := false
	for _, host := range hosts {
		liveHostIDs[host.ID] = struct{}{}
		window := hostAgentHealthWindow(host.IntervalSeconds)
		age := now.Sub(host.LastSeen)
		healthy := !host.LastSeen.IsZero() && age <= window
		key := hostConnectionPrefix + host.ID
		m.state.SetConnectionHealth(key, healthy)

		hostCopy := host
		if healthy {
			hostCopy.Status = "online"
			// Log status transition from offline to online
			if host.Status == "offline" {
				log.Debug().
					Str("hostID", host.ID).
					Str("hostname", host.Hostname).
					Dur("age", age).
					Dur("window", window).
					Msg("Host agent back online")
			}
			if host.Status != "online" {
				m.state.SetHostStatus(host.ID, "online")
				resourceRefreshNeeded = true
			}
			if m.alertManager != nil {
				m.alertManager.HandleHostOnline(hostCopy)
			}
		} else {
			// Log status transition from online to offline with diagnostic info
			if host.Status == "online" || host.Status == "" {
				log.Debug().
					Str("hostID", host.ID).
					Str("hostname", host.Hostname).
					Time("lastSeen", host.LastSeen).
					Dur("age", age).
					Dur("window", window).
					Int("intervalSeconds", host.IntervalSeconds).
					Bool("lastSeenZero", host.LastSeen.IsZero()).
					Msg("Host agent appears offline")
			}
			if expiredHost, changed := m.state.ExpireHostTelemetry(host.ID); expiredHost.ID != "" {
				hostCopy = expiredHost
				resourceRefreshNeeded = resourceRefreshNeeded || changed
			} else {
				hostCopy.Status = "offline"
			}
			if m.alertManager != nil {
				m.alertManager.HandleHostOfflineWithCorrelation(
					hostCopy,
					sharedSystemAlertCorrelationForHost(hostCopy, nodes),
				)
			}
		}
	}

	// Active alerts survive process restarts, while the live host snapshot does
	// not. Continuity entries provide the durable receipt timestamp needed to
	// expire those alerts after the same reporting lease, even if the agent
	// never reconnects.
	for _, entry := range m.recentStandaloneHostContinuityEntries() {
		if _, live := liveHostIDs[entry.HostID]; live {
			continue
		}
		window := hostAgentHealthWindow(entry.IntervalSeconds)
		healthy := !entry.LastSeen.IsZero() && now.Sub(entry.LastSeen) <= window
		m.state.SetConnectionHealth(hostConnectionPrefix+entry.HostID, healthy)
		if healthy {
			continue
		}
		host := hostFromContinuityEntry(entry)
		host.Status = "offline"
		if m.alertManager != nil {
			m.alertManager.HandleHostOfflineWithCorrelation(
				host,
				sharedSystemAlertCorrelationForHost(host, nodes),
			)
		}
	}

	if resourceRefreshNeeded {
		m.refreshUnifiedResourceStoreAfterAgentStateChange()
	}
}

func sharedSystemAlertCorrelationForHost(host models.Host, nodes []models.Node) *alerts.AlertCorrelation {
	linkedNodeID := strings.TrimSpace(host.LinkedNodeID)
	hostID := strings.TrimSpace(host.ID)
	if linkedNodeID == "" || hostID == "" {
		return nil
	}

	correlationKey := ""
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) != linkedNodeID || strings.TrimSpace(node.LinkedAgentID) != hostID {
			continue
		}
		instance := strings.TrimSpace(node.Instance)
		if instance == "" {
			return nil
		}
		candidate := "pve:" + instance
		if correlationKey != "" && correlationKey != candidate {
			return nil
		}
		correlationKey = candidate
	}

	return alerts.NewSharedSystemAlertCorrelation(
		correlationKey,
		alerts.AlertCorrelationRoleSupporting,
		"verified-proxmox-node-agent-link",
	)
}

// sortContent sorts comma-separated content values for consistent display
