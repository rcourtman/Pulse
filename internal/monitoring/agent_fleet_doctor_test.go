package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAgentFleetDiagnosticsDetectsStaleAgentVersion(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "pve-1",
		DisplayName:     "PVE 1",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.1.0",
	})

	diagnostics := monitor.GetAgentFleetDiagnostics("6.2.0", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-1")

	requireReasonCode(t, agent, "agent_version_stale")
	if agent.Status != AgentFleetStatusWarning {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusWarning)
	}
	if !hasSupportedRepair(agent, "copy_upgrade_command") {
		t.Fatalf("expected stale version diagnostic to expose the supported upgrade command action: %#v", agent.RepairActions)
	}
}

func TestAgentFleetDiagnosticsDetectsMissingDockerTelemetryFromProfile(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "docker-node",
		DisplayName:     "Docker Node",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
	})
	saveAgentFleetProfileState(t, monitor.persistence,
		[]models.AgentProfile{{
			ID:      "profile-docker",
			Name:    "Docker profile",
			Version: 2,
			Config: models.AgentConfigMap{
				"enable_docker": true,
			},
		}},
		[]models.AgentProfileAssignment{{
			AgentID:        "agent-1",
			ProfileID:      "profile-docker",
			ProfileVersion: 2,
			UpdatedAt:      now,
		}},
		[]models.ProfileDeploymentStatus{{
			AgentID:          "agent-1",
			ProfileID:        "profile-docker",
			AssignedVersion:  2,
			DeployedVersion:  2,
			DeploymentStatus: "deployed",
			LastDeployedAt:   now,
		}},
	)

	diagnostics := monitor.GetAgentFleetDiagnostics("6.2.0", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-1")
	reason := requireReasonCode(t, agent, "docker_expected_missing")

	if agent.Status != AgentFleetStatusCritical {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusCritical)
	}
	if !containsString(reason.Evidence, "Local causes can include missing Docker socket access, installing on the wrong host, or Docker mode being disabled") {
		t.Fatalf("missing Docker evidence should name unsupported local causes, got %#v", reason.Evidence)
	}
}

func TestAgentFleetDiagnosticsDetectsProfileVersionDrift(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "profile-node",
		DisplayName:     "Profile Node",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
	})
	saveAgentFleetProfileState(t, monitor.persistence,
		[]models.AgentProfile{{
			ID:      "profile-current",
			Name:    "Current profile",
			Version: 4,
			Config:  models.AgentConfigMap{},
		}},
		[]models.AgentProfileAssignment{{
			AgentID:        "agent-1",
			ProfileID:      "profile-current",
			ProfileVersion: 4,
			UpdatedAt:      now,
		}},
		[]models.ProfileDeploymentStatus{{
			AgentID:          "agent-1",
			ProfileID:        "profile-current",
			AssignedVersion:  4,
			DeployedVersion:  2,
			DeploymentStatus: "deployed",
			LastDeployedAt:   now.Add(-10 * time.Minute),
		}},
	)

	diagnostics := monitor.GetAgentFleetDiagnostics("6.2.0", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-1")
	reason := requireReasonCode(t, agent, "profile_version_drift")

	if agent.ProfileVersion != 4 || agent.DeployedProfileVersion != 2 {
		t.Fatalf("profile versions = assigned %d deployed %d, want assigned 4 deployed 2", agent.ProfileVersion, agent.DeployedProfileVersion)
	}
	if reason.Message != "The agent has profile version 2, but version 4 is assigned." {
		t.Fatalf("reason message = %q", reason.Message)
	}
}

func newAgentFleetDoctorTestMonitor(t *testing.T) *Monitor {
	t.Helper()
	return &Monitor{
		state:       models.NewState(),
		persistence: config.NewConfigPersistence(t.TempDir()),
		config:      &config.Config{},
	}
}

func saveAgentFleetProfileState(
	t *testing.T,
	persistence *config.ConfigPersistence,
	profiles []models.AgentProfile,
	assignments []models.AgentProfileAssignment,
	deployments []models.ProfileDeploymentStatus,
) {
	t.Helper()
	if err := persistence.SaveAgentProfiles(profiles); err != nil {
		t.Fatalf("SaveAgentProfiles: %v", err)
	}
	if err := persistence.SaveAgentProfileAssignments(assignments); err != nil {
		t.Fatalf("SaveAgentProfileAssignments: %v", err)
	}
	if err := persistence.SaveProfileDeploymentStatus(deployments); err != nil {
		t.Fatalf("SaveProfileDeploymentStatus: %v", err)
	}
}

func requireAgentDiagnostic(t *testing.T, diagnostics AgentFleetDiagnostics, rowKey string) AgentFleetAgentDiagnostic {
	t.Helper()
	for _, agent := range diagnostics.Agents {
		if agent.RowKey == rowKey {
			return agent
		}
	}
	t.Fatalf("diagnostic row %q not found in %#v", rowKey, diagnostics.Agents)
	return AgentFleetAgentDiagnostic{}
}

func requireReasonCode(t *testing.T, agent AgentFleetAgentDiagnostic, code string) AgentFleetDiagnosticReason {
	t.Helper()
	for _, reason := range agent.Reasons {
		if reason.Code == code {
			return reason
		}
	}
	t.Fatalf("reason %q not found in %#v", code, agent.Reasons)
	return AgentFleetDiagnosticReason{}
}

func hasSupportedRepair(agent AgentFleetAgentDiagnostic, code string) bool {
	for _, repair := range agent.RepairActions {
		if repair.Code == code && repair.Supported {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
