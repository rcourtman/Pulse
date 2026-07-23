package config_test

import (
	"reflect"
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// This file adds branch coverage for (*AlertConfig).UnmarshalJSON
// (types.go:276). UnmarshalJSON decodes into an alias type, then re-decodes
// the same bytes into a raw map, then calls NormalizeAlertConfigAliases. The
// tests assert the actual normalization result (legacy alias keys stripped,
// canonical keys retained), the malformed-JSON error path, and the
// nil-vs-present map branches.
//
// Purity: every case is exercised with in-memory []byte payloads -- no
// network, SSH, daemon, or database is required.

// TestBranchcov0723pmUnmarshalJSONErrorArms covers the first json.Unmarshal
// error return (types.go:279-281). It also demonstrates, via valid inputs, that
// the SECOND json.Unmarshal error return (types.go:285-287) is unreachable:
// both calls consume the same `data`, and decoding into map[string]json.RawMessage
// is strictly more permissive than decoding into the struct alias (it accepts
// any valid JSON object or null and never validates field types). Every input
// that makes the map decode fail (array/number/string/scalar) makes the struct
// decode fail first, so control never reaches the second error return when the
// first succeeds. See GLM_REPORT_go-alertcfg.md.
func TestBranchcov0723pmUnmarshalJSONErrorArms(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{name: "truncated object", data: `{bad`},
		{name: "trailing garbage after valid object", data: `{"enabled":true}garbage`},
		{name: "bare EOF", data: ``},
		{name: "top level array instead of object", data: `[1,2,3]`},
		{name: "top level number instead of object", data: `42`},
		{name: "top level string instead of object", data: `"hello"`},
		{name: "field type mismatch on bool", data: `{"enabled":"notabool"}`},
		{name: "field type mismatch nested", data: `{"guestDefaults":{"cpu":{"trigger":"x"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Pre-populate to prove the error path returns BEFORE mutating `*c`.
			cfg := &alertconfig.AlertConfig{Enabled: true, MinimumDelta: 7}
			err := cfg.UnmarshalJSON([]byte(tc.data))
			if err == nil {
				t.Fatalf("UnmarshalJSON(%q) err = nil, want non-nil", tc.data)
			}
			if cfg.Enabled != true {
				t.Fatalf("Enabled = %v, want true (config must be untouched on first-unmarshal error)", cfg.Enabled)
			}
			if cfg.MinimumDelta != 7 {
				t.Fatalf("MinimumDelta = %v, want 7 (config must be untouched on first-unmarshal error)", cfg.MinimumDelta)
			}
		})
	}

	// Empirically pin the unreachability of the second error arm: every input
	// the struct decoder accepts also round-trips through the raw-map decode
	// without error and reaches normalization.
	t.Run("valid object and null never hit second error arm", func(t *testing.T) {
		for _, data := range []string{`{}`, `null`, `{"enabled":true,"timeThresholds":{"guest":1}}`} {
			cfg := &alertconfig.AlertConfig{}
			if err := cfg.UnmarshalJSON([]byte(data)); err != nil {
				t.Fatalf("UnmarshalJSON(%q) err = %v, want nil (second arm should not fail)", data, err)
			}
		}
	})
}

// TestBranchcov0723pmUnmarshalJSONNormalization covers the success path of
// UnmarshalJSON and every observable branch of NormalizeAlertConfigAliases that
// it invokes: TimeThresholds nil (skip block) vs present (iterate), the
// "typeKey == all" / "typeKey == empty" continue arms (keys retained), the
// legacy-unsupported delete arm, and the canonical-survives arm.
func TestBranchcov0723pmUnmarshalJSONNormalization(t *testing.T) {
	cases := []struct {
		name string
		data string
		// wantTime is the exact expected TimeThresholds map after normalization.
		wantTime map[string]int
		// wantTimeNil asserts the map is nil (covers the nil-skip-block branch).
		wantTimeNil bool
	}{
		{
			// Empty object: both maps nil -> TimeThresholds block skipped
			// (!= nil false) and MetricTimeThresholds early-returns (len 0).
			name:        "empty object leaves TimeThresholds nil",
			data:        `{}`,
			wantTimeNil: true,
		},
		{
			// JSON null: struct decode succeeds (zero value), second decode of
			// null into the map leaves it nil; normalize skips both blocks.
			name:        "null json leaves TimeThresholds nil",
			data:        `null`,
			wantTimeNil: true,
		},
		{
			// Canonical keys only -> all retained, none deleted.
			name:     "canonical time threshold keys retained",
			data:     `{"timeThresholds":{"guest":30,"node":60,"storage":15}}`,
			wantTime: map[string]int{"guest": 30, "node": 60, "storage": 15},
		},
		{
			// Mixed: legacy alias keys (qemu/lxc/host) are unsupported -> deleted;
			// canonical keys (guest/node) survive. Asserts the delete arm AND the
			// retain arm in a single payload.
			name:     "legacy alias keys stripped while canonical survive",
			data:     `{"timeThresholds":{"qemu":10,"lxc":20,"host":40,"guest":30,"node":60}}`,
			wantTime: map[string]int{"guest": 30, "node": 60},
		},
		{
			// "all" maps to CanonicalAlertResourceType "all" -> continue (kept).
			// "" maps to "" -> continue (kept). guest is canonical -> kept.
			// None are deleted; this exercises both `continue` arms.
			name: "all and empty-string keys retained via continue arm",
			data: `{"timeThresholds":{"all":99,"":7,"guest":30}}`,
			wantTime: map[string]int{
				"all":   99,
				"":      7,
				"guest": 30,
			},
		},
		{
			// "kubernetes-cluster" is in the unsupported switch -> deleted;
			// "agent disk" canonicalizes to "agent disk" (default arm, not in
			// any multi-word canonical case) and is in the unsupported switch ->
			// deleted; "pbs" canonical -> kept.
			name:     "kubernetes-cluster and agent disk stripped pbs kept",
			data:     `{"timeThresholds":{"kubernetes-cluster":8,"agent disk":9,"pbs":12}}`,
			wantTime: map[string]int{"pbs": 12},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{}
			if err := cfg.UnmarshalJSON([]byte(tc.data)); err != nil {
				t.Fatalf("UnmarshalJSON(%q) err = %v, want nil", tc.data, err)
			}
			if tc.wantTimeNil {
				if cfg.TimeThresholds != nil {
					t.Fatalf("TimeThresholds = %v, want nil", cfg.TimeThresholds)
				}
				return
			}
			if cfg.TimeThresholds == nil {
				t.Fatalf("TimeThresholds = nil, want %v", tc.wantTime)
			}
			if !reflect.DeepEqual(cfg.TimeThresholds, tc.wantTime) {
				t.Fatalf("TimeThresholds = %v, want %v", cfg.TimeThresholds, tc.wantTime)
			}
		})
	}

	// Separately assert that a non-threshold canonical field round-trips, so the
	// test is not solely about map normalization.
	t.Run("canonical enabled flag decodes through alias", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		if err := cfg.UnmarshalJSON([]byte(`{"enabled":true}`)); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if !cfg.Enabled {
			t.Fatalf("Enabled = false, want true")
		}
	})
}

// TestBranchcov0723pmUnmarshalJSONMetricThresholds covers the
// MetricTimeThresholds half of NormalizeAlertConfigAliases: the len==0 early
// return (covered above by the empty/null cases) vs the iteration path, where
// legacy type keys are deleted, "all"/empty-string keys are retained via the
// continue arm, and canonical keys survive.
func TestBranchcov0723pmUnmarshalJSONMetricThresholds(t *testing.T) {
	t.Run("legacy type keys stripped canonical and all retained", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		data := `{"metricTimeThresholds":{"qemu":{"cpu":5},"guest":{"mem":10},"all":{"disk":15},"":{"net":20}}}`
		if err := cfg.UnmarshalJSON([]byte(data)); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if cfg.MetricTimeThresholds == nil {
			t.Fatal("MetricTimeThresholds = nil, want non-nil")
		}
		if _, ok := cfg.MetricTimeThresholds["qemu"]; ok {
			t.Fatalf("qemu should have been stripped, map=%+v", cfg.MetricTimeThresholds)
		}
		want := map[string]map[string]int{
			"guest": {"mem": 10},
			"all":   {"disk": 15},
			"":      {"net": 20},
		}
		if !reflect.DeepEqual(cfg.MetricTimeThresholds, want) {
			t.Fatalf("MetricTimeThresholds = %+v, want %+v", cfg.MetricTimeThresholds, want)
		}
	})

	t.Run("all legacy type keys stripped leaves empty map", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		data := `{"metricTimeThresholds":{"docker":{"cpu":1},"k8s":{"mem":2}}}`
		if err := cfg.UnmarshalJSON([]byte(data)); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if cfg.MetricTimeThresholds == nil {
			t.Fatal("MetricTimeThresholds = nil, want non-nil empty map")
		}
		if len(cfg.MetricTimeThresholds) != 0 {
			t.Fatalf("MetricTimeThresholds = %+v, want empty (both keys unsupported)", cfg.MetricTimeThresholds)
		}
	})

	t.Run("absent metricTimeThresholds leaves field nil", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		if err := cfg.UnmarshalJSON([]byte(`{"enabled":true}`)); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if cfg.MetricTimeThresholds != nil {
			t.Fatalf("MetricTimeThresholds = %+v, want nil (len==0 early return)", cfg.MetricTimeThresholds)
		}
	})
}
