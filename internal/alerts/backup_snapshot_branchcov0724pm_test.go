package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestBranchcov0724pmBackupAlertStillTriggered raises branch coverage of
// backupAlertStillTriggered (backup_snapshot.go:85, baseline 61.1%). The
// existing suite only reaches the trailing `return false`, so nearly every
// guard body and both threshold "still triggered" arms are uncovered. These
// cases drive: the nil-alert guard, the disabled-config guard, the
// ignored-VMID suppression, the orphaned-but-AlertOrphaned-off suppression,
// the ageDays-metadata-missing fall-back to alert.Value, the guestVmid-absent
// fall-back, and the critical/warning trigger arms -- each asserting a
// concrete, observable boolean.
//
// NOTE: the inner `if parsed := metadataIntValue(...); parsed > 0` arm at
// backup_snapshot.go:92 is provably unreachable (see report) and is therefore
// not exercised here.
func TestBranchcov0724pmBackupAlertStillTriggered(t *testing.T) {
	orphanedOff := false
	cases := []struct {
		name  string
		alert *Alert
		cfg   BackupAlertConfig
		want  bool
	}{
		{
			// Nil-alert guard: returns false regardless of cfg.
			name:  "nil_alert_returns_false",
			alert: nil,
			cfg:   BackupAlertConfig{Enabled: true, WarningDays: 3},
			want:  false,
		},
		{
			// Disabled-config guard: returns false.
			name:  "disabled_cfg_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{"ageDays": 99.0}},
			cfg:   BackupAlertConfig{Enabled: false, WarningDays: 3},
			want:  false,
		},
		{
			// guestVmid absent -> vmid=="" body is entered (parsed stays 0).
			// ageDays present (4) over warning threshold (3) -> still triggered.
			name:  "guest_vmid_absent_still_triggers_warning",
			alert: &Alert{Metadata: map[string]interface{}{"ageDays": 4.0}},
			cfg:   BackupAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want:  true,
		},
		{
			// Ignored VMID suppresses even when age would trigger.
			name: "ignored_vmid_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"ageDays":   99.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 3, IgnoreVMIDs: []string{"100"}},
			want: false,
		},
		{
			// Orphaned backup with AlertOrphaned pointing at false is suppressed.
			name: "orphaned_with_alert_orphaned_off_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"orphaned":  true,
				"ageDays":   99.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 3, AlertOrphaned: &orphanedOff},
			want: false,
		},
		{
			// ageDays metadata absent -> ageValue falls back to alert.Value (7),
			// which clears the warning threshold (5) -> still triggered.
			name: "age_falls_back_to_alert_value_triggers_warning",
			alert: &Alert{
				Value:    7.0,
				Metadata: map[string]interface{}{"guestVmid": "100"},
			},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 5},
			want: true,
		},
		{
			// Critical arm: ageDays (10) >= CriticalDays (5).
			name: "critical_threshold_triggers",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"ageDays":   10.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want: true,
		},
		{
			// Warning-only arm: ageDays (4) below critical (5) but >= warning (3).
			name: "warning_threshold_triggers_below_critical",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"ageDays":   4.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want: true,
		},
		{
			// Boundary: ageDays (3) below warning threshold (3 fails >=) -> false.
			name: "below_warning_threshold_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"ageDays":   2.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true, WarningDays: 3},
			want: false,
		},
		{
			// No thresholds configured at all -> never triggers.
			name: "no_thresholds_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"guestVmid": "100",
				"ageDays":   50.0,
			}},
			cfg:  BackupAlertConfig{Enabled: true},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := backupAlertStillTriggered(tc.alert, tc.cfg); got != tc.want {
				t.Fatalf("backupAlertStillTriggered want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmGuestMatchesBackupType raises branch coverage of
// guestMatchesBackupType (backup_snapshot.go:191, baseline 80%). The existing
// suite covers the "guest-type-unknown -> false" and the equality arms but not
// the "backup-type-unknown -> true" arm. These cases drive the unknown-backup-
// type arm (with a typed guest, to prove it matches anything), the guest-
// unknown arm, and both sides of the equality comparison.
func TestBranchcov0724pmGuestMatchesBackupType(t *testing.T) {
	cases := []struct {
		name       string
		guest      GuestLookup
		backupType string
		want       bool
	}{
		{
			// UNCOVERED arm: unknown backup type matches a typed guest.
			name:       "unknown_backup_type_matches_typed_guest",
			guest:      GuestLookup{Type: "qemu"},
			backupType: "host",
			want:       true,
		},
		{
			// UNCOVERED arm via empty backup type: "" is unknown -> matches.
			name:       "empty_backup_type_matches",
			guest:      GuestLookup{Type: "lxc"},
			backupType: "",
			want:       true,
		},
		{
			// Guest type unknown, backup typed -> never matches.
			name:       "unknown_guest_type_never_matches_typed_backup",
			guest:      GuestLookup{Type: "storage"},
			backupType: "qemu",
			want:       false,
		},
		{
			// Same kind (qemu == qemu).
			name:       "same_kind_matches",
			guest:      GuestLookup{Type: "qemu"},
			backupType: "vm",
			want:       true,
		},
		{
			// Different kind (lxc guest vs qemu backup).
			name:       "different_kind_does_not_match",
			guest:      GuestLookup{Type: "lxc"},
			backupType: "qemu",
			want:       false,
		},
		{
			// Alias normalisation on both sides (ct == container).
			name:       "alias_normalisation_both_sides",
			guest:      GuestLookup{Type: "ct"},
			backupType: "container",
			want:       true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := guestMatchesBackupType(tc.guest, tc.backupType); got != tc.want {
				t.Fatalf("guestMatchesBackupType want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmBackupOrphanInventoryReady raises branch coverage of
// backupOrphanInventoryReady (backup_snapshot.go:217, baseline 77.8%). The
// existing suite covers the nil-scope and final map-lookup arms. These cases
// drive the uncovered "non-PVE source -> ready (true)" arm and the
// "blank-instance/guestType -> not ready (false)" arm, and re-assert the map
// lookup for both a ready and not-ready inventory entry.
func TestBranchcov0724pmBackupOrphanInventoryReady(t *testing.T) {
	readyScope := &BackupInventoryScope{
		PVEOrphanInventoryReady: map[string]map[string]bool{
			"pve1": {"qemu": true, "lxc": false},
		},
	}
	cases := []struct {
		name   string
		scope  *BackupInventoryScope
		record backupRecord
		want   bool
	}{
		{
			// Nil scope -> inventory treated as ready.
			name:   "nil_scope_ready",
			scope:  nil,
			record: backupRecord{source: "PVE"},
			want:   true,
		},
		{
			// Scope present but map nil -> ready.
			name:   "nil_ready_map_ready",
			scope:  &BackupInventoryScope{},
			record: backupRecord{source: "PVE"},
			want:   true,
		},
		{
			// UNCOVERED arm: non-PVE source is always ready.
			name:   "non_pve_source_ready",
			scope:  readyScope,
			record: backupRecord{source: "PBS"},
			want:   true,
		},
		{
			// UNCOVERED arm: PVE source but blank instance -> not ready.
			name:   "pve_blank_instance_not_ready",
			scope:  readyScope,
			record: backupRecord{source: "PVE", instance: "  ", subjectType: "qemu"},
			want:   false,
		},
		{
			// UNCOVERED arm: PVE source but blank guest type -> not ready.
			name:   "pve_blank_guest_type_not_ready",
			scope:  readyScope,
			record: backupRecord{source: "PVE", instance: "pve1", subjectType: "host"},
			want:   false,
		},
		{
			// PVE source, inventory marks qemu ready -> true.
			name:   "pve_inventory_ready_true",
			scope:  readyScope,
			record: backupRecord{source: "PVE", instance: "pve1", subjectType: "qemu"},
			want:   true,
		},
		{
			// PVE source, inventory marks lxc not ready -> false.
			name:   "pve_inventory_ready_false",
			scope:  readyScope,
			record: backupRecord{source: "PVE", instance: "pve1", node: "n1", subjectType: "lxc"},
			want:   false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := backupOrphanInventoryReady(tc.scope, tc.record); got != tc.want {
				t.Fatalf("backupOrphanInventoryReady want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmBackupMatchesKnownPVETemplate raises branch coverage of
// backupMatchesKnownPVETemplate (backup_snapshot.go:232, baseline 80%). The
// existing suite covers the top guard and the final exists lookup. These cases
// drive the uncovered "unparseable/non-positive vmID -> false" arm and the
// "built subject key is empty -> false" arm, and re-assert a positive and a
// negative exists lookup against a real key.
func TestBranchcov0724pmBackupMatchesKnownPVETemplate(t *testing.T) {
	// Construct the exact key the implementation builds for a qemu template.
	qemuKey := BuildBackupPVETemplateSubjectKey("pve1", "qemu", "node1", 100)
	if qemuKey == "" {
		t.Fatalf("test setup: expected non-empty template subject key")
	}
	scope := &BackupInventoryScope{
		PVETemplateSubjects: map[string]struct{}{
			qemuKey:          {},
			"some-other-key": {},
		},
	}
	cases := []struct {
		name   string
		scope  *BackupInventoryScope
		record backupRecord
		want   bool
	}{
		{
			// Nil scope -> never a known template.
			name:   "nil_scope_false",
			scope:  nil,
			record: backupRecord{source: "PVE", vmID: "100"},
			want:   false,
		},
		{
			// Empty template set -> false (guard short-circuit).
			name:   "empty_template_set_false",
			scope:  &BackupInventoryScope{},
			record: backupRecord{source: "PVE", vmID: "100"},
			want:   false,
		},
		{
			// Non-PVE source -> false (guard).
			name:   "non_pve_source_false",
			scope:  scope,
			record: backupRecord{source: "PBS", vmID: "100"},
			want:   false,
		},
		{
			// UNCOVERED arm: PVE source but vmID not a number -> false.
			name:   "pve_non_numeric_vmid_false",
			scope:  scope,
			record: backupRecord{source: "PVE", vmID: "abc"},
			want:   false,
		},
		{
			// UNCOVERED arm: PVE source but vmID <= 0 -> false.
			name:   "pve_zero_vmid_false",
			scope:  scope,
			record: backupRecord{source: "PVE", vmID: "0"},
			want:   false,
		},
		{
			// UNCOVERED arm: valid vmID but blank instance -> empty key -> false.
			name:   "pve_blank_instance_empty_key_false",
			scope:  scope,
			record: backupRecord{source: "PVE", vmID: "100", instance: "", node: "node1", subjectType: "qemu"},
			want:   false,
		},
		{
			// Positive lookup: record matches the known qemu template key.
			name:   "known_template_matches_true",
			scope:  scope,
			record: backupRecord{source: "PVE", vmID: "100", instance: "pve1", node: "node1", subjectType: "qemu"},
			want:   true,
		},
		{
			// Negative lookup: well-formed key not present in the set.
			name:   "unknown_template_false",
			scope:  scope,
			record: backupRecord{source: "PVE", vmID: "999", instance: "pve1", node: "node1", subjectType: "qemu"},
			want:   false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := backupMatchesKnownPVETemplate(tc.scope, tc.record); got != tc.want {
				t.Fatalf("backupMatchesKnownPVETemplate want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0724pmCanonicalBackupSubjectResourceType raises branch coverage
// of canonicalBackupSubjectResourceType (backup_snapshot.go:257, baseline
// 75%). The existing suite covers the lookup-Type-present arm and the qemu/
// backup-subject switch outcomes, but not the "lxc" case nor the "vmID present
// -> VM" fall-through. These cases drive every arm with distinct expected
// ResourceType values.
func TestBranchcov0724pmCanonicalBackupSubjectResourceType(t *testing.T) {
	cases := []struct {
		name   string
		record backupRecord
		want   unifiedresources.ResourceType
	}{
		{
			// lookup.Type present (lxc) wins -> SystemContainer via guest helper.
			name:   "lookup_type_lxc_wins",
			record: backupRecord{lookup: GuestLookup{Type: "lxc"}, subjectType: "qemu"},
			want:   unifiedresources.ResourceTypeSystemContainer,
		},
		{
			// lookup.Type present (qemu) wins -> VM.
			name:   "lookup_type_qemu_wins",
			record: backupRecord{lookup: GuestLookup{Type: "qemu"}, subjectType: "lxc"},
			want:   unifiedresources.ResourceTypeVM,
		},
		{
			// UNCOVERED arm: no lookup type, subjectType normalises to lxc.
			name:   "subject_type_lxc",
			record: backupRecord{subjectType: "ct"},
			want:   unifiedresources.ResourceTypeSystemContainer,
		},
		{
			// subjectType normalises to qemu -> VM.
			name:   "subject_type_qemu",
			record: backupRecord{subjectType: "vm"},
			want:   unifiedresources.ResourceTypeVM,
		},
		{
			// UNCOVERED arm: unknown subjectType but vmID present -> VM.
			name:   "unknown_subject_with_vmid_vm",
			record: backupRecord{subjectType: "host", vmID: "100"},
			want:   unifiedresources.ResourceTypeVM,
		},
		{
			// Unknown subjectType and no vmID -> generic backup-subject.
			name:   "unknown_subject_no_vmid_backup_subject",
			record: backupRecord{subjectType: "storage"},
			want:   unifiedresources.ResourceType("backup-subject"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalBackupSubjectResourceType(tc.record); got != tc.want {
				t.Fatalf("canonicalBackupSubjectResourceType want %q, got %q", tc.want, got)
			}
		})
	}
}
