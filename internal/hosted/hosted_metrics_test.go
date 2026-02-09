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
