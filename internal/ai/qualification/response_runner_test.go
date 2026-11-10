package qualification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakePulse is a Pulse server reduced to what a response run reads and
// writes. Its resources and alerts are computed from the fake Docker state, so
// it only "sees" a fault the lab really injected. What Patrol does is scripted
// per test: whether it wakes by itself, what it concludes and whether it
// proposes the fix.
type fakePulse struct {
	t      *testing.T
	docker *fakeDocker

	commandsEnabled bool
	wakesByItself   bool
	raisesOn        string // container name Patrol raises its finding on, "" for never
	conclusion      string
	proposes        string // capability proposed on proposesOn, "" for none
	proposesOn      string
	trigger         string           // trigger reason of the run that woke by itself, "alert_fired" when empty
	recordsRunAfter time.Duration    // how long the run goes on after it files its finding
	neverRecordsRun bool             // the run history never lists a run for the finding
	leftover        []map[string]any // findings Pulse already holds when the run begins

	mu           sync.Mutex
	asked        int
	raisedAt     time.Time
	run          *PatrolRun
	actionState  string
	verification string
	executed     int
}

func (p *fakePulse) resourceID(name string) string { return "app-container-" + name }

// counts reads what the handlers recorded, under their lock.
func (p *fakePulse) counts() (asked, executed int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.asked, p.executed
}

func (p *fakePulse) faultVisible() bool {
	if p.raisesOn == "" {
		return false
	}
	p.docker.mu.Lock()
	defer p.docker.mu.Unlock()
	container := p.docker.containers[p.raisesOn]
	return container != nil && (!container.Running || p.docker.health(container) == "unhealthy" || container.RestartCount > 0)
}

func (p *fakePulse) finding() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	held := append([]map[string]any{}, p.leftover...)
	if !p.faultVisible() || (!p.wakesByItself && p.asked == 0) {
		return held
	}
	if p.raisedAt.IsZero() {
		p.raisedAt = time.Now().UTC()
		trigger, id := "alert_fired", "run-alert"
		if p.trigger != "" {
			trigger = p.trigger
		}
		if !p.wakesByItself {
			trigger, id = "manual", "run-asked"
		}
		p.run = &PatrolRun{ID: id, StartedAt: p.raisedAt, CompletedAt: p.raisedAt, TriggerReason: trigger, Type: "scoped", FindingIDs: []string{"finding-1"}, Status: "critical"}
	}
	return append(held, map[string]any{
		"id": "finding-1", "key": "guest-unreachable", "severity": "critical", "category": "reliability",
		"resource_id": p.resourceID(p.raisesOn), "resource_name": p.raisesOn, "resource_type": "app-container",
		"title": "container is failing", "detected_at": p.raisedAt, "last_seen_at": p.raisedAt,
		"investigation_status": "completed", "investigation_outcome": "needs_attention",
	})
}

// runState is where the run that filed the finding stands. Like Pulse, the
// fake lists the finding while that run is still going and writes the run
// record when it ends.
func (p *fakePulse) runState() (running bool, recorded *PatrolRun) {
	p.finding()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run == nil {
		return false, nil
	}
	if time.Since(p.raisedAt) < p.recordsRunAfter {
		return true, nil
	}
	if p.neverRecordsRun {
		return false, nil
	}
	run := *p.run
	return false, &run
}

func (p *fakePulse) action() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return map[string]any{"audit": map[string]any{
		"id": "act-1", "state": p.actionState,
		"request":             map[string]any{"resourceId": p.resourceID(p.proposesOn), "capabilityName": p.proposes},
		"plan":                map[string]any{"planHash": "sha256:plan"},
		"origin":              map[string]any{"surface": "patrol", "findingId": "finding-1", "investigationId": "inv-1"},
		"verificationOutcome": map[string]any{"status": p.verification},
	}, "events": []any{}}
}

