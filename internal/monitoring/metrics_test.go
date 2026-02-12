package monitoring

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	internalerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
)

// newTestPollMetrics creates a PollMetrics instance with an isolated registry for testing.
func newTestPollMetrics(t *testing.T) *PollMetrics {
	t.Helper()

	reg := prometheus.NewRegistry()

	pm := &PollMetrics{
		schedulerQueueReady: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_due_soon",
				Help:      "Number of tasks due to run within the immediate window.",
			},
		),
		schedulerQueueDepthByType: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_depth",
				Help:      "Current scheduler queue depth partitioned by instance type.",
			},
			[]string{"instance_type"},
		),
		schedulerDeadLetterDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "dead_letter_depth",
				Help:      "Number of tasks currently parked in the dead-letter queue per instance.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_state",
				Help:      "Circuit breaker state encoded as 0=closed, 1=half-open, 2=open, -1=unknown.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerFailureCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_failure_count",
				Help:      "Current consecutive failure count tracked by the circuit breaker.",
			},
			[]string{"instance_type", "instance"},
		),
		schedulerBreakerRetrySeconds: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "breaker_retry_seconds",
				Help:      "Seconds until the circuit breaker will allow another attempt.",
			},
			[]string{"instance_type", "instance"},
		),
		lastQueueTypeKeys: make(map[string]struct{}),
		lastDLQKeys:       make(map[string]struct{}),
	}

	reg.MustRegister(
		pm.schedulerQueueReady,
		pm.schedulerQueueDepthByType,
		pm.schedulerDeadLetterDepth,
		pm.schedulerBreakerState,
		pm.schedulerBreakerFailureCount,
		pm.schedulerBreakerRetrySeconds,
	)

	return pm
}

// getGaugeValue returns the value of a prometheus gauge.
func getGaugeValue(g prometheus.Gauge) float64 {
	m := &dto.Metric{}
	if err := g.Write(m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

// getGaugeVecValue returns the value for specific labels from a GaugeVec.
func getGaugeVecValue(gv *prometheus.GaugeVec, labels ...string) float64 {
	m := &dto.Metric{}
	gauge, err := gv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	if err := gauge.Write(m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

func TestUpdateQueueSnapshot_SetsDueWithinSeconds(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 42,
		PerType:          map[string]int{},
	})

	got := getGaugeValue(pm.schedulerQueueReady)
	if got != 42 {
		t.Fatalf("schedulerQueueReady = %v, want 42", got)
	}
}

func TestUpdateQueueSnapshot_UpdatesPerTypeQueueDepth(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 0,
		PerType: map[string]int{
			"pve": 10,
			"pbs": 5,
			"pmg": 3,
		},
	})

	cases := []struct {
		instanceType string
		want         float64
	}{
		{"pve", 10},
		{"pbs", 5},
		{"pmg", 3},
	}

	for _, tc := range cases {
		got := getGaugeVecValue(pm.schedulerQueueDepthByType, tc.instanceType)
		if got != tc.want {
			t.Errorf("queue_depth{instance_type=%q} = %v, want %v", tc.instanceType, got, tc.want)
		}
	}
}

func TestUpdateQueueSnapshot_ClearsStaleTypeKeys(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// First snapshot with pve and pbs
	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 0,
		PerType: map[string]int{
			"pve": 10,
			"pbs": 5,
		},
	})

	// Verify initial values
	if got := getGaugeVecValue(pm.schedulerQueueDepthByType, "pve"); got != 10 {
		t.Fatalf("initial pve = %v, want 10", got)
	}
	if got := getGaugeVecValue(pm.schedulerQueueDepthByType, "pbs"); got != 5 {
		t.Fatalf("initial pbs = %v, want 5", got)
	}

	// Second snapshot with only pve (pbs should be cleared)
	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 0,
		PerType: map[string]int{
			"pve": 8,
		},
	})

	if got := getGaugeVecValue(pm.schedulerQueueDepthByType, "pve"); got != 8 {
		t.Errorf("updated pve = %v, want 8", got)
	}
	if got := getGaugeVecValue(pm.schedulerQueueDepthByType, "pbs"); got != 0 {
		t.Errorf("stale pbs should be 0, got %v", got)
	}
}

func TestUpdateQueueSnapshot_EmptySnapshot(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// First add some data
	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 10,
		PerType: map[string]int{
			"pve": 5,
		},
	})

	// Then clear with empty snapshot
	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 0,
		PerType:          map[string]int{},
	})

	if got := getGaugeValue(pm.schedulerQueueReady); got != 0 {
		t.Errorf("schedulerQueueReady = %v, want 0", got)
	}
	if got := getGaugeVecValue(pm.schedulerQueueDepthByType, "pve"); got != 0 {
		t.Errorf("pve should be cleared to 0, got %v", got)
	}
}

