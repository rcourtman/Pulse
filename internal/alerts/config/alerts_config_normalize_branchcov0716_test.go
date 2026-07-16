package config_test

import (
	"reflect"
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// This file exercises genuinely uncovered branches of the pure/near-pure
// normalization helpers in internal/alerts/config/normalize.go. Test names use
// the BranchCov prefix so `-run BranchCov` selects them in isolation.

// floatEq compares two float64 values with a tiny tolerance to avoid surprises
// from any future floating-point derivation; the current implementations only
// perform exact assignments/subtractions so equality holds exactly.
func floatEq(a, b float64) bool {
	return a == b
}

// htEq compares two HysteresisThreshold values by value.
func htEq(a, b alertconfig.HysteresisThreshold) bool {
	return floatEq(a.Trigger, b.Trigger) && floatEq(a.Clear, b.Clear)
}

// TestBranchCovEnsureValidHysteresis covers every branch of EnsureValidHysteresis:
// nil receiver, disabled (Trigger<=0), Clear>=Trigger with positive result,
// Clear>=Trigger with negative result clamped to 0, and the no-op valid case.
func TestBranchCovEnsureValidHysteresis(t *testing.T) {
	tests := []struct {
		name        string
		input       *alertconfig.HysteresisThreshold
		metric      string
		wantTrigger float64
		wantClear   float64
	}{
		{
			name:   "nil threshold returns without panic",
			input:  nil,
			metric: "nil.metric",
		},
		{
			name:        "trigger zero leaves threshold untouched (disabled)",
			input:       &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 50},
			metric:      "disabled.zero",
			wantTrigger: 0,
			wantClear:   50,
		},
		{
			name:        "trigger negative leaves threshold untouched (disabled)",
			input:       &alertconfig.HysteresisThreshold{Trigger: -5, Clear: 90},
			metric:      "disabled.negative",
			wantTrigger: -5,
			wantClear:   90,
		},
		{
			name:        "clear greater than trigger auto-fixes to trigger-5",
			input:       &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 90},
			metric:      "cpu",
			wantTrigger: 80,
			wantClear:   75,
		},
		{
			name:        "clear equals trigger boundary still triggers fix (>=)",
			input:       &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 80},
			metric:      "boundary.equal",
			wantTrigger: 80,
			wantClear:   75,
		},
		{
			name:        "fix clamps to zero when trigger less than 5",
			input:       &alertconfig.HysteresisThreshold{Trigger: 3, Clear: 10},
			metric:      "clamp.zero",
			wantTrigger: 3,
			wantClear:   0,
		},
		{
			name:        "already valid threshold is untouched",
			input:       &alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
			metric:      "valid",
			wantTrigger: 85,
			wantClear:   80,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alertconfig.EnsureValidHysteresis(tc.input, tc.metric)
			if tc.input == nil {
				return
			}
			if !floatEq(tc.input.Trigger, tc.wantTrigger) {
				t.Fatalf("Trigger = %v, want %v", tc.input.Trigger, tc.wantTrigger)
			}
			if !floatEq(tc.input.Clear, tc.wantClear) {
				t.Fatalf("Clear = %v, want %v", tc.input.Clear, tc.wantClear)
			}
		})
	}
}

// TestBranchCovNormalizeStorageDefaults covers all three trigger/clear branches
// of NormalizeStorageDefaults including the inner clamp-to-zero sub-branch.
func TestBranchCovNormalizeStorageDefaults(t *testing.T) {
	tests := []struct {
		name        string
		trigger     float64
		clear       float64
		wantTrigger float64
		wantClear   float64
	}{
		{
			name:        "negative trigger resets to full default pair",
			trigger:     -10,
			clear:       1234,
			wantTrigger: 85,
			wantClear:   80,
		},
		{
			name:        "zero trigger forces clear to zero even when clear was positive",
			trigger:     0,
			clear:       50,
			wantTrigger: 0,
			wantClear:   0,
		},
		{
			name:        "positive trigger with non-positive clear derives clear as trigger-5",
			trigger:     90,
			clear:       0,
			wantTrigger: 90,
			wantClear:   85,
		},
		{
			name:        "positive trigger with negative clear derives clear and clamps to zero when trigger<5",
			trigger:     3,
			clear:       -1,
			wantTrigger: 3,
			wantClear:   0,
		},
		{
			name:        "already populated valid pair is left untouched",
			trigger:     95,
			clear:       88,
			wantTrigger: 95,
			wantClear:   88,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{
				StorageDefault: alertconfig.HysteresisThreshold{
					Trigger: tc.trigger,
					Clear:   tc.clear,
				},
			}
			alertconfig.NormalizeStorageDefaults(cfg)
			got := cfg.StorageDefault
			if !floatEq(got.Trigger, tc.wantTrigger) {
				t.Fatalf("Trigger = %v, want %v", got.Trigger, tc.wantTrigger)
			}
			if !floatEq(got.Clear, tc.wantClear) {
				t.Fatalf("Clear = %v, want %v", got.Clear, tc.wantClear)
			}
		})
	}
}

