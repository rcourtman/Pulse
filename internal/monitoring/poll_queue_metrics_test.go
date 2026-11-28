package monitoring

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestSchedulerQueueScrapeAggregatesLiveTypeCounts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := prometheus.NewRegistry()
		pm := newPollMetrics(reg)
		left, right := NewTaskQueue(), NewTaskQueue()
		now := time.Now()
		add := func(q *TaskQueue, kind, name string, due time.Time) {
			q.Upsert(ScheduledTask{InstanceType: InstanceType(kind), InstanceName: name, NextRun: due})
		}
		add(left, "pve", "first", now.Add(-15*time.Second))
		add(left, " pve ", "second", now.Add(time.Minute))
		// More types than the collector's inline scratch capacity.
		for i := range 9 {
			add(left, fmt.Sprintf("provider-%d", i), "peer", now.Add(time.Minute))
		}
		add(right, "pve", "third", now.Add(12*time.Second))
		add(right, "", "unknown", time.Time{})

		leftCtx, cancelLeft := context.WithCancel(context.Background())
		defer cancelLeft()
		defer pm.trackTaskQueue(leftCtx, left)()
		defer pm.trackTaskQueue(leftCtx, left)() // Multiple workers, one queue.
		defer pm.trackTaskQueue(context.Background(), right)()

		scrape := func(depth, due, age, pve, other int) {
			t.Helper()
			want := fmt.Sprintf(`# HELP pulse_monitor_poll_queue_depth Approximate number of poll tasks waiting to complete in the current cycle.
# TYPE pulse_monitor_poll_queue_depth gauge
pulse_monitor_poll_queue_depth %d
# HELP pulse_scheduler_queue_due_soon Number of tasks due to run within the immediate window (12s).
# TYPE pulse_scheduler_queue_due_soon gauge
pulse_scheduler_queue_due_soon %d
# HELP pulse_scheduler_queued_poll_overdue_seconds Seconds past the scheduled time of the most overdue timestamped poll still queued in running schedulers; zero if none.
# TYPE pulse_scheduler_queued_poll_overdue_seconds gauge
pulse_scheduler_queued_poll_overdue_seconds %d
# HELP pulse_scheduler_queue_depth Current scheduler queue depth partitioned by instance type.
# TYPE pulse_scheduler_queue_depth gauge
pulse_scheduler_queue_depth{instance_type="pve"} %d
pulse_scheduler_queue_depth{instance_type="unknown"} 1
`, depth, due, age, pve)
			for i := range 9 {
				want += fmt.Sprintf("pulse_scheduler_queue_depth{instance_type=\"provider-%d\"} %d\n", i, other)
			}
			if err := testutil.GatherAndCompare(reg, strings.NewReader(want),
				"pulse_monitor_poll_queue_depth", "pulse_scheduler_queue_due_soon",
				"pulse_scheduler_queued_poll_overdue_seconds", "pulse_scheduler_queue_depth"); err != nil {
				t.Fatal(err)
			}
		}

		scrape(13, 3, 15, 3, 1)
		cancelLeft() // Its live registration has stopped but its backlog still exists.
		scrape(2, 2, 0, 1, 0)
	})
}
