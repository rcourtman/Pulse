package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// deadGuestRig runs Patrol over a Proxmox container and a VM collected the
// way the monitor collects them, with the model's turn replaced by prose that
// calls no tool.
type deadGuestRig struct {
	t        *testing.T
	ps       *PatrolService
	provider *patrolTestStateProvider
	orch     *integrationOrchestrator
	prompts  []string
	ct       models.Container
	vm       models.VM
	ctID     string
	vmID     string
	observed time.Time
}

func newDeadGuestRig(t *testing.T, mode string) *deadGuestRig {
	t.Helper()
	rig := &deadGuestRig{t: t, provider: &patrolTestStateProvider{}, orch: newIntegrationOrchestrator(true)}

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: mode}
	svc.provider = &mockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc.SetChatService(&patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			rig.prompts = append(rig.prompts, req.Prompt)
			return &PatrolStreamResponse{Content: "The guest is powered off. No Pulse-offered remediation action was found."}, nil
		},
	})

	rig.ps = NewPatrolService(svc, rig.provider)
	rig.ps.SetConfig(PatrolConfig{Enabled: true, Interval: time.Hour, AnalyzeGuests: true, AnalyzeNodes: true})
	rig.ps.SetInvestigationOrchestrator(rig.orch)

	yes := true
	rig.ct = models.Container{ID: "homelab:delly:101", VMID: 101, Name: "gitea", Node: "delly", Instance: "homelab", Type: "lxc", OnBoot: &yes}
	rig.vm = models.VM{ID: "homelab:delly:160", VMID: 160, Name: "builder", Node: "delly", Instance: "homelab", Type: "qemu"}
	return rig
}

