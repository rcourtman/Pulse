package main

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/eval"
)

// These tests target the pure parser/helper functions in main.go (the
// auto-selection and CLI machinery). They were chosen because they have no
// network or model dependency, so every branch can be driven deterministically.
//
// fetchAutoModels is intentionally NOT covered here: it issues a live HTTP
// request and the package has no mock server. listScenarios is also skipped:
// it only prints to stdout and this package has no pre-existing convention for
// capturing os.Stdout.

// TestBranchcov0723AmEnvBool pins every arm of envBool. The source uses
// os.LookupEnv and lowercases+trims the value before a switch, so an unset
// key and an empty-string value both fall through to the default arm and
// return (false, false) — i.e. unset and empty are NOT distinguished.
func TestBranchcov0723AmEnvBool(t *testing.T) {
	const key = "EVAL_TEST_BRANCHCOV_BOOL"

	t.Run("unset returns false false", func(t *testing.T) {
		// Use a key that was never set. t.Setenv cannot express "unset", so
		// we rely on a uniquely-named key the environment does not carry.
		got, ok := envBool("EVAL_TEST_BRANCHCOV_BOOL_DEFINITELY_UNSET")
		if ok {
			t.Fatalf("unset key ok=true, want false")
		}
		if got {
			t.Fatalf("unset key got=true, want false")
		}
	})

	truthy := []string{"1", "true", "TRUE", "True", "yes", "YES", "y", "Y", "on", "ON", "  true  ", "\ton\n"}
	for _, v := range truthy {
		v := v
		t.Run("truthy spelling "+v, func(t *testing.T) {
			t.Setenv(key, v)
			got, ok := envBool(key)
			if !ok {
				t.Fatalf("envBool(%q) ok=false, want true", v)
			}
			if !got {
				t.Fatalf("envBool(%q) got=false, want true", v)
			}
		})
	}

	falsy := []string{"0", "false", "FALSE", "False", "no", "NO", "n", "N", "off", "OFF", "  off  "}
	for _, v := range falsy {
		v := v
		t.Run("falsy spelling "+v, func(t *testing.T) {
			t.Setenv(key, v)
			got, ok := envBool(key)
			if !ok {
				t.Fatalf("envBool(%q) ok=false, want true", v)
			}
			if got {
				t.Fatalf("envBool(%q) got=true, want false", v)
			}
		})
	}

	t.Run("garbage returns false false", func(t *testing.T) {
		t.Setenv(key, "maybe")
		got, ok := envBool(key)
		if ok {
			t.Fatalf("garbage ok=true, want false")
		}
		if got {
			t.Fatalf("garbage got=true, want false")
		}
	})

	t.Run("empty string set is not distinguished from unset", func(t *testing.T) {
		t.Setenv(key, "")
		got, ok := envBool(key)
		// Source: LookupEnv returns ("", true), then TrimSpace("")=="" which
		// hits the default arm of the switch -> (false, false). Identical to
		// the unset path.
		if ok {
			t.Fatalf("empty-string value ok=true, want false (should collapse to unset)")
		}
		if got {
			t.Fatalf("empty-string value got=true, want false")
		}
	})
}

