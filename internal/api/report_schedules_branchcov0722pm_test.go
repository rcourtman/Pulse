package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// This file raises branch/function coverage for a small set of pure or
// near-pure helpers in report_schedules.go. Each test drives concrete inputs
// and asserts concrete outputs; no tautologies and no live I/O.

func TestBranchcov0722PMHtmlEscape(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no_special_chars", "plain text 123", "plain text 123"},
		{"ampersand", "&", "&amp;"},
		{"less_than", "<", "&lt;"},
		{"greater_than", ">", "&gt;"},
		{"double_quote", "\"", "&quot;"},
		{"single_quote", "'", "&#39;"},
		{"all_specials", "a&b<c>d\"e'f", "a&amp;b&lt;c&gt;d&quot;e&#39;f"},
		// NewReplacer performs a single non-recursive pass: the '&' inserted by
		// escaping '<' must not be re-escaped, and an existing '&' is escaped.
		{"single_pass_ordering", "<&", "&lt;&amp;"},
		// Pre-existing entity text is not recognised; the literal '&' is escaped.
		{"existing_entity_not_recognised", "&amp;", "&amp;amp;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := htmlEscape(tc.input); got != tc.want {
				t.Fatalf("htmlEscape(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMParseReportScheduleWeekday(t *testing.T) {
	accepted := []struct {
		input string
		want  time.Weekday
	}{
		{"sunday", time.Sunday},
		{"sun", time.Sunday},
		{"monday", time.Monday},
		{"mon", time.Monday},
		{"tuesday", time.Tuesday},
		{"tue", time.Tuesday},
		{"wednesday", time.Wednesday},
		{"wed", time.Wednesday},
		{"thursday", time.Thursday},
		{"thu", time.Thursday},
		{"friday", time.Friday},
		{"fri", time.Friday},
		{"saturday", time.Saturday},
		{"sat", time.Saturday},
		// Case-insensitivity and surrounding whitespace are accepted.
		{"  Sunday  ", time.Sunday},
		{"MONDAY", time.Monday},
		{"FrI", time.Friday},
	}
	for _, tc := range accepted {
		t.Run("accepted/"+tc.input, func(t *testing.T) {
			got, ok := parseReportScheduleWeekday(tc.input)
			if !ok {
				t.Fatalf("parseReportScheduleWeekday(%q): ok = false, want true", tc.input)
			}
			if got != tc.want {
				t.Fatalf("parseReportScheduleWeekday(%q): weekday = %d, want %d", tc.input, got, tc.want)
			}
		})
	}

	rejected := []string{
		"",        // empty
		"funday",  // unknown full word
		"tues",    // partial abbreviation (not a valid form)
		"mondayy", // trailing characters
	}
	for _, input := range rejected {
		t.Run("rejected/"+input, func(t *testing.T) {
			got, ok := parseReportScheduleWeekday(input)
			if ok {
				t.Fatalf("parseReportScheduleWeekday(%q): ok = true, want false (returned %d)", input, got)
			}
			// On failure the function returns the zero value time.Sunday.
			if got != time.Sunday {
				t.Fatalf("parseReportScheduleWeekday(%q): returned weekday = %d, want time.Sunday(0) on failure", input, got)
			}
		})
	}
}

func TestBranchcov0722PMOccurrenceKey(t *testing.T) {
	utcOccurrence := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		cadence    config.ReportScheduleCadence
		occurrence time.Time
		want       string
	}{
		{
			name:       "populated_timezone",
			cadence:    config.ReportScheduleCadence{Type: "weekly", Timezone: "America/New_York"},
			occurrence: utcOccurrence,
			want:       "weekly:2026-07-15T09:00:00Z:America/New_York",
		},
		{
			name:       "empty_timezone_defaults_utc",
			cadence:    config.ReportScheduleCadence{Type: "monthly", Timezone: ""},
			occurrence: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			want:       "monthly:2026-01-02T03:04:05Z:UTC",
		},
		{
			name:       "whitespace_timezone_defaults_utc",
			cadence:    config.ReportScheduleCadence{Type: "monthly", Timezone: "   "},
			occurrence: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			want:       "monthly:2026-01-02T03:04:05Z:UTC",
		},
		{
			name:       "non_utc_occurrence_converted_to_utc",
			cadence:    config.ReportScheduleCadence{Type: "monthly", Timezone: "CET"},
			occurrence: time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600)),
			want:       "monthly:2026-01-02T02:04:05Z:CET",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schedule := config.ReportSchedule{Cadence: tc.cadence}
			if got := occurrenceKey(schedule, tc.occurrence); got != tc.want {
				t.Fatalf("occurrenceKey() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMLastReportScheduleOccurrenceAt(t *testing.T) {
	// All successful cases use UTC so the result is deterministic and offline.
	monthly := func(dom int, t string) config.ReportSchedule {
		return config.ReportSchedule{Cadence: config.ReportScheduleCadence{
			Type: config.ReportScheduleCadenceMonthly, DayOfMonth: dom, Time: t, Timezone: "UTC",
		}}
	}
	weekly := func(weekday string) config.ReportSchedule {
		return config.ReportSchedule{Cadence: config.ReportScheduleCadence{
			Type: config.ReportScheduleCadenceWeekly, Weekday: weekday, Time: "09:00", Timezone: "UTC",
		}}
	}

	successCases := []struct {
		name     string
		schedule config.ReportSchedule
		now      time.Time
		wantOcc  time.Time
		wantKey  string
	}{
		{
			name:     "monthly_occurrence_not_after_now_kept",
			schedule: monthly(15, "09:00"),
			now:      time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
			// 2026-07-15 09:00 is before now, so it is kept as-is.
			wantOcc: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
			wantKey: "monthly:2026-07-15T09:00:00Z:UTC",
		},
		{
			name:     "monthly_occurrence_after_now_rolls_back_month",
			schedule: monthly(15, "09:00"),
			now:      time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC),
			// 2026-07-15 09:00 is after now, so subtract one month -> 2026-06-15 09:00.
			wantOcc: time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC),
			wantKey: "monthly:2026-06-15T09:00:00Z:UTC",
		},
		{
			name:     "weekly_same_day_occurrence_not_after_now_kept",
			schedule: weekly("wednesday"),
			now:      time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC), // 2026-07-22 is a Wednesday
			// dayDelta = 0; 09:00 is before 10:00 so kept.
			wantOcc: time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC),
			wantKey: "weekly:2026-07-22T09:00:00Z:UTC",
		},
		{
			name:     "weekly_same_day_occurrence_after_now_rolls_back_week",
			schedule: weekly("wednesday"),
			now:      time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC), // Wednesday, before 09:00
			// 09:00 is after 08:00, so subtract 7 days -> 2026-07-15 09:00.
			wantOcc: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
			wantKey: "weekly:2026-07-15T09:00:00Z:UTC",
		},
		{
			name:     "weekly_different_weekday_back_dated",
			schedule: weekly("monday"),
			now:      time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC), // Wednesday
			// dayDelta = (Wed=3 - Mon=1 + 7) % 7 = 2 -> last Monday 2026-07-20 09:00 (before now, kept).
			wantOcc: time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC),
			wantKey: "weekly:2026-07-20T09:00:00Z:UTC",
		},
	}
	for _, tc := range successCases {
		t.Run(tc.name, func(t *testing.T) {
			occ, key, err := lastReportScheduleOccurrenceAt(tc.schedule, tc.now)
			if err != nil {
				t.Fatalf("lastReportScheduleOccurrenceAt() unexpected error: %v", err)
			}
			if !occ.Equal(tc.wantOcc) {
				t.Fatalf("occurrence = %v, want %v", occ, tc.wantOcc)
			}
			if key != tc.wantKey {
				t.Fatalf("key = %q, want %q", key, tc.wantKey)
			}
		})
	}

	errorCases := []struct {
		name       string
		schedule   config.ReportSchedule
		now        time.Time
		wantInErr  string
		wantZeroEq bool // whether the returned time must equal the zero time.Time
	}{
		{
			name:     "weekly_invalid_weekday",
			schedule: weekly("funday"),
			now:      time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "invalid_timezone",
			schedule: config.ReportSchedule{Cadence: config.ReportScheduleCadence{
				Type: config.ReportScheduleCadenceMonthly, DayOfMonth: 15, Time: "09:00", Timezone: "Not/A_Real_Zone",
			}},
			now: time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "invalid_clock",
			schedule: config.ReportSchedule{Cadence: config.ReportScheduleCadence{
				Type: config.ReportScheduleCadenceMonthly, DayOfMonth: 15, Time: "25:99", Timezone: "UTC",
			}},
			now: time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			name: "unsupported_cadence",
			schedule: config.ReportSchedule{Cadence: config.ReportScheduleCadence{
				Type: "daily", Time: "09:00", Timezone: "UTC",
			}},
			now:       time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC),
			wantInErr: "unsupported cadence",
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			occ, key, err := lastReportScheduleOccurrenceAt(tc.schedule, tc.now)
			if err == nil {
				t.Fatalf("lastReportScheduleOccurrenceAt() expected error, got occ=%v key=%q", occ, key)
			}
			if !occ.IsZero() {
				t.Fatalf("error path returned non-zero occurrence: %v", occ)
			}
			if key != "" {
				t.Fatalf("error path returned non-empty key: %q", key)
			}
			if tc.wantInErr != "" && !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

func TestBranchcov0722PMResourceHasAnyReportScheduleTag(t *testing.T) {
	set := func(tags ...string) map[string]struct{} {
		m := make(map[string]struct{}, len(tags))
		for _, t := range tags {
			m[t] = struct{}{}
		}
		return m
	}
	cases := []struct {
		name     string
		resource unifiedresources.Resource
		tags     map[string]struct{}
		want     bool
	}{
		{
			name:     "exact_match",
			resource: unifiedresources.Resource{Tags: []string{"prod"}},
			tags:     set("prod"),
			want:     true,
		},
		{
			name:     "uppercase_resource_tag_lowered_before_lookup",
			resource: unifiedresources.Resource{Tags: []string{"Prod"}},
			tags:     set("prod"),
			want:     true,
		},
		{
			name:     "whitespace_resource_tag_trimmed_before_lookup",
			resource: unifiedresources.Resource{Tags: []string{"  prod  "}},
			tags:     set("prod"),
			want:     true,
		},
		{
			name:     "second_resource_tag_matches",
			resource: unifiedresources.Resource{Tags: []string{"prod", "dev"}},
			tags:     set("dev"),
			want:     true,
		},
		{
			name:     "no_matching_tag",
			resource: unifiedresources.Resource{Tags: []string{"dev"}},
			tags:     set("prod"),
			want:     false,
		},
		{
			name:     "resource_with_no_tags",
			resource: unifiedresources.Resource{Tags: nil},
			tags:     set("prod"),
			want:     false,
		},
		{
			name:     "empty_tag_set",
			resource: unifiedresources.Resource{Tags: []string{"prod"}},
			tags:     set(),
			want:     false,
		},
		{
			// The function does NOT lowercase the map keys (callers are expected
			// to). An uppercase map key therefore must not match a lowercase tag.
			name:     "uppercase_map_key_does_not_match",
			resource: unifiedresources.Resource{Tags: []string{"prod"}},
			tags:     set("PROD"),
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resourceHasAnyReportScheduleTag(tc.resource, tc.tags); got != tc.want {
				t.Fatalf("resourceHasAnyReportScheduleTag() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMWriteReportScheduleValidationError(t *testing.T) {
	type errBody struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	t.Run("validation_error_type_uses_code_and_message", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeReportScheduleValidationError(rec, reportScheduleValidationError{
			code:    "invalid_name",
			message: "name must be shorter",
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		var body errBody
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Code != "invalid_name" {
			t.Fatalf("code = %q, want %q", body.Code, "invalid_name")
		}
		if body.Error != "name must be shorter" {
			t.Fatalf("error = %q, want %q", body.Error, "name must be shorter")
		}
	})

	t.Run("generic_error_falls_back_to_invalid_schedule", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeReportScheduleValidationError(rec, errors.New("something broke"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
		var body errBody
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Code != "invalid_schedule" {
			t.Fatalf("code = %q, want %q", body.Code, "invalid_schedule")
		}
		if body.Error != "something broke" {
			t.Fatalf("error = %q, want %q", body.Error, "something broke")
		}
	})
}
