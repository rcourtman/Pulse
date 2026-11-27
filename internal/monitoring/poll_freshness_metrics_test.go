package monitoring

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPollFreshnessScrapeKeepsOneSnapshotWhilePollsComplete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pm := newPollMetrics(prometheus.NewRegistry())
		collector := newPollFreshnessCollector(pm)
		at := time.Now()
		pm.storeLastSuccess("pve", "fixture", at.Add(-time.Minute))
		pm.RecordNodeResult(NodePollResult{
			InstanceType: "pve", InstanceName: "fixture", NodeName: "peer",
			Success: false, StartTime: at, EndTime: at,
		})
		samples := make(chan prometheus.Metric)
		go func() {
			collector.Collect(samples)
			close(samples)
		}()
		// The collector has copied both sets and is blocked sending the
		// first sample. Poll completions can still update both live maps.
		synctest.Wait()
		pm.storeLastSuccess("pve", "fixture", at)
		pm.storeNodeLastSuccess("pve", "fixture", "peer", at)

		want := map[*prometheus.Desc]float64{
			collector.connectionAge:  60,
			collector.connectionTime: float64(at.Add(-time.Minute).Unix()),
			collector.nodeAge:        -1, // No node success in this scrape's snapshot.
		}
		for sample := range samples {
			desc := sample.Desc()
			expected, ok := want[desc]
			if !ok {
				t.Errorf("unexpected or duplicate sample: %s", desc)
				continue
			}
			delete(want, desc)
			metric := &dto.Metric{}
			if err := sample.Write(metric); err != nil {
				t.Errorf("write sample: %v", err)
				continue
			}
			if got := metric.GetGauge().GetValue(); got != expected {
				t.Errorf("%s = %v, want %v", desc, got, expected)
			}
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["instance_type"] != "pve" || labels["instance"] != "fixture" ||
				(desc == collector.nodeAge && (len(labels) != 3 || labels["node"] != "peer")) ||
				(desc != collector.nodeAge && len(labels) != 2) {
				t.Errorf("%s: unexpected labels: %v", desc, labels)
			}
		}
		if len(want) != 0 {
			t.Errorf("missing samples: %v", want)
		}
	})
}

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
