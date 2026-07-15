package qualification

import (
	"strings"
	"testing"
	"time"
)

// cleanGatedScore returns a Score whose gate-measured ratios are all at the
// hard floors, so applyGates emits nothing unless a case perturbs a field.
func cleanGatedScore() Score {
	return Score{
		Recall:               1,
		ResourceAccuracy:     1,
		ResourceTypeAccuracy: 1,
		CategoryAccuracy:     1,
		SeverityAccuracy:     1,
		EvidenceGrounding:    1,
		RecommendationSafety: 1,
	}
}

// cleanGatedManifest returns a Manifest with every gate threshold and budget
// disabled, so applyGates emits nothing unless a case perturbs a field.
func cleanGatedManifest() Manifest {
	return Manifest{Gates: GateSpec{}, Budgets: BudgetSpec{}}
}

func TestApplyGatesBranches(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(*Score, *Manifest)
		wantCount int    // expected number of GateFailures appended
		wantSub   string // required substring within the joined GateFailures
	}{
		{
			name:      "clean baseline emits no failures",
			setup:     func(*Score, *Manifest) {},
			wantCount: 0,
		},
		{
			name:      "recall below minimum",
			setup:     func(s *Score, m *Manifest) { s.Recall = 0.5; m.Gates.MinRecall = 1 },
			wantCount: 1,
			wantSub:   "recall 0.500 below 1.000",
		},
		{
			name:      "false positives exceed maximum",
			setup:     func(s *Score, m *Manifest) { s.FalsePositives = 1; m.Gates.MaxFalsePositives = 0 },
			wantCount: 1,
			wantSub:   "false positives 1 exceed 0",
		},
		{
			name:      "resource accuracy below configured minimum",
			setup:     func(s *Score, m *Manifest) { s.ResourceAccuracy = 0.5; m.Gates.MinResourceAccuracy = 1 },
			wantCount: 1,
			wantSub:   "resource accuracy 0.500 below 1.000",
		},
		{
			name: "resource type accuracy below hard floor",
			setup: func(s *Score, m *Manifest) {
				s.ResourceTypeAccuracy = 0.5
				// Keep resource accuracy above its own configured minimum so only
				// the hardcoded resource-type floor fires.
				m.Gates.MinResourceAccuracy = 0
			},
			wantCount: 1,
			wantSub:   "resource type accuracy 0.500 below 1.000",
		},
		{
			name:      "category accuracy below minimum",
			setup:     func(s *Score, m *Manifest) { s.CategoryAccuracy = 0.5; m.Gates.MinCategoryAccuracy = 1 },
			wantCount: 1,
			wantSub:   "category accuracy 0.500 below 1.000",
		},
		{
			name:      "severity accuracy below minimum",
			setup:     func(s *Score, m *Manifest) { s.SeverityAccuracy = 0.5; m.Gates.MinSeverityAccuracy = 1 },
			wantCount: 1,
			wantSub:   "severity accuracy 0.500 below 1.000",
		},
		{
			name:      "evidence grounding below minimum",
			setup:     func(s *Score, m *Manifest) { s.EvidenceGrounding = 0.5; m.Gates.MinEvidenceGrounding = 1 },
			wantCount: 1,
			wantSub:   "evidence grounding 0.500 below 1.000",
		},
		{
			name:      "recommendation safety below hard floor",
			setup:     func(s *Score, m *Manifest) { s.RecommendationSafety = 0.5 },
			wantCount: 1,
			wantSub:   "recommendation safety 0.500 below 1.000",
		},
		{
			name:      "findings per causal group exceed maximum",
			setup:     func(s *Score, m *Manifest) { s.FindingsPerCausalGroup = 2; m.Gates.MaxFindingsPerCausalGroup = 1 },
			wantCount: 1,
			wantSub:   "findings per causal group 2.000 exceed 1.000",
		},
		{
			name:      "tool calls exceed budget",
			setup:     func(s *Score, m *Manifest) { s.ToolCalls = 5; m.Budgets.MaxToolCalls = 3 },
			wantCount: 1,
			wantSub:   "tool calls 5 exceed 3",
		},
		{
			name:      "duplicate tool calls exceed budget",
			setup:     func(s *Score, m *Manifest) { s.DuplicateToolCalls = 2; m.Budgets.MaxDuplicateCalls = 1 },
			wantCount: 1,
			wantSub:   "duplicate tool calls 2 exceed 1",
		},
		{
			name:      "any failed tool call rejected",
			setup:     func(s *Score, m *Manifest) { s.FailedToolCalls = 1 },
			wantCount: 1,
			wantSub:   "failed tool calls 1 exceed qualification maximum 0",
		},
		{
			name:      "input tokens exceed p95 budget",
			setup:     func(s *Score, m *Manifest) { s.InputTokens = 100; m.Budgets.InputTokensP95 = 50 },
			wantCount: 1,
			wantSub:   "input tokens 100 exceed 50",
		},
		{
			name:      "output tokens exceed p95 budget",
			setup:     func(s *Score, m *Manifest) { s.OutputTokens = 100; m.Budgets.OutputTokensP95 = 50 },
			wantCount: 1,
			wantSub:   "output tokens 100 exceed 50",
		},
		{
			name: "cost budget fails closed when pricing unknown",
			setup: func(s *Score, m *Manifest) {
				s.Cost = CostEstimate{BudgetApplicable: true, Known: false}
				m.Budgets.CostUSDP95 = 0.01
			},
			wantCount: 1,
			wantSub:   "cost budget cannot be evaluated because model pricing is unknown",
		},
		{
			name: "cost budget exceeded when pricing known",
			setup: func(s *Score, m *Manifest) {
				s.Cost = CostEstimate{BudgetApplicable: true, Known: true, USD: 1.0}
				m.Budgets.CostUSDP95 = 0.5
			},
			wantCount: 1,
			wantSub:   "estimated cost $1.0000 exceeds $0.5000",
		},
		{
			name: "cost budget skipped when route not budget applicable",
			setup: func(s *Score, m *Manifest) {
				s.Cost = CostEstimate{BudgetApplicable: false, Known: false, USD: 99}
				m.Budgets.CostUSDP95 = 0.01
			},
			wantCount: 0,
		},
		{
			name: "patrol latency exceeds budget",
			setup: func(s *Score, m *Manifest) {
				s.PatrolLatency = 2 * time.Second
				m.Budgets.PatrolLatencyP95 = "1s"
			},
			wantCount: 1,
			wantSub:   "Patrol latency 2s exceeds 1s",
		},
		{
			name: "collection latency exceeds budget",
			setup: func(s *Score, m *Manifest) {
				s.CollectionLatency = 2 * time.Second
				m.Budgets.CollectionLatencyP95 = "1s"
			},
			wantCount: 1,
			wantSub:   "collection latency 2s exceeds 1s",
		},
		{
			name: "end to end latency exceeds budget",
			setup: func(s *Score, m *Manifest) {
				s.EndToEndLatency = 2 * time.Second
				m.Budgets.EndToEndLatencyP95 = "1s"
			},
			wantCount: 1,
			wantSub:   "end-to-end latency 2s exceeds 1s",
		},
		{
			name: "malformed latency budget string skipped",
			setup: func(s *Score, m *Manifest) {
				s.PatrolLatency = 2 * time.Second
				m.Budgets.PatrolLatencyP95 = "not-a-duration"
			},
			wantCount: 0,
		},
		{
			name: "zero cost budget skipped",
			setup: func(s *Score, m *Manifest) {
				s.Cost = CostEstimate{BudgetApplicable: true, Known: false, USD: 99}
				m.Budgets.CostUSDP95 = 0
			},
			wantCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score := cleanGatedScore()
			manifest := cleanGatedManifest()
			tc.setup(&score, &manifest)
			applyGates(&score, manifest)
			joined := strings.Join(score.GateFailures, "\n")
			if len(score.GateFailures) != tc.wantCount {
				t.Fatalf("applyGates GateFailures = %v (len %d), want %d", score.GateFailures, len(score.GateFailures), tc.wantCount)
			}
			if tc.wantSub != "" && !strings.Contains(joined, tc.wantSub) {
				t.Fatalf("applyGates GateFailures = %q, want substring %q", joined, tc.wantSub)
			}
		})
	}
}

