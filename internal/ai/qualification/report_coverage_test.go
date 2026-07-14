package qualification

import (
	"reflect"
	"testing"
	"time"
)

func TestPercentileIndexBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		length int
		p      float64
		want   int
	}{
		{"zero length ninetyfive clamps to zero", 0, 0.95, 0},
		{"zero length zero p clamps to zero", 0, 0.0, 0},
		{"zero length huge p clamps to zero", 0, 5.0, 0},
		{"length one p zero clamps to zero", 1, 0.0, 0},
		{"length one p midpoint", 1, 0.5, 0},
		{"length one p ninetyfive", 1, 0.95, 0},
		{"length one p one", 1, 1.0, 0},
		{"length one p over one clamps to last", 1, 1.5, 0},
		{"length two p zero clamps to zero", 2, 0.0, 0},
		{"length two p midpoint picks lower", 2, 0.5, 0},
		{"length two p ninetyfive picks upper", 2, 0.95, 1},
		{"length ten p zero clamps to zero", 10, 0.0, 0},
		{"length ten p five percent", 10, 0.05, 0},
		{"length ten p fifteen percent", 10, 0.15, 1},
		{"length ten p midpoint", 10, 0.5, 4},
		{"length ten p ninetyfive last index", 10, 0.95, 9},
		{"length ten p one last index", 10, 1.0, 9},
		{"length ten p over one clamps to last", 10, 2.0, 9},
		{"length twenty p ninetyfive", 20, 0.95, 18},
		{"length four p zero clamps to zero", 4, 0.0, 0},
		{"length four p ninetyfive", 4, 0.95, 3},
		{"length five p midpoint", 5, 0.5, 2},
		{"length three p ninetyfive last index", 3, 0.95, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentileIndex(tc.length, tc.p); got != tc.want {
				t.Fatalf("percentileIndex(%d, %v) = %d, want %d", tc.length, tc.p, got, tc.want)
			}
		})
	}
}

func TestPercentileInt64Aggregation(t *testing.T) {
	cases := []struct {
		name   string
		values []int64
		p      float64
		want   int64
	}{
		{"nil slice returns zero", nil, 0.95, 0},
		{"empty slice returns zero", []int64{}, 0.95, 0},
		{"single element", []int64{42}, 0.95, 42},
		{"single element p zero", []int64{42}, 0.0, 42},
		{"single element p one", []int64{42}, 1.0, 42},
		{"unsorted five p ninetyfive picks max", []int64{50, 10, 40, 20, 30}, 0.95, 50},
		{"unsorted five p midpoint", []int64{50, 10, 40, 20, 30}, 0.5, 30},
		{"two elements p ninetyfive picks upper", []int64{100, 1}, 0.95, 100},
		{"duplicates collapse", []int64{5, 5, 5, 5}, 0.95, 5},
		{"negatives sort ascending", []int64{-10, 5, -3, 0}, 0.95, 5},
		{"p zero returns minimum", []int64{9, 3, 7, 1}, 0.0, 1},
		{"two elements p midpoint picks lower", []int64{8, 2}, 0.5, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentileInt64(tc.values, tc.p); got != tc.want {
				t.Fatalf("percentileInt64(%v, %v) = %d, want %d", tc.values, tc.p, got, tc.want)
			}
		})
	}
}

func TestPercentileIntAggregation(t *testing.T) {
	cases := []struct {
		name   string
		values []int
		p      float64
		want   int
	}{
		{"nil slice returns zero", nil, 0.95, 0},
		{"empty slice returns zero", []int{}, 0.95, 0},
		{"single element", []int{7}, 0.95, 7},
		{"single element p zero", []int{7}, 0.0, 7},
		{"unsorted p ninetyfive picks max", []int{4, 2, 5, 1, 3}, 0.95, 5},
		{"unsorted p midpoint", []int{4, 2, 5, 1, 3}, 0.5, 3},
		{"two elements p ninetyfive", []int{8, 2}, 0.95, 8},
		{"p zero returns minimum", []int{9, 1, 5}, 0.0, 1},
		{"duplicates", []int{2, 2, 2}, 0.95, 2},
		{"two elements p midpoint picks lower", []int{8, 2}, 0.5, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentileInt(tc.values, tc.p); got != tc.want {
				t.Fatalf("percentileInt(%v, %v) = %d, want %d", tc.values, tc.p, got, tc.want)
			}
		})
	}
}

