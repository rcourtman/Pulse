package chat

import (
	"regexp"
	"strings"
)

// Compiled regexes for structural tool call artifacts from various LLM providers.
// These patterns catch tool call markup that leaks into content when models are
// told not to use tools but still see tool definitions.
var (
	// XML-style tool call envelopes: <tool_call>...</tool_call>, <tool_calls>...</tool_calls>,
	// <function_call>...</function_call>, <function_calls>...</function_calls>
	xmlToolCallRe = regexp.MustCompile(`(?s)</?(?:tool_calls?|function_calls?)>`)

	// Pipe-delimited markers: <|plugin|>, <|interpreter|>, <|tool_call|>, etc.
	pipeMarkerRe = regexp.MustCompile(`<\|(?:plugin|interpreter|tool_call)\|>`)

	// MiniMax-style markers: minimax:tool_call followed by JSON-like content
	minimaxMarkerRe = regexp.MustCompile(`(?m)^minimax:tool_call\b`)
)

// cleanToolCallArtifacts removes LLM-internal tool call format leakage from content.
// Models that see tool definitions but are told not to use them sometimes dump native
// tool call markup into the content field. This function strips those artifacts.
// Applied to chat responses to prevent artifacts from being shown to users.
func cleanToolCallArtifacts(content string) string {
	if content == "" {
		return content
	}

	// Fast-path: DeepSeek DSML markers (literal string checks, no regex needed)
	dsmlMarkers := []string{
		"<｜DSML｜",  // Unicode pipe variant (opening)
		"</｜DSML｜", // Unicode pipe variant (closing)
		"<|DSML|",  // ASCII pipe variant (opening)
		"</|DSML|", // ASCII pipe variant (closing)
		"<｜/DSML｜", // Alternative Unicode closing
		"<|/DSML|", // Alternative ASCII closing
	}

	for _, marker := range dsmlMarkers {
		if idx := strings.Index(content, marker); idx >= 0 {
			content = strings.TrimSpace(content[:idx])
		}
	}

	// Structural patterns: XML-style envelopes
	if loc := xmlToolCallRe.FindStringIndex(content); loc != nil {
		content = strings.TrimSpace(content[:loc[0]])
	}

	// Pipe-delimited markers
	if loc := pipeMarkerRe.FindStringIndex(content); loc != nil {
		content = strings.TrimSpace(content[:loc[0]])
	}

	// MiniMax-style markers
	if loc := minimaxMarkerRe.FindStringIndex(content); loc != nil {
		content = strings.TrimSpace(content[:loc[0]])
	}

	return content
}

// containsToolCallMarker checks if content contains any known LLM-internal tool call markers.
// Used during streaming to detect when to stop forwarding content chunks.
func containsToolCallMarker(content string) bool {
	// Fast-path: DeepSeek DSML literal checks
	dsmlMarkers := []string{
		"<｜DSML｜", // Unicode pipe variant
		"<|DSML|", // ASCII pipe variant
	}
	for _, marker := range dsmlMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}

	// Structural patterns
	if xmlToolCallRe.MatchString(content) {
		return true
	}
	if pipeMarkerRe.MatchString(content) {
		return true
	}
	if minimaxMarkerRe.MatchString(content) {
		return true
	}

	return false
}
