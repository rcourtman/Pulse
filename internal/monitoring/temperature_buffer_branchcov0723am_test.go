package monitoring

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBranchcov0723Am_LimitedTemperatureBufferBytes exercises every branch of
// limitedTemperatureBuffer.Write (the three return paths) and pins the
// observable contract of Bytes: it returns the prefix of the input that fit
// within maxBytes, keeping the FIRST bytes (head), and the returned slice
// aliases the internal buffer.
func TestBranchcov0723Am_LimitedTemperatureBufferBytes(t *testing.T) {
	t.Run("empty buffer returns nil", func(t *testing.T) {
		b := &limitedTemperatureBuffer{maxBytes: 16}
		got := b.Bytes()
		// bytes.Buffer.Bytes() on an untouched buffer returns nil (b.buf[nil][0:]).
		assert.Nil(t, got, "empty buffer should expose no bytes")
		assert.Len(t, got, 0)
	})

	t.Run("write under limit exposes exact bytes", func(t *testing.T) {
		b := &limitedTemperatureBuffer{maxBytes: 16}
		n, err := b.Write([]byte("hello"))
		require.NoError(t, err, "in-budget Write must not error")
		assert.Equal(t, 5, n, "in-budget Write reports full length")
		assert.False(t, b.exceeded, "exceeded flag must stay false when under limit")
		assert.Equal(t, "hello", string(b.Bytes()), "Bytes must return exactly what was written")
	})

	t.Run("single write over limit keeps the head prefix and flags exceeded", func(t *testing.T) {
		const max = 5
		b := &limitedTemperatureBuffer{maxBytes: max}
		// "abcdefgh" is 8 bytes; only the first `max` fit.
		n, err := b.Write([]byte("abcdefgh"))
		require.ErrorIs(t, err, errTemperatureCommandOutputTooLarge, "over-budget Write must report the size-limit error")
		assert.Equal(t, max, n, "over-budget Write reports only the bytes that fit")
		assert.True(t, b.exceeded, "exceeded flag must flip true when truncating")
		// Truncation semantics: the HEAD (first max bytes) is retained.
		assert.Equal(t, "abcde", string(b.Bytes()), "Bytes must keep the first maxBytes bytes of the input")
	})

	t.Run("further write after limit is rejected without growing buffer", func(t *testing.T) {
		b := &limitedTemperatureBuffer{maxBytes: 5}
		_, _ = b.Write([]byte("abcdefgh")) // saturates the limit and flips exceeded
		before := string(b.Bytes())
		n, err := b.Write([]byte("XYZ"))
		require.ErrorIs(t, err, errTemperatureCommandOutputTooLarge, "Write after the limit is reached must short-circuit with the size-limit error")
		assert.Equal(t, 0, n, "no bytes may be appended once remaining <= 0")
		assert.Equal(t, before, string(b.Bytes()), "buffer contents must not change after the limit is hit")
	})

	t.Run("several writes accumulating across the limit boundary", func(t *testing.T) {
		b := &limitedTemperatureBuffer{maxBytes: 5}
		// First two writes fit entirely ("abc" + "de" == 5 bytes).
		n1, err1 := b.Write([]byte("abc"))
		require.NoError(t, err1)
		assert.Equal(t, 3, n1)
		n2, err2 := b.Write([]byte("de"))
		require.NoError(t, err2)
		assert.Equal(t, 2, n2)
		assert.False(t, b.exceeded, "exactly at limit is not yet exceeded")
		assert.Equal(t, "abcde", string(b.Bytes()))

		// Third write tips over the limit; nothing more fits.
		n3, err3 := b.Write([]byte("fghi"))
		require.ErrorIs(t, err3, errTemperatureCommandOutputTooLarge)
		assert.Equal(t, 0, n3, "no bytes may be appended once the buffer is full")
		assert.True(t, b.exceeded)
		assert.Equal(t, "abcde", string(b.Bytes()), "head prefix is preserved when accumulating across the limit")
	})

	t.Run("write larger than remaining keeps head and reports only what fit", func(t *testing.T) {
		b := &limitedTemperatureBuffer{maxBytes: 5}
		_, _ = b.Write([]byte("abc")) // 3 bytes used, 2 remaining
		// "defgh" is 5 bytes but only 2 fit; head retention means we keep "abcde".
		n, err := b.Write([]byte("defgh"))
		require.ErrorIs(t, err, errTemperatureCommandOutputTooLarge)
		assert.Equal(t, 2, n, "partial write reports only the bytes that fit (maxBytes - current len)")
		assert.True(t, b.exceeded)
		assert.Equal(t, "abcde", string(b.Bytes()), "must keep the first maxBytes bytes across two writes")
	})

	t.Run("returned slice aliases the internal buffer", func(t *testing.T) {
		// Documents a real aliasing property of bytes.Buffer.Bytes(): the
		// returned slice shares storage with the buffer, so mutations leak.
		// Reported as a suspected source-quality issue in GLM_REPORT_go-montemp.md.
		b := &limitedTemperatureBuffer{maxBytes: 16}
		_, err := b.Write([]byte("hello"))
		require.NoError(t, err)

		view := b.Bytes()
		require.Len(t, view, 5)
		view[0] = 'X' // mutate the returned slice in place

		assert.Equal(t, "Xello", string(b.Bytes()), "Bytes() aliases the internal buffer; mutating the returned slice leaks (suspected bug)")
	})
}

