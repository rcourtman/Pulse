package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/fleethealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

const (
	AgentFleetDiagnosticsSchemaVersion = 1

	AgentFleetStatusHealthy  = "healthy"
	AgentFleetStatusWarning  = "warning"
	AgentFleetStatusCritical = "critical"
	AgentFleetStatusRemoved  = "removed"

	AgentFleetReasonUpdateDisabled        = "agent_update_disabled"
	AgentFleetReasonUpdateFailed          = "agent_update_failed"
	AgentFleetReasonUpdateStateUnknown    = "agent_update_state_unknown"
	AgentFleetReasonUpdatePlatformUnknown = "agent_update_platform_unknown"
	AgentFleetReasonUpdateStateUnverified = "agent_update_installer_state_unverified"
	AgentFleetReasonModuleFailed          = "agent_module_failed"
	AgentFleetReasonModuleDegraded        = "agent_module_degraded"

	AgentFleetActionAllowReenroll      = "allow_reenroll"
	AgentFleetActionCopyUpgradeCommand = "copy_upgrade_command"
	AgentFleetRepairModeHandoff        = "handoff"
)

type AgentFleetDiagnostics struct {
	SchemaVersion            int                         `json:"schemaVersion"`
	GeneratedAt              int64                       `json:"generatedAt"`
	ServerVersion            string                      `json:"serverVersion,omitempty"`
	AgentUpdateTargetVersion string                      `json:"agentUpdateTargetVersion,omitempty"`
	Summary                  AgentFleetDiagnosticSummary `json:"summary"`
	Agents                   []AgentFleetAgentDiagnostic `json:"agents"`
}

type AgentFleetDiagnosticSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Removed  int `json:"removed"`
}

type AgentFleetAgentDiagnostic struct {
	RowKey                 string                       `json:"rowKey"`
	ID                     string                       `json:"id"`
	AgentID                string                       `json:"agentId,omitempty"`
	ConnectionID           string                       `json:"connectionId,omitempty"`
	Name                   string                       `json:"name"`
	Hostname               string                       `json:"hostname,omitempty"`
	Platform               string                       `json:"platform,omitempty"`
	OSName                 string                       `json:"osName,omitempty"`
	OSVersion              string                       `json:"osVersion,omitempty"`
	KernelVersion          string                       `json:"kernelVersion,omitempty"`
	Architecture           string                       `json:"architecture,omitempty"`
	MachineIDFingerprint   string                       `json:"machineIdFingerprint,omitempty"`
	ReportIP               string                       `json:"reportIp,omitempty"`
	InterfaceAddresses     []string                     `json:"interfaceAddresses,omitempty"`
	Types                  []string                     `json:"types"`
	Status                 string                       `json:"status"`
	RawStatus              string                       `json:"rawStatus,omitempty"`
	LastSeen               int64                        `json:"lastSeen,omitempty"`
	IntervalSeconds        int                          `json:"intervalSeconds,omitempty"`
	Version                string                       `json:"version,omitempty"`
	ProfileID              string                       `json:"profileId,omitempty"`
	ProfileName            string                       `json:"profileName,omitempty"`
	ProfileVersion         int                          `json:"profileVersion,omitempty"`
	DeployedProfileVersion int                          `json:"deployedProfileVersion,omitempty"`
	AgentUpdate            *AgentFleetDiagnosticUpdate  `json:"agentUpdate,omitempty"`
	AgentModules           []AgentFleetDiagnosticModule `json:"agentModules,omitempty"`
	Reasons                []AgentFleetDiagnosticReason `json:"reasons"`
	RepairActions          []AgentFleetDiagnosticRepair `json:"repairActions,omitempty"`
}

type AgentFleetDiagnosticUpdate struct {
	State            string     `json:"state"`
	AutoUpdate       bool       `json:"autoUpdate"`
	UpdatedFrom      string     `json:"updatedFrom,omitempty"`
	AvailableVersion string     `json:"availableVersion,omitempty"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty"`
	LastAttemptAt    *time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt    *time.Time `json:"lastSuccessAt,omitempty"`
	LastError        string     `json:"lastError,omitempty"`
}

type AgentFleetDiagnosticModule struct {
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	State     string    `json:"state"`
	LastError string    `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AgentFleetDiagnosticReason struct {
	Code     string   `json:"code"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Evidence []string `json:"evidence,omitempty"`
}

