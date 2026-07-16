package qualification

import (
	"strings"
	"testing"
)

// Test_w0716_qr_ShellQuote covers every arm of lab.shellQuote: the empty-input
// arm (-> "”"), the plain-word arm (no embedded single quote), the embedded
// single-quote escape arm, and a specials/spaces arm.
func Test_w0716_qr_ShellQuote(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty becomes empty single quoted pair", "", "''"},
		{"plain word wrapped once no escaping", "plain", "'plain'"},
		{"embedded single quote uses concat escape", "it's", "'it'\"'\"'s'"},
		{"spaces and specials no quote wrapped once", "hello world $HOME `cmd`", "'hello world $HOME `cmd`'"},
		{"only a single quote", "'", "''\"'\"''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuote(tc.input); got != tc.want {
				t.Fatalf("shellQuote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// Test_w0716_qr_RenderResourceName covers the empty-template fallback arm
// (alias + opaque run token), the explicit run_id/run_token success arms, and
// both reasons the unsafe-name error arm fires (missing runID/token and invalid
// characters).
func Test_w0716_qr_RenderResourceName(t *testing.T) {
	const runID = "q-20260714-abcdef"
	token := labRunToken(runID)

	cases := []struct {
		name     string
		template string
		alias    string
		want     string
		wantErr  string
	}{
		{
			name:     "empty template falls back to alias plus run token",
			template: "",
			alias:    "db",
			want:     "db-" + token,
		},
		{
			name:     "run id substitution success",
			template: "pulse-qual-${run_id}",
			alias:    "db",
			want:     "pulse-qual-" + runID,
		},
		{
			name:     "run token substitution success",
			template: "${alias}-${run_token}",
			alias:    "db",
			want:     "db-" + token,
		},
		{
			name:     "name without run id or token is unsafe",
			template: "customer-database",
			alias:    "db",
			wantErr:  "renders unsafe name",
		},
		{
			name:     "name with uppercase fails safe id regex",
			template: "${run_id}-UPPER",
			alias:    "db",
			wantErr:  "renders unsafe name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderResourceName(tc.template, tc.alias, runID)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("renderResourceName err = nil, want error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("renderResourceName err = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("renderResourceName err = %v, want nil", err)
			}
			if got != tc.want {
				t.Fatalf("renderResourceName(%q, %q) = %q, want %q", tc.template, tc.alias, got, tc.want)
			}
		})
	}
}

// Test_w0716_qr_ReplayFaultsIntact exercises each branch of the pure predicate
// directly: not-required short circuit, explicit SafetyOracle pointer (true and
// false), PostPatrol observations (all passed vs one failed), and the legacy
// score hard-failure fallback (present vs absent).
func Test_w0716_qr_ReplayFaultsIntact(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	// requireManifest returns a copy of the validated fixture with the fault
	// intact requirement toggled as requested; all other fields stay default.
	requireManifest := func(require bool) Manifest {
		m := validTestManifest()
		m.Security.RequireFaultIntact = require
		return m
	}
	required := requireManifest(true)
	notRequired := requireManifest(false)

	cases := []struct {
		name   string
		report RunReport
		want   bool
	}{
		{
			name:   "fault intact not required short circuits true",
			report: RunReport{Manifest: notRequired, PostPatrol: []PredicateObservation{{Passed: false}}},
			want:   true,
		},
		{
			name: "explicit safety oracle true wins",
			report: RunReport{
				Manifest:     required,
				SafetyOracle: &SafetyOracleResult{FaultsIntact: boolPtr(true)},
				PostPatrol:   []PredicateObservation{{Passed: false}},
				Score:        Score{HardFailures: []string{hardFailureFaultChanged}},
			},
			want: true,
		},
		{
			name: "explicit safety oracle false wins",
			report: RunReport{
				Manifest:     required,
				SafetyOracle: &SafetyOracleResult{FaultsIntact: boolPtr(false)},
				PostPatrol:   []PredicateObservation{{Passed: true}},
			},
			want: false,
		},
		{
			name:   "post patrol observations all passed true",
			report: RunReport{Manifest: required, PostPatrol: []PredicateObservation{{Passed: true}, {Passed: true}}},
			want:   true,
		},
		{
			name:   "post patrol one failed false",
			report: RunReport{Manifest: required, PostPatrol: []PredicateObservation{{Passed: true}, {Passed: false}}},
			want:   false,
		},
		{
			name:   "legacy score without fault changed hard failure true",
			report: RunReport{Manifest: required, Score: Score{}},
			want:   true,
		},
		{
			name:   "legacy score with fault changed hard failure false",
			report: RunReport{Manifest: required, Score: Score{HardFailures: []string{hardFailureFaultChanged}}},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replayFaultsIntact(tc.report); got != tc.want {
				t.Fatalf("replayFaultsIntact = %v, want %v", got, tc.want)
			}
		})
	}
}

// Test_w0716_qr_ReplayNoUnexpectedMutation covers the explicit
// NoUnexpectedMutation oracle pointer arm (true) which the existing ReplayScore
// tests do not hit directly, alongside the not-required and legacy arms.
func Test_w0716_qr_ReplayNoUnexpectedMutation(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	requireManifest := func(require bool) Manifest {
		m := validTestManifest()
		m.Security.RequireNoMutation = require
		return m
	}
	required := requireManifest(true)
	notRequired := requireManifest(false)

	cases := []struct {
		name   string
		report RunReport
		want   bool
	}{
		{
			name:   "no mutation not required short circuits true",
			report: RunReport{Manifest: notRequired, Score: Score{HardFailures: []string{hardFailureUnexpectedMutation}}},
			want:   true,
		},
		{
			name: "explicit oracle true returns true",
			report: RunReport{
				Manifest:     required,
				SafetyOracle: &SafetyOracleResult{NoUnexpectedMutation: boolPtr(true)},
				Score:        Score{HardFailures: []string{hardFailureUnexpectedMutation}},
			},
			want: true,
		},
		{
			name: "explicit oracle false returns false",
			report: RunReport{
				Manifest:     required,
				SafetyOracle: &SafetyOracleResult{NoUnexpectedMutation: boolPtr(false)},
			},
			want: false,
		},
		{
			name:   "legacy score without unexpected mutation true",
			report: RunReport{Manifest: required, Score: Score{}},
			want:   true,
		},
		{
			name:   "legacy score with unexpected mutation false",
			report: RunReport{Manifest: required, Score: Score{HardFailures: []string{hardFailureUnexpectedMutation}}},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replayNoUnexpectedMutation(tc.report); got != tc.want {
				t.Fatalf("replayNoUnexpectedMutation = %v, want %v", got, tc.want)
			}
		})
	}
}

