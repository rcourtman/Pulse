package monitoring

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
