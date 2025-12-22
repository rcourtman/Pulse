package ai

import (
	"testing"
)

func TestCleanThinkingTokens_DeepSeekMarkers(t *testing.T) {
	input := `## Analysis Summary

<｜end▁of▁thinking｜>

Now, also consider the duplicate PBS services. Let's add an info finding.<｜end▁of▁thinking｜>



Now, also check for any storage growth concerns: frigate-storage rising slow.

After comprehensive analysis of your infrastructure, I identified several issues.

### Key Findings:

1. **Critical CPU overload on Tower host**`

	result := cleanThinkingTokens(input)
	
	// Should NOT contain thinking markers
	if contains(result, "<｜end▁of▁thinking｜>") {
		t.Errorf("cleanThinkingTokens() should have removed DeepSeek thinking markers")
	}
	
	// Should NOT contain internal reasoning
	if contains(result, "Now, also consider") || contains(result, "Let's add an info") {
		t.Errorf("cleanThinkingTokens() should have removed internal reasoning")
	}
	
	// Should still contain the actual content
	if !contains(result, "## Analysis Summary") {
		t.Errorf("cleanThinkingTokens() removed header")
	}
	if !contains(result, "### Key Findings") {
		t.Errorf("cleanThinkingTokens() removed findings section")
	}
	if !contains(result, "Critical CPU overload") {
		t.Errorf("cleanThinkingTokens() removed actual finding")
	}
}

func TestCleanThinkingTokens_ASCIIVariant(t *testing.T) {
	input := `Some content<|end_of_thinking|>

Now, let's check something.

## Real Content`

	result := cleanThinkingTokens(input)
	
	if result != "## Real Content" {
		t.Errorf("cleanThinkingTokens() failed for ASCII variant: got %q", result)
	}
}

func TestCleanThinkingTokens_ReasoningPatterns(t *testing.T) {
	input := `## Analysis

Let's check the CPU usage.

Now, I need to look at memory.

### Findings

- Issue 1`

	result := cleanThinkingTokens(input)
	
	// Should remove the reasoning lines but keep the findings
	if !contains(result, "## Analysis") || !contains(result, "### Findings") || !contains(result, "- Issue 1") {
		t.Errorf("cleanThinkingTokens() removed too much: got %q", result)
	}
	
	if contains(result, "Let's check") || contains(result, "Now, I need") {
		t.Errorf("cleanThinkingTokens() should have removed reasoning: got %q", result)
	}
}

func TestCleanThinkingTokens_EmptyString(t *testing.T) {
	result := cleanThinkingTokens("")
	if result != "" {
		t.Errorf("cleanThinkingTokens(\"\") should return empty string, got %q", result)
	}
}

func TestCleanThinkingTokens_NoMarkers(t *testing.T) {
	input := `## Clean Analysis

This is a normal response without any thinking tokens.

### Findings

1. Issue one
2. Issue two`

	result := cleanThinkingTokens(input)
	
	// Should be mostly unchanged (just trimmed)
	if result != input {
		t.Errorf("cleanThinkingTokens() modified clean content:\nGot: %q\nExpected: %q", result, input)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
