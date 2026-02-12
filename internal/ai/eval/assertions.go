package eval

import (
	"encoding/json"
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

// AssertToolNotUsed checks that a specific tool was NOT called
func AssertToolNotUsed(toolName string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Name == toolName {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_not_used:%s", toolName),
					Passed:  false,
					Message: fmt.Sprintf("Tool '%s' was called", toolName),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_not_used:%s", toolName),
			Passed:  true,
			Message: fmt.Sprintf("Tool '%s' was not called", toolName),
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

// AssertContentContainsAny checks that the response contains at least one substring.
func AssertContentContainsAny(substrings ...string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, substring := range substrings {
			if strings.Contains(strings.ToLower(result.Content), strings.ToLower(substring)) {
				return AssertionResult{
					Name:    "content_contains_any",
					Passed:  true,
					Message: fmt.Sprintf("Content contains '%s'", substring),
				}
			}
		}
		return AssertionResult{
			Name:    "content_contains_any",
			Passed:  false,
			Message: fmt.Sprintf("Content does NOT contain any of: %v", substrings),
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
			if hasSuccessfulToolCall(result.ToolCalls) {
				return AssertionResult{
					Name:    "no_phantom_detection",
					Passed:  true,
					Message: "Phantom detection phrase present, but tool calls succeeded",
				}
			}
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
				strings.Contains(tc.Output, "READ_ONLY_VIOLATION") ||
				strings.Contains(tc.Output, "STRICT_RESOLUTION") {
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

// AssertEventualSuccessOrApproval checks that a tool succeeded or an approval was requested.
func AssertEventualSuccessOrApproval() Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if tc.Success {
				return AssertionResult{
					Name:    "eventual_success_or_approval",
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' succeeded", tc.Name),
				}
			}
		}
		if len(result.Approvals) > 0 {
			return AssertionResult{
				Name:    "eventual_success_or_approval",
				Passed:  true,
				Message: fmt.Sprintf("Approval requests: %d", len(result.Approvals)),
			}
		}
		return AssertionResult{
			Name:    "eventual_success_or_approval",
			Passed:  false,
			Message: "No tool calls succeeded and no approvals were requested",
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

// AssertMaxInputTokens checks that total input tokens stayed under a ceiling.
// This catches regressions where the model loops and burns excessive tokens.
func AssertMaxInputTokens(max int) Assertion {
	return func(result *StepResult) AssertionResult {
		if result.InputTokens <= max {
			return AssertionResult{
				Name:    fmt.Sprintf("max_input_tokens:%d", max),
				Passed:  true,
				Message: fmt.Sprintf("%d input tokens used (max: %d)", result.InputTokens, max),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("max_input_tokens:%d", max),
			Passed:  false,
			Message: fmt.Sprintf("%d input tokens used (expected at most %d)", result.InputTokens, max),
		}
	}
}

// AssertMaxToolCalls checks that the assistant made at most N tool calls.
// This catches regressions where the model loops excessively on simple tasks.
func AssertMaxToolCalls(max int) Assertion {
	return func(result *StepResult) AssertionResult {
		if len(result.ToolCalls) <= max {
			return AssertionResult{
				Name:    fmt.Sprintf("max_tool_calls:%d", max),
				Passed:  true,
				Message: fmt.Sprintf("%d tool calls made (max: %d)", len(result.ToolCalls), max),
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("max_tool_calls:%d", max),
			Passed:  false,
			Message: fmt.Sprintf("%d tool calls made (expected at most %d). Tools: %v", len(result.ToolCalls), max, getToolNames(result.ToolCalls)),
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

// AssertToolSequence checks that tool calls occurred in the given order.
// The sequence does not need to be contiguous, but order must be preserved.
func AssertToolSequence(sequence []string) Assertion {
	return func(result *StepResult) AssertionResult {
		if len(sequence) == 0 {
			return AssertionResult{
				Name:    "tool_sequence",
				Passed:  true,
				Message: "No sequence required",
			}
		}

		seqIdx := 0
		for _, tc := range result.ToolCalls {
			if tc.Name == sequence[seqIdx] {
				seqIdx++
				if seqIdx == len(sequence) {
					return AssertionResult{
						Name:    "tool_sequence",
						Passed:  true,
						Message: fmt.Sprintf("Tool sequence matched: %v", sequence),
					}
				}
			}
		}

		return AssertionResult{
			Name:    "tool_sequence",
			Passed:  false,
			Message: fmt.Sprintf("Tool sequence not found. Expected: %v, got: %v", sequence, getToolNames(result.ToolCalls)),
		}
	}
}

// AssertToolInputContains checks that a tool's input contains a substring.
func AssertToolInputContains(toolName, substring string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if toolName != "" && tc.Name != toolName {
				continue
			}
			if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(substring)) {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' input contains '%s'", tc.Name, substring),
				}
			}
			if toolName != "" {
				return AssertionResult{
					Name:    fmt.Sprintf("tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
					Passed:  false,
					Message: fmt.Sprintf("Tool '%s' input does NOT contain '%s'", toolName, substring),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("Tool '%s' was not called", toolName),
		}
	}
}

// AssertAnyToolInputContains checks that any tool input contains a substring.
// If toolName is empty, any tool input is considered.
func AssertAnyToolInputContains(toolName, substring string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if toolName != "" && tc.Name != toolName {
				continue
			}
			if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(substring)) {
				return AssertionResult{
					Name:    fmt.Sprintf("any_tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
					Passed:  true,
					Message: fmt.Sprintf("Tool '%s' input contains '%s'", tc.Name, substring),
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("any_tool_input:%s_contains:%s", toolName, truncate(substring, 20)),
			Passed:  false,
			Message: fmt.Sprintf("No tool input matched '%s'", substring),
		}
	}
}

// AssertAnyToolInputContainsAny checks that any tool input contains any of the substrings.
// If toolName is empty, any tool input is considered.
func AssertAnyToolInputContainsAny(toolName string, substrings ...string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if toolName != "" && tc.Name != toolName {
				continue
			}
			for _, substring := range substrings {
				if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(substring)) {
					return AssertionResult{
						Name:    fmt.Sprintf("any_tool_input:%s_contains_any", toolName),
						Passed:  true,
						Message: fmt.Sprintf("Tool '%s' input contains '%s'", tc.Name, substring),
					}
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("any_tool_input:%s_contains_any", toolName),
			Passed:  false,
			Message: fmt.Sprintf("No tool input matched any of: %v", substrings),
		}
	}
}

// AssertToolOutputContainsAny checks that a tool output contains any of the substrings.
// If toolName is empty, any tool output is considered.
func AssertToolOutputContainsAny(toolName string, substrings ...string) Assertion {
	return func(result *StepResult) AssertionResult {
		for _, tc := range result.ToolCalls {
			if toolName != "" && tc.Name != toolName {
				continue
			}
			for _, substring := range substrings {
				if strings.Contains(strings.ToLower(tc.Output), strings.ToLower(substring)) {
					return AssertionResult{
						Name:    fmt.Sprintf("tool_output:%s_contains_any", toolName),
						Passed:  true,
						Message: fmt.Sprintf("Tool '%s' output contains '%s'", tc.Name, substring),
					}
				}
			}
		}
		return AssertionResult{
			Name:    fmt.Sprintf("tool_output:%s_contains_any", toolName),
			Passed:  false,
			Message: fmt.Sprintf("No tool output matched any of: %v", substrings),
		}
	}
}

// AssertApprovalRequested checks that at least one approval request was emitted.
func AssertApprovalRequested() Assertion {
	return func(result *StepResult) AssertionResult {
		if len(result.Approvals) > 0 {
			return AssertionResult{
				Name:    "approval_requested",
				Passed:  true,
				Message: fmt.Sprintf("Approval requests: %d", len(result.Approvals)),
			}
		}
		return AssertionResult{
			Name:    "approval_requested",
			Passed:  false,
			Message: "No approval requests were captured",
		}
	}
}

type exploreStatusEvent struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

func parseExploreStatusEvents(rawEvents []SSEEvent) ([]exploreStatusEvent, error) {
	events := make([]exploreStatusEvent, 0, len(rawEvents))
	for _, raw := range rawEvents {
		if raw.Type != "explore_status" {
			continue
		}
		var data exploreStatusEvent
		if err := json.Unmarshal(raw.Data, &data); err != nil {
			return nil, fmt.Errorf("invalid explore_status payload: %w", err)
		}
		events = append(events, data)
	}
	return events, nil
}

// AssertExploreStatusSeen checks that at least one explore_status event is emitted.
func AssertExploreStatusSeen() Assertion {
	return func(result *StepResult) AssertionResult {
		events, err := parseExploreStatusEvents(result.RawEvents)
		if err != nil {
			return AssertionResult{
				Name:    "explore_status_seen",
				Passed:  false,
				Message: err.Error(),
			}
		}
		if len(events) == 0 {
			return AssertionResult{
				Name:    "explore_status_seen",
				Passed:  false,
				Message: "No explore_status events captured",
			}
		}
		return AssertionResult{
			Name:    "explore_status_seen",
			Passed:  true,
			Message: fmt.Sprintf("%d explore_status event(s) captured", len(events)),
		}
	}
}

// AssertExploreLifecycleValid checks explore_status phase ordering and required fields.
func AssertExploreLifecycleValid() Assertion {
	return func(result *StepResult) AssertionResult {
		events, err := parseExploreStatusEvents(result.RawEvents)
		if err != nil {
			return AssertionResult{
				Name:    "explore_lifecycle_valid",
				Passed:  false,
				Message: err.Error(),
			}
		}
		if len(events) == 0 {
			return AssertionResult{
				Name:    "explore_lifecycle_valid",
				Passed:  false,
				Message: "No explore_status events captured",
			}
		}

		allowed := map[string]struct{}{
			"started":   {},
			"completed": {},
			"failed":    {},
			"skipped":   {},
		}
		terminal := map[string]struct{}{
			"completed": {},
			"failed":    {},
			"skipped":   {},
		}

		first := events[0]
		if first.Phase != "started" && first.Phase != "skipped" {
			return AssertionResult{
				Name:    "explore_lifecycle_valid",
				Passed:  false,
				Message: fmt.Sprintf("First explore phase must be started or skipped, got %q", first.Phase),
			}
		}

		lastTerminalIdx := -1
		lastTerminal := ""
		for i, evt := range events {
			if _, ok := allowed[evt.Phase]; !ok {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: fmt.Sprintf("Unknown explore phase %q", evt.Phase),
				}
			}
			if strings.TrimSpace(evt.Message) == "" {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: fmt.Sprintf("Explore event at index %d has empty message", i),
				}
			}
			if evt.Phase == "started" && strings.TrimSpace(evt.Model) == "" {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: "Explore started event missing model",
				}
			}
			if evt.Phase == "completed" || evt.Phase == "failed" || evt.Phase == "skipped" {
				if strings.TrimSpace(evt.Outcome) == "" {
					return AssertionResult{
						Name:    "explore_lifecycle_valid",
						Passed:  false,
						Message: fmt.Sprintf("Explore %s event missing outcome", evt.Phase),
					}
				}
			}
			if evt.Phase == "skipped" && !strings.HasPrefix(evt.Outcome, "skipped_") {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: fmt.Sprintf("Explore skipped outcome must start with skipped_, got %q", evt.Outcome),
				}
			}
			if _, ok := terminal[evt.Phase]; ok {
				lastTerminalIdx = i
				lastTerminal = evt.Phase
			}
		}

		if lastTerminalIdx == -1 {
			return AssertionResult{
				Name:    "explore_lifecycle_valid",
				Passed:  false,
				Message: "Explore lifecycle missing terminal phase (completed/failed/skipped)",
			}
		}

		// If explore started, terminal must happen after start and must not be skipped.
		if first.Phase == "started" {
			if lastTerminalIdx == 0 {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: "Explore started without a later terminal phase",
				}
			}
			if lastTerminal == "skipped" {
				return AssertionResult{
					Name:    "explore_lifecycle_valid",
					Passed:  false,
					Message: "Explore cannot transition from started to skipped",
				}
			}
		}

		return AssertionResult{
			Name:    "explore_lifecycle_valid",
			Passed:  true,
			Message: fmt.Sprintf("Explore lifecycle valid: first=%s terminal=%s events=%d", first.Phase, lastTerminal, len(events)),
		}
	}
}

// AssertExploreFallbackHasContent checks that failed/skipped explore still yields assistant content.
func AssertExploreFallbackHasContent() Assertion {
	return func(result *StepResult) AssertionResult {
		events, err := parseExploreStatusEvents(result.RawEvents)
		if err != nil {
			return AssertionResult{
				Name:    "explore_fallback_has_content",
				Passed:  false,
				Message: err.Error(),
			}
		}
		if len(events) == 0 {
			return AssertionResult{
				Name:    "explore_fallback_has_content",
				Passed:  false,
				Message: "No explore_status events captured",
			}
		}

		terminal := events[len(events)-1]
		if terminal.Phase != "failed" && terminal.Phase != "skipped" {
			return AssertionResult{
				Name:    "explore_fallback_has_content",
				Passed:  true,
				Message: fmt.Sprintf("Explore terminal phase %q does not require fallback check", terminal.Phase),
			}
		}
		if strings.TrimSpace(result.Content) == "" {
			return AssertionResult{
				Name:    "explore_fallback_has_content",
				Passed:  false,
				Message: fmt.Sprintf("Explore terminal=%s but assistant content is empty", terminal.Phase),
			}
		}
		return AssertionResult{
			Name:    "explore_fallback_has_content",
			Passed:  true,
			Message: fmt.Sprintf("Explore terminal=%s and assistant content is present", terminal.Phase),
		}
	}
}

// AssertOnlyToolsUsed checks that all tool calls are in the allow list.
func AssertOnlyToolsUsed(allowed ...string) Assertion {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, tool := range allowed {
		allowedSet[tool] = struct{}{}
	}
	return func(result *StepResult) AssertionResult {
		var unexpected []string
		for _, tc := range result.ToolCalls {
			if _, ok := allowedSet[tc.Name]; !ok {
				unexpected = append(unexpected, tc.Name)
			}
		}
		if len(unexpected) == 0 {
			return AssertionResult{
				Name:    "only_tools_used",
				Passed:  true,
				Message: fmt.Sprintf("Only allowed tools used: %v", allowed),
			}
		}
		return AssertionResult{
			Name:    "only_tools_used",
			Passed:  false,
			Message: fmt.Sprintf("Unexpected tools used: %v", unexpected),
		}
	}
}

// AssertRoutingMismatchRecovered verifies recovery if a routing mismatch occurs.
// If a routing mismatch is seen, the tool input must target the specific container.
// If no mismatch is seen, the tool input should still target the node.
func AssertRoutingMismatchRecovered(nodeName, containerName string) Assertion {
	return func(result *StepResult) AssertionResult {
		sawMismatch := false
		for _, tc := range result.ToolCalls {
			if strings.Contains(strings.ToLower(tc.Output), "routing_mismatch") {
				sawMismatch = true
				break
			}
		}

		if sawMismatch {
			for _, tc := range result.ToolCalls {
				if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(containerName)) {
					return AssertionResult{
						Name:    "routing_mismatch_recovered",
						Passed:  true,
						Message: fmt.Sprintf("Routing mismatch recovered by targeting '%s'", containerName),
					}
				}
			}
			return AssertionResult{
				Name:    "routing_mismatch_recovered",
				Passed:  false,
				Message: fmt.Sprintf("Routing mismatch seen, but no tool input targeted '%s'", containerName),
			}
		}

		for _, tc := range result.ToolCalls {
			if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(nodeName)) {
				return AssertionResult{
					Name:    "routing_mismatch_recovered",
					Passed:  true,
					Message: fmt.Sprintf("No routing mismatch; tool input targeted node '%s'", nodeName),
				}
			}
		}

		return AssertionResult{
			Name:    "routing_mismatch_recovered",
			Passed:  false,
			Message: fmt.Sprintf("No routing mismatch and no tool input targeted '%s'", nodeName),
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

func hasSuccessfulToolCall(toolCalls []ToolCallEvent) bool {
	for _, tc := range toolCalls {
		if tc.Success {
			return true
		}
	}
	return false
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