func TestUpdateDeadLetterCounts_EmptyClearsPrevious(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// First add some tasks
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
		{Type: "pbs", Instance: "pbs1"},
	})

	// Verify they were set
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Fatalf("initial pve/pve1 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 1 {
		t.Fatalf("initial pbs/pbs1 = %v, want 1", got)
	}

	// Clear with empty slice
	pm.UpdateDeadLetterCounts([]DeadLetterTask{})

	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 0 {
		t.Errorf("pve/pve1 should be cleared to 0, got %v", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 0 {
		t.Errorf("pbs/pbs1 should be cleared to 0, got %v", got)
	}
}

func TestUpdateDeadLetterCounts_SingleTask(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "my-pve-instance"},
	})

	got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "my-pve-instance")
	if got != 1 {
		t.Fatalf("dead_letter_depth{pve,my-pve-instance} = %v, want 1", got)
	}
}

func TestUpdateDeadLetterCounts_AggregatesSameTypeInstance(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
		{Type: "pve", Instance: "pve1"},
		{Type: "pve", Instance: "pve1"},
		{Type: "pbs", Instance: "pbs1"},
		{Type: "pbs", Instance: "pbs1"},
	})

	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 3 {
		t.Errorf("pve/pve1 = %v, want 3", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 2 {
		t.Errorf("pbs/pbs1 = %v, want 2", got)
	}
}

func TestUpdateDeadLetterCounts_ClearsStaleKeys(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// First update with pve1 and pbs1
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
		{Type: "pbs", Instance: "pbs1"},
	})

	// Verify initial values
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Fatalf("initial pve/pve1 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 1 {
		t.Fatalf("initial pbs/pbs1 = %v, want 1", got)
	}

	// Second update with only pve1 (pbs1 should be cleared)
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
		{Type: "pve", Instance: "pve1"},
	})

	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 2 {
		t.Errorf("updated pve/pve1 = %v, want 2", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pbs", "pbs1"); got != 0 {
		t.Errorf("stale pbs/pbs1 should be 0, got %v", got)
	}
}

func TestUpdateDeadLetterCounts_NormalizesEmptyLabels(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// Empty Type and Instance should normalize to "unknown"
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "", Instance: ""},
		{Type: "  ", Instance: "  "},
	})

	got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "unknown", "unknown")
	if got != 2 {
		t.Fatalf("empty labels should normalize to unknown, got count %v, want 2", got)
	}
}

func TestUpdateDeadLetterCounts_MultipleInstancesSameType(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
		{Type: "pve", Instance: "pve2"},
		{Type: "pve", Instance: "pve3"},
	})

	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve1"); got != 1 {
		t.Errorf("pve/pve1 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve2"); got != 1 {
		t.Errorf("pve/pve2 = %v, want 1", got)
	}
	if got := getGaugeVecValue(pm.schedulerDeadLetterDepth, "pve", "pve3"); got != 1 {
		t.Errorf("pve/pve3 = %v, want 1", got)
	}
}

func TestSetBreakerState_ZeroRetryAt(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.SetBreakerState("pve", "pve1", "closed", 0, time.Time{})

	got := getGaugeVecValue(pm.schedulerBreakerRetrySeconds, "pve", "pve1")
	if got != 0 {
		t.Fatalf("retrySeconds = %v, want 0 for zero retryAt", got)
	}
}

func TestSetBreakerState_FutureRetryAt(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// Set retryAt 60 seconds in the future
	retryAt := time.Now().Add(60 * time.Second)
	pm.SetBreakerState("pve", "pve1", "open", 3, retryAt)

	got := getGaugeVecValue(pm.schedulerBreakerRetrySeconds, "pve", "pve1")
	// Allow some tolerance for test execution time
	if got < 55 || got > 65 {
		t.Fatalf("retrySeconds = %v, want ~60 for future retryAt", got)
	}
}

func TestSetBreakerState_PastRetryAtClampsToZero(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	// Set retryAt 60 seconds in the past (already expired)
	retryAt := time.Now().Add(-60 * time.Second)
	pm.SetBreakerState("pve", "pve1", "half_open", 1, retryAt)

	got := getGaugeVecValue(pm.schedulerBreakerRetrySeconds, "pve", "pve1")
	if got != 0 {
		t.Fatalf("retrySeconds = %v, want 0 for past retryAt", got)
	}
}

