package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestBranchcov0725amSnapshotAlertStillTriggered raises statement coverage of
// snapshotAlertStillTriggered (backup_snapshot.go:61, baseline 15.4%). The
// existing suite reaches snapshot evaluation only through the live
// CheckSnapshotsForInstance path, so snapshotAlertStillTriggered itself is
// almost entirely uncovered: only the trailing `return false` is hit. These
// cases independently drive every arm with assertions that distinguish them:
//   - the nil-alert guard and the disabled-cfg guard (both -> false)
//   - each of the four threshold "still triggered" arms in isolation:
//     critical-days, warning-days, critical-size, warning-size (each -> true)
//   - the `>=` boundary at exactly the threshold value (-> true) for both age
//     and size, plus the just-below value (-> false) to pin the comparison
//   - missing metadata (age/size default to 0 -> false)
//   - no thresholds configured at all (-> false)
//
// Each case is constructed so exactly one arm is the deciding one (e.g. to
// isolate the size arms the age is held below every age threshold).
func TestBranchcov0725amSnapshotAlertStillTriggered(t *testing.T) {
	cases := []struct {
		name  string
		alert *Alert
		cfg   SnapshotAlertConfig
		want  bool
	}{
		{
			// Nil-alert guard: returns false regardless of cfg.
			name:  "nil_alert_returns_false",
			alert: nil,
			cfg:   SnapshotAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want:  false,
		},
		{
			// Disabled-cfg guard: returns false even with triggering metadata.
			name:  "disabled_cfg_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{"snapshotAgeDays": 99.0}},
			cfg:   SnapshotAlertConfig{Enabled: false, WarningDays: 3, CriticalDays: 5},
			want:  false,
		},
		{
			// Critical-age arm: age 10 >= CriticalDays 5 (and > WarningDays 3).
			name: "critical_age_triggers",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 10.0,
				"snapshotSizeGiB": 0.0,
			}},
			cfg:  SnapshotAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want: true,
		},
		{
			// Warning-age arm: age 4 below CriticalDays 5 but >= WarningDays 3.
			name: "warning_age_triggers_below_critical",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 4.0,
			}},
			cfg:  SnapshotAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want: true,
		},
		{
			// Critical-size arm (age held low so it cannot fire):
			// size 10 >= CriticalSizeGiB 5.
			name: "critical_size_triggers",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 1.0,
				"snapshotSizeGiB": 10.0,
			}},
			cfg: SnapshotAlertConfig{
				Enabled:         true,
				WarningSizeGiB:  3,
				CriticalSizeGiB: 5,
			},
			want: true,
		},
		{
			// Warning-size arm (age held low): size 4 below CriticalSizeGiB 5
			// but >= WarningSizeGiB 3.
			name: "warning_size_triggers_below_critical",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 1.0,
				"snapshotSizeGiB": 4.0,
			}},
			cfg: SnapshotAlertConfig{
				Enabled:         true,
				WarningSizeGiB:  3,
				CriticalSizeGiB: 5,
			},
			want: true,
		},
		{
			// Age boundary: age exactly == CriticalDays -> `>=` is true.
			name: "age_at_exact_critical_boundary_triggers",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 5.0,
			}},
			cfg:  SnapshotAlertConfig{Enabled: true, CriticalDays: 5, WarningDays: 3},
			want: true,
		},
		{
			// Size boundary: size exactly == WarningSizeGiB -> `>=` is true.
			name: "size_at_exact_warning_boundary_triggers",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 1.0,
				"snapshotSizeGiB": 3.0,
			}},
			cfg: SnapshotAlertConfig{
				Enabled:         true,
				WarningSizeGiB:  3,
				CriticalSizeGiB: 5,
			},
			want: true,
		},
		{
			// Just-below boundary: age 2 < WarningDays 3 -> false.
			name: "age_below_warning_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 2.0,
			}},
			cfg:  SnapshotAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5},
			want: false,
		},
		{
			// Missing metadata: age/size default to 0 -> false.
			name:  "missing_metadata_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{"snapshotName": "daily"}},
			cfg:   SnapshotAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5, WarningSizeGiB: 3, CriticalSizeGiB: 5},
			want:  false,
		},
		{
			// No thresholds configured at all -> never triggers.
			name: "no_thresholds_returns_false",
			alert: &Alert{Metadata: map[string]interface{}{
				"snapshotAgeDays": 99.0,
				"snapshotSizeGiB": 99.0,
			}},
			cfg:  SnapshotAlertConfig{Enabled: true},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := snapshotAlertStillTriggered(tc.alert, tc.cfg); got != tc.want {
				t.Fatalf("snapshotAlertStillTriggered want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestBranchcov0725amParseStableGuestOverrideKey raises statement coverage of
// parseStableGuestOverrideKey (guest_override_identity.go:64, baseline 30.0%).
// The existing suite only reaches it indirectly via lookupGuestOverride for the
// happy path. These cases drive every deciding arm and assert the parsed
// identity (instance/node/vmid) so each producing arm is pinned:
//   - empty key, wrong part-count (2 and 4), and parts[0] != "guest" (the
//     canonical-key shape "inst:node:vmid") -> {}, false
//   - parts[0] == "guest" but vmid non-numeric, zero, or negative -> false
//   - instance fails isCanonicalGuestKeyPart (empty, or contains "/") -> false
//   - happy path: "guest:inst:100" -> {inst, inst, 100}, true (node is mirrored
//     from instance), including leading/trailing whitespace trimming
func TestBranchcov0725amParseStableGuestOverrideKey(t *testing.T) {
	cases := []struct {
		name      string
		key       string
		wantIdent guestOverrideIdentity
		wantOk    bool
	}{
		{
			// Empty key -> after TrimSpace still "" -> Split -> [""] len 1 -> false.
			name:      "empty_key_false",
			key:       "",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// Wrong part count: 2 parts -> false.
			name:      "two_parts_false",
			key:       "guest:100",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// Wrong part count: 4 parts -> false.
			name:      "four_parts_false",
			key:       "guest:inst:100:extra",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// parts[0] != "guest" (canonical-key shape) -> false. This is the
			// distinct arm from a valid canonical key, which parseCanonicalGuestKey
			// accepts but this function must reject.
			name:      "non_guest_prefix_false",
			key:       "pve1:node1:100",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// parts[0] == "guest" but vmid non-numeric -> false.
			name:      "non_numeric_vmid_false",
			key:       "guest:inst:abc",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// vmid == 0 -> `vmid <= 0` guard -> false.
			name:      "zero_vmid_false",
			key:       "guest:inst:0",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// vmid negative -> `vmid <= 0` guard -> false.
			name:      "negative_vmid_false",
			key:       "guest:inst:-5",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// instance empty -> isCanonicalGuestKeyPart("") false -> false.
			name:      "empty_instance_false",
			key:       "guest::100",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// instance contains "/" -> isCanonicalGuestKeyPart false -> false.
			name:      "instance_with_slash_false",
			key:       "guest:inst/sub:100",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// Happy path: node is mirrored from instance.
			name:      "valid_key_mirrors_node",
			key:       "guest:pve1:100",
			wantIdent: guestOverrideIdentity{instance: "pve1", node: "pve1", vmid: 100},
			wantOk:    true,
		},
		{
			// Leading/trailing whitespace is trimmed on the whole key and on
			// the instance segment before validation.
			name:      "valid_key_with_whitespace_trimmed",
			key:       "  guest:  pve1  :200  ",
			wantIdent: guestOverrideIdentity{instance: "pve1", node: "pve1", vmid: 200},
			wantOk:    true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotIdent, gotOk := parseStableGuestOverrideKey(tc.key)
			if gotOk != tc.wantOk {
				t.Fatalf("parseStableGuestOverrideKey(%q) ok = %v, want %v", tc.key, gotOk, tc.wantOk)
			}
			if gotIdent != tc.wantIdent {
				t.Fatalf("parseStableGuestOverrideKey(%q) = %+v, want %+v", tc.key, gotIdent, tc.wantIdent)
			}
		})
	}
}

// TestBranchcov0725amExtractGuestSnapshot raises statement coverage of
// extractGuestSnapshot (guest_snapshot.go:245, baseline 35.7%). The existing
// suite covers only the default (unknown-type) arm. These cases drive every
// other type-switch arm and each pointer-nil guard, asserting on the real
// returned snapshot (Kind, VMID, Name) so the producing arm is unambiguous:
//   - models.VM and *models.VM (non-nil) -> guestKindVM with propagated fields
//   - *models.VM nil -> empty canonical snapshot, false
//   - models.Container and *models.Container (non-nil) -> guestKindContainer
//   - *models.Container nil -> empty canonical snapshot, false
//   - guestSnapshot value and *guestSnapshot (non-nil) -> normalized passthrough
//   - *guestSnapshot nil -> empty canonical snapshot, false
//
// The already-covered default arm (struct{} -> false) is intentionally not
// duplicated here.
func TestBranchcov0725amExtractGuestSnapshot(t *testing.T) {
	cases := []struct {
		name     string
		guest    any
		wantOk   bool
		wantKind guestKind
		wantVMID int
		wantName string
		// wantNilCollections asserts the false-returning arms yield the
		// canonical empty snapshot (non-nil Disks/Tags slices).
		wantNilCollections bool
	}{
		{
			// models.VM value arm -> VM kind, fields propagated.
			name:     "vm_value",
			guest:    models.VM{Instance: "pve1", Node: "n1", VMID: 100, Name: "vm100"},
			wantOk:   true,
			wantKind: guestKindVM,
			wantVMID: 100,
			wantName: "vm100",
		},
		{
			// *models.VM non-nil arm -> VM kind, fields propagated.
			name:     "vm_ptr_nonnil",
			guest:    &models.VM{Instance: "pve1", Node: "n1", VMID: 101, Name: "vm101"},
			wantOk:   true,
			wantKind: guestKindVM,
			wantVMID: 101,
			wantName: "vm101",
		},
		{
			// *models.VM nil arm -> false, canonical empty snapshot.
			name:               "vm_ptr_nil",
			guest:              (*models.VM)(nil),
			wantOk:             false,
			wantKind:           guestKindUnknown,
			wantNilCollections: true,
		},
		{
			// models.Container value arm -> Container kind, fields propagated.
			name:     "container_value",
			guest:    models.Container{Instance: "pve2", Node: "n2", VMID: 200, Name: "ct200"},
			wantOk:   true,
			wantKind: guestKindContainer,
			wantVMID: 200,
			wantName: "ct200",
		},
		{
			// *models.Container non-nil arm -> Container kind.
			name:     "container_ptr_nonnil",
			guest:    &models.Container{Instance: "pve2", Node: "n2", VMID: 201, Name: "ct201"},
			wantOk:   true,
			wantKind: guestKindContainer,
			wantVMID: 201,
			wantName: "ct201",
		},
		{
			// *models.Container nil arm -> false, canonical empty snapshot.
			name:               "container_ptr_nil",
			guest:              (*models.Container)(nil),
			wantOk:             false,
			wantKind:           guestKindUnknown,
			wantNilCollections: true,
		},
		{
			// guestSnapshot value arm -> passthrough (and normalized).
			name:     "guest_snapshot_value",
			guest:    guestSnapshot{Kind: guestKindVM, VMID: 300, Name: "gs300", Disks: nil, Tags: nil},
			wantOk:   true,
			wantKind: guestKindVM,
			wantVMID: 300,
			wantName: "gs300",
		},
		{
			// *guestSnapshot non-nil arm -> passthrough (and normalized).
			name:     "guest_snapshot_ptr_nonnil",
			guest:    &guestSnapshot{Kind: guestKindContainer, VMID: 301, Name: "gs301"},
			wantOk:   true,
			wantKind: guestKindContainer,
			wantVMID: 301,
			wantName: "gs301",
		},
		{
			// *guestSnapshot nil arm -> false, canonical empty snapshot.
			name:               "guest_snapshot_ptr_nil",
			guest:              (*guestSnapshot)(nil),
			wantOk:             false,
			wantKind:           guestKindUnknown,
			wantNilCollections: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractGuestSnapshot(tc.guest)
			if ok != tc.wantOk {
				t.Fatalf("extractGuestSnapshot(%T) ok = %v, want %v", tc.guest, ok, tc.wantOk)
			}
			if got.Kind != tc.wantKind {
				t.Fatalf("extractGuestSnapshot(%T) Kind = %v, want %v", tc.guest, got.Kind, tc.wantKind)
			}
			if tc.wantOk {
				if got.VMID != tc.wantVMID {
					t.Fatalf("extractGuestSnapshot(%T) VMID = %d, want %d", tc.guest, got.VMID, tc.wantVMID)
				}
				if got.Name != tc.wantName {
					t.Fatalf("extractGuestSnapshot(%T) Name = %q, want %q", tc.guest, got.Name, tc.wantName)
				}
			}
			if tc.wantNilCollections {
				// The canonical empty snapshot must have non-nil empty slices.
				if got.Disks == nil || got.Tags == nil {
					t.Fatalf("extractGuestSnapshot(%T) expected normalized empty collections, got Disks=%v Tags=%v", tc.guest, got.Disks, got.Tags)
				}
				if len(got.Disks) != 0 || len(got.Tags) != 0 {
					t.Fatalf("extractGuestSnapshot(%T) expected empty collections, got Disks=%v Tags=%v", tc.guest, got.Disks, got.Tags)
				}
			}
		})
	}
}

// TestBranchcov0725amGuestOverrideIdentityFromGuestOrID raises statement
// coverage of guestOverrideIdentityFromGuestOrID (guest_override_identity.go:87,
// baseline 50.0%). These cases drive every type-switch arm, each pointer-nil
// guard, and both halves of the default fallback (canonical-key parse success
// vs. stable-key parse). Each case asserts the concrete returned identity so
// the producing arm is pinned:
//   - models.VM / *models.VM (non-nil) / models.Container / *models.Container
//     (non-nil) -> guestOverrideIdentityFromParts with the model's fields
//   - *models.VM nil and *models.Container nil -> {}, false
//   - a VM with no instance/node -> fromParts returns false (drives the
//     fromParts failure path reached only through the VM arm)
//   - default + canonical key "inst:node:vmid" (parts[0] != "guest") -> parsed
//   - default + stable key "guest:inst:vmid" (canonical parse rejects "guest"
//     prefix, stable parse succeeds) -> parsed with node mirrored from instance
//   - default + unparseable id -> both parses fail -> false
func TestBranchcov0725amGuestOverrideIdentityFromGuestOrID(t *testing.T) {
	cases := []struct {
		name      string
		guest     any
		guestID   string
		wantIdent guestOverrideIdentity
		wantOk    bool
	}{
		{
			// models.VM value arm.
			name:      "vm_value",
			guest:     models.VM{Instance: "pve1", Node: "node1", VMID: 100},
			wantIdent: guestOverrideIdentity{instance: "pve1", node: "node1", vmid: 100},
			wantOk:    true,
		},
		{
			// *models.VM non-nil arm.
			name:      "vm_ptr_nonnil",
			guest:     &models.VM{Instance: "pve1", Node: "node1", VMID: 101},
			wantIdent: guestOverrideIdentity{instance: "pve1", node: "node1", vmid: 101},
			wantOk:    true,
		},
		{
			// *models.VM nil arm.
			name:      "vm_ptr_nil",
			guest:     (*models.VM)(nil),
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// models.VM with no instance/node -> fromParts returns false
			// (instance/node empty). Drives the fromParts failure path.
			name:      "vm_missing_instance_node_false",
			guest:     models.VM{VMID: 102},
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// models.Container value arm.
			name:      "container_value",
			guest:     models.Container{Instance: "pve2", Node: "node2", VMID: 200},
			wantIdent: guestOverrideIdentity{instance: "pve2", node: "node2", vmid: 200},
			wantOk:    true,
		},
		{
			// *models.Container non-nil arm.
			name:      "container_ptr_nonnil",
			guest:     &models.Container{Instance: "pve2", Node: "node2", VMID: 201},
			wantIdent: guestOverrideIdentity{instance: "pve2", node: "node2", vmid: 201},
			wantOk:    true,
		},
		{
			// *models.Container nil arm.
			name:      "container_ptr_nil",
			guest:     (*models.Container)(nil),
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
		{
			// default arm: canonical key parses (parts[0] != "guest").
			name:      "default_canonical_key",
			guest:     nil,
			guestID:   "pve3:node3:300",
			wantIdent: guestOverrideIdentity{instance: "pve3", node: "node3", vmid: 300},
			wantOk:    true,
		},
		{
			// default arm: canonical parse rejects "guest:" prefix, stable
			// parse succeeds; node is mirrored from instance.
			name:      "default_stable_key",
			guest:     nil,
			guestID:   "guest:pve4:400",
			wantIdent: guestOverrideIdentity{instance: "pve4", node: "pve4", vmid: 400},
			wantOk:    true,
		},
		{
			// default arm: both parses fail (single-segment, non-numeric) ->
			// false.
			name:      "default_unparseable_false",
			guest:     nil,
			guestID:   "garbage",
			wantIdent: guestOverrideIdentity{},
			wantOk:    false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotIdent, gotOk := guestOverrideIdentityFromGuestOrID(tc.guest, tc.guestID)
			if gotOk != tc.wantOk {
				t.Fatalf("guestOverrideIdentityFromGuestOrID ok = %v, want %v", gotOk, tc.wantOk)
			}
			if gotIdent != tc.wantIdent {
				t.Fatalf("guestOverrideIdentityFromGuestOrID = %+v, want %+v", gotIdent, tc.wantIdent)
			}
		})
	}
}

// TestBranchcov0725amHasActiveAlertTrackingKeyNoLock raises statement coverage
// of (*Manager).hasActiveAlertTrackingKeyNoLock (canonical_identity.go:413,
// baseline 40.0%). The existing suite only reaches the direct-map-key lookup
// arm. These cases drive every arm with assertions that distinguish them:
//   - empty trackingKey -> false (guard)
//   - trackingKey is a direct map key -> true (direct-lookup arm)
//   - trackingKey is NOT a direct key but equals an alert's canonical tracking
//     key -> true (the iteration/scan arm), using an alert stored under a
//     legacy id whose derived canonical state differs from its storage key
//   - trackingKey absent everywhere -> false (final return)
//   - a nil-valued map entry is skipped without panic and without a false
//     positive (the nil-continue arm), proven by querying a key that is only
//     "reachable" through the nil entry
//
// A minimal Manager literal is used because the method reads m.activeAlerts
// only; no goroutines or persistence are required.
func TestBranchcov0725amHasActiveAlertTrackingKeyNoLock(t *testing.T) {
	// Build alerts that exercise both the direct-key and the canonical-scan
	// arms. The "legacy" alert is stored under its legacy ID but carries a
	// CanonicalState that canonicalTrackingKeyForAlert will derive.
	legacyAlert := &Alert{
		ID:              "backup-age-legacy-1",
		Type:            "backup-age",
		ResourceID:      "res-1",
		CanonicalSpecID: "res-1-backup-age",
		CanonicalState:  "res-1::res-1-backup-age",
	}
	legacyCanonicalKey := canonicalTrackingKeyForAlert(legacyAlert)
	if legacyCanonicalKey == "" || legacyCanonicalKey == legacyAlert.ID {
		t.Fatalf("test setup: legacy alert must have a canonical key distinct from its storage id; got %q", legacyCanonicalKey)
	}

	// directKey alert is stored under a plain key with no canonical identity,
	// so only the direct-lookup arm can find it.
	directKey := "plain-direct-key"
	directAlert := &Alert{ID: directKey, Type: "cpu"}

	m := &Manager{
		activeAlerts: map[string]*Alert{
			legacyAlert.ID: legacyAlert, // stored under legacy id, NOT under legacyCanonicalKey
			directKey:      directAlert,
			"nil-slot":     nil, // nil-valued entry to drive the continue arm
		},
	}

	cases := []struct {
		name        string
		trackingKey string
		want        bool
	}{
		{
			// Guard arm: empty key -> false.
			name:        "empty_key_false",
			trackingKey: "",
			want:        false,
		},
		{
			// Direct-lookup arm: key present in the map -> true.
			name:        "direct_key_true",
			trackingKey: directKey,
			want:        true,
		},
		{
			// Scan arm: key NOT a direct map key but equals an alert's derived
			// canonical tracking key -> true.
			name:        "canonical_scan_true",
			trackingKey: legacyCanonicalKey,
			want:        true,
		},
		{
			// Final return: key absent everywhere -> false.
			name:        "missing_key_false",
			trackingKey: "does-not-exist",
			want:        false,
		},
		{
			// Nil-continue arm: the only map entry is a nil-valued slot, so the
			// scan must skip it and return false without panicking. Using a
			// dedicated manager keeps this case isolated from the others.
			name:        "nil_entry_skipped_false",
			trackingKey: "anything",
			want:        false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mgr := m
			if tc.name == "nil_entry_skipped_false" {
				mgr = &Manager{activeAlerts: map[string]*Alert{"only-nil": nil}}
				if tc.trackingKey == "only-nil" {
					t.Fatal("test setup: tracking key must not be a direct map key for this case")
				}
			}
			if got := mgr.hasActiveAlertTrackingKeyNoLock(tc.trackingKey); got != tc.want {
				t.Fatalf("hasActiveAlertTrackingKeyNoLock(%q) = %v, want %v", tc.trackingKey, got, tc.want)
			}
		})
	}
}
