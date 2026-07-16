package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestSafetyOracleJSONPreservesCapturedFalse(t *testing.T) {
	faultsIntact := false
	noMutation := false
	report := RunReport{
		SchemaVersion: ReportSchemaVersion,
		SafetyOracle: &SafetyOracleResult{
			FaultsIntact:         &faultsIntact,
			NoUnexpectedMutation: &noMutation,
		},
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RunReport
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SafetyOracle == nil || decoded.SafetyOracle.FaultsIntact == nil || *decoded.SafetyOracle.FaultsIntact ||
		decoded.SafetyOracle.NoUnexpectedMutation == nil || *decoded.SafetyOracle.NoUnexpectedMutation {
		t.Fatalf("captured false safety values were not preserved: %+v", decoded.SafetyOracle)
	}
}

func TestRenderMarkdownReportsInvestigationEfficiencyCounters(t *testing.T) {
	manifest := validTestManifest()
	manifest.Track = TrackInvestigation
	manifest.Investigation = &InvestigationSpec{MinEvidenceIDs: 1, MaxEvidenceCalls: 10, RequireCompletedStatus: true}
	report := RunReport{
		Manifest: manifest,
		Passed:   true,
		Investigation: map[string]aicontracts.InvestigationSession{
			"finding-1": {
				TurnCount: 4, EvidenceCallCount: 6, ToolCallCount: 7,
				ToolsUsed: []string{"pulse_query", "pulse_read", "patrol_propose_action"},
			},
		},
	}
	markdown := renderMarkdown(report)
	for _, row := range []string{
		"| Investigation model turns | 4 |",
		"| Investigation evidence calls | 6 |",
		"| Investigation tool calls | 7 |",
		"| Distinct investigation tools | 3 |",
		"| Scenario evidence-call ceiling | 10 |",
	} {
		if !strings.Contains(markdown, row) {
			t.Fatalf("qualification report missing %q:\n%s", row, markdown)
		}
	}
}

func TestApplyQualificationGatesRequiresCatalogCoverageAndStatisticalFloor(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat.Qualification = 30
	digest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	comparison := ComparisonReport{
		GitSHAs:       []string{"revision-a"},
		PulseVersions: []string{"6.0.0-test"},
		Models:        []ModelSummary{{Model: "provider:model"}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{
				Model: "provider:model", Runs: 30, Passed: 30,
				PassRate: WilsonInterval(30, 30), FaultRecall: WilsonInterval(30, 30),
			},
			ScenarioID: manifest.ID, Track: TrackWatch, ManifestDigests: []string{digest},
		}},
	}
	if err := ApplyQualificationGates(&comparison, catalog, TrackWatch); err != nil {
		t.Fatal(err)
	}
	if len(comparison.Qualification) != 1 || !comparison.Qualification[0].Qualified || !comparison.Scenarios[0].Qualified {
		t.Fatalf("qualification = %+v", comparison)
	}

	comparison.Scenarios[0].Runs = 3
	comparison.Scenarios[0].Passed = 3
	comparison.Scenarios[0].PassRate = WilsonInterval(3, 3)
	comparison.Scenarios[0].FaultRecall = WilsonInterval(3, 3)
	comparison.Qualification = nil
	comparison.Scenarios[0].Failures = nil
	if err := ApplyQualificationGates(&comparison, catalog, TrackWatch); err != nil {
		t.Fatal(err)
	}
	if comparison.Qualification[0].Qualified {
		t.Fatal("small perfect sample must not qualify")
	}
}

