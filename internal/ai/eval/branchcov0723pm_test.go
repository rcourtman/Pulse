package eval

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

// This file adds catalog-invariant and branch-coverage tests for the scenario
// constructors in scenarios.go and patrol_scenarios.go. The constructors are
// pure (they read env vars and assemble structs), so every target is exercised
// directly without a network, SSH, daemon, or database.
//
// These tests assert real invariants that would catch a malformed scenario
// added later (unique names, populated required fields, runnable assertions,
// well-formed tool references, and the conditional append branches). They do
// NOT echo per-field literals of each constructor.

var branchcov0723pmToolTokenRe = regexp.MustCompile(`(?:pulse|patrol)_[a-z0-9_]+`)

// branchcov0723pmScenarioCtors is the complete catalog of public Scenario
// constructors in scenarios.go. Parity with the source file is enforced by
// TestBranchcov0723pm_ScenarioCatalogParity, which scans scenarios.go itself -
// the length assertion alone would be circular, since it only reads this table.
func branchcov0723pmScenarioCtors() []struct {
	name string
	fn   func() Scenario
} {
	return []struct {
		name string
		fn   func() Scenario
	}{
		{"ReadOnlyInfrastructureScenario", ReadOnlyInfrastructureScenario},
		{"RoutingValidationScenario", RoutingValidationScenario},
		{"RoutingMismatchRecoveryScenario", RoutingMismatchRecoveryScenario},
		{"LogTailingScenario", LogTailingScenario},
		{"ReadOnlyViolationRecoveryScenario", ReadOnlyViolationRecoveryScenario},
		{"SearchByIDScenario", SearchByIDScenario},
		{"AmbiguousResourceDisambiguationScenario", AmbiguousResourceDisambiguationScenario},
		{"ContextTargetCarryoverScenario", ContextTargetCarryoverScenario},
		{"ResourceContextHandoffScenario", ResourceContextHandoffScenario},
		{"DiscoveryScenario", DiscoveryScenario},
		{"QuickSmokeTest", QuickSmokeTest},
		{"TroubleshootingScenario", TroubleshootingScenario},
		{"DeepDiveScenario", DeepDiveScenario},
		{"ConfigInspectionScenario", ConfigInspectionScenario},
		{"ResourceAnalysisScenario", ResourceAnalysisScenario},
		{"MultiNodeScenario", MultiNodeScenario},
		{"DockerInDockerScenario", DockerInDockerScenario},
		{"ContextChainScenario", ContextChainScenario},
		{"WriteVerifyScenario", WriteVerifyScenario},
		{"ReadOnlyEnforcementScenario", ReadOnlyEnforcementScenario},
		{"StrictResolutionScenario", StrictResolutionScenario},
		{"StrictResolutionRecoveryScenario", StrictResolutionRecoveryScenario},
		{"StrictResolutionBlockScenario", StrictResolutionBlockScenario},
		{"ApprovalScenario", ApprovalScenario},
		{"ApprovalComboScenario", ApprovalComboScenario},
		{"ApprovalApproveScenario", ApprovalApproveScenario},
		{"ApprovalDenyScenario", ApprovalDenyScenario},
		{"GuestControlStopScenario", GuestControlStopScenario},
		{"GuestControlIdempotentScenario", GuestControlIdempotentScenario},
		{"GuestControlDiscoveryScenario", GuestControlDiscoveryScenario},
		{"GuestControlNaturalLanguageScenario", GuestControlNaturalLanguageScenario},
		{"GuestControlMultiMentionScenario", GuestControlMultiMentionScenario},
		{"ReadOnlyModelChoiceScenario", ReadOnlyModelChoiceScenario},
		{"ReadLoopRecoveryScenario", ReadLoopRecoveryScenario},
		{"AmbiguousIntentScenario", AmbiguousIntentScenario},
		{"NonInteractiveGuardrailScenario", NonInteractiveGuardrailScenario},
	}
}

func branchcov0723pmRunStepAssertions(t *testing.T, step Step) []AssertionResult {
	t.Helper()
	zero := &StepResult{}
	out := make([]AssertionResult, 0, len(step.Assertions))
	for i, a := range step.Assertions {
		if a == nil {
			t.Fatalf("nil Assertion at index %d in step %q", i, step.Name)
		}
		out = append(out, a(zero))
	}
	return out
}

