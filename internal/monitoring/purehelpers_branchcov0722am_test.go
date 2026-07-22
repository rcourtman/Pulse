package monitoring

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run "^TestBranchcov0722"`) for three pure helpers in package monitoring
// that previously had 0.0% coverage:
//
//   - hostThermalStateFromReadStateView(in *unifiedresources.HostThermalState) *models.HostThermalState  — monitor.go
//   - latestMetricPoint(points []MetricPoint) (MetricPoint, bool)                                        — mock_chart_history.go
//   - MemorySourceTrust(source string) string                                                            — memory_source_catalog.go
//
// Every arm of each function is exercised directly here:
//
//   - hostThermalStateFromReadStateView: the nil-input -> nil arm, the fully
//     populated mapping arm (every scalar, pointer and map field), nil nested
//     pointers -> nil, and a nil/empty LimitsPercent map -> nil (because
//     cloneStringIntMap returns nil when len(src)==0). Clone independence is
//     asserted for the three cloned *int fields and the cloned map in both
//     directions.
//   - latestMetricPoint: the empty/nil slice -> (zero, false) arm, the single
//     point arm (loop body never runs), and the multi-point arm. The function
//     selects by newest Timestamp (NOT last element), so out-of-order input
//     and exact-timestamp ties (first wins) are asserted against real
//     behaviour.
//   - MemorySourceTrust: every key in memorySourceCatalog (via direct
//     iteration so no new catalog entry can slip through untested), an
//     unknown source, the empty string, a whitespace-only string, and the
//     DescribeMemorySource lower-case+trim normalisation path.
//
// Conventions match sibling in-package tests in this directory (see
// availability_poller_purehelpers_branchcov0722am_test.go,
// connected_infrastructure_groupkey_branchcov0722am_test.go and
// canonical_guardrails_test.go): stdlib `testing` only, table-driven subtests,
// t.Fatalf assertions, no testify. Each case is value-in -> value-out.

// thermalIntAddr is a tiny helper so fixtures can take the address of a literal
// int without allocating a named variable each time.
func thermalIntAddr(v int) *int { return &v }

func TestBranchcov0722HostThermalStateFromReadStateView(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		if got := hostThermalStateFromReadStateView(nil); got != nil {
			t.Fatalf("hostThermalStateFromReadStateView(nil) = %+v, want nil", got)
		}
	})

	t.Run("fully populated input maps every field", func(t *testing.T) {
		in := &unifiedresources.HostThermalState{
			Source:                  "ipmi",
			Pressure:                "critical",
			ThermalWarningLevel:     thermalIntAddr(7),
			PerformanceWarningLevel: thermalIntAddr(4),
			CPUPowerStatus:          thermalIntAddr(2),
			LimitsPercent: map[string]int{
				"cpu": 85,
				"psu": 90,
			},
		}

		got := hostThermalStateFromReadStateView(in)
		if got == nil {
			t.Fatal("expected a projected HostThermalState, got nil")
		}

		// Structural equality against an explicitly constructed expected
		// value validates that every field is mapped. reflect.DeepEqual
		// dereferences pointers and compares maps by content, so distinct
		// allocations with equal values compare equal here.
		want := &models.HostThermalState{
			Source:                  "ipmi",
			Pressure:                "critical",
			ThermalWarningLevel:     thermalIntAddr(7),
			PerformanceWarningLevel: thermalIntAddr(4),
			CPUPowerStatus:          thermalIntAddr(2),
			LimitsPercent: map[string]int{
				"cpu": 85,
				"psu": 90,
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("projected state = %+v, want %+v", got, want)
		}

		// Explicit pointer-value checks keep the mapping intent legible
		// independent of the DeepEqual implementation detail above.
		if got.ThermalWarningLevel == nil || *got.ThermalWarningLevel != 7 {
			t.Fatalf("ThermalWarningLevel = %v, want pointer to 7", got.ThermalWarningLevel)
		}
		if got.PerformanceWarningLevel == nil || *got.PerformanceWarningLevel != 4 {
			t.Fatalf("PerformanceWarningLevel = %v, want pointer to 4", got.PerformanceWarningLevel)
		}
		if got.CPUPowerStatus == nil || *got.CPUPowerStatus != 2 {
			t.Fatalf("CPUPowerStatus = %v, want pointer to 2", got.CPUPowerStatus)
		}
	})

	t.Run("nil nested pointers map to nil", func(t *testing.T) {
		// Every pointer field nil; LimitsPercent nil. cloneIntPtr(nil)
		// returns nil for each, so the projection must carry nil through.
		in := &unifiedresources.HostThermalState{
			Source:                  "agent",
			Pressure:                "ok",
			ThermalWarningLevel:     nil,
			PerformanceWarningLevel: nil,
			CPUPowerStatus:          nil,
			LimitsPercent:           nil,
		}
		got := hostThermalStateFromReadStateView(in)
		if got == nil {
			t.Fatal("expected a projected HostThermalState, got nil")
		}
		if got.Source != "agent" || got.Pressure != "ok" {
			t.Fatalf("scalar fields not mapped: %+v", got)
		}
		if got.ThermalWarningLevel != nil || got.PerformanceWarningLevel != nil || got.CPUPowerStatus != nil {
			t.Fatalf("nil pointer fields not preserved as nil: %+v", got)
		}
		if got.LimitsPercent != nil {
			t.Fatalf("nil LimitsPercent must project to nil map, got %v", got.LimitsPercent)
		}
	})

	t.Run("empty LimitsPercent map projects to nil", func(t *testing.T) {
		// cloneStringIntMap returns nil when len(src)==0 (it does not
		// preserve a non-nil empty map). This asserts that real behaviour.
		in := &unifiedresources.HostThermalState{
			Source:        "agent",
			LimitsPercent: map[string]int{},
		}
		got := hostThermalStateFromReadStateView(in)
		if got == nil {
			t.Fatal("expected a projected HostThermalState, got nil")
		}
		if got.LimitsPercent != nil {
			t.Fatalf("empty LimitsPercent must project to nil (cloneStringIntMap len==0 arm), got %v", got.LimitsPercent)
		}
	})
}

// TestBranchcov0722HostThermalStateCloneIndependence asserts that mutating the
// returned pointer fields and map never affects the input, and vice versa.
// This is exactly the guarantee cloneIntPtr / cloneStringIntMap provide; the
// test pins it at the public-mapper boundary so a future refactor that drops
// the clone is caught.
func TestBranchcov0722HostThermalStateCloneIndependence(t *testing.T) {
	t.Run("mutating returned pointers does not affect input", func(t *testing.T) {
		in := &unifiedresources.HostThermalState{
			ThermalWarningLevel:     thermalIntAddr(10),
			PerformanceWarningLevel: thermalIntAddr(20),
			CPUPowerStatus:          thermalIntAddr(30),
		}
		out := hostThermalStateFromReadStateView(in)

		*out.ThermalWarningLevel = 9999
		*out.PerformanceWarningLevel = 9999
		*out.CPUPowerStatus = 9999

		if *in.ThermalWarningLevel != 10 || *in.PerformanceWarningLevel != 20 || *in.CPUPowerStatus != 30 {
			t.Fatalf("mutating returned pointer fields corrupted the input: tw=%d pw=%d cpu=%d",
				*in.ThermalWarningLevel, *in.PerformanceWarningLevel, *in.CPUPowerStatus)
		}
	})

	t.Run("mutating input pointers does not affect returned", func(t *testing.T) {
		in := &unifiedresources.HostThermalState{
			ThermalWarningLevel:     thermalIntAddr(10),
			PerformanceWarningLevel: thermalIntAddr(20),
			CPUPowerStatus:          thermalIntAddr(30),
		}
		out := hostThermalStateFromReadStateView(in)

		*in.ThermalWarningLevel = 9999
		*in.PerformanceWarningLevel = 9999
		*in.CPUPowerStatus = 9999

		if *out.ThermalWarningLevel != 10 || *out.PerformanceWarningLevel != 20 || *out.CPUPowerStatus != 30 {
			t.Fatalf("mutating input pointer fields corrupted the returned projection: tw=%d pw=%d cpu=%d",
				*out.ThermalWarningLevel, *out.PerformanceWarningLevel, *out.CPUPowerStatus)
		}
	})

	t.Run("mutating returned map does not affect input", func(t *testing.T) {
		in := &unifiedresources.HostThermalState{
			LimitsPercent: map[string]int{"cpu": 80, "psu": 90},
		}
		out := hostThermalStateFromReadStateView(in)

		delete(out.LimitsPercent, "cpu")
		out.LimitsPercent["psu"] = 1
		out.LimitsPercent["new"] = 2

		if len(in.LimitsPercent) != 2 || in.LimitsPercent["cpu"] != 80 || in.LimitsPercent["psu"] != 90 {
			t.Fatalf("mutating returned LimitsPercent corrupted the input: %v", in.LimitsPercent)
		}
	})

	t.Run("mutating input map does not affect returned", func(t *testing.T) {
		in := &unifiedresources.HostThermalState{
			LimitsPercent: map[string]int{"cpu": 80, "psu": 90},
		}
		out := hostThermalStateFromReadStateView(in)

		delete(in.LimitsPercent, "cpu")
		in.LimitsPercent["psu"] = 1
		in.LimitsPercent["new"] = 2

		if len(out.LimitsPercent) != 2 || out.LimitsPercent["cpu"] != 80 || out.LimitsPercent["psu"] != 90 {
			t.Fatalf("mutating input LimitsPercent corrupted the returned projection: %v", out.LimitsPercent)
		}
	})
}

func TestBranchcov0722LatestMetricPoint(t *testing.T) {
	base := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)

	t.Run("nil slice returns zero value and false", func(t *testing.T) {
		got, ok := latestMetricPoint(nil)
		if ok {
			t.Fatal("expected ok=false for nil slice")
		}
		if !reflect.DeepEqual(got, MetricPoint{}) {
			t.Fatalf("nil slice point = %+v, want zero MetricPoint", got)
		}
	})

	t.Run("empty slice returns zero value and false", func(t *testing.T) {
		got, ok := latestMetricPoint([]MetricPoint{})
		if ok {
			t.Fatal("expected ok=false for empty slice")
		}
		if !reflect.DeepEqual(got, MetricPoint{}) {
			t.Fatalf("empty slice point = %+v, want zero MetricPoint", got)
		}
	})

	t.Run("single point returns it and true", func(t *testing.T) {
		// Single point: the for-loop body (i:=1; i<len) never executes, so
		// the seed value is returned verbatim.
		want := MetricPoint{Value: 42.5, Timestamp: base}
		got, ok := latestMetricPoint([]MetricPoint{want})
		if !ok {
			t.Fatal("expected ok=true for single point")
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("single point = %+v, want %+v", got, want)
		}
	})

	t.Run("multiple in-order points return the newest", func(t *testing.T) {
		// Timestamps strictly increasing; the last element is also the newest.
		points := []MetricPoint{
			{Value: 1, Timestamp: base},
			{Value: 2, Timestamp: base.Add(time.Minute)},
			{Value: 3, Timestamp: base.Add(2 * time.Minute)},
		}
		got, ok := latestMetricPoint(points)
		if !ok {
			t.Fatal("expected ok=true for multiple points")
		}
		if got.Value != 3 || !got.Timestamp.Equal(base.Add(2*time.Minute)) {
			t.Fatalf("latest point = %+v, want value 3 at %+v", got, base.Add(2*time.Minute))
		}
	})

	t.Run("out-of-order points still select newest by timestamp not position", func(t *testing.T) {
		// Newest timestamp sits in the middle; a naive "last element" reader
		// would fail this. The function compares Timestamp.After, so the
		// middle point must win.
		newest := MetricPoint{Value: 99, Timestamp: base.Add(time.Hour)}
		points := []MetricPoint{
			{Value: 1, Timestamp: base},
			newest,
			{Value: 2, Timestamp: base.Add(time.Minute)},
		}
		got, ok := latestMetricPoint(points)
		if !ok {
			t.Fatal("expected ok=true for out-of-order points")
		}
		if !reflect.DeepEqual(got, newest) {
			t.Fatalf("out-of-order latest = %+v, want %+v", got, newest)
		}
	})

	t.Run("equal timestamps keep the first encountered", func(t *testing.T) {
		// points[i].Timestamp.After(latest.Timestamp) is strict: an equal
		// timestamp does not replace the seed, so the earliest equal point
		// wins. Asserts real behaviour.
		ts := base.Add(time.Minute)
		first := MetricPoint{Value: 1, Timestamp: ts}
		points := []MetricPoint{
			first,
			{Value: 2, Timestamp: ts},
			{Value: 3, Timestamp: ts},
		}
		got, ok := latestMetricPoint(points)
		if !ok {
			t.Fatal("expected ok=true for equal-timestamp points")
		}
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("equal-timestamp latest = %+v, want first %+v", got, first)
		}
	})
}

func TestBranchcov0722MemorySourceTrust(t *testing.T) {
	t.Run("every catalog entry returns its declared trust", func(t *testing.T) {
		// Iterate the package-level catalog directly so no recognised
		// source can ever slip through untested. A minimum count guards
		// against an accidentally emptied catalog making the loop vacuous.
		if len(memorySourceCatalog) < 10 {
			t.Fatalf("memorySourceCatalog unexpectedly small: %d entries", len(memorySourceCatalog))
		}
		for source, desc := range memorySourceCatalog {
			source, desc := source, desc
			t.Run(source, func(t *testing.T) {
				if got := MemorySourceTrust(source); got != desc.Trust {
					t.Fatalf("MemorySourceTrust(%q) = %q, want %q", source, got, desc.Trust)
				}
			})
		}
	})

	cases := []struct {
		name   string
		source string
		want   string
	}{
		// Arm: recognised "preferred" trust sources.
		{"available-field is preferred", "available-field", "preferred"},
		{"meminfo-available alias is preferred", "meminfo-available", "preferred"},
		// Arm: recognised "derived" trust sources.
		{"derived-free-buffers-cached is derived", "derived-free-buffers-cached", "derived"},
		{"calculated alias is derived", "calculated", "derived"},
		// Arm: recognised "fallback" trust sources.
		{"unknown catalog key is fallback", "unknown", "fallback"},
		{"agent is fallback", "agent", "fallback"},
		{"powered-off is fallback", "powered-off", "fallback"},

		// Arm: unknown source (not in catalog) -> DescribeMemorySource
		// returns Trust:"fallback" with Fallback:false.
		{"unrecognized source falls back", "not-a-real-source", "fallback"},

		// Arm: empty string is a real catalog key ("") -> "fallback".
		{"empty string is fallback", "", "fallback"},

		// Arm: whitespace-only string trims to "" which is a catalog key
		// -> "fallback".
		{"whitespace-only string is fallback", "   \t\n", "fallback"},

		// Arm: DescribeMemorySource lower-cases + trims before lookup, so
		// a mixed-case, whitespace-padded recognised source must still
		// resolve to its catalog trust. MemorySourceTrust inherits that
		// normalisation.
		{"mixed case with surrounding whitespace normalises to preferred", "  AvailAbLe-FiElD  ", "preferred"},
		{"uppercase derived alias normalises to derived", "CALCULATED", "derived"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := MemorySourceTrust(tc.source); got != tc.want {
				t.Fatalf("MemorySourceTrust(%q) = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}
