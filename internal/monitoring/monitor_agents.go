package monitoring

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/types"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) RemoveDockerHost(hostID string) (models.DockerHost, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.DockerHost{}, fmt.Errorf("docker host id is required")
	}

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
		tokenRemoved := m.config.RemoveAPIToken(host.TokenID)
		if tokenRemoved != nil {
			m.config.SortAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", host.TokenID).Msg("Failed to persist API token revocation after Docker host removal")
				} else {
					log.Info().Str("tokenID", host.TokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed Docker host")
				}
			}
		}
	}

	// Track removal to prevent resurrection from cached reports
	removedAt := time.Now()

	m.mu.Lock()
	m.removedDockerHosts[hostID] = removedAt
	// Unbind the token so it can be reused with a different agent if needed
	if host.TokenID != "" {
		delete(m.dockerTokenBindings, host.TokenID)
		log.Debug().
			Str("tokenID", host.TokenID).
			Str("dockerHostID", hostID).
			Msg("Unbound Docker agent token from removed host")
	}
	if cmd, ok := m.dockerCommands[hostID]; ok {
		delete(m.dockerCommandIndex, cmd.status.ID)
	}
	delete(m.dockerCommands, hostID)
	m.mu.Unlock()

	m.state.AddRemovedDockerHost(models.RemovedDockerHost{
		ID:          hostID,
		Hostname:    host.Hostname,
		DisplayName: host.DisplayName,
		RemovedAt:   removedAt,
	})

	m.state.RemoveConnectionHealth(dockerConnectionPrefix + hostID)
	if m.alertManager != nil {
		m.alertManager.HandleDockerHostRemoved(host)
		m.SyncAlertState()
	}

	log.Info().
		Str("dockerHost", host.Hostname).
		Str("dockerHostID", hostID).
		Bool("removed", removed).
		Msg("Docker host removed and alerts cleared")

	return host, nil
}

