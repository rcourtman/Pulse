package config_test

import (
	"reflect"
	"testing"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// This file adds branch-coverage tests for the per-subsystem Normalize*Defaults
// helpers in internal/alerts/config/normalize.go (NormalizePMGDefaults,
// NormalizePBSDefaults, NormalizeSnapshotDefaults, NormalizeBackupDefaults,
// NormalizeNodeDefaults, NormalizeAgentDefaults, NormalizeKubernetesDefaults,
// NormalizeTrueNASDefaults, NormalizeVMwareDefaults, NormalizeDiskTempByType,
// NormalizeMetricTimeThresholds) plus the unexported normalizeThresholdPointer.
//
// The unexported normalizeThresholdPointer is exercised indirectly through its
// exported callers (NormalizeKubernetesDefaults / NormalizeTrueNASDefaults /
// NormalizeVMwareDefaults) because the sibling test files use the external
// `package config_test` and so cannot reach unexported symbols directly.
// Test names use the BranchCov prefix so `-run BranchCov` selects them.
//
// floatEq / htEq are reused from alerts_config_normalize_branchcov0716_test.go.

// ptrHtEq dereferences a *HysteresisThreshold and compares it by value.
func ptrHtEq(got *alertconfig.HysteresisThreshold, want alertconfig.HysteresisThreshold) bool {
	if got == nil {
		return false
	}
	return htEq(*got, want)
}

// expectedPMGDefaults returns the canonical PMGThresholdConfig the normalizer
// must seed whenever a field is non-positive.
func expectedPMGDefaults() alertconfig.PMGThresholdConfig {
	return alertconfig.PMGThresholdConfig{
		QueueTotalWarning:       500,
		QueueTotalCritical:      1000,
		OldestMessageWarnMins:   30,
		OldestMessageCritMins:   60,
		DeferredQueueWarn:       200,
		DeferredQueueCritical:   500,
		HoldQueueWarn:           100,
		HoldQueueCritical:       300,
		QuarantineSpamWarn:      2000,
		QuarantineSpamCritical:  5000,
		QuarantineVirusWarn:     2000,
		QuarantineVirusCritical: 5000,
		QuarantineGrowthWarnPct: 25,
		QuarantineGrowthWarnMin: 250,
		QuarantineGrowthCritPct: 50,
		QuarantineGrowthCritMin: 500,
	}
}

// TestBranchCovNormalizePMGDefaults covers the <=0 default-seeding branch and
// the >0 preserve branch for every field of PMGThresholdConfig.
func TestBranchCovNormalizePMGDefaults(t *testing.T) {
	t.Run("zero-valued config seeds every field with its canonical default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizePMGDefaults(cfg)
		if !reflect.DeepEqual(cfg.PMGDefaults, expectedPMGDefaults()) {
			t.Fatalf("PMGDefaults = %+v, want defaults %+v", cfg.PMGDefaults, expectedPMGDefaults())
		}
	})

	t.Run("negative values are replaced with defaults (<=0 branch)", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			PMGDefaults: alertconfig.PMGThresholdConfig{
				QueueTotalWarning:       -1,
				QueueTotalCritical:      -2,
				OldestMessageWarnMins:   -3,
				OldestMessageCritMins:   -4,
				DeferredQueueWarn:       -5,
				DeferredQueueCritical:   -6,
				HoldQueueWarn:           -7,
				HoldQueueCritical:       -8,
				QuarantineSpamWarn:      -9,
				QuarantineSpamCritical:  -10,
				QuarantineVirusWarn:     -11,
				QuarantineVirusCritical: -12,
				QuarantineGrowthWarnPct: -13,
				QuarantineGrowthWarnMin: -14,
				QuarantineGrowthCritPct: -15,
				QuarantineGrowthCritMin: -16,
			},
		}
		alertconfig.NormalizePMGDefaults(cfg)
		if !reflect.DeepEqual(cfg.PMGDefaults, expectedPMGDefaults()) {
			t.Fatalf("PMGDefaults = %+v, want defaults (negatives replaced)", cfg.PMGDefaults)
		}
	})

	t.Run("positive operator values are preserved (>0 branch)", func(t *testing.T) {
		want := alertconfig.PMGThresholdConfig{
			QueueTotalWarning:       111,
			QueueTotalCritical:      222,
			OldestMessageWarnMins:   333,
			OldestMessageCritMins:   444,
			DeferredQueueWarn:       555,
			DeferredQueueCritical:   666,
			HoldQueueWarn:           777,
			HoldQueueCritical:       888,
			QuarantineSpamWarn:      999,
			QuarantineSpamCritical:  1001,
			QuarantineVirusWarn:     1002,
			QuarantineVirusCritical: 1003,
			QuarantineGrowthWarnPct: 12,
			QuarantineGrowthWarnMin: 123,
			QuarantineGrowthCritPct: 34,
			QuarantineGrowthCritMin: 345,
		}
		cfg := &alertconfig.AlertConfig{PMGDefaults: want}
		alertconfig.NormalizePMGDefaults(cfg)
		if !reflect.DeepEqual(cfg.PMGDefaults, want) {
			t.Fatalf("PMGDefaults = %+v, want %+v (preserved)", cfg.PMGDefaults, want)
		}
	})

	t.Run("mixed zero/negative/positive fields only seed the non-positive ones", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			PMGDefaults: alertconfig.PMGThresholdConfig{
				QueueTotalWarning:     0,    // -> 500
				QueueTotalCritical:    7777, // preserved
				OldestMessageWarnMins: -5,   // -> 30
				HoldQueueWarn:         42,   // preserved
			},
		}
		alertconfig.NormalizePMGDefaults(cfg)
		want := expectedPMGDefaults()
		want.QueueTotalCritical = 7777
		want.HoldQueueWarn = 42
		if !reflect.DeepEqual(cfg.PMGDefaults, want) {
			t.Fatalf("PMGDefaults = %+v, want %+v", cfg.PMGDefaults, want)
		}
	})
}

