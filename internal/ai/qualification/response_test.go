package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func loadFaultSetCatalog(t *testing.T) Catalog {
	t.Helper()
	catalog, err := LoadCatalog(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestFaultSetV1HoldsSevenLabelledScenarios(t *testing.T) {
	manifests := FaultSetManifests(loadFaultSetCatalog(t), "v1")
	var labels, expects []string
	for _, manifest := range manifests {
		labels = append(labels, manifest.Metadata[metadataFaultLabel])
		expects = append(expects, manifest.Outcome.Expect)
		if manifest.Patrol.Mode != "approval" {
			t.Errorf("%s runs in %q: fault set v1 is scored in Ask first mode", manifest.ID, manifest.Patrol.Mode)
		}
		if manifest.Lab.SharedHostOK {
			t.Errorf("%s may run on a shared host, and its fixtures shadow real image names", manifest.ID)
		}
	}
	if got, want := strings.Join(labels, " "), "D1 D2 D3 D4 D5 D6 D7"; got != want {
		t.Fatalf("fault set v1 labels = %q, want %q", got, want)
	}
	if got, want := strings.Join(expects, " "), "fixed fixed quiet handed_over fixed handed_over fixed"; got != want {
		t.Fatalf("fault set v1 expectations = %q, want %q", got, want)
	}
}

// phase1Record is one fault set v1 run from 9 November 2026, reduced to what
// the grader reads. phase1_grade is the grade a person gave it then.
type phase1Record struct {
	Source          string           `json:"source"`
	Model           string           `json:"model"`
	Scenario        string           `json:"scenario"`
	ScorecardColumn string           `json:"phase1_scorecard_column"`
	Phase1Grade     Grade            `json:"phase1_grade"`
	WokeByItself    string           `json:"phase1_woke_by_itself"`
	CapturedAt      time.Time        `json:"captured_at"`
	Evidence        ResponseEvidence `json:"evidence"`
}

func loadPhase1Records(t *testing.T) map[string]phase1Record {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "fault-set-v1", "phase1", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no Phase 1 evidence found: %v", err)
	}
	records := make(map[string]phase1Record, len(paths))
	for _, path := range paths {
		payload, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var record phase1Record
		if err := json.Unmarshal(payload, &record); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		records[strings.TrimSuffix(filepath.Base(path), ".json")] = record
	}
	return records
}

// The grader must give the stored Phase 1 runs the grades a person gave them,
// from the scenario rubric alone. This is the proof that no grade depends on
// someone reading the conclusion.
func TestPhase1TranscriptsReproduceTheScorecard(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	records := loadPhase1Records(t)
	resolved := map[string]int{}
	cells := map[string]int{}
	wakeWant := map[string]string{"yes": WakeSelf, "no": WakeAsked, "n/a": WakeNotApplicable}
	for name, record := range records {
		manifest, ok := catalog.ByID[record.Scenario]
		if !ok {
			t.Fatalf("%s names unknown scenario %s", name, record.Scenario)
		}
		graded := GradeResponse(manifest, record.Evidence)
		if record.ScorecardColumn == "" {
			continue
		}
		if graded.Grade != record.Phase1Grade {
			t.Errorf("%s (%s): graded %s (%s), the Phase 1 scorecard says %s", name, record.Source, graded.Grade, graded.Reason, record.Phase1Grade)
		}
		if got, want := record.Evidence.Wake.Mode, wakeWant[record.WokeByItself]; got != want {
			t.Errorf("%s: wake %q, the Phase 1 scorecard says woke by itself %q", name, got, record.WokeByItself)
		}
		if graded.Evidence.AgentProfile.Name != record.ScorecardColumn {
			t.Errorf("%s: agent profile %q in column %q", name, graded.Evidence.AgentProfile.Name, record.ScorecardColumn)
		}
		cells[record.ScorecardColumn]++
		if graded.Resolved {
			resolved[record.ScorecardColumn]++
		}
	}
	for column, want := range map[string]int{AgentProfileCommandsOn: 5, AgentProfileCommandsOff: 1} {
		if cells[column] != 6 {
			t.Errorf("%s column holds %d cells, want 6", column, cells[column])
		}
		if resolved[column] != want {
			t.Errorf("%s resolved %d of 6, the Phase 1 scorecard says %d of 6", column, resolved[column], want)
		}
	}
}

// Runs that were not scorecard cells are still pinned, so a rubric change
// that moves one of them is a visible decision.
func TestPhase1RunsOutsideTheScorecardKeepTheirGrades(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	records := loadPhase1Records(t)
	for name, want := range map[string]Grade{
		// Named the stopped database and that it needs starting. Commands
		// were on, so the fix was within reach and was not carried out.
		"sonnet-r1-d5": GradePartial,
		// Restarted the app instead of starting the database. Pulse reported
		// the restart as not verified and the lab saw no recovery.
		"sonnet-r1c-d5": GradePartial,
		// Asked with a full run, which left the alert-covered disk alone.
		"sonnet-r1c-d6": GradeNotLookedAt,
	} {
		record, ok := records[name]
		if !ok {
			t.Fatalf("missing Phase 1 evidence %s", name)
		}
		graded := GradeResponse(catalog.ByID[record.Scenario], record.Evidence)
		if graded.Grade != want {
			t.Errorf("%s: graded %s (%s), want %s", name, graded.Grade, graded.Reason, want)
		}
	}
	wrongFix := GradeResponse(catalog.ByID[records["sonnet-r1c-d5"].Scenario], records["sonnet-r1c-d5"].Evidence)
	if wrongFix.Checks.ActionOnTarget || wrongFix.Checks.PostconditionsMet {
		t.Errorf("a restart of the wrong container counted toward the fix: %+v", wrongFix.Checks)
	}
}

func TestPhase1ScorecardTable(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	var reports []RunReport
	for name, record := range loadPhase1Records(t) {
		if record.ScorecardColumn == "" {
			continue
		}
		graded := GradeResponse(catalog.ByID[record.Scenario], record.Evidence)
		reports = append(reports, RunReport{
			SchemaVersion: ReportSchemaVersion, RunID: name, GeneratedAt: record.CapturedAt,
			Manifest: catalog.ByID[record.Scenario], Environment: Environment{Model: record.Model}, Response: &graded,
		})
	}
	card := BuildScorecard("v1", reports)
	if len(card.Columns) != 2 || card.Columns[0].AgentProfile != AgentProfileCommandsOn || card.Columns[1].AgentProfile != AgentProfileCommandsOff {
		t.Fatalf("columns = %+v, want commands-on then commands-off", card.Columns)
	}
	if card.Columns[0].Resolved != 5 || card.Columns[0].Runs != 6 || card.Columns[1].Resolved != 1 || card.Columns[1].Runs != 6 {
		t.Fatalf("column totals = %+v, want 5 of 6 and 1 of 6", card.Columns)
	}
	var labels []string
	for _, row := range card.Rows {
		labels = append(labels, row.FaultLabel)
	}
	if got := strings.Join(labels, " "); got != "D1 D2 D3 D4 D5 D6" {
		t.Fatalf("rows = %q", got)
	}
	markdown := RenderScorecardMarkdown(card)
	for _, want := range []string{
		"| claude-subscription:sonnet, commands on | claude-subscription:sonnet, default agent (commands off) |",
		"| D1 web worker died, container up, health failing | fix it | **Fixed**, woke by itself (alert_fired) | **Handed back**, woke by itself (alert_fired) |",
		"| D2 worker segfaulted once, stays exited | fix it | **Fixed**, had to be asked | **Handed back**, had to be asked |",
		"| D3 finished one-shot job, Exited (0) | nothing | **Quiet** | **Quiet** |",
		"| D4 restart loop from a bad DATABASE_URL | say exactly what is wrong | **Handed over**, woke by itself (alert_fired) | **Handed back**, woke by itself (alert_fired) |",
		"| D5 app unhealthy because its database stopped | fix it | **Fixed**, woke by itself (alert_fired) | **Partial**, woke by itself (alert_fired) |",
		"| D6 host disk at 92% from one folder | say exactly what is wrong | **Partial**, had to be asked | **Not looked at**, had to be asked |",
		"| **Resolved or correctly handed over** | | **5 of 6** | **1 of 6** |",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("scorecard lacks line:\n%s\n\ngot:\n%s", want, markdown)
		}
	}
}