// TestBranchCovNormalizeDockerThreshold covers each branch of
// NormalizeDockerThreshold, including the early-return Trigger==0 path that
// intentionally preserves a positive Clear (unlike NormalizeStorageDefaults).
func TestBranchCovNormalizeDockerThreshold(t *testing.T) {
	const def = 80.0

	tests := []struct {
		name string
		in   alertconfig.HysteresisThreshold
		want alertconfig.HysteresisThreshold
	}{
		{
			name: "negative trigger falls back to default then derives clear",
			in:   alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
			want: alertconfig.HysteresisThreshold{Trigger: def, Clear: def - 5},
		},
		{
			name: "zero trigger with negative clear clamps clear to zero and returns early",
			in:   alertconfig.HysteresisThreshold{Trigger: 0, Clear: -7},
			want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		{
			name: "zero trigger with positive clear preserves clear (disabled metric)",
			in:   alertconfig.HysteresisThreshold{Trigger: 0, Clear: 42},
			want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 42},
		},
		{
			name: "positive trigger with non-positive clear derives clear",
			in:   alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
			want: alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		{
			name: "derived clear clamps to zero when trigger less than 5",
			in:   alertconfig.HysteresisThreshold{Trigger: 2, Clear: -1},
			want: alertconfig.HysteresisThreshold{Trigger: 2, Clear: 0},
		},
		{
			name: "clear above trigger gets repaired by EnsureValidHysteresis safety net",
			in:   alertconfig.HysteresisThreshold{Trigger: 80, Clear: 95},
			want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		{
			name: "valid pair untouched",
			in:   alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
			want: alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.NormalizeDockerThreshold(tc.in, def, "docker.test")
			if !htEq(got, tc.want) {
				t.Fatalf("NormalizeDockerThreshold(%+v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

// TestBranchCovNormalizeDockerDefaults covers NormalizeDockerDefaults: default
// seeding, the ServiceCritGapPct<ServiceWarnGapPct repair, every
// StatePoweredOffSeverity NormalizePoweredOffSeverity arm, and the
// UpdateAlertDelayHours==0 default.
func TestBranchCovNormalizeDockerDefaults(t *testing.T) {
	t.Run("all zero fields are seeded with canonical defaults", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeDockerDefaults(cfg)
		d := cfg.DockerDefaults

		// NOTE on actual behavior: NormalizeDockerThreshold treats Trigger==0 as
		// "disabled" (early return), so a zero-value CPU/Memory/Disk threshold is
		// NOT seeded with the defaultTrigger passed by NormalizeDockerDefaults.
		// Only the scalar fields (<= 0 guards) receive defaults. See GLM_REPORT.md
		// for the suspected source inconsistency this documents.
		if !htEq(d.CPU, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}) {
			t.Fatalf("CPU = %+v, want {0 0} (Trigger==0 disabled path, NOT seeded)", d.CPU)
		}
		if !htEq(d.Memory, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}) {
			t.Fatalf("Memory = %+v, want {0 0} (Trigger==0 disabled path, NOT seeded)", d.Memory)
		}
		if !htEq(d.Disk, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}) {
			t.Fatalf("Disk = %+v, want {0 0} (Trigger==0 disabled path, NOT seeded)", d.Disk)
		}
		if d.RestartCount != 3 {
			t.Fatalf("RestartCount = %d, want 3", d.RestartCount)
		}
		if d.RestartWindow != 300 {
			t.Fatalf("RestartWindow = %d, want 300", d.RestartWindow)
		}
		if d.MemoryWarnPct != 90 {
			t.Fatalf("MemoryWarnPct = %d, want 90", d.MemoryWarnPct)
		}
		if d.MemoryCriticalPct != 95 {
			t.Fatalf("MemoryCriticalPct = %d, want 95", d.MemoryCriticalPct)
		}
		if d.ServiceWarnGapPct != 10 {
			t.Fatalf("ServiceWarnGapPct = %d, want 10", d.ServiceWarnGapPct)
		}
		if d.ServiceCritGapPct != 50 {
			t.Fatalf("ServiceCritGapPct = %d, want 50", d.ServiceCritGapPct)
		}
		if d.StatePoweredOffSeverity != alertconfig.AlertLevelWarning {
			t.Fatalf("StatePoweredOffSeverity = %q, want %q", d.StatePoweredOffSeverity, alertconfig.AlertLevelWarning)
		}
		if d.UpdateAlertDelayHours != 24 {
			t.Fatalf("UpdateAlertDelayHours = %d, want 24", d.UpdateAlertDelayHours)
		}
	})

	t.Run("negative trigger CPU falls back to defaultTrigger and derives clear", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				CPU:    alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
				Memory: alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
				Disk:   alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		d := cfg.DockerDefaults
		if !htEq(d.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want {80 75}", d.CPU)
		}
		if !htEq(d.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want {85 80}", d.Memory)
		}
		if !htEq(d.Disk, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Disk = %+v, want {85 80}", d.Disk)
		}
	})

	t.Run("critical gap below warning gap is bumped up to warning gap", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				ServiceWarnGapPct: 20,
				ServiceCritGapPct: 5,
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.ServiceWarnGapPct != 20 {
			t.Fatalf("ServiceWarnGapPct = %d, want 20 (preserved)", cfg.DockerDefaults.ServiceWarnGapPct)
		}
		if cfg.DockerDefaults.ServiceCritGapPct != 20 {
			t.Fatalf("ServiceCritGapPct = %d, want 20 (bumped to warning gap)", cfg.DockerDefaults.ServiceCritGapPct)
		}
	})

	t.Run("critical gap above warning gap is left as configured", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				ServiceWarnGapPct: 10,
				ServiceCritGapPct: 60,
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.ServiceCritGapPct != 60 {
			t.Fatalf("ServiceCritGapPct = %d, want 60 (preserved)", cfg.DockerDefaults.ServiceCritGapPct)
		}
	})

	t.Run("StatePoweredOffSeverity empty defaults to warning", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.StatePoweredOffSeverity != alertconfig.AlertLevelWarning {
			t.Fatalf("severity = %q, want warning", cfg.DockerDefaults.StatePoweredOffSeverity)
		}
	})

	t.Run("StatePoweredOffSeverity critical is preserved through NormalizePoweredOffSeverity", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				StatePoweredOffSeverity: alertconfig.AlertLevelCritical,
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.StatePoweredOffSeverity != alertconfig.AlertLevelCritical {
			t.Fatalf("severity = %q, want critical", cfg.DockerDefaults.StatePoweredOffSeverity)
		}
	})

	t.Run("StatePoweredOffSeverity uppercase critical is normalized to critical", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				StatePoweredOffSeverity: alertconfig.AlertLevel("CRITICAL"),
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.StatePoweredOffSeverity != alertconfig.AlertLevelCritical {
			t.Fatalf("severity = %q, want critical (case-insensitive)", cfg.DockerDefaults.StatePoweredOffSeverity)
		}
	})

	t.Run("StatePoweredOffSeverity unrecognized value falls back to warning default arm", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				StatePoweredOffSeverity: alertconfig.AlertLevel("bogus"),
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.StatePoweredOffSeverity != alertconfig.AlertLevelWarning {
			t.Fatalf("severity = %q, want warning (default arm)", cfg.DockerDefaults.StatePoweredOffSeverity)
		}
	})

	t.Run("UpdateAlertDelayHours zero defaults to 24 and positive preserved", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeDockerDefaults(cfg)
		if cfg.DockerDefaults.UpdateAlertDelayHours != 24 {
			t.Fatalf("UpdateAlertDelayHours = %d, want 24", cfg.DockerDefaults.UpdateAlertDelayHours)
		}

		cfg2 := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{UpdateAlertDelayHours: 48},
		}
		alertconfig.NormalizeDockerDefaults(cfg2)
		if cfg2.DockerDefaults.UpdateAlertDelayHours != 48 {
			t.Fatalf("UpdateAlertDelayHours = %d, want 48 (preserved)", cfg2.DockerDefaults.UpdateAlertDelayHours)
		}
	})

	t.Run("positive threshold values preserved end to end", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DockerDefaults: alertconfig.DockerThresholdConfig{
				CPU:               alertconfig.HysteresisThreshold{Trigger: 77, Clear: 70},
				Memory:            alertconfig.HysteresisThreshold{Trigger: 88, Clear: 83},
				Disk:              alertconfig.HysteresisThreshold{Trigger: 99, Clear: 94},
				RestartCount:      7,
				RestartWindow:     600,
				MemoryWarnPct:     80,
				MemoryCriticalPct: 88,
			},
		}
		alertconfig.NormalizeDockerDefaults(cfg)
		d := cfg.DockerDefaults
		if !htEq(d.CPU, alertconfig.HysteresisThreshold{Trigger: 77, Clear: 70}) {
			t.Fatalf("CPU = %+v, want preserved", d.CPU)
		}
		if d.RestartCount != 7 {
			t.Fatalf("RestartCount = %d, want 7", d.RestartCount)
		}
		if d.MemoryCriticalPct != 88 {
			t.Fatalf("MemoryCriticalPct = %d, want 88", d.MemoryCriticalPct)
		}
	})
}

