package qualification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