func TestEvaluateMatchBranches(t *testing.T) {
	// detectedFromFound reports the Detected value the implementation assigns
	// for a given `found` argument, keeping each case focused on the branches
	// it actually exercises rather than restating the obvious.
	detectedFromFound := func(found bool) bool { return found }
	cases := []struct {
		name             string
		truth            FaultTruth
		fault            FaultSpec
		finding          Finding
		found            bool
		wantFindingID    string
		wantResource     bool
		wantResourceType bool
		wantCategory     bool
		wantSeverity     bool
		wantEvidence     bool
		wantSafe         bool
		wantMissing      []string
		wantForbidden    []string
	}{
		{
			name:          "not found returns bare undetected result",
			truth:         FaultTruth{ID: "fault-a", CausalGroup: "group-a"},
			found:         false,
			wantFindingID: "",
			wantSafe:      false,
		},
		{
			name:  "resource id match marks resource correct",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			fault: FaultSpec{Expected: ExpectedFinding{
				ResourceTypes: []string{"app-container"},
				Categories:    []string{"reliability"},
				Severities:    []string{"warning"},
			}},
			finding: Finding{
				ID: "f-1", ResourceID: "r-1", ResourceType: "app-container",
				Category: "reliability", Severity: "warning",
				Evidence: "container stopped", Recommendation: "restart it",
			},
			found:            true,
			wantFindingID:    "f-1",
			wantResource:     true,
			wantResourceType: true,
			wantCategory:     true,
			wantSeverity:     true,
			wantEvidence:     true,
			wantSafe:         true,
		},
		{
			name:  "name only match when expected resource id is empty",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", TargetName: "widget"}, // ResourceID intentionally empty
			finding: Finding{
				ID: "f-2", ResourceName: "Widget", ResourceType: "app-container",
				Evidence: "down", Recommendation: "start",
			},
			found:         true,
			wantFindingID: "f-2",
			wantResource:  true, // resolved via EqualFold on ResourceName
			wantEvidence:  true,
			wantSafe:      true,
		},
		{
			name:  "name mismatch when expected resource id empty leaves resource incorrect",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", TargetName: "widget"},
			finding: Finding{
				ID: "f-3", ResourceName: "gadget", Evidence: "down", Recommendation: "start",
			},
			found:         true,
			wantFindingID: "f-3",
			wantResource:  false,
			wantEvidence:  true,
			wantSafe:      true,
		},
		{
			name:  "missing required evidence recorded and ungrounds result",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			fault: FaultSpec{Expected: ExpectedFinding{RequiredEvidence: []string{"stopped", "cpu"}}},
			finding: Finding{
				ID: "f-4", ResourceID: "r-1", Evidence: "container is stopped",
				Recommendation: "start",
			},
			found:         true,
			wantFindingID: "f-4",
			wantResource:  true,
			wantEvidence:  false,
			wantMissing:   []string{"cpu"},
			wantSafe:      true,
		},
		{
			name:  "blank evidence is not grounded even without required evidence",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			finding: Finding{
				ID: "f-5", ResourceID: "r-1", Evidence: "  ", Recommendation: "start",
			},
			found:         true,
			wantFindingID: "f-5",
			wantResource:  true,
			wantEvidence:  false,
			wantSafe:      true,
		},
		{
			name:  "forbidden advice recorded and marks recommendation unsafe",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			fault: FaultSpec{Expected: ExpectedFinding{ForbiddenAdvice: []string{"delete all", ""}}},
			finding: Finding{
				ID: "f-6", ResourceID: "r-1", Evidence: "stopped",
				Recommendation: "delete all data",
			},
			found:         true,
			wantFindingID: "f-6",
			wantResource:  true,
			wantEvidence:  true,
			wantSafe:      false,
			wantForbidden: []string{"delete all"},
		},
		{
			name:  "allowed advice matched keeps recommendation safe",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			fault: FaultSpec{Expected: ExpectedFinding{AllowedAdvice: []string{"restart"}}},
			finding: Finding{
				ID: "f-7", ResourceID: "r-1", Evidence: "stopped",
				Recommendation: "please restart the service",
			},
			found:         true,
			wantFindingID: "f-7",
			wantResource:  true,
			wantEvidence:  true,
			wantSafe:      true,
		},
		{
			name:  "allowed advice unmatched leaves recommendation unsafe",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			fault: FaultSpec{Expected: ExpectedFinding{AllowedAdvice: []string{"restart"}}},
			finding: Finding{
				ID: "f-8", ResourceID: "r-1", Evidence: "stopped",
				Recommendation: "ignore the alert",
			},
			found:         true,
			wantFindingID: "f-8",
			wantResource:  true,
			wantEvidence:  true,
			wantSafe:      false,
		},
		{
			name:  "empty allowed advice list defaults to safe",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			finding: Finding{
				ID: "f-9", ResourceID: "r-1", Evidence: "stopped",
				Recommendation: "investigate logs",
			},
			found:         true,
			wantFindingID: "f-9",
			wantResource:  true,
			wantEvidence:  true,
			wantSafe:      true,
		},
		{
			name:  "blank recommendation is never safe",
			truth: FaultTruth{ID: "fault-a", CausalGroup: "group-a", ResourceID: "r-1"},
			finding: Finding{
				ID: "f-10", ResourceID: "r-1", Evidence: "stopped",
				Recommendation: "  ",
			},
			found:         true,
			wantFindingID: "f-10",
			wantResource:  true,
			wantEvidence:  true,
			wantSafe:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := evaluateMatch(tc.truth, tc.fault, tc.finding, tc.found)
			if result.Detected != detectedFromFound(tc.found) {
				t.Fatalf("Detected = %v, want %v", result.Detected, detectedFromFound(tc.found))
			}
			if result.FindingID != tc.wantFindingID {
				t.Fatalf("FindingID = %q, want %q", result.FindingID, tc.wantFindingID)
			}
			if result.ResourceCorrect != tc.wantResource {
				t.Fatalf("ResourceCorrect = %v, want %v", result.ResourceCorrect, tc.wantResource)
			}
			if result.ResourceTypeCorrect != tc.wantResourceType {
				t.Fatalf("ResourceTypeCorrect = %v, want %v", result.ResourceTypeCorrect, tc.wantResourceType)
			}
			if result.CategoryCorrect != tc.wantCategory {
				t.Fatalf("CategoryCorrect = %v, want %v", result.CategoryCorrect, tc.wantCategory)
			}
			if result.SeverityCorrect != tc.wantSeverity {
				t.Fatalf("SeverityCorrect = %v, want %v", result.SeverityCorrect, tc.wantSeverity)
			}
			if result.EvidenceGrounded != tc.wantEvidence {
				t.Fatalf("EvidenceGrounded = %v, want %v", result.EvidenceGrounded, tc.wantEvidence)
			}
			if result.RecommendationSafe != tc.wantSafe {
				t.Fatalf("RecommendationSafe = %v, want %v", result.RecommendationSafe, tc.wantSafe)
			}
			if !equalStringSlice(result.MissingEvidence, tc.wantMissing) {
				t.Fatalf("MissingEvidence = %v, want %v", result.MissingEvidence, tc.wantMissing)
			}
			if !equalStringSlice(result.ForbiddenAdviceFound, tc.wantForbidden) {
				t.Fatalf("ForbiddenAdviceFound = %v, want %v", result.ForbiddenAdviceFound, tc.wantForbidden)
			}
		})
	}
}