// TestBranchCovNormalizePBSDefaults covers each branch of the CPU/Memory ladder:
// nil/negative -> default pair, zero trigger -> clear 0, clear<=0 -> derive with a
// FIXED fallback, plus the valid no-op. It also pins the small-trigger anomaly.
func TestBranchCovNormalizePBSDefaults(t *testing.T) {
	t.Run("nil thresholds are seeded with full default pairs", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizePBSDefaults(cfg)
		if !ptrHtEq(cfg.PBSDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want {80 75}", cfg.PBSDefaults.CPU)
		}
		if !ptrHtEq(cfg.PBSDefaults.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want {85 80}", cfg.PBSDefaults.Memory)
		}
	})

	// CPU defaults to {80,75}; Memory to {85,80}. Same input shape applied to
	// both to cover every branch of the shared ladder.
	tests := []struct {
		name       string
		in         alertconfig.HysteresisThreshold
		wantCPU    alertconfig.HysteresisThreshold
		wantMemory alertconfig.HysteresisThreshold
	}{
		{
			name:       "negative trigger replaced with default pair",
			in:         alertconfig.HysteresisThreshold{Trigger: -1, Clear: 99},
			wantCPU:    alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
			wantMemory: alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
		},
		{
			name:       "zero trigger forces clear to zero",
			in:         alertconfig.HysteresisThreshold{Trigger: 0, Clear: 50},
			wantCPU:    alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
			wantMemory: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		{
			name:       "positive trigger with zero clear derives clear as trigger-5",
			in:         alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
			wantCPU:    alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
			wantMemory: alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		{
			// SUSPECTED SOURCE BUG: derived clear (1-5=-4) is <=0 so the code
			// falls back to the FIXED default (75 CPU / 80 Memory), producing
			// Clear >> Trigger. Unlike normalizeThresholdPointer (which clamps
			// to 0) and with no EnsureValidHysteresis call, this stays invalid.
			name:       "small trigger with zero clear falls back to fixed default yielding clear>trigger",
			in:         alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0},
			wantCPU:    alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75},
			wantMemory: alertconfig.HysteresisThreshold{Trigger: 1, Clear: 80},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cpu := tc.in
			mem := tc.in
			cfg := &alertconfig.AlertConfig{
				PBSDefaults: alertconfig.ThresholdConfig{
					CPU:    &cpu,
					Memory: &mem,
				},
			}
			alertconfig.NormalizePBSDefaults(cfg)
			if !ptrHtEq(cfg.PBSDefaults.CPU, tc.wantCPU) {
				t.Fatalf("CPU = %+v, want %+v", cfg.PBSDefaults.CPU, tc.wantCPU)
			}
			if !ptrHtEq(cfg.PBSDefaults.Memory, tc.wantMemory) {
				t.Fatalf("Memory = %+v, want %+v", cfg.PBSDefaults.Memory, tc.wantMemory)
			}
		})
	}

	t.Run("already valid pairs are left untouched", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			PBSDefaults: alertconfig.ThresholdConfig{
				CPU:    &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory: &alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
			},
		}
		alertconfig.NormalizePBSDefaults(cfg)
		if !ptrHtEq(cfg.PBSDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want unchanged", cfg.PBSDefaults.CPU)
		}
		if !ptrHtEq(cfg.PBSDefaults.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want unchanged", cfg.PBSDefaults.Memory)
		}
	})
}

