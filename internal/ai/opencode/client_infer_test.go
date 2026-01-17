package opencode

import (
	"testing"
)

func TestInferProviderFromModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		// Anthropic/Claude models
		{"claude-opus-4-5-20251101", "anthropic"},
		{"claude-3-5-haiku-latest", "anthropic"},
		{"Claude-Sonnet", "anthropic"},
		{"CLAUDE-3", "anthropic"},

		// OpenAI models
		{"gpt-4o", "openai"},
		{"gpt-4-turbo", "openai"},
		{"o1-preview", "openai"},
		{"o3-mini", "openai"},

		// Google/Gemini models
		{"gemini-2.5-flash", "google"},
		{"gemini-pro", "google"},
		{"Gemini-Ultra", "google"},

		// DeepSeek models
		{"deepseek-reasoner", "deepseek"},
		{"deepseek-coder", "deepseek"},

		// Ollama/local models
		{"llama3", "ollama"},
		{"llama-3.1-70b", "ollama"},
		{"mistral-7b", "ollama"},
		{"codellama-13b", "ollama"},
		{"phi-3", "ollama"},
		{"qwen2.5", "ollama"},

		// Unknown models - should return empty string
		{"unknown-model", ""},
		{"random", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := inferProviderFromModel(tt.model)
			if result != tt.expected {
				t.Errorf("inferProviderFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}
