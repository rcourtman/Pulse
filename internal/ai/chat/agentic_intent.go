package chat

import "strings"

// detectFreshDataIntent returns true when the user's latest message explicitly
// requests updated/fresh data (e.g. "check again", "refresh", "what's the
// latest status"). This bypasses the knowledge gate for the first turn so
// tools re-execute instead of returning cached results.
func detectFreshDataIntent(messages []Message) bool {
	// Walk backwards to find the last user message
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			lastUserContent = strings.ToLower(messages[i].Content)
			break
		}
	}
	if lastUserContent == "" {
		return false
	}

	// Strong refresh signals — these clearly indicate the user wants re-execution
	strongPatterns := []string{
		"check again", "look again", "try again", "run again",
		"refresh", "re-check", "recheck", "re check",
		"fresh data", "fresh look", "latest data",
		"has it changed", "did it change", "any changes",
		"what's happening now", "what is happening now",
	}
	for _, p := range strongPatterns {
		if strings.Contains(lastUserContent, p) {
			return true
		}
	}
	return false
}

// hasPhantomExecution detects when the model claims to have executed something
// but no actual tool calls were made. This catches models that "hallucinate"
// tool execution by writing about it instead of calling tools.
//
// We're intentionally conservative here to avoid false positives like:
// - "I checked the docs..." (not a tool)
// - "I ran through the logic..." (not a command)
//
// We only trigger when the model asserts:
// 1. Concrete system metrics/values (CPU %, memory usage, etc.)
// 2. Infrastructure state that requires live queries (running/stopped)
// 3. Fake tool call formatting
func hasPhantomExecution(content string) bool {
	if content == "" {
		return false
	}

	lower := strings.ToLower(content)

	// Category 1: Concrete metrics/values that MUST come from tools
	// These are specific enough that they can't be "general knowledge"
	metricsPatterns := []string{
		"cpu usage is ", "cpu is at ", "cpu at ",
		"memory usage is ", "memory is at ", "memory at ",
		"disk usage is ", "disk is at ", "storage at ",
		"using % ", "% cpu", "% memory", "% disk",
		"mb of ram", "gb of ram", "mb of memory", "gb of memory",
	}

	for _, pattern := range metricsPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 2: Claims of infrastructure state that require live queries
	// Must be specific claims about current state, not general discussion
	statePatterns := []string{
		"is currently running", "is currently stopped", "is currently down",
		"is now running", "is now stopped", "is now restarted",
		"the service is running", "the container is running",
		"the service is stopped", "the container is stopped",
		"the logs show", "the output shows", "the result shows",
		"according to the logs", "according to the output",
	}

	for _, pattern := range statePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 3: Fake tool call formatting (definite hallucination)
	fakeToolPatterns := []string{
		"```tool", "```json\n{\"tool", "tool_result:",
		"function_call:", "<tool_call>", "</tool_call>",
		"pulse_query(", "pulse_run_command(", "pulse_control(",
	}

	for _, pattern := range fakeToolPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Category 4: Past tense claims of SPECIFIC infrastructure actions
	// Only trigger if followed by concrete results (not "I checked and...")
	actionResultPatterns := []string{
		"i restarted the", "i stopped the", "i started the",
		"i killed the", "i terminated the",
		"successfully restarted", "successfully stopped", "successfully started",
		"has been restarted", "has been stopped", "has been started",
	}

	for _, pattern := range actionResultPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}