// TestBranchcov0723Am_GeneratePlateauSeries covers generatePlateauSeries
// directly. The function is deterministic given a seeded *rand.Rand, so all
// assertions are against concrete outputs.
func TestBranchcov0723Am_GeneratePlateauSeries(t *testing.T) {
	makeRng := func(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

	t.Run("points zero returns empty slice", func(t *testing.T) {
		rng := makeRng(7)
		got := generatePlateauSeries(50, 0, 0, 100, 100, rng)
		// Loop body never executes when points == 0; raw is allocated empty.
		assert.Len(t, got, 0, "points=0 must produce an empty series without panic")
	})

	t.Run("points negative panics at makeslice", func(t *testing.T) {
		// Documents observed behaviour: generatePlateauSeries does not guard
		// negative points, so `make([]float64, points)` panics. Reported as a
		// suspected source bug in the report (callers like GenerateSeededSeries
		// happen to gate on points <= 1, hiding this from production).
		defer func() {
			r := recover()
			require.NotNil(t, r, "expected makeslice panic for negative points")
			t.Logf("observed panic for negative points (suspected source bug): %v", r)
		}()
		rng := makeRng(7)
		_ = generatePlateauSeries(50, -3, 0, 100, 100, rng)
	})

	t.Run("points one panics at integer divide by zero", func(t *testing.T) {
		// Documents observed behaviour: with points < plateauCount (>=3),
		// segmentLen = points/plateauCount == 0 and `i / segmentLen` panics.
		// Reported as a suspected source bug; production callers gate on
		// points <= 1, so the panic is currently unreachable in normal use.
		defer func() {
			r := recover()
			require.NotNil(t, r, "expected divide-by-zero panic for points=1")
			t.Logf("observed panic for points=1 (suspected source bug): %v", r)
		}()
		rng := makeRng(7)
		_ = generatePlateauSeries(50, 1, 0, 100, 100, rng)
	})

	t.Run("normal run has correct length and stays within bounds", func(t *testing.T) {
		const (
			current = 50.0
			points  = 240
			min     = 0.0
			max     = 100.0
			span    = 100.0
		)
		rng := makeRng(123)
		got := generatePlateauSeries(current, points, min, max, span, rng)
		require.Len(t, got, points, "series length must equal requested points")

		for i, v := range got {
			if !(v >= min && v <= max) {
				t.Fatalf("point %d out of bounds: %v (want within [%v,%v])", i, v, min, max)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("point %d is non-finite: %v", i, v)
			}
		}
	})

	t.Run("identical seed produces identical series", func(t *testing.T) {
		a := generatePlateauSeries(62.5, 200, 10, 90, 80, makeRng(42))
		b := generatePlateauSeries(62.5, 200, 10, 90, 80, makeRng(42))
		require.Len(t, a, 200)
		require.Len(t, b, 200)
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("determinism violated at index %d: %v != %v", i, a[i], b[i])
			}
		}
	})

	t.Run("different seeds produce different series", func(t *testing.T) {
		a := generatePlateauSeries(62.5, 200, 10, 90, 80, makeRng(1))
		b := generatePlateauSeries(62.5, 200, 10, 90, 80, makeRng(999))
		require.Len(t, a, 200)
		require.Len(t, b, 200)
		diff := 0
		for i := range a {
			if a[i] != b[i] {
				diff++
			}
		}
		assert.Greater(t, diff, 0, "different seeds must yield different series; got identical output")
	})

	t.Run("min equals max collapses every value to that constant", func(t *testing.T) {
		const c = 42.0
		// span == 0 -> all offsets/noise terms vanish and clamp pins every level.
		got := generatePlateauSeries(c, 120, c, c, 0, makeRng(5))
		require.Len(t, got, 120)
		for i, v := range got {
			if v != c {
				t.Fatalf("point %d = %v, want %v when min==max==current", i, v, c)
			}
		}
	})

	t.Run("current outside bounds leaks past max into output", func(t *testing.T) {
		// generatePlateauSeries does NOT clamp `current` on entry (unlike its
		// only caller GenerateSeededSeries). The final plateau level is set to
		// `current + noise` without clamping, so out-of-range current leaks
		// into the series. This pins that observed (un-clamped) behaviour.
		const (
			current = 150.0
			min     = 0.0
			max     = 100.0
			span    = 100.0
		)
		got := generatePlateauSeries(current, 200, min, max, span, makeRng(31))
		require.Len(t, got, 200)

		// Last point belongs to the final plateau and tracks `current` closely.
		assert.Greater(t, got[len(got)-1], max,
			"out-of-range current must leak into the un-clamped tail; got last=%v", got[len(got)-1])

		// And at least one point must exceed max, proving the function does not
		// clamp its own output.
		anyOver := false
		for _, v := range got {
			if v > max {
				anyOver = true
				break
			}
		}
		assert.True(t, anyOver, "expected at least one value > max when current > max (function does not clamp output)")
	})
}

