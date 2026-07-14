package agentcapabilities

import "testing"

// This white-box test covers the three pure helper functions in markdown.go:
//   - markdownCountWord(count int) string
//   - pluralize(word string, count int) string
//   - mcpCapabilityCategoryHeading(category string, categories []CapabilityCategory) string
//
// The tests assert the exact returned string for every switch arm, the default
// arm, boundary values, and empty/nil inputs. No source behaviour is changed.

func TestMarkdownCountWord(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  string
	}{
		// Each named switch arm.
		{name: "zero", count: 0, want: "zero"},
		{name: "one", count: 1, want: "one"},
		{name: "two", count: 2, want: "two"},
		{name: "three", count: 3, want: "three"},
		{name: "four", count: 4, want: "four"},
		{name: "five upper boundary of word arms", count: 5, want: "five"},
		// Default arm: decimal formatting.
		{name: "six first default value", count: 6, want: "6"},
		{name: "negative default value", count: -1, want: "-1"},
		{name: "two digit default value", count: 42, want: "42"},
		{name: "large default value", count: 1000000, want: "1000000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := markdownCountWord(tc.count); got != tc.want {
				t.Fatalf("markdownCountWord(%d) = %q, want %q", tc.count, got, tc.want)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		name  string
		word  string
		count int
		want  string
	}{
		// count == 1 identity branch.
		{name: "count one returns word unchanged", word: "surface", count: 1, want: "surface"},
		{name: "count one with empty word", word: "", count: 1, want: ""},
		// else branch: append "s".
		{name: "count zero appends s", word: "surface", count: 0, want: "surfaces"},
		{name: "count two appends s", word: "surface", count: 2, want: "surfaces"},
		{name: "negative count appends s", word: "surface", count: -3, want: "surfaces"},
		{name: "empty word appends s", word: "", count: 5, want: "s"},
		// Confirms the implementation is a naive append: no "es"/"ies" rules.
		{name: "naive append does not special case box", word: "box", count: 3, want: "boxs"},
		{name: "count one does not special case box", word: "box", count: 1, want: "box"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pluralize(tc.word, tc.count); got != tc.want {
				t.Fatalf("pluralize(%q, %d) = %q, want %q", tc.word, tc.count, got, tc.want)
			}
		})
	}
}

func TestMCPCapabilityCategoryHeading(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		categories []CapabilityCategory
		want       string
	}{
		// Matched descriptor with a non-empty label returns the label.
		{
			name:       "matched descriptor non-empty label returns label",
			category:   "context",
			categories: []CapabilityCategory{{ID: "context", Label: "Context (read-only)"}},
			want:       "Context (read-only)",
		},
		// Matched descriptor with an empty label falls back to the raw id.
		{
			name:       "matched descriptor empty label returns raw category",
			category:   "context",
			categories: []CapabilityCategory{{ID: "context", Label: ""}},
			want:       "context",
		},
		// Matched descriptor with a whitespace-only label is treated as empty.
		{
			name:       "matched descriptor whitespace label returns raw category",
			category:   "context",
			categories: []CapabilityCategory{{ID: "context", Label: "   "}},
			want:       "context",
		},
		// The descriptor ID is trimmed before comparison.
		{
			name:       "matched descriptor id is trimmed before comparison",
			category:   "context",
			categories: []CapabilityCategory{{ID: "  context  ", Label: "Context"}},
			want:       "Context",
		},
		// First matching descriptor wins: an empty label on the first match
		// short-circuits and returns the raw id even if a later descriptor
		// carries a label.
		{
			name:       "first match with empty label wins over later label",
			category:   "context",
			categories: []CapabilityCategory{{ID: "context", Label: ""}, {ID: "context", Label: "Context"}},
			want:       "context",
		},
		// No match: the "uncategorized" sentinel maps to the title-cased fallback.
		{
			name:       "no match uncategorized sentinel returns title cased fallback",
			category:   "uncategorized",
			categories: []CapabilityCategory{{ID: "context", Label: "Context"}},
			want:       "Uncategorized",
		},
		// No match: any other id is returned verbatim.
		{
			name:       "no match unknown category returns raw category",
			category:   "custom",
			categories: []CapabilityCategory{{ID: "context", Label: "Context"}},
			want:       "custom",
		},
		// No match against an empty descriptor slice returns the raw category.
		{
			name:       "no match empty descriptor slice returns raw category",
			category:   "custom",
			categories: []CapabilityCategory{},
			want:       "custom",
		},
		// No match against a nil descriptor slice: the uncategorized fallback.
		{
			name:       "nil descriptor slice uncategorized sentinel",
			category:   "uncategorized",
			categories: nil,
			want:       "Uncategorized",
		},
		// An explicit "uncategorized" descriptor with an empty label returns the
		// lowercase raw id, NOT the title-cased "Uncategorized" fallback,
		// because the descriptor loop takes precedence.
		{
			name:       "explicit uncategorized descriptor empty label returns raw id",
			category:   "uncategorized",
			categories: []CapabilityCategory{{ID: "uncategorized", Label: ""}},
			want:       "uncategorized",
		},
		// An explicit "uncategorized" descriptor with a label returns the label.
		{
			name:       "explicit uncategorized descriptor with label returns label",
			category:   "uncategorized",
			categories: []CapabilityCategory{{ID: "uncategorized", Label: "Miscellaneous"}},
			want:       "Miscellaneous",
		},
		// Empty category with no matching descriptor returns the empty string.
		{
			name:       "empty category no match returns empty",
			category:   "",
			categories: []CapabilityCategory{{ID: "context", Label: "Context"}},
			want:       "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mcpCapabilityCategoryHeading(tc.category, tc.categories); got != tc.want {
				t.Fatalf("mcpCapabilityCategoryHeading(%q, %+v) = %q, want %q",
					tc.category, tc.categories, got, tc.want)
			}
		})
	}
}
