package cost

import "strings"

// ResolveProviderAndModel normalizes an AI usage event into a provider and model pair for reporting/pricing.
// It intentionally applies compatibility heuristics so OpenAI-compatible APIs (like DeepSeek) still price correctly.
func ResolveProviderAndModel(eventProvider, requestModel, responseModel string) (provider, model string) {
	provider = strings.ToLower(strings.TrimSpace(eventProvider))

	model = normalizeModelForProvider(provider, requestModel, responseModel)
	if provider == "" && requestModel != "" {
		parts := strings.SplitN(strings.TrimSpace(requestModel), ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
			provider = strings.ToLower(strings.TrimSpace(parts[0]))
		}
	}

	provider, model = inferProviderAndModel(provider, model)
	return provider, model
}

func normalizeModelForProvider(provider, requestModel, responseModel string) string {
	if strings.TrimSpace(requestModel) != "" {
		parts := strings.SplitN(requestModel, ":", 2)
		if len(parts) == 2 && strings.ToLower(strings.TrimSpace(parts[0])) == provider {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(requestModel)
	}
	if strings.TrimSpace(responseModel) != "" {
		parts := strings.SplitN(responseModel, ":", 2)
		if len(parts) == 2 && strings.ToLower(strings.TrimSpace(parts[0])) == provider {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(responseModel)
	}
	return ""
}

func inferProviderAndModel(provider, model string) (string, string) {
	switch provider {
	case "openai":
		trimmed := strings.ToLower(strings.TrimSpace(model))
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "deepseek" {
			return "deepseek", strings.TrimSpace(parts[1])
		}
		if strings.HasPrefix(trimmed, "deepseek") {
			return "deepseek", strings.TrimSpace(model)
		}
	}
	return provider, strings.TrimSpace(model)
}
