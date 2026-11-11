package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// What a model that calls no tools leaves behind: the fault in prose and no
// finding. Seen with a small local model on every critical alert.
const alertRunProseOnly = `## Infrastructure Status
Health Status: Unhealthy. Severity: Critical.

## Actions Taken
Next steps: investigate the container.`

// alertRunRig runs the scoped check a firing alert starts, over the unified
// inventory, with the model's turn replaced by a stub.
type alertRunRig struct {
	t             *testing.T
	ps            *PatrolService
	provider      *patrolTestStateProvider
	orch          *integrationOrchestrator
	alert         models.Alert
	containerID   string
	modelTurns    int
	notifications int
	// decisionPrompt is the follow-up question the last run handed to the chat loop.
	decisionPrompt string
	model          func(creator tools.PatrolFindingCreator) (*PatrolStreamResponse, error)
}

func newAlertRunRig(t *testing.T, mode string) *alertRunRig {
	t.Helper()
	rig := &alertRunRig{t: t, provider: &patrolTestStateProvider{}, orch: newIntegrationOrchestrator(true)}

	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "mock:model", PatrolAutonomyLevel: mode}
	svc.provider = &mockProvider{}
	executor := tools.NewPulseToolExecutor(tools.ExecutorConfig{})
	svc.SetChatService(&patrolMockChatService{
		executor: executor,
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			rig.modelTurns++
			rig.decisionPrompt = req.FindingDecisionPrompt
			if rig.model != nil {
				return rig.model(executor.GetPatrolFindingCreator())
			}
			return &PatrolStreamResponse{Content: alertRunProseOnly}, nil
		},
	})

	rig.ps = NewPatrolService(svc, rig.provider)
	rig.ps.SetConfig(PatrolConfig{Enabled: true, Interval: 10 * time.Minute, AnalyzeDocker: true})
	rig.ps.SetInvestigationOrchestrator(rig.orch)
	rig.ps.SetFindingNotifyCallback(func(*Finding) { rig.notifications++ })
	rig.ps.SetPushNotifyCallback(func(relay.PushNotificationPayload) { rig.notifications++ })

	dockerContainer := strings.Repeat("ab12", 16)
	rig.alert = models.Alert{
		ID: "docker-container-health-web", Type: "docker-container-health", Level: "critical",
		ResourceID:   alerts.DockerContainerResourceID("dock-agent-1", dockerContainer, "web"),
		ResourceName: "web", Message: "Docker container 'web' health is unhealthy", StartTime: time.Now().Add(-time.Minute),
	}
	rig.setAlertActive(true)
	return rig
}

