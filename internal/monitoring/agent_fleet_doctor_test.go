package monitoring

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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

func TestAgentFleetDiagnosticsExposesCollectorRoleScopeExcess(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{
		ID:     "collector-token",
		Scopes: []string{config.ScopeAgentReport, config.ScopeAgentConfigRead, config.ScopeSettingsWrite},
		Metadata: map[string]string{
			auth.RuntimeRoleMetadataKey: auth.RuntimeRoleMonitoringCollector,
		},
	}}
	monitor.state.UpsertHost(models.Host{
		ID:           "collector-agent",
		Hostname:     "collector-node",
		Platform:     "linux",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.2.0",
		TokenID:      "collector-token",
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-collector-agent")
	requireReasonCode(t, agent, AgentFleetReasonCredentialScopeExcess)
	if agent.Status != AgentFleetStatusCritical {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusCritical)
	}
}

func TestAgentFleetDiagnosticsDetectsMissingAndExpiredCredentials(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Hour)

	tests := []struct {
		name       string
		tokenID    string
		tokens     []config.APITokenRecord
		wantReason string
	}{
		{
			name:       "missing or revoked",
			tokenID:    "revoked-token",
			wantReason: AgentFleetReasonCredentialMissing,
		},
		{
			name:    "expired",
			tokenID: "expired-token",
			tokens: []config.APITokenRecord{{
				ID:        "expired-token",
				ExpiresAt: &expiredAt,
			}},
			wantReason: AgentFleetReasonCredentialExpired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			monitor := newAgentFleetDoctorTestMonitor(t)
			monitor.config.APITokens = test.tokens
			monitor.state.UpsertHost(models.Host{
				ID:           "agent-auth",
				Hostname:     "auth-node",
				Platform:     "linux",
				Status:       "online",
				LastSeen:     now,
				AgentVersion: "6.2.0",
				TokenID:      test.tokenID,
			})

			agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-auth")
			requireReasonCode(t, agent, test.wantReason)
			if agent.Status != AgentFleetStatusCritical {
				t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusCritical)
			}
			// The verdict must name the credential it judged, or an operator
			// cannot map it onto the token list to spot a stale host row.
			foundJudgedID := false
			for _, reason := range agent.Reasons {
				if reason.Code != test.wantReason {
					continue
				}
				for _, evidence := range reason.Evidence {
					if evidence == "Credential id: "+test.tokenID {
						foundJudgedID = true
					}
				}
			}
			if !foundJudgedID {
				t.Fatalf("credential reason evidence does not name judged token id %q: %#v", test.tokenID, agent.Reasons)
			}
			if !hasSupportedRepair(agent, AgentFleetActionRepairAuthentication) {
				t.Fatalf("expected safe authentication repair action: %#v", agent.RepairActions)
			}
		})
	}
}

func TestAgentFleetDiagnosticsTreatsFreshlyUsedUnlistedCredentialAsRegistryStale(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lastUsed := now.Add(-30 * time.Second)
	monitor := newAgentFleetDoctorTestMonitor(t)
	// The judged token is absent from the inventory, but the host row shows it
	// authenticated seconds ago (#1730). That contradiction means the server's
	// token registry view is stale, so the verdict must not read as a
	// credential outage and must not offer the repair-authentication loop.
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-auth",
		Hostname:        "auth-node",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now,
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
		TokenID:         "unlisted-token",
		TokenLastUsedAt: &lastUsed,
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-auth")
	reason := requireReasonCode(t, agent, AgentFleetReasonCredentialUnlisted)
	if reason.Severity != AgentFleetStatusWarning {
		t.Fatalf("severity = %q, want %q", reason.Severity, AgentFleetStatusWarning)
	}
	if agent.Status != AgentFleetStatusWarning {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusWarning)
	}
	for _, r := range agent.Reasons {
		if r.Code == AgentFleetReasonCredentialMissing {
			t.Fatalf("fresh authentication must suppress the missing-credential outage: %#v", agent.Reasons)
		}
	}
	for _, repair := range agent.RepairActions {
		if repair.Code == AgentFleetActionRepairAuthentication {
			t.Fatalf("registry-stale verdict must not offer repair authentication: %#v", agent.RepairActions)
		}
	}
}