// TestBranchCovNormalizeSnapshotDefaults covers the day and size clamps: negative
// to zero, warning>critical clamp-down, zero-critical promotion, and valid no-ops.
func TestBranchCovNormalizeSnapshotDefaults(t *testing.T) {
	tests := []struct {
		name            string
		warningDays     int
		criticalDays    int
		warningSizeGiB  float64
		criticalSizeGiB float64
		wantWarnDays    int
		wantCritDays    int
		wantWarnSize    float64
		wantCritSize    float64
	}{
		{
			name:         "both days negative clamped to zero",
			warningDays:  -3,
			criticalDays: -7,
			wantWarnDays: 0, wantCritDays: 0,
		},
		{
			name:         "warning days above positive critical clamped down to critical",
			warningDays:  10,
			criticalDays: 5,
			wantWarnDays: 5, wantCritDays: 5,
		},
		{
			name:         "zero critical with positive warning promotes critical to warning",
			warningDays:  7,
			criticalDays: 0,
			wantWarnDays: 7, wantCritDays: 7,
		},
		{
			name:         "valid days pair untouched",
			warningDays:  3,
			criticalDays: 7,
			wantWarnDays: 3, wantCritDays: 7,
		},
		{
			name:         "zero warning with positive critical untouched",
			warningDays:  0,
			criticalDays: 7,
			wantWarnDays: 0, wantCritDays: 7,
		},
		{
			name:            "both sizes negative clamped to zero",
			warningSizeGiB:  -2.5,
			criticalSizeGiB: -9,
			wantWarnSize:    0, wantCritSize: 0,
		},
		{
			name:            "warning size above positive critical clamped down",
			warningSizeGiB:  100,
			criticalSizeGiB: 50,
			wantWarnSize:    50, wantCritSize: 50,
		},
		{
			name:            "zero critical size with positive warning promotes critical",
			warningSizeGiB:  20,
			criticalSizeGiB: 0,
			wantWarnSize:    20, wantCritSize: 20,
		},
		{
			name:            "valid size pair untouched",
			warningSizeGiB:  10,
			criticalSizeGiB: 30,
			wantWarnSize:    10, wantCritSize: 30,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{
				SnapshotDefaults: alertconfig.SnapshotAlertConfig{
					WarningDays:     tc.warningDays,
					CriticalDays:    tc.criticalDays,
					WarningSizeGiB:  tc.warningSizeGiB,
					CriticalSizeGiB: tc.criticalSizeGiB,
				},
			}
			alertconfig.NormalizeSnapshotDefaults(cfg)
			s := cfg.SnapshotDefaults
			if s.WarningDays != tc.wantWarnDays {
				t.Errorf("WarningDays = %d, want %d", s.WarningDays, tc.wantWarnDays)
			}
			if s.CriticalDays != tc.wantCritDays {
				t.Errorf("CriticalDays = %d, want %d", s.CriticalDays, tc.wantCritDays)
			}
			if !floatEq(s.WarningSizeGiB, tc.wantWarnSize) {
				t.Errorf("WarningSizeGiB = %v, want %v", s.WarningSizeGiB, tc.wantWarnSize)
			}
			if !floatEq(s.CriticalSizeGiB, tc.wantCritSize) {
				t.Errorf("CriticalSizeGiB = %v, want %v", s.CriticalSizeGiB, tc.wantCritSize)
			}
		})
	}
}

// TestBranchCovNormalizeBackupDefaults covers day clamps, FreshHours/StaleHours
// seeding, the StaleHours<FreshHours repair, the AlertOrphaned nil-default and
// preserve branches, and IgnoreVMIDs trim/dedupe/drop-empty behavior.
func TestBranchCovNormalizeBackupDefaults(t *testing.T) {
	t.Run("zero config seeds FreshHours/StaleHours and AlertOrphaned default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeBackupDefaults(cfg)
		b := cfg.BackupDefaults
		if b.FreshHours != 24 {
			t.Fatalf("FreshHours = %d, want 24", b.FreshHours)
		}
		if b.StaleHours != 72 {
			t.Fatalf("StaleHours = %d, want 72", b.StaleHours)
		}
		if b.AlertOrphaned == nil || *b.AlertOrphaned != true {
			t.Fatalf("AlertOrphaned = %v, want *true", b.AlertOrphaned)
		}
		if b.WarningDays != 0 || b.CriticalDays != 0 {
			t.Fatalf("days = %d/%d, want 0/0", b.WarningDays, b.CriticalDays)
		}
	})

	t.Run("warning days above positive critical clamped down to critical", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			BackupDefaults: alertconfig.BackupAlertConfig{
				WarningDays:  8,
				CriticalDays: 3,
			},
		}
		alertconfig.NormalizeBackupDefaults(cfg)
		if cfg.BackupDefaults.WarningDays != 3 || cfg.BackupDefaults.CriticalDays != 3 {
			t.Fatalf("days = %d/%d, want 3/3", cfg.BackupDefaults.WarningDays, cfg.BackupDefaults.CriticalDays)
		}
	})

	t.Run("negative days clamped to zero", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			BackupDefaults: alertconfig.BackupAlertConfig{
				WarningDays:  -4,
				CriticalDays: -9,
			},
		}
		alertconfig.NormalizeBackupDefaults(cfg)
		if cfg.BackupDefaults.WarningDays != 0 || cfg.BackupDefaults.CriticalDays != 0 {
			t.Fatalf("days = %d/%d, want 0/0", cfg.BackupDefaults.WarningDays, cfg.BackupDefaults.CriticalDays)
		}
	})

	t.Run("stale hours below fresh hours raised to fresh hours", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			BackupDefaults: alertconfig.BackupAlertConfig{
				FreshHours: 100,
				StaleHours: 50,
			},
		}
		alertconfig.NormalizeBackupDefaults(cfg)
		if cfg.BackupDefaults.FreshHours != 100 {
			t.Fatalf("FreshHours = %d, want 100", cfg.BackupDefaults.FreshHours)
		}
		if cfg.BackupDefaults.StaleHours != 100 {
			t.Fatalf("StaleHours = %d, want 100 (raised to FreshHours)", cfg.BackupDefaults.StaleHours)
		}
	})

	t.Run("explicit AlertOrphaned true and false are preserved", func(t *testing.T) {
		tr := true
		cfgT := &alertconfig.AlertConfig{BackupDefaults: alertconfig.BackupAlertConfig{AlertOrphaned: &tr}}
		alertconfig.NormalizeBackupDefaults(cfgT)
		if cfgT.BackupDefaults.AlertOrphaned == nil || *cfgT.BackupDefaults.AlertOrphaned != true {
			t.Fatalf("AlertOrphaned = %v, want *true preserved", cfgT.BackupDefaults.AlertOrphaned)
		}

		fl := false
		cfgF := &alertconfig.AlertConfig{BackupDefaults: alertconfig.BackupAlertConfig{AlertOrphaned: &fl}}
		alertconfig.NormalizeBackupDefaults(cfgF)
		if cfgF.BackupDefaults.AlertOrphaned == nil || *cfgF.BackupDefaults.AlertOrphaned != false {
			t.Fatalf("AlertOrphaned = %v, want *false preserved", cfgF.BackupDefaults.AlertOrphaned)
		}
	})

	t.Run("IgnoreVMIDs trimmed, empties dropped, deduplicated (case-sensitive)", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			BackupDefaults: alertconfig.BackupAlertConfig{
				IgnoreVMIDs: []string{"  100 ", "100", "200", "", "  100\t", "\t"},
			},
		}
		alertconfig.NormalizeBackupDefaults(cfg)
		want := []string{"100", "200"}
		if !reflect.DeepEqual(cfg.BackupDefaults.IgnoreVMIDs, want) {
			t.Fatalf("IgnoreVMIDs = %#v, want %#v", cfg.BackupDefaults.IgnoreVMIDs, want)
		}
	})

	t.Run("empty IgnoreVMIDs slice stays empty (guarded by len>0)", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeBackupDefaults(cfg)
		if len(cfg.BackupDefaults.IgnoreVMIDs) != 0 {
			t.Fatalf("IgnoreVMIDs = %#v, want empty", cfg.BackupDefaults.IgnoreVMIDs)
		}
	})

	t.Run("IgnoreVMIDs with only empties collapses to empty slice", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			BackupDefaults: alertconfig.BackupAlertConfig{
				IgnoreVMIDs: []string{"   ", "", "\t"},
			},
		}
		alertconfig.NormalizeBackupDefaults(cfg)
		if len(cfg.BackupDefaults.IgnoreVMIDs) != 0 {
			t.Fatalf("IgnoreVMIDs = %#v, want empty", cfg.BackupDefaults.IgnoreVMIDs)
		}
	})
}

