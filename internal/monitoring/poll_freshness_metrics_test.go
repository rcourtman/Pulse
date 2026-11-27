package monitoring

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPollFreshnessScrapeKeepsLatestCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := prometheus.NewRegistry()
		pm := newPollMetrics(reg)
		record := func(end time.Time) {
			pm.RecordResult(PollResult{
				InstanceType: "pve", InstanceName: "fixture", Success: true,
				StartTime: end.Add(-time.Second), EndTime: end,
			})
			pm.RecordNodeResult(NodePollResult{
				InstanceType: "pve", InstanceName: "fixture", NodeName: "peer", Success: true,
				StartTime: end.Add(-time.Second), EndTime: end,
			})
		}
		scrape := func(phase string, end time.Time, age, count float64) {
			t.Helper()
			want := map[string]float64{
				"pulse_monitor_poll_last_success_timestamp":      float64(end.Unix()),
				"pulse_monitor_poll_staleness_seconds":           age,
				"pulse_monitor_node_poll_last_success_timestamp": float64(end.Unix()),
				"pulse_monitor_node_poll_staleness_seconds":      age,
				"pulse_monitor_poll_total":                       count,
				"pulse_monitor_node_poll_total":                  count,
			}
			families, err := reg.Gather()
			if err != nil {
				t.Fatal(err)
			}
			for _, family := range families {
				name := family.GetName()
				expected, ok := want[name]
				if !ok {
					continue
				}
				delete(want, name)
				if len(family.Metric) != 1 {
					t.Errorf("%s: %s has %d samples, want one", phase, name, len(family.Metric))
					continue
				}
				sample := family.Metric[0]
				got := sample.GetGauge().GetValue()
				if sample.Counter != nil {
					got = sample.GetCounter().GetValue()
				}
				if got != expected {
					t.Errorf("%s: %s = %v, want %v", phase, name, got, expected)
				}
			}
			if len(want) != 0 {
				t.Errorf("%s: missing metric families: %v", phase, want)
			}
		}

		first := time.Now()
		record(first)
		time.Sleep(20 * time.Second)
		latest := time.Now()
		record(latest)
		time.Sleep(5 * time.Second)
		scrape("latest completion", latest, 5, 2)

		// A recorder that was delayed after taking its end timestamp records
		// after the newer result. The observation still counts as an attempt.
		record(first.Add(time.Second))
		scrape("delayed older completion", latest, 5, 3)
		time.Sleep(25 * time.Second)
		scrape("no result during scrape interval", latest, 30, 3)

		recovered := time.Now()
		record(recovered)
		scrape("fresh completion after the quiet interval", recovered, 0, 4)
	})
}
