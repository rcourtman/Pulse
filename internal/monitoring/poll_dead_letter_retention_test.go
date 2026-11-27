package monitoring

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestDeadLetterLabelSnapshotTracksMembershipWithReusedBuffer(t *testing.T) {
	first, peer := NewTaskQueue(), NewTaskQueue()
	first.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "claimed"})
	first.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "parked", NextRun: time.Now().Add(time.Hour)})
	all := first.snapshotLabels(nil)
	if len(all) != 2 {
		t.Fatalf("initial snapshot = %v, want two entries", all)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	task, ok := first.WaitNext(ctx)
	if !ok || task.InstanceName != "claimed" {
		t.Fatalf("claimed = %v, %t, want the immediately due target", task, ok)
	}
	defer first.completeClaim(task, ScheduledTask{}, noPollReschedule)
	// A caller's earlier snapshot owns its labels even after the queue changes.
	if all[0] != (queuedTaskLabel{InstanceTypePVE, "claimed"}) {
		t.Fatalf("earlier snapshot was mutated: %v", all)
	}
	remaining := first.snapshotLabels(all)
	if len(remaining) != 1 || remaining[0] != (queuedTaskLabel{InstanceTypePBS, "parked"}) {
		t.Fatalf("after claim = %v, want only the queued PBS target", remaining)
	}
	if &remaining[0] != &all[0] {
		t.Fatal("small snapshot replaced reusable storage")
	}
	empty := peer.snapshotLabels(remaining)
	if len(empty) != 0 {
		t.Fatalf("empty peer inherited a previous queue's labels: %v", empty)
	}
	peer.Upsert(ScheduledTask{InstanceType: InstanceTypePMG, InstanceName: "peer"})
	other := peer.snapshotLabels(empty)
	if len(other) != 1 || other[0] != (queuedTaskLabel{InstanceTypePMG, "peer"}) {
		t.Fatalf("peer snapshot = %v, want only its own queued target", other)
	}
}

func TestSchedulerDeadLetterScrapeExpiresHistoricalLabels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const grace = 5 * time.Minute
		const retired = 64
		reg := prometheus.NewRegistry()
		pm := newPollMetrics(reg)
		queue := NewTaskQueue()
		defer pm.trackDeadLetterQueue(t.Context(), queue)()
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		peer := metricKey{"pve", "peer"}
		queue.Upsert(ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: peer.instance})

		both := map[metricKey]int{peer: 1}
		cleared := map[metricKey]int{peer: 1}
		for i := 0; i < retired; i++ {
			name := fmt.Sprintf("retired-%02d", i)
			queue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: name})
			key := metricKey{"pbs", name}
			both[key], cleared[key] = 1, 0
		}
		assertDeadLetterScrape(t, handler, "live", both)
		for key := range cleared {
			if key != peer {
				queue.Remove(InstanceTypePBS, key.instance)
			}
		}
		assertDeadLetterScrape(t, handler, "zero-after-removal", cleared)
		time.Sleep(grace - time.Second)
		assertDeadLetterScrape(t, handler, "zero-before-expiry", cleared)
		time.Sleep(time.Second)
		assertDeadLetterScrape(t, handler, "expired", map[metricKey]int{peer: 1})

		key := metricKey{"pbs", "retired-00"}
		queue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: key.instance})
		assertDeadLetterScrape(t, handler, "same-label-fails-again", map[metricKey]int{peer: 1, key: 1})
		queue.Remove(InstanceTypePBS, key.instance)
		// After a gap at least as long as the window, the first scrape
		// after recovery can omit the label without first observing zero.
		time.Sleep(grace)
		assertDeadLetterScrape(t, handler, "gap-after-recovery", map[metricKey]int{peer: 1})
	})
}
