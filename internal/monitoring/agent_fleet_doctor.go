package monitoring

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

const (
	AgentFleetStatusHealthy  = "healthy"
	AgentFleetStatusWarning  = "warning"
	AgentFleetStatusCritical = "critical"
	AgentFleetStatusRemoved  = "removed"
)

type AgentFleetDiagnostics struct {
	GeneratedAt   int64                       `json:"generatedAt"`
	ServerVersion string                      `json:"serverVersion,omitempty"`
	Summary       AgentFleetDiagnosticSummary `json:"summary"`
	Agents        []AgentFleetAgentDiagnostic `json:"agents"`
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
	Name                   string                       `json:"name"`
	Hostname               string                       `json:"hostname,omitempty"`
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
	Reasons                []AgentFleetDiagnosticReason `json:"reasons"`
	RepairActions          []AgentFleetDiagnosticRepair `json:"repairActions,omitempty"`
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

// GetAgentFleetDiagnostics returns a read-only fleet health view derived from
// reported agent state, server version, and existing profile deployment state.
func (m *Monitor) GetAgentFleetDiagnostics(serverVersion string, now time.Time) AgentFleetDiagnostics {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	out := AgentFleetDiagnostics{
		GeneratedAt:   now.UnixMilli(),
		ServerVersion: strings.TrimSpace(serverVersion),
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
		diagnostic := diagnoseAgentFleetSubject(subjects[i], state, out.ServerVersion, now, profileByID, assignmentByAgent, deploymentByAgentProfile)
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

	for i := range state.RemovedHosts {
		removed := state.RemovedHosts[i]
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
	result := AgentFleetAgentDiagnostic{
		RowKey:          subject.rowKey,
		ID:              subject.id,
		AgentID:         subject.agentID,
		Name:            firstNonEmpty(subject.name, subject.hostname, subject.id),
		Hostname:        subject.hostname,
		Types:           sortedAgentTypes(subject.types),
		RawStatus:       subject.rawStatus,
		IntervalSeconds: subject.intervalSeconds,
		Version:         subject.version,
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
			Code:        "allow_reenroll",
			Label:       "Allow re-enroll",
			Description: "Uses the existing allow re-enroll action for removed agents.",
			Supported:   true,
			Scope:       "settings:write",
		})
		return result
	}

	result.Reasons = append(result.Reasons, diagnoseAgentConnectivity(subject, now)...)
	result.Reasons = append(result.Reasons, diagnoseAgentVersion(subject, serverVersion)...)
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
			result.RepairActions = append(result.RepairActions, AgentFleetDiagnosticRepair{
				Code:        "copy_upgrade_command",
				Label:       "Copy upgrade command",
				Description: "Uses the existing installer command from Settings -> Agents; no remote command is queued.",
				Supported:   true,
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
	staleAfter := time.Duration(interval*5) * time.Second
	if staleAfter < 5*time.Minute {
		staleAfter = 5 * time.Minute
	}

	if subject.lastSeen.IsZero() {
		return append(reasons, AgentFleetDiagnosticReason{
			Code:     "agent_never_reported",
			Severity: AgentFleetStatusCritical,
			Message:  "This agent has no last-seen timestamp, so Pulse cannot confirm it is reporting.",
		})
	}

	age := now.Sub(subject.lastSeen)
	if age > staleAfter {
		reasons = append(reasons, AgentFleetDiagnosticReason{
			Code:     "agent_disconnected",
			Severity: AgentFleetStatusCritical,
			Message:  fmt.Sprintf("No report has arrived for %s; this is beyond the %s stale threshold for a %ds reporting interval.", roundDuration(age), roundDuration(staleAfter), interval),
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
			Message:  fmt.Sprintf("The agent reports status %q instead of online/running/healthy.", subject.rawStatus),
		})
	}

	return reasons
}

func diagnoseAgentVersion(subject agentFleetSubject, serverVersion string) []AgentFleetDiagnosticReason {
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

	serverVersion = strings.TrimSpace(serverVersion)
	if serverVersion == "" || strings.EqualFold(serverVersion, "dev") {
		return nil
	}

	serverParsed, err := updates.ParseVersion(serverVersion)
	if err != nil {
		return nil
	}
	agentParsed, err := updates.ParseVersion(agentVersion)
	if err != nil {
		return []AgentFleetDiagnosticReason{{
			Code:     "agent_version_unparseable",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("The agent reported version %q, which cannot be compared with server version %q.", agentVersion, serverVersion),
		}}
	}

	if serverParsed.IsNewerThan(agentParsed) {
		return []AgentFleetDiagnosticReason{{
			Code:     "agent_version_stale",
			Severity: AgentFleetStatusWarning,
			Message:  fmt.Sprintf("Agent version %s is older than the Pulse server version %s.", agentVersion, serverVersion),
			Evidence: []string{
				"Agent version: " + agentVersion,
				"Server version: " + serverVersion,
			},
		}}
	}

	return nil
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
		evidence = append(evidence, "Peer token ID: "+peerTokenID)
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

func roundDuration(duration time.Duration) string {
	if duration < time.Second {
		return duration.String()
	}
	return duration.Round(time.Second).String()
}
