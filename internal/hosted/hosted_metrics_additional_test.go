package hosted

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHostedMetricsRecordProvisionNormalizesStatus(t *testing.T) {
	resetHostedMetricsForTest(t)
	m := GetHostedMetrics()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "exact success", input: "success", expected: provisionStatusSuccess},
		{name: "trimmed mixed-case success", input: "  SuCcEsS  ", expected: provisionStatusSuccess},
		{name: "explicit failure", input: "failure", expected: provisionStatusFailure},
		{name: "empty defaults to failure", input: "", expected: provisionStatusFailure},
		{name: "unknown defaults to failure", input: "unknown", expected: provisionStatusFailure},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			before := testutil.ToFloat64(m.provisionsTotal.WithLabelValues(tc.expected))
			m.RecordProvision(tc.input)
			after := testutil.ToFloat64(m.provisionsTotal.WithLabelValues(tc.expected))

			if after != before+1 {
				t.Fatalf("expected provisions counter for %q to increment by 1, before=%v after=%v", tc.expected, before, after)
			}
		})
	}
}

func TestHostedMetricsRecordLifecycleTransitionNormalizesStatuses(t *testing.T) {
	resetHostedMetricsForTest(t)
	m := GetHostedMetrics()

	testCases := []struct {
		name         string
		from         string
		to           string
		expectedFrom string
		expectedTo   string
	}{
		{
			name:         "empty values default active",
			from:         "",
			to:           "",
			expectedFrom: lifecycleStatusActive,
			expectedTo:   lifecycleStatusActive,
		},
		{
			name:         "known statuses normalized",
			from:         " SUSPENDED ",
			to:           "pending_deletion",
			expectedFrom: lifecycleStatusSuspended,
			expectedTo:   lifecycleStatusPendingDeletion,
		},
		{
			name:         "unknown values default active",
			from:         "bogus",
			to:           "other",
			expectedFrom: lifecycleStatusActive,
			expectedTo:   lifecycleStatusActive,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			before := testutil.ToFloat64(m.lifecycleTransitionsTotal.WithLabelValues(tc.expectedFrom, tc.expectedTo))
			m.RecordLifecycleTransition(tc.from, tc.to)
			after := testutil.ToFloat64(m.lifecycleTransitionsTotal.WithLabelValues(tc.expectedFrom, tc.expectedTo))

			if after != before+1 {
				t.Fatalf(
					"expected lifecycle counter for %q->%q to increment by 1, before=%v after=%v",
					tc.expectedFrom,
					tc.expectedTo,
					before,
					after,
				)
			}
		})
	}
}
