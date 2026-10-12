package unifiedresources

import "testing"

// Branch-coverage tests for the PURE function normalizePair (store.go):
// CanonicalResourceID on both args, then returns them sorted ascending (swap
// when a>b). Every assertion pins a hand-computed expected value.

func TestBranchcov0721amPureHelpers(t *testing.T) {
	// ----------------------------------------------------------------------
	// normalizePair
	// ----------------------------------------------------------------------
	t.Run("normalizePair", func(t *testing.T) {
		t.Parallel()

		// normalizePair canonicalizes both args via CanonicalResourceID
		// (which is strings.TrimSpace, returning "" for empty/blank input)
		// and then returns them sorted ascending: swap when a>b, no swap
		// when a<=b. Each row below drives a specific branch and pins the
		// exact hand-computed (first, second) pair.
		cases := []struct {
			name      string
			a, b      string
			wantFirst string
			wantSec   string
		}{
			{
				// No-swap branch (a<=b): "alpha" < "zeta" → returned as-is.
				name:      "no_swap_branch_a_less_than_b",
				a:         "alpha",
				b:         "zeta",
				wantFirst: "alpha",
				wantSec:   "zeta",
			},
			{
				// Swap branch (a>b): "zeta" > "alpha" → swapped on return.
				name:      "swap_branch_a_greater_than_b",
				a:         "zeta",
				b:         "alpha",
				wantFirst: "alpha",
				wantSec:   "zeta",
			},
			{
				// Equal case (a==b, a>b is false): no swap, both identical.
				name:      "equal_inputs_no_swap",
				a:         "same",
				b:         "same",
				wantFirst: "same",
				wantSec:   "same",
			},
			{
				// Canonicalization applied: CanonicalResourceID trims
				// surrounding whitespace, so the raw inputs "  alpha  " and
				// "\tzeta\t" produce the canonical ("alpha","zeta"). This
				// proves the output reflects the canonical form, not the raw
				// input, and the trimmed forms are then sorted.
				name:      "canonicalization_trims_whitespace_then_sorts",
				a:         "  alpha  ",
				b:         "\tzeta\t",
				wantFirst: "alpha",
				wantSec:   "zeta",
			},
			{
				// Canonicalization + swap: trimmed "zeta" > trimmed "alpha".
				name:      "canonicalization_trims_whitespace_then_swaps",
				a:         "  zeta  ",
				b:         "\talpha\t",
				wantFirst: "alpha",
				wantSec:   "zeta",
			},
			{
				// Both inputs blank-after-trim: CanonicalResourceID returns
				// "" for each, "" is not > "" so no swap → ("","").
				name:      "both_inputs_blank_after_trim_yield_empty_pair",
				a:         "   ",
				b:         "\t",
				wantFirst: "",
				wantSec:   "",
			},
			{
				// One side blank after trim, the other not: "" < "host-1",
				// no swap, empty sorts first.
				name:      "one_blank_one_present_empty_sorts_first",
				a:         "host-1",
				b:         "  ",
				wantFirst: "",
				wantSec:   "host-1",
			},
			{
				// Lexicographic ordering with a common prefix: "host-10" >
				// "host-2" under byte comparison (since '1' < '2'), so swap.
				name:      "lexicographic_byte_order_host10_after_host2",
				a:         "host-10",
				b:         "host-2",
				wantFirst: "host-10",
				wantSec:   "host-2",
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				gotFirst, gotSec := normalizePair(tc.a, tc.b)
				if gotFirst != tc.wantFirst || gotSec != tc.wantSec {
					t.Fatalf("normalizePair(%q, %q) = (%q, %q), want (%q, %q)",
						tc.a, tc.b, gotFirst, gotSec, tc.wantFirst, tc.wantSec)
				}
				// Output is always sorted ascending (first <= second).
				if gotFirst > gotSec {
					t.Fatalf("normalizePair output not sorted: first=%q > second=%q", gotFirst, gotSec)
				}
			})
		}

		// Ordering invariant: normalizePair(x,y) == normalizePair(y,x) for
		// any x,y. Driven across a set of input pairs that exercise both the
		// swap and no-swap branches, including whitespace inputs.
		t.Run("ordering_invariant_normalizePair_xy_equals_normalizePair_yx", func(t *testing.T) {
			t.Parallel()
			pairs := [][2]string{
				{"alpha", "zeta"},
				{"zeta", "alpha"},
				{"same", "same"},
				{"  alpha  ", "\tzeta\t"},
				{"host-10", "host-2"},
				{"", "host-1"},
				{"   ", "\t"},
			}
			for _, p := range pairs {
				p := p
				t.Run(p[0]+"|"+p[1], func(t *testing.T) {
					t.Parallel()
					fa, sa := normalizePair(p[0], p[1])
					fb, sb := normalizePair(p[1], p[0])
					if fa != fb || sa != sb {
						t.Fatalf("ordering invariant violated: normalizePair(%q,%q)=(%q,%q) != normalizePair(%q,%q)=(%q,%q)",
							p[0], p[1], fa, sa, p[1], p[0], fb, sb)
					}
				})
			}
		})

		// Canonicalization is applied to BOTH arguments independently: even
		// when the raw inputs are already in sorted order, the returned
		// values must be the canonical (trimmed) forms, not the raw inputs.
		t.Run("canonicalization_reflects_canonical_form_not_raw_input", func(t *testing.T) {
			t.Parallel()
			gotFirst, gotSec := normalizePair("  alpha  ", "  bravo  ")
			if gotFirst != "alpha" || gotSec != "bravo" {
				t.Fatalf("expected trimmed canonical forms (alpha, bravo), got (%q, %q)", gotFirst, gotSec)
			}
			// And the raw untrimmed inputs would NOT have satisfied the
			// sorted-output contract on their own here is asserted by
			// confirming the leading byte differs from the raw input.
			if gotFirst == "  alpha  " {
				t.Fatalf("normalizePair returned the RAW untrimmed input %q instead of the canonical form", gotFirst)
			}
		})
	})
}
