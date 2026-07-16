package qualification

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// f64 returns a pointer to v; used so a table can distinguish "do not check"
// (nil) from a real 0.0 value.
func f64(v float64) *float64 { return &v }

// proTrackBaseSpec returns a fully-populated, passing InvestigationSpec. Each
// table case mutates a clone of it to drive a single uncovered branch.
func proTrackBaseSpec() *InvestigationSpec {
	return &InvestigationSpec{
		MinEvidenceIDs:         1,
		RequiredSummaryTerms:   []string{"stopped"},
		RootCauseResources:     []string{"dep"},
		AffectedResources:      []string{"client"},
		RequireCompletedStatus: true,
		MaxToolsUsed:           2,
	}
}

func proTrackBaseGround() GroundTruth {
	return GroundTruth{Resources: map[string]CollectedTruth{
		"dep":    {Alias: "dep", Name: "dep-name", ResourceID: "dep-id"},
		"client": {Alias: "client", Name: "client-name", ResourceID: "client-id"},
	}}
}

// proTrackPassingSummary names every scenario-owned causal and affected resource
// in the sections that markdownSection extracts, so a base invocation fully
// passes every investigation gate.
const proTrackPassingSummary = "### Investigation Summary\nThe dependency is stopped.\n\n" +
	"### Root Cause\n`dep-name` (`dep-id`) is stopped.\n\n" +
	"### Affected Resources\n`client-name` (`client-id`) is unhealthy."

func proTrackBaseInvestigation() aicontracts.InvestigationSession {
	return aicontracts.InvestigationSession{
		ID:          "inv-1",
		FindingID:   "finding-1",
		Status:      aicontracts.InvestigationStatusCompleted,
		Summary:     proTrackPassingSummary,
		EvidenceIDs: []string{"e1"},
		ToolsUsed:   []string{"t1"},
	}
}

