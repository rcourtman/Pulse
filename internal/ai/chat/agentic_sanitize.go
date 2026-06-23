package chat

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// Compiled regexes for Assistant presentation artifacts. Generic provider
// tool-call artifact detection lives in agentcapabilities so native Assistant
// and external adapter boundaries share the same leak guard.
var (
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
		"🤖", "",
		"🔧", "", "🛠️", "", "🛠", "", "🧰", "",
		"🔥", "", "💡", "", "📌", "", "📍", "", "📊", "", "📈", "", "📉", "",
	)
	decorativeAssistantSymbols = []string{
		"⚠️", "⚠", "🚨", "🛑", "⛔",
		"🔴", "🟠", "🟡", "🟢", "🔵", "🟣", "🟤", "⚫", "⚪",
		"✅", "❌", "❎", "✔️", "✔", "☑️", "☑", "✖️", "✖", "✗", "✘",
		"ℹ️", "ℹ", "❗", "❕", "❓", "❔",
		"🤖",
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
	return agentcapabilities.ProviderToolCallArtifactIndex(content, tools.AssistantProviderToolNameCatalog())
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
		if existing == "" && isCompactedToolPrelude(text) {
			if pending != nil {
				*pending = text
			}
			return "", false
		}
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
	if isCompactedToolPrelude(visible) {
		return ""
	}
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
	return agentcapabilities.SplitTrailingProviderToolNamePrefix(content, tools.AssistantProviderToolNameCatalog())
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
