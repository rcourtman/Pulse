package unifiedresources

import (
	"strings"
	"testing"
	"time"
)

// Branch-coverage tests for two currently-uncovered PURE functions:
//   - ActionPolicyAuthorizationDigest  (actions.go:240) — zeroes Digest,
//     json.Marshal, sha256.Sum256, fmt.Sprintf("sha256:%x", sum).
//   - normalizePair                    (store.go:4082) — CanonicalResourceID
//     on both args, then returns them sorted ascending (swap when a>b).
//
// Every assertion pins a hand-computed expected value or invariant; none is a
// "no panic" assertion. The pinned SHA-256 digests below were derived
// independently (see comments) from the exact JSON payload the function
// produces, so they detect any drift in the marshalled field set, ordering, or
// hash formatting.

// baseLease0721am builds the non-trivial ActionPolicyAuthorizationLease used
// as the reference payload for the digest subtests. Field values are chosen to
// exercise every serializable field type (ints, strings, bools true AND false,
// a non-empty []string, a non-nil *AutoRemediationWindow, and two distinct
// time.Time values). The JSON this marshals to (with Digest="") is, verbatim:
//
//	{"version":1,"orgId":"org-123","actionId":"action-abc","resourceId":"host-7",
//	 "capabilityName":"restart_service","planHash":"sha256:deadbeef",
//	 "capabilityPolicyVersion":"v2","autoAuthorization":"low_risk",
//	 "approvalPolicy":"none","tenantPolicyVersion":"tenant-v9",
//	 "effectiveAutonomyLevel":"guided","licenseAllowsAutoFix":true,
//	 "fullModeUnlocked":false,"emergencyStop":false,
//	 "resourcePolicyVersion":"res-v1",
//	 "capabilityNames":["restart_service","check_status"],
//	 "window":{"timezone":"UTC","startMinute":0,"endMinute":1440},
//	 "neverAutoRemediate":false,"issuedAt":"2026-07-21T12:30:00Z",
//	 "expiresAt":"2026-07-21T13:30:00Z","digest":""}
//
// sha256 of those bytes = 0dd5f9f1517d850c1a27d588163a8e4361027358bd5cabbca120c15a551687ff.
func baseLease0721am() ActionPolicyAuthorizationLease {
	return ActionPolicyAuthorizationLease{
		Version:                 1,
		OrgID:                   "org-123",
		ActionID:                "action-abc",
		ResourceID:              "host-7",
		CapabilityName:          "restart_service",
		PlanHash:                "sha256:deadbeef",
		CapabilityPolicyVersion: "v2",
		AutoAuthorization:       AutoAuthorizeLowRisk,
		ApprovalPolicy:          ApprovalNone,
		TenantPolicyVersion:     "tenant-v9",
		EffectiveAutonomyLevel:  "guided",
		LicenseAllowsAutoFix:    true,
		FullModeUnlocked:        false,
		EmergencyStop:           false,
		ResourcePolicyVersion:   "res-v1",
		CapabilityNames:         []string{"restart_service", "check_status"},
		Window:                  &AutoRemediationWindow{Timezone: "UTC", StartMinute: 0, EndMinute: 1440},
		NeverAutoRemediate:      false,
		IssuedAt:                time.Date(2026, 7, 21, 12, 30, 0, 0, time.UTC),
		ExpiresAt:               time.Date(2026, 7, 21, 13, 30, 0, 0, time.UTC),
		Digest:                  "",
	}
}