// TestBranchcov0723Am_MockNodeMetricsForChart covers
// mockNodeMetricsForChart. The chart-timestamp path uses time.Now(), so only
// structural invariants (key set, length, ordering, head/tail membership) are
// asserted, never exact values.
func TestBranchcov0723Am_MockNodeMetricsForChart(t *testing.T) {
	const nodeID = "branchcov-node-723am"

	expectAscending := func(t *testing.T, points []MetricPoint) {
		t.Helper()
		for i := 1; i < len(points); i++ {
			if points[i].Timestamp.Before(points[i-1].Timestamp) {
				t.Fatalf("points not in ascending timestamp order at index %d: %v before %v",
					i, points[i].Timestamp, points[i-1].Timestamp)
			}
		}
	}

	expectKeysExact := func(t *testing.T, got map[string][]MetricPoint, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("expected %d map entries (%v), got %d (%v)", len(want), want, len(got), keysOf(got))
		}
		for _, k := range want {
			if _, ok := got[k]; !ok {
				t.Fatalf("expected key %q missing from result; got keys %v", k, keysOf(got))
			}
		}
	}

	t.Run("nil metricTypes falls back to default node chart set", func(t *testing.T) {
		got := mockNodeMetricsForChart(nodeID, nil, time.Hour)
		expectKeysExact(t, got, mockNodeChartMetricTypes)
		for k, pts := range got {
			expectAscending(t, pts)
			assert.NotEmpty(t, pts, "default metric %q should produce points for a known node", k)
		}
	})

	t.Run("empty metricTypes falls back to default node chart set", func(t *testing.T) {
		got := mockNodeMetricsForChart(nodeID, []string{}, time.Hour)
		expectKeysExact(t, got, mockNodeChartMetricTypes)
	})

	t.Run("single requested metric type yields exactly one entry", func(t *testing.T) {
		got := mockNodeMetricsForChart(nodeID, []string{"cpu"}, time.Hour)
		expectKeysExact(t, got, []string{"cpu"})
		require.NotEmpty(t, got["cpu"], "cpu series should not be empty for a known node/metric")
		expectAscending(t, got["cpu"])
	})

	t.Run("several requested metric types yield one entry each and no extras", func(t *testing.T) {
		want := []string{"cpu", "memory", "disk"}
		got := mockNodeMetricsForChart(nodeID, want, time.Hour)
		expectKeysExact(t, got, want)
		for _, k := range want {
			require.NotEmpty(t, got[k], "%s series should not be empty", k)
			expectAscending(t, got[k])
		}
	})

	t.Run("unknown metric type still produces a map entry with sampled points", func(t *testing.T) {
		// Source has no allowlist for metric types: it forwards whatever the
		// caller asked for to the canonical sampler, which assigns default
		// bounds [0,100] for unknown metric names. Pin that contract.
		got := mockNodeMetricsForChart(nodeID, []string{"bogulus_metric_723am"}, time.Hour)
		expectKeysExact(t, got, []string{"bogulus_metric_723am"})
		require.NotEmpty(t, got["bogulus_metric_723am"], "unknown metric type should still yield sampled points (default bounds)")
		expectAscending(t, got["bogulus_metric_723am"])
	})

	t.Run("zero duration defaults to one hour window and preserves ordering", func(t *testing.T) {
		got := mockNodeMetricsForChart(nodeID, []string{"cpu"}, 0)
		expectKeysExact(t, got, []string{"cpu"})
		require.NotEmpty(t, got["cpu"], "zero duration must not produce an empty series (clamped to 1h)")
		expectAscending(t, got["cpu"])
	})

	t.Run("negative duration defaults to one hour window and preserves ordering", func(t *testing.T) {
		got := mockNodeMetricsForChart(nodeID, []string{"cpu"}, -2*time.Hour)
		expectKeysExact(t, got, []string{"cpu"})
		require.NotEmpty(t, got["cpu"], "negative duration must not produce an empty series (clamped to 1h)")
		expectAscending(t, got["cpu"])
	})

	t.Run("all returned series are time-ordered", func(t *testing.T) {
		// Comprehensive ordering check across the default set + a custom type,
		// exercising every codepath that materialises MetricPoint slices.
		types := append([]string(nil), mockNodeChartMetricTypes...)
		types = append(types, "temperature")
		got := mockNodeMetricsForChart(nodeID, types, 2*time.Hour)
		expectKeysExact(t, got, types)
		for k, pts := range got {
			expectAscending(t, pts)
			assert.NotEmpty(t, pts, "metric %q produced no points", k)
		}
	})

	t.Run("empty nodeID returns empty map even with valid metric types", func(t *testing.T) {
		// Trimming empty nodeID short-circuits before any sampling.
		got := mockNodeMetricsForChart("   ", []string{"cpu"}, time.Hour)
		assert.Len(t, got, 0, "blank nodeID must return an empty map (no entries)")
	})
}

// keysOf returns the key slice of a map for stable failure messages.
func keysOf(m map[string][]MetricPoint) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
