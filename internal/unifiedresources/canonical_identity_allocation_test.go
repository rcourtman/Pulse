package unifiedresources

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// Independent original implementation. Lowercasing is intentionally not
// EqualFold: those operations differ for some Unicode identities.
func uniqueTrimmedMapReference(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		aliases = append(aliases, trimmed)
	}
	return aliases
}
func TestCanonicalAliasesBoundedScanMatchesOriginalMap(t *testing.T) {
	cases := [][]string{nil, {}, {"", "  ", "\t"}, {" Host ", "host", "HOST", "other"}, {"K", "K", "k", "İ", "i", "Σ", "σ", "ς", "ſ", "S", "s"}}
	for _, count := range []int{7, 8, 9, 16, 128, 1024} {
		var values []string
		for i := 0; i < count; i++ {
			value := fmt.Sprintf("Alias-%d", i)
			values = append(values, " ", value, strings.ToLower(value), " "+value+" ")
		}
		cases = append(cases, values)
	}
	rng := rand.New(rand.NewSource(2199))
	alphabet := []string{"", " ", "Host", "host", "HOST", "agent:lab", " vm:100 ", "K", "K", "İ", "i", "Σ", "σ", "ς", "ſ", "S"}
	for i := 0; i < 200; i++ {
		values := make([]string, rng.Intn(80))
		for j := range values {
			values[j] = alphabet[rng.Intn(len(alphabet))]
		}
		cases = append(cases, values)
	}
	for i, values := range cases {
		got, want := uniqueTrimmed(values...), uniqueTrimmedMapReference(values...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: got %q, want %q", i, got, want)
		}
		if len(values) > 0 && len(got) > 0 {
			original := values[0]
			got[0] = "caller-edit"
			if values[0] != original {
				t.Fatal("aliases share caller storage")
			}
		}
	}
}
func TestCanonicalAliasesSmallVocabularyAvoidsPlaceholderMap(t *testing.T) {
	values := []string{"lab", "", "", "LAB", "agent:lab", "", "", "agent:lab", "lab.example", "", "", "", "", "lab.example", "", ""}
	optimized := testing.AllocsPerRun(100, func() { _ = uniqueTrimmed(values...) })
	original := testing.AllocsPerRun(100, func() { _ = uniqueTrimmedMapReference(values...) })
	if optimized >= original {
		t.Fatalf("common alias list allocations %.0f, original %.0f", optimized, original)
	}
	t.Logf("same alias vocabulary: allocations %.0f -> %.0f", original, optimized)
}
func BenchmarkCanonicalAliasesVocabulary(b *testing.B) {
	for _, size := range []int{4, 8, 9, 128} {
		values := make([]string, 0, size*3)
		for i := 0; i < size; i++ {
			values = append(values, fmt.Sprintf("alias-%d", i), "", fmt.Sprintf("ALIAS-%d", i))
		}
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = uniqueTrimmed(values...)
			}
		})
	}
}