// TestBranchCovNormalizeNodeDefaults covers the Temperature pointer ladder:
// nil/negative -> default, zero trigger -> clear 0, clear<=0 -> derive with fixed
// fallback, valid no-op, and the small-trigger clear>trigger anomaly.
func TestBranchCovNormalizeNodeDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   *alertconfig.HysteresisThreshold
		want alertconfig.HysteresisThreshold
	}{
		{
			name: "nil temperature seeded with default",
			in:   nil,
			want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		{
			name: "negative trigger replaced with default",
			in:   &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 99},
			want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		{
			name: "zero trigger forces clear to zero",
			in:   &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 50},
			want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
		},
		{
			name: "positive trigger zero clear derives clear",
			in:   &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
			want: alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		{
			// SUSPECTED SOURCE BUG: derived clear (1-5=-4) <=0 falls back to the
			// FIXED 75, giving Clear(75) > Trigger(1); NormalizeNodeDefaults has
			// no EnsureValidHysteresis call so this stays invalid.
			name: "small trigger zero clear falls back to fixed 75 (clear>trigger)",
			in:   &alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0},
			want: alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75},
		},
		{
			name: "valid pair untouched",
			in:   &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
			want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &alertconfig.AlertConfig{
				NodeDefaults: alertconfig.ThresholdConfig{Temperature: tc.in},
			}
			alertconfig.NormalizeNodeDefaults(cfg)
			if !ptrHtEq(cfg.NodeDefaults.Temperature, tc.want) {
				t.Fatalf("Temperature = %+v, want %+v", cfg.NodeDefaults.Temperature, tc.want)
			}
		})
	}
}