// TestBranchCovNormalizeDiskFillByType exercises the branch internals of
// NormalizeDiskFillByType that are distinct from the sibling-package coverage:
// the lower=="" drop arm, the "existing canonical key not overwritten" arm,
// and the per-key reset path for a negative clear with positive trigger.
func TestBranchCovNormalizeDiskFillByType(t *testing.T) {
	t.Run("whitespace only key is dropped and not re-added", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DiskFillByType: map[string]alertconfig.HysteresisThreshold{
				"   ": {Trigger: 99, Clear: 99},
			},
		}
		alertconfig.NormalizeDiskFillByType(cfg)

		if _, exists := cfg.DiskFillByType["   "]; exists {
			t.Fatalf("whitespace key should be removed, map=%+v", cfg.DiskFillByType)
		}
		if _, exists := cfg.DiskFillByType[""]; exists {
			t.Fatalf("empty canonical key should not be added, map=%+v", cfg.DiskFillByType)
		}
		for _, k := range []string{"nvme", "sata", "hdd"} {
			if _, ok := cfg.DiskFillByType[k]; !ok {
				t.Fatalf("canonical key %q missing, map=%+v", k, cfg.DiskFillByType)
			}
		}
	})

	t.Run("non-canonical already-lowercase key is left in place", func(t *testing.T) {
		custom := alertconfig.HysteresisThreshold{Trigger: 70, Clear: 60}
		cfg := &alertconfig.AlertConfig{
			DiskFillByType: map[string]alertconfig.HysteresisThreshold{
				"external": custom,
			},
		}
		alertconfig.NormalizeDiskFillByType(cfg)
		got, ok := cfg.DiskFillByType["external"]
		if !ok {
			t.Fatalf("non-canonical lowercase key should survive, map=%+v", cfg.DiskFillByType)
		}
		if !htEq(got, custom) {
			t.Fatalf("external = %+v, want %+v preserved", got, custom)
		}
	})

	t.Run("uppercased duplicate of existing canonical key does not overwrite it", func(t *testing.T) {
		original := alertconfig.HysteresisThreshold{Trigger: 50, Clear: 40}
		cfg := &alertconfig.AlertConfig{
			DiskFillByType: map[string]alertconfig.HysteresisThreshold{
				"NVME": {Trigger: 1, Clear: 1},
				"nvme": original,
			},
		}
		alertconfig.NormalizeDiskFillByType(cfg)
		if _, exists := cfg.DiskFillByType["NVME"]; exists {
			t.Fatalf("uppercase NVME key should be removed, map=%+v", cfg.DiskFillByType)
		}
		got := cfg.DiskFillByType["nvme"]
		if !htEq(got, original) {
			t.Fatalf("nvme = %+v, want %+v (lowercased dup must not overwrite existing)", got, original)
		}
	})

	t.Run("mixed-case key lowercased into absent canonical slot is added with value preserved", func(t *testing.T) {
		custom := alertconfig.HysteresisThreshold{Trigger: 93, Clear: 88}
		cfg := &alertconfig.AlertConfig{
			DiskFillByType: map[string]alertconfig.HysteresisThreshold{
				"NVMe": custom,
			},
		}
		alertconfig.NormalizeDiskFillByType(cfg)
		if _, exists := cfg.DiskFillByType["NVMe"]; exists {
			t.Fatalf("mixed-case NVMe should be removed, map=%+v", cfg.DiskFillByType)
		}
		got, ok := cfg.DiskFillByType["nvme"]
		if !ok {
			t.Fatalf("lowercased nvme should be added, map=%+v", cfg.DiskFillByType)
		}
		if !htEq(got, custom) {
			t.Fatalf("nvme = %+v, want %+v (operator value preserved through rename)", got, custom)
		}
	})

	t.Run("positive trigger with non-positive clear resets to default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DiskFillByType: map[string]alertconfig.HysteresisThreshold{
				"sata": {Trigger: 88, Clear: 0},
				"hdd":  {Trigger: 80, Clear: -3},
			},
		}
		alertconfig.NormalizeDiskFillByType(cfg)
		if sata := cfg.DiskFillByType["sata"]; !htEq(sata, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("sata = %+v, want default {90 85}", sata)
		}
		if hdd := cfg.DiskFillByType["hdd"]; !htEq(hdd, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("hdd = %+v, want default {85 80}", hdd)
		}
	})

	t.Run("nil map seeds canonical defaults", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeDiskFillByType(cfg)
		want := map[string]alertconfig.HysteresisThreshold{
			"nvme": {Trigger: 92, Clear: 87},
			"sata": {Trigger: 90, Clear: 85},
			"hdd":  {Trigger: 85, Clear: 80},
		}
		if !reflect.DeepEqual(cfg.DiskFillByType, want) {
			t.Fatalf("DiskFillByType = %+v, want %+v", cfg.DiskFillByType, want)
		}
	})
}