func TestScorecardListsUngradedRunsApartFromGrades(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	manifest := catalog.ByID["response.web-worker-died"]
	card := BuildScorecard("v1", []RunReport{{RunID: "q-broken", Manifest: manifest, Errors: []string{"baseline did not converge"}}})
	if len(card.Columns) != 0 || len(card.Invalid) != 1 || !strings.Contains(card.Invalid[0], "baseline did not converge") {
		t.Fatalf("a run with no grade must be listed as invalid and stay out of the table: %+v", card)
	}
	if markdown := RenderScorecardMarkdown(card); !strings.Contains(markdown, "not a verdict on Patrol") {
		t.Fatalf("invalid runs are not explained:\n%s", markdown)
	}
}

func TestConclusionTermMatching(t *testing.T) {
	text := newConclusionText("An admin must approve the pending \"start\" action on `nextcloud-db`.\nThe Docker/Podman command-agent is not connected. DATABASE_URL=10.0.0.5 lacks postgresql://; see /mnt/nvr (frigate/ folder). It restarted twice.")
	for term, want := range map[string]bool{
		"start":                  true,
		"restart":                false, // not inside "restarted"
		"restarted":              true,
		"nextcloud-db":           true,
		"nextcloud":              true, // a plain term ignores punctuation, so it also matches the first word of nextcloud-db
		"agent is not connected": true, // punctuation in the text is ignored for plain terms
		"command agent":          true,
		"DATABASE_URL":           true,
		"database":               false,
		"postgresql://":          true,
		"/mnt/nvr":               true,
		"/mnt/nv":                false,
		"frigate":                true,
		"frigate/recordings":     false,
		"":                       false,
	} {
		if got := text.has(term); got != want {
			t.Errorf("has(%q) = %t, want %t", term, got, want)
		}
	}
	missing := text.missingGroups([][]string{{"stopped", "start"}, {"gunicorn", "webserver"}})
	if len(missing) != 1 || missing[0][0] != "gunicorn" {
		t.Fatalf("missing groups = %v", missing)
	}
}

