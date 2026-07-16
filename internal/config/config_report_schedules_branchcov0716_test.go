package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file adds branch coverage for the report-schedule helpers in
// report_schedules.go: EmptyReportScheduleStore, NormalizeReportScheduleStore,
// NormalizeReportSchedule, normalizeReportScheduleStringSlice, and
// parseReportScheduleStoreJSON. Test functions use the BranchCov prefix so
// they can be selected with `-run BranchCov`.

// TestBranchCovEmptyReportScheduleStore verifies that EmptyReportScheduleStore
// returns a store whose Schedules slice is allocated (non-nil) and empty. The
// nil-vs-empty distinction is observable: json.Marshal renders a nil slice as
// `null` and an empty slice as `[]`.
func TestBranchCovEmptyReportScheduleStore(t *testing.T) {
	store := EmptyReportScheduleStore()
	require.NotNil(t, store.Schedules, "Schedules must be non-nil so it serializes as [] not null")
	assert.Len(t, store.Schedules, 0)

	encoded, err := json.Marshal(store)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"schedules":[]`)
	assert.NotContains(t, string(encoded), `"schedules":null`)
}

// TestBranchCovNormalizeReportScheduleStore covers both arms of the
// nil-Schedules guard (true and false) and exercises the per-element
// normalization loop with zero and multiple entries.
func TestBranchCovNormalizeReportScheduleStore(t *testing.T) {
	t.Run("nil schedules is replaced with a non-nil empty slice", func(t *testing.T) {
		got := NormalizeReportScheduleStore(ReportScheduleStore{Schedules: nil})
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("empty non-nil schedules stays empty and non-nil", func(t *testing.T) {
		got := NormalizeReportScheduleStore(ReportScheduleStore{Schedules: []ReportSchedule{}})
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("each schedule is normalized in place (loop runs for every index)", func(t *testing.T) {
		in := ReportScheduleStore{Schedules: []ReportSchedule{
			{ID: "  A  ", Name: "  First "},
			{ID: "\tB\t", Name: "Second "},
		}}
		got := NormalizeReportScheduleStore(in)
		require.Len(t, got.Schedules, 2)
		assert.Equal(t, "A", got.Schedules[0].ID)
		assert.Equal(t, "First", got.Schedules[0].Name)
		assert.Equal(t, "B", got.Schedules[1].ID)
		assert.Equal(t, "Second", got.Schedules[1].Name)
		// The shared backing array is mutated, so the caller's slice reflects
		// the normalized values too.
		assert.Equal(t, "A", in.Schedules[0].ID)
		assert.Equal(t, "B", in.Schedules[1].ID)
	})
}

// TestBranchCovNormalizeReportSchedule exercises every branch and
// transformation of NormalizeReportSchedule: whitespace trimming, the
// lowercased-vs-trim-only distinction between fields, both arms of the
// RetentionCount<=0 guard (including the boundary at 1), both arms of the nil
// Resources guard, and the per-resource normalization loop.
func TestBranchCovNormalizeReportSchedule(t *testing.T) {
	t.Run("strings are trimmed and only the designated fields are lowercased", func(t *testing.T) {
		in := ReportSchedule{
			ID:   "  Rep-1  ",
			Name: "  Monthly Report  ",
			Cadence: ReportScheduleCadence{
				Type:       "  MONTHLY  ",
				DayOfMonth: 1,
				Weekday:    "  MON  ",
				Time:       "  08:00  ",
				Timezone:   "  America/New_York  ",
			},
			Window:            "  last_month  ",
			Format:            "  PDF  ",
			Delivery:          ReportScheduleDelivery{Method: "  EMAIL  "},
			LastRunStatus:     "  OK  ",
			LastError:         "  boom  ",
			LastOccurrenceKey: "  key-1  ",
			RetentionCount:    5,
		}
		got := NormalizeReportSchedule(in)

		// Trimmed-only fields preserve original case.
		assert.Equal(t, "Rep-1", got.ID)
		assert.Equal(t, "Monthly Report", got.Name)
		assert.Equal(t, "08:00", got.Cadence.Time)
		assert.Equal(t, "America/New_York", got.Cadence.Timezone)
		assert.Equal(t, "last_month", got.Window)
		assert.Equal(t, "boom", got.LastError)
		assert.Equal(t, "key-1", got.LastOccurrenceKey)

		// Trimmed + lowercased fields.
		assert.Equal(t, "monthly", got.Cadence.Type)
		assert.Equal(t, "mon", got.Cadence.Weekday)
		assert.Equal(t, "pdf", got.Format)
		assert.Equal(t, "email", got.Delivery.Method)
		assert.Equal(t, "ok", got.LastRunStatus)

		// RetentionCount > 0 and integer fields like DayOfMonth are untouched.
		assert.Equal(t, 5, got.RetentionCount)
		assert.Equal(t, 1, got.Cadence.DayOfMonth)
	})

	t.Run("retention count guard: zero, negative, and boundary values", func(t *testing.T) {
		tests := []struct {
			name string
			in   int
			want int
		}{
			{"zero resets to default", 0, DefaultReportScheduleRetentionCount},
			{"negative resets to default", -7, DefaultReportScheduleRetentionCount},
			{"one is preserved (boundary)", 1, 1},
			{"large positive preserved", 999, 999},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				got := NormalizeReportSchedule(ReportSchedule{RetentionCount: tc.in})
				assert.Equal(t, tc.want, got.RetentionCount)
			})
		}
	})

	t.Run("nil scope resources is replaced with empty slice", func(t *testing.T) {
		got := NormalizeReportSchedule(ReportSchedule{}) // Scope.Resources is nil
		require.NotNil(t, got.Scope.Resources)
		assert.Len(t, got.Scope.Resources, 0)
	})

	t.Run("each scope resource is normalized (loop runs for every index)", func(t *testing.T) {
		in := ReportSchedule{Scope: ReportScheduleScope{Resources: []ReportScheduleResource{
			{ResourceType: "  HOST  ", ResourceID: "  h-1  ", Name: "  Host One  "},
			{ResourceType: "VM", ResourceID: "v-2", Name: "  VM Two  "},
		}}}
		got := NormalizeReportSchedule(in)
		require.Len(t, got.Scope.Resources, 2)
		assert.Equal(t, "host", got.Scope.Resources[0].ResourceType) // lowercased
		assert.Equal(t, "h-1", got.Scope.Resources[0].ResourceID)
		assert.Equal(t, "Host One", got.Scope.Resources[0].Name)
		assert.Equal(t, "vm", got.Scope.Resources[1].ResourceType)
		assert.Equal(t, "v-2", got.Scope.Resources[1].ResourceID)
		assert.Equal(t, "VM Two", got.Scope.Resources[1].Name)
	})

	t.Run("tags and delivery recipients are normalized via the slice helper", func(t *testing.T) {
		in := ReportSchedule{
			Scope:    ReportScheduleScope{Tags: []string{"  prod  ", "Prod", "  ", "staging"}},
			Delivery: ReportScheduleDelivery{To: []string{"ops@example.com", "  Ops@Example.com  ", ""}},
		}
		got := NormalizeReportSchedule(in)
		assert.Equal(t, []string{"prod", "staging"}, got.Scope.Tags)
		assert.Equal(t, []string{"ops@example.com"}, got.Delivery.To)
	})
}

// TestBranchCovNormalizeReportScheduleStringSlice covers every branch of
// normalizeReportScheduleStringSlice: the nil-input short-circuit, the
// empty/whitespace skip arm, the case-insensitive duplicate skip arm, the
// add-to-output arm, and first-occurrence ordering.
func TestBranchCovNormalizeReportScheduleStringSlice(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil input returns empty slice", nil, []string{}},
		{"empty input returns empty slice", []string{}, []string{}},
		{"all whitespace values filtered out", []string{"   ", "\t", "\n", " "}, []string{}},
		{"whitespace values trimmed before empty check", []string{"  a  ", "  ", "b"}, []string{"a", "b"}},
		{"case-insensitive duplicates keep first occurrence casing", []string{"Prod", "PROD", "prod", "Staging", "staging"}, []string{"Prod", "Staging"}},
		{"first occurrence ordering preserved", []string{"z", "a", "Z", "A", "m"}, []string{"z", "a", "m"}},
		{"mixed empty whitespace and valid deduped", []string{"", "  ", "x", "X", "y", ""}, []string{"x", "y"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeReportScheduleStringSlice(tc.in)
			require.NotNil(t, got, "result must be non-nil")
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestBranchCovParseReportScheduleStoreJSON covers every branch of
// parseReportScheduleStoreJSON: the empty/whitespace short-circuit, the
// object-form success path (store.Schedules != nil), the object-form-but-nil-
// schedules fallthrough, the bare-array-form success path, and the both-forms-
// fail error path.
func TestBranchCovParseReportScheduleStoreJSON(t *testing.T) {
	t.Run("nil data returns empty store no error", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON(nil)
		require.NoError(t, err)
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("empty byte slice returns empty store no error", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON([]byte{})
		require.NoError(t, err)
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("whitespace only data returns empty store no error", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON([]byte("   \n\t "))
		require.NoError(t, err)
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("object form with non-nil schedules is normalized", func(t *testing.T) {
		data := []byte(`{"schedules":[{"id":"  r1  ","name":"  Report 1  ","format":"  PDF  "}]}`)
		got, err := parseReportScheduleStoreJSON(data)
		require.NoError(t, err)
		require.Len(t, got.Schedules, 1)
		assert.Equal(t, "r1", got.Schedules[0].ID)
		assert.Equal(t, "Report 1", got.Schedules[0].Name)
		assert.Equal(t, "pdf", got.Schedules[0].Format) // lowercased by normalize
		// Default retention applied via normalize (retention_count was absent -> 0).
		assert.Equal(t, DefaultReportScheduleRetentionCount, got.Schedules[0].RetentionCount)
	})

	t.Run("object form with empty schedules array returns empty store", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON([]byte(`{"schedules":[]}`))
		require.NoError(t, err)
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("bare array form is parsed when object form fails", func(t *testing.T) {
		data := []byte(`[{"id":"  a  ","name":"A"}]`)
		got, err := parseReportScheduleStoreJSON(data)
		require.NoError(t, err)
		require.Len(t, got.Schedules, 1)
		assert.Equal(t, "a", got.Schedules[0].ID)
		assert.Equal(t, "A", got.Schedules[0].Name)
	})

	t.Run("empty bare array returns empty store", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON([]byte(`[]`))
		require.NoError(t, err)
		require.NotNil(t, got.Schedules)
		assert.Len(t, got.Schedules, 0)
	})

	t.Run("invalid JSON returns an error and zero store", func(t *testing.T) {
		got, err := parseReportScheduleStoreJSON([]byte(`{not json`))
		require.Error(t, err)
		// On the error path the function returns a zero ReportScheduleStore.
		assert.Nil(t, got.Schedules)
	})

	t.Run("object form with null schedules falls through and errors", func(t *testing.T) {
		// `{"schedules":null}` unmarshals cleanly but leaves store.Schedules
		// nil, so the parser falls through to the bare-array branch which
		// cannot interpret an object as an array and returns an error.
		got, err := parseReportScheduleStoreJSON([]byte(`{"schedules":null}`))
		require.Error(t, err)
		assert.Nil(t, got.Schedules)
	})

	t.Run("empty object without schedules field falls through and errors", func(t *testing.T) {
		// `{}` parses as an empty struct (no error) but store.Schedules is
		// nil, so it falls through; the object cannot be reinterpreted as a
		// bare array, yielding an error.
		got, err := parseReportScheduleStoreJSON([]byte(`{}`))
		require.Error(t, err)
		assert.Nil(t, got.Schedules)
	})
}
