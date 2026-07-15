package qualification

import (
	"encoding/json"
	"strings"
	"testing"
)

// This file is a white-box, table-driven branch-coverage suite for
// ApplyQualificationGates in report.go. Helpers and shared identifiers are
// prefixed with `qualgates` so they do not collide with sibling test files in
// package qualification. Each row constructs a small literal ComparisonReport
// and Catalog; no live service, executor, provider, DB, filesystem, or
// goroutine is required.

// qualgatesBaseline returns a ComparisonReport and Catalog that is known to
// fully qualify under track: a single model, single git revision, single Pulse
// runtime version, no dirty runs, one scenario whose manifest digest matches
// the catalog, with 30/30 passing runs (Wilson lower bound well above 0.85) and
// 30/30 fault recall. Tests mutate the returned pointer to introduce exactly
// the gate failure under test.
func qualgatesBaseline(t *testing.T, track Track) (*ComparisonReport, Catalog) {
	t.Helper()
	manifest := validTestManifest()
	manifest.Track = track
	manifest.Repeat.Qualification = 30
	digest, err := manifest.Digest()
	if err != nil {
		t.Fatalf("digest baseline manifest: %v", err)
	}
	catalog := Catalog{
		Manifests: []Manifest{manifest},
		ByID:      map[string]Manifest{manifest.ID: manifest},
	}
	comparison := &ComparisonReport{
		GitSHAs:       []string{"revision-a"},
		PulseVersions: []string{"6.0.0-test"},
		Models:        []ModelSummary{{Model: "provider:model"}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{
				Model:       "provider:model",
				Runs:        30,
				Passed:      30,
				PassRate:    WilsonInterval(30, 30),
				FaultRecall: WilsonInterval(30, 30),
			},
			ScenarioID:      manifest.ID,
			Track:           track,
			ManifestDigests: []string{digest},
		}},
	}
	return comparison, catalog
}

// qualgatesDigestOf recomputes a manifest digest, failing the test instead of
// returning an error so table rows stay terse.
func qualgatesDigestOf(t *testing.T, manifest Manifest) string {
	t.Helper()
	digest, err := manifest.Digest()
	if err != nil {
		t.Fatalf("digest manifest %s: %v", manifest.ID, err)
	}
	return digest
}

// TestApplyQualificationGatesRejectsUnsupportedTrack covers the early-return
// guard at report.go:369-371 for every unsupported track spelling, including
// the case-sensitivity boundary that a capitalised "Watch" must NOT match
// TrackWatch, and verifies no qualification verdicts are appended.
func TestApplyQualificationGatesRejectsUnsupportedTrack(t *testing.T) {
	cases := []struct {
		name  string
		track Track
	}{
		{"empty string is rejected", ""},
		{"unknown name is rejected", "bench"},
		{"capitalised watch is not watch", "Watch"},
		{"preview track is not supported", "preview"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comparison, catalog := qualgatesBaseline(t, TrackWatch)
			err := ApplyQualificationGates(comparison, catalog, tc.track)
			if err == nil {
				t.Fatalf("ApplyQualificationGates(track=%q) returned nil; want error", tc.track)
			}
			if !strings.Contains(err.Error(), "unsupported qualification track") {
				t.Fatalf("err = %q; want substring %q", err.Error(), "unsupported qualification track")
			}
			if !strings.Contains(err.Error(), string(tc.track)) {
				t.Fatalf("err = %q; want it to mention track %q", err.Error(), tc.track)
			}
			if len(comparison.Qualification) != 0 {
				t.Fatalf("Qualification populated after unsupported-track guard: %+v", comparison.Qualification)
			}
		})
	}
}