func responseTestFinding(alias, conclusion string) ResponseFinding {
	return ResponseFinding{ID: "f1", Severity: "critical", ResourceAlias: alias, InvestigationStatus: "completed", Conclusion: conclusion}
}

func TestGradeResponseLadder(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	database := catalog.ByID["response.app-unhealthy-database-stopped"]
	config := catalog.ByID["response.restart-loop-bad-config"]
	quiet := catalog.ByID["response.finished-job-stays-quiet"]
	on := AgentProfile{Name: AgentProfileCommandsOn, CommandsEnabled: true}
	off := AgentProfile{Name: AgentProfileCommandsOff}
	passed := []PredicateObservation{{Passed: true}, {Passed: true}}
	fix := func(target, capability, verification string) *ResponseAction {
		return &ResponseAction{Capability: capability, TargetAlias: target, Decision: ActionDecisionApprovedExecuted, FinalState: "completed", VerificationStatus: verification}
	}
	withAction := func(finding ResponseFinding, action *ResponseAction) ResponseFinding {
		finding.Action = action
		return finding
	}
	cause := "nextcloud-db is exited. Start nextcloud-db."

	cases := []struct {
		name     string
		manifest Manifest
		evidence ResponseEvidence
		want     Grade
	}{
		{"verified fix seen by the lab", database, ResponseEvidence{AgentProfile: on, Postconditions: passed,
			Findings: []ResponseFinding{withAction(responseTestFinding("nextcloud", cause), fix("nextcloud-db", "start", "verified"))}}, GradeFixed},
		{"Pulse says verified and the lab disagrees", database, ResponseEvidence{AgentProfile: on, Postconditions: []PredicateObservation{{Passed: true}, {Passed: false}},
			Findings: []ResponseFinding{withAction(responseTestFinding("nextcloud", cause), fix("nextcloud-db", "start", "verified"))}}, GradePartial},
		{"the lab recovered and Pulse did not verify", database, ResponseEvidence{AgentProfile: on, Postconditions: passed,
			Findings: []ResponseFinding{withAction(responseTestFinding("nextcloud", cause), fix("nextcloud-db", "start", "unknown"))}}, GradePartial},
		{"an action on the wrong resource is not the fix", database, ResponseEvidence{AgentProfile: on, Postconditions: passed,
			Findings: []ResponseFinding{withAction(responseTestFinding("nextcloud", "Restarting nextcloud."), fix("nextcloud", "restart", "verified"))}}, GradeHandedBack},
		{"a proposal nobody executed is not a fix", database, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{withAction(responseTestFinding("nextcloud", cause), &ResponseAction{Capability: "start", TargetAlias: "nextcloud-db", Decision: ActionDecisionNotApproved, FinalState: "pending_approval"})}}, GradePartial},
		{"right cause, commands off, nothing false said", database, ResponseEvidence{AgentProfile: off,
			Findings: []ResponseFinding{responseTestFinding("nextcloud", cause)}}, GradeHandedOver},
		{"right cause and a false statement", database, ResponseEvidence{AgentProfile: off,
			Findings: []ResponseFinding{responseTestFinding("nextcloud", cause+" Restore the command-agent connection first.")}}, GradePartial},
		{"only the alert restated", database, ResponseEvidence{AgentProfile: off,
			Findings: []ResponseFinding{responseTestFinding("nextcloud", "nextcloud is unhealthy. Check its logs.")}}, GradeHandedBack},
		{"finding with no stored conclusion", database, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{{ID: "f1", Severity: "critical", ResourceAlias: "nextcloud", InvestigationStatus: "running", Conclusion: cause}}}, GradeHandedBack},
		{"finding only on an untouched resource", database, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{responseTestFinding("paperless", cause)}}, GradeNotLookedAt},
		{"no finding", database, ResponseEvidence{AgentProfile: on}, GradeNotLookedAt},
		{"no action exists and the exact fact is stated", config, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{responseTestFinding("vaultwarden", "DATABASE_URL must start with postgresql://")}}, GradeHandedOver},
		{"the variable without its correct form", config, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{responseTestFinding("vaultwarden", "DATABASE_URL looks wrong.")}}, GradePartial},
		{"false statement and nothing found", config, ResponseEvidence{AgentProfile: off,
			Findings: []ResponseFinding{responseTestFinding("vaultwarden", "The agent is disconnected.")}}, GradeHandedBack},
		{"healthy lab, nothing raised", quiet, ResponseEvidence{AgentProfile: on}, GradeQuiet},
		{"healthy lab, finding on the finished job", quiet, ResponseEvidence{AgentProfile: on,
			Findings: []ResponseFinding{{ID: "f1", Severity: "warning", ResourceAlias: "vault-migrate"}}}, GradeFalseAlarm},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graded := GradeResponse(tc.manifest, tc.evidence)
			if graded.Grade != tc.want {
				t.Fatalf("grade = %s (%s, checks %+v), want %s", graded.Grade, graded.Reason, graded.Checks, tc.want)
			}
			if graded.Label != tc.want.Label() || graded.Resolved != tc.want.Resolved() || graded.Reason == "" {
				t.Fatalf("grade fields disagree: %+v", graded)
			}
		})
	}
}

