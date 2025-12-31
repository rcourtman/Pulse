package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStringFromAny(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		// nil
		{"nil", nil, ""},

		// string types
		{"empty string", "", ""},
		{"simple string", "hello", "hello"},
		{"string with spaces", "  hello world  ", "hello world"},
		{"string number", "42", "42"},

		// int types
		{"int zero", int(0), "0"},
		{"int positive", int(42), "42"},
		{"int negative", int(-42), "-42"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"int32", int32(2147483647), "2147483647"},

		// uint types
		{"uint", uint(42), "42"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},
		{"uint32", uint32(4294967295), "4294967295"},

		// float types
		{"float64 integer", float64(42), "42"},
		{"float64 decimal", float64(3.14159), "3.14159"},
		{"float64 negative", float64(-1.5), "-1.5"},
		{"float64 NaN", math.NaN(), ""},
		{"float64 +Inf", math.Inf(1), ""},
		{"float64 -Inf", math.Inf(-1), ""},
		{"float32", float32(3.14), "3.14"},

		// bool
		{"bool true", true, "true"},
		{"bool false", false, "false"},

		// json.Number
		{"json.Number int", json.Number("42"), "42"},
		{"json.Number float", json.Number("3.14"), "3.14"},

		// other types (fallback to fmt.Sprint)
		{"slice", []int{1, 2, 3}, "[1 2 3]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stringFromAny(tc.input)
			if got != tc.want {
				t.Errorf("stringFromAny(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIntFromAny(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   int
		wantOk bool
	}{
		// nil
		{"nil", nil, 0, false},

		// int types
		{"int zero", int(0), 0, true},
		{"int positive", int(42), 42, true},
		{"int negative", int(-42), -42, true},
		{"int8", int8(127), 127, true},
		{"int16", int16(32767), 32767, true},
		{"int32", int32(2147483647), 2147483647, true},
		{"int64", int64(42), 42, true},

		// uint types
		{"uint", uint(42), 42, true},
		{"uint8", uint8(255), 255, true},
		{"uint16", uint16(65535), 65535, true},
		{"uint32", uint32(42), 42, true},
		{"uint64", uint64(42), 42, true},

		// float types (rounded)
		{"float32 integer", float32(42.0), 42, true},
		{"float32 round down", float32(42.4), 42, true},
		{"float32 round up", float32(42.6), 43, true},
		{"float32 NaN", float32(math.NaN()), 0, false},
		{"float32 +Inf", float32(math.Inf(1)), 0, false},
		{"float32 -Inf", float32(math.Inf(-1)), 0, false},
		{"float64 integer", float64(42.0), 42, true},
		{"float64 round half", float64(42.5), 43, true},
		{"float64 NaN", math.NaN(), 0, false},
		{"float64 +Inf", math.Inf(1), 0, false},
		{"float64 -Inf", math.Inf(-1), 0, false},

		// json.Number
		{"json.Number int", json.Number("42"), 42, true},
		{"json.Number float", json.Number("42.6"), 43, true},
		{"json.Number invalid", json.Number("abc"), 0, false},

		// string
		{"string int", "42", 42, true},
		{"string negative", "-42", -42, true},
		{"string float", "42.6", 43, true},
		{"string empty", "", 0, false},
		{"string whitespace", "  42  ", 42, true},
		{"string invalid", "abc", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := intFromAny(tc.input)
			if ok != tc.wantOk {
				t.Errorf("intFromAny(%v) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if got != tc.want {
				t.Errorf("intFromAny(%v) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestBoolFromAny(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   bool
		wantOk bool
	}{
		// nil
		{"nil", nil, false, false},

		// bool
		{"bool true", true, true, true},
		{"bool false", false, false, true},

		// int types (non-zero = true)
		{"int 0", int(0), false, true},
		{"int 1", int(1), true, true},
		{"int -1", int(-1), true, true},
		{"int64 0", int64(0), false, true},
		{"int64 1", int64(1), true, true},

		// uint types
		{"uint 0", uint(0), false, true},
		{"uint 1", uint(1), true, true},

		// float types
		{"float64 0", float64(0), false, true},
		{"float64 1", float64(1), true, true},
		{"float64 0.5", float64(0.5), true, true},

		// json.Number
		{"json.Number 0", json.Number("0"), false, true},
		{"json.Number 1", json.Number("1"), true, true},

		// string truthy values
		{"string true", "true", true, true},
		{"string TRUE", "TRUE", true, true},
		{"string yes", "yes", true, true},
		{"string YES", "YES", true, true},
		{"string 1", "1", true, true},
		{"string on", "on", true, true},
		{"string enabled", "enabled", true, true},

		// string falsy values
		{"string false", "false", false, true},
		{"string FALSE", "FALSE", false, true},
		{"string no", "no", false, true},
		{"string NO", "NO", false, true},
		{"string 0", "0", false, true},
		{"string off", "off", false, true},
		{"string disabled", "disabled", false, true},

		// string with whitespace
		{"string true with spaces", "  true  ", true, true},

		// invalid string
		{"string invalid", "maybe", false, false},
		{"string empty", "", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := boolFromAny(tc.input)
			if ok != tc.wantOk {
				t.Errorf("boolFromAny(%v) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if got != tc.want {
				t.Errorf("boolFromAny(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFloatFromAny(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		want   float64
		wantOk bool
	}{
		// nil
		{"nil", nil, 0, false},

		// float types
		{"float64 zero", float64(0), 0, true},
		{"float64 positive", float64(3.14159), 3.14159, true},
		{"float64 negative", float64(-3.14159), -3.14159, true},
		{"float64 NaN", math.NaN(), 0, false},
		{"float64 +Inf", math.Inf(1), 0, false},
		{"float64 -Inf", math.Inf(-1), 0, false},
		{"float32", float32(3.14), float64(float32(3.14)), true},
		{"float32 NaN", float32(math.NaN()), 0, false},
		{"float32 +Inf", float32(math.Inf(1)), 0, false},
		{"float32 -Inf", float32(math.Inf(-1)), 0, false},

		// int types
		{"int", int(42), 42, true},
		{"int64", int64(42), 42, true},

		// uint types
		{"uint", uint(42), 42, true},
		{"uint64", uint64(42), 42, true},

		// json.Number
		{"json.Number int", json.Number("42"), 42, true},
		{"json.Number float", json.Number("3.14159"), 3.14159, true},
		{"json.Number invalid", json.Number("abc"), 0, false},

		// string
		{"string float", "3.14159", 3.14159, true},
		{"string int", "42", 42, true},
		{"string negative", "-3.14", -3.14, true},
		{"string empty", "", 0, false},
		{"string whitespace", "  3.14  ", 3.14, true},
		{"string invalid", "abc", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := floatFromAny(tc.input)
			if ok != tc.wantOk {
				t.Errorf("floatFromAny(%v) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if ok && got != tc.want {
				t.Errorf("floatFromAny(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseReplicationTime(t *testing.T) {
	// Fixed reference time for testing
	refTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	refUnix := refTime.Unix()

	tests := []struct {
		name     string
		input    interface{}
		wantNil  bool
		wantUnix int64
	}{
		// nil
		{"nil", nil, true, 0},

		// time.Time
		{"time.Time", refTime, false, refUnix},
		{"*time.Time", &refTime, false, refUnix},
		{"*time.Time nil", (*time.Time)(nil), true, 0},

		// unix timestamps as int
		{"int timestamp", int(refUnix), false, refUnix},
		{"int64 timestamp", int64(refUnix), false, refUnix},
		{"int zero", int(0), true, 0},
		{"int negative", int(-1), true, 0},

		// unix timestamps as uint
		{"uint timestamp", uint(refUnix), false, refUnix},
		{"uint64 timestamp", uint64(refUnix), false, refUnix},

		// unix timestamps as float
		{"float64 timestamp", float64(refUnix), false, refUnix},
		{"float64 zero", float64(0), true, 0},
		{"float64 negative", float64(-1), true, 0},
		{"float32 timestamp", float32(1000000), false, 1000000}, // smaller value for float32 precision
		{"float32 zero", float32(0), true, 0},
		{"float32 negative", float32(-1), true, 0},

		// json.Number
		{"json.Number timestamp", json.Number("1736936400"), false, 1736936400},
		{"json.Number zero", json.Number("0"), true, 0},
		{"json.Number negative", json.Number("-1"), true, 0},

		// int32 and uint32
		{"int32 timestamp", int32(refUnix), false, refUnix},
		{"uint32 timestamp", uint32(refUnix), false, refUnix},

		// string unix timestamp
		{"string unix", "1736936400", false, 1736936400},
		{"string zero", "0", true, 0},
		{"string negative", "-1", true, 0},

		// string N/A values
		{"string n/a", "n/a", true, 0},
		{"string N/A", "N/A", true, 0},
		{"string pending", "pending", true, 0},
		{"string dash", "-", true, 0},
		{"string empty", "", true, 0},

		// RFC3339 format
		{"string RFC3339", "2025-01-15T10:30:00Z", false, refUnix},

		// Common date formats
		{"string date time", "2025-01-15 10:30:00", false, refUnix},
		{"string date time T", "2025-01-15T10:30:00", false, refUnix},

		// Invalid date format (not matching any layout)
		{"string invalid date", "invalid-date-format", true, 0},
		{"string partial date", "2025-01-15", true, 0},

		// Unsupported type
		{"unsupported type bool", true, true, 0},
		{"unsupported type slice", []int{1, 2, 3}, true, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, gotUnix := parseReplicationTime(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Errorf("parseReplicationTime(%v) = %v, want nil", tc.input, got)
				}
				if gotUnix != 0 {
					t.Errorf("parseReplicationTime(%v) unix = %d, want 0", tc.input, gotUnix)
				}
			} else {
				if got == nil {
					t.Errorf("parseReplicationTime(%v) = nil, want non-nil", tc.input)
				}
				if gotUnix != tc.wantUnix {
					t.Errorf("parseReplicationTime(%v) unix = %d, want %d", tc.input, gotUnix, tc.wantUnix)
				}
			}
		})
	}
}

func TestParseDurationSeconds(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantSecs  int
		wantHuman string
	}{
		// nil
		{"nil", nil, 0, ""},

		// int types
		{"int zero", int(0), 0, "0"},
		{"int positive", int(120), 120, "120"},
		{"int negative", int(-1), 0, "-1"},
		{"int64", int64(3600), 3600, "3600"},

		// uint types
		{"uint", uint(120), 120, "120"},
		{"uint64", uint64(3600), 3600, "3600"},

		// float types
		{"float64 integer", float64(120), 120, "120"},
		{"float64 decimal", float64(120.5), 121, "120.5"},
		{"float64 negative", float64(-1), 0, "-1"},

		// json.Number
		{"json.Number", json.Number("120"), 120, "120"},
		{"json.Number float", json.Number("120.5"), 121, "120.5"},

		// string numeric
		{"string int", "120", 120, "120"},
		{"string float", "120.5", 121, "120.5"},
		{"string empty", "", 0, ""},

		// string HH:MM:SS format
		{"string MM:SS", "02:30", 150, "02:30"},
		{"string HH:MM:SS", "01:02:30", 3750, "01:02:30"},
		{"string HH:MM:SS zeros", "00:00:30", 30, "00:00:30"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSecs, gotHuman := parseDurationSeconds(tc.input)
			if gotSecs != tc.wantSecs {
				t.Errorf("parseDurationSeconds(%v) secs = %d, want %d", tc.input, gotSecs, tc.wantSecs)
			}
			if gotHuman != tc.wantHuman {
				t.Errorf("parseDurationSeconds(%v) human = %q, want %q", tc.input, gotHuman, tc.wantHuman)
			}
		})
	}
}

func TestParseHHMMSSToSeconds(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int
		wantOk bool
	}{
		// valid MM:SS
		{"MM:SS zeros", "00:00", 0, true},
		{"MM:SS simple", "02:30", 150, true},
		{"MM:SS max minutes", "59:59", 3599, true},

		// valid HH:MM:SS
		{"HH:MM:SS zeros", "00:00:00", 0, true},
		{"HH:MM:SS simple", "01:02:30", 3750, true},
		{"HH:MM:SS one hour", "01:00:00", 3600, true},
		{"HH:MM:SS large", "24:00:00", 86400, true},

		// invalid formats
		{"single value", "30", 0, false},
		{"too many parts", "01:02:03:04", 0, false},
		{"empty part", "01::30", 0, false},
		{"invalid number", "01:ab:30", 0, false},
		{"empty string", "", 0, false},

		// whitespace handling
		{"spaces in parts", " 01 : 02 : 30 ", 3750, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseHHMMSSToSeconds(tc.input)
			if ok != tc.wantOk {
				t.Errorf("parseHHMMSSToSeconds(%q) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if got != tc.want {
				t.Errorf("parseHHMMSSToSeconds(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestDecodeRaw(t *testing.T) {
	tests := []struct {
		name  string
		input json.RawMessage
		want  interface{}
	}{
		{"nil", nil, nil},
		{"string", json.RawMessage(`"hello"`), "hello"},
		{"number", json.RawMessage(`42`), float64(42)},
		{"bool true", json.RawMessage(`true`), true},
		{"bool false", json.RawMessage(`false`), false},
		{"null", json.RawMessage(`null`), nil},
		{"invalid json", json.RawMessage(`invalid`), nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeRaw(tc.input)
			if got != tc.want {
				t.Errorf("decodeRaw(%s) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestFirstNonNilRaw(t *testing.T) {
	entry := map[string]json.RawMessage{
		"key1": nil,
		"key2": json.RawMessage(`"value2"`),
		"key3": json.RawMessage(`"value3"`),
	}

	tests := []struct {
		name string
		keys []string
		want interface{}
	}{
		{"first key nil, second exists", []string{"key1", "key2"}, "value2"},
		{"first key exists", []string{"key2", "key3"}, "value2"},
		{"only last key exists", []string{"key1", "nonexistent", "key3"}, "value3"},
		{"no keys exist", []string{"nonexistent1", "nonexistent2"}, nil},
		{"empty keys", []string{}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := firstNonNilRaw(entry, tc.keys...)
			if got != tc.want {
				t.Errorf("firstNonNilRaw(%v) = %v, want %v", tc.keys, got, tc.want)
			}
		})
	}
}

func TestCopyFloat(t *testing.T) {
	val := 3.14159
	got := copyFloat(val)

	if got == nil {
		t.Fatal("copyFloat returned nil")
	}
	if *got != val {
		t.Errorf("copyFloat(%v) = %v, want %v", val, *got, val)
	}

	// Verify it's a copy
	val = 2.71828
	if *got == val {
		t.Error("copyFloat did not create a copy")
	}
}

func TestParseReplicationJob(t *testing.T) {
	entry := map[string]json.RawMessage{
		"id":         json.RawMessage(`"100-0"`),
		"guest":      json.RawMessage(`100`),
		"source":     json.RawMessage(`"node1"`),
		"target":     json.RawMessage(`"node2"`),
		"schedule":   json.RawMessage(`"*/15"`),
		"type":       json.RawMessage(`"local"`),
		"state":      json.RawMessage(`"ok"`),
		"enabled":    json.RawMessage(`1`),
		"last_sync":  json.RawMessage(`1736936400`),
		"fail_count": json.RawMessage(`0`),
		"rate":       json.RawMessage(`10.5`),
	}

	job := parseReplicationJob(entry)

	if job.ID != "100-0" {
		t.Errorf("ID = %q, want %q", job.ID, "100-0")
	}
	if job.GuestID != 100 {
		t.Errorf("GuestID = %d, want %d", job.GuestID, 100)
	}
	if job.JobNumber != 0 {
		t.Errorf("JobNumber = %d, want %d", job.JobNumber, 0)
	}
	if job.Source != "node1" {
		t.Errorf("Source = %q, want %q", job.Source, "node1")
	}
	if job.Target != "node2" {
		t.Errorf("Target = %q, want %q", job.Target, "node2")
	}
	if job.Schedule != "*/15" {
		t.Errorf("Schedule = %q, want %q", job.Schedule, "*/15")
	}
	if job.Type != "local" {
		t.Errorf("Type = %q, want %q", job.Type, "local")
	}
	if !job.Enabled {
		t.Error("Enabled should be true")
	}
	if job.State != "ok" {
		t.Errorf("State = %q, want %q", job.State, "ok")
	}
	if job.FailCount != 0 {
		t.Errorf("FailCount = %d, want %d", job.FailCount, 0)
	}
	if job.RateLimitMbps == nil || *job.RateLimitMbps != 10.5 {
		t.Errorf("RateLimitMbps = %v, want 10.5", job.RateLimitMbps)
	}
	if job.LastSyncUnix != 1736936400 {
		t.Errorf("LastSyncUnix = %d, want %d", job.LastSyncUnix, 1736936400)
	}
}

func TestParseReplicationJob_Disabled(t *testing.T) {
	// Test disabled via "disable" field
	entry := map[string]json.RawMessage{
		"id":      json.RawMessage(`"100-0"`),
		"disable": json.RawMessage(`true`),
	}

	job := parseReplicationJob(entry)
	if job.Enabled {
		t.Error("Job should be disabled via 'disable' field")
	}

	// Test disabled via "active" field
	entry = map[string]json.RawMessage{
		"id":     json.RawMessage(`"100-0"`),
		"active": json.RawMessage(`false`),
	}

	job = parseReplicationJob(entry)
	if job.Enabled {
		t.Error("Job should be disabled via 'active' field")
	}
}

func TestParseReplicationJob_AlternateFieldNames(t *testing.T) {
	// Test alternate field names like source-storage vs source_storage
	entry := map[string]json.RawMessage{
		"id":             json.RawMessage(`"100-0"`),
		"source-storage": json.RawMessage(`"local-zfs"`),
		"target-storage": json.RawMessage(`"remote-zfs"`),
		"last-sync":      json.RawMessage(`1736936400`),
		"fail-count":     json.RawMessage(`2`),
	}

	job := parseReplicationJob(entry)

	if job.SourceStorage != "local-zfs" {
		t.Errorf("SourceStorage = %q, want %q", job.SourceStorage, "local-zfs")
	}
	if job.TargetStorage != "remote-zfs" {
		t.Errorf("TargetStorage = %q, want %q", job.TargetStorage, "remote-zfs")
	}
	if job.FailCount != 2 {
		t.Errorf("FailCount = %d, want %d", job.FailCount, 2)
	}
	if job.LastSyncUnix != 1736936400 {
		t.Errorf("LastSyncUnix = %d, want %d", job.LastSyncUnix, 1736936400)
	}
}

func TestParseReplicationJob_JobNumberFromID(t *testing.T) {
	// Test parsing job number from ID when jobnum field is missing
	entry := map[string]json.RawMessage{
		"id": json.RawMessage(`"100-5"`),
	}

	job := parseReplicationJob(entry)

	if job.JobNumber != 5 {
		t.Errorf("JobNumber = %d, want %d (parsed from ID)", job.JobNumber, 5)
	}
}

func TestParseReplicationJob_StatusFallback(t *testing.T) {
	// Test status fallback from state when status is empty
	entry := map[string]json.RawMessage{
		"id":    json.RawMessage(`"100-0"`),
		"state": json.RawMessage(`"syncing"`),
	}

	job := parseReplicationJob(entry)

	if job.Status != "syncing" {
		t.Errorf("Status = %q, want %q (from state)", job.Status, "syncing")
	}
}

func TestParseReplicationJob_JobIDFallback(t *testing.T) {
	// Test fallback from "id" to "jobid" field when id is missing
	entry := map[string]json.RawMessage{
		"jobid": json.RawMessage(`"200-1"`),
		"guest": json.RawMessage(`200`),
	}

	job := parseReplicationJob(entry)

	if job.ID != "200-1" {
		t.Errorf("ID = %q, want %q (from jobid fallback)", job.ID, "200-1")
	}
	if job.JobNumber != 1 {
		t.Errorf("JobNumber = %d, want %d (parsed from ID)", job.JobNumber, 1)
	}
}

func TestParseReplicationJob_JobNumField(t *testing.T) {
	// Test jobnum field takes precedence over parsing from ID
	entry := map[string]json.RawMessage{
		"id":     json.RawMessage(`"100-0"`),
		"jobnum": json.RawMessage(`5`),
	}

	job := parseReplicationJob(entry)

	if job.JobNumber != 5 {
		t.Errorf("JobNumber = %d, want %d (from jobnum field)", job.JobNumber, 5)
	}
}

func TestParseReplicationJob_DurationFields(t *testing.T) {
	// Test last_sync_duration field
	entry := map[string]json.RawMessage{
		"id":                 json.RawMessage(`"100-0"`),
		"last_sync_duration": json.RawMessage(`120`),
		"duration":           json.RawMessage(`60`),
	}

	job := parseReplicationJob(entry)

	if job.LastSyncDurationSeconds != 120 {
		t.Errorf("LastSyncDurationSeconds = %d, want %d", job.LastSyncDurationSeconds, 120)
	}
	if job.DurationSeconds != 60 {
		t.Errorf("DurationSeconds = %d, want %d", job.DurationSeconds, 60)
	}
}

func TestParseReplicationJob_DurationFallback(t *testing.T) {
	// Test fallback to last-sync-duration when last_sync_duration is missing
	entry := map[string]json.RawMessage{
		"id":                 json.RawMessage(`"100-0"`),
		"last-sync-duration": json.RawMessage(`90`),
	}

	job := parseReplicationJob(entry)

	if job.LastSyncDurationSeconds != 90 {
		t.Errorf("LastSyncDurationSeconds = %d, want %d (from last-sync-duration fallback)", job.LastSyncDurationSeconds, 90)
	}
}

func TestParseReplicationJob_NextSyncFields(t *testing.T) {
	// Test next_sync field
	entry := map[string]json.RawMessage{
		"id":        json.RawMessage(`"100-0"`),
		"next_sync": json.RawMessage(`1736936500`),
	}

	job := parseReplicationJob(entry)

	if job.NextSyncUnix != 1736936500 {
		t.Errorf("NextSyncUnix = %d, want %d", job.NextSyncUnix, 1736936500)
	}
	if job.NextSyncTime == nil {
		t.Error("NextSyncTime should not be nil")
	}
}

func TestParseReplicationJob_NextSyncFallback(t *testing.T) {
	// Test fallback to next-sync when next_sync is missing
	entry := map[string]json.RawMessage{
		"id":        json.RawMessage(`"100-0"`),
		"next-sync": json.RawMessage(`1736936600`),
	}

	job := parseReplicationJob(entry)

	if job.NextSyncUnix != 1736936600 {
		t.Errorf("NextSyncUnix = %d, want %d (from next-sync fallback)", job.NextSyncUnix, 1736936600)
	}
}

// TestGetReplicationStatus_EnrichesWithStatusData tests that GetReplicationStatus
// fetches job config from /cluster/replication AND enriches with status data from
// /nodes/{node}/replication/{id}/status. This tests the fix for issue #992.
func TestGetReplicationStatus_EnrichesWithStatusData(t *testing.T) {
	// Track which endpoints were called
	var calledClusterReplication bool
	var calledStatusEndpoint bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api2/json/cluster/replication":
			calledClusterReplication = true
			// Return job config (this is what /cluster/replication returns - NO status data)
			fmt.Fprint(w, `{
				"data": [
					{
						"id": "100-0",
						"guest": 100,
						"source": "pve1",
						"target": "pve2",
						"schedule": "*/15",
						"type": "local"
					}
				]
			}`)

		case "/api2/json/nodes/pve1/replication/100-0/status":
			calledStatusEndpoint = true
			// Return status data (this is what the per-node endpoint returns)
			fmt.Fprint(w, `{
				"data": [
					{
						"last_sync": 1735689600,
						"next_sync": 1735690500,
						"duration": 120,
						"fail_count": 0,
						"state": "ok"
					}
				]
			}`)

		default:
			t.Logf("Unexpected request to: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "test@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	jobs, err := client.GetReplicationStatus(ctx)
	if err != nil {
		t.Fatalf("GetReplicationStatus failed: %v", err)
	}

	// Verify both endpoints were called
	if !calledClusterReplication {
		t.Error("Expected /cluster/replication to be called")
	}
	if !calledStatusEndpoint {
		t.Error("Expected /nodes/pve1/replication/100-0/status to be called")
	}

	// Verify we got one job
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]

	// Verify config fields came from /cluster/replication
	if job.ID != "100-0" {
		t.Errorf("ID = %q, want %q", job.ID, "100-0")
	}
	if job.GuestID != 100 {
		t.Errorf("GuestID = %d, want %d", job.GuestID, 100)
	}
	if job.Source != "pve1" {
		t.Errorf("Source = %q, want %q", job.Source, "pve1")
	}
	if job.Target != "pve2" {
		t.Errorf("Target = %q, want %q", job.Target, "pve2")
	}
	if job.Schedule != "*/15" {
		t.Errorf("Schedule = %q, want %q", job.Schedule, "*/15")
	}

	// Verify status fields came from /nodes/{node}/replication/{id}/status
	if job.LastSyncUnix != 1735689600 {
		t.Errorf("LastSyncUnix = %d, want %d", job.LastSyncUnix, 1735689600)
	}
	if job.LastSyncTime == nil {
		t.Error("LastSyncTime should not be nil")
	}
	if job.NextSyncUnix != 1735690500 {
		t.Errorf("NextSyncUnix = %d, want %d", job.NextSyncUnix, 1735690500)
	}
	if job.NextSyncTime == nil {
		t.Error("NextSyncTime should not be nil")
	}
	if job.DurationSeconds != 120 {
		t.Errorf("DurationSeconds = %d, want %d", job.DurationSeconds, 120)
	}
	if job.LastSyncDurationSeconds != 120 {
		t.Errorf("LastSyncDurationSeconds = %d, want %d", job.LastSyncDurationSeconds, 120)
	}
	if job.FailCount != 0 {
		t.Errorf("FailCount = %d, want %d", job.FailCount, 0)
	}
	if job.State != "ok" {
		t.Errorf("State = %q, want %q", job.State, "ok")
	}
}

// TestGetReplicationStatus_StatusEndpointFails tests that GetReplicationStatus
// still returns job config even if the per-node status endpoint fails.
func TestGetReplicationStatus_StatusEndpointFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api2/json/cluster/replication":
			// Return job config
			fmt.Fprint(w, `{
				"data": [
					{
						"id": "100-0",
						"guest": 100,
						"source": "pve1",
						"target": "pve2",
						"schedule": "*/15",
						"type": "local"
					}
				]
			}`)

		case "/api2/json/nodes/pve1/replication/100-0/status":
			// Status endpoint fails (404)
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors": "not found"}`)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "test@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()
	jobs, err := client.GetReplicationStatus(ctx)
	if err != nil {
		t.Fatalf("GetReplicationStatus failed: %v", err)
	}

	// Should still get the job config even though status failed
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]

	// Config fields should be populated
	if job.ID != "100-0" {
		t.Errorf("ID = %q, want %q", job.ID, "100-0")
	}
	if job.GuestID != 100 {
		t.Errorf("GuestID = %d, want %d", job.GuestID, 100)
	}

	// Status fields should be empty/zero (status endpoint failed)
	if job.LastSyncUnix != 0 {
		t.Errorf("LastSyncUnix = %d, want 0 (status endpoint failed)", job.LastSyncUnix)
	}
	if job.LastSyncTime != nil {
		t.Error("LastSyncTime should be nil (status endpoint failed)")
	}
}
