package providers

import "strings"

const DefaultContextWindow = 128_000

var modelContextWindows = map[string]int{
	// Anthropic Claude
	"claude-opus-4":     200_000,
	"claude-sonnet-4":   200_000,
	"claude-haiku-4":    200_000,
	"claude-3-5-sonnet": 200_000,
	"claude-3-5-haiku":  200_000,
	"claude-3-opus":     200_000,
	"claude-3-sonnet":   200_000,
	"claude-3-haiku":    200_000,

	// OpenAI
	"gpt-4o":      128_000,
	"gpt-4o-mini": 128_000,
	"gpt-4-turbo": 128_000,
	"gpt-4":       8_192,
	"o1":          200_000,
	"o1-mini":     128_000,
	"o1-preview":  128_000,
	"o3":          200_000,
	"o3-mini":     200_000,
	"o4-mini":     200_000,

	// Google Gemini
	"gemini-2.5-pro":   1_048_576,
	"gemini-2.5-flash": 1_048_576,
	"gemini-2.0-flash": 1_048_576,
	"gemini-1.5-pro":   2_097_152,
	"gemini-1.5-flash": 1_048_576,

	// DeepSeek
	"deepseek-chat":     128_000,
	"deepseek-reasoner": 128_000,

	// MiniMax
	"MiniMax-Text-01": 1_000_000,
	"abab7":           196_000,

	// xAI
	"grok-3":      131_072,
	"grok-3-mini": 131_072,
	"grok-2":      131_072,

	// Meta (via various providers)
	"llama-3.3": 128_000,
	"llama-3.1": 128_000,

	// Qwen
	"qwen-max":   131_072,
	"qwen-plus":  131_072,
	"qwen-turbo": 1_000_000,
}

// ContextWindowTokens returns the context window size in tokens for the given model.
// It uses fuzzy matching: strips provider prefix (before ":"), strips date suffixes,
// and tries progressively shorter prefixes for version tolerance.
// Returns DefaultContextWindow if no match is found.
func ContextWindowTokens(model string) int {
	modelName := extractModelName(model)
	if modelName == "" {
		return DefaultContextWindow
	}

	if tokens, ok := modelContextWindows[modelName]; ok {
		return tokens
	}

	modelWithoutDate := stripDateSuffix(modelName)
	if tokens, ok := modelContextWindows[modelWithoutDate]; ok {
		return tokens
	}

	if tokens, ok := longestPrefixMatch(modelName); ok {
		return tokens
	}
	if modelWithoutDate != modelName {
		if tokens, ok := longestPrefixMatch(modelWithoutDate); ok {
			return tokens
		}
	}

	if tokens, ok := caseInsensitiveMatch(modelName); ok {
		return tokens
	}
	if modelWithoutDate != modelName {
		if tokens, ok := caseInsensitiveMatch(modelWithoutDate); ok {
			return tokens
		}
	}

	return DefaultContextWindow
}

func extractModelName(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}

	if idx := strings.Index(model, ":"); idx >= 0 {
		model = strings.TrimSpace(model[idx+1:])
	}

	return model
}

func stripDateSuffix(model string) string {
	if len(model) >= 11 {
		// -YYYY-MM-DD
		n := len(model)
		if model[n-11] == '-' &&
			isDigits(model[n-10:n-6]) &&
			model[n-6] == '-' &&
			isDigits(model[n-5:n-3]) &&
			model[n-3] == '-' &&
			isDigits(model[n-2:n]) {
			return model[:n-11]
		}
	}

	if len(model) >= 9 {
		// -YYYYMMDD
		n := len(model)
		if model[n-9] == '-' && isDigits(model[n-8:n]) {
			return model[:n-9]
		}
	}

	return model
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func longestPrefixMatch(model string) (int, bool) {
	bestLen := -1
	bestTokens := 0
	for knownModel, tokens := range modelContextWindows {
		if strings.HasPrefix(model, knownModel) && len(knownModel) > bestLen {
			bestLen = len(knownModel)
			bestTokens = tokens
		}
	}

	if bestLen >= 0 {
		return bestTokens, true
	}
	return 0, false
}

func caseInsensitiveMatch(model string) (int, bool) {
	for knownModel, tokens := range modelContextWindows {
		if strings.EqualFold(model, knownModel) {
			return tokens, true
		}
	}

	modelLower := strings.ToLower(model)
	bestLen := -1
	bestTokens := 0
	for knownModel, tokens := range modelContextWindows {
		knownLower := strings.ToLower(knownModel)
		if strings.HasPrefix(modelLower, knownLower) && len(knownLower) > bestLen {
			bestLen = len(knownLower)
			bestTokens = tokens
		}
	}

	if bestLen >= 0 {
		return bestTokens, true
	}
	return 0, false
}