func TestOnlyThreeGradesCountAsResolved(t *testing.T) {
	var resolved []string
	for _, grade := range []Grade{GradeFixed, GradeHandedOver, GradePartial, GradeHandedBack, GradeQuiet, GradeNotLookedAt, GradeFalseAlarm, ""} {
		if grade.Resolved() {
			resolved = append(resolved, string(grade))
		}
		if grade.Label() == "" {
			t.Errorf("grade %q has no label", grade)
		}
	}
	sort.Strings(resolved)
	if got := strings.Join(resolved, " "); got != "fixed handed_over quiet" {
		t.Fatalf("resolved grades = %q", got)
	}
}

// A stored report has to grade the same again. Replay must use the evidence,
// never the grade written beside it.
func TestReplayGradesStoredResponseEvidenceAgain(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	record := loadPhase1Records(t)["sonnet-r1-d4"]
	graded := GradeResponse(catalog.ByID[record.Scenario], record.Evidence)
	report := RunReport{
		SchemaVersion: ReportSchemaVersion, RunID: "q-replay", Manifest: catalog.ByID[record.Scenario],
		Response: &ResponseResult{Grade: GradeFixed, Label: "Fixed", Resolved: true, Evidence: record.Evidence},
		Teardown: CleanupResult{Passed: true},
	}
	replayed := ReplayScore(report)
	if replayed.Response.Grade != graded.Grade || replayed.Response.Grade != GradeHandedOver {
		t.Fatalf("replay kept the stored grade %s, evidence grades %s", replayed.Response.Grade, graded.Grade)
	}
	if !replayed.Passed {
		t.Fatal("a graded response run with a clean teardown is a valid run")
	}
	markdown := renderMarkdown(replayed)
	for _, want := range []string{"VALID RUN", "**Handed over**", "commands on", "Patrol woke by itself, trigger `alert_fired`"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("report lacks %q:\n%s", want, markdown)
		}
	}
	report.Response = nil
	if ReplayScore(report).Passed {
		t.Fatal("a response report with no grade must not be a valid run")
	}
}

