package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const (
	replacedTestDockerHost    = "dock-agent-1"
	replacedTestContainerName = "immich-ml"
)

// replacedResourceRig runs real scoped Patrol runs over the unified inventory,
// with the model's turn replaced by a stub that reports one finding on the
// container that is currently there, under the ID the inventory gives it.
type replacedResourceRig struct {
	t        *testing.T
	ps       *PatrolService
	provider *patrolTestStateProvider
	orch     *integrationOrchestrator
	report   func(creator tools.PatrolFindingCreator) error
}

func newReplacedResourceRig(t *testing.T) *replacedResourceRig {
	t.Helper()
	rig := &replacedResourceRig{t: t, provider: &patrolTestStateProvider{}, orch: newIntegrationOrchestrator(true)}

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: "approval"}
	svc.provider = &mockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc.SetChatService(&patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			creator := executor.GetPatrolFindingCreator()
			if creator == nil {
				return nil, fmt.Errorf("patrol finding creator not set")
			}
			if rig.report != nil {
				if err := rig.report(creator); err != nil {
					return nil, err
				}
			}
			return &PatrolStreamResponse{Content: "done"}, nil
		},
	})

	rig.ps = NewPatrolService(svc, rig.provider)
	rig.ps.SetConfig(PatrolConfig{Enabled: true, Interval: 10 * time.Minute, AnalyzeDocker: true})
	rig.ps.SetInvestigationOrchestrator(rig.orch)
	return rig
}

