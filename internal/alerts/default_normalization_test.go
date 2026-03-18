package alerts

import "testing"

func TestNormalizeSnapshotDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   SnapshotAlertConfig
		want SnapshotAlertConfig
	}{
		{
			name: "negative values clamp to zero",
			in: SnapshotAlertConfig{
				WarningDays:     -1,
				CriticalDays:    -2,
				WarningSizeGiB:  -3,
				CriticalSizeGiB: -4,
			},
			want: SnapshotAlertConfig{},
		},
		{
			name: "warning values clamp down to critical",
			in: SnapshotAlertConfig{
				WarningDays:     10,
				CriticalDays:    3,
				WarningSizeGiB:  40,
				CriticalSizeGiB: 8,
			},
			want: SnapshotAlertConfig{
				WarningDays:     3,
				CriticalDays:    3,
				WarningSizeGiB:  8,
				CriticalSizeGiB: 8,
			},
		},
		{
			name: "critical inherits warning when critical unset",
			in: SnapshotAlertConfig{
				WarningDays:     7,
				CriticalDays:    0,
				WarningSizeGiB:  12,
				CriticalSizeGiB: 0,
			},
			want: SnapshotAlertConfig{
				WarningDays:     7,
				CriticalDays:    7,
				WarningSizeGiB:  12,
				CriticalSizeGiB: 12,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &AlertConfig{SnapshotDefaults: tc.in}
			normalizeSnapshotDefaults(cfg)
			got := cfg.SnapshotDefaults
			if got != tc.want {
				t.Fatalf("normalizeSnapshotDefaults() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNormalizeBackupDefaults(t *testing.T) {
	falseValue := false
	cfg := &AlertConfig{
		BackupDefaults: BackupAlertConfig{
			WarningDays:   10,
			CriticalDays:  3,
			AlertOrphaned: &falseValue,
			IgnoreVMIDs: []string{
				" 101 ",
				"",
				"101",
				" 10* ",
				"   ",
				"10*",
			},
		},
	}

	normalizeBackupDefaults(cfg)

	if cfg.BackupDefaults.WarningDays != 3 {
		t.Fatalf("WarningDays = %d, want 3", cfg.BackupDefaults.WarningDays)
	}
	if cfg.BackupDefaults.CriticalDays != 3 {
		t.Fatalf("CriticalDays = %d, want 3", cfg.BackupDefaults.CriticalDays)
	}
	if cfg.BackupDefaults.AlertOrphaned == nil {
		t.Fatal("AlertOrphaned should not be nil")
	}
	if *cfg.BackupDefaults.AlertOrphaned != false {
		t.Fatalf("AlertOrphaned = %v, want false", *cfg.BackupDefaults.AlertOrphaned)
	}

	wantIgnore := []string{"101", "10*"}
	if len(cfg.BackupDefaults.IgnoreVMIDs) != len(wantIgnore) {
		t.Fatalf("IgnoreVMIDs length = %d, want %d (%v)", len(cfg.BackupDefaults.IgnoreVMIDs), len(wantIgnore), cfg.BackupDefaults.IgnoreVMIDs)
	}
	for i := range wantIgnore {
		if cfg.BackupDefaults.IgnoreVMIDs[i] != wantIgnore[i] {
			t.Fatalf("IgnoreVMIDs[%d] = %q, want %q", i, cfg.BackupDefaults.IgnoreVMIDs[i], wantIgnore[i])
		}
	}
}

func TestNormalizeBackupDefaultsSetsAlertOrphanedDefaultAndClampsNegativeDays(t *testing.T) {
	cfg := &AlertConfig{
		BackupDefaults: BackupAlertConfig{
			WarningDays:  -2,
			CriticalDays: -1,
		},
	}

	normalizeBackupDefaults(cfg)

	if cfg.BackupDefaults.WarningDays != 0 {
		t.Fatalf("WarningDays = %d, want 0", cfg.BackupDefaults.WarningDays)
	}
	if cfg.BackupDefaults.CriticalDays != 0 {
		t.Fatalf("CriticalDays = %d, want 0", cfg.BackupDefaults.CriticalDays)
	}
	if cfg.BackupDefaults.AlertOrphaned == nil {
		t.Fatal("AlertOrphaned should be defaulted to true")
	}
	if !*cfg.BackupDefaults.AlertOrphaned {
		t.Fatalf("AlertOrphaned = %v, want true", *cfg.BackupDefaults.AlertOrphaned)
	}
}

func TestBackupIgnoreVMID(t *testing.T) {
	tests := []struct {
		name       string
		vmID       string
		ignoreList []string
		want       bool
	}{
		{
			name:       "empty vmid returns false",
			vmID:       "",
			ignoreList: []string{"101"},
			want:       false,
		},
		{
			name:       "exact match",
			vmID:       "101",
			ignoreList: []string{"101"},
			want:       true,
		},
		{
			name:       "prefix wildcard match",
			vmID:       "10123",
			ignoreList: []string{"101*"},
			want:       true,
		},
		{
			name:       "trim spaces before matching",
			vmID:       "200",
			ignoreList: []string{" 200 "},
			want:       true,
		},
		{
			name:       "star only pattern does not match everything",
			vmID:       "101",
			ignoreList: []string{"*"},
			want:       false,
		},
		{
			name:       "blank entries ignored",
			vmID:       "101",
			ignoreList: []string{" ", ""},
			want:       false,
		},
		{
			name:       "no match",
			vmID:       "999",
			ignoreList: []string{"101", "10*"},
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := backupIgnoreVMID(tc.vmID, tc.ignoreList); got != tc.want {
				t.Fatalf("backupIgnoreVMID(%q, %v) = %v, want %v", tc.vmID, tc.ignoreList, got, tc.want)
			}
		})
	}
}
