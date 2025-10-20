package monitoring

import (
	stdErrors "errors"
	"fmt"
	"sync"
	"time"

	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// PollMetrics manages Prometheus instrumentation for polling activity.
type PollMetrics struct {
	pollDuration *prometheus.HistogramVec
	pollResults  *prometheus.CounterVec
	pollErrors   *prometheus.CounterVec
	lastSuccess  *prometheus.GaugeVec
	staleness    *prometheus.GaugeVec
	queueDepth   prometheus.Gauge
	inflight     *prometheus.GaugeVec

	mu               sync.RWMutex
	lastSuccessByKey map[string]time.Time
	pending          int
}

var (
	pollMetricsInstance *PollMetrics
	pollMetricsOnce     sync.Once
)

func getPollMetrics() *PollMetrics {
	pollMetricsOnce.Do(func() {
		pollMetricsInstance = newPollMetrics()
	})
	return pollMetricsInstance
}

func newPollMetrics() *PollMetrics {
	pm := &PollMetrics{
		pollDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_duration_seconds",
				Help:      "Duration of polling operations per instance.",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 30},
			},
			[]string{"instance_type", "instance"},
		),
		pollResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_total",
				Help:      "Total polling attempts partitioned by result.",
			},
			[]string{"instance_type", "instance", "result"},
		),
		pollErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_errors_total",
				Help:      "Polling failures grouped by error type.",
			},
			[]string{"instance_type", "instance", "error_type"},
		),
		lastSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_last_success_timestamp",
				Help:      "Unix timestamp of the last successful poll.",
			},
			[]string{"instance_type", "instance"},
		),
		staleness: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_staleness_seconds",
				Help:      "Seconds since the last successful poll. -1 indicates no successes yet.",
			},
			[]string{"instance_type", "instance"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_queue_depth",
				Help:      "Approximate number of poll tasks waiting to complete in the current cycle.",
			},
		),
		inflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_inflight",
				Help:      "Current number of poll operations executing per instance type.",
			},
			[]string{"instance_type"},
		),
		lastSuccessByKey: make(map[string]time.Time),
	}

	prometheus.MustRegister(
		pm.pollDuration,
		pm.pollResults,
		pm.pollErrors,
		pm.lastSuccess,
		pm.staleness,
		pm.queueDepth,
		pm.inflight,
	)

	return pm
}

// RecordResult records metrics for a polling result.
func (pm *PollMetrics) RecordResult(result PollResult) {
	if pm == nil {
		return
	}

	labels := prometheus.Labels{
		"instance_type": result.InstanceType,
		"instance":      result.InstanceName,
	}

	duration := result.EndTime.Sub(result.StartTime).Seconds()
	if duration < 0 {
		duration = 0
	}
	pm.pollDuration.With(labels).Observe(duration)

	resultValue := "success"
	if !result.Success {
		resultValue = "error"
	}
	pm.pollResults.With(prometheus.Labels{
		"instance_type": result.InstanceType,
		"instance":      result.InstanceName,
		"result":        resultValue,
	}).Inc()

	if result.Success {
		pm.lastSuccess.With(labels).Set(float64(result.EndTime.Unix()))
		pm.storeLastSuccess(result.InstanceType, result.InstanceName, result.EndTime)
		pm.updateStaleness(result.InstanceType, result.InstanceName, 0)
	} else {
		errType := pm.classifyError(result.Error)
		pm.pollErrors.With(prometheus.Labels{
			"instance_type": result.InstanceType,
			"instance":      result.InstanceName,
			"error_type":    errType,
		}).Inc()

		if last, ok := pm.lastSuccessFor(result.InstanceType, result.InstanceName); ok && !last.IsZero() {
			staleness := result.EndTime.Sub(last).Seconds()
			if staleness < 0 {
				staleness = 0
			}
			pm.updateStaleness(result.InstanceType, result.InstanceName, staleness)
		} else {
			pm.updateStaleness(result.InstanceType, result.InstanceName, -1)
		}
	}

	pm.decrementPending()
}

// ResetQueueDepth sets the pending queue depth for the next polling cycle.
func (pm *PollMetrics) ResetQueueDepth(total int) {
	if pm == nil {
		return
	}
	if total < 0 {
		total = 0
	}

	pm.mu.Lock()
	pm.pending = total
	pm.mu.Unlock()
	pm.queueDepth.Set(float64(total))
}

// SetQueueDepth allows direct gauge control when needed.
func (pm *PollMetrics) SetQueueDepth(depth int) {
	if pm == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	pm.queueDepth.Set(float64(depth))
}

// IncInFlight increments the in-flight gauge for the given instance type.
func (pm *PollMetrics) IncInFlight(instanceType string) {
	if pm == nil {
		return
	}
	pm.inflight.WithLabelValues(instanceType).Inc()
}

// DecInFlight decrements the in-flight gauge for the given instance type.
func (pm *PollMetrics) DecInFlight(instanceType string) {
	if pm == nil {
		return
	}
	pm.inflight.WithLabelValues(instanceType).Dec()
}

func (pm *PollMetrics) decrementPending() {
	if pm == nil {
		return
	}

	pm.mu.Lock()
	if pm.pending > 0 {
		pm.pending--
	}
	current := pm.pending
	pm.mu.Unlock()

	pm.queueDepth.Set(float64(current))
}

func (pm *PollMetrics) storeLastSuccess(instanceType, instance string, ts time.Time) {
	pm.mu.Lock()
	pm.lastSuccessByKey[pm.key(instanceType, instance)] = ts
	pm.mu.Unlock()
}

func (pm *PollMetrics) lastSuccessFor(instanceType, instance string) (time.Time, bool) {
	pm.mu.RLock()
	ts, ok := pm.lastSuccessByKey[pm.key(instanceType, instance)]
	pm.mu.RUnlock()
	return ts, ok
}

func (pm *PollMetrics) updateStaleness(instanceType, instance string, value float64) {
	pm.staleness.WithLabelValues(instanceType, instance).Set(value)
}

func (pm *PollMetrics) key(instanceType, instance string) string {
	return fmt.Sprintf("%s::%s", instanceType, instance)
}

func (pm *PollMetrics) classifyError(err error) string {
	if err == nil {
		return "none"
	}

	var monitorErr *internalerrors.MonitorError
	if stdErrors.As(err, &monitorErr) {
		return string(monitorErr.Type)
	}

	return "unknown"
}
