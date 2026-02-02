package eval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPatrolAssertions(t *testing.T) {
	tests := []struct {
		name      string
		assertion PatrolAssertion
		result    PatrolRunResult
		passed    bool
	}{
		// PatrolAssertNoError
		{
			name:      "PatrolAssertNoError Pass",
			assertion: PatrolAssertNoError(),
			result:    PatrolRunResult{Error: nil},
			passed:    true,
		},
		{
			name:      "PatrolAssertNoError Fail",
			assertion: PatrolAssertNoError(),
			result:    PatrolRunResult{Error: fmt.Errorf("fail")},
			passed:    false,
		},

		// PatrolAssertCompleted
		{
			name:      "PatrolAssertCompleted Pass (Status)",
			assertion: PatrolAssertCompleted(),
			result:    PatrolRunResult{Completed: true},
			passed:    true,
		},
		{
			name:      "PatrolAssertCompleted Pass (SSE Event)",
			assertion: PatrolAssertCompleted(),
			result:    PatrolRunResult{Completed: false, RawEvents: []PatrolSSEEvent{{Type: "complete"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertCompleted Fail",
			assertion: PatrolAssertCompleted(),
			result:    PatrolRunResult{Completed: false, RawEvents: []PatrolSSEEvent{}},
			passed:    false,
		},

		// PatrolAssertDurationUnder
		{
			name:      "PatrolAssertDurationUnder Pass",
			assertion: PatrolAssertDurationUnder(1 * time.Second),
			result:    PatrolRunResult{Duration: 500 * time.Millisecond},
			passed:    true,
		},
		{
			name:      "PatrolAssertDurationUnder Fail",
			assertion: PatrolAssertDurationUnder(1 * time.Second),
			result:    PatrolRunResult{Duration: 2 * time.Second},
			passed:    false,
		},

		// PatrolAssertToolUsed
		{
			name:      "PatrolAssertToolUsed Pass",
			assertion: PatrolAssertToolUsed("scan"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "scan"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertToolUsed Fail",
			assertion: PatrolAssertToolUsed("scan"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "check"}}},
			passed:    false,
		},

		// PatrolAssertToolUsedAny
		{
			name:      "PatrolAssertToolUsedAny Pass",
			assertion: PatrolAssertToolUsedAny("scan", "check"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "check"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertToolUsedAny Fail",
			assertion: PatrolAssertToolUsedAny("scan", "check"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "other"}}},
			passed:    false,
		},

		// PatrolAssertInvestigatedBeforeReporting
		{
			name:      "PatrolAssertInvestigatedBeforeReporting Pass (No report)",
			assertion: PatrolAssertInvestigatedBeforeReporting("scan"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "scan"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertInvestigatedBeforeReporting Pass (Investigated then Report)",
			assertion: PatrolAssertInvestigatedBeforeReporting("scan"),
			result: PatrolRunResult{ToolCalls: []ToolCallEvent{
				{Name: "scan"},
				{Name: "patrol_report_finding"},
			}},
			passed: true,
		},
		{
			name:      "PatrolAssertInvestigatedBeforeReporting Fail (Report without Investigation)",
			assertion: PatrolAssertInvestigatedBeforeReporting("scan"),
			result: PatrolRunResult{ToolCalls: []ToolCallEvent{
				{Name: "patrol_report_finding"},
			}},
			passed: false,
		},

		// PatrolAssertMinToolCalls
		{
			name:      "PatrolAssertMinToolCalls Pass",
			assertion: PatrolAssertMinToolCalls(1),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t1"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertMinToolCalls Fail",
			assertion: PatrolAssertMinToolCalls(2),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t1"}}},
			passed:    false,
		},

		// PatrolAssertNoToolErrors
		{
			name:      "PatrolAssertNoToolErrors Pass",
			assertion: PatrolAssertNoToolErrors(),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Success: true}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertNoToolErrors Fail",
			assertion: PatrolAssertNoToolErrors(),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Success: false}}},
			passed:    false,
		},

		// PatrolAssertToolSuccessRate
		{
			name:      "PatrolAssertToolSuccessRate Pass",
			assertion: PatrolAssertToolSuccessRate(0.5),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Success: true}, {Success: false}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertToolSuccessRate Fail",
			assertion: PatrolAssertToolSuccessRate(0.6),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Success: true}, {Success: false}}},
			passed:    false,
		},

		// PatrolAssertToolSequence
		{
			name:      "PatrolAssertToolSequence Pass",
			assertion: PatrolAssertToolSequence([]string{"t1", "t2"}),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t1"}, {Name: "t2"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertToolSequence Fail",
			assertion: PatrolAssertToolSequence([]string{"t1", "t2"}),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t2"}, {Name: "t1"}}},
			passed:    false,
		},

		// PatrolAssertToolInputContains
		{
			name:      "PatrolAssertToolInputContains Pass",
			assertion: PatrolAssertToolInputContains("t1", "val"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t1", Input: "val"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertToolInputContains Fail",
			assertion: PatrolAssertToolInputContains("t1", "val"),
			result:    PatrolRunResult{ToolCalls: []ToolCallEvent{{Name: "t1", Input: "other"}}},
			passed:    false,
		},

		// PatrolAssertHasFindings
		{
			name:      "PatrolAssertHasFindings Pass",
			assertion: PatrolAssertHasFindings(),
			result:    PatrolRunResult{Findings: []PatrolFinding{{ID: "f1"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertHasFindings Fail",
			assertion: PatrolAssertHasFindings(),
			result:    PatrolRunResult{Findings: []PatrolFinding{}},
			passed:    false,
		},

		// PatrolAssertFindingCount
		{
			name:      "PatrolAssertFindingCount Pass",
			assertion: PatrolAssertFindingCount(1, 2),
			result:    PatrolRunResult{Findings: []PatrolFinding{{ID: "f1"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertFindingCount Fail",
			assertion: PatrolAssertFindingCount(2, 3),
			result:    PatrolRunResult{Findings: []PatrolFinding{{ID: "f1"}}},
			passed:    false,
		},

		// PatrolAssertAllFindingsValid
		{
			name:      "PatrolAssertAllFindingsValid Pass",
			assertion: PatrolAssertAllFindingsValid(),
			result: PatrolRunResult{Findings: []PatrolFinding{{
				Key: "k", Severity: "high", Title: "t", Description: "d", ResourceType: "r",
			}}},
			passed: true,
		},
		{
			name:      "PatrolAssertAllFindingsValid Fail",
			assertion: PatrolAssertAllFindingsValid(),
			result: PatrolRunResult{Findings: []PatrolFinding{{
				Key: "k", // missing others
			}}},
			passed: false,
		},

		// PatrolAssertFindingSeveritiesValid
		{
			name:      "PatrolAssertFindingSeveritiesValid Pass",
			assertion: PatrolAssertFindingSeveritiesValid(),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Severity: "critical"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertFindingSeveritiesValid Fail",
			assertion: PatrolAssertFindingSeveritiesValid(),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Severity: "invalid"}}},
			passed:    false,
		},

		// PatrolAssertFindingCategoriesValid
		{
			name:      "PatrolAssertFindingCategoriesValid Pass",
			assertion: PatrolAssertFindingCategoriesValid(),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Category: "security"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertFindingCategoriesValid Fail",
			assertion: PatrolAssertFindingCategoriesValid(),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Category: "invalid"}}},
			passed:    false,
		},

		// PatrolAssertFindingWithKey
		{
			name:      "PatrolAssertFindingWithKey Pass",
			assertion: PatrolAssertFindingWithKey("k1"),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Key: "k1"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertFindingWithKey Fail",
			assertion: PatrolAssertFindingWithKey("k1"),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Key: "k2"}}},
			passed:    false,
		},

		// PatrolAssertNoFindingWithKey
		{
			name:      "PatrolAssertNoFindingWithKey Pass",
			assertion: PatrolAssertNoFindingWithKey("k1"),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Key: "k2"}}},
			passed:    true,
		},
		{
			name:      "PatrolAssertNoFindingWithKey Fail",
			assertion: PatrolAssertNoFindingWithKey("k1"),
			result:    PatrolRunResult{Findings: []PatrolFinding{{Key: "k1"}}},
			passed:    false,
		},

		// PatrolAssertReportFindingFieldsPresent
		{
			name:      "PatrolAssertReportFindingFieldsPresent Pass",
			assertion: PatrolAssertReportFindingFieldsPresent(),
			result: PatrolRunResult{ToolCalls: []ToolCallEvent{
				{
					Name:    "patrol_report_finding",
					Input:   `{"key":"k","severity":"s","title":"t","description":"d","resource_type":"r"}`,
					Success: true,
				},
			}},
			passed: true,
		},
		{
			name:      "PatrolAssertReportFindingFieldsPresent Fail",
			assertion: PatrolAssertReportFindingFieldsPresent(),
			result: PatrolRunResult{ToolCalls: []ToolCallEvent{
				{
					Name:    "patrol_report_finding",
					Input:   `{"key":"k"}`, // missing fields
					Success: true,
				},
			}},
			passed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.assertion(&tc.result)
			assert.Equal(t, tc.passed, res.Passed, "Message: %s", res.Message)
		})
	}
}
