package chat

import "testing"

func TestCleanToolCallArtifacts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"clean content unchanged", "Hello, how can I help?", "Hello, how can I help?"},
		{"DeepSeek DSML unicode", "Here is my answer<｜DSML｜function_calls>...", "Here is my answer"},
		{"DeepSeek DSML ascii", "Response text<|DSML|invoke>...", "Response text"},
		{"XML tool_call envelope", "My response\n<tool_call>{\"name\":\"foo\"}</tool_call>", "My response"},
		{"XML tool_calls envelope", "Answer here\n<tool_calls>\n{\"name\":\"bar\"}\n</tool_calls>", "Answer here"},
		{"XML function_call envelope", "Done.\n<function_call>test</function_call>", "Done."},
		{"XML function_calls envelope", "Result\n<function_calls>stuff</function_calls>", "Result"},
		{"pipe plugin marker", "Some text<|plugin|>extra", "Some text"},
		{"pipe interpreter marker", "Output<|interpreter|>code", "Output"},
		{"pipe tool_call marker", "Here<|tool_call|>data", "Here"},
		{"minimax marker", "Response\nminimax:tool_call {\"name\":\"x\"}", "Response"},
		{"only markers no content", "<tool_call>junk</tool_call>", ""},
		{"whitespace before marker trimmed", "Hello   \n<tool_call>x</tool_call>", "Hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanToolCallArtifacts(tt.input)
			if got != tt.expected {
				t.Errorf("cleanToolCallArtifacts(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestContainsToolCallMarker(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"clean content", "Hello world", false},
		{"normal angle brackets", "<div>html</div>", false},
		{"DeepSeek DSML unicode", "text<｜DSML｜func>", true},
		{"DeepSeek DSML ascii", "text<|DSML|func>", true},
		{"tool_call tag", "<tool_call>json</tool_call>", true},
		{"tool_calls tag", "<tool_calls>json</tool_calls>", true},
		{"function_call tag", "<function_call>json</function_call>", true},
		{"function_calls tag", "<function_calls>json</function_calls>", true},
		{"pipe plugin", "<|plugin|>", true},
		{"pipe interpreter", "<|interpreter|>", true},
		{"pipe tool_call", "<|tool_call|>", true},
		{"minimax marker", "minimax:tool_call {}", true},
		{"minimax in middle", "text\nminimax:tool_call x", true},
		{"partial minimax no match", "the minimax:tool_callx", false}, // \b prevents this
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsToolCallMarker(tt.input)
			if got != tt.expected {
				t.Errorf("containsToolCallMarker(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