// TestApplyQualificationGatesScenarioGateFailures covers the per-scenario gate
// branches inside the per-model loop (report.go:426-453). Each row introduces
// exactly one failure mode on top of an otherwise-qualifying scenario, and the
// assertion verifies both the scenario-level failure message and that it is
// propagated up into the model verdict as "<manifestID>: <failure>".
func TestApplyQualificationGatesScenarioGateFailures(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat.Qualification = 30
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	baseDigest := qualgatesDigestOf(t, manifest)

	qualifyingScenario := func() ScenarioSummary {
		return ScenarioSummary{
			ModelSummary: ModelSummary{
				Model:       "provider:model",
				Runs:        30,
				Passed:      30,
				PassRate:    WilsonInterval(30, 30),
				FaultRecall: WilsonInterval(30, 30),
			},
			ScenarioID:      manifest.ID,
			Track:           TrackWatch,
			ManifestDigests: []string{baseDigest},
		}
	}

	cases := []struct {
		name             string
		mutate           func(*ScenarioSummary)
		wantFailureParts []string // each substring must appear in the joined verdict failures
		wantScenarioFail bool     // most rows mark scenario not-qualified; some leave it qualified
	}{
		{
			name: "passed less than runs records partial pass count",
			mutate: func(s *ScenarioSummary) {
				s.Runs = 30
				s.Passed = 27
				s.PassRate = WilsonInterval(27, 30)
			},
			wantFailureParts: []string{"only 27 of 30 runs passed"},
			wantScenarioFail: true,
		},
		{
			name: "fault recall total positive but lower below floor",
			mutate: func(s *ScenarioSummary) {
				s.FaultRecall = WilsonInterval(20, 30)
			},
			wantFailureParts: []string{"fault-recall Wilson lower bound"},
			wantScenarioFail: true,
		},
		{
			name: "nonzero false positives recorded",
			mutate: func(s *ScenarioSummary) {
				s.FalsePositives = 4
			},
			wantFailureParts: []string{"false positives=4 hard-failure runs=0"},
			wantScenarioFail: true,
		},
		{
			name: "nonzero hard-failure runs recorded",
			mutate: func(s *ScenarioSummary) {
				s.HardFailureRuns = 2
			},
			wantFailureParts: []string{"false positives=0 hard-failure runs=2"},
			wantScenarioFail: true,
		},
		{
			name: "scenario-local dirty runs",
			mutate: func(s *ScenarioSummary) {
				s.DirtyRuns = 3
			},
			wantFailureParts: []string{"3 run(s) used a dirty worktree"},
			wantScenarioFail: true,
		},
		{
			name: "scenario spans multiple git revisions",
			mutate: func(s *ScenarioSummary) {
				s.GitSHAs = []string{"sha-a", "sha-b"}
			},
			wantFailureParts: []string{"runs span 2 source revisions"},
			wantScenarioFail: true,
		},
		{
			name: "scenario manifest count exceeds one adds digest mismatch plus span failure",
			mutate: func(s *ScenarioSummary) {
				s.ManifestDigests = []string{baseDigest, strings.Repeat("a", 64)}
			},
			wantFailureParts: []string{
				"report manifest digest does not match the selected catalogue",
				"runs span 2 manifest revisions",
			},
			wantScenarioFail: true,
		},
		{
			name: "empty manifest digest list adds digest mismatch without span failure",
			mutate: func(s *ScenarioSummary) {
				s.ManifestDigests = nil
			},
			wantFailureParts: []string{"report manifest digest does not match the selected catalogue"},
			wantScenarioFail: true,
		},
		{
			name: "runs below required qualification repeat count without lowering pass rate",
			mutate: func(s *ScenarioSummary) {
				// WilsonInterval(29, 29) lower bound (~0.883) is still above
				// the 0.85 floor, so only the repeat-count gate fires.
				s.Runs = 29
				s.Passed = 29
				s.PassRate = WilsonInterval(29, 29)
				s.FaultRecall = WilsonInterval(29, 29)
			},
			wantFailureParts: []string{"runs 29 below qualification repeat 30"},
			wantScenarioFail: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scenario := qualifyingScenario()
			tc.mutate(&scenario)
			comparison := &ComparisonReport{
				GitSHAs:       []string{"revision-a"},
				PulseVersions: []string{"6.0.0-test"},
				Models:        []ModelSummary{{Model: "provider:model"}},
				Scenarios:     []ScenarioSummary{scenario},
			}
			if err := ApplyQualificationGates(comparison, catalog, TrackWatch); err != nil {
				t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
			}
			if len(comparison.Qualification) != 1 {
				t.Fatalf("Qualification = %+v; want exactly one verdict", comparison.Qualification)
			}
			verdict := comparison.Qualification[0]
			if verdict.Qualified {
				t.Fatalf("verdict qualified; want failure for %s", tc.name)
			}
			joined := strings.Join(verdict.Failures, " | ")
			for _, want := range tc.wantFailureParts {
				if !strings.Contains(joined, want) {
					t.Fatalf("verdict failures = %q; want substring %q", joined, want)
				}
			}
			// Failure must also be reflected at the scenario level, prefixed
			// with the manifest ID inside the verdict.
			if tc.wantScenarioFail {
				if comparison.Scenarios[0].Qualified {
					t.Fatalf("scenario.Qualified = true; want false for %s", tc.name)
				}
				if len(comparison.Scenarios[0].Failures) == 0 {
					t.Fatalf("scenario.Failures empty; want at least one for %s", tc.name)
				}
				for _, want := range tc.wantFailureParts {
					prefixed := manifest.ID + ": " + want
					if !strings.Contains(joined, prefixed) {
						t.Fatalf("verdict failures = %q; want scenario-prefixed substring %q", joined, prefixed)
					}
				}
			}
		})
	}
}