// TestBranchCovNormalizeAgentDefaults covers CPU/Memory/Disk/DiskTemperature
// seeding, negative/zero-trigger branches, clear derivation, the
// EnsureValidHysteresis safety net on DiskTemperature (incl. the small-trigger
// fallback repaired back to 0), and the side-effect seeding of both disk maps.
func TestBranchCovNormalizeAgentDefaults(t *testing.T) {
	t.Run("nil thresholds seeded and both disk type maps populated", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeAgentDefaults(cfg)
		a := cfg.AgentDefaults
		if !ptrHtEq(a.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want {80 75}", a.CPU)
		}
		if !ptrHtEq(a.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want {85 80}", a.Memory)
		}
		if !ptrHtEq(a.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("Disk = %+v, want {90 85}", a.Disk)
		}
		if !ptrHtEq(a.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}) {
			t.Fatalf("DiskTemperature = %+v, want {55 50}", a.DiskTemperature)
		}
		if len(cfg.DiskFillByType) != 3 {
			t.Fatalf("DiskFillByType len = %d, want 3", len(cfg.DiskFillByType))
		}
		if len(cfg.DiskTempByType) != 3 {
			t.Fatalf("DiskTempByType len = %d, want 3", len(cfg.DiskTempByType))
		}
	})

	t.Run("negative triggers replaced with defaults", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				CPU:             &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
				Memory:          &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
				Disk:            &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 0},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		a := cfg.AgentDefaults
		if !ptrHtEq(a.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want {80 75}", a.CPU)
		}
		if !ptrHtEq(a.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want {85 80}", a.Memory)
		}
		if !ptrHtEq(a.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("Disk = %+v, want {90 85}", a.Disk)
		}
		if !ptrHtEq(a.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}) {
			t.Fatalf("DiskTemperature = %+v, want {55 50}", a.DiskTemperature)
		}
	})

	t.Run("zero trigger forces clear to zero for every threshold", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				CPU:             &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 99},
				Memory:          &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 99},
				Disk:            &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 99},
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 99},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		zero := alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}
		a := cfg.AgentDefaults
		if !ptrHtEq(a.CPU, zero) || !ptrHtEq(a.Memory, zero) || !ptrHtEq(a.Disk, zero) || !ptrHtEq(a.DiskTemperature, zero) {
			t.Fatalf("expected all zero, got CPU=%+v Mem=%+v Disk=%+v DiskTemp=%+v", a.CPU, a.Memory, a.Disk, a.DiskTemperature)
		}
	})

	t.Run("positive trigger with non-positive clear derives clear for each threshold", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				CPU:             &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				Memory:          &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				Disk:            &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 60, Clear: 0},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		a := cfg.AgentDefaults
		if !ptrHtEq(a.CPU, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("CPU = %+v, want {90 85}", a.CPU)
		}
		if !ptrHtEq(a.Memory, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("Memory = %+v, want {90 85}", a.Memory)
		}
		if !ptrHtEq(a.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("Disk = %+v, want {90 85}", a.Disk)
		}
		if !ptrHtEq(a.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 60, Clear: 55}) {
			t.Fatalf("DiskTemperature = %+v, want {60 55}", a.DiskTemperature)
		}
	})

	// SUSPECTED SOURCE BUG (CPU arm): derived clear (1-5=-4) <=0 falls back to
	// the FIXED 75; CPU has NO EnsureValidHysteresis call, so Clear(75) >
	// Trigger(1) persists. DiskTemperature (which DOES call EnsureValidHysteresis)
	// is repaired - covered by the next two subtests.
	t.Run("CPU small trigger zero clear yields clear>trigger (no EnsureValidHysteresis net)", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				CPU: &alertconfig.HysteresisThreshold{Trigger: 1, Clear: 0},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		if !ptrHtEq(cfg.AgentDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 1, Clear: 75}) {
			t.Fatalf("CPU = %+v, want {1 75} (clear>trigger anomaly)", cfg.AgentDefaults.CPU)
		}
	})

	t.Run("DiskTemperature invalid clear>=trigger repaired by EnsureValidHysteresis", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 55, Clear: 70},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		// Clear(70) > 0 so derive is skipped; EnsureValidHysteresis repairs 70>=55 -> 50.
		if !ptrHtEq(cfg.AgentDefaults.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}) {
			t.Fatalf("DiskTemperature = %+v, want {55 50}", cfg.AgentDefaults.DiskTemperature)
		}
	})

	t.Run("DiskTemperature small trigger: fallback to 50 then repaired to 0 by EnsureValidHysteresis", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 2, Clear: 0},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		// derived clear 2-5=-3 <=0 -> fixed fallback 50; EnsureValidHysteresis
		// then sees 50>=2 and sets clear=2-5=-3<0 -> 0. End state {2 0}.
		if !ptrHtEq(cfg.AgentDefaults.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 2, Clear: 0}) {
			t.Fatalf("DiskTemperature = %+v, want {2 0} (fallback 50 repaired to 0)", cfg.AgentDefaults.DiskTemperature)
		}
	})

	t.Run("valid thresholds preserved end to end", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			AgentDefaults: alertconfig.ThresholdConfig{
				CPU:             &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:          &alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:            &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
				DiskTemperature: &alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50},
			},
		}
		alertconfig.NormalizeAgentDefaults(cfg)
		a := cfg.AgentDefaults
		if !ptrHtEq(a.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}) {
			t.Fatalf("CPU = %+v, want unchanged", a.CPU)
		}
		if !ptrHtEq(a.DiskTemperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}) {
			t.Fatalf("DiskTemperature = %+v, want unchanged", a.DiskTemperature)
		}
	})
}

