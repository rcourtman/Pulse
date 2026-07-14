package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFormatSessionCompactionToolCall exercises every branch of
// formatSessionCompactionToolCall: empty/nil input maps, the JSON marshal
// success and failure paths, secret redaction on both Input and Output, name
// trimming, output sanitization (including whitespace-only and emoji-stripped
// outputs), and the tool-output truncation boundary at
// sessionCompactionToolOutputMaxChars.
func TestFormatSessionCompactionToolCall(t *testing.T) {
	const toolOutputBudget = sessionCompactionToolOutputMaxChars // 1200
	longOutput := strings.Repeat("a", toolOutputBudget+100)
	boundaryOutput := strings.Repeat("a", toolOutputBudget)
	truncatedTail := "\n[truncated]"

	cases := []struct {
		name     string
		toolCall ToolCall
		want     string
	}{
		{
			name:     "zero value renders empty name and default input object literal",
			toolCall: ToolCall{},
			want:     "Tool call:  {}",
		},
		{
			name: "name is trimmed and default input object literal is kept",
			toolCall: ToolCall{
				Name: "  list_pods  ",
			},
			want: "Tool call: list_pods {}",
		},
		{
			name: "nil input leaves default object literal",
			toolCall: ToolCall{
				Name:  "ping",
				Input: nil,
			},
			want: "Tool call: ping {}",
		},
		{
			name: "empty input map leaves default object literal",
			toolCall: ToolCall{
				Name:  "ping",
				Input: map[string]interface{}{},
			},
			want: "Tool call: ping {}",
		},
		{
			name: "single key input is marshaled to compact json",
			toolCall: ToolCall{
				Name:  "read_file",
				Input: map[string]interface{}{"path": "/etc/hosts"},
			},
			want: `Tool call: read_file {"path":"/etc/hosts"}`,
		},
		{
			name: "multiple input keys are emitted in sorted json key order",
			toolCall: ToolCall{
				Name: "provision",
				Input: map[string]interface{}{
					"zebra": "1",
					"apple": "2",
					"mango": "3",
				},
			},
			want: `Tool call: provision {"apple":"2","mango":"3","zebra":"1"}`,
		},
		{
			name: "secret bearing input value is redacted after marshal",
			toolCall: ToolCall{
				Name: "login",
				Input: map[string]interface{}{
					"api_key": "sk-live-secret-value",
				},
			},
			want: `Tool call: login {"api_key":"[REDACTED]"}`,
		},
		{
			name: "input that fails to marshal falls back to default object literal",
			toolCall: ToolCall{
				Name:  "broken",
				Input: map[string]interface{}{"bad": make(chan int)},
			},
			want: "Tool call: broken {}",
		},
		{
			name: "output only is sanitized and rendered on its own line",
			toolCall: ToolCall{
				Output: "pod-1234 running",
			},
			want: "Tool call:  {}\nTool output: pod-1234 running",
		},
		{
			name: "whitespace only output produces no output line",
			toolCall: ToolCall{
				Name:   "noop",
				Output: "   \n\t  ",
			},
			want: "Tool call: noop {}",
		},
		{
			name: "empty output produces no output line",
			toolCall: ToolCall{
				Name:   "noop",
				Output: "",
			},
			want: "Tool call: noop {}",
		},
		{
			name: "secret bearing output value is redacted",
			toolCall: ToolCall{
				Name:   "fetch",
				Output: "api_key: sk-live-secret-value",
			},
			want: "Tool call: fetch {}\nTool output: api_key: [REDACTED]",
		},
		{
			name: "decorative emoji prefix in output is stripped by sanitization",
			toolCall: ToolCall{
				Output: "🟢 All good",
			},
			want: "Tool call:  {}\nTool output: All good",
		},
		{
			name: "output surrounding whitespace is trimmed",
			toolCall: ToolCall{
				Output: "\n  health: ok  \n",
			},
			want: "Tool call:  {}\nTool output: health: ok",
		},
		{
			name: "output exceeding the char budget is truncated",
			toolCall: ToolCall{
				Output: longOutput,
			},
			want: "Tool call:  {}\nTool output: " + boundaryOutput + truncatedTail,
		},
		{
			name: "output exactly at the char budget is not truncated",
			toolCall: ToolCall{
				Output: boundaryOutput,
			},
			want: "Tool call:  {}\nTool output: " + boundaryOutput,
		},
		{
			name: "name input and output are all rendered together",
			toolCall: ToolCall{
				Name:   "run_check",
				Input:  map[string]interface{}{"host": "vault.lan"},
				Output: "ok",
			},
			want: `Tool call: run_check {"host":"vault.lan"}` + "\nTool output: ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatSessionCompactionToolCall(tc.toolCall)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestFormatSessionCompactionToolCallDeterminism asserts that repeated calls
// with the same input produce byte-identical output (json.Marshal key sorting
// and redaction are stable).
func TestFormatSessionCompactionToolCallDeterminism(t *testing.T) {
	toolCall := ToolCall{
		Name:   "deploy",
		Input:  map[string]interface{}{"image": "nginx:1.25", "replicas": 3},
		Output: "deployments/apps Deployment created",
	}
	first := formatSessionCompactionToolCall(toolCall)
	for i := 0; i < 10; i++ {
		require.Equal(t, first, formatSessionCompactionToolCall(toolCall),
			"iteration %d produced non-deterministic output", i)
	}
	want := `Tool call: deploy {"image":"nginx:1.25","replicas":3}` + "\nTool output: deployments/apps Deployment created"
	require.Equal(t, want, first)
}
