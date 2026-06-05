package chat

import (
	"strings"
	"testing"
)

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
		// deepseek-v4-flash emits the double-pipe variant. Found by
		// exercising pulse_summarize in real chat — the user saw the
		// raw DSML as the assistant's "final response" because the
		// fast-path only listed single-pipe.
		{"DeepSeek DSML double-pipe unicode", "Summary done<｜｜DSML｜｜tool_calls>more", "Summary done"},
		{"DeepSeek DSML double-pipe ascii", "Result<||DSML||tool_calls>extra", "Result"},
		{"DeepSeek DSML triple-pipe regex backstop", "Output<｜｜｜DSML｜｜｜function_call>", "Output"},
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

		// Plain-JSON tool-call leak — qwen2.5 small models route the
		// invocation into content instead of through tool_calls.
		// Natural-language prose before the leak must be preserved.
		{
			"qwen2.5 leak: pulse_query get include",
			"Para obtener la IP de cada nodo, necesito especificar el ID del recurso. Sin embargo, puedo intentar obtener la información de todos los nodos de una vez.\n\n{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\",\"include\":\"ip\"}}",
			"Para obtener la IP de cada nodo, necesito especificar el ID del recurso. Sin embargo, puedo intentar obtener la información de todos los nodos de una vez.",
		},
		{
			"qwen2.5 leak: pulse_discovery get_node_ips",
			"Voy a recoger las IPs de los nodos ahora.\n{\"name\": \"pulse_discovery\", \"parameters\": {\"action\":\"get_node_ips\"}}",
			"Voy a recoger las IPs de los nodos ahora.",
		},
		{
			"qwen2.5 leak: pulse_query resource_id",
			"Looking up resource 233.\n{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\",\"resource_id\":\"233\"}}",
			"Looking up resource 233.",
		},
		{
			"qwen2.5 leak with code-fence wrapper",
			"Here's the call I want to make:\n```json\n{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\"}}\n```",
			"Here's the call I want to make:",
		},
		{
			"qwen2.5 leak only no prose",
			"{\"name\": \"pulse_query\", \"parameters\": {\"action\":\"get\"}}",
			"",
		},
		{
			"plain function leak: pulse_read",
			"Let me inspect the device nodes.\npulse_read(target_host=\"current_resource\", command=\"ls /dev | wc -l\")",
			"Let me inspect the device nodes.",
		},
		{
			"plain function leak inline after prose",
			"I will check that now. pulse_read(target_host=\"current_resource\", command=\"lsblk\")",
			"I will check that now.",
		},
		{
			"plain function leak with compacted internal prelude",
			"I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.pulse_read(target_host=\"current_resource\", command=\"lsblk\")",
			"",
		},
		{
			"plain function leak only no prose",
			"pulse_read(target_host=\"current_resource\", command=\"lsblk\")",
			"",
		},
		// Negative: unrelated JSON the user might share — no "name" field
		// matching an allowlisted tool, so the sanitiser leaves it alone.
		{
			"negative: unrelated JSON object",
			"Here's some data: {\"foo\": \"bar\"}",
			"Here's some data: {\"foo\": \"bar\"}",
		},
		{
			"negative: JSON with name but not a tool",
			"Resource: {\"name\": \"my-vm\", \"cpu\": 50}",
			"Resource: {\"name\": \"my-vm\", \"cpu\": 50}",
		},
		{
			"negative: name field not in allowlist",
			"Strange output: {\"name\": \"frobnicate\", \"parameters\": {}}",
			"Strange output: {\"name\": \"frobnicate\", \"parameters\": {}}",
		},
		// Edge: tool name appears as substring in prose — no JSON
		// structure, so the regex does not fire.
		{
			"edge: tool name as substring in prose",
			"You can call the pulse_query tool to look up resources.",
			"You can call the pulse_query tool to look up resources.",
		},
		{
			"negative: unknown function-style call",
			"Call helper(target_host=\"current_resource\") in the example.",
			"Call helper(target_host=\"current_resource\") in the example.",
		},
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
		{"DeepSeek DSML double-pipe unicode", "text<｜｜DSML｜｜func>", true},
		{"DeepSeek DSML double-pipe ascii", "text<||DSML||func>", true},
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

		// Plain-JSON tool-call leak detection during streaming.
		{"json leak pulse_query", "{\"name\": \"pulse_query\", \"parameters\": {}}", true},
		{"json leak pulse_discovery", "prose\n{\"name\": \"pulse_discovery\", \"parameters\": {}}", true},
		{"json leak with code-fence", "answer\n```json\n{\"name\": \"pulse_query\"}", true},
		{"plain function leak pulse_read", "pulse_read(target_host=\"current_resource\", command=\"lsblk\")", true},
		{"plain function leak after prose", "text. pulse_read(target_host=\"current_resource\")", true},
		{"json non-tool name", "{\"name\": \"frobnicate\", \"parameters\": {}}", false},
		{"json unrelated object", "{\"foo\": \"bar\"}", false},
		{"tool name as prose substring", "the pulse_query tool is useful", false},
		{"unknown function-style call", "helper(target_host=\"current_resource\")", false},
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

