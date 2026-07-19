package mockmodel

import (
	"math"
	"testing"
	"time"
)

// fixedAnchor is the deterministic reference timestamp used across the
// branch-coverage tests in this file. The mock model is fully seeded, so fixing
// the timestamp and seeds makes every assertion below reproducible.
var fixedAnchor = time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)

// hourlyTimestamps returns n evenly-spaced hourly timestamps ending at end
// inclusive. For n<=0 it returns nil so callers can exercise the empty-input
// branch.
func hourlyTimestamps(end time.Time, n int) []time.Time {
	if n <= 0 {
		return nil
	}
	out := make([]time.Time, n)
	for i := 0; i < n; i++ {
		out[n-1-i] = end.Add(-time.Duration(i) * time.Hour)
	}
	return out
}

// styleName renders a SeriesStyle for subtest labels.
func styleName(s SeriesStyle) string {
	switch s {
	case StyleSpiky:
		return "spiky"
	case StylePlateau:
		return "plateau"
	case StyleFlat:
		return "flat"
	default:
		return "unknown"
	}
}

func TestNormalizeBlendWeight_BranchesAndClamps(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name      string
		weight    float64
		step      time.Duration
		reference time.Duration
		wantExact bool
		want      float64 // exact expected value when wantExact is true
		lo, hi    float64 // inclusive bounds for the in-range scaling cases
	}{
		{name: "negative weight clamps to floor", weight: -0.5, step: time.Minute, reference: 2 * time.Minute, wantExact: true, want: 0.01},
		{name: "zero weight clamps to floor", weight: 0, step: time.Minute, reference: 2 * time.Minute, wantExact: true, want: 0.01},
		{name: "at one returns one", weight: 1, step: time.Minute, reference: 2 * time.Minute, wantExact: true, want: 1},
		{name: "above one clamps to one", weight: 2.5, step: time.Minute, reference: 2 * time.Minute, wantExact: true, want: 1},
		{name: "zero step returns weight unchanged", weight: 0.5, step: 0, reference: 2 * time.Minute, wantExact: true, want: 0.5},
		{name: "zero reference returns weight unchanged", weight: 0.5, step: time.Minute, reference: 0, wantExact: true, want: 0.5},
		{name: "negative step returns weight unchanged", weight: 0.5, step: -time.Minute, reference: 2 * time.Minute, wantExact: true, want: 0.5},
		{name: "negative reference returns weight unchanged", weight: 0.5, step: time.Minute, reference: -2 * time.Minute, wantExact: true, want: 0.5},
		{name: "equal step and reference returns weight unchanged", weight: 0.5, step: 2 * time.Minute, reference: 2 * time.Minute, wantExact: true, want: 0.5},
		{name: "in-range short step scales below weight", weight: 0.5, step: time.Minute, reference: 4 * time.Minute, wantExact: false, lo: 0.0005, hi: 0.5},
		{name: "in-range long step scales above weight", weight: 0.5, step: 4 * time.Minute, reference: time.Minute, wantExact: false, lo: 0.5, hi: 0.999},
		{name: "extreme ratio triggers upper clamp", weight: 0.99, step: 24 * time.Hour, reference: time.Millisecond, wantExact: true, want: 0.999},
		{name: "tiny ratio triggers lower clamp", weight: 0.01, step: time.Millisecond, reference: time.Hour, wantExact: true, want: 0.0005},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeBlendWeight(tc.weight, tc.step, tc.reference)
			if tc.wantExact {
				if math.Abs(got-tc.want) > eps {
					t.Fatalf("NormalizeBlendWeight(%v,%v,%v) = %v, want %v", tc.weight, tc.step, tc.reference, got, tc.want)
				}
				return
			}
			if got < tc.lo || got > tc.hi {
				t.Fatalf("NormalizeBlendWeight(%v,%v,%v) = %v, want within [%v,%v]", tc.weight, tc.step, tc.reference, got, tc.lo, tc.hi)
			}
		})
	}
}