// A valid response run is not a pass. It must stay out of model comparison,
// where it would count toward a pass rate and a launch gate.
func TestComparisonIgnoresResponseRuns(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	record := loadPhase1Records(t)["sonnet-r1-d4"]
	graded := GradeResponse(catalog.ByID[record.Scenario], record.Evidence)
	dir := t.TempDir()
	report := RunReport{
		SchemaVersion: ReportSchemaVersion, RunID: "q-20261110-0badc0de", GeneratedAt: record.CapturedAt, Manifest: catalog.ByID[record.Scenario],
		Environment: Environment{Model: record.Model}, Response: &graded, Teardown: CleanupResult{Passed: true}, Passed: true,
	}
	if err := WriteReport(dir, report); err != nil {
		t.Fatal(err)
	}
	comparison, err := CompareReports([]string{filepath.Join(dir, "report.json")})
	if err != nil {
		t.Fatal(err)
	}
	if len(comparison.Models) != 0 || len(comparison.Scenarios) != 0 {
		t.Fatalf("a response run entered model comparison: %+v", comparison)
	}
	if err := ApplyQualificationGates(&comparison, catalog, TrackResponse); err == nil {
		t.Fatal("the response track was accepted as a launch gate")
	}
	if _, err := BuildContributionBundle([]string{filepath.Join(dir, "report.json")}, catalog, TrackResponse); err == nil {
		t.Fatal("the response track was accepted for community evidence export")
	}
	markdown, err := os.ReadFile(filepath.Join(dir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), "VALID RUN") || strings.Contains(string(markdown), "Verdict: **PASS**") {
		t.Fatalf("a response report reads as a pass:\n%s", markdown)
	}
}

// The next step for a crashed worker is to start it. "No restart policy" is
// something Patrol observed, not that step, so it must not complete a
// hand-over by itself.
func TestCrashedWorkerRubricNeedsTheStartStep(t *testing.T) {
	manifest := loadFaultSetCatalog(t).ByID["response.worker-crashed-stays-exited"]
	off := AgentProfile{Name: AgentProfileCommandsOff}
	observed := "immich_machine_learning crashed with SIGSEGV. It has no restart policy, so it stays exited."
	grade := func(conclusion string) ResponseResult {
		return GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{responseTestFinding("immich-ml", conclusion)}})
	}
	if graded := grade(observed); graded.Grade != GradeHandedBack || len(graded.Checks.DecisiveMissing) != 1 {
		t.Fatalf("a conclusion with no next step graded %s, missing %v", graded.Grade, graded.Checks.DecisiveMissing)
	}
	for _, step := range []string{" Start the container.", " It has to be started by hand.", " Restart it."} {
		if graded := grade(observed + step); graded.Grade != GradeHandedOver {
			t.Fatalf("%q graded %s (%s)", step, graded.Grade, graded.Reason)
		}
	}
}

