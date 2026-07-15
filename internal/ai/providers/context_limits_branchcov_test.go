package providers

import "testing"

// TestContextWindowTokens_BranchCov_EmptyAndColonStripped exercises the
// early-return guard at context_limits.go:68 where extractModelName yields an
// empty string. This covers three ways to produce that empty model name:
//   - whitespace-only input
//   - a provider prefix whose right-hand side is empty after trimming
//   - a bare trailing colon with nothing after it
func TestContextWindowTokens_BranchCov_EmptyAndColonStripped(t *testing.T) {
	cases := []struct {
		name  string
		model string
	}{
		{"whitespace only", "   "},
		{"empty string", ""},
		{"prefix colon empty after trim", "anthropic:   "},
		{"prefix colon nothing after", "openai:"},
		{"bare colon", ":"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContextWindowTokens(tc.model)
			if got != DefaultContextWindow {
				t.Fatalf("ContextWindowTokens(%q) = %d, want %d (DefaultContextWindow)",
					tc.model, got, DefaultContextWindow)
			}
		})
	}
}

// TestContextWindowTokens_BranchCov_DateSuffixVariants hits the
// stripDateSuffix+exact-map branch at context_limits.go:76-79 for both the
// -YYYY-MM-DD and -YYYYMMDD suffix shapes.
func TestContextWindowTokens_BranchCov_DateSuffixVariants(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  int
	}{
		// -YYYY-MM-DD (11-char) suffix form.
		{"gpt-4o dashed date", "gpt-4o-2024-08-06", 128_000},
		{"claude-3-opus dashed date", "claude-3-opus-2024-02-29", 200_000},
		// -YYYYMMDD (9-char) suffix form.
		{"claude-opus-4 compact date", "claude-opus-4-20250514", 200_000},
		{"gemini-1.5-pro compact date", "gemini-1.5-pro-20240409", 2_097_152},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContextWindowTokens(tc.model)
			if got != tc.want {
				t.Fatalf("ContextWindowTokens(%q) = %d, want %d", tc.model, got, tc.want)
			}
		})
	}
}

// TestContextWindowTokens_BranchCov_EqualFoldMatch drives the EqualFold arm of
// caseInsensitiveMatch (context_limits.go:170). The exact map lookup and the
// case-sensitive longestPrefixMatch both miss because the only differing
// bytes are ASCII case, so resolution must come from strings.EqualFold.
func TestContextWindowTokens_BranchCov_EqualFoldMatch(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  int
	}{
		{"upper GPT-4O", "GPT-4O", 128_000},
		{"mixed Claude-Opus-4", "Claude-Opus-4", 200_000},
		// Preserves the mixed-case registry key "MiniMax-Text-01" via lowercased input.
		{"lower minimax", "minimax-text-01", 1_000_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContextWindowTokens(tc.model)
			if got != tc.want {
				t.Fatalf("ContextWindowTokens(%q) = %d, want %d", tc.model, got, tc.want)
			}
		})
	}
}

// TestContextWindowTokens_BranchCov_CaseInsensitivePrefixFallback exercises the
// fallback prefix phase of caseInsensitiveMatch (context_limits.go:175-184).
// The input differs from any known model in length and in case, so neither the
// exact map, the date strip, the case-sensitive longestPrefixMatch, nor
// EqualFold can resolve it; only the lowercased HasPrefix scan succeeds.
func TestContextWindowTokens_BranchCov_CaseInsensitivePrefixFallback(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  int
	}{
		{"upper Claude-3-5-Sonnet-Latest", "Claude-3-5-Sonnet-Latest", 200_000},
		{"upper GROK-3-MINI-HI", "GROK-3-MINI-HI", 131_072},
		{"upper GPT-4O-MINI-PREVIEW", "GPT-4O-MINI-PREVIEW", 128_000},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContextWindowTokens(tc.model)
			if got != tc.want {
				t.Fatalf("ContextWindowTokens(%q) = %d, want %d", tc.model, got, tc.want)
			}
		})
	}
}