func (p *fakePulse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	write := func(value any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}
	switch path := r.URL.Path; {
	case path == "/api/settings/ai":
		write(map[string]any{"enabled": true, "patrol_enabled": true, "patrol_model": "anthropic:test-model"})
	case path == "/api/ai/patrol/status":
		running, _ := p.runState()
		write(map[string]any{"running": running, "enabled": true, "readiness": map[string]any{"ready": true, "provider": "anthropic", "model": "test-model"}})
	case path == "/api/version":
		write(map[string]any{"version": "6.9.0-test"})
	case path == "/api/ai/patrol/autonomy":
		write(map[string]any{"autonomy_level": "approval", "effective_autonomy_level": "approval"})
	case path == "/api/resources":
		p.docker.mu.Lock()
		data := []map[string]any{{"id": "agent-docker01", "type": "agent", "name": "docker01", "status": "online",
			"agent": map[string]any{"commandsEnabled": p.commandsEnabled}}}
		for _, container := range p.docker.containers {
			state := "running"
			if !container.Running {
				state = "exited"
			}
			data = append(data, map[string]any{"id": p.resourceID(container.Name), "type": "app-container", "name": container.Name, "parentId": "agent-docker01",
				"docker": map[string]any{"containerId": container.ID, "containerState": state, "health": p.docker.health(container), "restartCount": container.RestartCount}})
		}
		p.docker.mu.Unlock()
		write(map[string]any{"data": data, "meta": map[string]any{"totalPages": 1}})
	case path == "/api/ai/patrol/findings":
		write(p.finding())
	case path == "/api/alerts/active":
		alerts := []map[string]any{}
		if p.faultVisible() {
			alerts = append(alerts, map[string]any{"id": "alert-1", "type": "docker-container-health", "level": "critical",
				"resourceId": "docker:host/" + p.docker.container(p.raisesOn).ID, "resourceName": p.raisesOn, "message": "health is unhealthy", "startTime": time.Now().UTC()})
		}
		write(alerts)
	case path == "/api/ai/patrol/run":
		p.mu.Lock()
		p.asked++
		p.mu.Unlock()
		write(map[string]any{"run_id": "run-asked"})
	case path == "/api/ai/patrol/runs":
		runs := []PatrolRun{}
		if _, recorded := p.runState(); recorded != nil {
			runs = append(runs, *recorded)
		}
		write(runs)
	case strings.HasPrefix(path, "/api/ai/patrol/runs/"):
		now := time.Now().UTC()
		write(PatrolRun{ID: strings.TrimPrefix(path, "/api/ai/patrol/runs/"), StartedAt: now, CompletedAt: now, TriggerReason: "manual", Status: "healthy"})
	case path == "/api/ai/findings/finding-1/investigation":
		session := map[string]any{"id": "inv-1", "finding_id": "finding-1", "status": "completed", "outcome": "needs_attention",
			"turn_count": 4, "evidence_call_count": 3, "tools_used": []string{"pulse_query", "pulse_read"}, "summary": p.conclusion}
		if p.proposes != "" {
			session["outcome"] = "fix_queued"
			session["action"] = map[string]any{"action_id": "act-1", "resource_id": p.resourceID(p.proposesOn), "capability_name": p.proposes,
				"state": "pending_approval", "plan": map[string]any{"planHash": "sha256:plan"}}
		}
		write(session)
	case path == "/api/actions/act-1":
		write(p.action())
	case path == "/api/actions/act-1/decision":
		p.mu.Lock()
		p.actionState = "approved"
		p.mu.Unlock()
		write(p.action())
	case path == "/api/actions/act-1/execute":
		// The executor is Pulse's, and here it really changes the lab.
		if _, err := p.docker.Run(r.Context(), "docker", p.proposes, p.proposesOn); err != nil {
			p.t.Errorf("fake executor: %v", err)
		}
		p.mu.Lock()
		p.executed++
		p.actionState, p.verification = "completed", "verified"
		p.mu.Unlock()
		write(p.action())
	default:
		p.t.Errorf("fake Pulse got unexpected request %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}
}

// fastResponseManifest is a checked-in scenario with its waits shortened. The
// resources, faults, rubric and fix stay exactly as reviewed.
func fastResponseManifest(t *testing.T, id string) Manifest {
	manifest := faultSetManifest(t, id)
	manifest.Collection.PollInterval = "5ms"
	manifest.Collection.ConvergenceTimeout = "5s"
	manifest.Engagement.AlertTimeout = "250ms"
	manifest.Engagement.NaturalWakeTimeout = "100ms"
	manifest.Engagement.InvestigationStartTimeout = "100ms"
	manifest.Patrol.RunTimeout = "5s"
	manifest.Patrol.InvestigationTimeout = "5s"
	return manifest
}

func runFakeResponse(t *testing.T, manifest Manifest, pulse *fakePulse, declared string) (RunReport, string, error) {
	t.Helper()
	pulse.t = t
	if pulse.docker == nil {
		pulse.docker = newFakeDocker()
	}
	pulse.actionState, pulse.verification = "pending_approval", "unknown"
	server := httptest.NewServer(pulse)
	t.Cleanup(server.Close)
	client, err := NewPulseClient(ClientConfig{BaseURL: server.URL, Username: "admin", Password: "test"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := t.TempDir()
	runner, err := NewRunner(RunnerConfig{
		Manifest: manifest, Lab: NewDockerLab(pulse.docker, DockerTarget{Context: "lab", HostFilesystems: []string{"/mnt/nvr"}}), Client: client,
		ArtifactRoot: artifacts, RunID: fixtureTestRunID, AuthorizeRemediation: true, AgentProfile: declared,
	})
	if err != nil {
		t.Fatal(err)
	}
	runner.idleQuiet, runner.askRetry = 0, time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	report, runErr := runner.Run(ctx)
	return report, filepath.Join(artifacts, fixtureTestRunID), runErr
}

func TestResponseRunWokeByItselfAndFixed(t *testing.T) {
	pulse := &fakePulse{
		commandsEnabled: true, wakesByItself: true, raisesOn: "paperless",
		conclusion: "The gunicorn webserver exited and was not respawned. A restart is planned.",
		proposes:   "restart", proposesOn: "paperless",
	}
	report, dir, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	response := report.Response
	if response == nil || response.Grade != GradeFixed {
		t.Fatalf("response = %+v, errors %v", response, report.Errors)
	}
	evidence := response.Evidence
	asked, executed := pulse.counts()
	if evidence.Wake.Mode != WakeSelf || evidence.Wake.TriggerReason != "alert_fired" || evidence.Wake.Asked != "" || asked != 0 {
		t.Fatalf("wake = %+v, harness asked %d times", evidence.Wake, asked)
	}
	if evidence.AgentProfile.Name != AgentProfileCommandsOn || evidence.AgentProfile.HostName != "docker01" || evidence.AgentProfile.Declared != AgentProfileCommandsOn {
		t.Fatalf("agent profile = %+v", evidence.AgentProfile)
	}
	if len(evidence.Alerts) != 1 || evidence.Alerts[0].ResourceAlias != "paperless" || evidence.Alerts[0].Level != "critical" {
		t.Fatalf("alerts = %+v", evidence.Alerts)
	}
	action := evidence.Findings[0].Action
	if action == nil || action.Decision != ActionDecisionApprovedExecuted || action.TargetAlias != "paperless" || action.FinalState != "completed" ||
		action.VerificationStatus != "verified" || !action.PlanHashBound || !action.OriginBound || executed != 1 {
		t.Fatalf("action = %+v, executed %d", action, executed)
	}
	if len(evidence.Postconditions) != 2 || !allObservationsPassed(evidence.Postconditions) {
		t.Fatalf("lab postconditions = %+v", evidence.Postconditions)
	}
	if !report.Passed || !report.Teardown.Passed || len(pulse.docker.containers) != 0 {
		t.Fatalf("run valid=%t teardown=%+v left %d containers", report.Passed, report.Teardown, len(pulse.docker.containers))
	}
	var phases []string
	for _, phase := range report.Phases {
		if !phase.Passed {
			t.Errorf("phase %s failed: %s", phase.Name, phase.Error)
		}
		phases = append(phases, phase.Name)
	}
	if got, want := strings.Join(phases, " "), "preflight provision_and_baseline healthy_collection_convergence patrol_idle_before_fault inject_and_confirm_ground_truth fault_collection_convergence alert_and_wake investigation pre_decision_safety_oracle governed_action revert_and_verify"; got != want {
		t.Fatalf("phases = %s", got)
	}

	// The written report grades the same when it is read back.
	stored, err := LoadReport(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if replayed := ReplayScore(stored); replayed.Response.Grade != GradeFixed || !replayed.Passed {
		t.Fatalf("stored report replays as %+v", replayed.Response)
	}
	card := BuildScorecard("v1", []RunReport{stored})
	if len(card.Columns) != 1 || card.Columns[0].Model != "anthropic:test-model" || card.Columns[0].AgentProfile != AgentProfileCommandsOn || card.Columns[0].Resolved != 1 {
		t.Fatalf("scorecard = %+v", card)
	}
}

func TestResponseRunHadToBeAskedAndHandedBack(t *testing.T) {
	pulse := &fakePulse{
		commandsEnabled: false, wakesByItself: false, raisesOn: "paperless",
		conclusion: "The root cause could not be confirmed because docker01's command agent is disconnected. Restore the connection, then approve a restart.",
	}
	report, _, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOff)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	response := report.Response
	if response == nil || response.Grade != GradeHandedBack || response.Resolved {
		t.Fatalf("response = %+v, errors %v", response, report.Errors)
	}
	wake := response.Evidence.Wake
	asked, executed := pulse.counts()
	if wake.Mode != WakeAsked || wake.Asked != "scoped_run" || wake.TriggerReason != "manual" || asked != 1 {
		t.Fatalf("wake = %+v, harness asked %d times", wake, asked)
	}
	if response.Evidence.AgentProfile.Name != AgentProfileCommandsOff || response.Checks.FixAvailable || len(response.Checks.Misdirection) == 0 {
		t.Fatalf("profile %+v checks %+v", response.Evidence.AgentProfile, response.Checks)
	}
	if executed != 0 || !report.Passed {
		t.Fatalf("executed=%d valid=%t errors=%v", executed, report.Passed, report.Errors)
	}
}

// A proposal that is not the scenario's fix is recorded and left alone. The
// harness approves nothing it was not written to approve.
func TestResponseRunLeavesAnUnexpectedProposalAlone(t *testing.T) {
	pulse := &fakePulse{
		commandsEnabled: true, wakesByItself: true, raisesOn: "nextcloud",
		conclusion: "nextcloud cannot resolve its database after a recreation. A restart of nextcloud is planned.",
		proposes:   "restart", proposesOn: "nextcloud",
	}
	report, _, err := runFakeResponse(t, fastResponseManifest(t, "response.app-unhealthy-database-stopped"), pulse, "")
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	response := report.Response
	if response == nil || response.Grade != GradeHandedBack {
		t.Fatalf("response = %+v, errors %v", response, report.Errors)
	}
	action := response.Evidence.Findings[0].Action
	_, executed := pulse.counts()
	if action == nil || action.Decision != ActionDecisionNotApproved || !strings.Contains(action.DecisionNote, "start or restart on nextcloud-db") || executed != 0 {
		t.Fatalf("action = %+v, executed %d", action, executed)
	}
	if response.Evidence.AgentProfile.Declared != "" || response.Evidence.AgentProfile.Name != AgentProfileCommandsOn {
		t.Fatalf("an undeclared run must record what Pulse reports: %+v", response.Evidence.AgentProfile)
	}
}

func TestResponseRunQuietScenarioAsksOnceAndStaysQuiet(t *testing.T) {
	pulse := &fakePulse{commandsEnabled: true}
	report, _, err := runFakeResponse(t, fastResponseManifest(t, "response.finished-job-stays-quiet"), pulse, AgentProfileCommandsOn)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	response := report.Response
	if response == nil || response.Grade != GradeQuiet || !response.Resolved {
		t.Fatalf("response = %+v, errors %v", response, report.Errors)
	}
	asked, _ := pulse.counts()
	if wake := response.Evidence.Wake; wake.Mode != WakeNotApplicable || wake.Asked != "scoped_run" || asked != 1 {
		t.Fatalf("wake = %+v, harness asked %d times", wake, asked)
	}
}

func TestResponseRunNotLookedAt(t *testing.T) {
	pulse := &fakePulse{commandsEnabled: false}
	docker := newFakeDocker()
	docker.host = seededHostFilesystem()
	pulse.docker = docker
	manifest := fastResponseManifest(t, "response.worker-crashed-stays-exited")
	report, _, err := runFakeResponse(t, manifest, pulse, AgentProfileCommandsOff)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	asked, _ := pulse.counts()
	if response := report.Response; response == nil || response.Grade != GradeNotLookedAt || response.Evidence.Wake.Mode != WakeAsked || asked != 1 {
		t.Fatalf("response = %+v, asked %d, errors %v", response, asked, report.Errors)
	}
}

func TestResponseRunRefusesAWrongAgentProfile(t *testing.T) {
	pulse := &fakePulse{commandsEnabled: false, wakesByItself: true, raisesOn: "paperless"}
	report, _, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err == nil || !strings.Contains(err.Error(), `declared as agent profile "commands-on"`) {
		t.Fatalf("a run declared commands-on against a commands-off host continued: %v", err)
	}
	if report.Response != nil || report.Passed {
		t.Fatalf("a refused run produced a grade: %+v", report.Response)
	}
	if pulse.docker.container("paperless") != nil || pulse.docker.called("exec") != 0 {
		t.Fatal("a refused run injected its fault or left the lab behind")
	}
	if card := BuildScorecard("v1", []RunReport{report}); len(card.Invalid) != 1 || len(card.Columns) != 0 {
		t.Fatalf("a refused run must show as ungraded: %+v", card)
	}
	if _, err := NewRunner(RunnerConfig{Manifest: fastResponseManifest(t, "response.web-worker-died"), Lab: NewDockerLab(newFakeDocker(), DockerTarget{Context: "lab"}),
		Client: &PulseClient{}, AuthorizeRemediation: true, AgentProfile: "commands-maybe"}); err == nil {
		t.Fatal("an unknown agent profile name was accepted")
	}
	if _, err := NewRunner(RunnerConfig{Manifest: fastResponseManifest(t, "response.web-worker-died"), Lab: NewDockerLab(newFakeDocker(), DockerTarget{Context: "lab"}),
		Client: &PulseClient{}}); err == nil {
		t.Fatal("a response scenario with a fix ran without the remediation authorization")
	}
}

// Pulse lists a finding while the run that filed it is still going and writes
// the run record when that run ends. The trigger is read from that record, so
// the harness has to wait for it. Five live replays printed an empty trigger
// for every run that woke by itself, because the harness looked once, early.
func TestResponseRunReadsTheTriggerOnceTheRunIsRecorded(t *testing.T) {
	pulse := &fakePulse{
		commandsEnabled: true, wakesByItself: true, raisesOn: "paperless", trigger: "alert_flapping", recordsRunAfter: 300 * time.Millisecond,
		conclusion: "The gunicorn webserver exited and was not respawned.",
	}
	report, dir, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	wake := report.Response.Evidence.Wake
	if asked, _ := pulse.counts(); wake.Mode != WakeSelf || wake.TriggerReason != "alert_flapping" || wake.RunID != "run-alert" || asked != 0 {
		t.Fatalf("wake = %+v, harness asked %d times", wake, asked)
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "| How the run started | Patrol woke by itself, trigger `alert_flapping` |") {
		t.Fatalf("report does not name the trigger:\n%s", markdown)
	}
	card := RenderScorecardMarkdown(BuildScorecard("v1", []RunReport{report}))
	if !strings.Contains(card, "woke by itself (alert_flapping)") {
		t.Fatalf("scorecard does not name the trigger:\n%s", card)
	}
}

// When Pulse never lists a run for the finding, the report says so. It does
// not print an empty trigger as if it were one.
func TestResponseRunWithNoRunRecordSaysSo(t *testing.T) {
	pulse := &fakePulse{
		commandsEnabled: true, wakesByItself: true, raisesOn: "paperless", recordsRunAfter: 50 * time.Millisecond, neverRecordsRun: true,
		conclusion: "The gunicorn webserver exited and was not respawned.",
	}
	report, dir, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err != nil {
		t.Fatalf("run: %v (errors %v)", err, report.Errors)
	}
	if wake := report.Response.Evidence.Wake; wake.Mode != WakeSelf || wake.TriggerReason != "" || wake.RunID != "" {
		t.Fatalf("wake = %+v", wake)
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(markdown); !strings.Contains(text, "Patrol woke by itself, Pulse recorded no run for the finding") || strings.Contains(text, "trigger ``") {
		t.Fatalf("report prints an empty trigger:\n%s", text)
	}
}

// A run on Pulse data that already saw the same fault is not independent: the
// earlier container is gone and its finding is still open under the name. The
// harness refuses before it injects anything.
func TestResponseRunRefusesAnOpenFindingOnANamesake(t *testing.T) {
	old := func(id, name string) map[string]any {
		return map[string]any{"id": id, "severity": "critical", "category": "reliability", "title": "container is failing",
			"resource_id": "app-container-removed-" + id, "resource_name": name, "resource_type": "app-container"}
	}
	pulse := &fakePulse{commandsEnabled: true, wakesByItself: true, raisesOn: "paperless", leftover: []map[string]any{old("finding-old", "Paperless ")}}
	report, _, err := runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err == nil || !strings.Contains(err.Error(), `open finding finding-old on another resource named "Paperless"`) || !strings.Contains(err.Error(), "fresh Pulse data directory") {
		t.Fatalf("a run on top of an earlier run's open finding continued: %v", err)
	}
	if report.Response != nil || report.Passed {
		t.Fatalf("a refused run produced a grade: %+v", report.Response)
	}
	if pulse.docker.container("paperless") != nil || pulse.docker.called("exec") != 0 {
		t.Fatal("a refused run injected its fault or left the lab behind")
	}

	// What does not make a run depend on an earlier one: a namesake finding
	// that is closed, and an open one on a resource this scenario is not
	// graded on.
	closed := old("finding-closed", "paperless")
	closed["resolved_at"] = time.Now().UTC()
	pulse = &fakePulse{
		commandsEnabled: true, wakesByItself: true, raisesOn: "paperless", leftover: []map[string]any{closed, old("finding-elsewhere", "vaultwarden")},
		conclusion: "The gunicorn webserver exited and was not respawned.",
	}
	report, _, err = runFakeResponse(t, fastResponseManifest(t, "response.web-worker-died"), pulse, AgentProfileCommandsOn)
	if err != nil || report.Response == nil || len(report.Response.Evidence.Findings) != 1 || report.Response.Evidence.Findings[0].ID != "finding-1" {
		t.Fatalf("run: %v, response %+v, errors %v", err, report.Response, report.Errors)
	}
}

// The harness asks in the order a person would click: Run Patrol on the
// affected resources, then Investigate on the firing alert when Pulse cannot
// resolve that selection, and a full run last.
func TestAskForRunFollowsWhatAPersonWouldClick(t *testing.T) {
	alert := &PulseAlert{ID: "agent:host/disk:mnt-nvr::metric-threshold:disk", Type: "disk", ResourceID: "agent:host/disk:mnt-nvr"}
	unresolved := `{"code":"patrol_scope_unresolved"}`
	busy := `{"code":"patrol_already_running"}`
	type reply struct {
		status int
		body   string
	}
	for name, tc := range map[string]struct {
		alert      *PulseAlert
		replies    []reply
		wantAsked  string
		wantBodies []string
		wantErr    bool
	}{
		"the selection resolves": {alert, []reply{{200, `{"run_id":"r1"}`}}, AskScopedRun,
			[]string{`{"resource_ids":["agent-1"]}`}, false},
		"busy, then the selection resolves": {alert, []reply{{409, busy}, {200, `{"run_id":"r1"}`}}, AskScopedRun,
			[]string{`{"resource_ids":["agent-1"]}`, `{"resource_ids":["agent-1"]}`}, false},
		"unresolved with a firing alert": {alert, []reply{{422, unresolved}, {200, `{"run_id":"r1"}`}}, AskAlertInvestigate,
			[]string{`{"resource_ids":["agent-1"]}`, `{"resource_ids":["agent:host/disk:mnt-nvr"],"alert_identifier":"agent:host/disk:mnt-nvr::metric-threshold:disk","alert_type":"disk"}`}, false},
		"unresolved with no alert": {nil, []reply{{422, unresolved}, {200, `{"run_id":"r1"}`}}, AskFullRun,
			[]string{`{"resource_ids":["agent-1"]}`, ``}, false},
		"the alert does not resolve either": {alert, []reply{{422, unresolved}, {422, unresolved}, {200, `{"run_id":"r1"}`}}, AskFullRun,
			[]string{`{"resource_ids":["agent-1"]}`, `{"resource_ids":["agent:host/disk:mnt-nvr"],"alert_identifier":"agent:host/disk:mnt-nvr::metric-threshold:disk","alert_type":"disk"}`, ``}, false},
		"Patrol is not ready": {alert, []reply{{409, `{"code":"patrol_readiness_not_ready"}`}}, "",
			[]string{`{"resource_ids":["agent-1"]}`}, true},
	} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			var bodies []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload, _ := io.ReadAll(r.Body)
				mu.Lock()
				next := tc.replies[len(bodies)]
				bodies = append(bodies, strings.TrimSpace(string(payload)))
				mu.Unlock()
				w.WriteHeader(next.status)
				_, _ = w.Write([]byte(next.body))
			}))
			defer server.Close()
			client, err := NewPulseClient(ClientConfig{BaseURL: server.URL, Username: "admin", Password: "test"})
			if err != nil {
				t.Fatal(err)
			}
			runner := &QualificationRunner{config: RunnerConfig{Client: client}, askRetry: time.Millisecond}
			var wake WakeObservation
			accepted, err := runner.askForRun(context.Background(), []string{"agent-1"}, tc.alert, time.Second, &wake)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, want error %t", err, tc.wantErr)
			}
			if wake.Asked != tc.wantAsked || (!tc.wantErr && accepted.RunID != "r1") {
				t.Fatalf("asked %q run %q, want %q", wake.Asked, accepted.RunID, tc.wantAsked)
			}
			mu.Lock()
			got := strings.Join(bodies, "\n")
			mu.Unlock()
			if got != strings.Join(tc.wantBodies, "\n") {
				t.Fatalf("requests:\n%s\nwant:\n%s", got, strings.Join(tc.wantBodies, "\n"))
			}
			if (wake.Asked == AskScopedRun) != (wake.AskDetail == "") && !tc.wantErr {
				t.Fatalf("a fallback must say why: %+v", wake)
			}
		})
	}
}