// The finished job is the control for a container Pulse itself leaves alone.
// The container state alert skips a short-lived container that exited 0 under
// a restart policy of "no" or "on-failure" (dockerContainerFinishedCleanly in
// internal/alerts). Provisioned under "always" or "unless-stopped" the job
// would raise that alert on every run, and a quiet grade would measure Patrol
// against an alert the lab was built to raise.
func TestFinishedJobScenarioProvisionsAJobPulseLeavesAlone(t *testing.T) {
	manifest, ok := loadFaultSetCatalog(t).ByID["response.finished-job-stays-quiet"]
	if !ok {
		t.Fatal("catalogue has no finished-job scenario")
	}
	if len(manifest.Faults) != 0 {
		t.Fatalf("the finished-job scenario injects %d faults, want none", len(manifest.Faults))
	}
	const job = "vault-migrate"
	found := false
	for _, resource := range manifest.Resources {
		if resource.Alias != job {
			continue
		}
		found = true
		if resource.Restart != "no" && resource.Restart != "on-failure" {
			t.Fatalf("finished job restart policy = %q: Docker would restart a clean exit, so Pulse would alert on it", resource.Restart)
		}
	}
	if !found {
		t.Fatalf("the finished-job scenario has no resource %q", job)
	}
	exitedCleanly := false
	for _, predicate := range manifest.Baseline {
		if predicate.Probe == "docker.exit_code" && predicate.Target == job && predicate.Operator == "eq" && string(predicate.Value) == "0" {
			exitedCleanly = true
		}
	}
	if !exitedCleanly {
		t.Fatal("the baseline does not require the finished job to have exited with code 0")
	}
}

// By default only a critical alert wakes Patrol when it fires. Pulse has
// Patrol look at a warning once it has stayed active for 10 minutes, checked
// every 30 seconds (sustainedAlertDelay and sustainedCheckInterval in
// internal/ai/unified). A scenario whose alert is a warning has to wait longer
// than that before the harness asks. Otherwise it is always graded "had to be
// asked", on a manual run.
func TestWarningScenariosOutwaitTheSustainedAlertWake(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	const sustainedWake = 10*time.Minute + 30*time.Second
	for _, id := range []string{"response.worker-crashed-stays-exited", "response.finished-job-stays-quiet", "response.host-disk-one-folder"} {
		manifest, ok := catalog.ByID[id]
		if !ok || manifest.Engagement == nil {
			t.Fatalf("catalogue has no response scenario %s", id)
		}
		wait, err := positiveDuration(manifest.Engagement.NaturalWakeTimeout)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		// The run that wakes still has to file its finding.
		if wait < sustainedWake+2*time.Minute {
			t.Errorf("%s waits %s for Patrol to wake by itself, and a warning wakes it after up to %s", id, wait, sustainedWake)
		}
		if manifest.Version < 2 {
			t.Errorf("%s changed its wait without a new version", id)
		}
	}
}

