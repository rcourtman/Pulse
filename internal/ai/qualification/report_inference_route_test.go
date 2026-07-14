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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferenceRouteForProviderEndpoint(tt.provider, tt.baseURL); got != tt.want {
				t.Fatalf("inferenceRouteForProviderEndpoint(%q, %q) = %q, want %q", tt.provider, tt.baseURL, got, tt.want)
			}
		})
	}
}