// setContainers replaces the collected inventory and returns the ID the
// inventory gives each named container.
func (r *replacedResourceRig) setContainers(containers ...models.DockerContainer) map[string]string {
	r.t.Helper()
	snapshot := models.StateSnapshot{DockerHosts: []models.DockerHost{{
		ID: replacedTestDockerHost, Hostname: "docker-box", Status: "online", LastSeen: time.Now(), Containers: containers,
	}}}
	for _, container := range containers {
		if container.State == "running" {
			continue
		}
		snapshot.ActiveAlerts = append(snapshot.ActiveAlerts, models.Alert{
			ID: "docker-container-state-" + container.ID[:8], Type: "docker-container-state", Level: "warning",
			ResourceID:   alerts.DockerContainerResourceID(replacedTestDockerHost, container.ID, container.Name),
			ResourceName: container.Name, Message: "Docker container '" + container.Name + "' is exited",
		})
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	r.provider.state = snapshot
	r.ps.SetReadState(registry)
	r.ps.SetUnifiedResourceProvider(adapter)

	ids := make(map[string]string, len(containers))
	for _, resource := range adapter.GetByType(unifiedresources.ResourceTypeAppContainer) {
		ids[resource.Name] = resource.ID
	}
	for _, container := range containers {
		if ids[container.Name] == "" {
			r.t.Fatalf("inventory has no ID for container %q", container.Name)
		}
	}
	return ids
}

// alertRun runs the scoped check Patrol starts when the container's state
// alert fires, and returns its run record.
func (r *replacedResourceRig) alertRun(containerID string) PatrolRunRecord {
	r.t.Helper()
	before := r.ps.runHistoryStore.Count()
	scope := AlertTriggeredPatrolScope("docker-container-state-"+containerID[:8],
		alerts.DockerContainerResourceID(replacedTestDockerHost, containerID, replacedTestContainerName),
		"app-container", "docker-container-state")
	scope.NoStream = true
	r.ps.runScopedPatrol(context.Background(), scope)
	if got := r.ps.runHistoryStore.Count(); got != before+1 {
		r.t.Fatalf("the alert run left %d run records, want one more than %d", got, before)
	}
	run := r.ps.runHistoryStore.GetRecent(1)[0]
	if run.Status == "error" {
		r.t.Fatalf("the alert run failed: %s: %s", run.ErrorSummary, run.ErrorDetail)
	}
	return run
}

func (r *replacedResourceRig) crashReport(resourceID string) func(tools.PatrolFindingCreator) error {
	return func(creator tools.PatrolFindingCreator) error {
		_, _, err := creator.CreateFinding(tools.PatrolFindingInput{
			Key: "crashed-sigsegv", Severity: "warning", Category: "reliability",
			ResourceID: resourceID, ResourceName: replacedTestContainerName, ResourceType: "app-container",
			Title:       replacedTestContainerName + " crashed with SIGSEGV (exit code 139)",
			Description: "The container exited with code 139.", Evidence: "State: exited, exit code 139",
		})
		return err
	}
}

func (r *replacedResourceRig) investigated() string {
	r.t.Helper()
	select {
	case id := <-r.orch.investigateCh:
		return id
	case <-time.After(3 * time.Second):
		return ""
	}
}

func (r *replacedResourceRig) activeFindings() []*Finding {
	return r.ps.findings.GetActive(FindingSeverityInfo)
}

// An image update removes a container and creates a new one with the same
// name. The inventory gives the new container a new ID. A finding left on the
// old container must not take in a report about the new one: its investigation
// is finished, so the new container's crash would never be investigated.
func TestPatrolFindingOnRemovedContainerDoesNotAbsorbItsReplacement(t *testing.T) {
	rig := newReplacedResourceRig(t)
	oldContainer := strings.Repeat("054d", 16)
	newContainer := strings.Repeat("fac7", 16)

	oldID := rig.setContainers(models.DockerContainer{ID: oldContainer, Name: replacedTestContainerName, State: "exited", ExitCode: 139})[replacedTestContainerName]
	rig.report = rig.crashReport(oldID)
	if run := rig.alertRun(oldContainer); run.NewFindings != 1 {
		t.Fatalf("first crash: run reported %d new findings, want 1", run.NewFindings)
	}
	active := rig.activeFindings()
	if len(active) != 1 || active[0].ResourceID != oldID {
		t.Fatalf("first crash: active findings = %+v, want one on %s", active, oldID)
	}
	oldFinding := active[0].ID
	if got := rig.investigated(); got != oldFinding {
		t.Fatalf("first crash: investigated %q, want %q", got, oldFinding)
	}
	// The investigation finished with a fix waiting for approval.
	if !rig.ps.findings.UpdateInvestigationOutcome(oldFinding, string(InvestigationOutcomeFixQueued)) {
		t.Fatal("could not record the finished investigation")
	}

	// The operator pulls a new image. Same name, new container, and it crashes too.
	newID := rig.setContainers(models.DockerContainer{ID: newContainer, Name: replacedTestContainerName, State: "exited", ExitCode: 139})[replacedTestContainerName]
	if newID == oldID {
		t.Fatalf("fixture: the inventory kept ID %s for the new container", oldID)
	}
	rig.report = rig.crashReport(newID)
	run := rig.alertRun(newContainer)

	if run.NewFindings != 1 || run.ExistingFindings != 0 {
		t.Errorf("replacement crash: run reported %d new and %d existing findings, want 1 new", run.NewFindings, run.ExistingFindings)
	}
	active = rig.activeFindings()
	if len(active) != 1 || active[0].ResourceID != newID || active[0].ID == oldFinding {
		t.Fatalf("replacement crash: active findings = %s, want one new finding on %s", describeFindings(active), newID)
	}
	old := rig.ps.findings.Get(oldFinding)
	if old == nil || old.ResolvedAt == nil || old.ResolveReason != "Resource no longer exists in infrastructure" {
		t.Errorf("the finding on the removed container must be closed as removed, got %s", describeFindings([]*Finding{old}))
	}
	if got := rig.investigated(); got != active[0].ID {
		t.Errorf("replacement crash: investigated %q, want the new finding %q", got, active[0].ID)
	}
}

// The same container reported again stays one finding, whatever key the model
// picks for it the second time.
func TestPatrolFindingOnPresentContainerStillAbsorbsARepeatReport(t *testing.T) {
	rig := newReplacedResourceRig(t)
	container := strings.Repeat("054d", 16)
	id := rig.setContainers(models.DockerContainer{ID: container, Name: replacedTestContainerName, State: "exited", ExitCode: 139})[replacedTestContainerName]

	rig.report = rig.crashReport(id)
	rig.alertRun(container)
	first := rig.activeFindings()
	if len(first) != 1 {
		t.Fatalf("first report: active findings = %s, want one", describeFindings(first))
	}

	rig.report = func(creator tools.PatrolFindingCreator) error {
		if _, _, err := creator.CreateFinding(tools.PatrolFindingInput{
			Key: "segfault-on-start", Severity: "warning", Category: "reliability",
			ResourceID: id, ResourceName: replacedTestContainerName, ResourceType: "app-container",
			Title:       replacedTestContainerName + " crashed with SIGSEGV (exit code 139)",
			Description: "The container exited with code 139 again.", Evidence: "State: exited, exit code 139",
		}); err != nil {
			return err
		}
		// A run must give a verdict on each open finding it was shown.
		return creator.(tools.PatrolFindingAssessor).AssessFinding(tools.PatrolFindingAssessmentInput{
			FindingID: first[0].ID, Verdict: "present",
			Evidence: "State: exited, exit code 139", Reason: "The container is still exited.",
		})
	}
	run := rig.alertRun(container)
	active := rig.activeFindings()
	if len(active) != 1 || active[0].ID != first[0].ID || active[0].ResolvedAt != nil {
		t.Fatalf("repeat report: active findings = %s, want the first finding only", describeFindings(active))
	}
	if run.NewFindings != 0 {
		t.Fatalf("repeat report: run reported %d new findings, want 0", run.NewFindings)
	}
}

// A second container on another host that shares the name keeps a finding
// alive today only when the finding's own ID still names something. Once that
// ID is gone the name is not enough.
func TestSeedFindingsClosesRegistryIDFindingWhenOnlyTheNameRemains(t *testing.T) {
	rig := newReplacedResourceRig(t)
	ids := rig.setContainers(
		models.DockerContainer{ID: strings.Repeat("fac7", 16), Name: replacedTestContainerName, State: "running"},
		models.DockerContainer{ID: strings.Repeat("0b1e", 16), Name: "db", State: "running"},
	)
	goneID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAppContainer, unifiedresources.SourceDocker, replacedTestDockerHost+"/"+strings.Repeat("054d", 16))

	now := time.Now().Add(-time.Hour)
	replaced := &Finding{ID: "replaced", Key: "crashed", ResourceID: goneID, ResourceName: replacedTestContainerName, ResourceType: "app-container",
		Title: "Old container crashed", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, DetectedAt: now, LastSeenAt: now}
	present := &Finding{ID: "present", Key: "restarts", ResourceID: ids["db"], ResourceName: "db", ResourceType: "app-container",
		Title: "db restarted", Severity: FindingSeverityWarning, Category: FindingCategoryReliability, DetectedAt: now, LastSeenAt: now}
	// Not every finding carries a registry ID. One that names its resource in
	// another form is still tied to it by name, as before.
	namedOnly := &Finding{ID: "named-only", Key: "backup-stale", ResourceID: "rollup:docker-box/db", ResourceName: "db", ResourceType: "app-container",
		Title: "db backup not verified", Severity: FindingSeverityWatch, Category: FindingCategoryBackup, DetectedAt: now, LastSeenAt: now}
	for _, f := range []*Finding{replaced, present, namedOnly} {
		rig.ps.findings.Add(f)
	}

	output, _ := rig.ps.seedFindingsAndContextState(nil, rig.ps.currentPatrolRuntimeState())

	if got := rig.ps.findings.Get("replaced"); got.ResolvedAt == nil || got.ResolveReason != "Resource no longer exists in infrastructure" {
		t.Errorf("a finding whose registry ID is gone must close even though its name is still in use: %s", describeFindings([]*Finding{got}))
	}
	for _, id := range []string{"present", "named-only"} {
		if got := rig.ps.findings.Get(id); got.ResolvedAt != nil {
			t.Errorf("finding %q is on a resource that is still there and must stay open", id)
		}
	}
	if strings.Contains(output, "Old container crashed") || !strings.Contains(output, "db restarted") {
		t.Errorf("the model must be shown the open findings only, got: %s", output)
	}
}