// TestApplyQualificationGatesMissingScenarioFailsVerdict covers the
// `scenario == nil` branch at report.go:417-421: a catalog manifest whose ID
// has no matching ScenarioSummary for the model under evaluation must mark the
// verdict unqualified with a "missing scenario <id>" failure and skip the
// remaining per-scenario gates (no panic, no digest lookup).
func TestApplyQualificationGatesMissingScenarioFailsVerdict(t *testing.T) {
	t.Run("catalog manifest absent from comparison scenarios", func(t *testing.T) {
		manifest := validTestManifest()
		manifest.Repeat.Qualification = 30
		catalog := Catalog{
			Manifests: []Manifest{manifest},
			ByID:      map[string]Manifest{manifest.ID: manifest},
		}
		// Comparison lists a different scenario ID than any catalog manifest,
		// so the inner scenario-search loop leaves scenario == nil.
		comparison := &ComparisonReport{
			GitSHAs:       []string{"revision-a"},
			PulseVersions: []string{"6.0.0-test"},
			Models:        []ModelSummary{{Model: "provider:model"}},
			Scenarios: []ScenarioSummary{{
				ModelSummary: ModelSummary{
					Model: "provider:model", Runs: 30, Passed: 30,
					PassRate: WilsonInterval(30, 30), FaultRecall: WilsonInterval(30, 30),
				},
				ScenarioID:      "watch.unrelated",
				Track:           TrackWatch,
				ManifestDigests: []string{qualgatesDigestOf(t, manifest)},
			}},
		}
		if err := ApplyQualificationGates(comparison, catalog, TrackWatch); err != nil {
			t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
		}
		if len(comparison.Qualification) != 1 {
			t.Fatalf("Qualification = %+v; want one verdict", comparison.Qualification)
		}
		verdict := comparison.Qualification[0]
		if verdict.Qualified {
			t.Fatal("verdict qualified; want missing-scenario failure")
		}
		joined := strings.Join(verdict.Failures, " | ")
		want := "missing scenario " + manifest.ID
		if !strings.Contains(joined, want) {
			t.Fatalf("verdict failures = %q; want substring %q", joined, want)
		}
		// The unrelated scenario is never iterated (only scenarios referenced by
		// a catalog manifest are evaluated), so ApplyQualificationGates must
		// neither append failures nor alter its Qualified flag. Set the flag
		// up-front so we can detect any mutation.
		if len(comparison.Scenarios[0].Failures) != 0 {
			t.Fatalf("unrelated scenario must not accumulate failures: %+v", comparison.Scenarios[0])
		}
	})

	t.Run("scenario exists for a different model only", func(t *testing.T) {
		// The scenario match requires both model AND scenario ID; a scenario
		// scoped to another model must not satisfy the model under evaluation.
		manifest := validTestManifest()
		manifest.Repeat.Qualification = 30
		catalog := Catalog{
			Manifests: []Manifest{manifest},
			ByID:      map[string]Manifest{manifest.ID: manifest},
		}
		comparison := &ComparisonReport{
			GitSHAs:       []string{"revision-a"},
			PulseVersions: []string{"6.0.0-test"},
			Models:        []ModelSummary{{Model: "provider:model"}},
			Scenarios: []ScenarioSummary{{
				ModelSummary: ModelSummary{
					Model: "provider:other", Runs: 30, Passed: 30,
					PassRate: WilsonInterval(30, 30), FaultRecall: WilsonInterval(30, 30),
				},
				ScenarioID:      manifest.ID,
				Track:           TrackWatch,
				ManifestDigests: []string{qualgatesDigestOf(t, manifest)},
			}},
		}
		if err := ApplyQualificationGates(comparison, catalog, TrackWatch); err != nil {
			t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
		}
		verdict := comparison.Qualification[0]
		if verdict.Model != "provider:model" {
			t.Fatalf("verdict model = %q; want provider:model", verdict.Model)
		}
		if verdict.Qualified {
			t.Fatal("verdict qualified; want missing-scenario failure for provider:model")
		}
		joined := strings.Join(verdict.Failures, " | ")
		want := "missing scenario " + manifest.ID
		if !strings.Contains(joined, want) {
			t.Fatalf("verdict failures = %q; want substring %q", joined, want)
		}
	})
}

