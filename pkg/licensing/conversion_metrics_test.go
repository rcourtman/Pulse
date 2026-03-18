package licensing

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestGetConversionMetricsSingletonAndCounters(t *testing.T) {
	registry := prometheus.NewRegistry()
	withConversionMetricsFactoryForTest(t, func() *ConversionMetrics {
		return newConversionMetrics(registry)
	})

	first := GetConversionMetrics()
	second := GetConversionMetrics()
	if first != second {
		t.Fatal("expected GetConversionMetrics to return singleton instance")
	}

	first.RecordEvent("", "")
	first.RecordEvent("checkout_completed", "pricing_modal")
	first.RecordInvalid("")
	first.RecordInvalid("schema")
	first.RecordSkipped("")
	first.RecordSkipped("disabled")

	if got := testutil.ToFloat64(first.eventsTotal.WithLabelValues("unknown", "unknown")); got != 1 {
		t.Fatalf("expected events_total unknown/unknown to be 1, got %v", got)
	}
	if got := testutil.ToFloat64(first.eventsTotal.WithLabelValues("checkout_completed", "pricing_modal")); got != 1 {
		t.Fatalf("expected events_total checkout_completed/pricing_modal to be 1, got %v", got)
	}
	if got := testutil.ToFloat64(first.invalidTotal.WithLabelValues("unknown")); got != 1 {
		t.Fatalf("expected invalid_total unknown to be 1, got %v", got)
	}
	if got := testutil.ToFloat64(first.invalidTotal.WithLabelValues("schema")); got != 1 {
		t.Fatalf("expected invalid_total schema to be 1, got %v", got)
	}
	if got := testutil.ToFloat64(first.skippedTotal.WithLabelValues("unknown")); got != 1 {
		t.Fatalf("expected skipped_total unknown to be 1, got %v", got)
	}
	if got := testutil.ToFloat64(first.skippedTotal.WithLabelValues("disabled")); got != 1 {
		t.Fatalf("expected skipped_total disabled to be 1, got %v", got)
	}
}

func TestNewConversionMetricsReusesAlreadyRegisteredCollectors(t *testing.T) {
	registry := prometheus.NewRegistry()

	first := newConversionMetrics(registry)
	second := newConversionMetrics(registry)

	if first.eventsTotal != second.eventsTotal {
		t.Fatal("events_total collector was not reused")
	}
	if first.invalidTotal != second.invalidTotal {
		t.Fatal("events_invalid_total collector was not reused")
	}
	if first.skippedTotal != second.skippedTotal {
		t.Fatal("events_skipped_total collector was not reused")
	}
}

func withConversionMetricsFactoryForTest(t *testing.T, factory func() *ConversionMetrics) {
	t.Helper()

	prevInstance := conversionMetricsInstance
	prevFactory := conversionMetricsFactory

	conversionMetricsInstance = nil
	conversionMetricsOnce = sync.Once{}
	conversionMetricsFactory = factory

	t.Cleanup(func() {
		conversionMetricsInstance = prevInstance
		conversionMetricsFactory = prevFactory
		conversionMetricsOnce = sync.Once{}
		if prevInstance != nil {
			conversionMetricsOnce.Do(func() {})
		}
	})
}