func TestSetBreakerState_StateConversion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state string
		want  float64
	}{
		{"closed", 0},
		{"CLOSED", 0},
		{"Closed", 0},
		{"half_open", 1},
		{"half-open", 1},
		{"HALF_OPEN", 1},
		{"open", 2},
		{"OPEN", 2},
		{"Open", 2},
		{"unknown_state", -1},
		{"", -1},
		{"invalid", -1},
	}

	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			pm := newTestPollMetrics(t)
			pm.SetBreakerState("pve", "pve1", tc.state, 0, time.Time{})

			got := getGaugeVecValue(pm.schedulerBreakerState, "pve", "pve1")
			if got != tc.want {
				t.Errorf("breakerState for %q = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestSetBreakerState_FailureCount(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.SetBreakerState("pve", "pve1", "open", 7, time.Time{})

	got := getGaugeVecValue(pm.schedulerBreakerFailureCount, "pve", "pve1")
	if got != 7 {
		t.Fatalf("failureCount = %v, want 7", got)
	}
}

func TestSetBreakerState_EmptyLabelsSanitized(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.SetBreakerState("", "", "open", 5, time.Time{})

	// Empty labels should be normalized to "unknown"
	gotState := getGaugeVecValue(pm.schedulerBreakerState, "unknown", "unknown")
	if gotState != 2 {
		t.Errorf("breaker_state{unknown,unknown} = %v, want 2 (open)", gotState)
	}

	gotFailures := getGaugeVecValue(pm.schedulerBreakerFailureCount, "unknown", "unknown")
	if gotFailures != 5 {
		t.Errorf("breaker_failure_count{unknown,unknown} = %v, want 5", gotFailures)
	}
}

func TestSetBreakerState_WhitespaceOnlyLabelsSanitized(t *testing.T) {
	t.Parallel()

	pm := newTestPollMetrics(t)

	pm.SetBreakerState("  ", "   ", "closed", 0, time.Time{})

	gotState := getGaugeVecValue(pm.schedulerBreakerState, "unknown", "unknown")
	if gotState != 0 {
		t.Errorf("breaker_state{unknown,unknown} = %v, want 0 (closed)", gotState)
	}
}

// newFullTestPollMetrics creates a PollMetrics instance with all fields for RecordResult testing.
func newFullTestPollMetrics(t *testing.T) *PollMetrics {
	t.Helper()

	reg := prometheus.NewRegistry()

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
				Help:      "Seconds since the last successful poll.",
			},
			[]string{"instance_type", "instance"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_queue_depth",
				Help:      "Approximate number of poll tasks waiting.",
			},
		),
		lastSuccessByKey: make(map[metricKey]time.Time),
	}

	reg.MustRegister(
		pm.pollDuration,
		pm.pollResults,
		pm.pollErrors,
		pm.lastSuccess,
		pm.staleness,
		pm.queueDepth,
	)

	return pm
}

// getCounterVecValue returns the value for specific labels from a CounterVec.
func getCounterVecValue(cv *prometheus.CounterVec, labels ...string) float64 {
	m := &dto.Metric{}
	counter, err := cv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	if err := counter.Write(m); err != nil {
		return 0
	}
	return m.GetCounter().GetValue()
}

// getHistogramSampleCount returns the sample count for specific labels from a HistogramVec.
func getHistogramSampleCount(hv *prometheus.HistogramVec, labels ...string) uint64 {
	m := &dto.Metric{}
	obs, err := hv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		return 0
	}
	return m.GetHistogram().GetSampleCount()
}

// getHistogramSampleSum returns the sample sum for specific labels from a HistogramVec.
func getHistogramSampleSum(hv *prometheus.HistogramVec, labels ...string) float64 {
	m := &dto.Metric{}
	obs, err := hv.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0
	}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		return 0
	}
	return m.GetHistogram().GetSampleSum()
}

func TestRecordResult_SuccessUpdatesLastSuccessAndStaleness(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	endTime := time.Now()
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "my-instance",
		StartTime:    endTime.Add(-500 * time.Millisecond),
		EndTime:      endTime,
		Success:      true,
	})

	// Check lastSuccess is set to EndTime
	gotLastSuccess := getGaugeVecValue(pm.lastSuccess, "pve", "my-instance")
	wantLastSuccess := float64(endTime.Unix())
	if gotLastSuccess != wantLastSuccess {
		t.Errorf("lastSuccess = %v, want %v", gotLastSuccess, wantLastSuccess)
	}

	// Check staleness is set to 0 on success
	gotStaleness := getGaugeVecValue(pm.staleness, "pve", "my-instance")
	if gotStaleness != 0 {
		t.Errorf("staleness = %v, want 0 on success", gotStaleness)
	}

	// Check success counter incremented
	gotSuccessCount := getCounterVecValue(pm.pollResults, "pve", "my-instance", "success")
	if gotSuccessCount != 1 {
		t.Errorf("poll_total{result=success} = %v, want 1", gotSuccessCount)
	}

	// Check internal lastSuccessByKey is updated
	ts, ok := pm.lastSuccessFor("pve", "my-instance")
	if !ok {
		t.Fatal("lastSuccessFor returned false, expected true")
	}
	if !ts.Equal(endTime) {
		t.Errorf("stored lastSuccess = %v, want %v", ts, endTime)
	}
}