// TestApplyQualificationGatesCatalogDigestErrorAborts covers the
// `manifest.Digest()` error path at report.go:422-425. Predicate.Value is a
// json.RawMessage; json.Marshal inside Digest runs it through the JSON
// compactor, which rejects raw bytes that are not valid JSON. The function
// must abort the whole evaluation (return the wrapped error) and leave no
// qualification verdicts behind.
func TestApplyQualificationGatesCatalogDigestErrorAborts(t *testing.T) {
	t.Run("invalid raw message in catalog manifest predicate aborts", func(t *testing.T) {
		manifest := validTestManifest()
		manifest.Repeat.Qualification = 1
		// Truncated JSON in a RawMessage field forces Digest()'s internal
		// json.Marshal to fail during compaction.
		manifest.Baseline = []Predicate{{
			Probe: "docker.running", Target: "target", Operator: "eq",
			Value: json.RawMessage("{ broken"),
		}}
		catalog := Catalog{
			Manifests: []Manifest{manifest},
			ByID:      map[string]Manifest{manifest.ID: manifest},
		}
		comparison := &ComparisonReport{
			GitSHAs:       []string{"revision-a"},
			PulseVersions: []string{"6.0.0-test"},
			Models:        []ModelSummary{{Model: "provider:model"}},
			Scenarios: []ScenarioSummary{{
				ModelSummary: ModelSummary{
					Model: "provider:model", Runs: 1, Passed: 1,
					PassRate: WilsonInterval(1, 1), FaultRecall: WilsonInterval(1, 1),
				},
				ScenarioID:      manifest.ID,
				Track:           TrackWatch,
				ManifestDigests: []string{"any-digest"},
			}},
		}
		err := ApplyQualificationGates(comparison, catalog, TrackWatch)
		if err == nil {
			t.Fatal("ApplyQualificationGates returned nil; want catalog digest error")
		}
		if !strings.Contains(err.Error(), "digest catalog scenario") {
			t.Fatalf("err = %q; want substring %q", err.Error(), "digest catalog scenario")
		}
		if !strings.Contains(err.Error(), manifest.ID) {
			t.Fatalf("err = %q; want it to mention manifest ID %q", err.Error(), manifest.ID)
		}
		if len(comparison.Qualification) != 0 {
			t.Fatalf("Qualification populated after digest abort: %+v", comparison.Qualification)
		}
	})
}