// TestBranchCovNormalizeGeneralSettings covers the default-seeding branch and
// the preserved-positive branch for every field.
func TestBranchCovNormalizeGeneralSettings(t *testing.T) {
	t.Run("zero valued config is fully seeded", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeGeneralSettings(cfg)
		if cfg.MinimumDelta != 2.0 {
			t.Fatalf("MinimumDelta = %v, want 2.0", cfg.MinimumDelta)
		}
		if cfg.SuppressionWindow != 5 {
			t.Fatalf("SuppressionWindow = %d, want 5", cfg.SuppressionWindow)
		}
		if cfg.HysteresisMargin != 5.0 {
			t.Fatalf("HysteresisMargin = %v, want 5.0", cfg.HysteresisMargin)
		}
		if cfg.ObservationWindowHours != 24 {
			t.Fatalf("ObservationWindowHours = %d, want 24", cfg.ObservationWindowHours)
		}
		if cfg.FlappingWindowSeconds != 300 {
			t.Fatalf("FlappingWindowSeconds = %d, want 300", cfg.FlappingWindowSeconds)
		}
		if cfg.FlappingThreshold != 5 {
			t.Fatalf("FlappingThreshold = %d, want 5", cfg.FlappingThreshold)
		}
		if cfg.FlappingCooldownMinutes != 15 {
			t.Fatalf("FlappingCooldownMinutes = %d, want 15", cfg.FlappingCooldownMinutes)
		}
	})

	t.Run("explicit positive values preserved, negative replaced", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			MinimumDelta:            1.5,
			SuppressionWindow:       9,
			HysteresisMargin:        2.5,
			ObservationWindowHours:  48,
			FlappingWindowSeconds:   120,
			FlappingThreshold:       -1,
			FlappingCooldownMinutes: 0,
		}
		alertconfig.NormalizeGeneralSettings(cfg)
		if cfg.MinimumDelta != 1.5 {
			t.Fatalf("MinimumDelta = %v, want 1.5", cfg.MinimumDelta)
		}
		if cfg.SuppressionWindow != 9 {
			t.Fatalf("SuppressionWindow = %d, want 9", cfg.SuppressionWindow)
		}
		if cfg.HysteresisMargin != 2.5 {
			t.Fatalf("HysteresisMargin = %v, want 2.5", cfg.HysteresisMargin)
		}
		if cfg.ObservationWindowHours != 48 {
			t.Fatalf("ObservationWindowHours = %d, want 48", cfg.ObservationWindowHours)
		}
		if cfg.FlappingWindowSeconds != 120 {
			t.Fatalf("FlappingWindowSeconds = %d, want 120", cfg.FlappingWindowSeconds)
		}
		if cfg.FlappingThreshold != 5 {
			t.Fatalf("FlappingThreshold = %d, want 5 (negative replaced)", cfg.FlappingThreshold)
		}
		if cfg.FlappingCooldownMinutes != 15 {
			t.Fatalf("FlappingCooldownMinutes = %d, want 15 (zero replaced)", cfg.FlappingCooldownMinutes)
		}
	})
}