func TestRecordResult_FailureIncrementsErrorCounter(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Use a MonitorError to test error classification
	monErr := internalerrors.NewMonitorError(
		internalerrors.ErrorTypeConnection,
		"poll_nodes",
		"pve1",
		errors.New("connection refused"),
	)

	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "pve1",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        monErr,
	})

	// Check error counter with classified type
	gotErrorCount := getCounterVecValue(pm.pollErrors, "pve", "pve1", "connection")
	if gotErrorCount != 1 {
		t.Errorf("poll_errors_total{error_type=connection} = %v, want 1", gotErrorCount)
	}

	// Check error result counter
	gotErrorResultCount := getCounterVecValue(pm.pollResults, "pve", "pve1", "error")
	if gotErrorResultCount != 1 {
		t.Errorf("poll_total{result=error} = %v, want 1", gotErrorResultCount)
	}
}

func TestRecordResult_FailureWithUnknownErrorType(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Non-MonitorError should classify as "unknown"
	pm.RecordResult(PollResult{
		InstanceType: "pbs",
		InstanceName: "pbs1",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        errors.New("some random error"),
	})

	gotErrorCount := getCounterVecValue(pm.pollErrors, "pbs", "pbs1", "unknown")
	if gotErrorCount != 1 {
		t.Errorf("poll_errors_total{error_type=unknown} = %v, want 1", gotErrorCount)
	}
}

func TestRecordResult_FailureWithNilErrorClassifiesAsNone(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	pm.RecordResult(PollResult{
		InstanceType: "pmg",
		InstanceName: "pmg1",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        nil,
	})

	gotErrorCount := getCounterVecValue(pm.pollErrors, "pmg", "pmg1", "none")
	if gotErrorCount != 1 {
		t.Errorf("poll_errors_total{error_type=none} = %v, want 1", gotErrorCount)
	}
}

func TestRecordResult_NegativeDurationClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	endTime := time.Now()
	startTime := endTime.Add(time.Second) // Start AFTER end = negative duration

	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "neg-test",
		StartTime:    startTime,
		EndTime:      endTime,
		Success:      true,
	})

	// Histogram should have recorded 0, not a negative value
	sampleSum := getHistogramSampleSum(pm.pollDuration, "pve", "neg-test")
	if sampleSum != 0 {
		t.Errorf("poll_duration sum = %v, want 0 for negative duration", sampleSum)
	}

	sampleCount := getHistogramSampleCount(pm.pollDuration, "pve", "neg-test")
	if sampleCount != 1 {
		t.Errorf("poll_duration count = %v, want 1", sampleCount)
	}
}

func TestRecordResult_LabelsSanitized(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	pm.RecordResult(PollResult{
		InstanceType: "",   // Should become "unknown"
		InstanceName: "  ", // Should become "unknown"
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      true,
	})

	// Check that metrics were recorded with sanitized labels
	gotSuccessCount := getCounterVecValue(pm.pollResults, "unknown", "unknown", "success")
	if gotSuccessCount != 1 {
		t.Errorf("poll_total{unknown,unknown,success} = %v, want 1", gotSuccessCount)
	}
}

func TestRecordResult_DecrementsPending(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Set initial pending count
	pm.ResetQueueDepth(5)

	// Record a result - should decrement pending
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "pve1",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      true,
	})

	// Check pending was decremented
	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 4 {
		t.Errorf("pending = %v, want 4 after decrement from 5", gotPending)
	}

	// Check queueDepth gauge reflects the new value
	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 4 {
		t.Errorf("queueDepth gauge = %v, want 4", gotQueueDepth)
	}
}