// When Patrol raises a finding and never starts on it, the harness asks once,
// the way the Investigate button on the finding does, and says that it did.
func TestSettleInvestigationsAsksOnceWhenNoneStarts(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/ai/findings/finding-1/reinvestigate" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, err := NewPulseClient(ClientConfig{BaseURL: server.URL, Username: "admin", Password: "test"})
	if err != nil {
		t.Fatal(err)
	}
	runner := &QualificationRunner{config: RunnerConfig{Client: client}}
	detected := time.Now().UTC()
	read := func() ([]Finding, []Finding, error) {
		status := ""
		if requests.Load() > 0 {
			status = "completed"
		}
		return []Finding{
			{ID: "finding-1", Severity: "warning", DetectedAt: detected, InvestigationStatus: status},
			{ID: "finding-2", Severity: "info", DetectedAt: detected.Add(time.Second)},
		}, nil, nil
	}
	var wake WakeObservation
	settled, err := runner.settleInvestigations(context.Background(), read, 20*time.Millisecond, 5*time.Second, time.Millisecond, &wake)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || wake.InvestigationStart != WakeAsked {
		t.Fatalf("asked %d times, investigation start %q", requests.Load(), wake.InvestigationStart)
	}
	if len(settled) != 2 || settled[0].ID != "finding-1" || settled[0].InvestigationStatus != "completed" {
		t.Fatalf("settled = %+v", settled)
	}

	// A finding below warning is never investigated, so nothing is waited
	// for and nothing is asked.
	requests.Store(0)
	wake = WakeObservation{}
	onlyInfo := func() ([]Finding, []Finding, error) {
		return []Finding{{ID: "finding-2", Severity: "info", DetectedAt: detected}}, nil, nil
	}
	if settled, err = runner.settleInvestigations(context.Background(), onlyInfo, time.Millisecond, time.Second, time.Millisecond, &wake); err != nil || len(settled) != 1 {
		t.Fatalf("settled = %+v, %v", settled, err)
	}
	if requests.Load() != 0 || wake.InvestigationStart != "" {
		t.Fatalf("asked %d times for an uninvestigable finding, start %q", requests.Load(), wake.InvestigationStart)
	}

	// A finding that never settles is returned as it stands when the wait
	// is over, and is then graded as having no conclusion.
	stuck := func() ([]Finding, []Finding, error) {
		return []Finding{{ID: "finding-1", Severity: "critical", DetectedAt: detected, InvestigationStatus: "running"}}, nil, nil
	}
	wake = WakeObservation{}
	if settled, err = runner.settleInvestigations(context.Background(), stuck, time.Second, 30*time.Millisecond, time.Millisecond, &wake); err != nil || len(settled) != 1 || settled[0].InvestigationStatus != "running" {
		t.Fatalf("settled = %+v, %v", settled, err)
	}
	if wake.InvestigationStart != WakeSelf {
		t.Fatalf("an investigation that started by itself was recorded as %q", wake.InvestigationStart)
	}
}
