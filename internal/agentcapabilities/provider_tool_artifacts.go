package agentcapabilities

import (
	"regexp"
	"strings"
)

// Compiled regexes for structural provider tool-call artifacts from common LLM
// providers. These guards belong to the shared provider boundary so Assistant
// streaming and external adapter bridges do not diverge on what counts as raw
// tool-call leakage.
var (
	providerDSMLMarkers = []string{
		"<｜DSML｜",
		"</｜DSML｜",
		"<｜｜DSML｜｜",
		"</｜｜DSML｜｜",
		"<||DSML||",
		"</||DSML||",
		"<|DSML|",
		"</|DSML|",
		"<｜/DSML｜",
		"<｜｜/DSML｜｜",
		"<|/DSML|",
		"<||/DSML||",
	}

	providerXMLToolCallRe = regexp.MustCompile(`(?s)</?(?:tool_calls?|function_calls?)>`)
	providerPipeMarkerRe  = regexp.MustCompile(`<\|(?:plugin|interpreter|tool_call)\|>`)
	providerMiniMaxRe     = regexp.MustCompile(`(?m)^minimax:tool_call\b`)
	providerDSMLRe        = regexp.MustCompile(`</?[\|｜]+/?DSML[\|｜]*`)

	providerJSONToolCallRe = regexp.MustCompile(
		`(?:^|\n)[ \t]*(?:` + "```" + `[ \t]*(?:json|JSON)?[ \t]*\n?[ \t]*)?\{[ \t\n]*"name"[ \t]*:[ \t]*"([a-zA-Z_][a-zA-Z0-9_]*)"`,
	)
	providerPlainFunctionToolCallRe = regexp.MustCompile(`(?:^|[^a-zA-Z0-9_])([a-zA-Z_][a-zA-Z0-9_]*)[ \t\r\n]*\(`)
)

// ProviderToolCallArtifactIndex returns the byte offset of the first known
// provider tool-call artifact in content, or -1 when content can be shown as
// normal assistant text. Tool-name-shaped leaks are gated by catalog so
// unrelated JSON and function examples are not treated as provider artifacts.
func ProviderToolCallArtifactIndex(content string, catalog ProviderToolNameCatalog) int {
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

	for _, marker := range providerDSMLMarkers {
		if idx := strings.Index(content, marker); idx >= 0 {
			record(idx)
		}
	}

	if loc := providerDSMLRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := providerXMLToolCallRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := providerPipeMarkerRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	if loc := providerMiniMaxRe.FindStringIndex(content); loc != nil {
		record(loc[0])
	}
	record(providerJSONToolCallLeakIndex(content, catalog))
	record(providerPlainFunctionToolCallLeakIndex(content, catalog))

	return first
}

// SplitTrailingProviderToolNamePrefix holds a trailing token fragment when it
// can still become a known provider tool name. Streaming callers can keep the
// held suffix private until the next chunk proves whether it is prose or a raw
// provider tool call.
func SplitTrailingProviderToolNamePrefix(content string, catalog ProviderToolNameCatalog) (visible string, held string) {
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
	if catalog.HasPrefix(token) {
		return content[:start], token
	}
	return content, ""
}

func providerJSONToolCallLeakIndex(content string, catalog ProviderToolNameCatalog) int {
	if content == "" {
		return -1
	}
	matches := providerJSONToolCallRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 || m[3] < 0 {
			continue
		}
		name := content[m[2]:m[3]]
		if catalog.Has(name) {
			return m[0]
		}
	}
	return -1
}

func providerPlainFunctionToolCallLeakIndex(content string, catalog ProviderToolNameCatalog) int {
	if content == "" {
		return -1
	}
	matches := providerPlainFunctionToolCallRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		if len(m) < 4 || m[2] < 0 || m[3] < 0 {
			continue
		}
		name := content[m[2]:m[3]]
		if catalog.Has(name) {
			return m[2]
		}
	}
	return -1
}