func branchcov0723pmNameSet(results []AssertionResult) map[string]int {
	m := make(map[string]int, len(results))
	for _, r := range results {
		m[r.Name]++
	}
	return m
}

func branchcov0723pmHasName(m map[string]int, name string) bool { return m[name] > 0 }

func branchcov0723pmRunPatrolAssertions(t *testing.T, ps PatrolScenario) []AssertionResult {
	t.Helper()
	zero := &PatrolRunResult{}
	out := make([]AssertionResult, 0, len(ps.Assertions))
	for i, a := range ps.Assertions {
		if a == nil {
			t.Fatalf("nil PatrolAssertion at index %d in %q", i, ps.Name)
		}
		out = append(out, a(zero))
	}
	return out
}

// branchcov0723pmKnownToolNames returns the canonical tool-name registry from
// internal/agentcapabilities. The eval package has no registry of its own, so
// this is the authoritative set referenced by the task.
func branchcov0723pmKnownToolNames() map[string]struct{} {
	return map[string]struct{}{
		agentcapabilities.PulseQueryToolName:                 {},
		agentcapabilities.PulseDiscoveryToolName:             {},
		agentcapabilities.PulseMetricsToolName:               {},
		agentcapabilities.PulseStorageToolName:               {},
		agentcapabilities.PulseDockerToolName:                {},
		agentcapabilities.PulseKubernetesToolName:            {},
		agentcapabilities.PulseAlertsToolName:                {},
		agentcapabilities.PulseReadToolName:                  {},
		agentcapabilities.PulseControlToolName:               {},
		agentcapabilities.PulseFileEditToolName:              {},
		agentcapabilities.PulseKnowledgeToolName:             {},
		agentcapabilities.PulsePMGToolName:                   {},
		agentcapabilities.PulseSummarizeToolName:             {},
		agentcapabilities.PulseRunCommandToolName:            {},
		agentcapabilities.PulseControlGuestToolName:          {},
		agentcapabilities.PulseControlDockerToolName:         {},
		agentcapabilities.PulseSearchResourcesToolName:       {},
		agentcapabilities.PulseGetResourceToolName:           {},
		agentcapabilities.PulseGetTopologyToolName:           {},
		agentcapabilities.PulseListInfrastructureToolName:    {},
		agentcapabilities.PulseGetConnectionHealthToolName:   {},
		agentcapabilities.PulseGetDockerLogsToolName:         {},
		agentcapabilities.PulseGetPerformanceMetricsToolName: {},
		agentcapabilities.PulseGetTemperaturesToolName:       {},
		agentcapabilities.PulseGetBaselinesToolName:          {},
		agentcapabilities.PulseGetPatternsToolName:           {},
		agentcapabilities.PatrolGetFindingsToolName:          {},
		agentcapabilities.PatrolAssessFindingToolName:        {},
		agentcapabilities.PatrolReportFindingToolName:        {},
		agentcapabilities.PatrolResolveFindingToolName:       {},
		agentcapabilities.PatrolProposeActionToolName:        {},
		agentcapabilities.PatrolActionCapabilitiesToolName:   {},
	}
}

// branchcov0723pmRegisteredOrPrefix reports whether tok is an exact registered
// tool name, and (separately) whether it is at least a prefix of one (a tool
// "family" reference such as pulse_file -> pulse_file_edit). A token that is
// neither is a genuine typo/malformation.
func branchcov0723pmRegisteredOrPrefix(tok string, known map[string]struct{}) (exact, prefix bool) {
	if _, ok := known[tok]; ok {
		return true, true
	}
	for name := range known {
		if strings.HasPrefix(name, tok) {
			return false, true
		}
	}
	return false, false
}

// TestBranchcov0723pm_ScenarioCatalogInvariants walks every Scenario
// constructor and asserts structural invariants that would catch a malformed
// scenario added later: non-empty unique names, populated descriptions, at
// least one step, well-formed steps (name/prompt/assertions), unique step
// names within a scenario, valid ApprovalDecision constants, well-formed
// mentions, and assertions that actually run (no nil/panic) and emit a
// non-empty Name+Message.
// branchcov0723pmSourceCtorRe matches an exported, zero-argument constructor
// returning a Scenario, which is the shape every entry in scenarios.go uses.
var branchcov0723pmSourceCtorRe = regexp.MustCompile(`(?m)^func ([A-Z][A-Za-z0-9]*)\(\) Scenario \{`)

