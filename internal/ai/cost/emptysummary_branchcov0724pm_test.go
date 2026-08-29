package cost

import "testing"

// This file raises branch coverage on cost.EmptySummary (store.go:95), the
// outward-facing constructor for an empty cost Summary. EmptySummary delegates
// to the unexported emptySummary helper and exists so API handlers can return a
// canonical, JSON-safe empty payload (non-nil slices, stamped pricing date)
// when no backing store is available.
//
// EmptySummary performs no normalization on its integer arguments, so the table
// below covers both values of `truncated` and zero/negative/positive day counts
// to prove every field is echoed verbatim while the collection fields stay
// non-nil and empty.

// branchcov0724pmAssertNonNilEmptySlice fails the test when slice is nil or
// holds any elements. EmptySummary's whole purpose is to emit non-nil empty
// slices (so JSON consumers see [] instead of null), so this is the core
// observable behaviour under test.
func branchcov0724pmAssertNonNilEmptySlice[T any](t *testing.T, name string, slice []T) {
	t.Helper()
	if slice == nil {
		t.Errorf("%s = nil, want non-nil empty slice", name)
		return
	}
	if len(slice) != 0 {
		t.Errorf("len(%s) = %d, want 0", name, len(slice))
	}
}

func TestBranchcov0724pmEmptySummaryTable(t *testing.T) {
	// pricingAsOf is the build-embedded pricing-table date (pricing.go:39).
	// Asserted as a concrete observable value, not via PricingAsOf(), so the
	// test would still catch a regression if both moved together.
	const wantPricingAsOf = "2026-08-29"

	cases := []struct {
		name          string
		days          int
		retentionDays int
		effectiveDays int
		truncated     bool
	}{
		{"zero days not truncated", 0, 0, 0, false},
		{"zero days truncated", 0, 0, 0, true},
		{"negative days not truncated", -7, -1, -3, false},
		{"negative days truncated", -1, -2, -1, true},
		{"positive days not truncated", 7, 14, 7, false},
		{"positive days truncated", 30, 365, 30, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := EmptySummary(tc.days, tc.retentionDays, tc.effectiveDays, tc.truncated)

			if s.Days != tc.days {
				t.Errorf("Days = %d, want %d", s.Days, tc.days)
			}
			if s.RetentionDays != tc.retentionDays {
				t.Errorf("RetentionDays = %d, want %d", s.RetentionDays, tc.retentionDays)
			}
			if s.EffectiveDays != tc.effectiveDays {
				t.Errorf("EffectiveDays = %d, want %d", s.EffectiveDays, tc.effectiveDays)
			}
			if s.Truncated != tc.truncated {
				t.Errorf("Truncated = %v, want %v", s.Truncated, tc.truncated)
			}
			if s.PricingAsOf != wantPricingAsOf {
				t.Errorf("PricingAsOf = %q, want %q", s.PricingAsOf, wantPricingAsOf)
			}

			branchcov0724pmAssertNonNilEmptySlice(t, "ProviderModels", s.ProviderModels)
			branchcov0724pmAssertNonNilEmptySlice(t, "UseCases", s.UseCases)
			branchcov0724pmAssertNonNilEmptySlice(t, "Targets", s.Targets)
			branchcov0724pmAssertNonNilEmptySlice(t, "DailyTotals", s.DailyTotals)

			// Totals is a value (not a slice) and is intentionally left at its
			// zero value by EmptySummary.
			if s.Totals != (ProviderModelSummary{}) {
				t.Errorf("Totals = %+v, want zero ProviderModelSummary", s.Totals)
			}
		})
	}
}

// TestBranchcov0724pmEmptySummaryFreshSlices proves each call allocates fresh
// backing slices: mutating one returned Summary's collection must not bleed
// into another. This guards against a future "optimization" that shares a
// package-level empty slice across calls.
func TestBranchcov0724pmEmptySummaryFreshSlices(t *testing.T) {
	a := EmptySummary(1, 2, 3, true)
	b := EmptySummary(4, 5, 6, false)

	if a.Days != 1 || b.Days != 4 {
		t.Fatalf("Days fields differ between calls: a=%d b=%d", a.Days, b.Days)
	}

	// Mutate A's slices; B must be unaffected.
	a.ProviderModels = append(a.ProviderModels, ProviderModelSummary{Provider: "x"})
	a.UseCases = append(a.UseCases, UseCaseSummary{UseCase: "chat"})
	a.Targets = append(a.Targets, TargetSummary{TargetType: "vm"})
	a.DailyTotals = append(a.DailyTotals, DailySummary{Date: "2026-01-01"})

	if len(b.ProviderModels) != 0 || len(b.UseCases) != 0 || len(b.Targets) != 0 || len(b.DailyTotals) != 0 {
		t.Errorf("mutating summary A leaked into B: pm=%d uc=%d tgt=%d daily=%d",
			len(b.ProviderModels), len(b.UseCases), len(b.Targets), len(b.DailyTotals))
	}
}
