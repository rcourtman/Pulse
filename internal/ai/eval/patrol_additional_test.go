package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeFindingsFromToolCalls(t *testing.T) {
	// 1. Basic case: API findings only
	apiFindings := []PatrolFinding{
		{ID: "f1", Title: "F1"},
	}
	merged := mergeFindingsFromToolCalls(apiFindings, nil)
	assert.Len(t, merged, 1)
	assert.Equal(t, "f1", merged[0].ID)

	// 2. Case: Tool calls provide new finding
	toolCalls := []ToolCallEvent{
		{
			Name:   "patrol_get_findings",
			Output: `{"findings": [{"id": "f2", "title": "F2"}]}`,
		},
	}
	merged = mergeFindingsFromToolCalls(apiFindings, toolCalls)
	assert.Len(t, merged, 2)

	// Convert to map for easy checking
	fMap := make(map[string]string)
	for _, f := range merged {
		fMap[f.ID] = f.Title
	}
	assert.Equal(t, "F1", fMap["f1"])
	assert.Equal(t, "F2", fMap["f2"])

	// 3. Case: Tool calls provide duplicate finding (should dedupe)
	toolCallsWithDupe := []ToolCallEvent{
		{
			Name:   "patrol_get_findings",
			Output: `{"findings": [{"id": "f1", "title": "F1 (New)"}]}`,
		},
	}
	merged = mergeFindingsFromToolCalls(apiFindings, toolCallsWithDupe)
	assert.Len(t, merged, 1)
	assert.Equal(t, "F1", merged[0].Title) // Should keep original API version

	// 4. Case: Malformed tool output
	toolCallsMalformed := []ToolCallEvent{
		{
			Name:   "patrol_get_findings",
			Output: `invalid json`,
		},
	}
	merged = mergeFindingsFromToolCalls(apiFindings, toolCallsMalformed)
	assert.Len(t, merged, 1)
}

func TestRunner_ParsePatrolSSEStream(t *testing.T) {
	runner := &Runner{config: DefaultConfig()}

	tests := []struct {
		name            string
		input           string
		expectedEvents  int
		expectedTools   int
		expectedContent string
	}{
		{
			name: "Basic flow",
			input: `data: {"type": "start"}
data: {"type": "content", "content": "Running checks"}
data: {"type": "tool_start", "tool_id": "t1", "tool_name": "scan", "tool_input": "all"}
data: {"type": "tool_end", "tool_id": "t1", "tool_output": "ok", "tool_success": true}
data: {"type": "complete"}
`,
			expectedEvents:  5,
			expectedTools:   1,
			expectedContent: "Running checks",
		},
		{
			name: "Phase updates",
			input: `data: {"type": "phase", "phase": "init"}
data: {"type": "phase", "phase": "scan"}
data: {"type": "complete"}
`,
			expectedEvents: 3,
		},
		{
			name: "Interleaved tool call",
			input: `data: {"type": "tool_start", "tool_id": "t1", "tool_name": "n1", "tool_input": "i1"}
data: {"type": "tool_start", "tool_id": "t2", "tool_name": "n2", "tool_input": "i2"}
data: {"type": "tool_end", "tool_id": "t1", "tool_output": "o1", "tool_success": true}
data: {"type": "tool_end", "tool_id": "t2", "tool_output": "o2", "tool_success": false}
data: {"type": "complete"}
`,
			expectedTools:  2,
			expectedEvents: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, tools, content, err := runner.parsePatrolSSEStream(context.Background(), strings.NewReader(tc.input))
			require.NoError(t, err)
			assert.Len(t, events, tc.expectedEvents)
			assert.Len(t, tools, tc.expectedTools)
			if tc.expectedContent != "" {
				assert.Equal(t, tc.expectedContent, content)
			}
		})
	}
}
