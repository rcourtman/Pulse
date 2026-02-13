package hosted

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func resetHostedMetricsForTest(t *testing.T) {
	t.Helper()

	originalRegisterer := prometheus.DefaultRegisterer
	originalGatherer := prometheus.DefaultGatherer

	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry

	hostedMetricsInstance = nil
	hostedMetricsOnce = sync.Once{}

	t.Cleanup(func() {
		hostedMetricsInstance = nil
		hostedMetricsOnce = sync.Once{}
		prometheus.DefaultRegisterer = originalRegisterer
		prometheus.DefaultGatherer = originalGatherer
	})
}

func TestHostedMetricsRegistersWithoutPanic(t *testing.T) {
	resetHostedMetricsForTest(t)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GetHostedMetrics panicked during registration: %v", r)
		}
	}()

	if got := GetHostedMetrics(); got == nil {
		t.Fatal("expected hosted metrics instance, got nil")
	}
}

func TestHostedMetricsCounterIncrement(t *testing.T) {
	resetHostedMetricsForTest(t)

	m := GetHostedMetrics()
	before := testutil.ToFloat64(m.signupsTotal)

	m.RecordSignup()

	after := testutil.ToFloat64(m.signupsTotal)
	if after != before+1 {
		t.Fatalf("expected signups counter to increment by 1, before=%v after=%v", before, after)
	}
}

func TestHostedMetricsGaugeSet(t *testing.T) {
	resetHostedMetricsForTest(t)

	m := GetHostedMetrics()
	m.SetActiveTenants(42)

	if got := testutil.ToFloat64(m.activeTenants); got != 42 {
		t.Fatalf("expected active tenants gauge to be 42, got %v", got)
	}
}

func TestParseProvisionMetricStatus(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  ProvisionMetricStatus
	}{
		{name: "success canonical", input: "success", want: ProvisionMetricStatusSuccess},
		{name: "success normalized", input: " SUCCESS ", want: ProvisionMetricStatusSuccess},
		{name: "unknown defaults to failure", input: "unknown", want: ProvisionMetricStatusFailure},
		{name: "empty defaults to failure", input: "", want: ProvisionMetricStatusFailure},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseProvisionMetricStatus(tc.input); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestParseLifecycleMetricStatus(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  LifecycleMetricStatus
	}{
		{name: "active canonical", input: "active", want: LifecycleMetricStatusActive},
		{name: "active default empty", input: "", want: LifecycleMetricStatusActive},
		{name: "active normalized", input: " ACTIVE ", want: LifecycleMetricStatusActive},
		{name: "suspended", input: "suspended", want: LifecycleMetricStatusSuspended},
		{name: "pending deletion", input: "pending_deletion", want: LifecycleMetricStatusPendingDeletion},
		{name: "unknown defaults to active", input: "unknown", want: LifecycleMetricStatusActive},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseLifecycleMetricStatus(tc.input); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestHostedMetricsRecordProvisionStatus(t *testing.T) {
	resetHostedMetricsForTest(t)

	m := GetHostedMetrics()
	m.RecordProvisionStatus(ProvisionMetricStatusSuccess)
	m.RecordProvision("unknown")

	if got := testutil.ToFloat64(m.provisionsTotal.WithLabelValues(string(ProvisionMetricStatusSuccess))); got != 1 {
		t.Fatalf("expected success provision count 1, got %v", got)
	}
	if got := testutil.ToFloat64(m.provisionsTotal.WithLabelValues(string(ProvisionMetricStatusFailure))); got != 1 {
		t.Fatalf("expected failure provision count 1, got %v", got)
	}
}

func TestHostedMetricsRecordLifecycleTransitionStatus(t *testing.T) {
	resetHostedMetricsForTest(t)

	m := GetHostedMetrics()
	m.RecordLifecycleTransitionStatus(LifecycleMetricStatusSuspended, LifecycleMetricStatusPendingDeletion)
	m.RecordLifecycleTransition("unknown", "")

	if got := testutil.ToFloat64(m.lifecycleTransitionsTotal.WithLabelValues(
		string(LifecycleMetricStatusSuspended),
		string(LifecycleMetricStatusPendingDeletion),
	)); got != 1 {
		t.Fatalf("expected suspended->pending_deletion transition count 1, got %v", got)
	}

	if got := testutil.ToFloat64(m.lifecycleTransitionsTotal.WithLabelValues(
		string(LifecycleMetricStatusActive),
		string(LifecycleMetricStatusActive),
	)); got != 1 {
		t.Fatalf("expected active->active transition count 1, got %v", got)
	}
}
