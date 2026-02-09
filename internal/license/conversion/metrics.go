package conversion

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// ConversionMetrics manages Prometheus instrumentation for conversion events.
type ConversionMetrics struct {
	eventsTotal  *prometheus.CounterVec
	invalidTotal *prometheus.CounterVec
}

var (
	conversionMetricsInstance *ConversionMetrics
	conversionMetricsOnce     sync.Once
)

// GetConversionMetrics returns the singleton conversion metrics instance.
func GetConversionMetrics() *ConversionMetrics {
	conversionMetricsOnce.Do(func() {
		conversionMetricsInstance = newConversionMetrics()
	})
	return conversionMetricsInstance
}

func newConversionMetrics() *ConversionMetrics {
	m := &ConversionMetrics{
		eventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "conversion",
				Name:      "events_total",
				Help:      "Total accepted conversion events by type and surface",
			},
			[]string{"type", "surface"},
		),
		invalidTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "conversion",
				Name:      "events_invalid_total",
				Help:      "Total invalid conversion events by reason",
			},
			[]string{"reason"},
		),
	}

	prometheus.MustRegister(
		m.eventsTotal,
		m.invalidTotal,
	)

	return m
}

// RecordEvent records an accepted conversion event.
func (m *ConversionMetrics) RecordEvent(eventType, surface string) {
	if eventType == "" {
		eventType = "unknown"
	}
	if surface == "" {
		surface = "unknown"
	}
	m.eventsTotal.WithLabelValues(eventType, surface).Inc()
}

// RecordInvalid records a conversion event validation failure.
func (m *ConversionMetrics) RecordInvalid(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	m.invalidTotal.WithLabelValues(reason).Inc()
}