// A model that miscopies a registry ID still names the resource. The report is
// filed under the real ID, so it is not closed as removed by the next run.
func TestPatrolFindingAdapterRepairsMiscopiedRegistryIDOnAFullRun(t *testing.T) {
	rig := newReplacedResourceRig(t)
	ids := rig.setContainers(models.DockerContainer{ID: strings.Repeat("fac7", 16), Name: replacedTestContainerName, State: "running"})
	realID := ids[replacedTestContainerName]
	miscopied := realID[:len(realID)-1] + map[bool]string{true: "0", false: "1"}[realID[len(realID)-1] != '0']

	adapter := newPatrolFindingCreatorAdapterState(rig.ps, rig.ps.currentPatrolRuntimeState())
	id, isNew, err := adapter.CreateFinding(tools.PatrolFindingInput{
		Key: "no-restart-policy", Severity: "watch", Category: "reliability",
		ResourceID: miscopied, ResourceName: replacedTestContainerName, ResourceType: "app-container",
		Title: "No restart policy", Description: "The container has no restart policy.", Evidence: "RestartPolicy: no",
	})
	if err != nil || !isNew {
		t.Fatalf("CreateFinding = (%q, %t, %v), want a new finding", id, isNew, err)
	}
	stored := rig.ps.findings.Get(id)
	if stored == nil || stored.ResourceID != realID {
		t.Fatalf("the finding must be filed under the real ID %s, got %s", realID, describeFindings([]*Finding{stored}))
	}
	rig.ps.seedFindingsAndContextState(nil, rig.ps.currentPatrolRuntimeState())
	if got := rig.ps.findings.Get(id); got.ResolvedAt != nil {
		t.Fatal("the next run closed a finding on a resource that is still there")
	}
}