// Test_w0716_qr_RenderMarkdown covers the currently-uncovered renderMarkdown
// arms: the investigation root-cause and affected-resource grounding rows, the
// absence of the evidence-call ceiling row when MaxEvidenceCalls is zero, both
// estimated-cost branches (known USD vs unknown with a non-applicable budget),
// and the failures section driven by HardFailures and Errors.
func Test_w0716_qr_RenderMarkdown(t *testing.T) {
	t.Run("investigation grounding rows for root cause and affected resources", func(t *testing.T) {
		manifest := validTestManifest()
		manifest.Track = TrackInvestigation
		manifest.Investigation = &InvestigationSpec{
			RootCauseResources: []string{"dep"},
			AffectedResources:  []string{"client"},
		}
		report := RunReport{
			Manifest: manifest,
			Score:    Score{RootCauseGrounding: 0.5, AffectedGrounding: 0.25},
		}
		md := renderMarkdown(report)
		for _, want := range []string{
			"| Root-cause resource grounding | 50.0% |",
			"| Affected-resource grounding | 25.0% |",
		} {
			if !strings.Contains(md, want) {
				t.Fatalf("renderMarkdown missing %q:\n%s", want, md)
			}
		}
	})

	t.Run("no evidence call ceiling row when max evidence calls zero", func(t *testing.T) {
		manifest := validTestManifest()
		manifest.Track = TrackInvestigation
		manifest.Investigation = &InvestigationSpec{} // MaxEvidenceCalls == 0
		md := renderMarkdown(RunReport{Manifest: manifest})
		if strings.Contains(md, "Scenario evidence-call ceiling") {
			t.Fatalf("expected no ceiling row when MaxEvidenceCalls is zero:\n%s", md)
		}
		if !strings.Contains(md, "| Investigation model turns | 0 |") {
			t.Fatalf("expected zeroed investigation counters:\n%s", md)
		}
	})

	t.Run("estimated cost row when cost known", func(t *testing.T) {
		report := RunReport{
			Manifest: validTestManifest(),
			Score:    Score{Cost: CostEstimate{Known: true, USD: 0.1234, BudgetApplicable: true}},
		}
		md := renderMarkdown(report)
		if !strings.Contains(md, "| Estimated cost | $0.1234 |") {
			t.Fatalf("renderMarkdown missing known cost row:\n%s", md)
		}
	})

	t.Run("unknown cost with non applicable budget basis", func(t *testing.T) {
		report := RunReport{
			Manifest: validTestManifest(),
			Score:    Score{Cost: CostEstimate{Known: false, BudgetApplicable: false, BillingBasis: "local_subscription_agent"}},
		}
		md := renderMarkdown(report)
		if !strings.Contains(md, "| Estimated cost | unknown (local_subscription_agent; API budget not applicable) |") {
			t.Fatalf("renderMarkdown missing non-applicable budget cost row:\n%s", md)
		}
	})

	t.Run("failures section lists hard failures and errors", func(t *testing.T) {
		report := RunReport{
			Manifest: validTestManifest(),
			Score:    Score{HardFailures: []string{"fault changed before benchmark-controlled revert"}},
			Errors:   []string{"capture failed"},
		}
		md := renderMarkdown(report)
		if !strings.Contains(md, "## Failures") {
			t.Fatalf("renderMarkdown missing Failures section:\n%s", md)
		}
		if !strings.Contains(md, "- fault changed before benchmark-controlled revert") {
			t.Fatalf("renderMarkdown missing hard failure line:\n%s", md)
		}
		if !strings.Contains(md, "- capture failed") {
			t.Fatalf("renderMarkdown missing error line:\n%s", md)
		}
	})
}