// TestApplyQualificationGatesGlobalComparabilityFailures covers the
// comparison-scoped gate branches at report.go:374-396 that are independent of
// any individual model: dirty runs alone, Pulse runtime version count != 1,
// and the cross-scenario manifest-digest aggregation that flags a scenario
// whose reports span more than one manifest revision.
func TestApplyQualificationGatesGlobalComparabilityFailures(t *testing.T) {
	manifest := validTestManifest()
	manifest.Repeat.Qualification = 1
	catalog := Catalog{Manifests: []Manifest{manifest}, ByID: map[string]Manifest{manifest.ID: manifest}}
	digest := qualgatesDigestOf(t, manifest)

	qualifyingScenario := func() ScenarioSummary {
		return ScenarioSummary{
			ModelSummary: ModelSummary{
				Model: "provider:model", Runs: 1, Passed: 1,
				PassRate: WilsonInterval(1, 1), FaultRecall: WilsonInterval(1, 1),
			},
			ScenarioID:      manifest.ID,
			Track:           TrackWatch,
			ManifestDigests: []string{digest},
		}
	}

	cases := []struct {
		name            string
		comparison      ComparisonReport
		wantFailures    []string // substrings that must appear in the verdict failures
		wantNotContains []string // substrings that must NOT appear (negative assertion)
	}{
		{
			name: "dirty runs alone fail comparability",
			comparison: ComparisonReport{
				GitSHAs: []string{"revision-a"}, DirtyRuns: 1,
				PulseVersions: []string{"6.0.0-test"},
				Models:        []ModelSummary{{Model: "provider:model"}},
				Scenarios:     []ScenarioSummary{qualifyingScenario()},
			},
			wantFailures:    []string{"1 run(s) were captured from a dirty worktree"},
			wantNotContains: []string{"source revisions", "Pulse runtime version"},
		},
		{
			name: "multiple pulse runtime versions fail comparability",
			comparison: ComparisonReport{
				GitSHAs:       []string{"revision-a"},
				PulseVersions: []string{"6.0.0-test", "6.1.0-test"},
				Models:        []ModelSummary{{Model: "provider:model"}},
				Scenarios:     []ScenarioSummary{qualifyingScenario()},
			},
			wantFailures:    []string{"reports contain 2 distinct recorded Pulse runtime versions; exactly one is required"},
			wantNotContains: []string{"dirty worktree", "source revisions"},
		},
		{
			name: "zero recorded pulse versions also fail comparability",
			comparison: ComparisonReport{
				GitSHAs:       []string{"revision-a"},
				PulseVersions: nil,
				Models:        []ModelSummary{{Model: "provider:model"}},
				Scenarios:     []ScenarioSummary{qualifyingScenario()},
			},
			wantFailures:    []string{"reports contain 0 distinct recorded Pulse runtime versions; exactly one is required"},
			wantNotContains: []string{"dirty worktree"},
		},
		{
			name: "scenario with multiple unique manifest digests aggregates to a global failure",
			comparison: ComparisonReport{
				GitSHAs:       []string{"revision-a"},
				PulseVersions: []string{"6.0.0-test"},
				Models:        []ModelSummary{{Model: "provider:model"}},
				Scenarios: []ScenarioSummary{func() ScenarioSummary {
					s := qualifyingScenario()
					// Two distinct digests in a single scenario trigger the
					// manifestDigestsByScenario aggregation branch.
					s.ManifestDigests = []string{digest, strings.Repeat("z", 64)}
					return s
				}()},
			},
			wantFailures: []string{
				"scenario " + manifest.ID + " contains 2 manifest revisions; exactly one is required",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comparison := tc.comparison
			if err := ApplyQualificationGates(&comparison, catalog, TrackWatch); err != nil {
				t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
			}
			if len(comparison.Qualification) != 1 {
				t.Fatalf("Qualification = %+v; want one verdict", comparison.Qualification)
			}
			verdict := comparison.Qualification[0]
			if verdict.Qualified {
				t.Fatalf("verdict qualified; want global comparability failure for %s", tc.name)
			}
			joined := strings.Join(verdict.Failures, " | ")
			for _, want := range tc.wantFailures {
				if !strings.Contains(joined, want) {
					t.Fatalf("verdict failures = %q; want substring %q", joined, want)
				}
			}
			for _, notWant := range tc.wantNotContains {
				if strings.Contains(joined, notWant) {
					t.Fatalf("verdict failures = %q; must NOT contain %q", joined, notWant)
				}
			}
		})
	}
}