func TestRecordResult_FailureStalenessWithPreviousSuccess(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// First, record a successful poll
	firstEndTime := time.Now().Add(-10 * time.Second)
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "stale-test",
		StartTime:    firstEndTime.Add(-time.Second),
		EndTime:      firstEndTime,
		Success:      true,
	})

	// Now record a failed poll
	secondEndTime := time.Now()
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "stale-test",
		StartTime:    secondEndTime.Add(-time.Second),
		EndTime:      secondEndTime,
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be ~10 seconds
	gotStaleness := getGaugeVecValue(pm.staleness, "pve", "stale-test")
	if gotStaleness < 9 || gotStaleness > 11 {
		t.Errorf("staleness = %v, want ~10 seconds", gotStaleness)
	}
}

func TestRecordResult_FailureStalenessWithoutPreviousSuccess(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Record a failure without any prior success
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "no-prior-success",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be -1 (no prior success)
	gotStaleness := getGaugeVecValue(pm.staleness, "pve", "no-prior-success")
	if gotStaleness != -1 {
		t.Errorf("staleness = %v, want -1 for no prior success", gotStaleness)
	}
}

func TestRecordResult_DurationObserved(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	endTime := time.Now()
	startTime := endTime.Add(-2 * time.Second)

	pm.RecordResult(PollResult{
		InstanceType: "pbs",
		InstanceName: "duration-test",
		StartTime:    startTime,
		EndTime:      endTime,
		Success:      true,
	})

	sampleSum := getHistogramSampleSum(pm.pollDuration, "pbs", "duration-test")
	if sampleSum < 1.9 || sampleSum > 2.1 {
		t.Errorf("poll_duration sum = %v, want ~2.0 seconds", sampleSum)
	}

	sampleCount := getHistogramSampleCount(pm.pollDuration, "pbs", "duration-test")
	if sampleCount != 1 {
		t.Errorf("poll_duration count = %v, want 1", sampleCount)
	}
}

// newInFlightTestPollMetrics creates a PollMetrics with inflight gauge for testing.
func newInFlightTestPollMetrics(t *testing.T) *PollMetrics {
	t.Helper()

	reg := prometheus.NewRegistry()

	pm := &PollMetrics{
		inflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_inflight",
				Help:      "Current number of poll operations executing per instance type.",
			},
			[]string{"instance_type"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "poll_queue_depth",
				Help:      "Approximate number of poll tasks waiting.",
			},
		),
	}

	reg.MustRegister(pm.inflight, pm.queueDepth)

	return pm
}

func TestResetQueueDepth_SetsPendingTotal(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	pm.ResetQueueDepth(42)

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 42 {
		t.Errorf("pending = %v, want 42", gotPending)
	}

	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 42 {
		t.Errorf("queueDepth gauge = %v, want 42", gotQueueDepth)
	}
}

func TestResetQueueDepth_NegativeClampsToZero(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// First set to positive value
	pm.ResetQueueDepth(10)

	// Then set to negative - should clamp to 0
	pm.ResetQueueDepth(-5)

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 0 {
		t.Errorf("pending = %v, want 0 for negative input", gotPending)
	}

	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 0 {
		t.Errorf("queueDepth gauge = %v, want 0 for negative input", gotQueueDepth)
	}
}

func TestResetQueueDepth_ZeroWorksCorrectly(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// First set to positive value
	pm.ResetQueueDepth(10)

	// Then reset to zero
	pm.ResetQueueDepth(0)

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 0 {
		t.Errorf("pending = %v, want 0", gotPending)
	}

	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 0 {
		t.Errorf("queueDepth gauge = %v, want 0", gotQueueDepth)
	}
}

func TestIncInFlight_IncrementsGauge(t *testing.T) {
	t.Parallel()

	pm := newInFlightTestPollMetrics(t)

	pm.IncInFlight("pve")
	pm.IncInFlight("pve")
	pm.IncInFlight("pbs")

	gotPve := getGaugeVecValue(pm.inflight, "pve")
	if gotPve != 2 {
		t.Errorf("inflight{pve} = %v, want 2", gotPve)
	}

	gotPbs := getGaugeVecValue(pm.inflight, "pbs")
	if gotPbs != 1 {
		t.Errorf("inflight{pbs} = %v, want 1", gotPbs)
	}
}

func TestDecInFlight_DecrementsGauge(t *testing.T) {
	t.Parallel()

	pm := newInFlightTestPollMetrics(t)

	// First increment a few times
	pm.IncInFlight("pve")
	pm.IncInFlight("pve")
	pm.IncInFlight("pve")

	// Then decrement
	pm.DecInFlight("pve")

	got := getGaugeVecValue(pm.inflight, "pve")
	if got != 2 {
		t.Errorf("inflight{pve} = %v, want 2 after inc(3) dec(1)", got)
	}

	// Decrement again
	pm.DecInFlight("pve")
	pm.DecInFlight("pve")

	got = getGaugeVecValue(pm.inflight, "pve")
	if got != 0 {
		t.Errorf("inflight{pve} = %v, want 0 after full decrement", got)
	}
}

