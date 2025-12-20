package cost

import "testing"

func TestResolveProviderAndModel(t *testing.T) {
	tests := []struct {
		name             string
		eventProvider    string
		requestModel     string
		responseModel    string
		expectedProvider string
		expectedModel    string
	}{
		{
			name:             "simple openai",
			eventProvider:    "openai",
			requestModel:     "gpt-4o",
			responseModel:    "",
			expectedProvider: "openai",
			expectedModel:    "gpt-4o",
		},
		{
			name:             "anthropic",
			eventProvider:    "anthropic",
			requestModel:     "claude-3-opus",
			responseModel:    "",
			expectedProvider: "anthropic",
			expectedModel:    "claude-3-opus",
		},
		{
			name:             "deepseek via openai format",
			eventProvider:    "openai",
			requestModel:     "deepseek:deepseek-chat",
			responseModel:    "",
			expectedProvider: "deepseek",
			expectedModel:    "deepseek-chat",
		},
		{
			name:             "deepseek with prefix",
			eventProvider:    "openai",
			requestModel:     "deepseek-reasoner",
			responseModel:    "",
			expectedProvider: "deepseek",
			expectedModel:    "deepseek-reasoner",
		},
		{
			name:             "empty provider inferred from model",
			eventProvider:    "",
			requestModel:     "openai:gpt-4",
			responseModel:    "",
			expectedProvider: "openai",
			expectedModel:    "openai:gpt-4", // Model is not parsed, just provider is inferred
		},
		{
			name:             "whitespace trimming",
			eventProvider:    "  OpenAI  ",
			requestModel:     "  gpt-4o  ",
			responseModel:    "",
			expectedProvider: "openai",
			expectedModel:    "gpt-4o",
		},
		{
			name:             "ollama provider",
			eventProvider:    "ollama",
			requestModel:     "llama3:8b",
			responseModel:    "",
			expectedProvider: "ollama",
			expectedModel:    "llama3:8b",
		},
		{
			name:             "gemini provider",
			eventProvider:    "gemini",
			requestModel:     "gemini-pro",
			responseModel:    "",
			expectedProvider: "gemini",
			expectedModel:    "gemini-pro",
		},
		{
			name:             "responseModel fallback",
			eventProvider:    "openai",
			requestModel:     "",
			responseModel:    "gpt-4o-2024-08-06",
			expectedProvider: "openai",
			expectedModel:    "gpt-4o-2024-08-06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model := ResolveProviderAndModel(tt.eventProvider, tt.requestModel, tt.responseModel)
			if provider != tt.expectedProvider {
				t.Errorf("provider = %q, want %q", provider, tt.expectedProvider)
			}
			if model != tt.expectedModel {
				t.Errorf("model = %q, want %q", model, tt.expectedModel)
			}
		})
	}
}

func TestInferProviderAndModel(t *testing.T) {
	tests := []struct {
		name             string
		provider         string
		model            string
		expectedProvider string
		expectedModel    string
	}{
		{
			name:             "openai with deepseek prefix",
			provider:         "openai",
			model:            "deepseek:deepseek-chat",
			expectedProvider: "deepseek",
			expectedModel:    "deepseek-chat",
		},
		{
			name:             "openai with deepseek model",
			provider:         "openai",
			model:            "deepseek-reasoner",
			expectedProvider: "deepseek",
			expectedModel:    "deepseek-reasoner",
		},
		{
			name:             "regular openai",
			provider:         "openai",
			model:            "gpt-4",
			expectedProvider: "openai",
			expectedModel:    "gpt-4",
		},
		{
			name:             "anthropic unchanged",
			provider:         "anthropic",
			model:            "claude-3-opus",
			expectedProvider: "anthropic",
			expectedModel:    "claude-3-opus",
		},
		{
			name:             "gemini unchanged",
			provider:         "gemini",
			model:            "gemini-pro",
			expectedProvider: "gemini",
			expectedModel:    "gemini-pro",
		},
		{
			name:             "whitespace trimmed",
			provider:         "openai",
			model:            "  gpt-4  ",
			expectedProvider: "openai",
			expectedModel:    "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model := inferProviderAndModel(tt.provider, tt.model)
			if provider != tt.expectedProvider {
				t.Errorf("provider = %q, want %q", provider, tt.expectedProvider)
			}
			if model != tt.expectedModel {
				t.Errorf("model = %q, want %q", model, tt.expectedModel)
			}
		})
	}
}