// TestBranchcov0723pm_ScenarioCatalogParity makes the catalog table's
// completeness claim real: it reads scenarios.go and fails if the file declares
// a constructor the table does not list (or the table lists one the file no
// longer declares). Without this, adding a 37th constructor would silently go
// untested while every other assertion in this file still passed.
func TestBranchcov0723pm_ScenarioCatalogParity(t *testing.T) {
	src, err := os.ReadFile("scenarios.go")
	require.NoError(t, err, "scenarios.go must be readable from the package dir")

	var inSource []string
	for _, m := range branchcov0723pmSourceCtorRe.FindAllStringSubmatch(string(src), -1) {
		inSource = append(inSource, m[1])
	}
	require.NotEmpty(t, inSource, "regex must find constructors; update it if the source style changed")

	var inTable []string
	for _, c := range branchcov0723pmScenarioCtors() {
		inTable = append(inTable, c.name)
	}

	sort.Strings(inSource)
	sort.Strings(inTable)
	assert.Equal(t, inSource, inTable,
		"scenarios.go constructors and the catalog table must match exactly")
}

func TestBranchcov0723pm_ScenarioCatalogInvariants(t *testing.T) {
	ctors := branchcov0723pmScenarioCtors()
	require.Len(t, ctors, 36, "catalog table must list every scenario constructor")

	seen := make(map[string]string, len(ctors))

	for _, c := range ctors {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s := c.fn()

			require.NotEmpty(t, s.Name, "Scenario.Name must be non-empty")
			require.NotEmpty(t, strings.TrimSpace(s.Description), "Scenario.Description must be non-empty")
			require.NotEmpty(t, s.Steps, "Scenario must define at least one Step")

			if prev, dup := seen[s.Name]; dup {
				t.Fatalf("duplicate Scenario.Name %q (also produced by %s)", s.Name, prev)
			}
			seen[s.Name] = c.name

			stepNames := make(map[string]struct{}, len(s.Steps))
			for i, step := range s.Steps {
				require.NotEmpty(t, strings.TrimSpace(step.Name), "step %d: Name must be non-empty", i)
				require.NotEmpty(t, strings.TrimSpace(step.Prompt), "step %d (%s): Prompt must be non-empty", i, step.Name)
				require.NotEmpty(t, step.Assertions, "step %d (%s): must define Assertions", i, step.Name)

				if _, dup := stepNames[step.Name]; dup {
					t.Errorf("duplicate Step.Name %q within %s", step.Name, c.name)
				}
				stepNames[step.Name] = struct{}{}

				switch step.ApprovalDecision {
				case ApprovalNone, ApprovalApprove, ApprovalDeny:
				default:
					t.Errorf("step %d (%s): invalid ApprovalDecision %q", i, step.Name, step.ApprovalDecision)
				}

				for j, m := range step.Mentions {
					assert.NotEmpty(t, m.ID, "step %d mention %d: ID required", i, j)
					assert.NotEmpty(t, m.Name, "step %d mention %d: Name required", i, j)
					assert.NotEmpty(t, m.Type, "step %d mention %d: Type required", i, j)
				}

				for k, res := range branchcov0723pmRunStepAssertions(t, step) {
					assert.NotEmpty(t, res.Name, "step %d (%s) assertion %d: Name empty", i, step.Name, k)
					assert.NotEmpty(t, res.Message, "step %d (%s) assertion %d: Message empty", i, step.Name, k)
				}

				// Handoff resources, when present, must carry an identity.
				for j, hr := range step.HandoffResources {
					assert.NotEmpty(t, hr.ID, "step %d handoff %d: ID required", i, j)
					assert.NotEmpty(t, hr.Name, "step %d handoff %d: Name required", i, j)
				}
			}
		})
	}

	// Every catalog name must be distinct (cross-checked a second way for clarity).
	assert.Len(t, seen, len(ctors), "Scenario names must be unique across the catalog")
}