func TestAgentFleetDiagnosticsKeepsMissingCredentialCriticalWhenUseIsStale(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	lastUsed := now.Add(-2 * time.Hour)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-auth",
		Hostname:        "auth-node",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now,
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
		TokenID:         "revoked-token",
		TokenLastUsedAt: &lastUsed,
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-auth")
	requireReasonCode(t, agent, AgentFleetReasonCredentialMissing)
	if agent.Status != AgentFleetStatusCritical {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusCritical)
	}
}

func TestAgentFleetDiagnosticsAcceptsActiveCredential(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{ID: "active-token"}}
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-auth",
		Hostname:     "auth-node",
		Platform:     "linux",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.2.0",
		TokenID:      "active-token",
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-auth")
	for _, reason := range agent.Reasons {
		if reason.Code == AgentFleetReasonCredentialMissing || reason.Code == AgentFleetReasonCredentialExpired {
			t.Fatalf("active credential produced repair reason: %#v", agent.Reasons)
		}
	}
}

func TestAgentFleetDiagnosticsRepairsCommandCredentialWithoutExecScope(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{
		ID: "report-only-token",
		Scopes: []string{
			config.ScopeAgentReport,
			config.ScopeAgentConfigRead,
		},
	}}
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-command-scope",
		Hostname:        "pve-command-scope",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now,
		AgentVersion:    "6.2.0",
		TokenID:         "report-only-token",
		CommandsEnabled: true,
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-command-scope")
	requireReasonCode(t, agent, AgentFleetReasonExecScopeMissing)
	if agent.Status != AgentFleetStatusCritical {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusCritical)
	}
	if !hasSupportedRepair(agent, AgentFleetActionRepairAuthentication) {
		t.Fatalf("expected command-scope mismatch to expose authentication repair: %#v", agent.RepairActions)
	}
}

func TestAgentFleetDiagnosticsAcceptsCommandCredentialWithExecScope(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{
		ID:     "command-token",
		Scopes: []string{config.ScopeAgentReport, config.ScopeAgentExec},
	}}
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-command-scope",
		Hostname:        "pve-command-scope",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now,
		AgentVersion:    "6.2.0",
		TokenID:         "command-token",
		CommandsEnabled: true,
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-command-scope")
	for _, reason := range agent.Reasons {
		if reason.Code == AgentFleetReasonExecScopeMissing {
			t.Fatalf("exec-scoped credential produced repair reason: %#v", agent.Reasons)
		}
	}
}

func TestAgentFleetDiagnosticsWarnsWhenMonitoringRuntimeHasExecCredential(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{
		ID:     "over-scoped-token",
		Scopes: []string{config.ScopeAgentReport, config.ScopeAgentExec},
	}}
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-monitoring-authority",
		Hostname:     "monitoring-node",
		Platform:     "linux",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.2.0",
		TokenID:      "over-scoped-token",
		AgentPrivilege: &models.AgentPrivilegeStatus{
			RunningAsRoot:    true,
			CommandAuthority: "monitoring-only",
		},
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-monitoring-authority")
	requireReasonCode(t, agent, AgentFleetReasonExecScopeExcess)
	if agent.Status != AgentFleetStatusWarning {
		t.Fatalf("status = %q, want %q", agent.Status, AgentFleetStatusWarning)
	}
	if agent.Privilege == nil || !agent.Privilege.CredentialKnown || !agent.Privilege.CredentialExec {
		t.Fatalf("credential authority was not projected into privilege diagnostics: %+v", agent.Privilege)
	}
}

func TestAgentFleetDiagnosticsWarnsWhenCommandCapableRuntimeCannotBeReenabled(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{
		ID:     "monitoring-token",
		Scopes: []string{config.ScopeAgentReport, config.ScopeAgentConfigRead},
	}}
	monitor.state.UpsertHost(models.Host{
		ID:           "agent-command-authority",
		Hostname:     "command-node",
		Platform:     "linux",
		Status:       "online",
		LastSeen:     now,
		AgentVersion: "6.2.0",
		TokenID:      "monitoring-token",
		AgentPrivilege: &models.AgentPrivilegeStatus{
			RunningAsRoot:    true,
			CommandAuthority: "command-capable",
		},
	})

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-command-authority")
	reason := requireReasonCode(t, agent, AgentFleetReasonExecScopeMissing)
	if reason.Severity != AgentFleetStatusWarning || agent.Status != AgentFleetStatusWarning {
		t.Fatalf("reason/status = %q/%q, want warning/warning", reason.Severity, agent.Status)
	}
	if agent.Privilege == nil || !agent.Privilege.CredentialKnown || agent.Privilege.CredentialExec {
		t.Fatalf("credential authority was not projected into privilege diagnostics: %+v", agent.Privilege)
	}
}

