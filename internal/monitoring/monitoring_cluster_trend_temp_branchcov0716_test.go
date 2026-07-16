package monitoring

import (
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run BranchCov`) for pure/near-pure helpers in the monitoring package
// whose conditional arms were previously uncovered or under-covered:
//
//   - monitorExistingClusterIPOverride          (monitor_pve_cluster.go)
//   - monitorExistingClusterFingerprint         (monitor_pve_cluster.go)
//   - buildStorageSummaryCapacityTrend          (monitor_metrics.go)
//   - parseNouveauGPUTemps                      (temperature.go)
//
// Conventions match sibling in-package tests in this directory (see
// monitoring_infra_keys_branchcov0716_test.go): same package, stdlib
// `testing` only, table-driven, no testify.
//
// The receiver of parseNouveauGPUTemps (*TemperatureCollector) is never
// dereferenced inside the function, so a zero-value collector is safe.

// floatEq compares two floats with an absolute tolerance. The percentages
// produced by buildStorageSummaryCapacityTrend carry rounding error on the
// order of 1e-15, so 1e-6 is comfortably strict.
func floatEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6
}

func TestBranchCovMonitorExistingClusterIPOverride(t *testing.T) {
	endpoints := []config.ClusterEndpoint{
		{NodeName: "pve-alpha", IPOverride: "  10.0.0.5  "},
		{NodeName: "  pve-beta  ", IPOverride: "10.0.0.6"},
		{NodeName: "pve-empty", IPOverride: "   "},
		{NodeName: "pve-zero", IPOverride: ""},
		{NodeName: "pve-dup", IPOverride: "10.0.0.7"},
		{NodeName: "pve-dup", IPOverride: "10.0.0.8"},
	}

	cases := []struct {
		name     string
		nodeName string
		existing []config.ClusterEndpoint
		want     string
	}{
		// Branch: nil/empty existing slice -> "" (loop never executes).
		{"nil existing slice returns empty", "pve-alpha", nil, ""},
		{"empty existing slice returns empty", "pve-alpha", []config.ClusterEndpoint{}, ""},

		// Branch: EqualFold match -> IPOverride returned, surrounding
		// whitespace trimmed by the TrimSpace on the returned value.
		{"exact match returns trimmed ip override", "pve-alpha", endpoints, "10.0.0.5"},

		// Branch: EqualFold is case-insensitive on BOTH the lookup name and
		// the stored NodeName.
		{"case-insensitive upper match", "PVE-ALPHA", endpoints, "10.0.0.5"},
		{"case-insensitive mixed match", "pve-Alpha", endpoints, "10.0.0.5"},

		// Branch: surrounding whitespace on the stored NodeName and on the
		// lookup name is trimmed before the EqualFold comparison.
		{"stored nodename whitespace trimmed on match", "pve-beta", endpoints, "10.0.0.6"},
		{"lookup nodename whitespace trimmed on match", "  pve-beta  ", endpoints, "10.0.0.6"},

		// Branch: matching entry whose IPOverride is whitespace-only -> "".
		{"matching entry with whitespace-only ip returns empty", "pve-empty", endpoints, ""},

		// Branch: matching entry whose IPOverride is literally empty -> "".
		{"matching entry with empty ip returns empty", "pve-zero", endpoints, ""},

		// Branch: first match wins when two endpoints share a NodeName.
		{"first matching entry wins on duplicate", "pve-dup", endpoints, "10.0.0.7"},

		// Branch: no matching entry -> "".
		{"no match returns empty", "pve-missing", endpoints, ""},
		{"whitespace-only lookup with no whitespace-only nodename returns empty", "   ", endpoints, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := monitorExistingClusterIPOverride(tc.nodeName, tc.existing)
			if got != tc.want {
				t.Fatalf("monitorExistingClusterIPOverride(%q, ...) = %q, want %q",
					tc.nodeName, got, tc.want)
			}
		})
	}
}

func TestBranchCovMonitorExistingClusterFingerprint(t *testing.T) {
	endpoints := []config.ClusterEndpoint{
		{NodeName: "pve-alpha", Fingerprint: "  AA:BB:CC:DD:EE:FF  "},
		{NodeName: "  pve-beta  ", Fingerprint: "11:22:33:44:55:66"},
		{NodeName: "pve-empty", Fingerprint: "   "},
		{NodeName: "pve-zero", Fingerprint: ""},
		{NodeName: "pve-dup", Fingerprint: "first:fp"},
		{NodeName: "pve-dup", Fingerprint: "second:fp"},
	}

	cases := []struct {
		name     string
		nodeName string
		existing []config.ClusterEndpoint
		want     string
	}{
		// Branch: nil/empty existing slice -> "".
		{"nil existing slice returns empty", "pve-alpha", nil, ""},
		{"empty existing slice returns empty", "pve-alpha", []config.ClusterEndpoint{}, ""},

		// Branch: EqualFold match -> Fingerprint returned (trimmed).
		{"exact match returns trimmed fingerprint", "pve-alpha", endpoints, "AA:BB:CC:DD:EE:FF"},

		// Branch: case-insensitive EqualFold on both sides.
		{"case-insensitive upper match", "PVE-ALPHA", endpoints, "AA:BB:CC:DD:EE:FF"},
		{"case-insensitive mixed match", "pve-Alpha", endpoints, "AA:BB:CC:DD:EE:FF"},

		// Branch: whitespace on stored and lookup NodeName trimmed.
		{"stored nodename whitespace trimmed on match", "pve-beta", endpoints, "11:22:33:44:55:66"},
		{"lookup nodename whitespace trimmed on match", "  pve-beta  ", endpoints, "11:22:33:44:55:66"},

		// Branch: matching entry with whitespace-only Fingerprint -> "".
		{"matching entry with whitespace-only fingerprint returns empty", "pve-empty", endpoints, ""},

		// Branch: matching entry with empty Fingerprint -> "".
		{"matching entry with empty fingerprint returns empty", "pve-zero", endpoints, ""},

		// Branch: first match wins on duplicate NodeName.
		{"first matching entry wins on duplicate", "pve-dup", endpoints, "first:fp"},

		// Branch: no matching entry -> "".
		{"no match returns empty", "pve-missing", endpoints, ""},
		{"whitespace-only lookup with no whitespace-only nodename returns empty", "   ", endpoints, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := monitorExistingClusterFingerprint(tc.nodeName, tc.existing)
			if got != tc.want {
				t.Fatalf("monitorExistingClusterFingerprint(%q, ...) = %q, want %q",
					tc.nodeName, got, tc.want)
			}
		})
	}
}

func TestBranchCovBuildStorageSummaryCapacityTrend(t *testing.T) {
	// Fixed millisecond timestamps so assertions are deterministic and the
	// "older point seen after newer" branch can be exercised deterministically.
	const (
		t1ms int64 = 1_000_000
		t2ms int64 = 2_000_000
		t3ms int64 = 3_000_000
	)
	t1 := time.UnixMilli(t1ms)
	t2 := time.UnixMilli(t2ms)
	t3 := time.UnixMilli(t3ms)

	pt := func(ts time.Time, v float64) MetricPoint {
		return MetricPoint{Timestamp: ts, Value: v}
	}

	cases := []struct {
		name        string
		poolMetrics map[string]map[string][]MetricPoint
		// wantNil marks cases that must return a nil slice (early-return path).
		wantNil bool
		// wantPoints is the expected output slice. For non-nil-empty results
		// pass a non-nil zero-length slice; for nil results set wantNil=true.
		wantPoints []MetricPoint
		wantOldest int64
	}{
		// Branch: nil input -> len(buckets)==0 early return path AND
		// oldestTimestamp never set (stays 0).
		{"nil input returns nil slice and zero oldest",
			nil, true, nil, 0},

		// Branch: empty pool map -> same early-return path.
		{"empty pool map returns nil slice and zero oldest",
			map[string]map[string][]MetricPoint{}, true, nil, 0},

		// Branch: pool present but inner metric map nil -> reading
		// nil["used"] / nil["avail"] yields empty slices, so no buckets.
		{"pool with nil inner metric map returns nil slice",
			map[string]map[string][]MetricPoint{"p1": nil}, true, nil, 0},

		// Branch: pool with neither "used" nor "avail" key -> buckets empty.
		{"pool with unrelated metric keys returns nil slice",
			map[string]map[string][]MetricPoint{
				"p1": {"total": []MetricPoint{pt(t1, 100)}},
			}, true, nil, 0},

		// Happy path: single pool, used + avail at same timestamp ->
		// bucket hasUsed && hasAvail -> emitted at used/(used+avail)*100.
		{"single pool used and avail at same ts emits percentage",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 25)},
					"avail": []MetricPoint{pt(t1, 75)},
				},
			},
			false, []MetricPoint{{Timestamp: t1, Value: 25.0}}, t1ms},

		// Branch: aggregation across pools at the SAME timestamp. The
		// second pool's used and avail both find an existing bucket
		// (exercises bucket != nil arm in BOTH inner loops); values sum.
		{"two pools at same ts aggregate by summing",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 50)},
					"avail": []MetricPoint{pt(t1, 50)},
				},
				"p2": {
					"used":  []MetricPoint{pt(t1, 100)},
					"avail": []MetricPoint{pt(t1, 100)},
				},
			},
			// total used=150, total=300 -> 150/300*100 = 50
			false, []MetricPoint{{Timestamp: t1, Value: 50.0}}, t1ms},

		// Branch: multiple timestamps returned sorted ascending; oldest is
		// the minimum. Within one pool, used points create the buckets and
		// the subsequent avail loop finds them existing.
		{"multiple timestamps returned sorted ascending",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t3, 30), pt(t1, 10), pt(t2, 20)},
					"avail": []MetricPoint{pt(t3, 70), pt(t1, 90), pt(t2, 80)},
				},
			},
			false,
			[]MetricPoint{
				{Timestamp: t1, Value: 10.0},
				{Timestamp: t2, Value: 20.0},
				{Timestamp: t3, Value: 30.0},
			},
			t1ms},

		// Branch: oldestTimestamp update arm `timestamp < oldestTimestamp`
		// -- iteration sees the newer timestamp first, then the older one.
		{"older point seen after newer still sets oldest to min",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t2, 20), pt(t1, 10)},
					"avail": []MetricPoint{pt(t2, 80), pt(t1, 90)},
				},
			},
			false,
			[]MetricPoint{
				{Timestamp: t1, Value: 10.0},
				{Timestamp: t2, Value: 20.0},
			},
			t1ms},

		// Branch: used-only bucket -> hasAvail=false -> skipped in output
		// loop. Buckets non-empty so we go past the early return; result is
		// a NON-nil empty slice and oldestTimestamp is still set.
		{"used only bucket skipped but oldest timestamp set",
			map[string]map[string][]MetricPoint{
				"p1": {"used": []MetricPoint{pt(t1, 50)}},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: avail-only bucket -> hasUsed=false -> skipped.
		{"avail only bucket skipped but oldest timestamp set",
			map[string]map[string][]MetricPoint{
				"p1": {"avail": []MetricPoint{pt(t1, 50)}},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: used and avail land at DIFFERENT timestamps -> each
		// bucket is half-complete -> all skipped; oldest is min(t1,t2)=t1.
		{"used and avail at different timestamps both skipped",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 50)},
					"avail": []MetricPoint{pt(t2, 50)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: total == 0 (used=0, avail=0) -> `total <= 0` skip arm.
		{"zero total skipped via total le zero",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 0)},
					"avail": []MetricPoint{pt(t1, 0)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: total negative (avail negative) -> `total <= 0` skip arm.
		{"negative total skipped via total le zero",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 5)},
					"avail": []MetricPoint{pt(t1, -10)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: NaN used propagates to NaN total -> math.IsNaN skip arm.
		{"nan used makes total nan and is skipped",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, math.NaN())},
					"avail": []MetricPoint{pt(t1, 50)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: +Inf used -> +Inf total -> math.IsInf(total, 0) skip arm.
		{"positive inf used makes total inf and is skipped",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, math.Inf(1))},
					"avail": []MetricPoint{pt(t1, 50)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: -Inf used -> -Inf total -> IsInf skip arm.
		{"negative inf used makes total neg inf and is skipped",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, math.Inf(-1))},
					"avail": []MetricPoint{pt(t1, 50)},
				},
			},
			false, []MetricPoint{}, t1ms},

		// Branch: negative used with positive total -> NOT skipped; emits a
		// NEGATIVE percentage. This documents real (suspect) behavior; see
		// GLM_REPORT.md "suspected source bugs".
		{"negative used yields negative percentage when total positive",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, -5)},
					"avail": []MetricPoint{pt(t1, 10)},
				},
			},
			// (-5 / 5) * 100 = -100
			false, []MetricPoint{{Timestamp: t1, Value: -100.0}}, t1ms},

		// Mixed: one full bucket (t1) emits, one half bucket (t2, used-only)
		// is skipped but still contributes to oldestTimestamp when older.
		{"mixed full and half bucket only emits full bucket",
			map[string]map[string][]MetricPoint{
				"p1": {
					"used":  []MetricPoint{pt(t1, 30), pt(t2, 99)},
					"avail": []MetricPoint{pt(t1, 70)},
				},
			},
			false, []MetricPoint{{Timestamp: t1, Value: 30.0}}, t1ms},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPoints, gotOldest := buildStorageSummaryCapacityTrend(tc.poolMetrics)

			if gotOldest != tc.wantOldest {
				t.Errorf("oldest timestamp = %d, want %d", gotOldest, tc.wantOldest)
			}

			if tc.wantNil {
				if gotPoints != nil {
					t.Fatalf("expected nil points slice, got %v", gotPoints)
				}
				return
			}
			if gotPoints == nil {
				t.Fatalf("expected non-nil points slice, got nil")
			}
			if len(gotPoints) != len(tc.wantPoints) {
				t.Fatalf("points length = %d, want %d (got=%v)",
					len(gotPoints), len(tc.wantPoints), gotPoints)
			}
			for i, want := range tc.wantPoints {
				got := gotPoints[i]
				if !got.Timestamp.Equal(want.Timestamp) {
					t.Errorf("point[%d].Timestamp = %v, want %v", i, got.Timestamp, want.Timestamp)
				}
				if !floatEq(got.Value, want.Value) {
					t.Errorf("point[%d].Value = %v, want %v", i, got.Value, want.Value)
				}
			}
		})
	}
}

func TestBranchCovParseNouveauGPUTemps(t *testing.T) {
	// parseNouveauGPUTemps never dereferences its *TemperatureCollector
	// receiver, so a zero-value instance is sufficient.
	tc := &TemperatureCollector{}

	cases := []struct {
		name     string
		chipName string
		chipMap  map[string]interface{}
		// seedGPU pre-populates temp.GPU to exercise the append-to-existing
		// path (the function always appends, never replaces).
		seedGPU []models.GPUTemp
		wantGPU []models.GPUTemp
	}{
		// Branch: nil chipMap -> loop body never runs -> gpuTemp.Edge stays
		// 0 -> nothing appended. Result slice stays nil.
		{"nil chipmap appends nothing", "nouveau-pci-0100", nil, nil, nil},

		// Branch: empty chipMap -> same as nil.
		{"empty chipmap appends nothing",
			"nouveau-pci-0100", map[string]interface{}{}, nil, nil},

		// Happy path: "GPU core" sensor with a float64 temp1_input.
		// Documents that Device is set verbatim from chipName and Edge gets
		// the value (the function maps gpu/core sensors to Edge only).
		{"gpu core sensor with float input appends edge entry",
			"nouveau-pci-0100",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": 54.0},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0100", Edge: 54.0}}},

		// Branch: sensor name contains "gpu" only (no "core") -> still
		// mapped (the OR arm of the Contains check).
		{"sensor name containing only gpu maps to edge",
			"nouveau-pci-0200",
			map[string]interface{}{
				"GPU": map[string]interface{}{"temp1_input": 60.0},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0200", Edge: 60.0}}},

		// Branch: sensor name contains "core" only (no "gpu") -> mapped.
		{"sensor name containing only core maps to edge",
			"nouveau-pci-0300",
			map[string]interface{}{
				"Core": map[string]interface{}{"temp1_input": 61.0},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0300", Edge: 61.0}}},

		// Branch: case-insensitive name match via ToLower - uppercase
		// "GPU CORE" still hits both Contains arms.
		{"uppercase gpu core sensor maps to edge",
			"nouveau-pci-0400",
			map[string]interface{}{
				"GPU CORE": map[string]interface{}{"temp1_input": 62.0},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0400", Edge: 62.0}}},

		// Branch: int-typed temp input is accepted by extractTempInput.
		{"int typed temp input accepted",
			"nouveau-pci-0500",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": 63},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0500", Edge: 63.0}}},

		// Branch: string-typed temp input (millidegrees >= 1000) is divided
		// by 1000 by parseStringTemperature.
		{"string typed millidegree input scaled down",
			"nouveau-pci-0600",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": "64000"},
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-0600", Edge: 64.0}}},

		// Branch: sensor value is NOT a map[string]interface{} (the type
		// assertion `ok` fails) -> continue, nothing appended.
		{"non-map sensor value skipped string",
			"nouveau-pci-0700",
			map[string]interface{}{
				"GPU core": "not-a-map",
			},
			nil, nil},
		{"non-map sensor value skipped int",
			"nouveau-pci-0700",
			map[string]interface{}{
				"GPU core": 42,
			},
			nil, nil},
		{"non-map sensor value skipped slice",
			"nouveau-pci-0700",
			map[string]interface{}{
				"GPU core": []interface{}{1, 2},
			},
			nil, nil},

		// Branch: sensor map has no `*_input` key -> extractTempInput
		// returns NaN -> `math.IsNaN(tempVal)` skip arm.
		{"sensor map without input key skipped",
			"nouveau-pci-0800",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_max": 80.0},
			},
			nil, nil},

		// Branch: temp input explicitly NaN -> IsNaN skip arm.
		{"nan temp input skipped",
			"nouveau-pci-0900",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": math.NaN()},
			},
			nil, nil},

		// Branch: temp input <= 0 -> `tempVal <= 0` skip arm (boundary at 0
		// and below).
		{"zero temp input skipped",
			"nouveau-pci-1000",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": 0.0},
			},
			nil, nil},
		{"negative temp input skipped",
			"nouveau-pci-1000",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": -5.0},
			},
			nil, nil},

		// Branch: valid temp but sensor name contains NEITHER "gpu" NOR
		// "core" -> the assignment arm is skipped -> Edge stays 0 -> not
		// appended (the trailing `gpuTemp.Edge > 0` guard).
		{"unrelated sensor name with valid temp not appended",
			"nouveau-pci-1100",
			map[string]interface{}{
				"Memory": map[string]interface{}{"temp1_input": 70.0},
			},
			nil, nil},

		// Branch: multiple matching sensors -> Edge is overwritten on each
		// iteration; the LAST one encountered wins. (Go map iteration order
		// is randomized, so we use a single-element chipMap to make the
		// "last wins" assertion deterministic; this case instead documents
		// that a single match correctly populates Edge when other
		// non-matching sensors are also present.)
		{"valid sensor among noise appends only the valid one",
			"nouveau-pci-1200",
			map[string]interface{}{
				"Noise1":    map[string]interface{}{"temp1_input": 1.0},  // name doesn't match
				"GPU core":  map[string]interface{}{"temp1_input": 55.0}, // matches
				"bad-shape": "ignored",                                   // non-map
			},
			nil,
			[]models.GPUTemp{{Device: "nouveau-pci-1200", Edge: 55.0}}},

		// Branch: pre-existing temp.GPU is preserved; the parsed entry is
		// APPENDED (never replaces).
		{"parsed entry appended to pre-existing gpu slice",
			"nouveau-pci-1300",
			map[string]interface{}{
				"GPU core": map[string]interface{}{"temp1_input": 66.0},
			},
			[]models.GPUTemp{{Device: "amdgpu-pci-0001", Edge: 40.0, Junction: 45.0}},
			[]models.GPUTemp{
				{Device: "amdgpu-pci-0001", Edge: 40.0, Junction: 45.0},
				{Device: "nouveau-pci-1300", Edge: 66.0},
			}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			temp := &models.Temperature{GPU: c.seedGPU}
			tc.parseNouveauGPUTemps(c.chipName, c.chipMap, temp)

			// Normalize nil -> empty for comparison parity: an unset slice
			// and an explicitly-empty slice both mean "no GPU appended".
			got := temp.GPU
			if len(got) == 0 && len(c.wantGPU) == 0 {
				return
			}
			if len(got) != len(c.wantGPU) {
				t.Fatalf("GPU slice length = %d, want %d (got=%+v)",
					len(got), len(c.wantGPU), got)
			}
			for i, want := range c.wantGPU {
				g := got[i]
				if g.Device != want.Device {
					t.Errorf("GPU[%d].Device = %q, want %q", i, g.Device, want.Device)
				}
				if !floatEq(g.Edge, want.Edge) {
					t.Errorf("GPU[%d].Edge = %v, want %v", i, g.Edge, want.Edge)
				}
				if !floatEq(g.Junction, want.Junction) {
					t.Errorf("GPU[%d].Junction = %v, want %v", i, g.Junction, want.Junction)
				}
				if !floatEq(g.Mem, want.Mem) {
					t.Errorf("GPU[%d].Mem = %v, want %v", i, g.Mem, want.Mem)
				}
			}
		})
	}
}