// TestContextWindowTokens_BranchCov_GenuinelyUnknown hits the final
// DefaultContextWindow return at context_limits.go:99 for a model that no
// fuzzy strategy can resolve (no colon, no date suffix, no prefix in any case).
func TestContextWindowTokens_BranchCov_GenuinelyUnknown(t *testing.T) {
	cases := []struct {
		name  string
		model string
	}{
		{"nonsense", "zzz-not-a-real-model"},
		{"leading digits", "12345"},
		{"symbols only", "---"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ContextWindowTokens(tc.model)
			if got != DefaultContextWindow {
				t.Fatalf("ContextWindowTokens(%q) = %d, want %d (DefaultContextWindow)",
					tc.model, got, DefaultContextWindow)
			}
		})
	}
}

// TestExtractModelName_BranchCov covers every branch of extractModelName
// (context_limits.go:102-113): empty/whitespace guard, no-colon passthrough,
// and the colon-split + TrimSpace path including trailing-colon emptiness and
// only-the-first-colon-wins semantics.
func TestExtractModelName_BranchCov(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \t ", ""},
		{"no colon trimmed passthrough", "  gpt-4o  ", "gpt-4o"},
		{"simple prefix strip", "anthropic:claude-opus-4", "claude-opus-4"},
		{"prefix strip with inner whitespace", "anthropic:   claude-opus-4   ", "claude-opus-4"},
		{"bare colon yields empty", "anthropic:", ""},
		{"colon only yields empty", ":", ""},
		{"only first colon splits", "openai:gpt-4o:alias", "gpt-4o:alias"},
		{"leading colon keeps remainder", ":gpt-4o", "gpt-4o"},
		{"trailing colon yields empty", "gpt-4o:", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractModelName(tc.model)
			if got != tc.want {
				t.Fatalf("extractModelName(%q) = %q, want %q", tc.model, got, tc.want)
			}
		})
	}
}

// TestCaseInsensitiveMatch_BranchCov covers all three return sites of
// caseInsensitiveMatch (context_limits.go:168-189): the EqualFold exact match,
// the lowercased-prefix fallback, and the no-match (0,false) outcome.
func TestCaseInsensitiveMatch_BranchCov(t *testing.T) {
	cases := []struct {
		name      string
		model     string
		wantTok   int
		wantFound bool
	}{
		// EqualFold arm: identical length, differing ASCII case.
		{"equalfold upper GPT-4O", "GPT-4O", 128_000, true},
		{"equalfold lower minimax-text-01", "minimax-text-01", 1_000_000, true},
		// Lowercased prefix fallback: longer than any known model, case differs.
		{"prefix Claude-3-5-Sonnet-Latest", "Claude-3-5-Sonnet-Latest", 200_000, true},
		{"prefix GROK-3-MINI-hi", "GROK-3-MINI-hi", 131_072, true},
		// No match at all.
		{"no match nonsense", "zzz-not-a-real-model", 0, false},
		{"no match symbols", "---", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTok, gotFound := caseInsensitiveMatch(tc.model)
			if gotFound != tc.wantFound {
				t.Fatalf("caseInsensitiveMatch(%q) found = %v, want %v",
					tc.model, gotFound, tc.wantFound)
			}
			if gotFound && gotTok != tc.wantTok {
				t.Fatalf("caseInsensitiveMatch(%q) tokens = %d, want %d",
					tc.model, gotTok, tc.wantTok)
			}
			if !gotFound && gotTok != 0 {
				t.Fatalf("caseInsensitiveMatch(%q) tokens = %d, want 0 when not found",
					tc.model, gotTok)
			}
		})
	}
}

// TestIsDigits_BranchCov covers every branch of isDigits
// (context_limits.go:140-150): empty-string guard, the loop hit on a
// non-digit byte (both leading and trailing positions), the ASCII '-' case,
// and the all-digits success path.
func TestIsDigits_BranchCov(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"", false},    // empty guard
		{"0", true},    // single digit success
		{"123", true},  // multi-digit success
		{"2024", true}, // year-like success
		{"12a", false}, // trailing non-digit
		{"a1", false},  // leading non-digit
		{"-5", false},  // '-' is not a digit
		{"1 2", false}, // embedded space is not a digit
		{"１２", false},  // full-width digits are non-ASCII, must be rejected
	}

	for _, tc := range cases {
		got := isDigits(tc.s)
		if got != tc.want {
			t.Fatalf("isDigits(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}
