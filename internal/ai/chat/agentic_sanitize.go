package chat

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// Compiled regexes for structural tool call artifacts from various LLM providers.
// These patterns catch tool call markup that leaks into content when models are
// told not to use tools but still see tool definitions.
var (
	dsmlMarkers = []string{
		"<｜DSML｜",    // Unicode single pipe (opening)
		"</｜DSML｜",   // Unicode single pipe (closing)
		"<｜｜DSML｜｜",  // Unicode double pipe (opening) — deepseek-v4-flash
		"</｜｜DSML｜｜", // Unicode double pipe (closing)
		"<||DSML||",  // ASCII double pipe (opening)
		"</||DSML||", // ASCII double pipe (closing)
		"<|DSML|",    // ASCII single pipe (opening)
		"</|DSML|",   // ASCII single pipe (closing)
		"<｜/DSML｜",   // Alternative Unicode closing
		"<｜｜/DSML｜｜", // Alternative Unicode double-pipe closing
		"<|/DSML|",   // Alternative ASCII closing
		"<||/DSML||", // Alternative ASCII double-pipe closing
	}

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

	// Plain function-style tool-call leak: some models emit
	// pulse_read(target_host="...", command="...") as assistant content
	// instead of a structured tool call. Gate on canonical tool names so a
	// random prose function call is not stripped.
	plainFunctionToolCallRe = regexp.MustCompile(`(?:^|[^a-zA-Z0-9_])([a-zA-Z_][a-zA-Z0-9_]*)[ \t\r\n]*\(`)

	// Operational providers often decorate alert/status answers with emoji
	// badges even when instructed not to. Pulse renders state through typed
	// tool/progress rows, so these glyphs are treated as presentation artifacts
	// at the browser/API boundary rather than assistant prose.
	decorativeAssistantSymbolReplacer = strings.NewReplacer(
		"\ufe0f", "",
		"⚠️", "", "⚠", "",
		"🚨", "", "🛑", "", "⛔", "",
		"🔴", "", "🟠", "", "🟡", "", "🟢", "", "🔵", "", "🟣", "", "🟤", "", "⚫", "", "⚪", "",
		"✅", "", "❌", "", "❎", "", "✔️", "", "✔", "", "☑️", "", "☑", "", "✖️", "", "✖", "", "✗", "", "✘", "",
		"ℹ️", "", "ℹ", "", "❗", "", "❕", "", "❓", "", "❔", "",
		"🔧", "", "🛠️", "", "🛠", "", "🧰", "",
		"🔥", "", "💡", "", "📌", "", "📍", "", "📊", "", "📈", "", "📉", "",
	)
	decorativeAssistantSymbols = []string{
		"⚠️", "⚠", "🚨", "🛑", "⛔",
		"🔴", "🟠", "🟡", "🟢", "🔵", "🟣", "🟤", "⚫", "⚪",
		"✅", "❌", "❎", "✔️", "✔", "☑️", "☑", "✖️", "✖", "✗", "✘",
		"ℹ️", "ℹ", "❗", "❕", "❓", "❔",
		"🔧", "🛠️", "🛠", "🧰",
		"🔥", "💡", "📌", "📍", "📊", "📈", "📉",
	}
	decorativeWhitespaceGapRe       = regexp.MustCompile(`([^\s])[ \t]{2,}([^\s])`)
	decorativeSpaceBeforePunctRe    = regexp.MustCompile(`[ \t]+([,.;:!?])`)
	decorativeTightHeadingRe        = regexp.MustCompile(`^([ \t]{0,3}#{1,6})([^#\s])`)
	decorativeTightListMarkerRe     = regexp.MustCompile(`^([ \t]*(?:[-*+]|[0-9]+[.)]))([^ \t])`)
	decorativeTightColonRe          = regexp.MustCompile(`(:)([A-Z][A-Za-z])`)
	decorativeAssistantFenceStartRe = regexp.MustCompile(`^[ \t]*(?:` + "```" + `|~~~)`)
)

// cleanToolCallArtifacts removes LLM-internal tool call format leakage from content.
// Models that see tool definitions but are told not to use them sometimes dump native
// tool call markup into the content field. This function strips those artifacts.
// Applied to chat responses to prevent artifacts from being shown to users.
func cleanToolCallArtifacts(content string) string {
	if content == "" {
		return content
	}

	if idx := toolCallArtifactIndex(content); idx >= 0 {
		prefix := strings.TrimSpace(content[:idx])
		if isCompactedToolPrelude(prefix) {
			return ""
		}
		content = prefix
	}

	return cleanDecorativeAssistantSymbols(content)
}

