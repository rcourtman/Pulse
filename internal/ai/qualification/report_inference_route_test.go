package qualification

import "testing"

func TestInferenceRouteForProviderEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		baseURL  string
		want     string
	}{
		{name: "Zai coding plan", provider: "zai", baseURL: "https://api.z.ai/api/coding/paas/v4", want: "coding_plan_allowance"},
		{name: "Zai coding completions", provider: "ZAI", baseURL: "https://api.z.ai/api/coding/paas/v4/chat/completions", want: "coding_plan_allowance"},
		{name: "Zai metered API", provider: "zai", baseURL: "https://api.z.ai/api/paas/v4", want: "metered_api"},
		{name: "unparseable Zai endpoint", provider: "zai", baseURL: "://", want: "metered_api"},
		{name: "local subscription agent", provider: "codex-subscription", baseURL: "", want: "local_subscription_agent"},
		{name: "Ollama local model server", provider: "ollama", baseURL: "http://127.0.0.1:11434", want: "local_model_server"},
		{name: "OpenAI local vLLM loopback", provider: "openai", baseURL: "http://127.0.0.1:8000/v1", want: "local_model_server"},
		{name: "OpenAI local llama.cpp LAN", provider: "openai", baseURL: "http://192.168.1.42:8080/v1", want: "local_model_server"},
		{name: "OpenAI local .lan host", provider: "openai", baseURL: "http://vllm.lan:8000/v1", want: "local_model_server"},
		{name: "OpenAI official cloud API", provider: "openai", baseURL: "https://api.openai.com/v1", want: "metered_api"},
		{name: "OpenAI public custom gateway", provider: "openai", baseURL: "https://gateway.example.com/v1", want: "metered_api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferenceRouteForProviderEndpoint(tt.provider, tt.baseURL); got != tt.want {
				t.Fatalf("inferenceRouteForProviderEndpoint(%q, %q) = %q, want %q", tt.provider, tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestInferenceEngineForProviderEndpoint(t *testing.T) {
	if got := inferenceEngineForProviderEndpoint("ollama", ""); got != "ollama" {
		t.Fatalf("ollama engine = %q, want ollama", got)
	}
	if got := inferenceEngineForProviderEndpoint("openai", "http://127.0.0.1:8000/v1"); got != "openai_compatible_local" {
		t.Fatalf("local openai engine = %q, want openai_compatible_local", got)
	}
	if got := inferenceEngineForProviderEndpoint("openai", "https://api.openai.com/v1"); got != "" {
		t.Fatalf("cloud openai engine = %q, want empty", got)
	}
}