func TestNormalizeBlendWeight_Deterministic(t *testing.T) {
	args := []struct {
		w    float64
		step time.Duration
		ref  time.Duration
	}{
		{0.5, time.Minute, 4 * time.Minute},
		{0.3, 2 * time.Minute, 3 * time.Minute},
		{0.7, 30 * time.Second, 90 * time.Second},
	}
	for _, a := range args {
		first := NormalizeBlendWeight(a.w, a.step, a.ref)
		second := NormalizeBlendWeight(a.w, a.step, a.ref)
		third := NormalizeBlendWeight(a.w, a.step, a.ref)
		if first != second || second != third {
			t.Fatalf("expected deterministic output for (%v,%v,%v), got %v %v %v", a.w, a.step, a.ref, first, second, third)
		}
	}
}

func TestSeriesForTimestamps_EmptyTimestampsReturnsEmpty(t *testing.T) {
	for _, style := range []SeriesStyle{StyleSpiky, StylePlateau, StyleFlat} {
		style := style
		t.Run(styleName(style), func(t *testing.T) {
			got := SeriesForTimestamps(50, nil, 7, 0, 100, style)
			if len(got) != 0 {
				t.Fatalf("expected len=0 for nil timestamps, got %d", len(got))
			}
			got2 := SeriesForTimestamps(50, []time.Time{}, 7, 0, 100, style)
			if len(got2) != 0 {
				t.Fatalf("expected len=0 for zero-length timestamps, got %d", len(got2))
			}
		})
	}
}

func TestSeriesForTimestamps_LengthBoundsAndDeterminism(t *testing.T) {
	timestamps := hourlyTimestamps(fixedAnchor, 6)
	const min, max = 5.0, 95.0
	for _, style := range []SeriesStyle{StyleSpiky, StylePlateau, StyleFlat} {
		style := style
		t.Run(styleName(style), func(t *testing.T) {
			series := SeriesForTimestamps(50, timestamps, 11, min, max, style)
			if len(series) != len(timestamps) {
				t.Fatalf("expected len=%d, got %d", len(timestamps), len(series))
			}
			for i, v := range series {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("idx %d: non-finite value %v", i, v)
				}
				if v < min || v > max {
					t.Fatalf("idx %d: value %v outside [%v,%v]", i, v, min, max)
				}
			}
			// In-range current anchors the tail to that exact value.
			if series[len(series)-1] != 50.0 {
				t.Fatalf("expected tail to equal in-range current 50, got %v", series[len(series)-1])
			}
			// Determinism: identical args must produce identical output.
			again := SeriesForTimestamps(50, timestamps, 11, min, max, style)
			for i := range series {
				if series[i] != again[i] {
					t.Fatalf("non-deterministic at idx %d: %v vs %v", i, series[i], again[i])
				}
			}
		})
	}
}

func TestSeriesForTimestamps_ClampsOutOfRangeCurrentToTail(t *testing.T) {
	timestamps := hourlyTimestamps(fixedAnchor, 4)
	const min, max = 10.0, 90.0

	hi := SeriesForTimestamps(500, timestamps, 3, min, max, StyleSpiky)
	if hi[len(hi)-1] != max {
		t.Fatalf("expected above-range current clamped to max=%v at tail, got %v", max, hi[len(hi)-1])
	}
	for _, v := range hi {
		if v < min || v > max {
			t.Fatalf("above-range series value %v outside [%v,%v]", v, min, max)
		}
	}

	lo := SeriesForTimestamps(-50, timestamps, 3, min, max, StyleSpiky)
	if lo[len(lo)-1] != min {
		t.Fatalf("expected below-range current clamped to min=%v at tail, got %v", min, lo[len(lo)-1])
	}
	for _, v := range lo {
		if v < min || v > max {
			t.Fatalf("below-range series value %v outside [%v,%v]", v, min, max)
		}
	}
}

