package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// setExitedContainer rebuilds the rig's state with its container exited with
// exitCode, and its alert the warning the alert subsystem raises for a
// container that is not running.
func (r *alertRunRig) setExitedContainer(exitCode int, oomKilled *bool) {
	r.t.Helper()
	r.setContainerState("exited", fmt.Sprintf("Exited (%d) 12 minutes ago", exitCode), exitCode, oomKilled)
}

func (r *alertRunRig) setContainerState(state, status string, exitCode int, oomKilled *bool) {
	r.t.Helper()
	r.alert.ID = "docker-container-state-web"
	r.alert.Type = "docker-container-state"
	r.alert.Level = "warning"
	r.alert.Message = fmt.Sprintf("Docker container 'web' is %s", status)
	snapshot := models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID: "dock-agent-1", Hostname: "docker-box", Status: "online", LastSeen: time.Now(),
			Containers: []models.DockerContainer{{
				ID: strings.Repeat("ab12", 16), Name: "web", State: state, Status: status,
				ExitCode: exitCode, OOMKilled: oomKilled, RestartPolicy: "no",
			}},
		}},
		ActiveAlerts: []models.Alert{r.alert},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	r.provider.state = snapshot
	r.ps.SetReadState(registry)
	r.ps.SetUnifiedResourceProvider(adapter)
	r.containerID = ""
	for _, resource := range adapter.GetByType(unifiedresources.ResourceTypeAppContainer) {
		r.containerID = resource.ID
	}
	if r.containerID == "" {
		r.t.Fatal("inventory has no ID for the container")
	}
}

func (r *alertRunRig) sustainedScope() PatrolScope {
	scope := SustainedAlertPatrolScope(r.alert.ID, r.alert.ResourceID, "app-container", r.alert.Type)
	scope.NoStream = true
	return scope
}

// On Docker fault set v1 D2, qwen3:8b woke by itself on a worker that had
// crashed with a segfault and stayed down. It wrote "Exited (139) [...]
// segmentation fault [...] investigate the cause" and filed nothing, and the
// alert fallback left a warning that stayed active to the model, so nothing
// was investigated. A container that exited with an error is a fault under
// the rule that check is given, so Pulse opens the finding when the model
// files none.
func TestSustainedWarningOnAFailedContainerOpensTheFinding(t *testing.T) {
	yes := true
	for _, tc := range []struct {
		name  string
		state string
		exit  int
		oom   *bool
	}{
		{name: "segfault", state: "exited", exit: 139},
		{name: "error exit", state: "exited", exit: 1},
		{name: "memory killed", state: "exited", exit: 137, oom: &yes},
		{name: "stuck restarting", state: "restarting", exit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
			status := fmt.Sprintf("Exited (%d) 12 minutes ago", tc.exit)
			if tc.state == "restarting" {
				status = fmt.Sprintf("Restarting (%d) 4 seconds ago", tc.exit)
			}
			rig.setContainerState(tc.state, status, tc.exit, tc.oom)

			run := rig.run(rig.sustainedScope())

			active := rig.active()
			if len(active) != 1 {
				t.Fatalf("active findings = %s, want one opened from the alert", listAlertRunFindings(active))
			}
			finding := active[0]
			if finding.Source != "alert" || finding.AlertIdentifier != rig.alert.ID || finding.ResourceID != rig.containerID || finding.Severity != FindingSeverityWarning {
				t.Errorf("finding = %s, want a warning from alert %q on %s", listAlertRunFindings(active), rig.alert.ID, rig.containerID)
			}
			if got := rig.investigated(); got != finding.ID {
				t.Errorf("investigated %q, want the finding opened from the alert %q", got, finding.ID)
			}
			if run.NewFindings != 1 || len(run.FindingIDs) != 1 || run.FindingIDs[0] != finding.ID {
				t.Errorf("run record: %d new, findings %v, want the opened finding", run.NewFindings, run.FindingIDs)
			}

			// Waking on the same alert again does not open a second one.
			rig.run(rig.sustainedScope())
			if again := rig.active(); len(again) != 1 || again[0].ID != finding.ID {
				t.Errorf("second sustained run: active findings = %s, want the same one", listAlertRunFindings(again))
			}
		})
	}
}

// A container somebody stopped, or one Pulse cannot tell failed, keeps the
// model's judgment. So does every mode that does not investigate, and a run
// where the model filed its own finding.
func TestSustainedWarningKeepsTheModelsJudgmentWherePulseSeesNoFault(t *testing.T) {
	for _, tc := range []struct {
		name string
		exit int
	}{
		{name: "docker stop (SIGTERM)", exit: 143},
		{name: "docker stop after the grace period (SIGKILL)", exit: 137},
		{name: "interrupted (SIGINT)", exit: 130},
		{name: "clean exit", exit: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
			rig.setExitedContainer(tc.exit, nil)
			rig.run(rig.sustainedScope())
			if active := rig.active(); len(active) != 0 {
				t.Fatalf("active findings = %s, want none", listAlertRunFindings(active))
			}
			rig.notInvestigated(tc.name)
		})
	}

	t.Run("Watch only", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyMonitor)
		rig.setExitedContainer(139, nil)
		rig.run(rig.sustainedScope())
		if active := rig.active(); len(active) != 0 {
			t.Fatalf("Watch only: active findings = %s, want none", listAlertRunFindings(active))
		}
		rig.notInvestigated("Watch only")
	})

	t.Run("the model filed its own finding", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		rig.setExitedContainer(139, nil)
		rig.model = func(creator tools.PatrolFindingCreator) (*PatrolStreamResponse, error) {
			if _, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key: "segfault", Severity: "warning", Category: "reliability",
				ResourceID: rig.containerID, ResourceName: "web", ResourceType: "app-container",
				Title: "web crashed with a segfault and stays down", Description: "Exit 139, no restart policy.", Evidence: "Exited (139)",
			}); err != nil {
				return nil, err
			}
			return &PatrolStreamResponse{Content: alertRunProseOnly}, nil
		}
		rig.run(rig.sustainedScope())
		active := rig.active()
		if len(active) != 1 || active[0].Source != "ai-analysis" {
			t.Fatalf("active findings = %s, want only the model's finding", listAlertRunFindings(active))
		}
	})
}