type AgentFleetDiagnosticRepair struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
	Mode        string `json:"mode"`
	Platform    string `json:"platform,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type agentFleetSubject struct {
	rowKey          string
	id              string
	agentID         string
	name            string
	hostname        string
	types           map[string]struct{}
	rawStatus       string
	lastSeen        time.Time
	intervalSeconds int
	version         string
	tokenID         string
	host            *models.Host
	docker          *models.DockerHost
	kubernetes      *models.KubernetesCluster
	removed         bool
	removedAt       time.Time
}

// GetAgentFleetDiagnostics preserves the original call contract for callers
// where the server version is also the agent update target.
func (m *Monitor) GetAgentFleetDiagnostics(serverVersion string, now time.Time) AgentFleetDiagnostics {
	return m.GetAgentFleetDiagnosticsForTarget(serverVersion, serverVersion, now)
}

// GetAgentFleetDiagnosticsForTarget returns a read-only fleet health view
// derived from reported state, the canonical agent update target, and existing
// profile deployment state.
func (m *Monitor) GetAgentFleetDiagnosticsForTarget(serverVersion, agentUpdateTargetVersion string, now time.Time) AgentFleetDiagnostics {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	out := AgentFleetDiagnostics{
		SchemaVersion:            AgentFleetDiagnosticsSchemaVersion,
		GeneratedAt:              now.UnixMilli(),
		ServerVersion:            strings.TrimSpace(serverVersion),
		AgentUpdateTargetVersion: strings.TrimSpace(agentUpdateTargetVersion),
	}
	if m == nil || m.state == nil {
		return out
	}

	state := m.GetState()
	profiles, assignments, deployments := m.agentFleetProfileState()
	profileByID := mapProfilesByID(profiles)
	assignmentByAgent := mapAssignmentsByAgent(assignments)
	deploymentByAgentProfile := mapDeploymentsByAgentProfile(deployments)
	subjects := buildAgentFleetSubjects(state)

	for i := range subjects {
		diagnostic := diagnoseAgentFleetSubject(subjects[i], state, out.AgentUpdateTargetVersion, now, profileByID, assignmentByAgent, deploymentByAgentProfile)
		out.Agents = append(out.Agents, diagnostic)
	}

	sort.Slice(out.Agents, func(i, j int) bool {
		if out.Agents[i].Status != out.Agents[j].Status {
			return agentFleetStatusRank(out.Agents[i].Status) > agentFleetStatusRank(out.Agents[j].Status)
		}
		return strings.ToLower(out.Agents[i].Name) < strings.ToLower(out.Agents[j].Name)
	})

	out.Summary.Total = len(out.Agents)
	for _, agent := range out.Agents {
		switch agent.Status {
		case AgentFleetStatusCritical:
			out.Summary.Critical++
		case AgentFleetStatusWarning:
			out.Summary.Warning++
		case AgentFleetStatusRemoved:
			out.Summary.Removed++
		default:
			out.Summary.Healthy++
		}
	}

	return out
}

func (m *Monitor) agentFleetProfileState() ([]models.AgentProfile, []models.AgentProfileAssignment, []models.ProfileDeploymentStatus) {
	if m == nil || m.persistence == nil {
		return nil, nil, nil
	}

	profiles, assignments := m.getAgentProfileCache()
	deployments, err := m.persistence.LoadProfileDeploymentStatus()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load agent profile deployment status for fleet diagnostics")
	}
	return profiles, assignments, deployments
}

func buildAgentFleetSubjects(state models.StateSnapshot) []agentFleetSubject {
	subjectsByID := make(map[string]*agentFleetSubject)
	order := make([]string, 0, len(state.Hosts)+len(state.DockerHosts)+len(state.KubernetesClusters))

	getOrCreate := func(id string) *agentFleetSubject {
		id = strings.TrimSpace(id)
		if existing := subjectsByID[id]; existing != nil {
			return existing
		}
		subject := &agentFleetSubject{
			rowKey: "agent-" + id,
			id:     id,
			types:  map[string]struct{}{},
		}
		subjectsByID[id] = subject
		order = append(order, id)
		return subject
	}

	for i := range state.Hosts {
		host := state.Hosts[i]
		if strings.TrimSpace(host.ID) == "" {
			continue
		}
		subject := getOrCreate(host.ID)
		subject.host = &host
		subject.types["host"] = struct{}{}
		subject.agentID = firstNonEmpty(subject.agentID, host.ID)
		subject.name = firstNonEmpty(host.DisplayName, host.Hostname, host.ID)
		subject.hostname = firstNonEmpty(host.Hostname, subject.hostname)
		subject.rawStatus = firstNonEmpty(host.Status, subject.rawStatus)
		subject.lastSeen = newestTime(subject.lastSeen, host.LastSeen)
		subject.intervalSeconds = maxInt(subject.intervalSeconds, host.IntervalSeconds)
		subject.version = firstNonEmpty(host.AgentVersion, subject.version)
		subject.tokenID = firstNonEmpty(host.TokenID, subject.tokenID)
	}

	for i := range state.DockerHosts {
		docker := state.DockerHosts[i]
		if strings.TrimSpace(docker.ID) == "" || docker.Hidden {
			continue
		}
		subject := getOrCreate(docker.ID)
		subject.docker = &docker
		subject.types["docker"] = struct{}{}
		subject.agentID = firstNonEmpty(subject.agentID, docker.AgentID, docker.ID)
		subject.name = firstNonEmpty(docker.CustomDisplayName, docker.DisplayName, docker.Hostname, docker.ID, subject.name)
		subject.hostname = firstNonEmpty(docker.Hostname, subject.hostname)
		subject.rawStatus = firstNonEmpty(docker.Status, subject.rawStatus)
		subject.lastSeen = newestTime(subject.lastSeen, docker.LastSeen)
		subject.intervalSeconds = maxInt(subject.intervalSeconds, docker.IntervalSeconds)
		subject.version = firstNonEmpty(subject.version, docker.AgentVersion, docker.DockerVersion)
		subject.tokenID = firstNonEmpty(subject.tokenID, docker.TokenID)
	}

	for i := range state.KubernetesClusters {
		cluster := state.KubernetesClusters[i]
		if strings.TrimSpace(cluster.ID) == "" || cluster.Hidden {
			continue
		}
		id := "k8s:" + cluster.ID
		subject := &agentFleetSubject{
			rowKey:          "k8s-" + cluster.ID,
			id:              cluster.ID,
			agentID:         firstNonEmpty(cluster.AgentID, cluster.ID),
			name:            firstNonEmpty(cluster.CustomDisplayName, cluster.DisplayName, cluster.Name, cluster.ID),
			types:           map[string]struct{}{"kubernetes": {}},
			rawStatus:       cluster.Status,
			lastSeen:        cluster.LastSeen,
			intervalSeconds: cluster.IntervalSeconds,
			version:         firstNonEmpty(cluster.AgentVersion, cluster.Version),
			tokenID:         cluster.TokenID,
			kubernetes:      &cluster,
		}
		subjectsByID[id] = subject
		order = append(order, id)
	}

	for i := range state.RemovedDockerHosts {
		removed := state.RemovedDockerHosts[i]
		subjectsByID["removed-docker:"+removed.ID] = &agentFleetSubject{
			rowKey:    "removed-docker-" + removed.ID,
			id:        removed.ID,
			name:      firstNonEmpty(removed.DisplayName, removed.Hostname, removed.ID),
			hostname:  removed.Hostname,
			types:     map[string]struct{}{"docker": {}},
			removed:   true,
			removedAt: removed.RemovedAt,
		}
		order = append(order, "removed-docker:"+removed.ID)
	}

	for i := range state.RemovedHostAgents {
		removed := state.RemovedHostAgents[i]
		subjectsByID["removed-host:"+removed.ID] = &agentFleetSubject{
			rowKey:    "removed-host-" + removed.ID,
			id:        removed.ID,
			name:      firstNonEmpty(removed.DisplayName, removed.Hostname, removed.ID),
			hostname:  removed.Hostname,
			types:     map[string]struct{}{"host": {}},
			removed:   true,
			removedAt: removed.RemovedAt,
		}
		order = append(order, "removed-host:"+removed.ID)
	}

	for i := range state.RemovedKubernetesClusters {
		removed := state.RemovedKubernetesClusters[i]
		subjectsByID["removed-k8s:"+removed.ID] = &agentFleetSubject{
			rowKey:    "removed-k8s-" + removed.ID,
			id:        removed.ID,
			name:      firstNonEmpty(removed.DisplayName, removed.Name, removed.ID),
			types:     map[string]struct{}{"kubernetes": {}},
			removed:   true,
			removedAt: removed.RemovedAt,
		}
		order = append(order, "removed-k8s:"+removed.ID)
	}

	subjects := make([]agentFleetSubject, 0, len(order))
	for _, key := range order {
		if subject := subjectsByID[key]; subject != nil {
			subjects = append(subjects, *subject)
		}
	}
	return subjects
}

func diagnoseAgentFleetSubject(
	subject agentFleetSubject,
	state models.StateSnapshot,
	serverVersion string,
	now time.Time,
	profileByID map[string]models.AgentProfile,
	assignmentByAgent map[string]models.AgentProfileAssignment,
	deploymentByAgentProfile map[string]models.ProfileDeploymentStatus,
) AgentFleetAgentDiagnostic {
	identity := agentFleetIdentityForSubject(subject)
	result := AgentFleetAgentDiagnostic{
		RowKey:               subject.rowKey,
		ID:                   subject.id,
		AgentID:              subject.agentID,
		ConnectionID:         identity.connectionID,
		Name:                 firstNonEmpty(subject.name, subject.hostname, subject.id),
		Hostname:             subject.hostname,
		Platform:             identity.platform,
		OSName:               identity.osName,
		OSVersion:            identity.osVersion,
		KernelVersion:        identity.kernelVersion,
		Architecture:         identity.architecture,
		MachineIDFingerprint: identity.machineIDFingerprint,
		ReportIP:             identity.reportIP,
		InterfaceAddresses:   identity.interfaceAddresses,
		Types:                sortedAgentTypes(subject.types),
		RawStatus:            subject.rawStatus,
		IntervalSeconds:      subject.intervalSeconds,
		Version:              subject.version,
		AgentUpdate:          agentFleetUpdateForSubject(subject),
		AgentModules:         agentFleetModulesForSubject(subject),
	}
	if !subject.lastSeen.IsZero() {
		result.LastSeen = subject.lastSeen.UnixMilli()
	}

	if subject.removed {
		result.Status = AgentFleetStatusRemoved
		if !subject.removedAt.IsZero() {
			result.LastSeen = subject.removedAt.UnixMilli()
		}
		result.Reasons = append(result.Reasons, AgentFleetDiagnosticReason{
			Code:     "agent_removed_blocked",
			Severity: AgentFleetStatusWarning,
			Message:  "This agent is intentionally removed and blocked from re-enrolling until an admin allows it.",
			Evidence: []string{
				fmt.Sprintf("Removed at %s", subject.removedAt.UTC().Format(time.RFC3339)),
			},
		})
		result.RepairActions = append(result.RepairActions, AgentFleetDiagnosticRepair{
			Code:        AgentFleetActionAllowReenroll,
			Label:       "Allow re-enroll",
			Description: "Uses the existing allow re-enroll action for removed agents.",
			Supported:   true,
			Mode:        AgentFleetRepairModeHandoff,
			Scope:       "settings:write",
		})
		return result
	}

	result.Reasons = append(result.Reasons, diagnoseAgentConnectivity(subject, now)...)
	result.Reasons = append(result.Reasons, diagnoseAgentVersion(subject, serverVersion)...)
	result.Reasons = append(result.Reasons, diagnoseAgentUpdate(subject, serverVersion)...)
	result.Reasons = append(result.Reasons, diagnoseAgentModules(subject)...)
	result.Reasons = append(result.Reasons, diagnoseAgentIdentitySplit(subject, state)...)

	assignment, hasAssignment := findAgentAssignment(subject, assignmentByAgent)
	if hasAssignment {
		result.ProfileID = assignment.ProfileID
		profile, hasProfile := profileByID[assignment.ProfileID]
		if hasProfile {
			result.ProfileName = profile.Name
			result.ProfileVersion = expectedProfileVersion(profile, assignment)
			result.Reasons = append(result.Reasons, diagnoseProfileDeployment(subject, assignment, profile, deploymentByAgentProfile)...)
			result.Reasons = append(result.Reasons, diagnoseProfileCapabilityDrift(subject, state, profile)...)
		} else {
			result.Reasons = append(result.Reasons, AgentFleetDiagnosticReason{
				Code:     "profile_missing",
				Severity: AgentFleetStatusWarning,
				Message:  "A profile is assigned to this agent, but that profile no longer exists.",
				Evidence: []string{
					"Assigned profile ID: " + assignment.ProfileID,
				},
			})
		}

		if deployment, ok := deploymentByAgentProfile[deploymentKey(assignment.AgentID, assignment.ProfileID)]; ok {
			result.DeployedProfileVersion = deployment.DeployedVersion
		}
	}

	for _, reason := range result.Reasons {
		if reason.Code == "agent_version_stale" {
			platform, supported := safeAgentUpdatePlatform(subject)
			if !supported {
				result.Reasons = append(result.Reasons, AgentFleetDiagnosticReason{
					Code:     AgentFleetReasonUpdatePlatformUnknown,
					Severity: AgentFleetStatusWarning,
					Message:  "Pulse cannot safely choose an agent installer command because the reported platform is missing or unsupported.",
				})
			} else if platform == platformsupport.RuntimePlatformFreeBSD {
				supported = false
				result.Reasons = append(result.Reasons, AgentFleetDiagnosticReason{
					Code:     AgentFleetReasonUpdateStateUnverified,
					Severity: AgentFleetStatusWarning,
					Message:  "Pulse cannot verify the saved FreeBSD or pfSense installer state required for a safe in-place update.",
				})
			}
			result.RepairActions = append(result.RepairActions, AgentFleetDiagnosticRepair{
				Code:        AgentFleetActionCopyUpgradeCommand,
				Label:       "Copy upgrade command",
				Description: "Uses the existing installer command from Settings -> Agents; no remote command is queued.",
				Supported:   supported,
				Mode:        AgentFleetRepairModeHandoff,
				Platform:    platform,
				Scope:       "local_admin_shell",
			})
			break
		}
	}

	result.Status = diagnosticStatusFromReasons(result.Reasons)
	return result
}

func diagnoseAgentConnectivity(subject agentFleetSubject, now time.Time) []AgentFleetDiagnosticReason {
	if subject.kubernetes == nil && subject.host == nil && subject.docker == nil {
		return nil
	}

	reasons := []AgentFleetDiagnosticReason{}
	interval := subject.intervalSeconds
	if interval <= 0 {
		interval = 30
	}

	staleThreshold := fleethealth.AgentStaleThreshold(subject.intervalSeconds)
	switch fleethealth.DeriveAgentLiveness(subject.lastSeen, now, subject.intervalSeconds) {
	case fleethealth.AgentLivenessPending:
		return append(reasons, AgentFleetDiagnosticReason{
			Code:     "agent_never_reported",
			Severity: AgentFleetStatusCritical,
			Message:  "This agent has no last-seen timestamp, so Pulse cannot confirm it is reporting.",
		})
	case fleethealth.AgentLivenessStale:
		age := now.Sub(subject.lastSeen)
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "agent_disconnected",
			Severity: AgentFleetStatusCritical,
			Message:  fmt.Sprintf("No report has arrived for %s. Pulse marks an agent stale after %s without a report.", formatFleetDuration(age), formatFleetDuration(staleThreshold)),
			Evidence: []string{
				"Last seen: " + subject.lastSeen.UTC().Format(time.RFC3339),
				fmt.Sprintf("Expected report interval: %ds", interval),
			},
		})
	}

	if subject.rawStatus != "" && !agentFleetStatusIsOnline(subject.rawStatus) {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "agent_status_not_online",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The agent's most recent report described it as %q rather than online.", subject.rawStatus),
		})
	}

	return reasons
}

func diagnoseAgentVersion(subject agentFleetSubject, targetVersion string) []AgentFleetDiagnosticReason {
	if subject.removed {
		return nil
	}
	agentVersion := strings.TrimSpace(subject.version)
	if agentVersion == "" {
		return []AgentFleetDiagnosticReason{{
			Code:     "agent_version_missing",
			Severity: AgentFleetStatusWarning,
			Message:  "The agent did not report a version, so Pulse cannot verify update health.",
		}}
	}

	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" || strings.EqualFold(targetVersion, "dev") {
		return nil
	}

	if _, err := updates.ParseVersion(targetVersion); err != nil {
		return nil
	}
	if _, err := updates.ParseVersion(agentVersion); err != nil {
		return []AgentFleetDiagnosticReason{{
			Code:     "agent_version_unparseable",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The agent reported version %q, which cannot be compared with update target %q.", agentVersion, targetVersion),
		}}
	}

	if fleethealth.DeriveAgentVersionDrift(agentVersion, targetVersion) == fleethealth.AgentVersionBehind {
		return []AgentFleetDiagnosticReason{{
			Code:     "agent_version_stale",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("Agent version %s is older than the agent update target %s.", agentVersion, targetVersion),
			Evidence: []string{
				"Agent version: " + agentVersion,
				"Agent update target: " + targetVersion,
			},
		}}
	}

	return nil
}

func diagnoseAgentUpdate(subject agentFleetSubject, targetVersion string) []AgentFleetDiagnosticReason {
	if subject.host == nil || subject.host.AgentUpdate == nil {
		return nil
	}

	update := subject.host.AgentUpdate
	state := strings.ToLower(strings.TrimSpace(update.State))
	evidence := updateStatusEvidence(update)
	switch state {
	case "", "idle", "checking", "update-available", "updating":
		return nil
	case "disabled":
		if fleethealth.DeriveAgentVersionDrift(subject.version, targetVersion) != fleethealth.AgentVersionBehind {
			return nil
		}
		return []AgentFleetDiagnosticReason{{
			Code:     AgentFleetReasonUpdateDisabled,
			Severity: AgentFleetStatusWarning,
			Message:  "The agent is behind the update target and its automatic updater is disabled.",
			Evidence: evidence,
		}}
	case "error":
		return []AgentFleetDiagnosticReason{{
			Code:     AgentFleetReasonUpdateFailed,
			Severity: AgentFleetStatusWarning,
			Message:  "The agent's most recent self-update check or attempt failed.",
			Evidence: evidence,
		}}
	default:
		return []AgentFleetDiagnosticReason{{
			Code:     AgentFleetReasonUpdateStateUnknown,
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The agent reported an unrecognized updater state %q.", update.State),
			Evidence: evidence,
		}}
	}
}

func diagnoseAgentModules(subject agentFleetSubject) []AgentFleetDiagnosticReason {
	if subject.host == nil {
		return nil
	}

	reasons := make([]AgentFleetDiagnosticReason, 0)
	for _, module := range subject.host.AgentModules {
		if !module.Enabled {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(module.State))
		if state == "running" {
			continue
		}
		severity := AgentFleetStatusWarning
		code := AgentFleetReasonModuleDegraded
		message := fmt.Sprintf("Enabled agent module %q is not running.", strings.TrimSpace(module.Name))
		if state == "error" || state == "failed" {
			severity = AgentFleetStatusCritical
			code = AgentFleetReasonModuleFailed
			message = fmt.Sprintf("Enabled agent module %q failed.", strings.TrimSpace(module.Name))
		}
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     code,
			Severity: severity,
			Message:  message,
			Evidence: moduleStatusEvidence(module),
		})
	}
	return reasons
}

type agentFleetIdentityEvidence struct {
	connectionID         string
	platform             string
	osName               string
	osVersion            string
	kernelVersion        string
	architecture         string
	machineIDFingerprint string
	reportIP             string
	interfaceAddresses   []string
}

func agentFleetIdentityForSubject(subject agentFleetSubject) agentFleetIdentityEvidence {
	identity := agentFleetIdentityEvidence{}
	var machineID string
	var interfaces []models.HostNetworkInterface

	if subject.host != nil {
		host := subject.host
		identity.connectionID = fleethealth.AgentConnectionID(host.ID)
		identity.platform, _ = safeAgentUpdatePlatform(subject)
		if identity.platform == "" {
			identity.platform = platformsupport.NormalizeAgentReportedPlatform(host.Platform)
		}
		identity.osName = strings.TrimSpace(host.OSName)
		identity.osVersion = strings.TrimSpace(host.OSVersion)
		identity.kernelVersion = strings.TrimSpace(host.KernelVersion)
		identity.architecture = strings.TrimSpace(host.Architecture)
		identity.reportIP = safeDiagnosticIPAddress(host.ReportIP)
		machineID = host.MachineID
		interfaces = append(interfaces, host.NetworkInterfaces...)
	}

	if subject.docker != nil {
		docker := subject.docker
		if identity.platform == "" {
			identity.platform, _ = safeAgentUpdatePlatform(subject)
			if identity.platform == "" {
				identity.platform = platformsupport.NormalizeAgentReportedPlatform(docker.OS)
			}
		}
		identity.osName = firstNonEmpty(identity.osName, docker.OS)
		identity.kernelVersion = firstNonEmpty(identity.kernelVersion, docker.KernelVersion)
		identity.architecture = firstNonEmpty(identity.architecture, docker.Architecture)
		machineID = firstNonEmpty(machineID, docker.MachineID)
		interfaces = append(interfaces, docker.NetworkInterfaces...)
	}

	identity.machineIDFingerprint = machineIDFingerprint(machineID)
	identity.interfaceAddresses = safeDiagnosticInterfaceAddresses(interfaces)
	return identity
}

func agentFleetUpdateForSubject(subject agentFleetSubject) *AgentFleetDiagnosticUpdate {
	if subject.host == nil || subject.host.AgentUpdate == nil {
		return nil
	}
	update := subject.host.AgentUpdate
	return &AgentFleetDiagnosticUpdate{
		State:            strings.TrimSpace(update.State),
		AutoUpdate:       update.AutoUpdate,
		UpdatedFrom:      strings.TrimSpace(update.UpdatedFrom),
		AvailableVersion: strings.TrimSpace(update.AvailableVersion),
		LastCheckedAt:    cloneFleetDiagnosticTime(update.LastCheckedAt),
		LastAttemptAt:    cloneFleetDiagnosticTime(update.LastAttemptAt),
		LastSuccessAt:    cloneFleetDiagnosticTime(update.LastSuccessAt),
		LastError:        safeDiagnosticError(update.LastError),
	}
}

func agentFleetModulesForSubject(subject agentFleetSubject) []AgentFleetDiagnosticModule {
	if subject.host == nil || len(subject.host.AgentModules) == 0 {
		return nil
	}
	modules := make([]AgentFleetDiagnosticModule, 0, len(subject.host.AgentModules))
	for _, module := range subject.host.AgentModules {
		modules = append(modules, AgentFleetDiagnosticModule{
			Name:      strings.TrimSpace(module.Name),
			Enabled:   module.Enabled,
			State:     strings.TrimSpace(module.State),
			LastError: safeDiagnosticError(module.LastError),
			UpdatedAt: module.UpdatedAt.UTC(),
		})
	}
	sort.Slice(modules, func(i, j int) bool {
		return strings.ToLower(modules[i].Name) < strings.ToLower(modules[j].Name)
	})
	return modules
}

func safeAgentUpdatePlatform(subject agentFleetSubject) (string, bool) {
	values := make([]string, 0, 3)
	if subject.host != nil {
		values = append(values, subject.host.Platform, subject.host.OSName)
	}
	if subject.docker != nil {
		values = append(values, subject.docker.OS)
	}

	for _, value := range values {
		appliance := strings.ToLower(strings.TrimSpace(value))
		if strings.Contains(appliance, "pfsense") || strings.Contains(appliance, "opnsense") {
			return platformsupport.RuntimePlatformFreeBSD, true
		}
		normalized := platformsupport.NormalizeAgentReportedPlatform(value)
		switch normalized {
		case platformsupport.RuntimePlatformWindows,
			platformsupport.RuntimePlatformMacOS,
			platformsupport.RuntimePlatformFreeBSD,
			platformsupport.RuntimePlatformLinux:
			return normalized, true
		}
		if knownLinuxAgentPlatform(normalized) {
			return platformsupport.RuntimePlatformLinux, true
		}
	}
	return "", false
}

func knownLinuxAgentPlatform(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, token := range []string{
		"almalinux", "alpine", "amazon linux", "arch", "centos", "debian",
		"fedora", "gentoo", "linux", "nixos", "opensuse", "oracle linux",
		"proxmox", "qnap", "raspbian", "red hat", "rhel", "rocky", "suse",
		"synology", "ubuntu", "unraid",
	} {
		if value == token || strings.Contains(value, token+" ") || strings.Contains(value, token+"-") {
			return true
		}
	}
	return false
}

func machineIDFingerprint(machineID string) string {
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(machineID))
	return "sha256:" + hex.EncodeToString(digest[:8])
}

func safeDiagnosticInterfaceAddresses(interfaces []models.HostNetworkInterface) []string {
	const maxAddresses = 32
	set := make(map[string]struct{})
	for _, iface := range interfaces {
		for _, value := range iface.Addresses {
			if normalized := safeDiagnosticIPAddress(value); normalized != "" {
				set[normalized] = struct{}{}
			}
		}
	}
	addresses := make([]string, 0, len(set))
	for value := range set {
		addresses = append(addresses, value)
	}
	sort.Strings(addresses)
	if len(addresses) > maxAddresses {
		addresses = addresses[:maxAddresses]
	}
	return addresses
}

func safeDiagnosticIPAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return address.String()
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.String()
	}
	return ""
}

func updateStatusEvidence(update *models.AgentUpdateStatus) []string {
	if update == nil {
		return nil
	}
	evidence := nonEmptyStrings("Updater state: " + strings.TrimSpace(update.State))
	if update.LastCheckedAt != nil {
		evidence = append(evidence, "Last checked: "+update.LastCheckedAt.UTC().Format(time.RFC3339))
	}
	if update.LastAttemptAt != nil {
		evidence = append(evidence, "Last attempted: "+update.LastAttemptAt.UTC().Format(time.RFC3339))
	}
	if lastError := safeDiagnosticError(update.LastError); lastError != "" {
		evidence = append(evidence, "Last error: "+lastError)
	}
	return evidence
}

func moduleStatusEvidence(module models.AgentModuleStatus) []string {
	evidence := nonEmptyStrings(
		"Module: "+strings.TrimSpace(module.Name),
		"Module state: "+strings.TrimSpace(module.State),
	)
	if !module.UpdatedAt.IsZero() {
		evidence = append(evidence, "Updated at: "+module.UpdatedAt.UTC().Format(time.RFC3339))
	}
	if lastError := safeDiagnosticError(module.LastError); lastError != "" {
		evidence = append(evidence, "Last error: "+lastError)
	}
	return evidence
}

func safeDiagnosticError(value string) string {
	const maxErrorLength = 512
	value = strings.TrimSpace(unifiedresources.RedactAuditText(value))
	runes := []rune(value)
	if len(runes) > maxErrorLength {
		value = string(runes[:maxErrorLength]) + "..."
	}
	return value
}

func cloneFleetDiagnosticTime(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func diagnoseAgentIdentitySplit(subject agentFleetSubject, state models.StateSnapshot) []AgentFleetDiagnosticReason {
	if subject.hostname == "" || subject.removed {
		return nil
	}

	if subject.host != nil && subject.docker == nil {
		if docker, ok := findLikelyDockerPeer(subject, state.DockerHosts); ok {
			return []AgentFleetDiagnosticReason{identitySplitReason("Docker", docker.ID, docker.AgentID, docker.TokenID)}
		}
	}
	if subject.docker != nil && subject.host == nil {
		if host, ok := findLikelyHostPeer(subject, state.Hosts); ok {
			return []AgentFleetDiagnosticReason{identitySplitReason("Host", host.ID, host.ID, host.TokenID)}
		}
	}

	return nil
}

func diagnoseProfileDeployment(
	subject agentFleetSubject,
	assignment models.AgentProfileAssignment,
	profile models.AgentProfile,
	deploymentByAgentProfile map[string]models.ProfileDeploymentStatus,
) []AgentFleetDiagnosticReason {
	expectedVersion := expectedProfileVersion(profile, assignment)
	if expectedVersion <= 0 {
		expectedVersion = profile.Version
	}

	deployment, ok := deploymentByAgentProfile[deploymentKey(assignment.AgentID, assignment.ProfileID)]
	if !ok {
		return []AgentFleetDiagnosticReason{{
			Code:     "profile_deployment_missing",
			Severity: AgentFleetStatusWarning,
			Message:  "A profile is assigned, but no deployment acknowledgement exists for this agent.",
			Evidence: []string{
				fmt.Sprintf("Assigned profile: %s", firstNonEmpty(profile.Name, profile.ID)),
				fmt.Sprintf("Expected profile version: %d", expectedVersion),
			},
		}}
	}

	reasons := []AgentFleetDiagnosticReason{}
	if deployment.ProfileID != assignment.ProfileID {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "profile_deployment_mismatch",
			Severity: AgentFleetStatusWarning,
			Message:  "The agent acknowledged a different profile than the one currently assigned.",
			Evidence: []string{
				"Assigned profile ID: " + assignment.ProfileID,
				"Deployed profile ID: " + deployment.ProfileID,
			},
		})
	}
	if deployment.DeploymentStatus == "failed" {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "profile_deployment_failed",
			Severity: AgentFleetStatusCritical,
			Message:  "The assigned profile failed to deploy to this agent.",
			Evidence: nonEmptyStrings(deployment.ErrorMessage),
		})
	} else if deployment.DeploymentStatus != "" && deployment.DeploymentStatus != "deployed" {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "profile_deployment_pending",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The assigned profile is still marked %q for this agent.", deployment.DeploymentStatus),
		})
	}
	if expectedVersion > 0 && deployment.DeployedVersion > 0 && deployment.DeployedVersion < expectedVersion {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "profile_version_drift",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The agent has profile version %d, but version %d is assigned.", deployment.DeployedVersion, expectedVersion),
			Evidence: []string{
				fmt.Sprintf("Profile: %s", firstNonEmpty(profile.Name, profile.ID)),
				fmt.Sprintf("Last deployed at: %s", deployment.LastDeployedAt.UTC().Format(time.RFC3339)),
			},
		})
	}

	_ = subject
	return reasons
}

func diagnoseProfileCapabilityDrift(subject agentFleetSubject, state models.StateSnapshot, profile models.AgentProfile) []AgentFleetDiagnosticReason {
	reasons := []AgentFleetDiagnosticReason{}

	if enabled, ok := configBool(profile.Config, "enable_docker"); ok && enabled {
		if subject.docker == nil {
			if _, split := findLikelyDockerPeer(subject, state.DockerHosts); split {
				reasons = append(reasons, AgentFleetDiagnosticReason{
					Code:     "docker_profile_identity_split",
					Severity: AgentFleetStatusWarning,
					Message:  "The assigned profile enables Docker monitoring, but Docker telemetry is reporting under a separate agent identity.",
					Evidence: []string{
						"Profile key enable_docker=true",
						"Likely same host matched by hostname or token, not by agent ID",
					},
				})
			} else {
				reasons = append(reasons, AgentFleetDiagnosticReason{
					Code:     "docker_expected_missing",
					Severity: AgentFleetStatusCritical,
					Message:  "The assigned profile enables Docker monitoring, but no Docker host telemetry is reporting for this agent.",
					Evidence: []string{
						"Profile key enable_docker=true",
						"No matching Docker host record by agent ID, host ID, hostname, or token",
						"Local causes can include missing Docker socket access, installing on the wrong host, or Docker mode being disabled",
					},
				})
			}
		}
	}

	if enabled, ok := configBool(profile.Config, "enable_kubernetes"); ok && enabled && subject.kubernetes == nil && !hasType(subject.types, "kubernetes") {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "kubernetes_expected_missing",
			Severity: AgentFleetStatusWarning,
			Message:  "The assigned profile enables Kubernetes monitoring, but no Kubernetes cluster telemetry is reporting for this agent.",
			Evidence: []string{"Profile key enable_kubernetes=true"},
		})
	}

	if enabled, ok := configBool(profile.Config, "enable_proxmox"); ok && enabled && subject.host != nil && subject.host.LinkedNodeID == "" {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "proxmox_profile_unlinked",
			Severity: AgentFleetStatusWarning,
			Message:  "The assigned profile enables Proxmox mode, but this host agent is not linked to a Proxmox node.",
			Evidence: []string{"Profile key enable_proxmox=true"},
		})
	}

	if enabled, ok := configBool(profile.Config, "enable_host"); ok && !enabled && subject.host != nil {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "host_profile_not_applied",
			Severity: AgentFleetStatusWarning,
			Message:  "The assigned profile disables host monitoring, but host telemetry is still reporting.",
			Evidence: []string{"Profile key enable_host=false"},
		})
	}

	return reasons
}

func identitySplitReason(peerType, peerID, peerAgentID, peerTokenID string) AgentFleetDiagnosticReason {
	evidence := []string{"Peer type: " + peerType, "Peer ID: " + peerID}
	if peerAgentID != "" {
		evidence = append(evidence, "Peer agent ID: "+peerAgentID)
	}
	if peerTokenID != "" {
		evidence = append(evidence, "Peer shares the reporting token binding")
	}
	return AgentFleetDiagnosticReason{
		Code:     "agent_identity_split",
		Severity: AgentFleetStatusWarning,
		Message:  "Host and workload telemetry appear to belong to the same machine but are reporting as separate agent identities.",
		Evidence: evidence,
	}
}

func findLikelyDockerPeer(subject agentFleetSubject, dockerHosts []models.DockerHost) (models.DockerHost, bool) {
	for _, docker := range dockerHosts {
		if docker.Hidden || docker.ID == subject.id {
			continue
		}
		if sameAgentIdentity(subject, docker.ID, docker.AgentID, docker.Hostname, docker.TokenID) {
			return docker, true
		}
	}
	return models.DockerHost{}, false
}

func findLikelyHostPeer(subject agentFleetSubject, hosts []models.Host) (models.Host, bool) {
	for _, host := range hosts {
		if host.ID == subject.id {
			continue
		}
		if sameAgentIdentity(subject, host.ID, host.ID, host.Hostname, host.TokenID) {
			return host, true
		}
	}
	return models.Host{}, false
}

func sameAgentIdentity(subject agentFleetSubject, id, agentID, hostname, tokenID string) bool {
	if subject.agentID != "" && (subject.agentID == agentID || subject.agentID == id) {
		return true
	}
	if subject.tokenID != "" && tokenID != "" && subject.tokenID == tokenID {
		return true
	}
	if subject.hostname != "" && strings.EqualFold(subject.hostname, hostname) {
		return true
	}
	return false
}

func mapProfilesByID(profiles []models.AgentProfile) map[string]models.AgentProfile {
	result := make(map[string]models.AgentProfile, len(profiles))
	for _, profile := range profiles {
		result[profile.ID] = profile
	}
	return result
}

func mapAssignmentsByAgent(assignments []models.AgentProfileAssignment) map[string]models.AgentProfileAssignment {
	result := make(map[string]models.AgentProfileAssignment, len(assignments))
	for _, assignment := range assignments {
		result[assignment.AgentID] = assignment
	}
	return result
}

func mapDeploymentsByAgentProfile(deployments []models.ProfileDeploymentStatus) map[string]models.ProfileDeploymentStatus {
	result := make(map[string]models.ProfileDeploymentStatus, len(deployments))
	for _, deployment := range deployments {
		result[deploymentKey(deployment.AgentID, deployment.ProfileID)] = deployment
	}
	return result
}

func findAgentAssignment(subject agentFleetSubject, assignmentByAgent map[string]models.AgentProfileAssignment) (models.AgentProfileAssignment, bool) {
	for _, id := range uniqueNonEmptyStrings(subject.agentID, subject.id) {
		if assignment, ok := assignmentByAgent[id]; ok {
			return assignment, true
		}
	}
	return models.AgentProfileAssignment{}, false
}

func deploymentKey(agentID, profileID string) string {
	return strings.TrimSpace(agentID) + "\x00" + strings.TrimSpace(profileID)
}

func expectedProfileVersion(profile models.AgentProfile, assignment models.AgentProfileAssignment) int {
	if assignment.ProfileVersion > 0 {
		return assignment.ProfileVersion
	}
	return profile.Version
}

func configBool(config models.AgentConfigMap, key string) (bool, bool) {
	raw, ok := config[key]
	if !ok {
		return false, false
	}
	switch value := raw.(type) {
	case bool:
		return value, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		return parsed, err == nil
	default:
		return false, false
	}
}

func sortedAgentTypes(types map[string]struct{}) []string {
	order := []string{"host", "docker", "kubernetes"}
	out := make([]string, 0, len(types))
	for _, candidate := range order {
		if _, ok := types[candidate]; ok {
			out = append(out, candidate)
		}
	}
	return out
}

func diagnosticStatusFromReasons(reasons []AgentFleetDiagnosticReason) string {
	status := AgentFleetStatusHealthy
	for _, reason := range reasons {
		if agentFleetStatusRank(reason.Severity) > agentFleetStatusRank(status) {
			status = reason.Severity
		}
	}
	return status
}

func agentFleetStatusRank(status string) int {
	switch status {
	case AgentFleetStatusCritical:
		return 3
	case AgentFleetStatusWarning:
		return 2
	case AgentFleetStatusRemoved:
		return 1
	default:
		return 0
	}
}

func agentFleetStatusIsOnline(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "running", "healthy":
		return true
	default:
		return false
	}
}

func hasType(types map[string]struct{}, kind string) bool {
	_, ok := types[kind]
	return ok
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func newestTime(a, b time.Time) time.Time {
	if a.IsZero() || b.After(a) {
		return b
	}
	return a
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// formatFleetDuration renders a duration the way a person would say it
// ("45s", "5m", "10m 2s", "1h 3m", "2d 4h") instead of Go's "5m0s" form.
func formatFleetDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	duration = duration.Round(time.Second)
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	switch {
	case days > 0:
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, hours)
	case duration >= time.Hour:
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case duration >= time.Minute:
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
