package eval

import (
	"fmt"
	"strings"
)

// === Common Assertions ===

// AssertToolUsed checks that a specific tool was called
func AssertToolUsed(toolName string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Name == toolName {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_used:%s", toolName),
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' was called", toolName),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_used:%s", toolName),
			Passed:  false,
			Message: fmt.Sprintf("Tool '%s' was NOT called. Tools used: %v", toolName, getToolNames(result.ToolCalls)),
		}
	}
}

// AssertAnyToolUsed checks that at least one tool was called
func AssertAnyToolUsed() Assertion {
	return func(result *StepResult) AssertionResult {
		if len(result.ToolCalls) > 0 {
			return AssertionResult{
				Name:    "any_tool_used",
				Passed:  true,
				Message: fmt.Sprintf("%d tool(s) called: %v", len(result.ToolCalls), getToolNames(result.ToolCalls)),
			}
		}
		return AssertionResult{
			Name:    "any_tool_used",
			Passed:  false,
			Message: "No tools were called",
		}
	}
}

// AssertNoToolErrors checks that all tool calls succeeded
func AssertNoToolErrors() Assertion {
	return func(result *StepResult) AssertionResult {
		var failures []string
		for _, tc := range result.ToolCalls {
			if !tc.Success {
				failures = append(failures, fmt.Sprintf("%s: %s", tc.Name, truncate(tc.Output, 100)))
			}
		}
		if len(failures) == 0 {
			return AssertionResult{
				Name:    "no_tool_errors",
				Passed:  true,
				Message: "All tool calls succeeded",
			}
		}
		return AssertionResult{
			Name:    "no_tool_errors",
			Passed:  false,
			Message: fmt.Sprintf("Tool failures: %v", failures),
		}
	}
}