// expectedTimeThresholdKeys is the canonical set NormalizeTimeThresholds
// guarantees to populate (the "all" key is intentionally NOT auto-added).
var expectedTimeThresholdKeys = []string{
	"guest", "node", "storage", "pbs", "agent",
	"k8s-cluster", "k8s-node", "k8s-deployment", "k8s-namespace", "pod",
	"truenas-system", "truenas-pool", "truenas-dataset", "truenas-disk",
	"vmware-host", "vmware-vm", "vmware-datastore", "vmware-network",
}

// TestBranchCovNormalizeTimeThresholds covers the nil-init path, the
// preserve-existing path, the negative-delay repair path, and the special-cased
// "all" key (negative repaired, positive preserved, absent not inserted).
func TestBranchCovNormalizeTimeThresholds(t *testing.T) {
	t.Run("nil TimeThresholds is initialized with all default keys set to 5", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeTimeThresholds(cfg)
		if cfg.TimeThresholds == nil {
			t.Fatal("TimeThresholds should be initialized, got nil")
		}
		for _, k := range expectedTimeThresholdKeys {
			if v, ok := cfg.TimeThresholds[k]; !ok || v != 5 {
				t.Fatalf("TimeThresholds[%q] = %d ok=%v, want 5", k, v, ok)
			}
		}
		if _, ok := cfg.TimeThresholds["all"]; ok {
			t.Fatalf("\"all\" key should NOT be auto-added, map=%+v", cfg.TimeThresholds)
		}
	})

	t.Run("existing positive delays preserved and missing keys filled", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			TimeThresholds: map[string]int{"guest": 30},
		}
		alertconfig.NormalizeTimeThresholds(cfg)
		if cfg.TimeThresholds["guest"] != 30 {
			t.Fatalf("guest = %d, want 30 (preserved)", cfg.TimeThresholds["guest"])
		}
		if cfg.TimeThresholds["node"] != 5 {
			t.Fatalf("node = %d, want 5 (filled)", cfg.TimeThresholds["node"])
		}
	})

	t.Run("negative per-type delay repaired to 5", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			TimeThresholds: map[string]int{"storage": -12},
		}
		alertconfig.NormalizeTimeThresholds(cfg)
		if cfg.TimeThresholds["storage"] != 5 {
			t.Fatalf("storage = %d, want 5 (repaired)", cfg.TimeThresholds["storage"])
		}
	})

	t.Run("negative all delay repaired while positive all preserved", func(t *testing.T) {
		t.Run("negative all", func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{
				TimeThresholds: map[string]int{"all": -1},
			}
			alertconfig.NormalizeTimeThresholds(cfg)
			if cfg.TimeThresholds["all"] != 5 {
				t.Fatalf("all = %d, want 5 (repaired)", cfg.TimeThresholds["all"])
			}
		})
		t.Run("positive all", func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{
				TimeThresholds: map[string]int{"all": 100},
			}
			alertconfig.NormalizeTimeThresholds(cfg)
			if cfg.TimeThresholds["all"] != 100 {
				t.Fatalf("all = %d, want 100 (preserved)", cfg.TimeThresholds["all"])
			}
		})
	})

	t.Run("nil MetricTimeThresholds collapses to nil", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeTimeThresholds(cfg)
		if cfg.MetricTimeThresholds != nil {
			t.Fatalf("MetricTimeThresholds = %+v, want nil", cfg.MetricTimeThresholds)
		}
	})
}

