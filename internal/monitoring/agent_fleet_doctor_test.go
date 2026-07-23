package monitoring

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
)

func TestAgentFleetDiagnosticsDetectsStaleAgentVersion(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "pve-1",
		DisplayName:     "PVE 1",
		Platform:        "linux",
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

func TestAgentFleetDiagnosticsSurfacesReportedUpdateModuleAndIdentityEvidence(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	checkedAt := now.Add(-time.Minute)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-identity",
		Hostname:        "windows-node",
		DisplayName:     "Windows Node",
		Platform:        "Microsoft Windows 11 Pro",
		OSName:          "Windows 11",
		OSVersion:       "24H2",
		KernelVersion:   "10.0.26100",
		Architecture:    "amd64",
		MachineID:       "raw-machine-id-must-not-leak",
		ReportIP:        "192.0.2.10",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
		NetworkInterfaces: []models.HostNetworkInterface{{
			Name:      "Ethernet",
			Addresses: []string{"192.0.2.10/24", "not-an-ip"},
		}},
		AgentUpdate: &models.AgentUpdateStatus{
			State:         "error",
			AutoUpdate:    true,
			LastCheckedAt: &checkedAt,
			LastError:     "download failed token=must-not-leak",
		},
		AgentModules: []models.AgentModuleStatus{{
			Name:      "docker",
			Enabled:   true,
			State:     "failed",
			LastError: "socket unavailable password=must-not-leak",
			UpdatedAt: checkedAt,
		}},
	})
	before := monitor.GetState()

	diagnostics := monitor.GetAgentFleetDiagnosticsForTarget("6.2.0-pro", "6.2.0", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-identity")
	requireReasonCode(t, agent, AgentFleetReasonUpdateFailed)
	requireReasonCode(t, agent, AgentFleetReasonModuleFailed)

	if diagnostics.ServerVersion != "6.2.0-pro" || diagnostics.AgentUpdateTargetVersion != "6.2.0" {
		t.Fatalf("version identities = server %q target %q", diagnostics.ServerVersion, diagnostics.AgentUpdateTargetVersion)
	}
	if agent.ConnectionID != "agent:agent-identity" || agent.Platform != "windows" || agent.Architecture != "amd64" {
		t.Fatalf("canonical identity = %+v", agent)
	}
	if agent.MachineIDFingerprint == "" || strings.Contains(agent.MachineIDFingerprint, "raw-machine-id") {
		t.Fatalf("unsafe machine identity fingerprint %q", agent.MachineIDFingerprint)
	}
	if agent.ReportIP != "192.0.2.10" || !reflect.DeepEqual(agent.InterfaceAddresses, []string{"192.0.2.10/24"}) {
		t.Fatalf("safe IP evidence = report %q interfaces %#v", agent.ReportIP, agent.InterfaceAddresses)
	}
	if agent.AgentUpdate == nil || agent.AgentUpdate.LastCheckedAt == nil || strings.Contains(agent.AgentUpdate.LastError, "must-not-leak") {
		t.Fatalf("update evidence was missing or unsafe: %+v", agent.AgentUpdate)
	}
	if len(agent.AgentModules) != 1 || strings.Contains(agent.AgentModules[0].LastError, "must-not-leak") {
		t.Fatalf("module evidence was missing or unsafe: %+v", agent.AgentModules)
	}
	if after := monitor.GetState(); !reflect.DeepEqual(before, after) {
		t.Fatal("fleet diagnostics mutated monitor state")
	}
}

func TestAgentFleetDiagnosticsUpdaterReasonCodes(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		version      string
		updaterState string
		wantReason   string
	}{
		{name: "disabled while behind", version: "6.1.0", updaterState: "disabled", wantReason: AgentFleetReasonUpdateDisabled},
		{name: "failed", version: "6.2.0", updaterState: "error", wantReason: AgentFleetReasonUpdateFailed},
		{name: "unknown state", version: "6.2.0", updaterState: "paused-by-policy", wantReason: AgentFleetReasonUpdateStateUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			monitor := newAgentFleetDoctorTestMonitor(t)
			monitor.state.UpsertHost(models.Host{
				ID:           "agent-1",
				Hostname:     "node-1",
				Platform:     "linux",
				Status:       "online",
				LastSeen:     now,
				AgentVersion: test.version,
				AgentUpdate:  &models.AgentUpdateStatus{State: test.updaterState},
			})

			agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-1")
			requireReasonCode(t, agent, test.wantReason)
		})
	}
}

