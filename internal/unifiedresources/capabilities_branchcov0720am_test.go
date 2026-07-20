package unifiedresources

import "testing"

// Branch-coverage tests for the ActionAutoAuthorizationClass validators in
// capabilities.go.
//
// Two target functions, each with genuine conditional logic:
//
//   - IsValidActionAutoAuthorizationClass: a switch with a true-arm (empty +
//     the three canonical classes) and a default false-arm.
//   - NormalizeActionAutoAuthorizationClass: an `||` short-circuit with three
//     distinct arms — empty input, non-empty invalid input, and valid
//     pass-through.
//
// Assertions are behavioural: for the validator we assert the boolean
// classification across each arm; for the normalizer we assert the
// fallback-vs-passthrough contract (fallback coerces the value away from the
// input and to the canonical "never" sentinel; pass-through preserves the
// input value exactly). We never pin a literal string — only the published
// constants and the input/output relationship.

// TestBranchcov0720am_IsValidActionAutoAuthorizationClass exercises every
// case-arm of the validator's switch: the empty string, each canonical class
// constant, and several representative invalid values (a totally bogus value,
// a permissive-sounding-but-unknown value, a wrong-case near-miss, and an
// untrimmed variant of a valid value).
func TestBranchcov0720am_IsValidActionAutoAuthorizationClass(t *testing.T) {
	tests := []struct {
		name string
		in   ActionAutoAuthorizationClass
		want bool
	}{
		{name: "empty is valid", in: "", want: true},
		{name: "never is valid", in: AutoAuthorizeNever, want: true},
		{name: "low_risk is valid", in: AutoAuthorizeLowRisk, want: true},
		{name: "elevated is valid", in: AutoAuthorizeElevated, want: true},
		{name: "bogus high_risk is invalid", in: "high_risk", want: false},
		{name: "permissive-sounding always is invalid", in: "always", want: false},
		{name: "wrong-case NEVER is invalid (case-sensitive)", in: "NEVER", want: false},
		{name: "untrimmed low_risk is invalid (no implicit trim)", in: " low_risk ", want: false},
		{name: "arbitrary non-empty garbage is invalid", in: "auto", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidActionAutoAuthorizationClass(tt.in)
			if got != tt.want {
				t.Fatalf("IsValidActionAutoAuthorizationClass(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestBranchcov0720am_NormalizeActionAutoAuthorizationClass exercises all
// three control-flow arms of the normalizer:
//
//   - empty input  -> fallback (AutoAuthorizeNever), and the result is
//     non-empty (proving coercion occurred rather than pass-through of "").
//   - non-empty invalid input -> fallback (AutoAuthorizeNever), and the
//     result differs from the input (proving coercion rather than pass-through).
//   - each canonical non-empty valid input -> the exact same value is
//     returned (pass-through / identity), which is the discriminative proof
//     that the function does not blindly collapse everything to "never".
func TestBranchcov0720am_NormalizeActionAutoAuthorizationClass(t *testing.T) {
	tests := []struct {
		name       string
		in         ActionAutoAuthorizationClass
		want       ActionAutoAuthorizationClass
		coerced    bool // true iff the normalizer must change the value (fallback arms)
		discrimAPI bool // true iff want is a distinct canonical used to discriminate pass-through
	}{
		{name: "empty input falls back to never", in: "", want: AutoAuthorizeNever, coerced: true},
		{name: "bogus high_risk falls back to never", in: "high_risk", want: AutoAuthorizeNever, coerced: true},
		{name: "permissive-sounding always falls back to never", in: "always", want: AutoAuthorizeNever, coerced: true},
		{name: "wrong-case NEVER falls back to never", in: "NEVER", want: AutoAuthorizeNever, coerced: true},
		{name: "never passes through", in: AutoAuthorizeNever, want: AutoAuthorizeNever, discrimAPI: true},
		{name: "low_risk passes through unchanged", in: AutoAuthorizeLowRisk, want: AutoAuthorizeLowRisk, discrimAPI: true},
		{name: "elevated passes through unchanged", in: AutoAuthorizeElevated, want: AutoAuthorizeElevated, discrimAPI: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeActionAutoAuthorizationClass(tt.in)
			if got != tt.want {
				t.Fatalf("NormalizeActionAutoAuthorizationClass(%q) = %q, want %q", tt.in, got, tt.want)
			}
			// Invariant: the normalizer never returns an empty string.
			// This proves the empty-input arm coerces rather than passing "" through.
			if got == "" {
				t.Fatalf("NormalizeActionAutoAuthorizationClass(%q) returned empty; normalizer must always emit a non-empty canonical", tt.in)
			}
			if tt.coerced {
				// On the fallback arms the result must differ from the
				// (non-canonical or empty) input — this is what distinguishes
				// a real fallback from a no-op pass-through.
				if got == tt.in {
					t.Fatalf("NormalizeActionAutoAuthorizationClass(%q): expected coercion but result equals input %q", tt.in, got)
				}
			}
			if tt.discrimAPI {
				// On the pass-through arms the result must equal the input
				// exactly — this proves the function is not collapsing every
				// valid input onto "never".
				if got != tt.in {
					t.Fatalf("NormalizeActionAutoAuthorizationClass(%q): expected identity pass-through but got %q", tt.in, got)
				}
			}
		})
	}
}

// TestBranchcov0720am_NormalizeActionAutoAuthorizationClass_FallbackConsistency
// asserts the behavioural contract that every invalid/empty input collapses to
// the SAME canonical sentinel as an explicit AutoAuthorizeNever input. This is
// the user-facing guarantee ("unknown always means never") and is asserted by
// comparing outputs of the function under test against each other rather than
// by re-stating a literal.
func TestBranchcov0720am_NormalizeActionAutoAuthorizationClass_FallbackConsistency(t *testing.T) {
	neverResult := NormalizeActionAutoAuthorizationClass(AutoAuthorizeNever)
	if neverResult == "" {
		t.Fatalf("baseline: Normalize(never) is empty, want the never canonical")
	}

	invalidInputs := []ActionAutoAuthorizationClass{
		"",
		"high_risk",
		"always",
		"NEVER",
		" low_risk ",
		"auto",
		"unknown",
	}
	for _, in := range invalidInputs {
		got := NormalizeActionAutoAuthorizationClass(in)
		if got != neverResult {
			t.Errorf("Normalize(%q) = %q, want same canonical as Normalize(never)=%q (unknown must always mean never)", in, got, neverResult)
		}
		// And the result must differ from the (non-canonical) input where the
		// input is non-empty, proving the value was actually coerced.
		if in != "" && got == in {
			t.Errorf("Normalize(%q) returned the input unchanged; invalid input must be coerced to never", in)
		}
	}
}

// TestBranchcov0720am_NormalizeActionAutoAuthorizationClass_PassThroughPreservesDistinct
// is the positive discriminative test for the pass-through arm: feeding the two
// non-"never" canonical classes must preserve them distinctly. If the
// normalizer were buggy and collapsed everything to "never", low_risk and
// elevated would compare equal — this test would fail.
func TestBranchcov0720am_NormalizeActionAutoAuthorizationClass_PassThroughPreservesDistinct(t *testing.T) {
	low := NormalizeActionAutoAuthorizationClass(AutoAuthorizeLowRisk)
	elev := NormalizeActionAutoAuthorizationClass(AutoAuthorizeElevated)
	never := NormalizeActionAutoAuthorizationClass(AutoAuthorizeNever)

	if low == elev {
		t.Fatalf("low_risk and elevated normalised to the same value %q; pass-through must preserve distinct canonicals", low)
	}
	if low == never {
		t.Fatalf("low_risk normalised to never %q; pass-through must preserve low_risk as distinct from never", low)
	}
	if elev == never {
		t.Fatalf("elevated normalised to never %q; pass-through must preserve elevated as distinct from never", elev)
	}
	if low != AutoAuthorizeLowRisk {
		t.Fatalf("low_risk pass-through: got %q, want low_risk", low)
	}
	if elev != AutoAuthorizeElevated {
		t.Fatalf("elevated pass-through: got %q, want elevated", elev)
	}
}
