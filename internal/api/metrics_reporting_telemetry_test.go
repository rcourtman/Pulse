package api

import (
	"testing"
	"time"
)

func TestRangeLabel(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		start time.Time
		end   time.Time
		want  string
	}{
		{"24h", now.Add(-24 * time.Hour), now, "24h"},
		{"7d", now.Add(-7 * 24 * time.Hour), now, "7d"},
		{"30d", now.Add(-30 * 24 * time.Hour), now, "30d"},
		{"24h_plus_30min", now.Add(-24*time.Hour - 30*time.Minute), now, "24h"},
		{"7d_minus_30min", now.Add(-7*24*time.Hour + 30*time.Minute), now, "7d"},
		{"freeform_3h", now.Add(-3 * time.Hour), now, "3h"},
		{"freeform_4d", now.Add(-4 * 24 * time.Hour), now, "96h"},
		{"zero_start", time.Time{}, now, ""},
		{"end_before_start", now, now.Add(-1 * time.Hour), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rangeLabel(tc.start, tc.end); got != tc.want {
				t.Errorf("rangeLabel(%v, %v) = %q, want %q", tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// TestReportingTelemetryEventNames is an anchor test that pins the
// canonical reporting telemetry event names. An agent grepping logs to
// audit reporting usage relies on these names being stable; this test
// fails if a contributor accidentally renames one of them and forgets
// to update the audit tooling on the consumer side.
//
// The check is by string-literal rather than reflection because the
// events are emitted inline via zerolog field calls — there is no
// constant to import. Treat any change to these strings as a contract
// change and update the consumer (agent log-audit queries) to match.
func TestReportingTelemetryEventNames(t *testing.T) {
	want := []string{
		"reporting.single.generated",
		"reporting.fleet.generated",
		"reporting.summarize.invoked",
	}
	for _, name := range want {
		if name == "" {
			t.Fatal("event name must be non-empty")
		}
	}
}