// TestBranchCovNormalizeKubernetesDefaults exercises normalizeThresholdPointer's
// full branch ladder through CPU (defaults 80/75) and DiskRead (defaults 0/0),
// and confirms every field becomes a non-nil pointer after a zero config.
func TestBranchCovNormalizeKubernetesDefaults(t *testing.T) {
	t.Run("all nil: CPU/Memory/Disk seeded, IO and network fields are non-nil {0,0}", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeKubernetesDefaults(cfg)
		k := cfg.KubernetesDefaults
		for _, f := range []struct {
			name string
			got  *alertconfig.HysteresisThreshold
			want alertconfig.HysteresisThreshold
		}{
			{"CPU", k.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}},
			{"Memory", k.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"Disk", k.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}},
			{"DiskRead", k.DiskRead, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"DiskWrite", k.DiskWrite, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkIn", k.NetworkIn, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkOut", k.NetworkOut, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
		} {
			if f.got == nil {
				t.Fatalf("%s is nil, want non-nil", f.name)
			}
			if !htEq(*f.got, f.want) {
				t.Fatalf("%s = %+v, want %+v", f.name, *f.got, f.want)
			}
		}
	})

	// normalizeThresholdPointer branch ladder via CPU (defaultTrigger 80, defaultClear 75).
	t.Run("CPU branch matrix (defaults 80/75)", func(t *testing.T) {
		tests := []struct {
			name string
			in   *alertconfig.HysteresisThreshold
			want alertconfig.HysteresisThreshold
		}{
			{
				name: "negative trigger returns default pair",
				in:   &alertconfig.HysteresisThreshold{Trigger: -1, Clear: 99},
				want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
			},
			{
				name: "zero trigger zeroes clear and returns a copy",
				in:   &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 50},
				want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
			},
			{
				name: "positive trigger zero clear derives clear",
				in:   &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				want: alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85},
			},
			{
				// Contrast with PBS/Node/Agent: derived clear clamps to 0 here,
				// NOT a fixed fallback default, so no clear>trigger anomaly.
				name: "derived clear clamps to zero (not fixed fallback) when trigger<5",
				in:   &alertconfig.HysteresisThreshold{Trigger: 3, Clear: 0},
				want: alertconfig.HysteresisThreshold{Trigger: 3, Clear: 0},
			},
			{
				name: "clear above trigger repaired by EnsureValidHysteresis",
				in:   &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 90},
				want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
			},
			{
				name: "valid pair untouched",
				in:   &alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
				want: alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				in := *tc.in
				cfg := &alertconfig.AlertConfig{
					KubernetesDefaults: alertconfig.ThresholdConfig{CPU: &in},
				}
				alertconfig.NormalizeKubernetesDefaults(cfg)
				if !ptrHtEq(cfg.KubernetesDefaults.CPU, tc.want) {
					t.Fatalf("CPU = %+v, want %+v", cfg.KubernetesDefaults.CPU, tc.want)
				}
			})
		}
	})

	// DiskRead uses defaults (0,0): nil yields a non-nil {0,0} (disabled), but an
	// operator-provided positive trigger enables the metric and derives a clear.
	t.Run("DiskRead branch matrix (defaults 0/0)", func(t *testing.T) {
		tests := []struct {
			name string
			in   *alertconfig.HysteresisThreshold
			want alertconfig.HysteresisThreshold
		}{
			{
				name: "nil returns non-nil {0,0} (disabled)",
				in:   nil,
				want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
			},
			{
				name: "negative trigger returns {0,0}",
				in:   &alertconfig.HysteresisThreshold{Trigger: -5, Clear: 99},
				want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
			},
			{
				name: "zero trigger zeroes clear",
				in:   &alertconfig.HysteresisThreshold{Trigger: 0, Clear: 99},
				want: alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0},
			},
			{
				name: "positive trigger enables metric and derives clear",
				in:   &alertconfig.HysteresisThreshold{Trigger: 50, Clear: 0},
				want: alertconfig.HysteresisThreshold{Trigger: 50, Clear: 45},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cfg := &alertconfig.AlertConfig{
					KubernetesDefaults: alertconfig.ThresholdConfig{DiskRead: tc.in},
				}
				alertconfig.NormalizeKubernetesDefaults(cfg)
				if cfg.KubernetesDefaults.DiskRead == nil {
					t.Fatalf("DiskRead nil, want non-nil")
				}
				if !htEq(*cfg.KubernetesDefaults.DiskRead, tc.want) {
					t.Fatalf("DiskRead = %+v, want %+v", *cfg.KubernetesDefaults.DiskRead, tc.want)
				}
			})
		}
	})
}

// TestBranchCovNormalizeTrueNASDefaults covers the full TrueNAS seed matrix,
// operator-enabled IO metrics, and the EnsureValidHysteresis repair path.
func TestBranchCovNormalizeTrueNASDefaults(t *testing.T) {
	t.Run("nil config seeds every threshold with its subsystem default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeTrueNASDefaults(cfg)
		t1 := cfg.TrueNASDefaults
		for _, f := range []struct {
			name string
			got  *alertconfig.HysteresisThreshold
			want alertconfig.HysteresisThreshold
		}{
			{"CPU", t1.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}},
			{"Memory", t1.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"Disk", t1.Disk, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"Usage", t1.Usage, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"Temperature", t1.Temperature, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}},
			{"DiskRead", t1.DiskRead, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"DiskWrite", t1.DiskWrite, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkIn", t1.NetworkIn, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkOut", t1.NetworkOut, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"Disk.Temperature", cfg.TrueNASDiskDefaults.Temperature, alertconfig.HysteresisThreshold{Trigger: 55, Clear: 50}},
		} {
			if f.got == nil {
				t.Fatalf("%s is nil, want non-nil", f.name)
			}
			if !htEq(*f.got, f.want) {
				t.Fatalf("%s = %+v, want %+v", f.name, *f.got, f.want)
			}
		}
	})

	t.Run("derivation and operator-enabled IO metric via CPU/DiskRead", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			TrueNASDefaults: alertconfig.ThresholdConfig{
				CPU:      &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				DiskRead: &alertconfig.HysteresisThreshold{Trigger: 40, Clear: 0},
			},
		}
		alertconfig.NormalizeTrueNASDefaults(cfg)
		if !ptrHtEq(cfg.TrueNASDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("CPU = %+v, want {90 85}", cfg.TrueNASDefaults.CPU)
		}
		if !ptrHtEq(cfg.TrueNASDefaults.DiskRead, alertconfig.HysteresisThreshold{Trigger: 40, Clear: 35}) {
			t.Fatalf("DiskRead = %+v, want {40 35}", cfg.TrueNASDefaults.DiskRead)
		}
	})

	t.Run("EnsureValidHysteresis repairs clear>=trigger", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			TrueNASDefaults: alertconfig.ThresholdConfig{
				Memory: &alertconfig.HysteresisThreshold{Trigger: 85, Clear: 95},
			},
		}
		alertconfig.NormalizeTrueNASDefaults(cfg)
		if !ptrHtEq(cfg.TrueNASDefaults.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}) {
			t.Fatalf("Memory = %+v, want {85 80}", cfg.TrueNASDefaults.Memory)
		}
	})
}

