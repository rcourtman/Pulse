package monitoring

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

func TestSchedulerBreakerScrapeKeepsOneSnapshotWhilePollsComplete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		collector := newPollBreakerCollector()
		key := makeMetricKey("pve", "fixture")
		now := time.Now()
		collector.record(key, breakerMetricObservation{state: 2, failures: 3, retryAt: now.Add(time.Minute)})

		samples := make(chan prometheus.Metric)
		go func() {
			defer close(samples)
			collector.Collect(samples)
		}()
		// The scraper is blocked emitting its first sample. The poll must
		// be able to publish both updates before the reader resumes.
		synctest.Wait()
		written := make(chan struct{})
		go func() {
			collector.record(key, breakerMetricObservation{})
			collector.record(makeMetricKey("pbs", "fixture"), breakerMetricObservation{state: 1})
			close(written)
		}()
		synctest.Wait()
		select {
		case <-written:
		default:
			t.Error("slow metrics reader blocked poll completion")
		}

		want := map[*prometheus.Desc]float64{
			collector.state: 2, collector.failures: 3, collector.retrySeconds: 60,
		}
		for sample := range samples {
			expected, ok := want[sample.Desc()]
			if !ok {
				t.Errorf("unexpected or duplicate sample: %s", sample.Desc())
				continue
			}
			delete(want, sample.Desc())
			var metric dto.Metric
			if err := sample.Write(&metric); err != nil {
				t.Errorf("write sample: %v", err)
				continue
			}
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if got := metric.GetGauge().GetValue(); got != expected || labels["instance_type"] != "pve" || labels["instance"] != "fixture" {
				t.Errorf("old snapshot sample %s: got %v %v, want %v for pve/fixture", sample.Desc(), labels, got, expected)
			}
		}
		<-written
		if len(want) != 0 {
			t.Errorf("missing %d samples from old snapshot", len(want))
		}
	})
}

func TestSchedulerBreakerRetryCountdownWithoutNewResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := prometheus.NewRegistry()
		m := &Monitor{pollMetrics: newPollMetrics(reg), pollStatusMap: make(map[string]*pollStatus)}
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		const target = "fixture-peer"
		const pveLabels = `{instance="fixture-peer",instance_type="pve"}`
		const pbsLabels = `{instance="fixture-peer",instance_type="pbs"}`
		const retry = "pulse_scheduler_breaker_retry_seconds"
		const stateCode = "pulse_scheduler_breaker_state"
		const failures = "pulse_scheduler_breaker_failure_count"
		scrape := func(phase string) string {
			t.Helper()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: scrape status %d", phase, rec.Code)
			}
			body := rec.Body.String()
			for line := range strings.SplitSeq(body, "\n") {
				if strings.HasPrefix(line, retry+"{") {
					t.Logf("%s: %s", phase, line)
				}
			}
			return body
		}
		check := func(phase, body, name string, want int) {
			t.Helper()
			line := name + " " + strconv.Itoa(want)
			if !strings.Contains(body, "\n"+line+"\n") {
				t.Errorf("%s: missing %q", phase, line)
			}
		}
		verify := func(phase string, seconds, state, failureCount int) {
			t.Helper()
			body := scrape(phase)
			check(phase, body, retry+pveLabels, seconds)
			check(phase, body, stateCode+pveLabels, state)
			check(phase, body, failures+pveLabels, failureCount)
			check(phase, body, retry+pbsLabels, 0)
			check(phase, body, stateCode+pbsLabels, 0)
			check(phase, body, failures+pbsLabels, 0)
		}

		m.recordTaskResult(InstanceTypePBS, target, nil)
		if strings.Contains(scrape("before any PVE outcome"), retry+pveLabels) {
			t.Fatal("an unobserved PVE target should not have a zero countdown")
		}
		task := ScheduledTask{InstanceName: target, InstanceType: InstanceTypePVE}
		for i := 0; i < 3; i++ {
			if !m.allowExecution(task) {
				t.Fatalf("attempt %d unexpectedly refused", i+1)
			}
			m.recordTaskResult(task.InstanceType, task.InstanceName, errors.New("fixture poll failed"))
			if i < 2 {
				time.Sleep(30 * time.Second)
			}
		}
		verify("third failed poll opened the breaker", 40, 2, 3)

		// Read only the exporter. A Health snapshot takes the Monitor lock and
		// can refresh recorded gauges; neither it nor a new poll should be
		// needed to tell how much of the saved delay is left.
		func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			time.Sleep(25 * time.Second)
			verify("25s without an update, breaker still open", 15, 2, 3)
			time.Sleep(15 * time.Second)
			verify("open deadline reached, no probe yet", 0, 2, 3)
			time.Sleep(5 * time.Second)
			verify("past the open deadline, still no probe", 0, 2, 3)
		}()

		if !m.allowExecution(task) {
			t.Fatal("the original admission policy refused a probe past the deadline")
		}
		verify("probe admitted at its existing window", 30, 1, 3)
		time.Sleep(4 * time.Second)
		verify("half-open probe window ages without another result", 26, 1, 3)
		m.recordTaskResult(task.InstanceType, task.InstanceName, nil)
		verify("successful probe closed the breaker", 0, 0, 0)
	})
}