func TestAppendVisibleContentBeforeToolLeak(t *testing.T) {
	var builder strings.Builder
	var pending string
	delta, leakFound := appendVisibleContentBeforeToolLeak(&builder, &pending, "Let me check. pu")
	if leakFound {
		t.Fatal("partial tool name should not be treated as a leak yet")
	}
	if delta != "Let me check. " || builder.String() != "Let me check. " || pending != "pu" {
		t.Fatalf("unexpected first delta=%q builder=%q pending=%q", delta, builder.String(), pending)
	}

	delta, leakFound = appendVisibleContentBeforeToolLeak(
		&builder,
		&pending,
		"lse_read(target_host=\"current_resource\", command=\"lsblk\")",
	)
	if !leakFound {
		t.Fatal("expected split plain function call to be detected")
	}
	if delta != "" {
		t.Fatalf("expected no visible delta once split call completed, got %q", delta)
	}
	if builder.String() != "Let me check. " || pending != "" {
		t.Fatalf("builder should not append leaked call suffix, got builder=%q pending=%q", builder.String(), pending)
	}
}

func TestAppendVisibleContentBeforeToolLeak_DropsCompactedPrelude(t *testing.T) {
	var builder strings.Builder
	var pending string

	delta, leakFound := appendVisibleContentBeforeToolLeak(
		&builder,
		&pending,
		"I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices.pulse_read(target_host=\"current_resource\", command=\"lsblk\")",
	)
	if !leakFound {
		t.Fatal("expected compacted raw function call to be detected")
	}
	if delta != "" || builder.String() != "" || pending != "" {
		t.Fatalf("compacted prelude should be suppressed, got delta=%q builder=%q pending=%q", delta, builder.String(), pending)
	}
}

func TestAppendVisibleContentBeforeToolLeak_ClearsCompactedPreludeWhenSplit(t *testing.T) {
	var builder strings.Builder
	var pending string

	first := "I'llcheckthedevicenodesinsidethecontainertoanswerthat.Letmecounttheentriesin/devandlisttheblockdevices."
	delta, leakFound := appendVisibleContentBeforeToolLeak(&builder, &pending, first)
	if leakFound {
		t.Fatal("first compacted chunk should not be classified as a tool leak by itself")
	}
	if delta != first || builder.String() != first {
		t.Fatalf("unexpected first compacted delta=%q builder=%q", delta, builder.String())
	}

	delta, leakFound = appendVisibleContentBeforeToolLeak(
		&builder,
		&pending,
		"pulse_read(target_host=\"current_resource\", command=\"lsblk\")",
	)
	if !leakFound {
		t.Fatal("expected split compacted raw function call to be detected")
	}
	if delta != "" || builder.String() != "" || pending != "" {
		t.Fatalf("compacted prelude should be cleared after split leak, got delta=%q builder=%q pending=%q", delta, builder.String(), pending)
	}
}