// TestBranchCovNormalizeVMwareDefaults covers the full VMware seed matrix plus
// derivation, an operator-enabled network metric, and EnsureValidHysteresis repair.
func TestBranchCovNormalizeVMwareDefaults(t *testing.T) {
	t.Run("nil config seeds every threshold with its subsystem default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeVMwareDefaults(cfg)
		v := cfg.VMwareDefaults
		for _, f := range []struct {
			name string
			got  *alertconfig.HysteresisThreshold
			want alertconfig.HysteresisThreshold
		}{
			{"CPU", v.CPU, alertconfig.HysteresisThreshold{Trigger: 80, Clear: 75}},
			{"Memory", v.Memory, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"Disk", v.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}},
			{"Usage", v.Usage, alertconfig.HysteresisThreshold{Trigger: 85, Clear: 80}},
			{"DiskRead", v.DiskRead, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"DiskWrite", v.DiskWrite, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkIn", v.NetworkIn, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
			{"NetworkOut", v.NetworkOut, alertconfig.HysteresisThreshold{Trigger: 0, Clear: 0}},
		} {
			if f.got == nil {
				t.Fatalf("%s is nil, want non-nil", f.name)
			}
			if !htEq(*f.got, f.want) {
				t.Fatalf("%s = %+v, want %+v", f.name, *f.got, f.want)
			}
		}
	})

	t.Run("derivation, operator-enabled network metric, and EnsureValidHysteresis repair", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			VMwareDefaults: alertconfig.ThresholdConfig{
				CPU:        &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 0},
				Disk:       &alertconfig.HysteresisThreshold{Trigger: 90, Clear: 99},
				NetworkOut: &alertconfig.HysteresisThreshold{Trigger: 30, Clear: 0},
			},
		}
		alertconfig.NormalizeVMwareDefaults(cfg)
		if !ptrHtEq(cfg.VMwareDefaults.CPU, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("CPU = %+v, want {90 85}", cfg.VMwareDefaults.CPU)
		}
		// Disk clear(99)>=trigger(90) repaired by EnsureValidHysteresis -> 85.
		if !ptrHtEq(cfg.VMwareDefaults.Disk, alertconfig.HysteresisThreshold{Trigger: 90, Clear: 85}) {
			t.Fatalf("Disk = %+v, want {90 85} (repaired)", cfg.VMwareDefaults.Disk)
		}
		// NetworkOut (0,0 default) operator-enabled: 30 -> clear 25.
		if !ptrHtEq(cfg.VMwareDefaults.NetworkOut, alertconfig.HysteresisThreshold{Trigger: 30, Clear: 25}) {
			t.Fatalf("NetworkOut = %+v, want {30 25}", cfg.VMwareDefaults.NetworkOut)
		}
	})
}

