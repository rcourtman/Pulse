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

	// DeepSeek DSML markers with arbitrary pipe-count variants. The
	// fast-path string list below covers the single- and double-pipe
	// forms we've actually observed in the wild
	// (`<｜DSML｜...>` and `<｜｜DSML｜｜...>` — the latter from
	// deepseek-v4-flash). This regex is the catch-all backstop for any
	// pipe-count variant the model might emit so the chat orchestrator
	// never falls through to showing raw DSML to the user as the
	// assistant's "final response."
	dsmlRe = regexp.MustCompile(`</?[\|｜]+/?DSML[\|｜]*`)
)

// cleanToolCallArtifacts removes LLM-internal tool call format leakage from content.
// Models that see tool definitions but are told not to use them sometimes dump native
// tool call markup into the content field. This function strips those artifacts.
// Applied to chat responses to prevent artifacts from being shown to users.
func cleanToolCallArtifacts(content string) string {
	if content == "" {
		return content
	}

	// Fast-path: DeepSeek DSML markers (literal string checks, no regex needed).
	// Includes both single- and double-pipe variants — deepseek-v4-flash
	// emits the double-pipe form ("<｜｜DSML｜｜tool_calls>"). The single-pipe
	// list alone left the double-pipe form unsanitised, so users saw the
	// raw DSML in chat as the assistant's "final response."
	dsmlMarkers := []string{
		"<｜DSML｜",    // Unicode single pipe (opening)
		"</｜DSML｜",   // Unicode single pipe (closing)
		"<｜｜DSML｜｜",  // Unicode double pipe (opening) — deepseek-v4-flash
		"</｜｜DSML｜｜", // Unicode double pipe (closing)
		"<|DSML|",    // ASCII single pipe (opening)
		"</|DSML|",   // ASCII single pipe (closing)
		"<||DSML||",  // ASCII double pipe (opening)
		"</||DSML||", // ASCII double pipe (closing)
		"<｜/DSML｜",   // Alternative Unicode closing
		"<｜｜/DSML｜｜", // Alternative Unicode double-pipe closing
		"<|/DSML|",   // Alternative ASCII closing
		"<||/DSML||", // Alternative ASCII double-pipe closing
	}

	for _, marker := range dsmlMarkers {
		if idx := strings.Index(content, marker); idx >= 0 {
			content = strings.TrimSpace(content[:idx])
		}
	}

	// Backstop regex catches arbitrary pipe-count variants the fast-path
	// list above might miss as new model behaviours surface.
	if loc := dsmlRe.FindStringIndex(content); loc != nil {
		content = strings.TrimSpace(content[:loc[0]])
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
	// Fast-path: DeepSeek DSML literal checks. Covers single- and
	// double-pipe variants. deepseek-v4-flash uses double-pipe; older
	// deepseek variants use single. Both leak into content as text
	// rather than going through the tool-call channel.
	dsmlMarkers := []string{
		"<｜DSML｜",   // Unicode single pipe
		"<｜｜DSML｜｜", // Unicode double pipe (deepseek-v4-flash)
		"<|DSML|",   // ASCII single pipe
		"<||DSML||", // ASCII double pipe
	}
	for _, marker := range dsmlMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}

	// Structural patterns
	if dsmlRe.MatchString(content) {
		return true
	}
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
