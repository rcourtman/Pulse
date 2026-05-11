package chat

import (
	"regexp"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
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

	// Plain-JSON tool-call leak: weak local models (qwen2.5:11b/14b and
	// similar small Ollama models) frequently emit Pulse tool invocations
	// as JSON inside content instead of through the structured tool_calls
	// channel. The leak shape is a JSON object whose first key is "name"
	// with a value matching a known Pulse tool, optionally wrapped in a
	// markdown code fence. Anchored on (?:^|\n) so prose containing JSON
	// fragments inline (e.g. "the field {"name":"x"} in the schema") is
	// not stripped. The captured tool name is gated against the canonical
	// allowlist in tools.IsKnownToolName so arbitrary JSON the user might
	// legitimately share is left untouched.
	jsonToolCallRe = regexp.MustCompile(
		`(?:^|\n)[ \t]*(?:` + "```" + `[ \t]*(?:json|JSON)?[ \t]*\n?[ \t]*)?\{[ \t\n]*"name"[ \t]*:[ \t]*"([a-zA-Z_][a-zA-Z0-9_]*)"`,
	)
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

	// Plain-JSON tool-call leak from weak local models (qwen2.5 small).
	// Allowlist-gated to avoid stripping legitimate user JSON.
	if idx := findJSONToolCallLeak(content); idx >= 0 {
		content = strings.TrimSpace(content[:idx])
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
	if findJSONToolCallLeak(content) >= 0 {
		return true
	}

	return false
}

// findJSONToolCallLeak returns the byte offset to strip from when a plain-JSON
// tool-call leak is found, or -1 if none. A match must (a) sit at start of
// content or after a newline, (b) be a JSON object whose first key is "name",
// and (c) reference a tool in tools.IsKnownToolName's allowlist. Returning
// the position of the leading newline (or 0) lets the caller preserve any
// natural-language text before the leak via TrimSpace.
func findJSONToolCallLeak(content string) int {
	if content == "" {
		return -1
	}
	matches := jsonToolCallRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 || m[3] < 0 {
			continue
		}
		name := content[m[2]:m[3]]
		if tools.IsKnownToolName(name) {
			return m[0]
		}
	}
	return -1
}
