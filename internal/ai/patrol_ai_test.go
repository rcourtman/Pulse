package ai

import (
	"testing"
)

// --- DeepSeek marker tests ---

func TestCleanThinkingTokens_DeepSeek(t *testing.T) {
	input := "Some analysis\n<｜end▁of▁thinking｜>\nActual content here"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "end▁of▁thinking") {
		t.Errorf("DeepSeek thinking marker should be removed, got: %s", result)
	}
	if !containsSubstr(result, "Actual content here") {
		t.Errorf("Actual content should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_DeepSeekMarkers(t *testing.T) {
	input := `## Analysis Summary

<｜end▁of▁thinking｜>

Now, also consider the duplicate PBS services. Let's add an info finding.<｜end▁of▁thinking｜>



Now, also check for any storage growth concerns: frigate-storage rising slow.

After comprehensive analysis of your infrastructure, I identified several issues.

### Key Findings:

1. **Critical CPU overload on Tower host**`

	result := CleanThinkingTokens(input)

	if containsSubstr(result, "<｜end▁of▁thinking｜>") {
		t.Errorf("CleanThinkingTokens() should have removed DeepSeek thinking markers")
	}
	if containsSubstr(result, "Now, also consider") || containsSubstr(result, "Let's add an info") {
		t.Errorf("CleanThinkingTokens() should have removed internal reasoning")
	}
	if !containsSubstr(result, "## Analysis Summary") {
		t.Errorf("CleanThinkingTokens() removed header")
	}
	if !containsSubstr(result, "### Key Findings") {
		t.Errorf("CleanThinkingTokens() removed findings section")
	}
	if !containsSubstr(result, "Critical CPU overload") {
		t.Errorf("CleanThinkingTokens() removed actual finding")
	}
}

func TestCleanThinkingTokens_ASCIIVariant(t *testing.T) {
	input := `Some content<|end_of_thinking|>

Now, let's check something.

## Real Content`

	result := CleanThinkingTokens(input)

	if result != "## Real Content" {
		t.Errorf("CleanThinkingTokens() failed for ASCII variant: got %q", result)
	}
}

// --- Block-level tag tests ---

func TestCleanThinkingTokens_ThinkBlock(t *testing.T) {
	input := "Before content\n<think>This is internal reasoning\nthat spans multiple lines</think>\nAfter content"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "internal reasoning") {
		t.Errorf("<think> block content should be removed, got: %s", result)
	}
	if !containsSubstr(result, "Before content") {
		t.Errorf("Content before think block should be preserved, got: %s", result)
	}
	if !containsSubstr(result, "After content") {
		t.Errorf("Content after think block should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_ThoughtBlock(t *testing.T) {
	input := "Start\n<thought>Some deep thought process here</thought>\nEnd"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "deep thought") {
		t.Errorf("<thought> block content should be removed, got: %s", result)
	}
	if !containsSubstr(result, "Start") {
		t.Errorf("Content before thought block should be preserved, got: %s", result)
	}
	if !containsSubstr(result, "End") {
		t.Errorf("Content after thought block should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_ReasoningBlock(t *testing.T) {
	input := "Start\n<|reasoning|>Internal reasoning here<|/reasoning|>\nEnd"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "Internal reasoning") {
		t.Errorf("<|reasoning|> block content should be removed, got: %s", result)
	}
	if !containsSubstr(result, "Start") {
		t.Errorf("Content before reasoning block should be preserved, got: %s", result)
	}
	if !containsSubstr(result, "End") {
		t.Errorf("Content after reasoning block should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_CaseInsensitiveBlocks(t *testing.T) {
	input := "Before\n<THINK>uppercase thinking</THINK>\nAfter"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "uppercase thinking") {
		t.Errorf("Case-insensitive <THINK> block should be removed, got: %s", result)
	}
	if !containsSubstr(result, "After") {
		t.Errorf("Content after block should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_MultipleBlocks(t *testing.T) {
	input := "<think>first block</think>content between<think>second block</think>final content"
	result := CleanThinkingTokens(input)
	if containsSubstr(result, "first block") || containsSubstr(result, "second block") {
		t.Errorf("All think blocks should be removed, got: %s", result)
	}
	if !containsSubstr(result, "content between") || !containsSubstr(result, "final content") {
		t.Errorf("Content between blocks should be preserved, got: %s", result)
	}
}

func TestCleanThinkingTokens_UnclosedBlock(t *testing.T) {
	input := "Start content<think>unclosed block with no end"
	result := CleanThinkingTokens(input)
	if !containsSubstr(result, "Start content") {
		t.Errorf("Content before unclosed block should be preserved, got: %s", result)
	}
	if containsSubstr(result, "unclosed block") {
		t.Errorf("Unclosed block content should be removed, got: %s", result)
	}
}

// --- Reasoning pattern tests ---

func TestCleanThinkingTokens_ReasoningPatterns(t *testing.T) {
	input := `## Analysis

Let's check the CPU usage.

Now, I need to look at memory.

### Findings

- Issue 1`

	result := CleanThinkingTokens(input)

	if !containsSubstr(result, "## Analysis") || !containsSubstr(result, "### Findings") || !containsSubstr(result, "- Issue 1") {
		t.Errorf("CleanThinkingTokens() removed too much: got %q", result)
	}

	if containsSubstr(result, "Let's check") || containsSubstr(result, "Now, I need") {
		t.Errorf("CleanThinkingTokens() should have removed reasoning: got %q", result)
	}
}

// --- Edge case tests ---

func TestCleanThinkingTokens_NoMarkers(t *testing.T) {
	input := `## Clean Analysis

This is a normal response without any thinking tokens.

### Findings

1. Issue one
2. Issue two`

	result := CleanThinkingTokens(input)
	if result != input {
		t.Errorf("CleanThinkingTokens() modified clean content:\nGot: %q\nExpected: %q", result, input)
	}
}

func TestCleanThinkingTokens_EmptyContent(t *testing.T) {
	result := CleanThinkingTokens("")
	if result != "" {
		t.Errorf("Empty string should return empty, got: %q", result)
	}
}

// containsSubstr is a helper to check substring presence.
// Named to avoid conflict with any standard library additions.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCleanThinkingTokens_DeepSeekDSMLFormat(t *testing.T) {
	// This test verifies that DeepSeek's internal function call format is removed.
	// When DeepSeek doesn't properly use the function calling API, it outputs
	// its internal markup like <｜DSML｜function_calls> into visible responses.
	input := `I've successfully updated the services.yaml file to remove the InfluxDB entry. Let me verify the change:

<｜DSML｜function_calls>
<｜DSML｜invoke name="pulse_read">
<｜DSML｜parameter name="action" string="true">exec</｜DSML｜parameter>
<｜DSML｜parameter name="command" string="true">grep -A 10 "Monitoring:" /opt/homepage/config/services.yaml</｜DSML｜parameter>
<｜DSML｜parameter name="target_host" string="true">homepage-docker</｜DSML｜parameter>
</｜DSML｜invoke>
</｜DSML｜function_calls>`

	result := CleanThinkingTokens(input)

	expected := "I've successfully updated the services.yaml file to remove the InfluxDB entry. Let me verify the change:"
	if result != expected {
		t.Errorf("CleanThinkingTokens() failed to remove DeepSeek DSML format:\nGot: %q\nExpected: %q", result, expected)
	}
	if containsSubstr(result, "DSML") {
		t.Errorf("CleanThinkingTokens() should have removed all DSML markers, got: %s", result)
	}
	if containsSubstr(result, "pulse_read") {
		t.Errorf("CleanThinkingTokens() should have removed tool call content, got: %s", result)
	}
}

func TestCleanThinkingTokens_DeepSeekDSMLFormatASCII(t *testing.T) {
	// Test the ASCII variant of DSML markers
	input := `Command completed successfully.

<|DSML|function_calls>
<|DSML|invoke name="pulse_query">
<|DSML|parameter name="action">health</|DSML|parameter>
</|DSML|invoke>
</|DSML|function_calls>`

	result := CleanThinkingTokens(input)
	expected := "Command completed successfully."
	if result != expected {
		t.Errorf("CleanThinkingTokens() failed to remove ASCII DSML format:\nGot: %q\nExpected: %q", result, expected)
	}
}