func TestAgentFleetDiagnosticsDoesNotValidateSyntheticMockCredentials(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, false) })
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.config.APITokens = []config.APITokenRecord{{ID: "synthetic-token"}}

	if inventory := monitor.agentFleetTokenInventory(now); inventory.known {
		t.Fatalf("mock token inventory = %+v, want deliberately unknown", inventory)
	}
}

func TestAgentFleetDiagnosticsBlocksGenericRepairForDuplicateHostInstallations(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	for _, host := range []models.Host{
		{
			ID:           "agent-primary",
			Hostname:     "pi",
			MachineID:    "machine-pi",
			Platform:     "linux",
			Status:       "online",
			LastSeen:     now,
			AgentVersion: "6.2.0-rc.11",
		},
		{
			ID:           "agent-legacy",
			Hostname:     "PI.local",
			MachineID:    "machine-pi",
			Platform:     "linux",
			Status:       "online",
			LastSeen:     now.Add(-time.Minute),
			AgentVersion: "6.2.0-rc.6",
		},
	} {
		monitor.state.UpsertHost(host)
	}

	for _, rowKey := range []string{"agent-agent-primary", "agent-agent-legacy"} {
		agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0-rc.11", now), rowKey)
		requireReasonCode(t, agent, AgentFleetReasonDuplicateInstallation)
		for _, repair := range agent.RepairActions {
			if (repair.Code == AgentFleetActionCopyUpgradeCommand || repair.Code == AgentFleetActionRepairAuthentication) && repair.Supported {
				t.Fatalf("duplicate installation offered unsafe generic repair: %#v", agent.RepairActions)
			}
		}
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

func TestAgentFleetDiagnosticsSurfacesTypedPrivilegeHelperDegradation(t *testing.T) {
	now := time.Date(2026, 8, 31, 11, 30, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-safe-profile",
		Hostname:        "pve-safe",
		DisplayName:     "PVE Safe",
		Platform:        "linux",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.4.2",
		AgentModules: []models.AgentModuleStatus{{
			Name:      agentshost.ModuleNameTypedPrivilegeHelper,
			Enabled:   true,
			State:     "degraded",
			LastError: "smart.snapshot: helper unavailable token=must-not-leak",
			UpdatedAt: now.Add(-time.Minute),
		}},
	})

	diagnostics := monitor.GetAgentFleetDiagnosticsForTarget("6.4.2", "6.4.2", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-safe-profile")
	requireReasonCode(t, agent, AgentFleetReasonPrivilegeHelperDegraded)
	if agent.Status != AgentFleetStatusWarning {
		t.Fatalf("status = %q, want warning", agent.Status)
	}
	if len(agent.AgentModules) != 1 || agent.AgentModules[0].Name != agentshost.ModuleNameTypedPrivilegeHelper {
		t.Fatalf("helper module evidence = %+v", agent.AgentModules)
	}
	if strings.Contains(agent.AgentModules[0].LastError, "must-not-leak") {
		t.Fatalf("helper module error was not redacted: %q", agent.AgentModules[0].LastError)
	}
	found := false
	for _, reason := range agent.Reasons {
		if reason.Code != AgentFleetReasonPrivilegeHelperDegraded {
			continue
		}
		found = true
		if !strings.Contains(reason.Message, "omitted without widening collector privilege") {
			t.Fatalf("helper degradation message = %q", reason.Message)
		}
		for _, evidence := range reason.Evidence {
			if strings.Contains(evidence, "must-not-leak") {
				t.Fatalf("helper reason evidence was not redacted: %#v", reason.Evidence)
			}
		}
	}
	if !found {
		t.Fatalf("helper degradation reason missing from %+v", agent.Reasons)
	}
}

// A least-privilege install is an intentional hardening profile: the doctor
// must surface the reported privilege descriptively and must not degrade the
// agent's health status on that evidence alone.
func TestAgentFleetDiagnosticsSurfacesPrivilegeProfileWithoutDegradingHealth(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-least-priv",
		Hostname:        "pve-node",
		Status:          "online",
		LastSeen:        now.Add(-30 * time.Second),
		IntervalSeconds: 30,
		AgentVersion:    "6.2.0",
		AgentPrivilege: &models.AgentPrivilegeStatus{
			RunningAsRoot:    false,
			ServiceUser:      "pulse-agent",
			CommandAuthority: "monitoring-only",
			TypedHelper:      true,
			SmartctlHelper:   true,
			PctHelper:        false,
		},
	})

	diagnostics := monitor.GetAgentFleetDiagnosticsForTarget("6.2.0", "6.2.0", now)
	agent := requireAgentDiagnostic(t, diagnostics, "agent-agent-least-priv")

	if agent.Privilege == nil {
		t.Fatal("privilege profile missing from diagnostics")
	}
	if agent.Privilege.RunningAsRoot || agent.Privilege.ServiceUser != "pulse-agent" ||
		agent.Privilege.CommandAuthority != "monitoring-only" ||
		!agent.Privilege.TypedHelper ||
		!agent.Privilege.SmartctlHelper || agent.Privilege.PctHelper {
		t.Fatalf("privilege profile = %+v", agent.Privilege)
	}
	if agent.Status != AgentFleetStatusHealthy {
		t.Fatalf("least-privilege agent status = %q, want healthy; reasons = %+v", agent.Status, agent.Reasons)
	}
	for _, reason := range agent.Reasons {
		if strings.Contains(strings.ToLower(reason.Message), "privilege") {
			t.Fatalf("privilege surfaced as a health reason: %+v", reason)
		}
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

func TestDiagnoseProfileCapabilityDriftOnlyRequiresPVEProfilesToLinkToNodes(t *testing.T) {
	host := models.Host{ID: "agent-1", Hostname: "pbs01"}
	subject := agentFleetSubject{host: &host}

	for _, test := range []struct {
		name         string
		proxmoxType  interface{}
		pbsInstances []models.PBSInstance
		wantReason   bool
	}{
		{name: "PBS", proxmoxType: "pbs", wantReason: false},
		{
			name:        "auto detected PBS",
			proxmoxType: "auto",
			pbsInstances: []models.PBSInstance{{
				Name: "pbs01",
				Host: "https://192.168.1.121:8007",
			}},
			wantReason: false,
		},
		{
			name:        "missing type on known PBS host",
			proxmoxType: nil,
			pbsInstances: []models.PBSInstance{{
				Name: "pbs01",
				Host: "https://192.168.1.121:8007",
			}},
			wantReason: false,
		},
		{name: "auto without PBS match", proxmoxType: "auto", wantReason: true},
		{name: "PVE", proxmoxType: "pve", wantReason: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			profileConfig := models.AgentConfigMap{"enable_proxmox": true}
			if test.proxmoxType != nil {
				profileConfig["proxmox_type"] = test.proxmoxType
			}
			reasons := diagnoseProfileCapabilityDrift(
				subject,
				models.StateSnapshot{PBSInstances: test.pbsInstances},
				models.AgentProfile{Config: profileConfig},
			)

			found := false
			for _, reason := range reasons {
				if reason.Code == "proxmox_profile_unlinked" {
					found = true
				}
			}
			if found != test.wantReason {
				t.Fatalf("proxmox_profile_unlinked present = %v, want %v; reasons = %#v", found, test.wantReason, reasons)
			}
		})
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

func TestAgentFleetDiagnosticsDoesNotRequireLegacyProfileDeploymentAcknowledgement(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	monitor := newAgentFleetDoctorTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:              "agent-1",
		Hostname:        "profile-node",
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
		nil,
	)

	agent := requireAgentDiagnostic(t, monitor.GetAgentFleetDiagnostics("6.2.0", now), "agent-agent-1")
	if agent.Status != AgentFleetStatusHealthy {
		t.Fatalf("status = %q, want %q; reasons = %#v", agent.Status, AgentFleetStatusHealthy, agent.Reasons)
	}
	if agent.ProfileVersion != 4 || agent.DeployedProfileVersion != 0 {
		t.Fatalf("profile versions = assigned %d deployed %d, want assigned 4 with no legacy deployment version", agent.ProfileVersion, agent.DeployedProfileVersion)
	}
	for _, reason := range agent.Reasons {
		if reason.Code == "profile_deployment_missing" {
			t.Fatalf("missing legacy deployment acknowledgement must not create attention: %#v", agent.Reasons)
		}
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