func TestWriteComparisonReportPublishesOnlyQualifiedRecommendation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "publication")
	comparison := ComparisonReport{
		GitSHAs: []string{"deadbeef"}, PulseVersions: []string{"6.0.0-test"},
		Models: []ModelSummary{{
			Model: "provider:qualified", Runs: 30, Passed: 30,
			PassRate: WilsonInterval(30, 30), FaultRecall: WilsonInterval(30, 30),
		}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{Model: "provider:qualified", Runs: 30, Passed: 30},
			ScenarioID:   "watch.test", Track: TrackWatch, Qualified: true,
		}},
		Qualification: []ModelQualification{{Model: "provider:qualified", Track: TrackWatch, Qualified: true}},
	}
	if err := WriteComparisonReport(dir, comparison); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"comparison.json", "comparison.md", "SHA256SUMS"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", name, info.Mode().Perm())
		}
	}
	payload, err := os.ReadFile(filepath.Join(dir, "comparison.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "Recommended for the tested `watch` track: `provider:qualified`") {
		t.Fatalf("publication does not contain qualified recommendation:\n%s", payload)
	}

	comparison.Qualification[0].Qualified = false
	comparison.Qualification[0].Failures = []string{"scenario gate failed"}
	comparison.Scenarios[0].Qualified = false
	if err := WriteComparisonReport(dir, comparison); err != nil {
		t.Fatal(err)
	}
	payload, err = os.ReadFile(filepath.Join(dir, "comparison.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "No tested model is recommended") || strings.Contains(string(payload), "Recommended for the tested") {
		t.Fatalf("failed model must not be recommended:\n%s", payload)
	}
}

func TestWriteReportPreservesArtifactsWhenReplayCaptureIsIncomplete(t *testing.T) {
	dir := t.TempDir()
	report := RunReport{
		SchemaVersion: ReportSchemaVersion, RunID: "q-incomplete-replay", Passed: true,
		Manifest: validTestManifest(),
		PatrolRun: PatrolRun{ToolCalls: []ToolCall{{
			ID: "truncated", ToolName: "patrol_report_finding", Input: `{"description":"cut...`, Success: true,
		}}},
	}
	if err := WriteReport(dir, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ground-truth.json", "report.json", "report.md", "replay.json", "SHA256SUMS"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	written, err := LoadReport(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if written.Passed || !strings.Contains(strings.Join(written.Errors, "\n"), "replay capture incomplete") {
		t.Fatalf("incomplete capture must fail explicitly without losing artifacts: %+v", written)
	}
}

func TestApplyQualificationGatesRejectsIncomparableRuns(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat.Qualification = 1
	digest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	comparison := ComparisonReport{
		GitSHAs: []string{"revision-a", "revision-b"}, DirtyRuns: 1,
		PulseVersions: []string{"6.0.0-test"},
		Models:        []ModelSummary{{Model: "provider:model"}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{
				Model: "provider:model", Runs: 1, Passed: 1,
				PassRate: WilsonInterval(1, 1), FaultRecall: WilsonInterval(1, 1),
			},
			ScenarioID: manifest.ID, Track: TrackWatch, ManifestDigests: []string{digest},
		}},
	}
	if err := ApplyQualificationGates(&comparison, catalog, TrackWatch); err != nil {
		t.Fatal(err)
	}
	if comparison.Qualification[0].Qualified {
		t.Fatal("dirty reports spanning revisions must not qualify")
	}
	failures := strings.Join(comparison.Qualification[0].Failures, " ")
	if !strings.Contains(failures, "dirty worktree") || !strings.Contains(failures, "source revisions") {
		t.Fatalf("comparability failures = %q", failures)
	}
}

func TestApplyQualificationGatesRejectsStaleScenarioManifest(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat = RepeatSpec{Development: 1, Nightly: 1, Qualification: 1}
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	comparison := ComparisonReport{
		GitSHAs: []string{"revision-a"}, PulseVersions: []string{"6.1.0-test"},
		Models: []ModelSummary{{Model: "provider:model"}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{
				Model: "provider:model", Runs: 1, Passed: 1,
				PassRate: WilsonInterval(1, 1), FaultRecall: WilsonInterval(1, 1),
			},
			ScenarioID: manifest.ID, Track: TrackWatch,
			ManifestDigests: []string{strings.Repeat("0", 64)},
		}},
	}
	if err := ApplyQualificationGates(&comparison, catalog, TrackWatch); err != nil {
		t.Fatal(err)
	}
	if comparison.Qualification[0].Qualified || !strings.Contains(strings.Join(comparison.Qualification[0].Failures, " "), "does not match the selected catalogue") {
		t.Fatalf("stale manifest qualification = %+v", comparison.Qualification)
	}
}

func TestWriteReportIncludesReplayAndChecksumsWithPrivateMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "report")
	report := RunReport{
		SchemaVersion: ReportSchemaVersion, RunID: "q-report-test", Manifest: validTestManifest(),
		Environment: Environment{Model: "provider:model"},
		PatrolRun:   PatrolRun{ToolCalls: []ToolCall{{ID: "call-1", ToolName: "get_resource", Input: `{}`, Output: `{}`, Success: true}}},
	}
	if err := WriteReport(dir, report); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ground-truth.json", "report.json", "report.md", "replay.json", "SHA256SUMS"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", name, info.Mode().Perm())
		}
	}
}