// setAlertActive rebuilds the collected state with the container unhealthy and
// its alert firing, or healthy with the alert gone.
func (r *alertRunRig) setAlertActive(active bool) {
	r.t.Helper()
	health := "healthy"
	if active {
		health = "unhealthy"
	}
	snapshot := models.StateSnapshot{DockerHosts: []models.DockerHost{{
		ID: "dock-agent-1", Hostname: "docker-box", Status: "online", LastSeen: time.Now(),
		Containers: []models.DockerContainer{{ID: strings.Repeat("ab12", 16), Name: "web", State: "running", Health: health}},
	}}}
	if active {
		snapshot.ActiveAlerts = []models.Alert{r.alert}
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	adapter := unifiedresources.NewUnifiedAIAdapter(registry)
	r.provider.state = snapshot
	r.ps.SetReadState(registry)
	r.ps.SetUnifiedResourceProvider(adapter)
	for _, resource := range adapter.GetByType(unifiedresources.ResourceTypeAppContainer) {
		r.containerID = resource.ID
	}
	if r.containerID == "" {
		r.t.Fatal("inventory has no ID for the container")
	}
}

func (r *alertRunRig) firedScope() PatrolScope {
	scope := AlertTriggeredPatrolScope(r.alert.ID, r.alert.ResourceID, "app-container", r.alert.Type)
	scope.NoStream = true
	return scope
}

// run executes one scoped run and returns its record.
func (r *alertRunRig) run(scope PatrolScope) PatrolRunRecord {
	r.t.Helper()
	before, turns := r.ps.runHistoryStore.Count(), r.modelTurns
	r.ps.runScopedPatrol(context.Background(), scope)
	if got := r.ps.runHistoryStore.Count(); got != before+1 {
		r.t.Fatalf("the run left %d run records, want one more than %d", got, before)
	}
	if r.modelTurns != turns+1 {
		r.t.Fatalf("the model was asked %d times, want once", r.modelTurns-turns)
	}
	return r.ps.runHistoryStore.GetRecent(1)[0]
}

func (r *alertRunRig) investigated() string {
	r.t.Helper()
	select {
	case id := <-r.orch.investigateCh:
		return id
	case <-time.After(2 * time.Second):
		return ""
	}
}

func (r *alertRunRig) notInvestigated(when string) {
	r.t.Helper()
	select {
	case id := <-r.orch.investigateCh:
		r.t.Errorf("%s: finding %q was investigated, want none", when, id)
	case <-time.After(300 * time.Millisecond):
	}
}

func (r *alertRunRig) active() []*Finding {
	return r.ps.findings.GetActive(FindingSeverityInfo)
}

func listAlertRunFindings(findings []*Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		if f == nil {
			parts = append(parts, "<nil>")
			continue
		}
		parts = append(parts, fmt.Sprintf("{%s %q on %s, source %q, alert %q}", f.Severity, f.Title, f.ResourceID, f.Source, f.AlertIdentifier))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// An alert the operator asked Patrol to investigate wakes Patrol. When the
// model writes about the fault and files no finding, the alert must still be
// investigated: Pulse opens the finding from the alert.
func TestAlertRunWithoutAFindingOpensOneFromTheAlert(t *testing.T) {
	rig := newAlertRunRig(t, config.PatrolAutonomyApproval)

	run := rig.run(rig.firedScope())

	active := rig.active()
	if len(active) != 1 {
		t.Fatalf("active findings = %s, want one opened from the alert", listAlertRunFindings(active))
	}
	finding := active[0]
	if finding.ResourceID != rig.containerID || finding.ResourceName != "web" || finding.Severity != FindingSeverityCritical {
		t.Errorf("finding = %s, want a critical finding on %s", listAlertRunFindings(active), rig.containerID)
	}
	if finding.AlertIdentifier != rig.alert.ID || finding.MirrorsAlertID != rig.alert.ID || finding.Source != "alert" {
		t.Errorf("finding must be marked as coming from alert %q and folded under it: %s, mirrors %q", rig.alert.ID, listAlertRunFindings(active), finding.MirrorsAlertID)
	}
	if !strings.Contains(finding.Title, "health is unhealthy") {
		t.Errorf("finding title %q must carry the alert's text", finding.Title)
	}
	if got := rig.investigated(); got != finding.ID {
		t.Errorf("investigated %q, want the finding opened from the alert %q", got, finding.ID)
	}
	if rig.notifications != 0 {
		t.Errorf("the finding repeats an alert the operator was already told about, sent %d more notifications", rig.notifications)
	}
	if run.Status != "critical" || run.NewFindings != 1 || len(run.FindingIDs) != 1 || run.FindingIDs[0] != finding.ID {
		t.Errorf("run record = status %q, %d new, findings %v, want critical with the opened finding", run.Status, run.NewFindings, run.FindingIDs)
	}
	if strings.Contains(run.AIAnalysis, "all clear") || !strings.Contains(run.AIAnalysis, "opened one from the alert") {
		t.Errorf("run summary must say what happened, got:\n%s", run.AIAnalysis)
	}

	// The alert clears. The finding closes with it, like any finding that
	// restates an alert.
	rig.setAlertActive(false)
	if closed := rig.ps.ResolveFindingsForClearedAlert(rig.alert.ID, rig.alert.ResourceID, rig.alert.Type); closed != 1 {
		t.Fatalf("clearing the alert closed %d findings, want 1", closed)
	}
	if left := rig.active(); len(left) != 0 {
		t.Fatalf("findings still open after the alert cleared: %s", listAlertRunFindings(left))
	}

	// It fires again. One finding, the same one, and one more investigation.
	rig.setAlertActive(true)
	rig.run(rig.firedScope())
	active = rig.active()
	if len(active) != 1 || active[0].ID != finding.ID || active[0].RegressionCount != 1 {
		t.Fatalf("second episode: active findings = %s, want the first finding reopened", listAlertRunFindings(active))
	}
	if got := rig.investigated(); got != finding.ID {
		t.Errorf("second episode: investigated %q, want %q", got, finding.ID)
	}
}

// Watch only never investigates, so a finding there would only repeat the
// alert. The run still must not claim that all is clear.
func TestAlertRunWithoutAFindingStaysQuietInWatchOnly(t *testing.T) {
	rig := newAlertRunRig(t, config.PatrolAutonomyMonitor)

	run := rig.run(rig.firedScope())

	if active := rig.active(); len(active) != 0 {
		t.Fatalf("Watch only: active findings = %s, want none", listAlertRunFindings(active))
	}
	rig.notInvestigated("Watch only")
	if strings.Contains(run.AIAnalysis, "all clear") || !strings.Contains(run.AIAnalysis, "is still active") {
		t.Errorf("a run on a firing alert must not claim all clear, got:\n%s", run.AIAnalysis)
	}
	if run.FindingsSummary == "All healthy" {
		t.Errorf("a run on a firing alert must not be summed up as %q", run.FindingsSummary)
	}
}

func TestAlertRunWithoutAFindingKeepsTheModelsJudgment(t *testing.T) {
	t.Run("a warning that stayed active", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		scope := SustainedAlertPatrolScope(rig.alert.ID, rig.alert.ResourceID, "app-container", rig.alert.Type)
		scope.NoStream = true
		run := rig.run(scope)
		if active := rig.active(); len(active) != 0 {
			t.Fatalf("a sustained alert run that files nothing must stay quiet, got %s", listAlertRunFindings(active))
		}
		rig.notInvestigated("sustained alert")
		if strings.Contains(run.AIAnalysis, "all clear") {
			t.Errorf("a run on a firing alert must not claim all clear, got:\n%s", run.AIAnalysis)
		}
	})

	t.Run("a run a person started", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		rig.run(PatrolScope{ResourceIDs: []string{rig.alert.ResourceID}, AlertIdentifier: rig.alert.ID, Reason: TriggerReasonManual, NoStream: true})
		if active := rig.active(); len(active) != 0 {
			t.Fatalf("a manual run that files nothing must stay quiet, got %s", listAlertRunFindings(active))
		}
		rig.notInvestigated("manual run")
	})

	// The same run as the Patrol page starts it: the resource and no alert.
	// Pulse still knows the resource has an active alert, so the record must
	// not sum the run up as healthy.
	t.Run("a run a person started on the resource", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		run := rig.run(PatrolScope{ResourceIDs: []string{rig.containerID}, Reason: TriggerReasonManual, NoStream: true})
		if active := rig.active(); len(active) != 0 {
			t.Fatalf("a manual run that files nothing must stay quiet, got %s", listAlertRunFindings(active))
		}
		rig.notInvestigated("manual run on the resource")
		if strings.Contains(run.AIAnalysis, "all clear") || !strings.Contains(run.AIAnalysis, "critical alert on web is still active") {
			t.Errorf("a run on a resource with an active alert must say so, got:\n%s", run.AIAnalysis)
		}
		if run.FindingsSummary == "All healthy" {
			t.Errorf("a run on a resource with an active alert must not be summed up as %q", run.FindingsSummary)
		}

		// Once the alert is gone, a quiet run is a quiet run.
		rig.setAlertActive(false)
		run = rig.run(PatrolScope{ResourceIDs: []string{rig.containerID}, Reason: TriggerReasonManual, NoStream: true})
		if !strings.Contains(run.AIAnalysis, "all clear") || run.FindingsSummary != "All healthy" {
			t.Errorf("a run on a resource with no alert: summary %q, analysis:\n%s", run.FindingsSummary, run.AIAnalysis)
		}
	})

	t.Run("the model filed its own finding", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		rig.model = func(creator tools.PatrolFindingCreator) (*PatrolStreamResponse, error) {
			if _, _, err := creator.CreateFinding(tools.PatrolFindingInput{
				Key: "oom-killed", Severity: "critical", Category: "reliability",
				ResourceID: rig.containerID, ResourceName: "web", ResourceType: "app-container",
				Title: "web is killed for memory on every start", Description: "The kernel kills the process.", Evidence: "OOMKilled: true",
			}); err != nil {
				return nil, err
			}
			return &PatrolStreamResponse{Content: alertRunProseOnly}, nil
		}
		rig.run(rig.firedScope())
		active := rig.active()
		if len(active) != 1 || active[0].Source != "ai-analysis" {
			t.Fatalf("active findings = %s, want only the model's finding", listAlertRunFindings(active))
		}
		if got := rig.investigated(); got != active[0].ID {
			t.Errorf("investigated %q, want the model's finding %q", got, active[0].ID)
		}
	})

	t.Run("the alert cleared while the model was looking", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		rig.model = func(tools.PatrolFindingCreator) (*PatrolStreamResponse, error) {
			rig.setAlertActive(false)
			return &PatrolStreamResponse{Content: alertRunProseOnly}, nil
		}
		rig.run(rig.firedScope())
		if active := rig.active(); len(active) != 0 {
			t.Fatalf("no finding may be opened for an alert that is gone, got %s", listAlertRunFindings(active))
		}
		rig.notInvestigated("cleared alert")
	})

	t.Run("the model could not be reached", func(t *testing.T) {
		rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
		rig.model = func(tools.PatrolFindingCreator) (*PatrolStreamResponse, error) {
			return nil, errors.New("provider connection reset")
		}
		rig.run(rig.firedScope())
		for _, f := range rig.active() {
			if f.Source == "alert" {
				t.Fatalf("a failed model call must not open a finding from the alert, got %s", listAlertRunFindings(rig.active()))
			}
		}
	})
}