// TestApplyQualificationGatesSkipsOffTrackCatalogManifests covers the
// `manifest.Track != track` continue branch at report.go:406-408. A catalog
// manifest pinned to a different track must be silently skipped — it must
// neither produce a missing-scenario failure nor otherwise disqualify the
// model when the on-track scenario qualifies.
func TestApplyQualificationGatesSkipsOffTrackCatalogManifests(t *testing.T) {
	watchManifest := validTestManifest()
	watchManifest.Track = TrackWatch
	watchManifest.Repeat.Qualification = 30

	investigationManifest := validTestManifest()
	investigationManifest.ID = "investigation.offtrack"
	investigationManifest.Track = TrackInvestigation
	investigationManifest.Repeat.Qualification = 30

	catalog := Catalog{
		Manifests: []Manifest{watchManifest, investigationManifest},
		ByID: map[string]Manifest{
			watchManifest.ID:         watchManifest,
			investigationManifest.ID: investigationManifest,
		},
	}
	// Comparison only has a scenario for the watch manifest; the investigation
	// manifest in the catalog must be skipped, not reported as missing.
	comparison := &ComparisonReport{
		GitSHAs:       []string{"revision-a"},
		PulseVersions: []string{"6.0.0-test"},
		Models:        []ModelSummary{{Model: "provider:model"}},
		Scenarios: []ScenarioSummary{{
			ModelSummary: ModelSummary{
				Model: "provider:model", Runs: 30, Passed: 30,
				PassRate: WilsonInterval(30, 30), FaultRecall: WilsonInterval(30, 30),
			},
			ScenarioID:      watchManifest.ID,
			Track:           TrackWatch,
			ManifestDigests: []string{qualgatesDigestOf(t, watchManifest)},
		}},
	}
	if err := ApplyQualificationGates(comparison, catalog, TrackWatch); err != nil {
		t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
	}
	if len(comparison.Qualification) != 1 {
		t.Fatalf("Qualification = %+v; want one verdict", comparison.Qualification)
	}
	verdict := comparison.Qualification[0]
	if !verdict.Qualified {
		t.Fatalf("off-track catalog manifest leaked into watch verdict: %+v", verdict)
	}
	joined := strings.Join(verdict.Failures, " | ")
	if strings.Contains(joined, "missing scenario") {
		t.Fatalf("off-track manifest must be skipped, not reported missing: %q", joined)
	}
	if strings.Contains(joined, investigationManifest.ID) {
		t.Fatalf("off-track manifest ID must not appear in verdict failures: %q", joined)
	}
}

// TestApplyQualificationGatesQualifiesAlternateTracks exercises the supported
// non-Watch tracks (TrackInvestigation and TrackRemediation) end-to-end so the
// guard at report.go:369 admits them and the per-manifest `manifest.Track !=
// track` filter at report.go:406 does not skip them. The manifest from
// validTestManifest has no Investigation or Remediation spec populated, but
// ApplyQualificationGates does not consult those fields, so the gate should
// still pass when the scenario summary is otherwise qualifying.
func TestApplyQualificationGatesQualifiesAlternateTracks(t *testing.T) {
	cases := []struct {
		name  string
		track Track
	}{
		{"investigation track qualifies", TrackInvestigation},
		{"remediation track qualifies", TrackRemediation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comparison, catalog := qualgatesBaseline(t, tc.track)
			if err := ApplyQualificationGates(comparison, catalog, tc.track); err != nil {
				t.Fatalf("ApplyQualificationGates returned unexpected error: %v", err)
			}
			if len(comparison.Qualification) != 1 {
				t.Fatalf("Qualification = %+v; want one verdict", comparison.Qualification)
			}
			verdict := comparison.Qualification[0]
			if verdict.Track != tc.track {
				t.Fatalf("verdict.Track = %q; want %q", verdict.Track, tc.track)
			}
			if !verdict.Qualified {
				t.Fatalf("verdict not qualified; alternate track should pass: %+v", verdict)
			}
			if !comparison.Scenarios[0].Qualified {
				t.Fatalf("scenario not qualified: %+v", comparison.Scenarios[0])
			}
		})
	}
}