func TestDecrementPending_DecrementsWhenPositive(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Set initial pending count
	pm.ResetQueueDepth(5)

	pm.decrementPending()

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 4 {
		t.Errorf("pending = %v, want 4 after decrement from 5", gotPending)
	}
}

func TestDecrementPending_DoesNotGoBelowZero(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Start at 0 (default)
	pm.decrementPending()

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 0 {
		t.Errorf("pending = %v, want 0 (should not go negative)", gotPending)
	}

	// Also verify the gauge is 0
	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 0 {
		t.Errorf("queueDepth gauge = %v, want 0", gotQueueDepth)
	}
}

func TestDecrementPending_UpdatesQueueDepthGauge(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	pm.ResetQueueDepth(10)
	pm.decrementPending()

	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 9 {
		t.Errorf("queueDepth gauge = %v, want 9", gotQueueDepth)
	}
}

func TestDecrementPending_MultipleDecrements(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	pm.ResetQueueDepth(5)

	// Decrement 5 times
	for i := 0; i < 5; i++ {
		pm.decrementPending()
	}

	pm.mu.RLock()
	gotPending := pm.pending
	pm.mu.RUnlock()

	if gotPending != 0 {
		t.Errorf("pending = %v, want 0 after 5 decrements from 5", gotPending)
	}

	gotQueueDepth := getGaugeValue(pm.queueDepth)
	if gotQueueDepth != 0 {
		t.Errorf("queueDepth gauge = %v, want 0", gotQueueDepth)
	}

	// Decrement one more time - should stay at 0
	pm.decrementPending()

	pm.mu.RLock()
	gotPending = pm.pending
	pm.mu.RUnlock()

	if gotPending != 0 {
		t.Errorf("pending = %v, want 0 after extra decrement", gotPending)
	}
}

func TestRecordResult_FailureNegativeStalenessClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newFullTestPollMetrics(t)

	// Record a success with a future timestamp
	futureTime := time.Now().Add(10 * time.Second)
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "negative-staleness",
		StartTime:    futureTime.Add(-time.Second),
		EndTime:      futureTime,
		Success:      true,
	})

	// Now record a failure with an EndTime BEFORE the last success
	// This creates a negative staleness calculation (EndTime - lastSuccess < 0)
	pastTime := futureTime.Add(-5 * time.Second)
	pm.RecordResult(PollResult{
		InstanceType: "pve",
		InstanceName: "negative-staleness",
		StartTime:    pastTime.Add(-time.Second),
		EndTime:      pastTime,
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be clamped to 0, not negative
	gotStaleness := getGaugeVecValue(pm.staleness, "pve", "negative-staleness")
	if gotStaleness != 0 {
		t.Errorf("staleness = %v, want 0 for negative staleness calculation", gotStaleness)
	}
}

// newNodeTestPollMetrics creates a PollMetrics instance with node-level metrics for testing.
func newNodeTestPollMetrics(t *testing.T) *PollMetrics {
	t.Helper()

	reg := prometheus.NewRegistry()

	pm := &PollMetrics{
		nodePollDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_duration_seconds",
				Help:      "Duration of polling operations per node.",
				Buckets:   []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 30},
			},
			[]string{"instance_type", "instance", "node"},
		),
		nodePollResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_total",
				Help:      "Total polling attempts per node partitioned by result.",
			},
			[]string{"instance_type", "instance", "node", "result"},
		),
		nodePollErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_errors_total",
				Help:      "Polling failures per node grouped by error type.",
			},
			[]string{"instance_type", "instance", "node", "error_type"},
		),
		nodeLastSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_last_success_timestamp",
				Help:      "Unix timestamp of the last successful poll for a node.",
			},
			[]string{"instance_type", "instance", "node"},
		),
		nodeStaleness: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "pulse",
				Subsystem: "monitor",
				Name:      "node_poll_staleness_seconds",
				Help:      "Seconds since the last successful poll for a node.",
			},
			[]string{"instance_type", "instance", "node"},
		),
		nodeLastSuccessByKey: make(map[nodeMetricKey]time.Time),
	}

	reg.MustRegister(
		pm.nodePollDuration,
		pm.nodePollResults,
		pm.nodePollErrors,
		pm.nodeLastSuccess,
		pm.nodeStaleness,
	)

	return pm
}