// The stopped service is the other half of the finished job. Pulse's alert
// rule cannot tell them apart (both exited 0 soon after starting, with no
// restart policy), so it alerts on neither and Patrol has to judge. The
// scenario only measures that when the lab really produces the case the rule
// skips, and when the run that judges is the one the schedule makes.
func TestStoppedServiceScenarioProvisionsAStopNoAlertCovers(t *testing.T) {
	manifest, ok := loadFaultSetCatalog(t).ByID["response.service-stopped-no-alert"]
	if !ok {
		t.Fatal("catalogue has no stopped-service scenario")
	}
	const service, job = "gitea", "vault-migrate"
	for _, alias := range []string{service, job} {
		resource, ok := manifest.resource(alias)
		if !ok {
			t.Fatalf("the scenario has no resource %q", alias)
		}
		if resource.Restart != "no" && resource.Restart != "on-failure" {
			t.Fatalf("%s restart policy = %q: Docker would restart a clean exit, so Pulse would alert on it", alias, resource.Restart)
		}
	}
	if len(manifest.Faults) != 1 || manifest.Faults[0].Injector.Kind != "stop" || manifest.Faults[0].Injector.Resource != service {
		t.Fatalf("faults = %+v, want one stop of %s", manifest.Faults, service)
	}
	exitedCleanly := false
	for _, predicate := range manifest.Faults[0].Oracle {
		if predicate.Probe == "docker.exit_code" && predicate.Target == service && predicate.Operator == "eq" && string(predicate.Value) == "0" {
			exitedCleanly = true
		}
	}
	if !exitedCleanly {
		t.Fatal("the fault oracle does not require the service to have exited with code 0, and any other exit raises an alert")
	}
	fixture, ok := fixtureByKey("gitea")
	if !ok || !strings.Contains(fixture.Entrypoint, "trap ") || !strings.Contains(fixture.Entrypoint, "exit 0") {
		t.Fatalf("the gitea fixture has to end with exit code 0 on SIGTERM: %q", fixture.Entrypoint)
	}
	if manifest.Engagement.Fallback != EngagementFallbackFullRun {
		t.Fatalf("fallback = %q: a scoped run reads the selected container and never follows the no-alert flag", manifest.Engagement.Fallback)
	}
	if got := strings.Join(manifest.Outcome.QuietResources, " "); got != job {
		t.Fatalf("quiet resources = %q, want the finished job", got)
	}
}

// Reporting the stopped service is only half of the scenario. A run that also
// reports the finished job has not told them apart.
func TestStoppedServiceRubricFailsWhenTheFinishedJobIsReported(t *testing.T) {
	manifest := loadFaultSetCatalog(t).ByID["response.service-stopped-no-alert"]
	off := AgentProfile{Name: AgentProfileCommandsOff}
	on := AgentProfile{Name: AgentProfileCommandsOn, CommandsEnabled: true}
	conclusion := "gitea exited with exit code 0 and has no restart policy, so it stays down. Pulse raised no alert. Start the container."
	service := responseTestFinding("gitea", conclusion)
	job := ResponseFinding{ID: "f2", Severity: "warning", ResourceAlias: "vault-migrate"}
	bystander := ResponseFinding{ID: "f3", Severity: "warning", ResourceAlias: "paperless"}

	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{service}}); graded.Grade != GradeHandedOver {
		t.Fatalf("service reported, job quiet: %s (%s)", graded.Grade, graded.Reason)
	}
	fixed := service
	fixed.Action = &ResponseAction{Capability: "start", TargetAlias: "gitea", Decision: ActionDecisionApprovedExecuted, FinalState: "completed", VerificationStatus: "verified"}
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: on, Postconditions: []PredicateObservation{{Passed: true}}, Findings: []ResponseFinding{fixed}}); graded.Grade != GradeFixed {
		t.Fatalf("service started, job quiet: %s (%s)", graded.Grade, graded.Reason)
	}
	graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: on, Postconditions: []PredicateObservation{{Passed: true}}, Findings: []ResponseFinding{fixed}, OtherFindings: []ResponseFinding{job}})
	if graded.Grade != GradeFalseAlarm || graded.Resolved || strings.Join(graded.Checks.QuietReported, " ") != "vault-migrate" {
		t.Fatalf("job reported as well: %s (%s, %+v)", graded.Grade, graded.Reason, graded.Checks)
	}
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, OtherFindings: []ResponseFinding{job}}); graded.Grade != GradeFalseAlarm {
		t.Fatalf("only the job reported: %s (%s)", graded.Grade, graded.Reason)
	}
	// A finding on a negative control the scenario does not name is recorded
	// and not graded, as in every other scenario.
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{service}, OtherFindings: []ResponseFinding{bystander}}); graded.Grade != GradeHandedOver {
		t.Fatalf("an unnamed negative control changed the grade: %s (%s)", graded.Grade, graded.Reason)
	}
	// The step a default agent wrote on the first live run. A rubric copied
	// from the crashed worker missed it, because it named the service.
	named := responseTestFinding("gitea", "gitea exited cleanly and Pulse raised no alert. If unintentional, restart the gitea container after reviewing the logs.")
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{named}}); graded.Grade != GradeHandedOver {
		t.Fatalf("a hand-over that names the service graded %s (%s)", graded.Grade, graded.Reason)
	}
	// "No restart policy" is a fact about the container and not a next step.
	policy := responseTestFinding("gitea", "gitea exited cleanly and has no restart policy.")
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{policy}}); graded.Grade != GradeHandedBack {
		t.Fatalf("a conclusion with no next step graded %s (%s)", graded.Grade, graded.Reason)
	}
	// Without the clean exit the conclusion says no more than "it is stopped".
	bare := responseTestFinding("gitea", "gitea is stopped. Start the container.")
	if graded := GradeResponse(manifest, ResponseEvidence{AgentProfile: off, Findings: []ResponseFinding{bare}}); graded.Grade != GradeHandedBack {
		t.Fatalf("a conclusion without the clean exit graded %s (%s)", graded.Grade, graded.Reason)
	}
}