// TestBranchcov0723pm_ScenarioCatalogToolNames extracts every tool-name token
// referenced by the catalog's assertions (via their observable result Names)
// and asserts each is either an exact registered tool or a tool-family prefix.
// Tokens that are only a prefix (not an exact tool) are logged for review.
func TestBranchcov0723pm_ScenarioCatalogToolNames(t *testing.T) {
	known := branchcov0723pmKnownToolNames()
	zero := &StepResult{}

	referenced := make(map[string]struct{})
	for _, c := range branchcov0723pmScenarioCtors() {
		s := c.fn()
		for _, step := range s.Steps {
			for _, a := range step.Assertions {
				require.NotNil(t, a)
				res := a(zero)
				for _, raw := range branchcov0723pmToolTokenRe.FindAllString(res.Name, -1) {
					// Assertion Names embed the tool name followed by a
					// _contains / _contains_any suffix (e.g.
					// "tool_input:pulse_query_contains:..."). Strip those known
					// suffixes so the token resolves to the real tool name.
					tok := strings.TrimSuffix(raw, "_contains_any")
					tok = strings.TrimSuffix(tok, "_contains")
					referenced[tok] = struct{}{}
				}
			}
		}
	}
	require.NotEmpty(t, referenced, "expected the catalog to reference at least one tool")

	var prefixOnly, bogus []string
	for tok := range referenced {
		exact, prefix := branchcov0723pmRegisteredOrPrefix(tok, known)
		switch {
		case exact:
			// registered tool
		case prefix:
			prefixOnly = append(prefixOnly, tok)
		default:
			bogus = append(bogus, tok)
		}
	}

	sort.Strings(bogus)
	sort.Strings(prefixOnly)

	// Hard invariant: every referenced tool token is a real tool or a real
	// tool family. A bare typo (e.g. "pulse_qery") lands here and fails.
	assert.Empty(t, bogus, "referenced tool tokens are not registered tools or prefixes: %v", bogus)

	// Soft signal: tokens that are only a family prefix (not an exact tool)
	// are surfaced for review without failing the suite.
	if len(prefixOnly) > 0 {
		t.Logf("tool tokens referenced only as a family prefix (not exact registered tools): %v", prefixOnly)
	}
}

// --- Conditional-append branch coverage ---
//
// The constructors below append different assertions depending on env-driven
// evalTargets flags. Each subtest pins the relevant flag and asserts the
// observable difference (presence/absence of specific assertion result Names
// and exact counts), exercising both arms of every conditional.

func TestBranchcov0723pm_WriteVerifyScenario_Branches(t *testing.T) {
	t.Run("require_write_verify_false", func(t *testing.T) {
		ensureEnvUnset(t, "EVAL_REQUIRE_WRITE_VERIFY")
		s := WriteVerifyScenario()
		require.Len(t, s.Steps, 1)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		assert.Len(t, res, 4)
		assert.True(t, branchcov0723pmHasName(names, "no_error"))
		assert.True(t, branchcov0723pmHasName(names, "eventual_success"))
		assert.False(t, branchcov0723pmHasName(names, "tool_used:pulse_control"))
		assert.False(t, branchcov0723pmHasName(names, "tool_used:pulse_read"))
		assert.False(t, branchcov0723pmHasName(names, "tool_sequence"))
	})

	t.Run("require_write_verify_true", func(t *testing.T) {
		t.Setenv("EVAL_REQUIRE_WRITE_VERIFY", "true")
		s := WriteVerifyScenario()
		require.Len(t, s.Steps, 1)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		assert.Len(t, res, 7)
		assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_control"))
		assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_read"))
		assert.True(t, branchcov0723pmHasName(names, "tool_sequence"))
	})
}