func TestRecordNodeResult_SuccessUpdatesMetrics(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	endTime := time.Now()
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "my-pve",
		NodeName:     "node1",
		StartTime:    endTime.Add(-500 * time.Millisecond),
		EndTime:      endTime,
		Success:      true,
	})

	// Check lastSuccess is set
	gotLastSuccess := getGaugeVecValue(pm.nodeLastSuccess, "pve", "my-pve", "node1")
	wantLastSuccess := float64(endTime.Unix())
	if gotLastSuccess != wantLastSuccess {
		t.Errorf("nodeLastSuccess = %v, want %v", gotLastSuccess, wantLastSuccess)
	}

	// Check staleness is set to 0 on success
	gotStaleness := getGaugeVecValue(pm.nodeStaleness, "pve", "my-pve", "node1")
	if gotStaleness != 0 {
		t.Errorf("nodeStaleness = %v, want 0 on success", gotStaleness)
	}

	// Check success counter incremented
	gotSuccessCount := getCounterVecValue(pm.nodePollResults, "pve", "my-pve", "node1", "success")
	if gotSuccessCount != 1 {
		t.Errorf("node_poll_total{result=success} = %v, want 1", gotSuccessCount)
	}

	// Check internal nodeLastSuccessByKey is updated
	ts, ok := pm.lastNodeSuccessFor("pve", "my-pve", "node1")
	if !ok {
		t.Fatal("lastNodeSuccessFor returned false, expected true")
	}
	if !ts.Equal(endTime) {
		t.Errorf("stored nodeLastSuccess = %v, want %v", ts, endTime)
	}
}

func TestRecordNodeResult_FailureIncrementsErrorCounter(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	monErr := internalerrors.NewMonitorError(
		internalerrors.ErrorTypeTimeout,
		"poll_node",
		"pve1",
		errors.New("timeout"),
	)

	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "pve1",
		NodeName:     "node2",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        monErr,
	})

	// Check error counter with classified type
	gotErrorCount := getCounterVecValue(pm.nodePollErrors, "pve", "pve1", "node2", "timeout")
	if gotErrorCount != 1 {
		t.Errorf("node_poll_errors_total{error_type=timeout} = %v, want 1", gotErrorCount)
	}

	// Check error result counter
	gotErrorResultCount := getCounterVecValue(pm.nodePollResults, "pve", "pve1", "node2", "error")
	if gotErrorResultCount != 1 {
		t.Errorf("node_poll_total{result=error} = %v, want 1", gotErrorResultCount)
	}
}

func TestRecordNodeResult_NegativeDurationClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	endTime := time.Now()
	startTime := endTime.Add(time.Second) // Start AFTER end = negative duration

	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "neg-test",
		NodeName:     "node1",
		StartTime:    startTime,
		EndTime:      endTime,
		Success:      true,
	})

	sampleSum := getHistogramSampleSum(pm.nodePollDuration, "pve", "neg-test", "node1")
	if sampleSum != 0 {
		t.Errorf("node_poll_duration sum = %v, want 0 for negative duration", sampleSum)
	}
}

func TestRecordNodeResult_EmptyNodeNameNormalized(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "pve1",
		NodeName:     "",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      true,
	})

	// Empty node name should normalize to "unknown-node"
	gotSuccessCount := getCounterVecValue(pm.nodePollResults, "pve", "pve1", "unknown-node", "success")
	if gotSuccessCount != 1 {
		t.Errorf("node_poll_total{node=unknown-node,result=success} = %v, want 1", gotSuccessCount)
	}
}

func TestRecordNodeResult_FailureStalenessWithPreviousSuccess(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	// First, record a successful poll
	firstEndTime := time.Now().Add(-10 * time.Second)
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "stale-test",
		NodeName:     "node1",
		StartTime:    firstEndTime.Add(-time.Second),
		EndTime:      firstEndTime,
		Success:      true,
	})

	// Now record a failed poll
	secondEndTime := time.Now()
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "stale-test",
		NodeName:     "node1",
		StartTime:    secondEndTime.Add(-time.Second),
		EndTime:      secondEndTime,
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be ~10 seconds
	gotStaleness := getGaugeVecValue(pm.nodeStaleness, "pve", "stale-test", "node1")
	if gotStaleness < 9 || gotStaleness > 11 {
		t.Errorf("nodeStaleness = %v, want ~10 seconds", gotStaleness)
	}
}

func TestRecordNodeResult_FailureStalenessWithoutPreviousSuccess(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	// Record a failure without any prior success
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "no-prior-success",
		NodeName:     "node1",
		StartTime:    time.Now().Add(-time.Second),
		EndTime:      time.Now(),
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be -1 (no prior success)
	gotStaleness := getGaugeVecValue(pm.nodeStaleness, "pve", "no-prior-success", "node1")
	if gotStaleness != -1 {
		t.Errorf("nodeStaleness = %v, want -1 for no prior success", gotStaleness)
	}
}