// collect publishes one collection of both guests, observed now, so the
// inventory calls the source fresh however slowly the test runs.
func (r *deadGuestRig) collect(ctStatus, vmStatus string, activeAlerts ...models.Alert) {
	r.t.Helper()
	observed := time.Now().UTC()
	if !observed.After(r.observed) {
		observed = r.observed.Add(time.Millisecond)
	}
	r.observed = observed
	r.ct.Status, r.ct.LastSeen = ctStatus, r.observed
	r.vm.Status, r.vm.LastSeen = vmStatus, r.observed
	snapshot := models.StateSnapshot{
		Nodes:        []models.Node{{ID: "homelab-delly", Name: "delly", Instance: "homelab", Status: "online", LastSeen: r.observed}},
		Containers:   []models.Container{r.ct},
		VMs:          []models.VM{r.vm},
		ActiveAlerts: activeAlerts,
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	r.provider.state = snapshot
	r.ps.SetReadState(registry)
	r.ps.SetUnifiedResourceProvider(adapter)
	for _, resource := range adapter.GetByType(unifiedresources.ResourceTypeSystemContainer) {
		r.ctID = resource.ID
	}
	for _, resource := range adapter.GetByType(unifiedresources.ResourceTypeVM) {
		r.vmID = resource.ID
	}
	if r.ctID == "" || r.vmID == "" {
		r.t.Fatalf("inventory lacks a guest: container %q, vm %q", r.ctID, r.vmID)
	}
}

func (r *deadGuestRig) stoppedFinding(resourceID string) *Finding {
	return r.ps.GetFindings().Get(proxmoxGuestStoppedFindingIDPrefix + resourceID)
}

func (r *deadGuestRig) investigated() string {
	select {
	case id := <-r.orch.investigateCh:
		return id
	case <-time.After(2 * time.Second):
		return ""
	}
}

// poweredOffAlertFromManager has the real alert manager raise the powered-off
// alert for a stopped guest, and returns it as the monitor publishes it.
func poweredOffAlertFromManager(t *testing.T, guest any, instance string) models.Alert {
	t.Helper()
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.TimeThresholds = map[string]int{"guest": 0, "node": 0, "storage": 0, "pbs": 0, "agent": 0}
	manager.UpdateConfig(cfg)

	deadline := time.Now().Add(20 * time.Second)
	for {
		manager.CheckGuest(guest, instance)
		for _, active := range manager.GetActiveAlerts() {
			if active.Type == "powered-off" {
				return models.Alert{
					ID: active.ID, Type: active.Type, Level: string(active.Level),
					ResourceID: active.ResourceID, ResourceName: active.ResourceName,
					Node: active.Node, Instance: active.Instance, Message: active.Message,
					Value: active.Value, Threshold: active.Threshold, StartTime: active.StartTime,
					Metadata: active.Metadata,
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the alert manager raised no powered-off alert, active: %+v", manager.GetActiveAlerts())
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// A guest that dies between two full runs is looked at only by the scoped runs
// its alert or a person starts. Those must run the built-in stopped-guest
// check too. On a Proxmox lab a killed container got a sustained alert run and
// a manual run, and neither said it had stopped.
func TestScopedPatrolRunsTheStoppedGuestCheck(t *testing.T) {
	rig := newDeadGuestRig(t, config.PatrolAutonomyApproval)

	// A full run sees both guests running.
	rig.collect("running", "running")
	rig.ps.runPatrol(context.Background())
	if finding := rig.stoppedFinding(rig.ctID); finding != nil {
		t.Fatalf("a running guest has a stopped finding: %#v", finding)
	}

	// Both die. The container's powered-off alert stays active and starts a
	// sustained run. No full run happens.
	rig.ct.Status = "stopped"
	alert := poweredOffAlertFromManager(t, rig.ct, "homelab")
	t.Logf("alert %s on resource %s", alert.ID, alert.ResourceID)
	rig.collect("stopped", "stopped", alert)
	rig.prompts = nil
	scope := SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "system-container", alert.Type)
	scope.NoStream = true
	rig.ps.runScopedPatrol(context.Background(), scope)

	ctFinding := rig.stoppedFinding(rig.ctID)
	if ctFinding == nil || !ctFinding.IsActive() || ctFinding.Source != proxmoxGuestLifecycleFindingSource {
		t.Fatalf("after the alert run the dead container has no built-in stopped finding: %#v", ctFinding)
	}
	record := rig.ps.runHistoryStore.GetRecent(1)[0]
	if len(record.FindingIDs) != 1 || record.FindingIDs[0] != ctFinding.ID {
		t.Errorf("the run record lists findings %v, want only the container's %s", record.FindingIDs, ctFinding.ID)
	}

	// The check reads the whole inventory, so the VM nobody asked about gets
	// its finding now and not at the next full run. It is not on this run's
	// record. Both findings are investigated, as after a full run.
	vmFinding := rig.stoppedFinding(rig.vmID)
	if vmFinding == nil || !vmFinding.IsActive() {
		t.Fatalf("the dead VM outside the run's scope has no stopped finding: %#v", vmFinding)
	}
	investigated := map[string]bool{rig.investigated(): true, rig.investigated(): true}
	if !investigated[ctFinding.ID] || !investigated[vmFinding.ID] {
		t.Errorf("investigated %v, want both stopped findings", investigated)
	}

	// The model is told what it could not see, and not to file the stop twice.
	if len(rig.prompts) != 1 {
		t.Fatalf("the model was asked %d times, want once", len(rig.prompts))
	}
	for _, want := range []string{
		`Pulse declares these actions for "gitea": start.`,
		"so do not promise that it will work",
		"Start on boot is on for this guest, so it is meant to be running.",
		"Pulse has already reported that this guest stopped. Do not report the stop again.",
	} {
		if !strings.Contains(rig.prompts[0], want) {
			t.Errorf("the alert run's prompt lacks %q", want)
		}
	}
	if strings.Contains(rig.prompts[0], "Pulse offers an action") {
		t.Error("the prompt still asks about actions Pulse offers, which the scan cannot see")
	}

	// A scoped run on the VM alone must not forget or resolve the container's finding.
	rig.collect("stopped", "stopped", alert)
	rig.ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{rig.vmID}, Reason: TriggerReasonManual, NoStream: true})
	if finding := rig.stoppedFinding(rig.ctID); finding == nil || !finding.IsActive() {
		t.Errorf("a scoped run on the VM closed the container's finding: %#v", finding)
	}

	// The container is started again. The next scoped run closes its finding.
	rig.collect("running", "stopped")
	rig.ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{rig.vmID}, Reason: TriggerReasonManual, NoStream: true})
	if finding := rig.stoppedFinding(rig.ctID); finding == nil || finding.IsActive() {
		t.Errorf("the container runs again and its stopped finding is still open: %#v", finding)
	}
	if finding := rig.stoppedFinding(rig.vmID); finding == nil || !finding.IsActive() {
		t.Errorf("the VM is still stopped and its finding was closed: %#v", finding)
	}
}

// A manual run on a dead guest raises the stopped finding too, in Watch only
// as well, where nothing is investigated.
func TestManualRunOnADeadGuestRaisesTheStoppedFinding(t *testing.T) {
	rig := newDeadGuestRig(t, config.PatrolAutonomyMonitor)
	rig.collect("running", "running")
	rig.ps.runPatrol(context.Background())

	rig.collect("stopped", "running")
	rig.ps.runScopedPatrol(context.Background(), PatrolScope{ResourceIDs: []string{rig.ctID}, Reason: TriggerReasonManual, NoStream: true})

	finding := rig.stoppedFinding(rig.ctID)
	if finding == nil || !finding.IsActive() {
		t.Fatalf("the manual run on the dead container raised no stopped finding: %#v", finding)
	}
	record := rig.ps.runHistoryStore.GetRecent(1)[0]
	if len(record.FindingIDs) != 1 || record.FindingIDs[0] != finding.ID || record.NewFindings != 1 {
		t.Errorf("run record: findings %v, new %d, want the stopped finding counted once", record.FindingIDs, record.NewFindings)
	}
	select {
	case id := <-rig.orch.investigateCh:
		t.Errorf("Watch only investigated %s", id)
	case <-time.After(300 * time.Millisecond):
	}
}

// What an alert run is told about the resource its alert is on.
func TestAlertRunSeedStatesWhatPulseDeclaresForTheAlertsResource(t *testing.T) {
	yes, no := true, false
	seed := func(t *testing.T, onBoot *bool, status string, scopeFor func(models.Alert) PatrolScope) string {
		t.Helper()
		rig := newDeadGuestRig(t, config.PatrolAutonomyMonitor)
		rig.ct.OnBoot = onBoot
		alert := models.Alert{ID: "homelab:delly:101-powered-state", Type: "powered-off", Level: "warning", ResourceID: rig.ct.ID, ResourceName: "gitea", Message: "Container 'gitea' is powered off"}
		rig.collect(status, "running", alert)
		state := rig.ps.currentPatrolRuntimeState()
		resolved, resolution := resolvePatrolScopeState(state, scopeFor(alert))
		if len(resolution.UnmatchedResourceIDs) != 0 || len(resolution.ResolvedResourceIDs) == 0 {
			t.Fatalf("scope did not resolve: %+v", resolution)
		}
		output, _ := rig.ps.seedFindingsAndContextState(&resolved, filterPatrolStateByScopeState(state, resolved))
		return output
	}
	sustained := func(alert models.Alert) PatrolScope {
		return SustainedAlertPatrolScope(alert.ID, alert.ResourceID, "system-container", alert.Type)
	}

	dead := seed(t, &yes, "stopped", sustained)
	for _, want := range []string{`Pulse declares these actions for "gitea": start.`, "Start on boot is on for this guest, so it is meant to be running."} {
		if !strings.Contains(dead, want) {
			t.Errorf("stopped guest with start on boot: seed lacks %q:\n%s", want, dead)
		}
	}
	if strings.Contains(dead, "already reported") {
		t.Errorf("no stopped finding exists, and the seed says Pulse reported one:\n%s", dead)
	}

	fired := seed(t, &yes, "stopped", func(alert models.Alert) PatrolScope {
		return AlertTriggeredPatrolScope(alert.ID, alert.ResourceID, "system-container", alert.Type)
	})
	if !strings.Contains(fired, `Pulse declares these actions for "gitea": start.`) {
		t.Errorf("a fired alert's run is not told the declared actions:\n%s", fired)
	}

	if unknown := seed(t, nil, "stopped", sustained); strings.Contains(unknown, "Start on boot") {
		t.Errorf("Proxmox did not report start on boot, and the seed states it:\n%s", unknown)
	}

	running := seed(t, &no, "running", sustained)
	if !strings.Contains(running, `Pulse declares these actions for "gitea": reboot, shutdown, stop.`) || !strings.Contains(running, "Start on boot is off for this guest.") {
		t.Errorf("running guest: seed does not state its actions and setting:\n%s", running)
	}
	if strings.Contains(running, "meant to be running") {
		t.Errorf("a running guest is described as down:\n%s", running)
	}

	cleared := seed(t, &yes, "stopped", func(alert models.Alert) PatrolScope {
		return AlertClearedPatrolScope(alert.ID, alert.ResourceID, "system-container")
	})
	if strings.Contains(cleared, "Pulse declares") {
		t.Errorf("a cleared alert's check is told about actions:\n%s", cleared)
	}
}
