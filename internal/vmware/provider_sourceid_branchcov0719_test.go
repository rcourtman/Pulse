package vmware

import (
	"strings"
	"testing"
)

// TestSourceID exercises the exported SourceID helper, which delegates to
// vmwareSourceID -> filterNonEmptyStrings -> strings.Join(parts, ":").
//
// filterNonEmptyStrings (a) trims whitespace on each component, (b) drops
// any component that is empty after trimming, and (c) deduplicates the
// remaining components case-insensitively, preserving the first occurrence.
// The cases below drive each of those branches through the public SourceID
// entry point so coverage lands on the named target function.
func TestSourceID(t *testing.T) {
	cases := []struct {
		name            string
		connectionID    string
		entityType      string
		managedObjectID string
		want            string
	}{
		{
			name:            "all three populated joins with colon",
			connectionID:    "vc-1",
			entityType:      "host",
			managedObjectID: "host-101",
			want:            "vc-1:host:host-101",
		},
		{
			name:            "empty middle component is dropped",
			connectionID:    "vc-1",
			entityType:      "",
			managedObjectID: "vm-201",
			want:            "vc-1:vm-201",
		},
		{
			name:            "whitespace-only component is treated as empty",
			connectionID:    "vc-1",
			entityType:      "   ",
			managedObjectID: "vm-201",
			want:            "vc-1:vm-201",
		},
		{
			name:            "all components empty returns empty string",
			connectionID:    "",
			entityType:      "",
			managedObjectID: "",
			want:            "",
		},
		{
			name:            "leading and trailing whitespace trimmed from each component",
			connectionID:    "  vc-1  ",
			entityType:      "\thost\n",
			managedObjectID: " host-101 ",
			want:            "vc-1:host:host-101",
		},
		{
			name:            "case-insensitive duplicate components collapse to first occurrence",
			connectionID:    "VC-1",
			entityType:      "vc-1",
			managedObjectID: "VC-1",
			want:            "VC-1",
		},
		{
			name:            "distinct inputs differ only in the third component keep both",
			connectionID:    "vc-1",
			entityType:      "host",
			managedObjectID: "host-102",
			want:            "vc-1:host:host-102",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SourceID(tc.connectionID, tc.entityType, tc.managedObjectID)
			if got != tc.want {
				t.Fatalf("SourceID(%q, %q, %q) = %q, want %q",
					tc.connectionID, tc.entityType, tc.managedObjectID, got, tc.want)
			}
		})
	}

	t.Run("different inputs produce different IDs", func(t *testing.T) {
		a := SourceID("vc-1", "host", "host-101")
		b := SourceID("vc-1", "host", "host-102")
		if a == b {
			t.Fatalf("expected distinct IDs for distinct managedObjectID, got %q == %q", a, b)
		}
	})

	t.Run("identical inputs are stable across calls", func(t *testing.T) {
		first := SourceID("vc-1", "vm", "vm-201")
		second := SourceID("vc-1", "vm", "vm-201")
		if first != second {
			t.Fatalf("SourceID not stable: first=%q second=%q", first, second)
		}
		// Sanity-check the stable value matches the documented colon-join
		// format so this subtest asserts real behaviour, not just equality.
		if want := strings.Join([]string{"vc-1", "vm", "vm-201"}, ":"); first != want {
			t.Fatalf("stable SourceID = %q, want canonical %q", first, want)
		}
	})
}