// TestBranchCovValidateHysteresisThresholds covers the repair of an invalid
// value-typed threshold (StorageDefault), a pointer threshold, an unchanged
// valid config, and safe handling of nil pointers.
func TestBranchCovValidateHysteresisThresholds(t *testing.T) {
	t.Run("invalid value-typed StorageDefault is repaired in place", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			StorageDefault: alertconfig.HysteresisThreshold{Trigger: 85, Clear: 95},
		}
		alertconfig.ValidateHysteresisThresholds(cfg)
		if cfg.StorageDefault.Clear != 80 {
			t.Fatalf("StorageDefault.Clear = %v, want 80 (trigger-5)", cfg.StorageDefault.Clear)
		}
	})

	t.Run("invalid pointer threshold on guest defaults is repaired", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			GuestDefaults: alertconfig.ThresholdConfig{
				CPU: &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 90},
			},
		}
		alertconfig.ValidateHysteresisThresholds(cfg)
		if cfg.GuestDefaults.CPU == nil || cfg.GuestDefaults.CPU.Clear != 75 {
			t.Fatalf("GuestDefaults.CPU.Clear = %v, want 75", cfg.GuestDefaults.CPU)
		}
	})

	t.Run("already valid config is unchanged", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			StorageDefault: alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
			GuestDefaults: alertconfig.ThresholdConfig{
				Memory: &alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
			},
		}
		alertconfig.ValidateHysteresisThresholds(cfg)
		if cfg.StorageDefault.Clear != 80 {
			t.Fatalf("StorageDefault.Clear = %v, want 80 (untouched)", cfg.StorageDefault.Clear)
		}
		if cfg.GuestDefaults.Memory == nil || cfg.GuestDefaults.Memory.Clear != 80 {
			t.Fatalf("GuestDefaults.Memory.Clear = %v, want 80 (untouched)", cfg.GuestDefaults.Memory)
		}
	})

	t.Run("nil pointer thresholds do not panic", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.ValidateHysteresisThresholds(cfg)
		if cfg.GuestDefaults.CPU != nil {
			t.Fatalf("GuestDefaults.CPU = %v, want nil", cfg.GuestDefaults.CPU)
		}
	})
}