// The operator's answer to the finding holds across episodes.
func TestAlertRunDoesNotReopenADismissedAlertFinding(t *testing.T) {
	rig := newAlertRunRig(t, config.PatrolAutonomyApproval)
	rig.run(rig.firedScope())
	active := rig.active()
	if len(active) != 1 {
		t.Fatalf("active findings = %s, want one opened from the alert", listAlertRunFindings(active))
	}
	if got := rig.investigated(); got != active[0].ID {
		t.Fatalf("investigated %q, want %q", got, active[0].ID)
	}
	if !rig.ps.findings.Dismiss(active[0].ID, "not_an_issue", "this container is allowed to be unhealthy") {
		t.Fatal("could not dismiss the finding")
	}

	rig.run(rig.firedScope())
	if left := rig.active(); len(left) != 0 {
		t.Fatalf("a dismissed alert finding came back: %s", listAlertRunFindings(left))
	}
	rig.notInvestigated("dismissed finding")
}

// A scoped run on a resource with an active alert hands the chat loop one
// follow-up question, to be asked if the run ends in prose and records
// nothing. A run with no active alert on its resource asks nothing.
func TestAlertRunHandsTheLoopAFindingDecisionQuestion(t *testing.T) {
	rig := newAlertRunRig(t, config.PatrolAutonomyApproval)

	rig.run(PatrolScope{ResourceIDs: []string{rig.containerID}, Reason: TriggerReasonManual, NoStream: true})
	for _, want := range []string{"critical alert active on \"web\"", rig.alert.Message, "patrol_report_finding", "no finding is needed"} {
		if !strings.Contains(rig.decisionPrompt, want) {
			t.Errorf("manual run on a resource with an active alert: question lacks %q, got %q", want, rig.decisionPrompt)
		}
	}

	sustained := SustainedAlertPatrolScope(rig.alert.ID, rig.alert.ResourceID, "app-container", rig.alert.Type)
	sustained.NoStream = true
	rig.run(sustained)
	if rig.decisionPrompt == "" {
		t.Error("sustained alert run: no question was handed to the loop")
	}

	rig.setAlertActive(false)
	rig.run(PatrolScope{ResourceIDs: []string{rig.containerID}, Reason: TriggerReasonManual, NoStream: true})
	if rig.decisionPrompt != "" {
		t.Errorf("a run on a resource with no active alert must ask nothing, got %q", rig.decisionPrompt)
	}
}