func cleanDecorativeAssistantSymbols(content string) string {
	if content == "" {
		return content
	}

	var builder strings.Builder
	builder.Grow(len(content))

	inFence := false
	lines := strings.SplitAfter(content, "\n")
	for _, segment := range lines {
		line := strings.TrimSuffix(segment, "\n")
		hasNewline := len(segment) > len(line)

		if decorativeAssistantFenceStartRe.MatchString(line) {
			builder.WriteString(line)
			if hasNewline {
				builder.WriteByte('\n')
			}
			inFence = !inFence
			continue
		}
		if inFence {
			builder.WriteString(line)
			if hasNewline {
				builder.WriteByte('\n')
			}
			continue
		}

		cleaned := decorativeAssistantSymbolReplacer.Replace(line)
		if cleaned != line {
			if lineStartsWithDecorativeAssistantSymbol(line) {
				cleaned = strings.TrimLeft(cleaned, " \t")
			}
			cleaned = decorativeTightHeadingRe.ReplaceAllString(cleaned, "$1 $2")
			cleaned = decorativeTightListMarkerRe.ReplaceAllString(cleaned, "$1 $2")
			cleaned = decorativeTightColonRe.ReplaceAllString(cleaned, "$1 $2")
			cleaned = decorativeWhitespaceGapRe.ReplaceAllString(cleaned, "$1 $2")
			cleaned = decorativeSpaceBeforePunctRe.ReplaceAllString(cleaned, "$1")
		}

		builder.WriteString(cleaned)
		if hasNewline {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func lineStartsWithDecorativeAssistantSymbol(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	for _, symbol := range decorativeAssistantSymbols {
		if strings.HasPrefix(trimmed, symbol) {
			return true
		}
	}
	return false
}

// containsToolCallMarker checks if content contains any known LLM-internal tool call markers.
// Used during streaming to detect when to stop forwarding content chunks.
func containsToolCallMarker(content string) bool {
	return toolCallArtifactIndex(content) >= 0
}

// toolCallArtifactIndex returns the byte offset of the first known tool-call
// artifact in content, or -1 if no artifact is present.
func toolCallArtifactIndex(content string) int {
	if content == "" {
		return -1
	}

	first := -1
	record := func(idx int) {
		if idx < 0 {
			return
		}
		if first < 0 || idx < first {
			first = idx
		}
	}

	for _, marker := range dsmlMarkers {
		if idx := strings.Index(content, marker); idx >= 0 {
			record(idx)
		}
	}

	if loc := dsmlRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := xmlToolCallRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := pipeMarkerRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := minimaxMarkerRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	record(findJSONToolCallLeak(content))
	record(findPlainFunctionToolCallLeak(content))

	return first
}

func appendVisibleContentBeforeToolLeak(
	builder *strings.Builder,
	pending *string,
	text string,
) (visibleDelta string, leakFound bool) {
	if text == "" && (pending == nil || *pending == "") {
		return "", false
	}

	pendingText := ""
	if pending != nil {
		pendingText = *pending
		*pending = ""
	}
	text = pendingText + text

	existing := builder.String()
	candidate := existing + text
	idx := toolCallArtifactIndex(candidate)
	if idx < 0 {
		visible, held := splitTrailingPotentialToolNamePrefix(text)
		if pending != nil {
			*pending = held
		}
		return appendSanitizedVisibleDelta(builder, visible), false
	}

	if idx > len(existing) {
		if isCompactedToolPrelude(candidate[:idx]) {
			builder.Reset()
			if pending != nil {
				*pending = ""
			}
			return "", true
		}
		visibleDelta, _ = splitTrailingPotentialToolNamePrefix(candidate[len(existing):idx])
		visibleDelta = appendSanitizedVisibleDelta(builder, visibleDelta)
	} else if isCompactedToolPrelude(candidate[:idx]) {
		builder.Reset()
		if pending != nil {
			*pending = ""
		}
	}
	return visibleDelta, true
}

func flushPendingVisibleContent(builder *strings.Builder, pending *string) string {
	if pending == nil || *pending == "" {
		return ""
	}
	visible := *pending
	*pending = ""
	return appendSanitizedVisibleDelta(builder, visible)
}

func appendSanitizedVisibleDelta(builder *strings.Builder, visible string) string {
	if visible == "" {
		return ""
	}
	existing := builder.String()
	cleanedCandidate := cleanDecorativeAssistantSymbols(existing + visible)
	if !strings.HasPrefix(cleanedCandidate, existing) {
		cleanedCandidate = existing + cleanDecorativeAssistantSymbols(visible)
	}
	delta := strings.TrimPrefix(cleanedCandidate, existing)
	builder.WriteString(delta)
	return delta
}

func splitTrailingPotentialToolNamePrefix(content string) (visible string, held string) {
	if content == "" {
		return "", ""
	}

	start := len(content)
	for start > 0 {
		ch := content[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			start--
			continue
		}
		break
	}

	if start == len(content) {
		return content, ""
	}

	token := content[start:]
	if tools.IsKnownToolNamePrefix(token) {
		return content[:start], token
	}
	return content, ""
}

func isCompactedToolPrelude(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	letters := 0
	whitespace := 0
	for _, ch := range trimmed {
		if unicode.IsLetter(ch) {
			letters++
		}
		if unicode.IsSpace(ch) {
			whitespace++
		}
	}

	if letters < 16 {
		return false
	}
	if whitespace == 0 {
		return true
	}
	return letters >= 48 && whitespace <= 1
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

func findPlainFunctionToolCallLeak(content string) int {
	if content == "" {
		return -1
	}
	matches := plainFunctionToolCallRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 || m[3] < 0 {
			continue
		}
		name := content[m[2]:m[3]]
		if tools.IsKnownToolName(name) {
			return m[2]
		}
	}
	return -1
}
