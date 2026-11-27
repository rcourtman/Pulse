package monitoring

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		// Do not log the whole historical series on failure.
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("after-expiry scrape returned %d", rec.Code)
		}
		if got := strings.Count(rec.Body.String(), "pulse_scheduler_dead_letter_depth{"); got != 1 {
			t.Errorf("after expiry: got %d series, want 1", got)
		}

		key := metricKey{"pbs", "retired-00"}
		queue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: key.instance})
		assertDeadLetterScrape(t, handler, "same-label-fails-again", map[metricKey]int{peer: 1, key: 1})
		queue.Remove(InstanceTypePBS, key.instance)
		assertDeadLetterScrape(t, handler, "same-label-recovers-again", map[metricKey]int{peer: 1, key: 0})
	})
}