func TestRecordNodeResult_FailureNegativeStalenessClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newNodeTestPollMetrics(t)

	// Record a success with a future timestamp
	futureTime := time.Now().Add(10 * time.Second)
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "neg-stale",
		NodeName:     "node1",
		StartTime:    futureTime.Add(-time.Second),
		EndTime:      futureTime,
		Success:      true,
	})

	// Now record a failure with an EndTime BEFORE the last success
	pastTime := futureTime.Add(-5 * time.Second)
	pm.RecordNodeResult(NodePollResult{
		InstanceType: "pve",
		InstanceName: "neg-stale",
		NodeName:     "node1",
		StartTime:    pastTime.Add(-time.Second),
		EndTime:      pastTime,
		Success:      false,
		Error:        errors.New("failed"),
	})

	// Staleness should be clamped to 0, not negative
	gotStaleness := getGaugeVecValue(pm.nodeStaleness, "pve", "neg-stale", "node1")
	if gotStaleness != 0 {
		t.Errorf("nodeStaleness = %v, want 0 for negative staleness calculation", gotStaleness)
	}
}

// newQueueWaitTestPollMetrics creates a PollMetrics instance for RecordQueueWait testing.
func newQueueWaitTestPollMetrics(t *testing.T) *PollMetrics {
	t.Helper()

	reg := prometheus.NewRegistry()

	pm := &PollMetrics{
		schedulerQueueWait: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "scheduler",
				Name:      "queue_wait_seconds",
				Help:      "Observed wait time between task readiness and execution.",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"instance_type"},
		),
	}

	reg.MustRegister(pm.schedulerQueueWait)

	return pm
}

func TestRecordQueueWait_RecordsWaitTime(t *testing.T) {
	t.Parallel()

	pm := newQueueWaitTestPollMetrics(t)

	pm.RecordQueueWait("pve", 2*time.Second)

	sampleCount := getHistogramSampleCount(pm.schedulerQueueWait, "pve")
	if sampleCount != 1 {
		t.Errorf("queue_wait count = %v, want 1", sampleCount)
	}

	sampleSum := getHistogramSampleSum(pm.schedulerQueueWait, "pve")
	if sampleSum < 1.9 || sampleSum > 2.1 {
		t.Errorf("queue_wait sum = %v, want ~2.0 seconds", sampleSum)
	}
}

func TestRecordQueueWait_NegativeWaitClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newQueueWaitTestPollMetrics(t)

	pm.RecordQueueWait("pve", -5*time.Second)

	sampleSum := getHistogramSampleSum(pm.schedulerQueueWait, "pve")
	if sampleSum != 0 {
		t.Errorf("queue_wait sum = %v, want 0 for negative wait", sampleSum)
	}
}

func TestRecordQueueWait_EmptyTypeNormalized(t *testing.T) {
	t.Parallel()

	pm := newQueueWaitTestPollMetrics(t)

	pm.RecordQueueWait("", time.Second)

	// Empty label should normalize to "unknown"
	sampleCount := getHistogramSampleCount(pm.schedulerQueueWait, "unknown")
	if sampleCount != 1 {
		t.Errorf("queue_wait{unknown} count = %v, want 1", sampleCount)
	}
}

func TestSetQueueDepth_SetsGauge(t *testing.T) {
	t.Parallel()

	pm := newInFlightTestPollMetrics(t)

	pm.SetQueueDepth(42)

	got := getGaugeValue(pm.queueDepth)
	if got != 42 {
		t.Errorf("queueDepth = %v, want 42", got)
	}
}

func TestSetQueueDepth_NegativeClampedToZero(t *testing.T) {
	t.Parallel()

	pm := newInFlightTestPollMetrics(t)

	pm.SetQueueDepth(-10)

	got := getGaugeValue(pm.queueDepth)
	if got != 0 {
		t.Errorf("queueDepth = %v, want 0 for negative input", got)
	}
}

func TestSetQueueDepth_ZeroWorks(t *testing.T) {
	t.Parallel()

	pm := newInFlightTestPollMetrics(t)

	// First set to positive
	pm.SetQueueDepth(10)

	// Then set to zero
	pm.SetQueueDepth(0)

	got := getGaugeValue(pm.queueDepth)
	if got != 0 {
		t.Errorf("queueDepth = %v, want 0", got)
	}
}