func TestPercentileFloatAggregation(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		p      float64
		want   float64
	}{
		{"nil slice returns zero", nil, 0.95, 0},
		{"empty slice returns zero", []float64{}, 0.95, 0},
		{"single element", []float64{1.5}, 0.95, 1.5},
		{"single element p zero", []float64{1.5}, 0.0, 1.5},
		{"unsorted p ninetyfive picks max", []float64{0.1, 3.3, 2.2, 1.1}, 0.95, 3.3},
		{"unsorted p zero picks min", []float64{0.1, 3.3, 2.2, 1.1}, 0.0, 0.1},
		{"two elements p ninetyfive", []float64{2.5, 0.5}, 0.95, 2.5},
		{"duplicates", []float64{1.0, 1.0, 1.0}, 0.95, 1.0},
		{"negatives sort ascending", []float64{-1.5, 2.0, -0.5}, 0.95, 2.0},
		{"two elements p midpoint picks lower", []float64{2.5, 0.5}, 0.5, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentileFloat(tc.values, tc.p); got != tc.want {
				t.Fatalf("percentileFloat(%v, %v) = %v, want %v", tc.values, tc.p, got, tc.want)
			}
		})
	}
}

func TestSortedMapKeys(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]bool
		want []string
	}{
		{"nil map returns empty non-nil", nil, []string{}},
		{"empty map returns empty non-nil", map[string]bool{}, []string{}},
		{"single key", map[string]bool{"a": true}, []string{"a"}},
		{"multiple keys sorted ascending", map[string]bool{"c": true, "a": true, "b": true}, []string{"a", "b", "c"}},
		{"false values still included", map[string]bool{"a": false, "b": true}, []string{"a", "b"}},
		{"ascii ordering uppercase before lowercase", map[string]bool{"z": true, "A": true, "a1": true}, []string{"A", "a1", "z"}},
		{"whitespace keys preserved and sorted by byte", map[string]bool{" b": true, "a": true}, []string{" b", "a"}},
		{"deduplicates identical keys", map[string]bool{"x": true}, []string{"x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedMapKeys(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("sortedMapKeys(%v) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSummarizeModelAggregation(t *testing.T) {
	// makeReport builds a RunReport populated only with the fields summarizeModel
	// consumes; it is a table-local closure to avoid package-level collisions.
	makeReport := func(passed bool, gitSHA string, dirty bool, pulseVersion string,
		truePos, faults, falsePos, inTok, outTok int,
		collectionMs, e2eMs int64, costKnown bool, costUSD float64, hardFailures []string) RunReport {
		return RunReport{
			Passed:      passed,
			Environment: Environment{GitSHA: gitSHA, GitDirty: dirty, PulseVersion: pulseVersion},
			Score: Score{
				TruePositives:     truePos,
				Faults:            faults,
				FalsePositives:    falsePos,
				InputTokens:       inTok,
				OutputTokens:      outTok,
				CollectionLatency: time.Duration(collectionMs) * time.Millisecond,
				EndToEndLatency:   time.Duration(e2eMs) * time.Millisecond,
				Cost:              CostEstimate{Known: costKnown, USD: costUSD},
				HardFailures:      hardFailures,
			},
		}
	}

	t.Run("nil reports yields zeroed summary with degenerate intervals", func(t *testing.T) {
		got := summarizeModel("provider:none", nil)
		want := ModelSummary{
			Model:         "provider:none",
			PassRate:      WilsonInterval(0, 0),
			FaultRecall:   WilsonInterval(0, 0),
			GitSHAs:       []string{},
			PulseVersions: []string{},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summarizeModel(nil) = %+v, want %+v", got, want)
		}
		if got.PassRate.Estimate != 1 || got.PassRate.Lower != 0 || got.PassRate.Upper != 1 {
			t.Fatalf("degenerate PassRate interval = %+v", got.PassRate)
		}
		if got.FaultRecall.Estimate != 1 || got.FaultRecall.Lower != 0 || got.FaultRecall.Upper != 1 {
			t.Fatalf("degenerate FaultRecall interval = %+v", got.FaultRecall)
		}
	})

	t.Run("single passed run with known cost", func(t *testing.T) {
		report := makeReport(true, "abc123", false, "6.0.0", 2, 3, 1, 100, 50, 500, 1000, true, 0.05, nil)
		got := summarizeModel("provider:m", []RunReport{report})
		want := ModelSummary{
			Model:                  "provider:m",
			Runs:                   1,
			Passed:                 1,
			PassRate:               WilsonInterval(1, 1),
			FaultRecall:            WilsonInterval(2, 3),
			FalsePositives:         1,
			P95CollectionLatencyMs: 500,
			P95LatencyMs:           1000,
			P95InputTokens:         100,
			P95OutputTokens:        50,
			P95CostUSD:             0.05,
			KnownCostRuns:          1,
			GitSHAs:                []string{"abc123"},
			PulseVersions:          []string{"6.0.0"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summarizeModel(single) = %+v, want %+v", got, want)
		}
	})

	t.Run("multiple runs aggregate pass recall latency tokens and cost", func(t *testing.T) {
		reports := []RunReport{
			makeReport(true, "sha1 ", false, "v1", 1, 2, 0, 10, 5, 100, 200, true, 0.01, nil),
			makeReport(false, "", true, " v2 ", 0, 1, 2, 20, 10, 300, 400, false, 0, []string{"hard"}),
			makeReport(true, "sha1", false, "", 2, 2, 0, 30, 15, 200, 600, true, 0.03, nil),
		}
		got := summarizeModel("provider:multi", reports)
		want := ModelSummary{
			Model:                  "provider:multi",
			Runs:                   3,
			Passed:                 2,
			PassRate:               WilsonInterval(2, 3),
			FaultRecall:            WilsonInterval(3, 5),
			FalsePositives:         2,
			P95CollectionLatencyMs: 300,
			P95LatencyMs:           600,
			P95InputTokens:         30,
			P95OutputTokens:        15,
			P95CostUSD:             0.03,
			KnownCostRuns:          2,
			HardFailureRuns:        1,
			DirtyRuns:              1,
			GitSHAs:                []string{"sha1"},
			PulseVersions:          []string{"v1", "v2"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summarizeModel(multi) = %+v, want %+v", got, want)
		}
		if got.PassRate.Success != 2 || got.PassRate.Total != 3 {
			t.Fatalf("PassRate inputs = success %d total %d, want 2/3", got.PassRate.Success, got.PassRate.Total)
		}
		if got.FaultRecall.Success != 3 || got.FaultRecall.Total != 5 {
			t.Fatalf("FaultRecall inputs = success %d total %d, want 3/5", got.FaultRecall.Success, got.FaultRecall.Total)
		}
	})

	t.Run("all unknown cost yields zero p95 cost", func(t *testing.T) {
		report := makeReport(true, "abc", false, "v1", 1, 1, 0, 10, 5, 100, 200, false, 0, nil)
		got := summarizeModel("provider:uncosted", []RunReport{report})
		want := ModelSummary{
			Model:                  "provider:uncosted",
			Runs:                   1,
			Passed:                 1,
			PassRate:               WilsonInterval(1, 1),
			FaultRecall:            WilsonInterval(1, 1),
			P95CollectionLatencyMs: 100,
			P95LatencyMs:           200,
			P95InputTokens:         10,
			P95OutputTokens:        5,
			P95CostUSD:             0,
			GitSHAs:                []string{"abc"},
			PulseVersions:          []string{"v1"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summarizeModel(uncosted) = %+v, want %+v", got, want)
		}
		if got.KnownCostRuns != 0 {
			t.Fatalf("KnownCostRuns = %d, want 0", got.KnownCostRuns)
		}
	})

	t.Run("whitespace only sha and version are dropped", func(t *testing.T) {
		report := makeReport(true, "   ", false, "  ", 0, 0, 0, 1, 1, 1, 1, false, 0, nil)
		got := summarizeModel("provider:ws", []RunReport{report})
		if len(got.GitSHAs) != 0 {
			t.Fatalf("GitSHAs = %#v, want empty", got.GitSHAs)
		}
		if len(got.PulseVersions) != 0 {
			t.Fatalf("PulseVersions = %#v, want empty", got.PulseVersions)
		}
		if got.GitSHAs == nil || got.PulseVersions == nil {
			t.Fatalf("expected non-nil empty slices; GitSHAs=%#v PulseVersions=%#v", got.GitSHAs, got.PulseVersions)
		}
	})

	t.Run("distinct shas and versions deduplicated and sorted", func(t *testing.T) {
		reports := []RunReport{
			makeReport(true, "shaB", false, "v2", 0, 0, 0, 1, 1, 1, 1, false, 0, nil),
			makeReport(true, "shaA", false, "v1", 0, 0, 0, 1, 1, 1, 1, false, 0, nil),
			makeReport(true, "shaA", false, "v2", 0, 0, 0, 1, 1, 1, 1, false, 0, nil),
		}
		got := summarizeModel("provider:dedup", reports)
		wantSHA := []string{"shaA", "shaB"}
		wantVer := []string{"v1", "v2"}
		if !reflect.DeepEqual(got.GitSHAs, wantSHA) {
			t.Fatalf("GitSHAs = %#v, want %#v", got.GitSHAs, wantSHA)
		}
		if !reflect.DeepEqual(got.PulseVersions, wantVer) {
			t.Fatalf("PulseVersions = %#v, want %#v", got.PulseVersions, wantVer)
		}
	})

	t.Run("hard failure runs counted only when slice non-empty", func(t *testing.T) {
		reports := []RunReport{
			makeReport(true, "", false, "", 0, 0, 0, 1, 1, 1, 1, false, 0, nil),
			makeReport(true, "", false, "", 0, 0, 0, 1, 1, 1, 1, false, 0, []string{}),
			makeReport(true, "", false, "", 0, 0, 0, 1, 1, 1, 1, false, 0, []string{"boom"}),
		}
		got := summarizeModel("provider:hard", reports)
		if got.HardFailureRuns != 1 {
			t.Fatalf("HardFailureRuns = %d, want 1", got.HardFailureRuns)
		}
	})
}