// TestBranchCovValidateQuietHoursTimezone covers all four combinations of the
// Enabled && Timezone != "" guard plus the error/success outcomes of
// time.LoadLocation.
func TestBranchCovValidateQuietHoursTimezone(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		timezone    string
		wantEnabled bool
	}{
		{
			name:        "enabled with invalid timezone gets disabled",
			enabled:     true,
			timezone:    "Nowhere/Invalid",
			wantEnabled: false,
		},
		{
			name:        "enabled with valid timezone stays enabled",
			enabled:     true,
			timezone:    "America/New_York",
			wantEnabled: true,
		},
		{
			name:        "enabled with empty timezone is a no-op (stays enabled)",
			enabled:     true,
			timezone:    "",
			wantEnabled: true,
		},
		{
			name:        "disabled with invalid timezone is a no-op (stays disabled)",
			enabled:     false,
			timezone:    "Nowhere/Invalid",
			wantEnabled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{}
			cfg.Schedule.QuietHours.Enabled = tc.enabled
			cfg.Schedule.QuietHours.Timezone = tc.timezone
			alertconfig.ValidateQuietHoursTimezone(cfg)
			if cfg.Schedule.QuietHours.Enabled != tc.wantEnabled {
				t.Fatalf("QuietHours.Enabled = %v, want %v", cfg.Schedule.QuietHours.Enabled, tc.wantEnabled)
			}
		})
	}
}

// TestBranchCovNormalizeDockerIgnoredPrefixes covers the empty/nil early
// returns, the all-whitespace collapse to nil, case-insensitive dedup that
// preserves the first-seen original casing, and surrounding-space trimming.
func TestBranchCovNormalizeDockerIgnoredPrefixes(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil input returns nil", in: nil, want: nil},
		{name: "empty input returns nil", in: []string{}, want: nil},
		{name: "all whitespace entries collapse to nil", in: []string{"  ", "", "\t"}, want: nil},
		{
			name: "case-insensitive dedup keeps first occurrence original casing",
			in:   []string{"Foo", "foo", "FOO"},
			want: []string{"Foo"},
		},
		{
			name: "leading and trailing spaces are trimmed but inner case preserved",
			in:   []string{"  MyPrefix ", "\tBar\t"},
			want: []string{"MyPrefix", "Bar"},
		},
		{
			name: "distinct prefixes all retained in first-seen order",
			in:   []string{"alpha", "Beta", "ALPHA"},
			want: []string{"alpha", "Beta"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.NormalizeDockerIgnoredPrefixes(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got=%v want=%v)", len(got), len(tc.want), got, tc.want)
			}
			// Normalize nil vs empty []string{} for comparison after length check.
			if len(got) == 0 {
				if got != nil {
					t.Fatalf("expected nil slice for empty result, got %v", got)
				}
				return
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("result[%d] = %q, want %q (got=%v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}