// RemoveHostAgent removes a host agent from monitoring state and clears related data.
func (m *Monitor) RemoveHostAgent(hostID string) (models.Host, error) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return models.Host{}, fmt.Errorf("host id is required")
	}

	host, removed := m.state.RemoveHost(hostID)
	if !removed {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("hostID", hostID).Msg("Host not present in state during removal")
		}
		host = models.Host{
			ID:       hostID,
			Hostname: hostID,
		}
	}

	tokenID := strings.TrimSpace(host.TokenID)
	hostname := strings.TrimSpace(host.Hostname)

	tokenStillUsed := false
	if tokenID != "" && m.state != nil {
		for _, other := range m.state.GetHosts() {
			if strings.TrimSpace(other.TokenID) == tokenID {
				tokenStillUsed = true
				break
			}
		}
		if !tokenStillUsed {
			for _, other := range m.state.GetDockerHosts() {
				if strings.TrimSpace(other.TokenID) == tokenID {
					tokenStillUsed = true
					break
				}
			}
		}
	}

	var tokenRemoved *config.APITokenRecord
	if tokenID != "" && !tokenStillUsed {
		tokenRemoved = m.config.RemoveAPIToken(tokenID)
		if tokenRemoved != nil {
			m.config.SortAPITokens()

			if m.persistence != nil {
				if err := m.persistence.SaveAPITokens(m.config.APITokens); err != nil {
					log.Warn().Err(err).Str("tokenID", tokenID).Msg("Failed to persist API token revocation after host agent removal")
				} else {
					log.Info().Str("tokenID", tokenID).Str("tokenName", host.TokenName).Msg("API token revoked for removed host agent")
				}
			}
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
			key := fmt.Sprintf("%s:%s", tokenID, hostname)
			if _, exists := m.hostTokenBindings[key]; exists {
				delete(m.hostTokenBindings, key)
			}
		}

		if tokenRemoved != nil {
			prefix := tokenID + ":"
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

	// Clear LinkedHostAgentID from any nodes that were linked to this host agent
	unlinkedCount := m.state.UnlinkNodesFromHostAgent(hostID)
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
		Msg("Host agent removed from monitoring")

	if m.alertManager != nil {
		m.alertManager.HandleHostRemoved(host)
	}

	return host, nil
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
		return err
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
	CommandsEnabled *bool                  `json:"commandsEnabled,omitempty"` // nil = use agent default
	Settings        map[string]interface{} `json:"settings,omitempty"`        // Merged profile settings
	IssuedAt        *time.Time             `json:"issuedAt,omitempty"`
	ExpiresAt       *time.Time             `json:"expiresAt,omitempty"`
	Signature       string                 `json:"signature,omitempty"`
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
					cfg.Settings = p.Config
					break
				}
			}
		}
	}

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
		log.Warn().Err(err).Msg("Failed to load agent profile assignments for cache")
	} else {
		assignments = loadedAssignments
	}

	if loadedProfiles, err := m.persistence.LoadAgentProfiles(); err != nil {
		log.Warn().Err(err).Msg("Failed to load agent profiles for cache")
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

	// Also update the Host model in state for immediate UI feedback
	// The agent will confirm on its next report, but this provides instant feedback
	if commandsEnabled != nil {
		m.state.SetHostCommandsEnabled(hostID, *commandsEnabled)
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

	customName = strings.TrimSpace(customName)

	// Persist to Docker metadata store first
	var hostMeta *config.DockerHostMetadata
	if customName != "" {
		hostMeta = &config.DockerHostMetadata{
			CustomDisplayName: customName,
		}
	}
	if err := m.dockerMetadataStore.SetHostMetadata(hostID, hostMeta); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to persist Docker host metadata")
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

// AllowDockerHostReenroll removes a host ID from the removal blocklist so it can report again.
func (m *Monitor) AllowDockerHostReenroll(hostID string) error {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return fmt.Errorf("docker host id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.removedDockerHosts[hostID]; !exists {
		host, found := m.GetDockerHost(hostID)
		event := log.Info().
			Str("dockerHostID", hostID)
		if found {
			event = event.Str("dockerHost", host.Hostname)
		}
		event.Msg("Allow re-enroll requested but host was not blocked; ignoring")
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

// RebuildTokenBindings reconstructs agent-to-token binding maps from the current
// state of Docker hosts and host agents. This should be called after API tokens
// are reloaded from disk to ensure bindings remain consistent with the new token set.
// It preserves bindings for tokens that still exist and removes orphaned entries.
func (m *Monitor) RebuildTokenBindings() {
	if m == nil || m.state == nil || m.config == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Build a set of valid token IDs from the current config
	validTokens := make(map[string]struct{})
	for _, token := range m.config.APITokens {
		if token.ID != "" {
			validTokens[token.ID] = struct{}{}
		}
	}

	// Rebuild Docker token bindings
	newDockerBindings := make(map[string]string)
	dockerHosts := m.state.GetDockerHosts()
	for _, host := range dockerHosts {
		tokenID := strings.TrimSpace(host.TokenID)
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		// Use AgentID if available, otherwise fall back to host ID
		agentID := strings.TrimSpace(host.AgentID)
		if agentID == "" {
			agentID = host.ID
		}
		if agentID != "" {
			newDockerBindings[tokenID] = agentID
		}
	}

	// Rebuild Host agent token bindings
	newHostBindings := make(map[string]string)
	hosts := m.state.GetHosts()
	for _, host := range hosts {
		tokenID := strings.TrimSpace(host.TokenID)
		if tokenID == "" {
			continue
		}
		// Only keep bindings for tokens that still exist in config
		if _, valid := validTokens[tokenID]; !valid {
			continue
		}
		hostname := strings.TrimSpace(host.Hostname)
		if hostname == "" || host.ID == "" {
			continue
		}
		newHostBindings[fmt.Sprintf("%s:%s", tokenID, hostname)] = host.ID
	}

	// Log what changed
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

// ApplyDockerReport ingests a docker agent report into the shared state.
func (m *Monitor) ApplyDockerReport(report agentsdocker.Report, tokenRecord *config.APITokenRecord) (models.DockerHost, error) {
	hostsSnapshot := m.state.GetDockerHosts()
	identifier, legacyIDs, previous, hasPrevious := resolveDockerHostIdentifier(report, tokenRecord, hostsSnapshot)
	if strings.TrimSpace(identifier) == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing agent identifier")
	}

	// Check if this host was deliberately removed - reject report to prevent resurrection
	m.mu.RLock()
	removedAt, wasRemoved := m.removedDockerHosts[identifier]
	if !wasRemoved {
		for _, legacyID := range legacyIDs {
			if legacyID == "" || legacyID == identifier {
				continue
			}
			if ts, ok := m.removedDockerHosts[legacyID]; ok {
				removedAt = ts
				wasRemoved = true
				break
			}
		}
	}
	m.mu.RUnlock()

	if wasRemoved {
		log.Info().
			Str("dockerHostID", identifier).
			Time("removedAt", removedAt).
			Msg("Rejecting report from deliberately removed Docker host")
		return models.DockerHost{}, fmt.Errorf("docker host %q was removed at %v and cannot report again. Use Allow re-enroll in Settings -> Agents -> Removed Docker Hosts or rerun the installer with a docker:manage token to clear this block", identifier, removedAt.Format(time.RFC3339))
	}

	// Enforce token uniqueness: each token can only be bound to one agent
	if tokenRecord != nil && tokenRecord.ID != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		agentID := strings.TrimSpace(report.Agent.ID)
		if agentID == "" {
			agentID = identifier
		}

		m.mu.Lock()
		if boundAgentID, exists := m.dockerTokenBindings[tokenID]; exists {
			if boundAgentID != agentID {
				m.mu.Unlock()
				// Find the conflicting host to provide helpful error message
				conflictingHostname := "unknown"
				for _, host := range hostsSnapshot {
					if host.AgentID == boundAgentID || host.ID == boundAgentID {
						conflictingHostname = host.Hostname
						if host.CustomDisplayName != "" {
							conflictingHostname = host.CustomDisplayName
						} else if host.DisplayName != "" {
							conflictingHostname = host.DisplayName
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
				return models.DockerHost{}, fmt.Errorf("API token%s is already in use by agent %q (host: %s). Each Docker agent must use a unique API token. Generate a new token for this agent", tokenHint, boundAgentID, conflictingHostname)
			}
		} else {
			// First time seeing this token - bind it to this agent
			m.dockerTokenBindings[tokenID] = agentID
			log.Debug().
				Str("tokenID", tokenID).
				Str("agentID", agentID).
				Str("hostname", report.Host.Hostname).
				Msg("Bound Docker agent token to agent identity")
		}
		m.mu.Unlock()
	}

	hostname := strings.TrimSpace(report.Host.Hostname)
	if hostname == "" {
		return models.DockerHost{}, fmt.Errorf("docker report missing hostname")
	}

	timestamp := report.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

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
			ID:             payload.ID,
			Name:           payload.Name,
			Image:          payload.Image,
			ImageDigest:    payload.ImageDigest,
			State:          payload.State,
			Status:         payload.Status,
			Health:         payload.Health,
			CPUPercent:     safeFloat(payload.CPUPercent),
			MemoryUsage:    payload.MemoryUsageBytes,
			MemoryLimit:    payload.MemoryLimitBytes,
			MemoryPercent:  safeFloat(payload.MemoryPercent),
			UptimeSeconds:  payload.UptimeSeconds,
			RestartCount:   payload.RestartCount,
			ExitCode:       payload.ExitCode,
			CreatedAt:      payload.CreatedAt,
			StartedAt:      payload.StartedAt,
			FinishedAt:     payload.FinishedAt,
			NetworkRXBytes: payload.NetworkRXBytes,
			NetworkTXBytes: payload.NetworkTXBytes,
		}

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
			metrics := types.IOMetrics{
				NetworkIn:  clampUint64ToInt64(payload.NetworkRXBytes),
				NetworkOut: clampUint64ToInt64(payload.NetworkTXBytes),
				Timestamp:  timestamp,
			}
			if payload.BlockIO != nil {
				metrics.DiskRead = clampUint64ToInt64(payload.BlockIO.ReadBytes)
				metrics.DiskWrite = clampUint64ToInt64(payload.BlockIO.WriteBytes)
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

	services := convertDockerServices(report.Services)
	tasks := convertDockerTasks(report.Tasks)
	swarmInfo := convertDockerSwarmInfo(report.Host.Swarm)

	loadAverage := make([]float64, 0, len(report.Host.LoadAverage))
	if len(report.Host.LoadAverage) > 0 {
		loadAverage = append(loadAverage, report.Host.LoadAverage...)
	}

	var memory models.Memory
	if report.Host.Memory.TotalBytes > 0 || report.Host.Memory.UsedBytes > 0 {
		memory = models.Memory{
			Total:     report.Host.Memory.TotalBytes,
			Used:      report.Host.Memory.UsedBytes,
			Free:      report.Host.Memory.FreeBytes,
			Usage:     safeFloat(report.Host.Memory.Usage),
			SwapTotal: report.Host.Memory.SwapTotal,
			SwapUsed:  report.Host.Memory.SwapUsed,
		}
	}
	// Fallback: if gopsutil's memory reading failed but Docker's TotalMemoryBytes
	// is valid (possibly already a fallback from the agent), use that for Total.
	// This handles Docker-in-LXC scenarios where both Docker and gopsutil may
	// fail to read memory stats, but the agent fix provides a valid fallback.
	if memory.Total <= 0 && report.Host.TotalMemoryBytes > 0 {
		memory.Total = report.Host.TotalMemoryBytes
	}

	// Additional fallback for Docker-in-LXC: gopsutil may read Total and Free
	// correctly from cgroup limits but return 0 for Used. Calculate Used from
	// Total - Free when this happens. This fixes the "0B / 7GB" display issue.
	if memory.Used <= 0 && memory.Total > 0 && memory.Free > 0 {
		memory.Used = memory.Total - memory.Free
		if memory.Used < 0 {
			memory.Used = 0
		}
		// Recalculate usage percentage
		if memory.Total > 0 {
			memory.Usage = safePercentage(float64(memory.Used), float64(memory.Total))
		}
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
		agentVersion = normalizeAgentVersion(previous.AgentVersion)
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
		CPUs:              report.Host.TotalCPU,
		TotalMemoryBytes:  report.Host.TotalMemoryBytes,
		UptimeSeconds:     report.Host.UptimeSeconds,
		CPUUsage:          safeFloat(report.Host.CPUUsagePercent),
		LoadAverage:       loadAverage,
		Memory:            memory,
		Disks:             disks,
		NetworkInterfaces: networkIfaces,
		Status:            "online",
		LastSeen:          timestamp,
		IntervalSeconds:   report.Agent.IntervalSeconds,
		AgentVersion:      agentVersion,
		Containers:        containers,
		Services:          services,
		Tasks:             tasks,
		Swarm:             swarmInfo,
		IsLegacy:          isLegacyDockerAgent(report.Agent.Type),
	}

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
		host.TokenID = previous.TokenID
		host.TokenName = previous.TokenName
		host.TokenHint = previous.TokenHint
		host.TokenLastUsedAt = previous.TokenLastUsedAt
	}

	// Load custom display name from metadata store if not already set
	if host.CustomDisplayName == "" {
		if hostMeta := m.dockerMetadataStore.GetHostMetadata(identifier); hostMeta != nil {
			host.CustomDisplayName = hostMeta.CustomDisplayName
		}
	}

	m.state.UpsertDockerHost(host)
	m.state.SetConnectionHealth(dockerConnectionPrefix+host.ID, true)

	// Check if the host was previously hidden and is now visible again
	if hasPrevious && previous.Hidden && !host.Hidden {
		log.Info().
			Str("dockerHost", host.Hostname).
			Str("dockerHostID", host.ID).
			Msg("Docker host auto-unhidden after receiving report")
	}

	// Check if the host was pending uninstall - if so, log a warning that uninstall failed and clear the flag
	if hasPrevious && previous.PendingUninstall {
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

	// Record Docker HOST metrics for sparkline charts
	now := time.Now()
	hostMetricKey := fmt.Sprintf("dockerHost:%s", host.ID)

	// Record host CPU usage
	m.metricsHistory.AddGuestMetric(hostMetricKey, "cpu", host.CPUUsage, now)

	// Record host Memory usage
	m.metricsHistory.AddGuestMetric(hostMetricKey, "memory", host.Memory.Usage, now)

	// Record host Disk usage (use first disk or calculate total)
	var hostDiskPercent float64
	if len(host.Disks) > 0 {
		hostDiskPercent = host.Disks[0].Usage
	}
	m.metricsHistory.AddGuestMetric(hostMetricKey, "disk", hostDiskPercent, now)

	// Also write to persistent SQLite store
	if m.metricsStore != nil {
		m.metricsStore.Write("dockerHost", host.ID, "cpu", host.CPUUsage, now)
		m.metricsStore.Write("dockerHost", host.ID, "memory", host.Memory.Usage, now)
		m.metricsStore.Write("dockerHost", host.ID, "disk", hostDiskPercent, now)
	}

	// Record Docker CONTAINER metrics for sparkline charts
	// Use a prefixed key (docker:containerID) to distinguish from Proxmox containers
	for _, container := range containers {
		if container.ID == "" {
			continue
		}
		// Build a unique metric key for Docker containers
		metricKey := fmt.Sprintf("docker:%s", container.ID)

		// Record CPU (already a percentage 0-100)
		m.metricsHistory.AddGuestMetric(metricKey, "cpu", container.CPUPercent, now)

		// Record Memory (already a percentage 0-100)
		m.metricsHistory.AddGuestMetric(metricKey, "memory", container.MemoryPercent, now)

		// Record Disk usage as percentage of writable layer vs root filesystem
		var diskPercent float64
		if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
			diskPercent = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
			if diskPercent > 100 {
				diskPercent = 100
			}
		}
		m.metricsHistory.AddGuestMetric(metricKey, "disk", diskPercent, now)

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
		if container.NetInRate > 0 {
			m.metricsHistory.AddGuestMetric(metricKey, "netin", container.NetInRate, now)
		}
		if container.NetOutRate > 0 {
			m.metricsHistory.AddGuestMetric(metricKey, "netout", container.NetOutRate, now)
		}
		if diskReadRate > 0 {
			m.metricsHistory.AddGuestMetric(metricKey, "diskread", diskReadRate, now)
		}
		if diskWriteRate > 0 {
			m.metricsHistory.AddGuestMetric(metricKey, "diskwrite", diskWriteRate, now)
		}

		// Also write to persistent SQLite store for long-term storage
		if m.metricsStore != nil {
			m.metricsStore.Write("dockerContainer", container.ID, "cpu", container.CPUPercent, now)
			m.metricsStore.Write("dockerContainer", container.ID, "memory", container.MemoryPercent, now)
			m.metricsStore.Write("dockerContainer", container.ID, "disk", diskPercent, now)
			if container.NetInRate > 0 {
				m.metricsStore.Write("dockerContainer", container.ID, "netin", container.NetInRate, now)
			}
			if container.NetOutRate > 0 {
				m.metricsStore.Write("dockerContainer", container.ID, "netout", container.NetOutRate, now)
			}
			if diskReadRate > 0 {
				m.metricsStore.Write("dockerContainer", container.ID, "diskread", diskReadRate, now)
			}
			if diskWriteRate > 0 {
				m.metricsStore.Write("dockerContainer", container.ID, "diskwrite", diskWriteRate, now)
			}
		}
	}

	log.Debug().
		Str("dockerHost", host.Hostname).
		Int("containers", len(containers)).
		Msg("Docker host report processed")

	return host, nil
}

// ApplyHostReport ingests a host agent report into the shared state.
func (m *Monitor) ApplyHostReport(report agentshost.Report, tokenRecord *config.APITokenRecord) (models.Host, error) {
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
		baseIdentifier = fmt.Sprintf("host-%s", hex.EncodeToString(sum[:6]))
	}

	existingHosts := m.state.GetHosts()

	identifier := baseIdentifier
	if tokenRecord != nil && strings.TrimSpace(tokenRecord.ID) != "" {
		tokenID := strings.TrimSpace(tokenRecord.ID)
		bindingKey := fmt.Sprintf("%s:%s", tokenID, hostname)

		m.mu.Lock()
		if m.hostTokenBindings == nil {
			m.hostTokenBindings = make(map[string]string)
		}
		boundID := strings.TrimSpace(m.hostTokenBindings[bindingKey])
		m.mu.Unlock()

		// If we already have a binding for this token+hostname, use it to keep host IDs stable
		// even if another colliding host disappears later.
		if boundID != "" {
			identifier = boundID
		} else {
			bindingID := baseIdentifier
			for _, candidate := range existingHosts {
				if candidate.ID != bindingID {
					continue
				}
				if strings.TrimSpace(candidate.Hostname) == hostname && strings.TrimSpace(candidate.TokenID) == tokenID {
					break
				}

				seed := strings.Join([]string{tokenID, hostname, bindingID}, "|")
				sum := sha1.Sum([]byte(seed))
				suffix := hex.EncodeToString(sum[:4])

				base := bindingID
				if base == "" {
					base = "host"
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
			if existing := strings.TrimSpace(m.hostTokenBindings[bindingKey]); existing != "" {
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
		}
	}

	var previous models.Host
	var hasPrevious bool
	for _, candidate := range existingHosts {
		if candidate.ID == identifier {
			previous = candidate
			hasPrevious = true
			break
		}
	}

	displayName := strings.TrimSpace(report.Host.DisplayName)
	if displayName == "" {
		displayName = hostname
	}

	timestamp := report.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	memory := models.Memory{
		Total:     report.Metrics.Memory.TotalBytes,
		Used:      report.Metrics.Memory.UsedBytes,
		Free:      report.Metrics.Memory.FreeBytes,
		Usage:     safeFloat(report.Metrics.Memory.Usage),
		SwapTotal: report.Metrics.Memory.SwapTotal,
		SwapUsed:  report.Metrics.Memory.SwapUsed,
	}

	// Fallback for LXC environments: gopsutil may read Total and Free correctly
	// from cgroup limits but return 0 for Used. Calculate Used from Total - Free.
	if memory.Used <= 0 && memory.Total > 0 && memory.Free > 0 {
		memory.Used = memory.Total - memory.Free
		if memory.Used < 0 {
			memory.Used = 0
		}
	}

	if memory.Usage <= 0 && memory.Total > 0 {
		memory.Usage = safePercentage(float64(memory.Used), float64(memory.Total))
	}

	disks := make([]models.Disk, 0, len(report.Disks))
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
			RebuildPercent: array.RebuildPercent,
			RebuildSpeed:   array.RebuildSpeed,
		})
	}

	// Convert Ceph data from agent report
	var cephData *models.HostCephCluster
	if report.Ceph != nil {
		cephData = convertAgentCephToModels(report.Ceph)
	}

	host := models.Host{
		ID:                identifier,
		Hostname:          hostname,
		DisplayName:       displayName,
		Platform:          strings.TrimSpace(strings.ToLower(report.Host.Platform)),
		OSName:            strings.TrimSpace(report.Host.OSName),
		OSVersion:         strings.TrimSpace(report.Host.OSVersion),
		KernelVersion:     strings.TrimSpace(report.Host.KernelVersion),
		Architecture:      strings.TrimSpace(report.Host.Architecture),
		CPUCount:          report.Host.CPUCount,
		CPUUsage:          safeFloat(report.Metrics.CPUUsagePercent),
		LoadAverage:       append([]float64(nil), report.Host.LoadAverage...),
		Memory:            memory,
		Disks:             disks,
		DiskIO:            diskIO,
		NetworkInterfaces: network,
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: cloneStringFloatMap(report.Sensors.TemperatureCelsius),
			FanRPM:             cloneStringFloatMap(report.Sensors.FanRPM),
			Additional:         cloneStringFloatMap(report.Sensors.Additional),
			SMART:              convertAgentSMARTToModels(report.Sensors.SMART),
		},
		RAID:            raid,
		Ceph:            cephData,
		Status:          "online",
		UptimeSeconds:   report.Host.UptimeSeconds,
		IntervalSeconds: report.Agent.IntervalSeconds,
		LastSeen:        timestamp,
		AgentVersion:    strings.TrimSpace(report.Agent.Version),
		MachineID:       strings.TrimSpace(report.Host.MachineID),
		CommandsEnabled: report.Agent.CommandsEnabled,
		ReportIP:        strings.TrimSpace(report.Host.ReportIP),
		Tags:            append([]string(nil), report.Tags...),
		IsLegacy:        isLegacyHostAgent(report.Agent.Type),
	}

	// Apply any pending commands execution override from server config
	// This ensures the UI remains stable when the user toggles this setting,
	// even if the agent hasn't yet picked up the new config in this report cycle.
	if cfg := m.GetHostAgentConfig(identifier); cfg.CommandsEnabled != nil {
		host.CommandsEnabled = *cfg.CommandsEnabled
	}

	if len(host.LoadAverage) == 0 {
		host.LoadAverage = nil
	}
	if len(host.Disks) == 0 {
		host.Disks = nil
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
		host.TokenID = previous.TokenID
		host.TokenName = previous.TokenName
		host.TokenHint = previous.TokenHint
		host.TokenLastUsedAt = previous.TokenLastUsedAt
	}

	// Link host agent to matching PVE node/VM/container by hostname
	// This prevents duplication when users install agents on PVE cluster nodes
	linkedNodeID, linkedVMID, linkedContainerID := m.findLinkedProxmoxEntity(hostname)
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
	now := time.Now()

	var totalRXBytes, totalTXBytes uint64
	for _, nic := range host.NetworkInterfaces {
		totalRXBytes += nic.RXBytes
		totalTXBytes += nic.TXBytes
	}
	var totalDiskReadBytes, totalDiskWriteBytes uint64
	for _, d := range host.DiskIO {
		totalDiskReadBytes += d.ReadBytes
		totalDiskWriteBytes += d.WriteBytes
	}

	hostRateKey := fmt.Sprintf("host:%s", host.ID)
	currentMetrics := IOMetrics{
		DiskRead:   int64(totalDiskReadBytes),
		DiskWrite:  int64(totalDiskWriteBytes),
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

	// Update the linked PVE node to point back to this host agent
	if host.LinkedNodeID != "" {
		m.linkNodeToHostAgent(host.LinkedNodeID, host.ID)
	}

	// If host reports Ceph data, also update the global CephClusters state
	if report.Ceph != nil {
		cephCluster := convertAgentCephToGlobalCluster(report.Ceph, hostname, identifier, timestamp)
		m.state.UpsertCephCluster(cephCluster)
		log.Debug().
			Str("hostId", identifier).
			Str("hostname", hostname).
			Str("fsid", cephCluster.FSID).
			Str("health", cephCluster.Health).
			Int("osds", cephCluster.NumOSDs).
			Msg("Updated Ceph cluster from host agent")
	}

	if m.alertManager != nil {
		m.alertManager.CheckHost(host)
	}

	// Record Host metrics for sparkline charts
	hostMetricKey := fmt.Sprintf("host:%s", host.ID)

	var hostDiskPercent float64
	if len(host.Disks) > 0 {
		hostDiskPercent = host.Disks[0].Usage
	}

	if m.metricsHistory != nil {
		m.metricsHistory.AddGuestMetric(hostMetricKey, "cpu", host.CPUUsage, now)
		m.metricsHistory.AddGuestMetric(hostMetricKey, "memory", host.Memory.Usage, now)
		m.metricsHistory.AddGuestMetric(hostMetricKey, "disk", hostDiskPercent, now)

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

	if m.metricsStore != nil {
		m.metricsStore.Write("host", host.ID, "cpu", host.CPUUsage, now)
		m.metricsStore.Write("host", host.ID, "memory", host.Memory.Usage, now)
		m.metricsStore.Write("host", host.ID, "disk", hostDiskPercent, now)
		if netInRate >= 0 {
			m.metricsStore.Write("host", host.ID, "netin", netInRate, now)
		}
		if netOutRate >= 0 {
			m.metricsStore.Write("host", host.ID, "netout", netOutRate, now)
		}
		if diskReadRate >= 0 {
			m.metricsStore.Write("host", host.ID, "diskread", diskReadRate, now)
		}
		if diskWriteRate >= 0 {
			m.metricsStore.Write("host", host.ID, "diskwrite", diskWriteRate, now)
		}
	}

	return host, nil
}

// findLinkedProxmoxEntity searches for a PVE node, VM, or container with a matching hostname.
// Returns the IDs of matched entities (empty string if no match).
// When multiple entities match the same hostname (e.g., two PVE instances both have a node
// named "pve"), this function returns empty strings to avoid incorrect linking. Users should
// manually link agents to nodes via the UI in such cases.
func (m *Monitor) findLinkedProxmoxEntity(hostname string) (nodeID, vmID, containerID string) {
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

	state := m.GetState()

	// Check PVE nodes first - but detect ambiguity when multiple nodes match
	var matchingNodes []models.Node
	for _, node := range state.Nodes {
		if matchHostname(node.Name) {
			matchingNodes = append(matchingNodes, node)
		}
	}
	if len(matchingNodes) == 1 {
		return matchingNodes[0].ID, "", ""
	}
	if len(matchingNodes) > 1 {
		// Multiple nodes with the same hostname - can't auto-link, would cause data mixing
		log.Warn().
			Str("hostname", hostname).
			Int("matchCount", len(matchingNodes)).
			Strs("instances", func() []string {
				instances := make([]string, len(matchingNodes))
				for i, n := range matchingNodes {
					instances[i] = n.Instance
				}
				return instances
			}()).
			Msg("Multiple PVE nodes match hostname - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	// Check VMs - same pattern for ambiguity detection
	var matchingVMs []models.VM
	for _, vm := range state.VMs {
		if matchHostname(vm.Name) {
			matchingVMs = append(matchingVMs, vm)
		}
	}
	if len(matchingVMs) == 1 {
		return "", matchingVMs[0].ID, ""
	}
	if len(matchingVMs) > 1 {
		log.Warn().
			Str("hostname", hostname).
			Int("matchCount", len(matchingVMs)).
			Msg("Multiple VMs match hostname - cannot auto-link host agent. Manual linking required via UI.")
		return "", "", ""
	}

	// Check containers - same pattern
	var matchingCTs []models.Container
	for _, ct := range state.Containers {
		if matchHostname(ct.Name) {
			matchingCTs = append(matchingCTs, ct)
		}
	}
	if len(matchingCTs) == 1 {
		return "", "", matchingCTs[0].ID
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

// cleanupRemovedDockerHosts removes entries from the removed hosts map that are older than 24 hours.
func (m *Monitor) cleanupRemovedDockerHosts(now time.Time) {
	// Collect IDs to remove first to avoid holding lock during state update
	var toRemove []string

	m.mu.Lock()
	for hostID, removedAt := range m.removedDockerHosts {
		if now.Sub(removedAt) > removedDockerHostsTTL {
			toRemove = append(toRemove, hostID)
		}
	}
	m.mu.Unlock()

	// Remove from state and map without holding both locks
	for _, hostID := range toRemove {
		m.state.RemoveRemovedDockerHost(hostID)

		m.mu.Lock()
		removedAt := m.removedDockerHosts[hostID]
		delete(m.removedDockerHosts, hostID)
		m.mu.Unlock()

		log.Debug().
			Str("dockerHostID", hostID).
			Time("removedAt", removedAt).
			Msg("Cleaned up old removed Docker host entry")
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

	// Clean up circuit breakers for keys not in active clients
	// Build set of active keys from pveClients and pbsClients
	activeKeys := make(map[string]struct{})
	for key := range m.pveClients {
		activeKeys[key] = struct{}{}
	}
	for key := range m.pbsClients {
		activeKeys[key] = struct{}{}
	}
	for key := range m.pmgClients {
		activeKeys[key] = struct{}{}
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
// Entries older than 2x the cache TTL (1 minute) are removed to prevent unbounded growth
// when nodes are removed from the cluster.
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

// evaluateHostAgents updates health for host agents based on last report time.
func (m *Monitor) evaluateHostAgents(now time.Time) {
	hosts := m.state.GetHosts()
	for _, host := range hosts {
		interval := host.IntervalSeconds
		if interval <= 0 {
			interval = int(hostMinimumHealthWindow / time.Second)
		}

		window := time.Duration(interval) * time.Second * hostOfflineGraceMultiplier
		if window < hostMinimumHealthWindow {
			window = hostMinimumHealthWindow
		} else if window > hostMaximumHealthWindow {
			window = hostMaximumHealthWindow
		}

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
			m.state.SetHostStatus(host.ID, "online")
			if m.alertManager != nil {
				m.alertManager.HandleHostOnline(hostCopy)
			}
		} else {
			hostCopy.Status = "offline"
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
			m.state.SetHostStatus(host.ID, "offline")
			if m.alertManager != nil {
				m.alertManager.HandleHostOffline(hostCopy)
			}
		}
	}
}

// sortContent sorts comma-separated content values for consistent display
