package monitoring

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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
		lastQueueTypeKeys: make(map[string]struct{}),
		lastDLQKeys:       make(map[string]struct{}),
	}

	reg.MustRegister(
		pm.schedulerQueueReady,
		pm.schedulerQueueDepthByType,
		pm.schedulerDeadLetterDepth,
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

func TestUpdateQueueSnapshot_NilPollMetrics(t *testing.T) {
	t.Parallel()

	var pm *PollMetrics
	// Should not panic
	pm.UpdateQueueSnapshot(QueueSnapshot{
		DueWithinSeconds: 5,
		PerType:          map[string]int{"pve": 10},
	})
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

func TestUpdateDeadLetterCounts_NilPollMetrics(t *testing.T) {
	t.Parallel()

	var pm *PollMetrics
	// Should not panic
	pm.UpdateDeadLetterCounts([]DeadLetterTask{
		{Type: "pve", Instance: "pve1"},
	})
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