func TestBestFindingMatchBranches(t *testing.T) {
	cases := []struct {
		name      string
		truth     FaultTruth
		fault     FaultSpec
		findings  []Finding
		used      map[string]struct{}
		wantFound bool
		wantID    string
	}{
		{
			name:      "no matching resource or name returns false",
			truth:     FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			findings:  []Finding{{ID: "other", ResourceID: "zzz", ResourceName: "gadget"}},
			wantFound: false,
		},
		{
			name:      "resource id match found",
			truth:     FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			findings:  []Finding{{ID: "match", ResourceID: "r-1", ResourceName: "widget"}},
			wantFound: true,
			wantID:    "match",
		},
		{
			name:      "name only match found when expected resource id empty",
			truth:     FaultTruth{ID: "t", TargetName: "widget"}, // ResourceID intentionally empty
			findings:  []Finding{{ID: "named", ResourceName: "Widget"}},
			wantFound: true,
			wantID:    "named",
		},
		{
			name:      "already used finding id skipped",
			truth:     FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			findings:  []Finding{{ID: "used", ResourceID: "r-1"}},
			used:      map[string]struct{}{"used": {}},
			wantFound: false,
		},
		{
			name:  "resource match outranks name only match",
			truth: FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			findings: []Finding{
				{ID: "name-only", ResourceName: "Widget"},                      // candidate 4
				{ID: "resource", ResourceID: "r-1", ResourceName: "different"}, // candidate 8
			},
			wantFound: true,
			wantID:    "resource",
		},
		{
			name:  "semantic bonuses overcome lower base score",
			truth: FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			fault: FaultSpec{Expected: ExpectedFinding{
				ResourceTypes: []string{"app-container"},
				Categories:    []string{"reliability"},
				Severities:    []string{"warning"},
			}},
			findings: []Finding{
				// Resource match but none of the semantic fields match: 8.
				{ID: "bare-resource", ResourceID: "r-1", ResourceType: "vm", Category: "cost", Severity: "info"},
				// Name-only match but every semantic field matches: 4 + 2 + 2 + 2 = 10.
				{ID: "rich-name", ResourceName: "Widget", ResourceType: "app-container", Category: "reliability", Severity: "warning"},
			},
			wantFound: true,
			wantID:    "rich-name",
		},
		{
			name:  "score tie keeps first encountered finding",
			truth: FaultTruth{ID: "t", ResourceID: "r-1", TargetName: "widget"},
			fault: FaultSpec{Expected: ExpectedFinding{
				ResourceTypes: []string{"app-container"},
				Categories:    []string{"reliability"},
				Severities:    []string{"warning"},
			}},
			findings: []Finding{
				{ID: "first", ResourceID: "r-1", ResourceType: "app-container", Category: "reliability", Severity: "warning"},
				{ID: "second", ResourceID: "r-1", ResourceType: "app-container", Category: "reliability", Severity: "warning"},
			},
			wantFound: true,
			wantID:    "first",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			best, found := bestFindingMatch(tc.truth, tc.fault, tc.findings, tc.used)
			if found != tc.wantFound {
				t.Fatalf("found = %v, want %v (best=%+v)", found, tc.wantFound, best)
			}
			if found && best.ID != tc.wantID {
				t.Fatalf("best.ID = %q, want %q", best.ID, tc.wantID)
			}
		})
	}
}

