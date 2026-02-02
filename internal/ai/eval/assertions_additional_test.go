package eval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAssertions(t *testing.T) {
	tests := []struct {
		name      string
		assertion Assertion
		result    StepResult
		passed    bool
	}{
		// AssertToolUsed
		{
			name:      "AssertToolUsed Pass",
			assertion: AssertToolUsed("test_tool"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "test_tool"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolUsed Fail",
			assertion: AssertToolUsed("test_tool"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "other_tool"}},
			},
			passed: false,
		},

		// AssertToolNotUsed
		{
			name:      "AssertToolNotUsed Pass",
			assertion: AssertToolNotUsed("test_tool"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "other_tool"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolNotUsed Fail",
			assertion: AssertToolNotUsed("test_tool"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "test_tool"}},
			},
			passed: false,
		},

		// AssertAnyToolUsed
		{
			name:      "AssertAnyToolUsed Pass",
			assertion: AssertAnyToolUsed(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool"}},
			},
			passed: true,
		},
		{
			name:      "AssertAnyToolUsed Fail",
			assertion: AssertAnyToolUsed(),
			result:    StepResult{},
			passed:    false,
		},

		// AssertNoToolErrors
		{
			name:      "AssertNoToolErrors Pass",
			assertion: AssertNoToolErrors(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool", Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertNoToolErrors Fail",
			assertion: AssertNoToolErrors(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool", Success: false, Output: "err"}},
			},
			passed: false,
		},

		// AssertContentContains
		{
			name:      "AssertContentContains Pass",
			assertion: AssertContentContains("hello"),
			result: StepResult{
				Content: "Hello world",
			},
			passed: true,
		},
		{
			name:      "AssertContentContains Fail",
			assertion: AssertContentContains("hello"),
			result: StepResult{
				Content: "Hi world",
			},
			passed: false,
		},

		// AssertContentContainsAny
		{
			name:      "AssertContentContainsAny Pass",
			assertion: AssertContentContainsAny("hello", "hi"),
			result: StepResult{
				Content: "Hi world",
			},
			passed: true,
		},
		{
			name:      "AssertContentContainsAny Fail",
			assertion: AssertContentContainsAny("hello", "hi"),
			result: StepResult{
				Content: "Greetings world",
			},
			passed: false,
		},

		// AssertContentNotContains
		{
			name:      "AssertContentNotContains Pass",
			assertion: AssertContentNotContains("error"),
			result: StepResult{
				Content: "All good",
			},
			passed: true,
		},
		{
			name:      "AssertContentNotContains Fail",
			assertion: AssertContentNotContains("error"),
			result: StepResult{
				Content: "An error occurred",
			},
			passed: false,
		},

		// AssertNoPhantomDetection
		{
			name:      "AssertNoPhantomDetection Pass (No phantom)",
			assertion: AssertNoPhantomDetection(),
			result: StepResult{
				Content: "Normal response",
			},
			passed: true,
		},
		{
			name:      "AssertNoPhantomDetection Pass (Phantom but recovered)",
			assertion: AssertNoPhantomDetection(),
			result: StepResult{
				Content:   "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request... Here is the data.",
				ToolCalls: []ToolCallEvent{{Name: "tool", Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertNoPhantomDetection Fail",
			assertion: AssertNoPhantomDetection(),
			result: StepResult{
				Content:   "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request",
				ToolCalls: []ToolCallEvent{{Name: "tool", Success: false}},
			},
			passed: false,
		},

		// AssertToolOutputContains
		{
			name:      "AssertToolOutputContains Pass",
			assertion: AssertToolOutputContains("tool1", "success"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool1", Output: "Operation success"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolOutputContains Fail (Content mismatch)",
			assertion: AssertToolOutputContains("tool1", "success"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool1", Output: "Operation failed"}},
			},
			passed: false,
		},
		{
			name:      "AssertToolOutputContains Fail (Tool not called)",
			assertion: AssertToolOutputContains("tool1", "success"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool2", Output: "Operation success"}},
			},
			passed: false,
		},

		// AssertNoError
		{
			name:      "AssertNoError Pass",
			assertion: AssertNoError(),
			result:    StepResult{Error: nil},
			passed:    true,
		},
		{
			name:      "AssertNoError Fail",
			assertion: AssertNoError(),
			result:    StepResult{Error: fmt.Errorf("fail")},
			passed:    false,
		},

		// AssertDurationUnder
		{
			name:      "AssertDurationUnder Pass",
			assertion: AssertDurationUnder("1s"),
			result:    StepResult{Duration: 500 * time.Millisecond},
			passed:    true,
		},
		{
			name:      "AssertDurationUnder Fail",
			assertion: AssertDurationUnder("1s"),
			result:    StepResult{Duration: 2 * time.Second},
			passed:    false,
		},

		// AssertToolNotBlocked
		{
			name:      "AssertToolNotBlocked Pass",
			assertion: AssertToolNotBlocked(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool", Output: "ok"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolNotBlocked Fail",
			assertion: AssertToolNotBlocked(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool", Output: `{"blocked":true}`}},
			},
			passed: false,
		},
		{
			name:      "AssertToolNotBlocked Fail (Routing Mismatch)",
			assertion: AssertToolNotBlocked(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "tool", Output: "ROUTING_MISMATCH"}},
			},
			passed: false,
		},

		// AssertEventualSuccess
		{
			name:      "AssertEventualSuccess Pass",
			assertion: AssertEventualSuccess(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: false}, {Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertEventualSuccess Fail",
			assertion: AssertEventualSuccess(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: false}, {Success: false}},
			},
			passed: false,
		},

		// AssertEventualSuccessOrApproval
		{
			name:      "AssertEventualSuccessOrApproval Pass (Success)",
			assertion: AssertEventualSuccessOrApproval(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertEventualSuccessOrApproval Pass (Approval)",
			assertion: AssertEventualSuccessOrApproval(),
			result: StepResult{
				Approvals: []ApprovalEvent{{ApprovalID: "1"}},
			},
			passed: true,
		},
		{
			name:      "AssertEventualSuccessOrApproval Fail",
			assertion: AssertEventualSuccessOrApproval(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: false}},
				Approvals: []ApprovalEvent{},
			},
			passed: false,
		},

		// AssertMinToolCalls
		{
			name:      "AssertMinToolCalls Pass",
			assertion: AssertMinToolCalls(2),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{}, {}},
			},
			passed: true,
		},
		{
			name:      "AssertMinToolCalls Fail",
			assertion: AssertMinToolCalls(2),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{}},
			},
			passed: false,
		},

		// AssertMaxInputTokens
		{
			name:      "AssertMaxInputTokens Pass",
			assertion: AssertMaxInputTokens(100),
			result:    StepResult{InputTokens: 50},
			passed:    true,
		},
		{
			name:      "AssertMaxInputTokens Fail",
			assertion: AssertMaxInputTokens(100),
			result:    StepResult{InputTokens: 150},
			passed:    false,
		},

		// AssertMaxToolCalls
		{
			name:      "AssertMaxToolCalls Pass",
			assertion: AssertMaxToolCalls(2),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{}, {}},
			},
			passed: true,
		},
		{
			name:      "AssertMaxToolCalls Fail",
			assertion: AssertMaxToolCalls(2),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{}, {}, {}},
			},
			passed: false,
		},

		// AssertHasContent
		{
			name:      "AssertHasContent Pass",
			assertion: AssertHasContent(),
			result:    StepResult{Content: "This assumes content length check is > 50 chars... so let's make it long enough to pass the check defined in assertions.go"},
			passed:    true,
		},
		{
			name:      "AssertHasContent Fail",
			assertion: AssertHasContent(),
			result:    StepResult{Content: "Too short"},
			passed:    false,
		},

		// AssertModelRecovered
		{
			name:      "AssertModelRecovered Pass (No blocks)",
			assertion: AssertModelRecovered(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertModelRecovered Pass (Blocked then success)",
			assertion: AssertModelRecovered(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: false}, {Success: true}},
			},
			passed: true,
		},
		{
			name:      "AssertModelRecovered Fail",
			assertion: AssertModelRecovered(),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Success: false}, {Success: false}},
			},
			passed: false,
		},

		// AssertToolSequence
		{
			name:      "AssertToolSequence Pass",
			assertion: AssertToolSequence([]string{"t1", "t2"}),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1"}, {Name: "other"}, {Name: "t2"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolSequence Fail",
			assertion: AssertToolSequence([]string{"t1", "t2"}),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t2"}, {Name: "t1"}},
			},
			passed: false,
		},

		// AssertToolInputContains
		{
			name:      "AssertToolInputContains Pass",
			assertion: AssertToolInputContains("t1", "val"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1", Input: "some val here"}},
			},
			passed: true,
		},
		{
			name:      "AssertToolInputContains Fail",
			assertion: AssertToolInputContains("t1", "val"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1", Input: "nothing"}},
			},
			passed: false,
		},

		// AssertAnyToolInputContains
		{
			name:      "AssertAnyToolInputContains Pass",
			assertion: AssertAnyToolInputContains("", "val"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1", Input: "val"}},
			},
			passed: true,
		},

		// AssertAnyToolInputContainsAny
		{
			name:      "AssertAnyToolInputContainsAny Pass",
			assertion: AssertAnyToolInputContainsAny("", "val", "foo"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1", Input: "foo"}},
			},
			passed: true,
		},

		// AssertToolOutputContainsAny
		{
			name:      "AssertToolOutputContainsAny Pass",
			assertion: AssertToolOutputContainsAny("t1", "success", "ok"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "t1", Output: "status: ok"}},
			},
			passed: true,
		},

		// AssertApprovalRequested
		{
			name:      "AssertApprovalRequested Pass",
			assertion: AssertApprovalRequested(),
			result:    StepResult{Approvals: []ApprovalEvent{{}}},
			passed:    true,
		},

		// AssertOnlyToolsUsed
		{
			name:      "AssertOnlyToolsUsed Pass",
			assertion: AssertOnlyToolsUsed("a", "b"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "a"}, {Name: "b"}},
			},
			passed: true,
		},
		{
			name:      "AssertOnlyToolsUsed Fail",
			assertion: AssertOnlyToolsUsed("a"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{{Name: "b"}},
			},
			passed: false,
		},

		// AssertRoutingMismatchRecovered
		{
			name:      "AssertRoutingMismatchRecovered Pass (Mismatch -> Recovery)",
			assertion: AssertRoutingMismatchRecovered("node1", "cont1"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{
					{Output: "routing_mismatch"},
					{Input: "target cont1"},
				},
			},
			passed: true,
		},
		{
			name:      "AssertRoutingMismatchRecovered Pass (No Mismatch -> Target Node)",
			assertion: AssertRoutingMismatchRecovered("node1", "cont1"),
			result: StepResult{
				ToolCalls: []ToolCallEvent{
					{Input: "target node1"},
				},
			},
			passed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.assertion(&tc.result)
			assert.Equal(t, tc.passed, res.Passed, "Message: %s", res.Message)
		})
	}
}