func TestAgentFleetDiagnosticsDoesNotOfferUpgradeCommandForUnknownPlatform(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-unknown-platform",
		Hostname:     "unknown-node",
		Platform:     "plan9",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.1.0",
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-unknown-platform")
	requireReasonCode(t, agent, AgentFleetReasonUpdatePlatformUnknown)
	repair := requireRepairCode(t, agent, AgentFleetActionCopyUpgradeCommand)
	if repair.Supported || repair.Platform != "" || repair.Mode != AgentFleetRepairModeHandoff {
		t.Fatalf("unsafe platform repair = %+v", repair)
	}
}

func TestAgentFleetDiagnosticsOffersUpgradeCommandForLegacyLinuxDistro(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-mageia",
		Hostname:     "mageia-node",
		Platform:     "mageia",
		OSName:       "Mageia",
		Architecture: "x86_64",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.1.0-rc.4",
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.1.1", now), "agent-agent-mageia")
	repair := requireRepairCode(t, agent, AgentFleetActionCopyUpgradeCommand)
	if !repair.Supported || repair.Platform != platformsupport.RuntimePlatformLinux {
		t.Fatalf("Mageia upgrade repair = %+v, want supported Linux repair", repair)
	}
	if agent.Platform != platformsupport.RuntimePlatformLinux {
		t.Fatalf("Mageia diagnostic platform = %q, want linux", agent.Platform)
	}
}

func TestAgentFleetDiagnosticsDoesNotOfferUpgradeCommandForUnverifiedFreeBSDState(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-pfsense",
		Hostname:     "firewall",
		Platform:     "pfSense",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.1.0",
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-pfsense")
	requireReasonCode(t, agent, AgentFleetReasonUpdateStateUnverified)
	repair := requireRepairCode(t, agent, AgentFleetActionCopyUpgradeCommand)
	if repair.Supported || repair.Platform != platformsupport.RuntimePlatformFreeBSD {
		t.Fatalf("unverified FreeBSD repair = %+v", repair)
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

func TestAgentFleetDiagnosticsRetainPlatformForRemovedAgents(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.removedDockerHosts = make(map[string]time.Time)
	monitor.dockerTokenBindings = make(map[string]string)
	monitor.dockerCommands = make(map[string]*dockerHostCommand)
	monitor.dockerCommandIndex = make(map[string]string)

	monitor.state.UpsertHost(models.Host{
		ID:       "removed-windows",
		Hostname: "win-node",
		Platform: "Microsoft Windows Server 2022",
		Status:   "online",
		LastSeen: now.Add(-time.Minute),
	})
	monitor.state.UpsertHost(models.Host{
		ID:       "removed-unknown",
		Hostname: "mystery-node",
		Platform: "BeOS",
		Status:   "online",
		LastSeen: now.Add(-time.Minute),
	})
	monitor.state.UpsertDockerHost(models.DockerHost{
		ID:       "removed-docker",
		Hostname: "docker-node",
		OS:       "Ubuntu 22.04",
		Status:   "online",
		LastSeen: now.Add(-time.Minute),
	})

	for _, hostID := range []string{"removed-windows", "removed-unknown"} {
		if _, err := monitor.RemoveHostAgent(hostID); err != nil {
			t.Fatalf("RemoveHostAgent(%s): %v", hostID, err)
		}
	}
	if _, err := monitor.RemoveDockerHost("removed-docker"); err != nil {
		t.Fatalf("RemoveDockerHost: %v", err)
	}
	monitor.state.AddRemovedKubernetesCluster(models.RemovedKubernetesCluster{
		ID:        "removed-k8s",
		Name:      "cluster",
		RemovedAt: now,
	})

	diagnostics := monitor.GetAgentFleetDiagnostics("6.2.0", now)
	tests := []struct {
		rowKey       string
		wantPlatform string
	}{
		{rowKey: "removed-host-removed-windows", wantPlatform: platformsupport.RuntimePlatformWindows},
		{rowKey: "removed-host-removed-unknown", wantPlatform: ""},
		{rowKey: "removed-docker-removed-docker", wantPlatform: platformsupport.RuntimePlatformLinux},
		{rowKey: "removed-k8s-removed-k8s", wantPlatform: ""},
	}
	for _, test := range tests {
		agent := requireAgentDiagnostic(t, diagnostics, test.rowKey)
		if agent.Status != AgentFleetStatusRemoved {
			t.Fatalf("%s status = %q, want %q", test.rowKey, agent.Status, AgentFleetStatusRemoved)
		}
		if agent.Platform != test.wantPlatform {
			t.Fatalf("%s platform = %q, want %q", test.rowKey, agent.Platform, test.wantPlatform)
		}
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

func requireRepairCode(t *testing.T, agent AgentFleetAgentDiagnostic, code string) AgentFleetDiagnosticRepair {
	t.Helper()
	for _, repair := range agent.RepairActions {
		if repair.Code == code {
			return repair
		}
	}
	t.Fatalf("repair %q not found in %#v", code, agent.RepairActions)
	return AgentFleetDiagnosticRepair{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
