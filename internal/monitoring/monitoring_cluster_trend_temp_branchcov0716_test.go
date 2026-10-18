package monitoring

import (
	"math"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// This file is a purpose-built branch-coverage test set (selected via
// `-run BranchCov`) for pure/near-pure helpers in the monitoring package
// whose conditional arms were previously uncovered or under-covered:
//
//   - monitorExistingClusterIPOverride          (monitor_pve_cluster.go)
//   - monitorExistingClusterFingerprint         (monitor_pve_cluster.go)
//   - parseNouveauGPUTemps                      (temperature.go)
//
// Conventions match sibling in-package tests in this directory (see
// monitoring_infra_keys_branchcov0716_test.go): same package, stdlib
// `testing` only, table-driven, no testify.
//
// The receiver of parseNouveauGPUTemps (*TemperatureCollector) is never
// dereferenced inside the function, so a zero-value collector is safe.

// floatEq compares two floats with an absolute tolerance. 1e-6 is comfortably
// strict for the parsed temperatures compared below.
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