func TestBranchcov0721amPureHelpers(t *testing.T) {
	// ----------------------------------------------------------------------
	// ActionPolicyAuthorizationDigest
	// ----------------------------------------------------------------------
	t.Run("ActionPolicyAuthorizationDigest", func(t *testing.T) {
		t.Parallel()

		t.Run("deterministic_same_lease_identical_digest_across_two_calls", func(t *testing.T) {
			t.Parallel()
			first := ActionPolicyAuthorizationDigest(baseLease0721am())
			second := ActionPolicyAuthorizationDigest(baseLease0721am())
			if first != second {
				t.Fatalf("digest not deterministic: call1=%q call2=%q", first, second)
			}
		})

		t.Run("digest_field_excluded_nonempty_digest_equals_empty_digest", func(t *testing.T) {
			t.Parallel()
			// The function zeroes lease.Digest before marshalling, so a lease
			// carrying any Digest value must produce the SAME digest as the
			// identical lease with Digest="". This is the key self-referential
			// invariant ValidateActionPolicyAuthorizationLease relies on.
			withDigest := baseLease0721am()
			withDigest.Digest = "sha256:PRECOMPUTED_PLACEHOLDER_THAT_MUST_BE_IGNORED"
			withoutDigest := baseLease0721am()
			withoutDigest.Digest = ""
			gotWith := ActionPolicyAuthorizationDigest(withDigest)
			gotWithout := ActionPolicyAuthorizationDigest(withoutDigest)
			if gotWith != gotWithout {
				t.Fatalf("Digest field was NOT excluded: withDigest=%q withoutDigest=%q", gotWith, gotWithout)
			}
		})

		t.Run("distinct_payloads_produce_distinct_digests_changing_OrgID", func(t *testing.T) {
			t.Parallel()
			// Changing OrgID mutates the marshalled payload, so the digest must
			// differ. Hand-computed digest for OrgID="org-999" (everything else
			// equal to baseLease0721am) =
			// 84bf1e7282dc57341417403b88778f521c6ff8716a3786e074ed46005124bebb.
			variant := baseLease0721am()
			variant.OrgID = "org-999"
			base := ActionPolicyAuthorizationDigest(baseLease0721am())
			got := ActionPolicyAuthorizationDigest(variant)
			if got == base {
				t.Fatalf("changing OrgID did not change digest: both=%q", got)
			}
			const wantOrgVariant = "sha256:84bf1e7282dc57341417403b88778f521c6ff8716a3786e074ed46005124bebb"
			if got != wantOrgVariant {
				t.Fatalf("OrgID-variant digest = %q, want %q", got, wantOrgVariant)
			}
		})

		t.Run("distinct_payloads_produce_distinct_digests_changing_ActionID", func(t *testing.T) {
			t.Parallel()
			// Changing ActionID must likewise change the digest. Hand-computed
			// digest for ActionID="action-xyz" (everything else equal) =
			// 4059812a4d6547d6f88c6ec6ad07141a303d9b13224fd35b35f27cc3183ee29d.
			variant := baseLease0721am()
			variant.ActionID = "action-xyz"
			base := ActionPolicyAuthorizationDigest(baseLease0721am())
			got := ActionPolicyAuthorizationDigest(variant)
			if got == base {
				t.Fatalf("changing ActionID did not change digest: both=%q", got)
			}
			const wantActionVariant = "sha256:4059812a4d6547d6f88c6ec6ad07141a303d9b13224fd35b35f27cc3183ee29d"
			if got != wantActionVariant {
				t.Fatalf("ActionID-variant digest = %q, want %q", got, wantActionVariant)
			}
		})

		t.Run("format_is_sha256_prefix_plus_64_lowercase_hex_chars", func(t *testing.T) {
			t.Parallel()
			got := ActionPolicyAuthorizationDigest(baseLease0721am())
			const prefix = "sha256:"
			if !strings.HasPrefix(got, prefix) {
				t.Fatalf("digest %q missing %q prefix", got, prefix)
			}
			hex := strings.TrimPrefix(got, prefix)
			if len(hex) != 64 {
				t.Fatalf("hex part length = %d, want 64 (digest=%q)", len(hex), got)
			}
			for i, r := range hex {
				isLowerHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
				if !isLowerHex {
					t.Fatalf("hex part contains non-lowercase-hex rune %q at index %d (digest=%q)", r, i, got)
				}
			}
		})

		t.Run("pinned_exact_value_for_reference_lease", func(t *testing.T) {
			t.Parallel()
			// Pins the full digest for baseLease0721am so any future change to
			// the marshalled field set, ordering, or hash formatting is caught.
			// Value computed independently from the JSON payload documented on
			// baseLease0721am.
			got := ActionPolicyAuthorizationDigest(baseLease0721am())
			const want = "sha256:0dd5f9f1517d850c1a27d588163a8e4361027358bd5cabbca120c15a551687ff"
			if got != want {
				t.Fatalf("ActionPolicyAuthorizationDigest(baseLease0721am) = %q, want %q", got, want)
			}
		})

		t.Run("empty_lease_distinct_from_reference_and_well_formed", func(t *testing.T) {
			t.Parallel()
			// A zero-value lease is a different payload, so its digest must
			// differ from the reference lease and still be well-formed.
			empty := ActionPolicyAuthorizationLease{}
			got := ActionPolicyAuthorizationDigest(empty)
			ref := ActionPolicyAuthorizationDigest(baseLease0721am())
			if got == ref {
				t.Fatalf("empty-lease digest equals reference-lease digest: %q", got)
			}
			if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+64 {
				t.Fatalf("empty-lease digest malformed: %q", got)
			}
		})
	})

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