func describeFindings(findings []*Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		if f == nil {
			parts = append(parts, "<nil>")
			continue
		}
		state := "open"
		if f.ResolvedAt != nil {
			state = "resolved (" + f.ResolveReason + ")"
		}
		parts = append(parts, fmt.Sprintf("{%s on %s, %s}", f.ID, f.ResourceID, state))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// The form Patrol treats as a registry ID is the form the registry mints.
func TestPatrolRegistryMintedResourceIDMatchesTheRegistry(t *testing.T) {
	for _, id := range []string{
		unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAppContainer, unifiedresources.SourceDocker, "host/container"),
		unifiedresources.SourceSpecificID(unifiedresources.ResourceTypePhysicalDisk, unifiedresources.SourceAgent, "host/serial"),
		unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, "machine-1"),
		unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "pve", 100),
	} {
		if !patrolRegistryMintedResourceID(id) {
			t.Errorf("registry ID %q was not recognised", id)
		}
	}
	for _, id := range []string{
		"", "immich-ml", "node-1", "vm-100", "pve1:node1:100", "rollup:docker-box/db",
		strings.Repeat("ab12", 16), "66cd990e-2def-40c7-a9e4-c450a9e450de",
		alerts.DockerContainerResourceID("dock-agent-1", strings.Repeat("ab12", 16), "web"),
		"app-container-FAC7824B7E0EB99D", "-fac7824b7e0eb99d", "app-container-fac7824b7e0eb99",
	} {
		if patrolRegistryMintedResourceID(id) {
			t.Errorf("%q is not a registry ID and was taken for one", id)
		}
	}
}