// AssertContentContains checks that the response contains a substring
func AssertContentContains(substring string) Assertion {
	return func(result *StepResult) AssertionResult {
		if strings.Contains(strings.ToLower(result.Content), strings.ToLower(substring)) {
			return AssertionResult{
				Name:    fmt.Sprintf("content_contains:%s", truncate(substring, 20)),
				Passed:  true,
				Message: fmt.Sprintf("Content contains '%s'", substring),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("content_contains:%s", truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("Content does NOT contain '%s'", substring),
		}
	}
}

// AssertContentNotContains checks that the response does NOT contain a substring
func AssertContentNotContains(substring string) Assertion {
	return func(result *StepResult) AssertionResult {
		if !strings.Contains(strings.ToLower(result.Content), strings.ToLower(substring)) {
			return AssertionResult{
				Name:    fmt.Sprintf("content_not_contains:%s", truncate(substring, 20)),
				Passed:  true,
				Message: fmt.Sprintf("Content does not contain '%s'", substring),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("content_not_contains:%s", truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("Content SHOULD NOT contain '%s' but does", substring),
		}
	}
}

// AssertNoPhantomDetection checks that phantom detection did not trigger
func AssertNoPhantomDetection() Assertion {
	return func(result *StepResult) AssertionResult {
		// The exact phantom detection message from agentic.go
		phantomMessage := "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request"
		if strings.Contains(result.Content, phantomMessage) {
			// Find where in the content it appears
			idx := strings.Index(result.Content, phantomMessage)
			context := result.Content[max(0, idx-50):min(len(result.Content), idx+100)]
			return AssertionResult{
				Name:    "no_phantom_detection",
				Passed:  false,
				Message: fmt.Sprintf("Phantom detection triggered, found at: ...%s...", context),
			}
		}
		return AssertionResult{
			Name:    "no_phantom_detection",
			Passed:  true,
			Message: "No phantom detection",
		}
	}
}

// AssertToolOutputContains checks that a specific tool's output contains a substring
func AssertToolOutputContains(toolName, substring string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Name == toolName {
				if strings.Contains(strings.ToLower(tc.Output), strings.ToLower(substring)) {
					return AssertionResult{
						Name:    fmt.Sprintf("tool_output:%s_contains:%s", toolName, truncate(substring, 20)),
						Passed:  true,
						Message: fmt.Sprintf("Tool '%s' output contains '%s'", toolName, substring),
					}
				}
				return AssertionResult{
					Name:    fmt.Sprintf("tool_output:%s_contains:%s", toolName, truncate(substring, 20)),
					Passed:  false,
					Message: fmt.Sprintf("Tool '%s' output does NOT contain '%s'", toolName, substring),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_output:%s_contains:%s", toolName, truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("Tool '%s' was not called", toolName),
		}
	}
}

// AssertNoError checks that no execution error occurred
func AssertNoError() Assertion {
	return func(result *StepResult) AssertionResult {
		if result.Error == nil {
			return AssertionResult{
				Name:    "no_error",
				Passed:  true,
				Message: "No execution error",
			}
		}
		return AssertionResult{
			Name:    "no_error",
			Passed:  false,
			Message: fmt.Sprintf("Execution error: %v", result.Error),
		}
	}
}

// AssertDurationUnder checks that the step completed within a time limit
func AssertDurationUnder(maxDuration string) Assertion {
	return func(result *StepResult) AssertionResult {
		// Parse duration - simplified, just handle seconds for now
		var maxSec float64
		fmt.Sscanf(maxDuration, "%fs", &maxSec)
		if maxSec == 0 {
			fmt.Sscanf(maxDuration, "%f", &maxSec)
		}

		actualSec := result.Duration.Seconds()
		if actualSec <= maxSec {
			return AssertionResult{
				Name:    fmt.Sprintf("duration_under:%s", maxDuration),
				Passed:  true,
				Message: fmt.Sprintf("Completed in %.1fs (max: %.1fs)", actualSec, maxSec),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("duration_under:%s", maxDuration),
			Passed:  false,
			Message: fmt.Sprintf("Took %.1fs which exceeds max of %.1fs", actualSec, maxSec),
		}
	}
}

// AssertToolNotBlocked checks that no tools were blocked
func AssertToolNotBlocked() Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if strings.Contains(tc.Output, `"blocked":true`) ||
				strings.Contains(tc.Output, "ROUTING_MISMATCH") ||
				strings.Contains(tc.Output, "FSM_BLOCKED") ||
				strings.Contains(tc.Output, "READ_ONLY_VIOLATION") {
				return AssertionResult{
					Name:    "tool_not_blocked",
					Passed:  false,
					Message: fmt.Sprintf("Tool '%s' was blocked: %s", tc.Name, truncate(tc.Output, 100)),
				}
			}
		}
		return AssertionResult{
			Name:    "tool_not_blocked",
			Passed:  true,
			Message: "No tools were blocked",
		}
	}
}

// AssertEventualSuccess checks that at least one tool succeeded (allows intermediate failures)
// This is useful for complex workflows where some tools may be blocked but the model recovers.
func AssertEventualSuccess() Assertion {
	return func(result *StepResult) AssertionResult {
		successCount := 0
		for _, tc := range result.ToolCalls {
			if tc.Success {
				successCount++
			}
		}
		if successCount > 0 {
			return AssertionResult{
				Name:    "eventual_success",
				Passed:  true,
				Message: fmt.Sprintf("%d/%d tool calls succeeded", successCount, len(result.ToolCalls)),
			}
		}
		return AssertionResult{
			Name:    "eventual_success",
			Passed:  false,
			Message: "No tool calls succeeded",
		}
	}
}

// AssertMinToolCalls checks that at least N tools were called
func AssertMinToolCalls(min int) Assertion {
	return func(result *StepResult) AssertionResult {
		if len(result.ToolCalls) >= min {
			return AssertionResult{
				Name:    fmt.Sprintf("min_tool_calls:%d", min),
				Passed:  true,
				Message: fmt.Sprintf("%d tool calls made (min: %d)", len(result.ToolCalls), min),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("min_tool_calls:%d", min),
			Passed:  false,
			Message: fmt.Sprintf("Only %d tool calls made (expected at least %d)", len(result.ToolCalls), min),
		}
	}
}

// AssertHasContent checks that the assistant produced a non-empty response
func AssertHasContent() Assertion {
	return func(result *StepResult) AssertionResult {
		content := strings.TrimSpace(result.Content)
		if len(content) > 50 {
			return AssertionResult{
				Name:    "has_content",
				Passed:  true,
				Message: fmt.Sprintf("Response has %d characters", len(content)),
			}
		}
		return AssertionResult{
			Name:    "has_content",
			Passed:  false,
			Message: fmt.Sprintf("Response too short or empty (%d chars)", len(content)),
		}
	}
}

// AssertModelRecovered checks that if any tools were blocked, the model eventually succeeded
// with at least one tool call (indicating recovery from the block)
func AssertModelRecovered() Assertion {
	return func(result *StepResult) AssertionResult {
		blockedCount := 0
		successAfterBlock := false
		sawBlock := false

		for _, tc := range result.ToolCalls {
			if !tc.Success {
				blockedCount++
				sawBlock = true
			} else if sawBlock {
				successAfterBlock = true
			}
		}

		if blockedCount == 0 {
			return AssertionResult{
				Name:    "model_recovered",
				Passed:  true,
				Message: "No blocks to recover from",
			}
		}

		if successAfterBlock {
			return AssertionResult{
				Name:    "model_recovered",
				Passed:  true,
				Message: fmt.Sprintf("Model recovered from %d block(s)", blockedCount),
			}
		}

		return AssertionResult{
			Name:    "model_recovered",
			Passed:  false,
			Message: fmt.Sprintf("Model did not recover from %d block(s)", blockedCount),
		}
	}
}

// === Helper functions ===

func getToolNames(toolCalls []ToolCallEvent) []string {
	names := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		names[i] = tc.Name
	}
	return names
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
