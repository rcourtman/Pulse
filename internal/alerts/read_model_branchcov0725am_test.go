package alerts

import (
	"encoding/json"
	"testing"
)

// TestBranchcov0725amMetadataBoolValue raises statement coverage of
// metadataBoolValue (read_model.go:196). The existing suite only reaches it
// indirectly through the live sort path, leaving most type-switch arms
// unmeasured (~25%). These cases drive, in order:
//   - nil metadata map (short-circuit return false)
//   - populated map but missing key (return false)
//   - present key whose value is nil interface (falls through, return false)
//   - bool true / bool false
//   - every accepted truthy string token ("1","true","yes","on"), case- and
//     whitespace-insensitive, plus non-truthy strings that must fall through
//   - every signed/unsigned int type: nonzero -> true, zero -> false
//   - float32/float64 nonzero -> true, zero -> false
//   - json.Number that parses to a nonzero/zero int
//   - json.Number that fails to parse (falls through, return false)
//   - an unsupported type ([]string) -> default return false
//
// Each case carries a distinct key/value so the assertion pins exactly which
// arm produced the result.
func TestBranchcov0725amMetadataBoolValue(t *testing.T) {
	cases := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		want     bool
	}{
		{"nil_metadata_returns_false", nil, "protectionReduced", false},
		{"missing_key_returns_false", map[string]interface{}{"other": true}, "protectionReduced", false},
		{"nil_value_returns_false", map[string]interface{}{"k": nil}, "k", false},
		{"bool_true", map[string]interface{}{"k": true}, "k", true},
		{"bool_false", map[string]interface{}{"k": false}, "k", false},

		{"string_one_is_true", map[string]interface{}{"k": "1"}, "k", true},
		{"string_true_is_true", map[string]interface{}{"k": "true"}, "k", true},
		{"string_yes_is_true", map[string]interface{}{"k": "yes"}, "k", true},
		{"string_on_is_true", map[string]interface{}{"k": "on"}, "k", true},
		{"string_TRUE_uppercase_is_true", map[string]interface{}{"k": "TRUE"}, "k", true},
		{"string_YES_mixedcase_is_true", map[string]interface{}{"k": "YeS"}, "k", true},
		{"string_padded_true_is_true", map[string]interface{}{"k": "  true  "}, "k", true},
		{"string_zero_is_false", map[string]interface{}{"k": "0"}, "k", false},
		{"string_false_is_false", map[string]interface{}{"k": "false"}, "k", false},
		{"string_no_is_false", map[string]interface{}{"k": "no"}, "k", false},
		{"string_off_is_false", map[string]interface{}{"k": "off"}, "k", false},
		{"string_unknown_is_false", map[string]interface{}{"k": "maybe"}, "k", false},
		{"string_empty_is_false", map[string]interface{}{"k": ""}, "k", false},

		{"int_zero_is_false", map[string]interface{}{"k": 0}, "k", false},
		{"int_nonzero_is_true", map[string]interface{}{"k": 5}, "k", true},
		{"int_negative_is_true", map[string]interface{}{"k": -3}, "k", true},
		{"int8_nonzero_is_true", map[string]interface{}{"k": int8(2)}, "k", true},
		{"int8_zero_is_false", map[string]interface{}{"k": int8(0)}, "k", false},
		{"int16_nonzero_is_true", map[string]interface{}{"k": int16(7)}, "k", true},
		{"int32_nonzero_is_true", map[string]interface{}{"k": int32(7)}, "k", true},
		{"int64_nonzero_is_true", map[string]interface{}{"k": int64(9)}, "k", true},
		{"uint_nonzero_is_true", map[string]interface{}{"k": uint(4)}, "k", true},
		{"uint8_nonzero_is_true", map[string]interface{}{"k": uint8(4)}, "k", true},
		{"uint16_nonzero_is_true", map[string]interface{}{"k": uint16(4)}, "k", true},
		{"uint32_nonzero_is_true", map[string]interface{}{"k": uint32(4)}, "k", true},
		{"uint64_nonzero_is_true", map[string]interface{}{"k": uint64(4)}, "k", true},
		{"uint_zero_is_false", map[string]interface{}{"k": uint(0)}, "k", false},

		{"float32_nonzero_is_true", map[string]interface{}{"k": float32(1.5)}, "k", true},
		{"float32_zero_is_false", map[string]interface{}{"k": float32(0)}, "k", false},
		{"float64_nonzero_is_true", map[string]interface{}{"k": float64(2.5)}, "k", true},
		{"float64_zero_is_false", map[string]interface{}{"k": float64(0)}, "k", false},

		{"json_number_one_is_true", map[string]interface{}{"k": json.Number("1")}, "k", true},
		{"json_number_zero_is_false", map[string]interface{}{"k": json.Number("0")}, "k", false},
		{"json_number_unparseable_is_false", map[string]interface{}{"k": json.Number("not-a-number")}, "k", false},

		{"unsupported_type_is_false", map[string]interface{}{"k": []string{"true"}}, "k", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := metadataBoolValue(tc.metadata, tc.key); got != tc.want {
				t.Fatalf("metadataBoolValue(%v, %q) = %v, want %v", tc.metadata, tc.key, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amMetadataIntValue raises statement coverage of
// metadataIntValue (read_model.go:127). The existing suite only exercises the
// int and the nil/miss paths indirectly (~27.8%). These cases drive every
// accepted underlying type (all signed/unsigned int widths, float32/float64,
// json.Number, string), the truncation behaviour of floats, the
// trim-then-Atoi behaviour of strings, the json.Number Int64() success and
// failure paths (including an overflow), and the wrong-type/nil fall-through
// to 0. Each numeric value is distinct so the assertion pins exactly which
// arm produced it.
func TestBranchcov0725amMetadataIntValue(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int
	}{
		{"nil_returns_zero", nil, 0},
		{"int", 42, 42},
		{"int_negative", -17, -17},
		{"int8", int8(8), 8},
		{"int16", int16(1600), 1600},
		{"int32", int32(320000), 320000},
		{"int64", int64(6400000), 6400000},
		{"uint", uint(13), 13},
		{"uint8", uint8(200), 200},
		{"uint16", uint16(40000), 40000},
		{"uint32", uint32(500000), 500000},
		{"uint64", uint64(9000000), 9000000},
		{"float32_truncates", float32(3.9), 3},
		{"float32_negative_truncates", float32(-2.7), -2},
		{"float64_truncates", float64(99.999), 99},
		{"float64_negative_truncates", float64(-1.1), -1},
		{"json_number_int", json.Number("42"), 42},
		{"json_number_negative_int", json.Number("-5"), -5},
		// json.Number holding a float is NOT a valid int64 -> Int64() errors -> 0.
		{"json_number_float_is_zero", json.Number("3.9"), 0},
		// json.Number overflowing int64 -> Int64() errors -> 0.
		{"json_number_overflow_is_zero", json.Number("99999999999999999999"), 0},
		{"json_number_unparseable_is_zero", json.Number("abc"), 0},
		{"string_int", "42", 42},
		{"string_negative_int", "-9", -9},
		{"string_padded_int_trimmed", "   77   ", 77},
		// A string holding a float is rejected by Atoi -> 0.
		{"string_float_is_zero", "3.9", 0},
		{"string_non_numeric_is_zero", "abc", 0},
		{"string_empty_is_zero", "", 0},
		{"unsupported_type_is_zero", []int{1, 2}, 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := metadataIntValue(tc.value); got != tc.want {
				t.Fatalf("metadataIntValue(%T(%v)) = %d, want %d", tc.value, tc.value, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amAlertTypeSortRank raises statement coverage of
// alertTypeSortRank (read_model.go:108). The existing suite hits a subset of
// types through live evaluation (~37.5%). These cases drive every named
// switch arm (including each member of the multi-value case clauses) and the
// default/fallback arm, pinning the exact rank for every input class. Because
// the sort comparator orders by descending rank, the exact rank fully
// determines ordering.
func TestBranchcov0725amAlertTypeSortRank(t *testing.T) {
	cases := []struct {
		name     string
		alertTpe string
		want     int
	}{
		{"backup_posture_incident_is_6", "backup-posture-incident", 6},
		{"backup_storage_incident_is_5", "backup-storage-incident", 5},
		{"storage_incident_is_4", "storage-incident", 4},
		{"zfs_pool_state_is_4", "zfs-pool-state", 4},
		{"zfs_pool_errors_is_4", "zfs-pool-errors", 4},
		{"resource_incident_is_4", "resource-incident", 4},
		{"disk_health_is_3", "disk-health", 3},
		{"disk_wearout_is_3", "disk-wearout", 3},
		{"zfs_device_is_3", "zfs-device", 3},
		{"offline_is_2", "offline", 2},
		{"connectivity_is_2", "connectivity", 2},
		{"powered_off_is_2", "powered-off", 2},
		{"docker_host_offline_is_2", "docker-host-offline", 2},
		{"unknown_type_default_is_1", "some-other-type", 1},
		{"empty_type_default_is_1", "", 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := alertTypeSortRank(Alert{Type: tc.alertTpe})
			if got != tc.want {
				t.Fatalf("alertTypeSortRank(Type=%q) = %d, want %d", tc.alertTpe, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amAlertProtectionSortRank raises statement coverage of
// alertProtectionSortRank (read_model.go:62). The existing suite reaches ~50%.
// These cases drive all three switch arms: protectionReduced -> 2,
// rebuildInProgress -> 1, and the default -> 0, plus the nil-metadata and
// missing-key fall-throughs. Crucially, when protectionReduced is true the
// function must return 2 even if rebuildInProgress is also set (first-match
// precedence), which the precedence case pins.
func TestBranchcov0725amAlertProtectionSortRank(t *testing.T) {
	cases := []struct {
		name string
		meta map[string]interface{}
		want int
	}{
		{"nil_metadata_is_0", nil, 0},
		{"empty_metadata_is_0", map[string]interface{}{}, 0},
		{"protection_reduced_is_2", map[string]interface{}{"protectionReduced": true}, 2},
		{"rebuild_in_progress_is_1", map[string]interface{}{"rebuildInProgress": true}, 1},
		{"neither_flag_is_0", map[string]interface{}{"unrelated": true}, 0},
		{"protection_reduced_beats_rebuild", map[string]interface{}{"protectionReduced": true, "rebuildInProgress": true}, 2},
		// Falsy values must not trigger the flag arms.
		{"protection_reduced_false_is_0", map[string]interface{}{"protectionReduced": false}, 0},
		{"rebuild_in_progress_false_is_0", map[string]interface{}{"rebuildInProgress": false}, 0},
		{"protection_reduced_string_truthy_is_2", map[string]interface{}{"protectionReduced": "yes"}, 2},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := alertProtectionSortRank(Alert{Metadata: tc.meta})
			if got != tc.want {
				t.Fatalf("alertProtectionSortRank(%v) = %d, want %d", tc.meta, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amAlertRecoverabilitySortRank raises statement coverage of
// alertRecoverabilitySortRank (read_model.go:91). The existing suite reaches
// ~57.1%. The function is a top-down switch over conjuncts of metadataBoolValue
// and metadataIntValue, so each arm requires a precise combination of flags to
// defeat the earlier cases. These cases drive all six arms:
//  1. backupTarget && protectedWorkloadCount>0          -> 2
//  2. backupServer && protectedWorkloadCount>0          -> 2
//  3. backupTarget (protectedWorkloadCount<=0)          -> 1
//  4. backupServer && affectedDatastoreCount>0          -> 1
//  5. backupServer (neither count>0)                    -> 1
//  6. default (no backup role)                          -> 0
//
// plus the nil-metadata and boundary (count == 0) cases that prove the
// `> 0` thresholds are exclusive at zero.
func TestBranchcov0725amAlertRecoverabilitySortRank(t *testing.T) {
	cases := []struct {
		name string
		meta map[string]interface{}
		want int
	}{
		{"nil_metadata_is_0", nil, 0},
		{"empty_metadata_is_0", map[string]interface{}{}, 0},
		{
			"arm1_backup_target_with_protected_workloads_is_2",
			map[string]interface{}{"backupTarget": true, "protectedWorkloadCount": 3},
			2,
		},
		{
			"arm2_backup_server_with_protected_workloads_is_2",
			map[string]interface{}{"backupServer": true, "protectedWorkloadCount": 2},
			2,
		},
		{
			"arm3_backup_target_no_protected_workloads_is_1",
			map[string]interface{}{"backupTarget": true, "protectedWorkloadCount": 0},
			1,
		},
		{
			"arm3_backup_target_missing_count_is_1",
			map[string]interface{}{"backupTarget": true},
			1,
		},
		{
			"arm4_backup_server_with_affected_datastores_is_1",
			map[string]interface{}{"backupServer": true, "affectedDatastoreCount": 4},
			1,
		},
		{
			"arm5_backup_server_no_counts_is_1",
			map[string]interface{}{"backupServer": true},
			1,
		},
		{
			"default_no_backup_role_is_0",
			map[string]interface{}{"unrelated": true},
			0,
		},
		{
			// backupTarget must defeat backupServer precedence: when both are
			// set with protected workloads, arm 1 (rank 2) wins over arm 2.
			"backup_target_beats_backup_server_when_both_set",
			map[string]interface{}{"backupTarget": true, "backupServer": true, "protectedWorkloadCount": 5},
			2,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := alertRecoverabilitySortRank(Alert{Metadata: tc.meta})
			if got != tc.want {
				t.Fatalf("alertRecoverabilitySortRank(%v) = %d, want %d", tc.meta, got, tc.want)
			}
		})
	}
}

// TestBranchcov0725amNumericConditionValue raises statement coverage of
// numericConditionValue (filter_evaluation.go:135). The existing suite only
// reaches it indirectly through evaluateVMCondition, exercising float64, int,
// and the string (default) path (~22.2%). These cases drive every accepted
// numeric type arm, the json.Number Float64() success and failure paths
// (including an overflow that errors), and the default/failure arms (nil and
// an unsupported type). Because the function returns (float64, bool), each
// case asserts BOTH the converted value and the ok flag, using distinct
// magnitudes so the assertion pins the producing arm.
func TestBranchcov0725amNumericConditionValue(t *testing.T) {
	cases := []struct {
		name    string
		raw     any
		wantVal float64
		wantOk  bool
	}{
		{"float64", 12.5, 12.5, true},
		{"float64_negative", -3.25, -3.25, true},
		{"float64_zero", 0.0, 0.0, true},
		{"float32", float32(7.5), 7.5, true},
		{"int", 42, 42, true},
		{"int_negative", -9, -9, true},
		{"int8", int8(8), 8, true},
		{"int16", int16(1600), 1600, true},
		{"int32", int32(320000), 320000, true},
		{"int64", int64(6400000), 6400000, true},
		{"uint", uint(13), 13, true},
		{"uint8", uint8(200), 200, true},
		{"uint16", uint16(40000), 40000, true},
		{"uint32", uint32(500000), 500000, true},
		{"uint64", uint64(9000000), 9000000, true},
		{"json_number_int", json.Number("42"), 42, true},
		{"json_number_float", json.Number("3.5"), 3.5, true},
		{"json_number_negative", json.Number("-2.25"), -2.25, true},
		// Overflow: 1e400 is not representable as float64 -> ParseFloat errors -> ok false.
		{"json_number_overflow_is_false", json.Number("1e400"), 0, false},
		// Non-numeric text -> ParseFloat errors -> ok false.
		{"json_number_unparseable_is_false", json.Number("not-a-number"), 0, false},
		// nil hits the default arm -> ok false.
		{"nil_is_false", nil, 0, false},
		// string is NOT accepted (unlike metadataIntValue) -> default arm -> ok false.
		{"string_is_false", "42", 0, false},
		// Unsupported type -> default arm -> ok false.
		{"slice_is_false", []int{1}, 0, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotOk := numericConditionValue(tc.raw)
			if gotOk != tc.wantOk {
				t.Fatalf("numericConditionValue(%T(%v)) ok = %v, want %v", tc.raw, tc.raw, gotOk, tc.wantOk)
			}
			if gotOk && gotVal != tc.wantVal {
				t.Fatalf("numericConditionValue(%T(%v)) = %v, want %v", tc.raw, tc.raw, gotVal, tc.wantVal)
			}
			// When ok is false the implementation returns 0; pin that too.
			if !tc.wantOk && gotVal != 0 {
				t.Fatalf("numericConditionValue(%T(%v)) val = %v on failure, want 0", tc.raw, tc.raw, gotVal)
			}
		})
	}
}