func TestQuietResourcesMustBeNegativeControlsOfAFaultedScenario(t *testing.T) {
	catalog := loadFaultSetCatalog(t)
	stopped := catalog.ByID["response.service-stopped-no-alert"]
	outcome := *stopped.Outcome
	outcome.QuietResources = []string{"gitea"}
	stopped.Outcome = &outcome
	if err := stopped.Validate(); err == nil || !strings.Contains(err.Error(), "is not a negative control") {
		t.Fatalf("a faulted resource was accepted as quiet: %v", err)
	}
	quiet := catalog.ByID["response.finished-job-stays-quiet"]
	outcome = *quiet.Outcome
	outcome.QuietResources = []string{"vault-migrate"}
	quiet.Outcome = &outcome
	if err := quiet.Validate(); err == nil || !strings.Contains(err.Error(), "remove outcome.quiet_resources") {
		t.Fatalf("a quiet scenario accepted quiet_resources: %v", err)
	}
	full := catalog.ByID["response.worker-crashed-stays-exited"]
	engagement := *full.Engagement
	engagement.Fallback = "ask_nicely"
	full.Engagement = &engagement
	if err := full.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported engagement.fallback") {
		t.Fatalf("an unknown fallback was accepted: %v", err)
	}
}

// The published schema is what an author checks a scenario against before the
// Go loader sees it. The stopped-service scenario passed the loader and failed
// the schema, which knew neither its fallback nor its quiet resources. Every
// field of the two response-track specs and every fallback the loader accepts
// has to exist in the schema.
func TestPublishedSchemaKnowsTheResponseTrackVocabulary(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "tests", "qualification", "patrol", "patrol.qual.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	for def, spec := range map[string]reflect.Type{"engagement": reflect.TypeOf(EngagementSpec{}), "outcome": reflect.TypeOf(OutcomeSpec{})} {
		for i := 0; i < spec.NumField(); i++ {
			field, _, _ := strings.Cut(spec.Field(i).Tag.Get("json"), ",")
			if _, ok := schema.Defs[def].Properties[field]; !ok {
				t.Errorf("the published schema has no %s.%s", def, field)
			}
		}
	}
	fallbacks := append([]string(nil), schema.Defs["engagement"].Properties["fallback"].Enum...)
	sort.Strings(fallbacks)
	if got, want := strings.Join(fallbacks, " "), EngagementFallbackFullRun+" "+EngagementFallbackScopedRun; got != want {
		t.Fatalf("published fallbacks = %q, the loader accepts %q", got, want)
	}
}
