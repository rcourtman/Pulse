package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestFaultSetV1HoldsSixLabelledScenarios(t *testing.T) {
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
	if got, want := strings.Join(labels, " "), "D1 D2 D3 D4 D5 D6"; got != want {
		t.Fatalf("fault set v1 labels = %q, want %q", got, want)
	}
	if got, want := strings.Join(expects, " "), "fixed fixed quiet handed_over fixed handed_over"; got != want {
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