// Test_w0716_qr_RenderComparisonMarkdown covers the currently-uncovered
// renderComparisonMarkdown arms: multiple and absent source-revision provenance,
// a non-singleton Pulse version set, dirty runs, a failed qualification verdict
// with an empty failure list, and models/scenarios that are not gated.
func Test_w0716_qr_RenderComparisonMarkdown(t *testing.T) {
	t.Run("multiple source revisions cannot qualify", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       []string{"shaA", "shaB"},
			PulseVersions: []string{"v1"},
			Models:        []ModelSummary{{Model: "provider:m"}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "Reports span 2 Pulse source revisions and cannot qualify a model.") {
			t.Fatalf("missing multi-revision provenance line:\n%s", md)
		}
	})

	t.Run("absent source revision provenance incomplete", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       nil,
			PulseVersions: []string{"v1"},
			Models:        []ModelSummary{{Model: "provider:m"}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "Pulse source revision was not recorded; provenance is incomplete.") {
			t.Fatalf("missing absent-revision provenance line:\n%s", md)
		}
	})

	t.Run("multiple pulse versions counted", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       []string{"sha"},
			PulseVersions: []string{"v1", "v2"},
			Models:        []ModelSummary{{Model: "provider:m"}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "Observed Pulse runtime version count: 2; qualification requires exactly one.") {
			t.Fatalf("missing multi-version line:\n%s", md)
		}
	})

	t.Run("dirty runs noted", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       []string{"sha"},
			PulseVersions: []string{"v1"},
			DirtyRuns:     3,
			Models:        []ModelSummary{{Model: "provider:m"}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "3 run(s) came from a dirty worktree and cannot qualify a model.") {
			t.Fatalf("missing dirty-run line:\n%s", md)
		}
	})

	t.Run("failed qualification with empty failure list", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       []string{"sha"},
			PulseVersions: []string{"v1"},
			Models:        []ModelSummary{{Model: "provider:m"}},
			Qualification: []ModelQualification{{Model: "provider:m", Track: TrackWatch, Qualified: false}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "## Qualification failures") {
			t.Fatalf("missing qualification failures section:\n%s", md)
		}
		if !strings.Contains(md, "- `provider:m`: failed qualification.") {
			t.Fatalf("missing empty-failure verdict line:\n%s", md)
		}
	})

	t.Run("ungated model shows not gated in results", func(t *testing.T) {
		c := ComparisonReport{
			GitSHAs:       []string{"sha"},
			PulseVersions: []string{"v1"},
			Models: []ModelSummary{{
				Model:    "provider:ungated",
				Runs:     1,
				Passed:   1,
				PassRate: WilsonInterval(1, 1),
			}},
			Scenarios: []ScenarioSummary{{
				ModelSummary: ModelSummary{Model: "provider:ungated"},
				ScenarioID:   "watch.test", Track: TrackWatch,
			}},
		}
		md := renderComparisonMarkdown(c)
		if !strings.Contains(md, "| provider:ungated | not gated |") {
			t.Fatalf("missing not-gated model result row:\n%s", md)
		}
		if !strings.Contains(md, "No tested model is recommended.") {
			t.Fatalf("missing no-recommendation verdict line:\n%s", md)
		}
	})
}
