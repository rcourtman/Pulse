package licensing

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// ConversionMetrics manages Prometheus instrumentation for conversion events.
type ConversionMetrics struct {
	eventsTotal  *prometheus.CounterVec
	invalidTotal *prometheus.CounterVec
	skippedTotal *prometheus.CounterVec
}

var (
	conversionMetricsInstance *ConversionMetrics
	conversionMetricsOnce     sync.Once
	conversionMetricsFactory  = defaultConversionMetricsFactory
)

// GetConversionMetrics returns the singleton conversion metrics instance.
func GetConversionMetrics() *ConversionMetrics {
	conversionMetricsOnce.Do(func() {
		conversionMetricsInstance = conversionMetricsFactory()
	})
	return conversionMetricsInstance
}

func defaultConversionMetricsFactory() *ConversionMetrics {
	return newConversionMetrics(prometheus.DefaultRegisterer)
}

func newConversionMetrics(registerer prometheus.Registerer) *ConversionMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

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
		skippedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "conversion",
				Name:      "events_skipped_total",
				Help:      "Total conversion events skipped by collection reason",
			},
			[]string{"reason"},
		),
	}

	m.eventsTotal = registerCounterVec(registerer, m.eventsTotal)
	m.invalidTotal = registerCounterVec(registerer, m.invalidTotal)
	m.skippedTotal = registerCounterVec(registerer, m.skippedTotal)

	return m
}

func registerCounterVec(registerer prometheus.Registerer, counter *prometheus.CounterVec) *prometheus.CounterVec {
	if err := registerer.Register(counter); err != nil {
		if alreadyRegisteredErr, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := alreadyRegisteredErr.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
		panic(err)
	}
	return counter
}

func defaultLabel(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// RecordEvent records an accepted conversion event.
func (m *ConversionMetrics) RecordEvent(eventType, surface string) {
	if m == nil || m.eventsTotal == nil {
		return
	}
	m.eventsTotal.WithLabelValues(defaultLabel(eventType), defaultLabel(surface)).Inc()
}

// RecordInvalid records a conversion event validation failure.
func (m *ConversionMetrics) RecordInvalid(reason string) {
	if m == nil || m.invalidTotal == nil {
		return
	}
	m.invalidTotal.WithLabelValues(defaultLabel(reason)).Inc()
}

// RecordSkipped records a conversion event skipped by runtime collection controls.
func (m *ConversionMetrics) RecordSkipped(reason string) {
	if m == nil || m.skippedTotal == nil {
		return
	}
	m.skippedTotal.WithLabelValues(defaultLabel(reason)).Inc()
}