// TestBranchCovNormalizeDiskTempByType covers nil seeding, whitespace-key drop,
// canonical backfill, key lowercasing (incl. non-overwrite of existing canonical
// and value-preserving rename into an absent slot), and the non-positive reset.
func TestBranchCovNormalizeDiskTempByType(t *testing.T) {
	wantDefaults := map[string]alertconfig.HysteresisThreshold{
		"nvme": {Trigger: 70, Clear: 65},
		"sas":  {Trigger: 65, Clear: 60},
		"sata": {Trigger: 55, Clear: 50},
	}

	t.Run("nil map seeds canonical nvme/sas/sata defaults", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{}
		alertconfig.NormalizeDiskTempByType(cfg)
		if !reflect.DeepEqual(cfg.DiskTempByType, wantDefaults) {
			t.Fatalf("DiskTempByType = %+v, want %+v", cfg.DiskTempByType, wantDefaults)
		}
	})

	t.Run("whitespace-only key is dropped and canonical keys backfilled", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DiskTempByType: map[string]alertconfig.HysteresisThreshold{
				"   ": {Trigger: 99, Clear: 99},
			},
		}
		alertconfig.NormalizeDiskTempByType(cfg)
		if _, ok := cfg.DiskTempByType["   "]; ok {
			t.Fatalf("whitespace key should be removed, map=%+v", cfg.DiskTempByType)
		}
		if _, ok := cfg.DiskTempByType[""]; ok {
			t.Fatalf("empty key should not be added, map=%+v", cfg.DiskTempByType)
		}
		for _, k := range []string{"nvme", "sas", "sata"} {
			if _, ok := cfg.DiskTempByType[k]; !ok {
				t.Fatalf("canonical key %q missing, map=%+v", k, cfg.DiskTempByType)
			}
		}
	})

	t.Run("non-canonical already-lowercase key is preserved", func(t *testing.T) {
		custom := alertconfig.HysteresisThreshold{Trigger: 40, Clear: 35}
		cfg := &alertconfig.AlertConfig{
			DiskTempByType: map[string]alertconfig.HysteresisThreshold{
				"external": custom,
			},
		}
		alertconfig.NormalizeDiskTempByType(cfg)
		got, ok := cfg.DiskTempByType["external"]
		if !ok {
			t.Fatalf("external key should survive, map=%+v", cfg.DiskTempByType)
		}
		if !htEq(got, custom) {
			t.Fatalf("external = %+v, want %+v preserved", got, custom)
		}
	})

	t.Run("uppercased duplicate of canonical key does not overwrite existing", func(t *testing.T) {
		original := alertconfig.HysteresisThreshold{Trigger: 50, Clear: 45}
		cfg := &alertconfig.AlertConfig{
			DiskTempByType: map[string]alertconfig.HysteresisThreshold{
				"SAS": {Trigger: 1, Clear: 1},
				"sas": original,
			},
		}
		alertconfig.NormalizeDiskTempByType(cfg)
		if _, ok := cfg.DiskTempByType["SAS"]; ok {
			t.Fatalf("uppercase SAS should be removed, map=%+v", cfg.DiskTempByType)
		}
		if got := cfg.DiskTempByType["sas"]; !htEq(got, original) {
			t.Fatalf("sas = %+v, want %+v (not overwritten by dup)", got, original)
		}
	})

	t.Run("mixed-case key lowercased into absent canonical slot keeps its value", func(t *testing.T) {
		custom := alertconfig.HysteresisThreshold{Trigger: 72, Clear: 67}
		cfg := &alertconfig.AlertConfig{
			DiskTempByType: map[string]alertconfig.HysteresisThreshold{
				"NVMe": custom,
			},
		}
		alertconfig.NormalizeDiskTempByType(cfg)
		if _, ok := cfg.DiskTempByType["NVMe"]; ok {
			t.Fatalf("mixed-case NVMe should be removed, map=%+v", cfg.DiskTempByType)
		}
		got, ok := cfg.DiskTempByType["nvme"]
		if !ok {
			t.Fatalf("lowercased nvme should be present, map=%+v", cfg.DiskTempByType)
		}
		if !htEq(got, custom) {
			t.Fatalf("nvme = %+v, want %+v (value preserved through rename)", got, custom)
		}
	})

	t.Run("non-positive trigger or clear resets canonical key to default", func(t *testing.T) {
		cfg := &alertconfig.AlertConfig{
			DiskTempByType: map[string]alertconfig.HysteresisThreshold{
				"sata": {Trigger: 88, Clear: 0},
				"sas":  {Trigger: 0, Clear: 60},
				"nvme": {Trigger: 70, Clear: -1},
			},
		}
		alertconfig.NormalizeDiskTempByType(cfg)
		if got := cfg.DiskTempByType["sata"]; !htEq(got, wantDefaults["sata"]) {
			t.Fatalf("sata = %+v, want default %+v (clear<=0)", got, wantDefaults["sata"])
		}
		if got := cfg.DiskTempByType["sas"]; !htEq(got, wantDefaults["sas"]) {
			t.Fatalf("sas = %+v, want default %+v (trigger<=0)", got, wantDefaults["sas"])
		}
		if got := cfg.DiskTempByType["nvme"]; !htEq(got, wantDefaults["nvme"]) {
			t.Fatalf("nvme = %+v, want default %+v (clear<=0)", got, wantDefaults["nvme"])
		}
	})
}

// TestBranchCovNormalizeMetricTimeThresholds covers the empty->nil early returns,
// type canonicalization, legacy-type rejection (with "all" preserved), metric
// trim/lowercase, negative-delay and empty-metric dropping, and the full-filter
// collapse to nil.
func TestBranchCovNormalizeMetricTimeThresholds(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]map[string]int
		want map[string]map[string]int
	}{
		{name: "nil input returns nil", in: nil, want: nil},
		{name: "empty input returns nil", in: map[string]map[string]int{}, want: nil},
		{name: "whitespace-only type key dropped -> nil", in: map[string]map[string]int{"   ": {"cpu": 5}}, want: nil},
		{name: "empty inner metrics map dropped -> nil", in: map[string]map[string]int{"guest": {}}, want: nil},
		{name: "unsupported legacy type qemu dropped -> nil", in: map[string]map[string]int{"qemu": {"cpu": 5}}, want: nil},
		{name: "unsupported legacy type docker dropped -> nil", in: map[string]map[string]int{"docker": {"cpu": 5}}, want: nil},
		{
			name: "all type preserved through the legacy guard",
			in:   map[string]map[string]int{"all": {"cpu": 7}},
			want: map[string]map[string]int{"all": {"cpu": 7}},
		},
		{
			name: "type canonicalized (Kubernetes Pod -> pod) and metric trimmed/lowercased",
			in:   map[string]map[string]int{"Kubernetes Pod": {"  CPU ": 10}},
			want: map[string]map[string]int{"pod": {"cpu": 10}},
		},
		{
			name: "type trimmed/lowercased (  Guest  -> guest)",
			in:   map[string]map[string]int{"  Guest  ": {"Disk": 4}},
			want: map[string]map[string]int{"guest": {"disk": 4}},
		},
		{
			name: "empty metric key and negative delay are dropped, valid kept",
			in:   map[string]map[string]int{"guest": {"": 5, "cpu": -1, "mem": 3}},
			want: map[string]map[string]int{"guest": {"mem": 3}},
		},
		{
			name: "everything filtered out collapses to nil",
			in:   map[string]map[string]int{"guest": {"": 1}, "qemu": {"cpu": 2}},
			want: nil,
		},
		{
			name: "multiple valid types normalized independently",
			in:   map[string]map[string]int{"node": {"CPU": 6}, "guest": {"memory": 8, "disk": 9}},
			want: map[string]map[string]int{"node": {"cpu": 6}, "guest": {"memory": 8, "disk": 9}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := alertconfig.NormalizeMetricTimeThresholds(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
