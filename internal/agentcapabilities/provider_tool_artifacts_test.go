package agentcapabilities

import (
	"strings"
	"testing"
)

func TestProviderToolCallArtifactIndexDetectsSharedProviderLeakShapes(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{
		"pulse_query",
		"pulse_discovery",
		"pulse_read",
		PulseQuestionToolName,
	})

	tests := []struct {
		name      string
		input     string
		wantIndex int
	}{
		{
			name:      "DeepSeek DSML marker",
			input:     "Answer<|DSML|invoke>",
			wantIndex: strings.Index("Answer<|DSML|invoke>", "<|DSML|"),
		},
		{
			name:      "XML tool_call envelope",
			input:     "Answer\n<tool_call>{\"name\":\"pulse_query\"}</tool_call>",
			wantIndex: strings.Index("Answer\n<tool_call>{\"name\":\"pulse_query\"}</tool_call>", "<tool_call>"),
		},
		{
			name:      "pipe marker",
			input:     "Some text<|tool_call|>extra",
			wantIndex: strings.Index("Some text<|tool_call|>extra", "<|tool_call|>"),
		},
		{
			name:      "minimax marker",
			input:     "Response\nminimax:tool_call {\"name\":\"pulse_query\"}",
			wantIndex: strings.Index("Response\nminimax:tool_call {\"name\":\"pulse_query\"}", "minimax:tool_call"),
		},
		{
			name:      "JSON tool call leak preserves prose before newline",
			input:     "Looking up resource 233.\n{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\"}}",
			wantIndex: strings.Index("Looking up resource 233.\n{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\"}}", "\n"),
		},
		{
			name:      "JSON native Assistant question leak",
			input:     "Need one more detail.\n{\"name\":\"pulse_question\",\"input\":{\"questions\":[]}}",
			wantIndex: strings.Index("Need one more detail.\n{\"name\":\"pulse_question\",\"input\":{\"questions\":[]}}", "\n"),
		},
		{
			name:      "code fenced JSON tool call leak",
			input:     "Here's the call:\n```json\n{\"name\":\"pulse_discovery\",\"parameters\":{}}",
			wantIndex: strings.Index("Here's the call:\n```json\n{\"name\":\"pulse_discovery\",\"parameters\":{}}", "\n"),
		},
		{
			name:      "function-style tool call leak",
			input:     "I will check that now. pulse_read(target_host=\"current_resource\")",
			wantIndex: strings.Index("I will check that now. pulse_read(target_host=\"current_resource\")", "pulse_read"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProviderToolCallArtifactIndex(tt.input, catalog); got != tt.wantIndex {
				t.Fatalf("ProviderToolCallArtifactIndex() = %d, want %d", got, tt.wantIndex)
			}
		})
	}
}

func TestProviderToolCallArtifactIndexIgnoresNonToolJSONAndFunctions(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{"pulse_query", "pulse_read"})
	for _, input := range []string{
		`{"name": "my-vm", "cpu": 50}`,
		`{"name": "frobnicate", "parameters": {}}`,
		`{"foo": "bar"}`,
		`helper(target_host="current_resource")`,
		`the pulse_query tool is useful`,
	} {
		if got := ProviderToolCallArtifactIndex(input, catalog); got != -1 {
			t.Fatalf("ProviderToolCallArtifactIndex(%q) = %d, want -1", input, got)
		}
	}
}

func TestSplitTrailingProviderToolNamePrefixUsesSharedCatalog(t *testing.T) {
	catalog := NewProviderToolNameCatalog([]string{"pulse_query", "pulse_read", "patrol_report_finding"})

	tests := []struct {
		name        string
		input       string
		wantVisible string
		wantHeld    string
	}{
		{
			name:        "partial tool name is held",
			input:       "Let me check. pu",
			wantVisible: "Let me check. ",
			wantHeld:    "pu",
		},
		{
			name:        "complete tool name is held until next chunk proves prose or call",
			input:       "pulse_read",
			wantVisible: "",
			wantHeld:    "pulse_read",
		},
		{
			name:        "unknown trailing token remains visible",
			input:       "This is prose",
			wantVisible: "This is prose",
			wantHeld:    "",
		},
		{
			name:        "patrol prefix is held",
			input:       "Investigating patrol_rep",
			wantVisible: "Investigating ",
			wantHeld:    "patrol_rep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, held := SplitTrailingProviderToolNamePrefix(tt.input, catalog)
			if visible != tt.wantVisible || held != tt.wantHeld {
				t.Fatalf("SplitTrailingProviderToolNamePrefix() = (%q, %q), want (%q, %q)", visible, held, tt.wantVisible, tt.wantHeld)
			}
		})
	}
}