// On Docker fault set v1 D6, qwen3:8b woke by itself on a full mount of a host
// whose agent has commands on, and filed nothing. A disk alert on a host or
// guest Pulse can read is a finding under the rule that check is given: its
// investigation reads what fills the disk. Where Pulse cannot read, the
// finding would only repeat the alert, so the model keeps its judgment.
func TestSustainedDiskWarningOpensTheFindingOnlyWherePulseCanRead(t *testing.T) {
	for _, tc := range []struct {
		name    string
		catalog func(hostID string) aicontracts.ActionCapabilityCatalog
		want    bool
	}{
		{name: "commands on", want: true, catalog: func(hostID string) aicontracts.ActionCapabilityCatalog {
			return aicontracts.ActionCapabilityCatalog{Reads: &aicontracts.CommandReadHost{HostResourceID: hostID, HostName: "docker01"}}
		}},
		{name: "commands off", want: false, catalog: func(string) aicontracts.ActionCapabilityCatalog {
			return aicontracts.ActionCapabilityCatalog{CommandsDisabled: &aicontracts.CommandsDisabledHost{HostName: "docker01"}}
		}},
		{name: "reads only on another host", want: false, catalog: func(string) aicontracts.ActionCapabilityCatalog {
			return aicontracts.ActionCapabilityCatalog{Reads: &aicontracts.CommandReadHost{HostResourceID: "agent-other", HostName: "other"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps, scope, state, alert := hostDiskSustainedCheck(t, tc.catalog)
			if got := ps.sustainedAlertCallsForFinding(&scope, state, &alert); got != tc.want {
				t.Errorf("sustainedAlertCallsForFinding = %v, want %v", got, tc.want)
			}
		})
	}
}

// hostDiskSustainedCheck is hostDiskSeedAndDecision's D6 shape, returned as
// the resolved sustained scope and the state that check runs on.
func hostDiskSustainedCheck(t *testing.T, catalog func(hostID string) aicontracts.ActionCapabilityCatalog) (*PatrolService, PatrolScope, patrolRuntimeState, models.Alert) {
	t.Helper()
	now := time.Now().UTC()
	host := models.Host{
		ID: "66cd990e-host", Hostname: "docker01", MachineID: "machine-docker01", Status: "online", LastSeen: now,
		CommandsEnabled: true,
		Disks:           []models.Disk{{Mountpoint: "/", Usage: 47}, {Mountpoint: "/mnt/nvr", Device: "/dev/loop2", Usage: 92.3}},
	}
	alert := models.Alert{
		ID: "agent:66cd990e-host/disk:mnt-nvr::metric-threshold:disk", Type: "disk", Level: "warning",
		ResourceID: "agent:66cd990e-host/disk:mnt-nvr", ResourceName: "docker01 (/mnt/nvr)",
		Message: "Disk at 92.3%", Value: 92.3, Threshold: 90, StartTime: now.Add(-11 * time.Minute),
		Metadata: map[string]interface{}{"mountpoint": "/mnt/nvr", "device": "/dev/loop2"},
	}
	snapshot := models.StateSnapshot{Hosts: []models.Host{host}, ActiveAlerts: []models.Alert{alert}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	_, hostID, found := registry.GetByReference(host.ID)
	if !found {
		t.Fatal("fixture host missing from inventory")
	}

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: config.PatrolAutonomyApproval}
	svc.provider = &mockProvider{}
	svc.SetChatService(&patrolMockChatService{executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{})})
	ps := NewPatrolService(svc, &patrolTestStateProvider{state: snapshot})
	ps.SetConfig(PatrolConfig{Enabled: true, Interval: time.Hour, AnalyzeNodes: true})
	ps.SetReadState(registry)
	ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(registry))
	ps.SetActionCapabilityLookup(func(_ context.Context, resourceID string) (aicontracts.ActionCapabilityCatalog, error) {
		if resourceID != hostID {
			return aicontracts.ActionCapabilityCatalog{}, nil
		}
		return catalog(hostID), nil
	})

	state := ps.currentPatrolRuntimeState()
	resolved, resolution := resolvePatrolScopeState(state, SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "agent", alert.Type))
	if len(resolution.EffectiveResourceIDs) == 0 {
		t.Fatalf("scope did not resolve: %+v", resolution)
	}
	return ps, resolved, state, alert
}