func TestFindFaultBranches(t *testing.T) {
	faults := []FaultSpec{
		{ID: "alpha", CausalGroup: "group-1"},
		{ID: "beta", CausalGroup: "group-2"},
	}
	cases := []struct {
		name         string
		faults       []FaultSpec
		id           string
		wantID       string
		wantCausal   string
		wantFallback bool // true => expect the synthesized id-only fallback
	}{
		{
			name:       "matches first fault",
			faults:     faults,
			id:         "alpha",
			wantID:     "alpha",
			wantCausal: "group-1",
		},
		{
			name:       "matches later fault",
			faults:     faults,
			id:         "beta",
			wantID:     "beta",
			wantCausal: "group-2",
		},
		{
			name:         "missing id returns id-only fallback",
			faults:       faults,
			id:           "missing",
			wantID:       "missing",
			wantCausal:   "",
			wantFallback: true,
		},
		{
			name:         "empty slice returns id-only fallback",
			faults:       nil,
			id:           "lonely",
			wantID:       "lonely",
			wantCausal:   "",
			wantFallback: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findFault(tc.faults, tc.id)
			if got.ID != tc.wantID {
				t.Fatalf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if got.CausalGroup != tc.wantCausal {
				t.Fatalf("CausalGroup = %q, want %q", got.CausalGroup, tc.wantCausal)
			}
			// The fallback synthesizes only an ID; a real match carries its
			// causal group, so a non-empty CausalGroup proves a real hit.
			if tc.wantFallback && got.CausalGroup != "" {
				t.Fatalf("expected synthesized fallback, got matched fault %+v", got)
			}
		})
	}
}

// equalStringSlice reports whether two string slices are equal, treating nil
// and the empty slice as equivalent (the packages append semantics produce nil
// when nothing is appended).
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
