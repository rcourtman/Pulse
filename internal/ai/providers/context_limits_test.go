package providers

import "testing"

func TestContextWindowTokens_ExactMatch(t *testing.T) {
	testCases := []struct {
		model string
		want  int
	}{
		{model: "claude-opus-4", want: 200_000},
		{model: "gpt-4", want: 8_192},
		{model: "gemini-1.5-pro", want: 2_097_152},
	}

	for _, tc := range testCases {
		got := ContextWindowTokens(tc.model)
		if got != tc.want {
			t.Fatalf("ContextWindowTokens(%q) = %d, want %d", tc.model, got, tc.want)
		}
	}
}

func TestContextWindowTokens_WithProviderPrefix(t *testing.T) {
	got := ContextWindowTokens("anthropic:claude-sonnet-4-20250514")
	want := 200_000
	if got != want {
		t.Fatalf("ContextWindowTokens(with provider prefix) = %d, want %d", got, want)
	}
}

func TestContextWindowTokens_DateSuffix(t *testing.T) {
	got := ContextWindowTokens("gpt-4o-2024-08-06")
	want := 128_000
	if got != want {
		t.Fatalf("ContextWindowTokens(with date suffix) = %d, want %d", got, want)
	}
}

func TestContextWindowTokens_PrefixMatch(t *testing.T) {
	got := ContextWindowTokens("claude-3-5-sonnet-latest")
	want := 200_000
	if got != want {
		t.Fatalf("ContextWindowTokens(prefix match) = %d, want %d", got, want)
	}
}

func TestContextWindowTokens_CaseInsensitive(t *testing.T) {
	got := ContextWindowTokens("minimax-text-01")
	want := 1_000_000
	if got != want {
		t.Fatalf("ContextWindowTokens(case insensitive) = %d, want %d", got, want)
	}
}

func TestContextWindowTokens_Unknown(t *testing.T) {
	got := ContextWindowTokens("unknown:model-does-not-exist")
	want := DefaultContextWindow
	if got != want {
		t.Fatalf("ContextWindowTokens(unknown model) = %d, want %d", got, want)
	}
}
