package monitoring

import (
	"testing"
	"time"
)

// PlannedPollInterval feeds the connections aggregator the cadence the
// adaptive scheduler actually promised, so the active→stale cutoff can track
// stretched schedules (#1437).
func TestPlannedPollIntervalReportsLastScheduled(t *testing.T) {
	sched := NewAdaptiveScheduler(DefaultSchedulerConfig(), nil, nil, nil)
	now := time.Now()
	tasks := sched.BuildPlan(now, []InstanceDescriptor{{
		Name:        "pve-a",
		Type:        InstanceTypePVE,
		LastSuccess: now,
	}}, 0)
	if len(tasks) != 1 {
		t.Fatalf("planned tasks = %d, want 1", len(tasks))
	}
	if tasks[0].Interval <= 0 {
		t.Fatalf("planned interval = %v, want > 0", tasks[0].Interval)
	}

	m := &Monitor{scheduler: sched}
	if got := m.PlannedPollInterval(InstanceTypePVE, "pve-a"); got != tasks[0].Interval {
		t.Fatalf("PlannedPollInterval = %v, want %v", got, tasks[0].Interval)
	}
	if got := m.PlannedPollInterval(InstanceTypePVE, "unknown"); got != 0 {
		t.Fatalf("unknown instance interval = %v, want 0", got)
	}
	if got := (&Monitor{}).PlannedPollInterval(InstanceTypePVE, "pve-a"); got != 0 {
		t.Fatalf("schedulerless monitor interval = %v, want 0", got)
	}
	var nilMonitor *Monitor
	if got := nilMonitor.PlannedPollInterval(InstanceTypePVE, "pve-a"); got != 0 {
		t.Fatalf("nil monitor interval = %v, want 0", got)
	}
}