// TestApplyProTrackGatesBranchCoverage exercises the uncovered branches of
// ApplyProTrackGates: the Watch early return, the nil spec short circuit, the
// total==0 path, every summary-term / term-group / forbidden-term arm, the
// root-cause and affected grounding hits and misses, the MaxToolsUsed gate, the
// insufficient-evidence hard failure, and every remediation arm.
func TestApplyProTrackGatesBranchCoverage(t *testing.T) {
	cases := []struct {
		name string
		// setup returns the inputs handed to ApplyProTrackGates.
		setup func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult)
		// score is the Score value seeded before ApplyProTrackGates runs, so
		// cases can prove (e.g. the Watch return) that it is left untouched.
		score                      func() Score
		wantPassed                 bool
		hardHas, hardNone          []string
		gateHas, gateNone          []string
		rootCauseGrounding         *float64
		affectedGrounding          *float64
		investigationGrounding     *float64
		investigationCompletion    *float64
		checkPassedUnchanged       bool // assert score.Passed == the score() seed value
		checkHardFailuresUnchanged bool // assert HardFailures slice identity unchanged
	}{
		{
			name: "watch track returns early leaving score untouched",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest() // TrackWatch
				return m, GroundTruth{}, nil, RemediationResult{}
			},
			score: func() Score {
				return Score{Passed: true, HardFailures: []string{"preexisting"}, InvestigationGrounding: 7}
			},
			checkPassedUnchanged:       true,
			checkHardFailuresUnchanged: true,
			investigationGrounding:     f64(7), // untouched: gate never assigned it
		},
		{
			name: "nil investigation spec fails closed",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = nil
				return m, GroundTruth{}, nil, RemediationResult{}
			},
			wantPassed: false,
			hardHas:    []string{"investigation expectations are missing"},
		},
		{
			name: "no investigation evidence captured hard fails but reports unit ratios",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{}, RemediationResult{}
			},
			wantPassed:              false,
			hardHas:                 []string{"no investigation evidence was captured"},
			investigationGrounding:  f64(1), // ratio(0,0) == 1
			investigationCompletion: f64(1),
		},
		{
			name: "completed status not required accepts running investigation",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.RequireCompletedStatus = false
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.Status = aicontracts.InvestigationStatusRunning
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed:              true,
			hardNone:                []string{"ended with status"},
			investigationCompletion: f64(1),
		},
		{
			name: "non-completed status when required hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				inv := proTrackBaseInvestigation()
				inv.Status = aicontracts.InvestigationStatusFailed
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed:              false,
			hardHas:                 []string{"ended with status", string(aicontracts.InvestigationStatusFailed)},
			investigationCompletion: f64(0),
		},
		{
			name: "missing required summary term hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				inv := proTrackBaseInvestigation()
				// Remove every occurrence of the required term while keeping
				// the grounded resource sections intact (root-cause grounding
				// only checks for dep-name/dep-id, not the word "stopped").
				inv.Summary = strings.ReplaceAll(inv.Summary, "stopped", "halted")
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: false,
			hardHas:    []string{"lacks required ground-truth term", `"stopped"`},
		},
		{
			name: "term group matches via non-empty alternative skipping empty entries",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				// The first alternative is empty and must be skipped; "halted"
				// is the accepted alternative appended to the summary.
				spec.RequiredSummaryTermGroups = [][]string{{"", "halted"}}
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.Summary += "\nThe dependency is halted."
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: true,
			hardNone:   []string{"lacks every accepted ground-truth term"},
		},
		{
			name: "term group unmatched when no alternative matches",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				// Neither alternative appears in the base summary.
				spec.RequiredSummaryTermGroups = [][]string{{"halted", "exited"}}
				m.Investigation = spec
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, RemediationResult{}
			},
			wantPassed: false,
			hardHas:    []string{"lacks every accepted ground-truth term"},
		},
		{
			name: "forbidden summary term present hard fails while empty forbidden entry is skipped",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.ForbiddenSummaryTerms = []string{"", "leak"}
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.Summary += "\nA secret leak was observed."
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: false,
			hardHas:    []string{"contains forbidden term", `"leak"`},
		},
		{
			name: "root cause grounding hit reports full ratio",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, RemediationResult{}
			},
			wantPassed:         true,
			hardNone:           []string{"Root Cause section"},
			rootCauseGrounding: f64(1),
		},
		{
			name: "root cause grounding miss with affected still grounded",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				inv := proTrackBaseInvestigation()
				// Root Cause section exists but names the wrong resource.
				inv.Summary = "### Root Cause\n`client-name` is the problem.\n\n### Affected Resources\n`client-name` (`client-id`) is unhealthy."
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed:         false,
			hardHas:            []string{"Root Cause section"},
			hardNone:           []string{"Affected Resources section"},
			rootCauseGrounding: f64(0),
			affectedGrounding:  f64(1),
		},
		{
			name: "affected grounding miss with root cause still grounded",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				inv := proTrackBaseInvestigation()
				inv.Summary = "### Root Cause\n`dep-name` (`dep-id`) is stopped.\n\n### Affected Resources\n`dep-name` is also affected."
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed:         false,
			hardHas:            []string{"Affected Resources section"},
			affectedGrounding:  f64(0),
			rootCauseGrounding: f64(1),
		},
		{
			name: "insufficient evidence ids hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.MinEvidenceIDs = 2
				m.Investigation = spec
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, RemediationResult{}
			},
			wantPassed: false,
			hardHas:    []string{"made 1 evidence calls; requires 2"},
		},
		{
			name: "max tools used exceeded records gate failure",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.MaxToolsUsed = 2
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.ToolsUsed = []string{"a", "b", "c"}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: false,
			hardNone:   []string{"tools"},
			gateHas:    []string{"used 3 tools; maximum is 2"},
		},
		{
			name: "max tools used zero skips the tool budget gate",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.MaxToolsUsed = 0
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.ToolsUsed = []string{"a", "b", "c"}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: true,
			gateNone:   []string{"tools"},
		},
		{
			name: "max evidence calls gates actual call volume independently of tool diversity",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				spec := proTrackBaseSpec()
				spec.MaxEvidenceCalls = 2
				m.Investigation = spec
				inv := proTrackBaseInvestigation()
				inv.EvidenceCallCount = 3
				inv.EvidenceIDs = []string{"e1", "e2", "e3"}
				inv.ToolsUsed = []string{"one-tool-name"}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": inv}, RemediationResult{}
			},
			wantPassed: false,
			gateHas:    []string{"made 3 evidence calls; maximum is 2"},
			gateNone:   []string{"used 3 tools"},
		},
		{
			name: "remediation missing action id hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "observe"}
				rem := RemediationResult{ActionID: "", OriginBound: true, PlanHashBound: true, Passed: true, Authorized: true}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: false,
			hardHas:    []string{"no exact governed action was captured"},
		},
		{
			name: "remediation origin binding failed hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "observe"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: false, PlanHashBound: true, Passed: true, Authorized: true}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: false,
			hardHas:    []string{"governed action identity or origin binding failed"},
		},
		{
			name: "remediation plan hash binding failed hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "observe"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: false, Passed: true, Authorized: true}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: false,
			hardHas:    []string{"governed action identity or origin binding failed"},
		},
		{
			name: "remediation did not pass hard fails",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "observe"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: true, Passed: false, Authorized: true}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: false,
			hardHas:    []string{"governed remediation track did not pass"},
		},
		{
			name: "remediation authorization gate fires for non-observe decision",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "execute"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: true, Passed: true, Authorized: false}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: false,
			hardHas:    []string{"proceeded without the benchmark authorization gate"},
		},
		{
			name: "remediation observe decision skips authorization gate",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "observe"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: true, Passed: true, Authorized: false}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: true,
			hardNone:   []string{"authorization gate"},
		},
		{
			name: "remediation nil spec skips authorization gate",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = nil
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: true, Passed: true, Authorized: false}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: true,
			hardNone:   []string{"authorization gate"},
		},
		{
			name: "full investigation track passes with grounded ratios",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackInvestigation
				m.Investigation = proTrackBaseSpec()
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, RemediationResult{}
			},
			wantPassed:              true,
			investigationGrounding:  f64(1),
			investigationCompletion: f64(1),
			rootCauseGrounding:      f64(1),
			affectedGrounding:       f64(1),
		},
		{
			name: "full remediation track passes when authorized for execute decision",
			setup: func() (Manifest, GroundTruth, map[string]aicontracts.InvestigationSession, RemediationResult) {
				m := validTestManifest()
				m.Track = TrackRemediation
				m.Investigation = proTrackBaseSpec()
				m.Remediation = &RemediationSpec{Decision: "execute"}
				rem := RemediationResult{ActionID: "action-1", OriginBound: true, PlanHashBound: true, Passed: true, Authorized: true}
				return m, proTrackBaseGround(), map[string]aicontracts.InvestigationSession{"finding-1": proTrackBaseInvestigation()}, rem
			},
			wantPassed: true,
			hardNone:   []string{"authorization gate", "origin binding", "did not pass", "governed action"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest, ground, investigations, remediation := tc.setup()
			seed := Score{Passed: true}
			if tc.score != nil {
				seed = tc.score()
			}
			score := seed
			ApplyProTrackGates(&score, manifest, ground, investigations, remediation)

			joinedHard := strings.Join(score.HardFailures, "\n")
			joinedGate := strings.Join(score.GateFailures, "\n")

			if tc.checkPassedUnchanged {
				if score.Passed != seed.Passed {
					t.Fatalf("Watch early return must leave Passed untouched: got %v want %v", score.Passed, seed.Passed)
				}
			} else if score.Passed != tc.wantPassed {
				t.Fatalf("Passed = %v, want %v (hard=%q gate=%q)", score.Passed, tc.wantPassed, joinedHard, joinedGate)
			}

			if tc.checkHardFailuresUnchanged {
				if len(score.HardFailures) != len(seed.HardFailures) || (len(score.HardFailures) > 0 && score.HardFailures[0] != seed.HardFailures[0]) {
					t.Fatalf("Watch early return must leave HardFailures untouched: got %v want %v", score.HardFailures, seed.HardFailures)
				}
			}

			for _, want := range tc.hardHas {
				if !strings.Contains(joinedHard, want) {
					t.Fatalf("expected HardFailures to contain %q; got %q", want, joinedHard)
				}
			}
			for _, avoid := range tc.hardNone {
				if strings.Contains(joinedHard, avoid) {
					t.Fatalf("expected HardFailures to NOT contain %q; got %q", avoid, joinedHard)
				}
			}
			for _, want := range tc.gateHas {
				if !strings.Contains(joinedGate, want) {
					t.Fatalf("expected GateFailures to contain %q; got %q", want, joinedGate)
				}
			}
			for _, avoid := range tc.gateNone {
				if strings.Contains(joinedGate, avoid) {
					t.Fatalf("expected GateFailures to NOT contain %q; got %q", avoid, joinedGate)
				}
			}

			if tc.rootCauseGrounding != nil && score.RootCauseGrounding != *tc.rootCauseGrounding {
				t.Fatalf("RootCauseGrounding = %v, want %v", score.RootCauseGrounding, *tc.rootCauseGrounding)
			}
			if tc.affectedGrounding != nil && score.AffectedGrounding != *tc.affectedGrounding {
				t.Fatalf("AffectedGrounding = %v, want %v", score.AffectedGrounding, *tc.affectedGrounding)
			}
			if tc.investigationGrounding != nil && score.InvestigationGrounding != *tc.investigationGrounding {
				t.Fatalf("InvestigationGrounding = %v, want %v", score.InvestigationGrounding, *tc.investigationGrounding)
			}
			if tc.investigationCompletion != nil && score.InvestigationCompletion != *tc.investigationCompletion {
				t.Fatalf("InvestigationCompletion = %v, want %v", score.InvestigationCompletion, *tc.investigationCompletion)
			}
		})
	}
}