func TestBranchcov0723pm_StrictResolutionScenario_Branches(t *testing.T) {
	cases := []struct {
		name             string
		strict, recovery bool
	}{
		{"both_false", false, false},
		{"strict_only", true, false},
		{"recovery_only", false, true},
		{"both_true", true, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.strict {
				t.Setenv("EVAL_STRICT_RESOLUTION", "true")
			} else {
				ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
			}
			if tc.recovery {
				t.Setenv("EVAL_REQUIRE_STRICT_RECOVERY", "true")
			} else {
				ensureEnvUnset(t, "EVAL_REQUIRE_STRICT_RECOVERY")
			}

			s := StrictResolutionScenario()
			require.Len(t, s.Steps, 2)

			step1 := branchcov0723pmRunStepAssertions(t, s.Steps[0])
			n1 := branchcov0723pmNameSet(step1)
			if tc.strict {
				assert.Len(t, step1, 3)
				assert.True(t, branchcov0723pmHasName(n1, "tool_output:pulse_control_contains_any"))
			} else {
				assert.Len(t, step1, 2)
				assert.False(t, branchcov0723pmHasName(n1, "tool_output:pulse_control_contains_any"))
			}

			step2 := branchcov0723pmRunStepAssertions(t, s.Steps[1])
			n2 := branchcov0723pmNameSet(step2)
			if tc.recovery {
				assert.Len(t, step2, 4)
				assert.True(t, branchcov0723pmHasName(n2, "tool_sequence"))
			} else {
				assert.Len(t, step2, 3)
				assert.False(t, branchcov0723pmHasName(n2, "tool_sequence"))
			}
		})
	}
}

func TestBranchcov0723pm_StrictResolutionRecoveryScenario_Branches(t *testing.T) {
	cases := []struct {
		name             string
		strict, recovery bool
		wantTotal        int
	}{
		{"both_false", false, false, 2},
		{"strict_only", true, false, 6},
		{"recovery_only", false, true, 3},
		{"both_true", true, true, 7},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.strict {
				t.Setenv("EVAL_STRICT_RESOLUTION", "true")
			} else {
				ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
			}
			if tc.recovery {
				t.Setenv("EVAL_REQUIRE_STRICT_RECOVERY", "true")
			} else {
				ensureEnvUnset(t, "EVAL_REQUIRE_STRICT_RECOVERY")
			}

			s := StrictResolutionRecoveryScenario()
			require.Len(t, s.Steps, 1)

			// Auto-deny is wired unconditionally so the eval never hangs.
			assert.Equal(t, ApprovalDeny, s.Steps[0].ApprovalDecision)
			assert.NotEmpty(t, s.Steps[0].ApprovalReason)

			res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
			names := branchcov0723pmNameSet(res)
			assert.Len(t, res, tc.wantTotal)

			if tc.strict {
				assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_control"))
				assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_query"))
				assert.True(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"))
				assert.True(t, branchcov0723pmHasName(names, "model_recovered"))
			} else {
				assert.False(t, branchcov0723pmHasName(names, "tool_used:pulse_control"))
				assert.False(t, branchcov0723pmHasName(names, "model_recovered"))
			}
			if tc.recovery {
				assert.True(t, branchcov0723pmHasName(names, "tool_sequence"))
			} else {
				assert.False(t, branchcov0723pmHasName(names, "tool_sequence"))
			}
		})
	}
}