// TestBranchcov0723AmEnvInt pins envInt, which uses fmt.Sscanf "%d". The
// "%d" verb is lenient: it parses a leading integer out of "12abc" (-> 12),
// accepts "+7" and "-5", but errors (and yields ok=false) on pure garbage,
// empty, and overflow.
func TestBranchcov0723AmEnvInt(t *testing.T) {
	const key = "EVAL_TEST_BRANCHCOV_INT"

	t.Run("unset returns 0 false", func(t *testing.T) {
		got, ok := envInt("EVAL_TEST_BRANCHCOV_INT_DEFINITELY_UNSET")
		if ok {
			t.Fatalf("unset ok=true, want false")
		}
		if got != 0 {
			t.Fatalf("unset got=%d, want 0", got)
		}
	})

	cases := []struct {
		name   string
		value  string
		want   int
		wantOK bool
	}{
		{"plain positive", "42", 42, true},
		{"negative", "-5", -5, true},
		{"plus sign", "+7", 7, true},
		{"surrounding whitespace trimmed by source", "  42  ", 42, true},
		{"leading integer parsed out of trailing garbage", "12abc", 12, true},
		{"hex prefix parses leading zero then stops", "0x10", 0, true},
		{"pure garbage fails", "abc", 0, false},
		{"empty string set fails like unset", "", 0, false},
		{"overflow fails", "99999999999999999999999", 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.value)
			got, ok := envInt(key)
			if ok != tc.wantOK {
				t.Fatalf("envInt(%q) ok=%v, want %v", tc.value, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("envInt(%q) got=%d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

// TestBranchcov0723AmParseModelList pins parseModelList: trims, splits on
// comma, drops empty fields, and does NOT dedup.
func TestBranchcov0723AmParseModelList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty is nil", "", nil},
		{"whitespace only is nil", "   \t\n  ", nil},
		{"single", "openai:gpt-4", []string{"openai:gpt-4"}},
		{"several", "openai:gpt-4, anthropic:claude ,gemini:pro", []string{"openai:gpt-4", "anthropic:claude", "gemini:pro"}},
		{"duplicates kept (no dedup)", "openai:a,openai:a,openai:b", []string{"openai:a", "openai:a", "openai:b"}},
		{"leading separator drops empty first field", ",openai:a,openai:b", []string{"openai:a", "openai:b"}},
		{"trailing separator drops empty last field", "openai:a,openai:b,", []string{"openai:a", "openai:b"}},
		{"interior empty fields dropped", "openai:a,,openai:b", []string{"openai:a", "openai:b"}},
		// All-comma input is NOT nil: after trimming the raw string is still
		// ",,," (non-empty), so the source reaches `make` and returns a
		// non-nil empty slice. Only truly empty/whitespace input returns nil.
		{"all-empty fields yields non-nil empty slice", ",,,", []string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parseModelList(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("parseModelList(%q)=%v, want %v", tc.raw, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("parseModelList(%q)[%d]=%q, want %q (full: %v)", tc.raw, i, got[i], tc.want[i], got)
				}
			}
			if tc.want == nil && got != nil {
				t.Fatalf("parseModelList(%q)=%v, want nil", tc.raw, got)
			}
		})
	}
}

// TestBranchcov0723AmParseProviderFilter pins parseProviderFilter: it does
// NOT lowercase, so mixed-case entries become distinct keys. Returns nil for
// empty input.
func TestBranchcov0723AmParseProviderFilter(t *testing.T) {
	t.Run("empty is nil", func(t *testing.T) {
		if got := parseProviderFilter(""); got != nil {
			t.Fatalf("parseProviderFilter(\"\")=%v, want nil", got)
		}
	})
	t.Run("whitespace only is nil", func(t *testing.T) {
		if got := parseProviderFilter("   "); got != nil {
			t.Fatalf("parseProviderFilter(\"   \")=%v, want nil", got)
		}
	})
	t.Run("single provider", func(t *testing.T) {
		got := parseProviderFilter("openai")
		if len(got) != 1 || !got["openai"] {
			t.Fatalf("parseProviderFilter(\"openai\")=%v, want {openai:true}", got)
		}
	})
	t.Run("several providers trimmed", func(t *testing.T) {
		got := parseProviderFilter("openai, anthropic , gemini")
		want := map[string]bool{"openai": true, "anthropic": true, "gemini": true}
		if len(got) != 3 || !got["openai"] || !got["anthropic"] || !got["gemini"] {
			t.Fatalf("parseProviderFilter got=%v, want %v", got, want)
		}
	})
	t.Run("duplicates collapse into one key", func(t *testing.T) {
		got := parseProviderFilter("openai,openai,openai")
		if len(got) != 1 || !got["openai"] {
			t.Fatalf("duplicates got=%v, want single {openai:true}", got)
		}
	})
	t.Run("NOT lowercased: case variants are distinct keys", func(t *testing.T) {
		// The whole point of pinning this: parseProviderFilter preserves case,
		// unlike parseExcludeKeywords which lowercases.
		got := parseProviderFilter("OpenAI,openai,OPENAI")
		if len(got) != 3 {
			t.Fatalf("case variants got=%v, want 3 distinct keys", got)
		}
		if !got["OpenAI"] || !got["openai"] || !got["OPENAI"] {
			t.Fatalf("case variants got=%v, want OpenAI/openai/OPENAI all true", got)
		}
	})
	t.Run("empty fields dropped", func(t *testing.T) {
		got := parseProviderFilter(",openai,,")
		if len(got) != 1 || !got["openai"] {
			t.Fatalf("got=%v, want {openai:true}", got)
		}
	})
}

// TestBranchcov0723AmParseProviderFilterWithDefault pins the ONLY behaviour
// that distinguishes this function from parseProviderFilter: empty/whitespace
// input returns the fixed six-provider default map; any other input
// delegates to parseProviderFilter unchanged.
func TestBranchcov0723AmParseProviderFilterWithDefault(t *testing.T) {
	expectedDefault := map[string]bool{
		"openai":     true,
		"openrouter": true,
		"anthropic":  true,
		"deepseek":   true,
		"gemini":     true,
		"ollama":     true,
	}
	t.Run("empty input yields the six-provider default", func(t *testing.T) {
		got := parseProviderFilterWithDefault("")
		if len(got) != len(expectedDefault) {
			t.Fatalf("default len=%d, want %d (%v)", len(got), len(expectedDefault), got)
		}
		for k := range expectedDefault {
			if !got[k] {
				t.Fatalf("default map missing %q, got=%v", k, got)
			}
		}
	})
	t.Run("whitespace input also yields the default", func(t *testing.T) {
		got := parseProviderFilterWithDefault("   \t ")
		for k := range expectedDefault {
			if !got[k] {
				t.Fatalf("whitespace default missing %q, got=%v", k, got)
			}
		}
	})
	t.Run("non-empty delegates to parseProviderFilter (not the default)", func(t *testing.T) {
		// A single custom provider must NOT be merged with the defaults.
		got := parseProviderFilterWithDefault("acme")
		if len(got) != 1 || !got["acme"] {
			t.Fatalf("non-empty got=%v, want {acme:true} only (no defaults)", got)
		}
		if got["openai"] {
			t.Fatalf("non-empty input leaked default openai into result: %v", got)
		}
	})
	t.Run("non-empty lowercases-not, matching parseProviderFilter", func(t *testing.T) {
		got := parseProviderFilterWithDefault("OpenAI")
		if len(got) != 1 || !got["OpenAI"] || got["openai"] {
			t.Fatalf("case preserved? got=%v", got)
		}
	})
}

// TestBranchcov0723AmParseExcludeKeywords pins parseExcludeKeywords, whose
// normalisation differs from parseProviderFilter: it LOWERCASES each entry,
// and it has a default keyword list plus a "disable" sentinel.
func TestBranchcov0723AmParseExcludeKeywords(t *testing.T) {
	t.Run("empty yields the default keyword list", func(t *testing.T) {
		got := parseExcludeKeywords("")
		// Pin a representative subset of the well-known defaults plus the
		// exact length so additions are noticed.
		wantCount := 14
		if len(got) != wantCount {
			t.Fatalf("default exclude list len=%d, want %d (%v)", len(got), wantCount, got)
		}
		mustContain := []string{"codex", "image", "vision", "video", "audio", "speech", "embed", "embedding", "moderation", "rerank", "tts", "realtime", "transcribe", "openai:gpt-5.2-pro"}
		gotSet := make(map[string]bool, len(got))
		for _, k := range got {
			gotSet[k] = true
		}
		for _, w := range mustContain {
			if !gotSet[w] {
				t.Fatalf("default list missing %q, got=%v", w, got)
			}
		}
	})
	t.Run("whitespace yields the default keyword list", func(t *testing.T) {
		got := parseExcludeKeywords("  \t ")
		if len(got) == 0 {
			t.Fatalf("whitespace should yield default list, got empty")
		}
	})

	disableSentinels := []string{"0", "false", "off", "none", "FALSE", "Off", "NONE"}
	for _, s := range disableSentinels {
		s := s
		t.Run("disable sentinel "+s+" returns nil", func(t *testing.T) {
			got := parseExcludeKeywords(s)
			if got != nil {
				t.Fatalf("parseExcludeKeywords(%q)=%v, want nil", s, got)
			}
		})
	}

	t.Run("single keyword lowercased", func(t *testing.T) {
		got := parseExcludeKeywords("Image")
		if len(got) != 1 || got[0] != "image" {
			t.Fatalf("got=%v, want [image]", got)
		}
	})
	t.Run("several keywords trimmed and lowercased", func(t *testing.T) {
		got := parseExcludeKeywords("Image, VISION ,Audio")
		want := []string{"image", "vision", "audio"}
		if len(got) != len(want) {
			t.Fatalf("got=%v, want %v", got, want)
		}
		for i, w := range want {
			if got[i] != w {
				t.Fatalf("got[%d]=%q, want %q (full: %v)", i, got[i], w, got)
			}
		}
	})
	t.Run("empty fields dropped", func(t *testing.T) {
		got := parseExcludeKeywords(",image,,")
		if len(got) != 1 || got[0] != "image" {
			t.Fatalf("got=%v, want [image]", got)
		}
	})
	t.Run("custom keyword with colon preserved after lowercasing", func(t *testing.T) {
		// A custom exclusion like a model id should survive intact (lowercased).
		got := parseExcludeKeywords("OpenAI:GPT-4-Vision")
		if len(got) != 1 || got[0] != "openai:gpt-4-vision" {
			t.Fatalf("got=%v, want [openai:gpt-4-vision]", got)
		}
	})
}

// TestBranchcov0723AmHasAnyKeyword pins hasAnyKeyword. The source builds
// target = lowercased(ID + " " + Name + " " + Description) and applies
// case-sensitive strings.Contains against each (un-lowercased) keyword, so a
// mixed-case keyword will NOT match even when its lowercase would.
func TestBranchcov0723AmHasAnyKeyword(t *testing.T) {
	model := apiModelInfo{
		ID:          "openai:probe-id-123",
		Name:        "SpecialNameToken",
		Description: "UniqueDescToken here",
	}

	t.Run("nil keywords false", func(t *testing.T) {
		if hasAnyKeyword(model, nil) {
			t.Fatalf("nil keywords -> true, want false")
		}
	})
	t.Run("empty keywords slice false", func(t *testing.T) {
		if hasAnyKeyword(model, []string{}) {
			t.Fatalf("empty slice -> true, want false")
		}
	})
	t.Run("only empty-string keywords skipped -> false", func(t *testing.T) {
		if hasAnyKeyword(model, []string{"", "", ""}) {
			t.Fatalf("only-empty keywords -> true, want false")
		}
	})
	t.Run("keyword matching ID field", func(t *testing.T) {
		if !hasAnyKeyword(model, []string{"probe-id-123"}) {
			t.Fatalf("keyword in ID should match")
		}
	})
	t.Run("keyword matching Name field", func(t *testing.T) {
		if !hasAnyKeyword(model, []string{"specialnametoken"}) {
			t.Fatalf("lowercased keyword in Name should match")
		}
	})
	t.Run("keyword matching Description field", func(t *testing.T) {
		if !hasAnyKeyword(model, []string{"uniquedesctoken"}) {
			t.Fatalf("lowercased keyword in Description should match")
		}
	})
	t.Run("mixed-case keyword does NOT match (target is lowercased, keyword is not)", func(t *testing.T) {
		// "SpecialNameToken" lowercased becomes "specialnametoken" in target;
		// Contains is case-sensitive, so the uppercase keyword fails.
		if hasAnyKeyword(model, []string{"SpecialNameToken"}) {
			t.Fatalf("mixed-case keyword should not match lowercased target")
		}
	})
	t.Run("near-miss non-existent keyword false", func(t *testing.T) {
		if hasAnyKeyword(model, []string{"zzz-not-present-anywhere"}) {
			t.Fatalf("non-existent keyword should not match")
		}
	})
	t.Run("empty keyword alongside a real match still matches", func(t *testing.T) {
		if !hasAnyKeyword(model, []string{"", "probe-id-123"}) {
			t.Fatalf("empty+real keyword should match on the real one")
		}
	})
}

// TestBranchcov0723AmCountProvider pins countProvider: matching is a strict
// prefix test on "<provider>:", so a provider name appearing anywhere other
// than the prefix is NOT counted.
func TestBranchcov0723AmCountProvider(t *testing.T) {
	t.Run("empty provider returns 0", func(t *testing.T) {
		if got := countProvider([]string{"openai:a", "openai:b"}, ""); got != 0 {
			t.Fatalf("empty provider -> %d, want 0", got)
		}
	})
	t.Run("empty models returns 0", func(t *testing.T) {
		if got := countProvider(nil, "openai"); got != 0 {
			t.Fatalf("nil models -> %d, want 0", got)
		}
	})
	t.Run("no match returns 0", func(t *testing.T) {
		got := countProvider([]string{"anthropic:claude", "gemini:pro"}, "openai")
		if got != 0 {
			t.Fatalf("no match -> %d, want 0", got)
		}
	})
	t.Run("several matches counted", func(t *testing.T) {
		got := countProvider([]string{"openai:gpt-4", "openai:gpt-3.5", "anthropic:claude", "openai:o1"}, "openai")
		if got != 3 {
			t.Fatalf("got %d, want 3", got)
		}
	})
	t.Run("provider substring in non-prefix position not counted", func(t *testing.T) {
		// HasPrefix("foo-openai-bar", "openai:") is false.
		got := countProvider([]string{"foo-openai-bar", "prefix:openai:thing"}, "openai")
		if got != 0 {
			t.Fatalf("non-prefix occurrence -> %d, want 0", got)
		}
	})
	t.Run("bare provider name without colon not counted", func(t *testing.T) {
		// "openai" does not start with "openai:".
		got := countProvider([]string{"openai", "openaiproxy:x"}, "openai")
		if got != 0 {
			t.Fatalf("bare provider name -> %d, want 0", got)
		}
	})
}

// TestBranchcov0723AmSelectionReason covers all six reachable reason strings
// of selectionReason by driving (model.Notable, model.CreatedAt, stat.Notable)
// through every combination.
func TestBranchcov0723AmSelectionReason(t *testing.T) {
	// 1704067200 = 2024-01-01 00:00:00 UTC -> "2024-01-02"? Verify:
	// time.Unix(1704067200,0).UTC() = 2024-01-01 00:00:00 -> Format "2006-01-02" = "2024-01-01".
	const ts2024 = 1704067200
	cases := []struct {
		name  string
		model apiModelInfo
		stat  providerStats
		want  string
	}{
		{
			name:  "notable stat zero and model notable",
			model: apiModelInfo{Notable: true, CreatedAt: ts2024},
			stat:  providerStats{Total: 1, Notable: 0},
			want:  "no notable models; notable",
		},
		{
			name:  "notable stat zero and model not notable with created_at",
			model: apiModelInfo{Notable: false, CreatedAt: ts2024},
			stat:  providerStats{Total: 1, Notable: 0},
			want:  "no notable models; created_at=2024-01-01",
		},
		{
			name:  "notable stat zero and model not notable without created_at (fallback)",
			model: apiModelInfo{Notable: false, CreatedAt: 0},
			stat:  providerStats{Total: 1, Notable: 0},
			want:  "no notable models; fallback",
		},
		{
			name:  "stat has notable and model notable",
			model: apiModelInfo{Notable: true, CreatedAt: ts2024},
			stat:  providerStats{Total: 2, Notable: 1},
			want:  "notable",
		},
		{
			name:  "stat has notable and model not notable with created_at",
			model: apiModelInfo{Notable: false, CreatedAt: ts2024},
			stat:  providerStats{Total: 2, Notable: 1},
			want:  "created_at=2024-01-01",
		},
		{
			name:  "stat has notable and model not notable without created_at (fallback)",
			model: apiModelInfo{Notable: false, CreatedAt: 0},
			stat:  providerStats{Total: 2, Notable: 1},
			want:  "fallback",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := selectionReason(tc.model, tc.stat)
			if got != tc.want {
				t.Fatalf("selectionReason(...) = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBranchcov0723AmSortedProviders pins the exact sort (ascending string
// order over the map keys) plus determinism across randomised map iteration.
func TestBranchcov0723AmSortedProviders(t *testing.T) {
	t.Run("nil map returns empty slice", func(t *testing.T) {
		got := sortedProviders(nil)
		if len(got) != 0 {
			t.Fatalf("nil map -> %v, want empty", got)
		}
	})
	t.Run("empty map returns empty slice", func(t *testing.T) {
		got := sortedProviders(map[string]providerStats{})
		if len(got) != 0 {
			t.Fatalf("empty map -> %v, want empty", got)
		}
	})
	t.Run("exact ascending order regardless of insertion", func(t *testing.T) {
		stats := map[string]providerStats{
			"openrouter": {Total: 2},
			"openai":     {Total: 5},
			"anthropic":  {Total: 1},
			"deepseek":   {Total: 3},
			"ollama":     {Total: 1},
			"gemini":     {Total: 1},
		}
		got := sortedProviders(stats)
		want := []string{"anthropic", "deepseek", "gemini", "ollama", "openai", "openrouter"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("index %d got %q, want ordering %v (full got=%v)", i, got[i], want, got)
			}
		}
	})
	t.Run("deterministic across many calls over random map iteration", func(t *testing.T) {
		stats := map[string]providerStats{
			"openrouter": {},
			"openai":     {},
			"anthropic":  {},
			"deepseek":   {},
			"ollama":     {},
			"gemini":     {},
			"mistral":    {},
			"cohere":     {},
			"meta":       {},
			"ai21":       {},
		}
		first := sortedProviders(stats)
		firstKey := strings.Join(first, ",")
		for i := 0; i < 200; i++ {
			got := sortedProviders(stats)
			if k := strings.Join(got, ","); k != firstKey {
				t.Fatalf("iteration %d produced %v, want deterministic %v", i, got, first)
			}
		}
	})
}

// TestBranchcov0723AmGetPatrolScenarios asserts the exact scenario NAMES for
// each switch arm, including the patrol-quality alias and the empty/unknown
// fallback to nil.
func TestBranchcov0723AmGetPatrolScenarios(t *testing.T) {
	names := func(scenarios []eval.PatrolScenario) []string {
		out := make([]string, len(scenarios))
		for i, s := range scenarios {
			out[i] = s.Name
		}
		return out
	}
	t.Run("patrol returns all four in declared order", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol"))
		want := []string{"Patrol Basic Run", "Patrol Investigation Quality", "Patrol Finding Quality", "Patrol Signal Coverage"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("index %d got %q, want %q (full: %v)", i, got[i], want[i], got)
			}
		}
	})
	t.Run("patrol-basic", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol-basic"))
		want := []string{"Patrol Basic Run"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("patrol-investigation", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol-investigation"))
		want := []string{"Patrol Investigation Quality"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("patrol-finding-quality", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol-finding-quality"))
		want := []string{"Patrol Finding Quality"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("patrol-signal-coverage", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol-signal-coverage"))
		want := []string{"Patrol Signal Coverage"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("patrol-quality alias maps to signal coverage", func(t *testing.T) {
		got := names(getPatrolScenarios("patrol-quality"))
		want := []string{"Patrol Signal Coverage"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("unknown name returns nil", func(t *testing.T) {
		if got := getPatrolScenarios("does-not-exist"); got != nil {
			t.Fatalf("unknown -> %v, want nil", got)
		}
	})
	t.Run("empty name returns nil", func(t *testing.T) {
		if got := getPatrolScenarios(""); got != nil {
			t.Fatalf("empty -> %v, want nil", got)
		}
	})
}

// TestBranchcov0723AmGetScenarios asserts the scenario NAMES for a single
// scenario arm, two collection arms (all and matrix), and the empty/unknown
// fallback to nil.
func TestBranchcov0723AmGetScenarios(t *testing.T) {
	names := func(scenarios []eval.Scenario) []string {
		out := make([]string, len(scenarios))
		for i, s := range scenarios {
			out[i] = s.Name
		}
		return out
	}
	t.Run("smoke returns single Quick Smoke Test", func(t *testing.T) {
		got := names(getScenarios("smoke"))
		want := []string{"Quick Smoke Test"}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("all returns the 11 basic scenarios in declared order", func(t *testing.T) {
		got := names(getScenarios("all"))
		want := []string{
			"Quick Smoke Test",
			"Read-Only Infrastructure",
			"Routing Validation",
			"Routing Mismatch Recovery",
			"Log Tailing (Bounded)",
			"Read-Only Violation Recovery",
			"Search Then Get By ID",
			"Ambiguous Resource Disambiguation",
			"Context Target Carryover",
			"Resource Context Handoff",
			"Infrastructure Discovery",
		}
		if len(got) != len(want) {
			t.Fatalf("all -> %d scenarios %v, want %d (%v)", len(got), got, len(want), want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("all[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
			}
		}
	})
	t.Run("matrix returns smoke + readonly only", func(t *testing.T) {
		got := names(getScenarios("matrix"))
		want := []string{"Quick Smoke Test", "Read-Only Infrastructure"}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
	t.Run("unknown name returns nil", func(t *testing.T) {
		if got := getScenarios("totally-unknown-scenario"); got != nil {
			t.Fatalf("unknown -> %v, want nil", got)
		}
	})
	t.Run("empty name returns nil", func(t *testing.T) {
		if got := getScenarios(""); got != nil {
			t.Fatalf("empty -> %v, want nil", got)
		}
	})
}