// TestSeriesForProfile_AllProfilesBoundedAndDeterministic exercises the internal
// seriesForProfile helper directly across every metricProfile so each of its
// style branches receives coverage attribution. The values it produces must be
// finite, within [min,max], and identical across repeated calls.
func TestSeriesForProfile_AllProfilesBoundedAndDeterministic(t *testing.T) {
	timestamps := hourlyTimestamps(fixedAnchor, 8)
	const min, max = 0.0, 100.0
	profiles := []struct {
		name string
		p    metricProfile
	}{
		{"compute", profileCompute},
		{"memory", profileMemory},
		{"diskio", profileDiskIO},
		{"network", profileNetwork},
		{"capacity", profileCapacity},
		{"thermal", profileThermal},
		{"flat", profileFlat},
	}
	for _, pc := range profiles {
		pc := pc
		t.Run(pc.name, func(t *testing.T) {
			first := seriesForProfile(50, timestamps, 17, min, max, pc.p)
			second := seriesForProfile(50, timestamps, 17, min, max, pc.p)
			if len(first) != len(timestamps) {
				t.Fatalf("%s: expected len=%d, got %d", pc.name, len(timestamps), len(first))
			}
			for i, v := range first {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("%s idx %d: non-finite %v", pc.name, i, v)
				}
				if v < min || v > max {
					t.Fatalf("%s idx %d: %v outside [%v,%v]", pc.name, i, v, min, max)
				}
				if v != second[i] {
					t.Fatalf("%s idx %d: non-deterministic %v vs %v", pc.name, i, v, second[i])
				}
			}
			if first[len(first)-1] != 50.0 {
				t.Fatalf("%s: expected tail to equal in-range current 50, got %v", pc.name, first[len(first)-1])
			}
		})
	}
}

func TestSeriesForProfile_EmptyTimestampsReturnsNil(t *testing.T) {
	for _, p := range []metricProfile{profileCompute, profileMemory, profileDiskIO, profileFlat} {
		got := seriesForProfile(50, nil, 1, 0, 100, p)
		if got != nil {
			t.Fatalf("profile %d: expected nil for empty timestamps, got %v (len=%d)", p, got, len(got))
		}
	}
}

// TestDiskIOValue_BoundedAndDeterministic asserts the documented behaviour of
// diskIOValue: identical inputs produce identical output, the result is finite,
// and across every realistic role modifier set the value stays inside
// [min, min+span]. The bounds are verified empirically across all role profiles.
func TestDiskIOValue_BoundedAndDeterministic(t *testing.T) {
	const min, span = 10.0, 100.0
	const hi = min + span
	for _, role := range []string{"", "database", "backup", "web", "storage", "ci"} {
		mods := metricRoleProfile(role)
		for _, seed := range []uint64{0, 1, 7, 42, 99} {
			a := diskIOValue(seed, min, span, mods, fixedAnchor)
			b := diskIOValue(seed, min, span, mods, fixedAnchor)
			if a != b {
				t.Fatalf("role=%q seed=%d: non-deterministic %v vs %v", role, seed, a, b)
			}
			if math.IsNaN(a) || math.IsInf(a, 0) {
				t.Fatalf("role=%q seed=%d: non-finite %v", role, seed, a)
			}
			if a < min || a > hi {
				t.Fatalf("role=%q seed=%d: %v outside [%v,%v]", role, seed, a, min, hi)
			}
		}
	}
}

// TestFlatValue_BoundedAndDeterministic asserts the same shape of guarantees
// for flatValue as TestDiskIOValue does for diskIOValue.
func TestFlatValue_BoundedAndDeterministic(t *testing.T) {
	const min, span = 10.0, 100.0
	const hi = min + span
	for _, role := range []string{"", "database", "storage", "cache", "media"} {
		mods := metricRoleProfile(role)
		for _, seed := range []uint64{0, 1, 5, 23, 77} {
			a := flatValue(seed, min, span, mods, fixedAnchor)
			b := flatValue(seed, min, span, mods, fixedAnchor)
			if a != b {
				t.Fatalf("role=%q seed=%d: non-deterministic %v vs %v", role, seed, a, b)
			}
			if math.IsNaN(a) || math.IsInf(a, 0) {
				t.Fatalf("role=%q seed=%d: non-finite %v", role, seed, a)
			}
			if a < min || a > hi {
				t.Fatalf("role=%q seed=%d: %v outside [%v,%v]", role, seed, a, min, hi)
			}
		}
	}
}