func TestBranchcov0723pm_StrictResolutionBlockScenario_Branches(t *testing.T) {
	t.Run("strict_false_omits_output_assertion", func(t *testing.T) {
		ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
		s := StrictResolutionBlockScenario()
		require.Len(t, s.Steps, 1)
		assert.Equal(t, ApprovalDeny, s.Steps[0].ApprovalDecision)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		assert.Len(t, res, 4)
		assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_query"))
		assert.True(t, branchcov0723pmHasName(names, "tool_used:pulse_control"))
		assert.False(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"))
	})

	t.Run("strict_true_adds_output_assertion", func(t *testing.T) {
		t.Setenv("EVAL_STRICT_RESOLUTION", "true")
		s := StrictResolutionBlockScenario()
		require.Len(t, s.Steps, 1)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		assert.Len(t, res, 5)
		assert.True(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"))
	})

	// The local writeCmd defaulting mirrors approvalWriteCommand: an empty or
	// literal "true" command is rewritten to a safe touch target.
	t.Run("default_write_command_rewrites_to_touch", func(t *testing.T) {
		ensureEnvUnset(t, "EVAL_WRITE_COMMAND")
		ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
		s := StrictResolutionBlockScenario()
		require.Len(t, s.Steps, 1)
		assert.Contains(t, s.Steps[0].Prompt, "touch /tmp/pulse_eval_strict")
	})

	t.Run("explicit_true_write_command_rewrites_to_touch", func(t *testing.T) {
		t.Setenv("EVAL_WRITE_COMMAND", "true")
		ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
		s := StrictResolutionBlockScenario()
		require.Len(t, s.Steps, 1)
		assert.Contains(t, s.Steps[0].Prompt, "touch /tmp/pulse_eval_strict")
	})

	t.Run("custom_write_command_preserved", func(t *testing.T) {
		t.Setenv("EVAL_WRITE_COMMAND", "echo branchcov0723pm-custom")
		ensureEnvUnset(t, "EVAL_STRICT_RESOLUTION")
		s := StrictResolutionBlockScenario()
		require.Len(t, s.Steps, 1)
		assert.Contains(t, s.Steps[0].Prompt, "echo branchcov0723pm-custom")
		assert.NotContains(t, s.Steps[0].Prompt, "touch /tmp/pulse_eval_strict")
	})
}

// TestBranchcov0723pm_ApprovalFamily_Branches covers the ExpectApproval
// conditional in the four Approval constructors. When the flag is set, an
// approval_requested assertion is appended; ApprovalApprove/Deny/Combo also
// swap an eventual_success assertion for tool-output + approval assertions.
// The ApprovalDecision constants wired by each constructor are pinned too.
func TestBranchcov0723pm_ApprovalFamily_Branches(t *testing.T) {
	ctors := []struct {
		name string
		fn   func() Scenario
	}{
		{"ApprovalScenario", ApprovalScenario},
		{"ApprovalApproveScenario", ApprovalApproveScenario},
		{"ApprovalDenyScenario", ApprovalDenyScenario},
		{"ApprovalComboScenario", ApprovalComboScenario},
	}
	for _, c := range ctors {
		c := c
		t.Run(c.name+"/expect_approval_false", func(t *testing.T) {
			ensureEnvUnset(t, "EVAL_EXPECT_APPROVAL")
			s := c.fn()
			branchcov0723pmAssertApprovalBranch(t, s, c.name, false)
		})
		t.Run(c.name+"/expect_approval_true", func(t *testing.T) {
			t.Setenv("EVAL_EXPECT_APPROVAL", "true")
			s := c.fn()
			branchcov0723pmAssertApprovalBranch(t, s, c.name, true)
		})
	}
}

func branchcov0723pmAssertApprovalBranch(t *testing.T, s Scenario, ctorName string, expectApproval bool) {
	t.Helper()

	switch ctorName {
	case "ApprovalScenario":
		require.Len(t, s.Steps, 1)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		if expectApproval {
			assert.Len(t, res, 3)
			assert.True(t, branchcov0723pmHasName(names, "approval_requested"))
		} else {
			assert.Len(t, res, 2)
			assert.False(t, branchcov0723pmHasName(names, "approval_requested"))
			assert.False(t, branchcov0723pmHasName(names, "eventual_success"))
		}

	case "ApprovalApproveScenario":
		require.Len(t, s.Steps, 1)
		assert.Equal(t, ApprovalApprove, s.Steps[0].ApprovalDecision)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		if expectApproval {
			assert.Len(t, res, 4)
			assert.True(t, branchcov0723pmHasName(names, "approval_requested"))
			assert.True(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"))
			assert.False(t, branchcov0723pmHasName(names, "eventual_success"))
		} else {
			assert.Len(t, res, 3)
			assert.False(t, branchcov0723pmHasName(names, "approval_requested"))
			assert.True(t, branchcov0723pmHasName(names, "eventual_success"))
		}

	case "ApprovalDenyScenario":
		require.Len(t, s.Steps, 1)
		assert.Equal(t, ApprovalDeny, s.Steps[0].ApprovalDecision)
		res := branchcov0723pmRunStepAssertions(t, s.Steps[0])
		names := branchcov0723pmNameSet(res)
		if expectApproval {
			assert.Len(t, res, 4)
			assert.True(t, branchcov0723pmHasName(names, "approval_requested"))
			assert.True(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"))
			assert.False(t, branchcov0723pmHasName(names, "eventual_success"))
		} else {
			assert.Len(t, res, 3)
			assert.False(t, branchcov0723pmHasName(names, "approval_requested"))
			assert.True(t, branchcov0723pmHasName(names, "eventual_success"))
		}

	case "ApprovalComboScenario":
		require.Len(t, s.Steps, 2)
		assert.Equal(t, ApprovalApprove, s.Steps[0].ApprovalDecision)
		assert.Equal(t, ApprovalDeny, s.Steps[1].ApprovalDecision)
		for i, step := range s.Steps {
			res := branchcov0723pmRunStepAssertions(t, step)
			names := branchcov0723pmNameSet(res)
			if expectApproval {
				assert.Len(t, res, 4, "combo step %d", i)
				assert.True(t, branchcov0723pmHasName(names, "approval_requested"), "combo step %d", i)
				assert.True(t, branchcov0723pmHasName(names, "tool_output:pulse_control_contains_any"), "combo step %d", i)
				assert.False(t, branchcov0723pmHasName(names, "eventual_success"), "combo step %d", i)
			} else {
				assert.Len(t, res, 3, "combo step %d", i)
				assert.False(t, branchcov0723pmHasName(names, "approval_requested"), "combo step %d", i)
				assert.True(t, branchcov0723pmHasName(names, "eventual_success"), "combo step %d", i)
			}
		}
	}
}

// --- Patrol scenario catalog ---

func branchcov0723pmPatrolCtors() []struct {
	name string
	fn   func() PatrolScenario
} {
	return []struct {
		name string
		fn   func() PatrolScenario
	}{
		{"PatrolBasicScenario", PatrolBasicScenario},
		{"PatrolInvestigationScenario", PatrolInvestigationScenario},
		{"PatrolFindingQualityScenario", PatrolFindingQualityScenario},
		{"PatrolSignalCoverageScenario", PatrolSignalCoverageScenario},
	}
}

// TestBranchcov0723pm_PatrolScenarioCatalogInvariants asserts the structural
// invariants for every PatrolScenario constructor: non-empty unique names,
// populated descriptions, at least one assertion, and assertions that run on a
// zero result without panicking while emitting non-empty Name+Message.
func TestBranchcov0723pm_PatrolScenarioCatalogInvariants(t *testing.T) {
	ctors := branchcov0723pmPatrolCtors()
	require.Len(t, ctors, 4)

	seen := make(map[string]string, len(ctors))
	for _, c := range ctors {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ps := c.fn()
			require.NotEmpty(t, ps.Name)
			require.NotEmpty(t, strings.TrimSpace(ps.Description))
			require.NotEmpty(t, ps.Assertions, "PatrolScenario must define Assertions")

			if prev, dup := seen[ps.Name]; dup {
				t.Fatalf("duplicate PatrolScenario.Name %q (also produced by %s)", ps.Name, prev)
			}
			seen[ps.Name] = c.name

			for k, res := range branchcov0723pmRunPatrolAssertions(t, ps) {
				assert.NotEmpty(t, res.Name, "assertion %d: Name empty", k)
				assert.NotEmpty(t, res.Message, "assertion %d: Message empty", k)
			}
		})
	}
	assert.Len(t, seen, len(ctors), "PatrolScenario names must be unique")
}

// TestBranchcov0723pm_AllPatrolScenariosOrdering covers AllPatrolScenarios.
// Its per-entry contents are already covered by the catalog-invariant test
// above, and comparing them here against the same constructors AllPatrolScenarios
// itself calls would be circular, so this asserts only what is independently
// observable: the four entries, in declaration order, none dropped or repeated.
func TestBranchcov0723pm_AllPatrolScenariosOrdering(t *testing.T) {
	all := AllPatrolScenarios()
	require.Len(t, all, 4)

	// The only non-circular signal available here is ORDER and completeness:
	// comparing each entry field-by-field against the same constructors the
	// function itself calls would assert nothing. So assert the identity and
	// sequence of the four names, and that none is dropped or duplicated.
	gotNames := []string{all[0].Name, all[1].Name, all[2].Name, all[3].Name}
	assert.Equal(t, []string{
		PatrolBasicScenario().Name,
		PatrolInvestigationScenario().Name,
		PatrolFindingQualityScenario().Name,
		PatrolSignalCoverageScenario().Name,
	}, gotNames, "AllPatrolScenarios must return the four constructors in declaration order")

	seen := map[string]bool{}
	for _, s := range all {
		assert.False(t, seen[s.Name], "duplicate scenario %q in AllPatrolScenarios", s.Name)
		seen[s.Name] = true
		assert.NotEmpty(t, s.Assertions, "scenario %q must carry assertions", s.Name)
	}
}