// TestInvestigationSectionGroundedBranchCoverage exercises every branch of the
// pure helper: the empty-alias guard, the missing-section early return, the
// missing-resource map lookup, and each name/id containment combination.
func TestInvestigationSectionGroundedBranchCoverage(t *testing.T) {
	resources := map[string]CollectedTruth{
		"named":      {Alias: "named", Name: "Named Resource", ResourceID: "id-only"},
		"identified": {Alias: "identified", Name: "", ResourceID: "res-123"},
		"empty":      {Alias: "empty", Name: "", ResourceID: ""},
		"both":       {Alias: "both", Name: "Both Name", ResourceID: "both-id"},
	}
	const summary = "### Root Cause\nNamed Resource and res-123 and Both Name / both-id appear here.\n"

	cases := []struct {
		name      string
		summary   string
		heading   string
		aliases   []string
		resources map[string]CollectedTruth
		want      bool
	}{
		{
			name:    "nil aliases returns true",
			summary: "anything",
			heading: "Root Cause",
			aliases: nil,
			want:    true,
		},
		{
			name:    "empty aliases slice returns true",
			summary: "anything",
			heading: "Root Cause",
			aliases: []string{},
			want:    true,
		},
		{
			name:    "missing heading returns false",
			summary: "no headings here at all",
			heading: "Root Cause",
			aliases: []string{"named"},
			want:    false,
		},
		{
			name:    "empty summary returns false",
			summary: "",
			heading: "Root Cause",
			aliases: []string{"named"},
			want:    false,
		},
		{
			name:      "alias absent from resource map returns false",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"unknown"},
			resources: resources,
			want:      false,
		},
		{
			name:      "resource name match with empty id returns true",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"named"},
			resources: resources,
			want:      true,
		},
		{
			name:      "resource id match with empty name returns true",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"identified"},
			resources: resources,
			want:      true,
		},
		{
			name:      "both name and id present and matching returns true",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"both"},
			resources: resources,
			want:      true,
		},
		{
			name:      "neither name nor id contained returns false",
			summary:   "### Root Cause\nnothing relevant is mentioned here\n",
			heading:   "Root Cause",
			aliases:   []string{"both"},
			resources: resources,
			want:      false,
		},
		{
			name:      "resource with empty name and id returns false",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"empty"},
			resources: resources,
			want:      false,
		},
		{
			name:      "all aliases grounded returns true",
			summary:   summary,
			heading:   "Root Cause",
			aliases:   []string{"named", "identified", "both"},
			resources: resources,
			want:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.resources
			if res == nil {
				res = map[string]CollectedTruth{}
			}
			got := investigationSectionGrounded(tc.summary, tc.heading, tc.aliases, res)
			if got != tc.want {
				t.Fatalf("investigationSectionGrounded(%q, %q, %v) = %v, want %v", tc.summary, tc.heading, tc.aliases, got, tc.want)
			}
		})
	}
}
