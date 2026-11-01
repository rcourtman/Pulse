package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// ioCounterStamp is the JSON value of a counter read stamp.
func ioCounterStamp(t *testing.T, at time.Time) string {
	t.Helper()
	raw, err := json.Marshal(at.UTC())
	if err != nil {
		t.Fatalf("encode counter read stamp: %v", err)
	}
	return string(raw)
}

// ioCounterWireReport decodes report from the JSON a current agent sends,
// with stamp spliced in as its ioCountersCollectedAt value; an empty stamp is
// an agent that sends none. A server without the field ignores it, so these
// tests compile there and fail on its behaviour.
func ioCounterWireReport(t *testing.T, report agentshost.Report, stamp string) agentshost.Report {
	t.Helper()
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encode report: %v", err)
	}
	if stamp != "" {
		raw = append(raw[:len(raw)-1], []byte(`,"ioCountersCollectedAt":`+stamp+`}`)...)
	}
	var decoded agentshost.Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode wire report: %v", err)
	}
	return decoded
}

// slowTail is how long optional collectors ran after the counter read before
// the report Timestamp: 20s on even slots, none on odd ones.
func slowTail(slot int) time.Duration {
	if slot%2 == 0 {
		return 20 * time.Second
	}
	return 0
}

// Counters are read every 30s at 0/30/60/...; optional collectors then delay
// the report Timestamp to 0/50/60/110/... A drained FIFO delivers the reports
// back to back. Rates must divide by the 30s between counter reads, not by
// the 50s/10s between Timestamps or by the receipt gap.
func TestApplyHostReportSlowOptionalTailRatesUseIOCounterReadTime(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-48 * time.Hour))
	report := func(slot int) agentshost.Report {
		authored := agent.collectedAt(slot).Add(slowTail(slot))
		return ioCounterWireReport(t, agent.hostReport("stream-a", uint64(slot), slot, authored), ioCounterStamp(t, agent.collectedAt(slot)))
	}

	first := applyCounterClockHost(t, m, report(1))
	wantHostRatesUnknown(t, "first sample", first)
	for slot := 2; slot <= 11; slot++ {
		host := applyCounterClockHost(t, m, report(slot))
		wantHostTrueRates(t, fmt.Sprintf("drained slot %d", slot), host)
	}
	wantHistoryTrueRates(t, m, first.ID)

	// Received just now from two-day-old reads: cleanup keyed on the sample
	// clock would drop the baselines.
	m.cleanupRateTracker(time.Now())
	wantHostTrueRates(t, "report after rate-series cleanup", applyCounterClockHost(t, m, report(12)))
}

// Negative control: the same slow-tail reports from an agent that sends no
// counter read stamp keep the report-Timestamp fallback, so their rates carry
// the tail (30/50 and 30/10 of the real rate). This proves the tail above
// reaches the denominator, and that an older agent is left as it was.
func TestApplyHostReportOlderAgentSlowTailKeepsTimestampFallback(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	report := func(slot int) agentshost.Report {
		return ioCounterWireReport(t, agent.hostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot).Add(slowTail(slot))), "")
	}
	applyCounterClockHost(t, m, report(1))
	slow := applyCounterClockHost(t, m, report(2))
	wantTrueRate(t, "older agent after a slow tail", slow.NetInRate, counterClockNetInPerSecond*30.0/50)
	fast := applyCounterClockHost(t, m, report(3))
	wantTrueRate(t, "older agent after the tail ends", fast.NetInRate, counterClockNetInPerSecond*30.0/10)
}

// Every sampling-mode transition and source clock fault either has a
// comparable interval in one mode or reports no rate and writes no rate
// history: report-Timestamp and counter-read times share a clock but not an
// offset, so a switch is a new baseline, and a stamp that does not advance
// drops it. A counter reset keeps its existing rebase to zero.
func TestApplyHostReportIOCounterReadTimeTransitions(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-6 * time.Hour))
	at := agent.collectedAt
	back := func(slot int) time.Time { return at(slot).Add(-time.Hour) }
	stamp := func(readAt time.Time) string { return ioCounterStamp(t, readAt) }

	// A legacy report sets receipt-time baselines; its stamp has no ordered
	// clock behind it.
	legacy := ioCounterWireReport(t, agent.hostReport("", 0, 1, at(1)), stamp(at(1)))
	hostID := applyCounterClockHost(t, m, legacy).ID

	steps := []struct {
		name   string
		stream string
		seq    uint64
		slot   int
		stamp  string
		want   string
	}{
		{"older sequenced agent after receipt-time baselines", "stream-a", 1, 2, "", "unknown"},
		{"older agent on its report Timestamp", "stream-a", 2, 3, "", "true"},
		{"agent starts stamping its counter read", "stream-a", 3, 4, stamp(at(4)), "unknown"},
		{"counter-read interval", "stream-a", 4, 5, stamp(at(5)), "true"},
		{"stamp missing", "stream-a", 5, 6, "", "unknown"},
		{"stamp returns", "stream-a", 6, 7, stamp(at(7)), "unknown"},
		{"counter-read interval again", "stream-a", 7, 8, stamp(at(8)), "true"},
		{"zero stamp", "stream-a", 8, 9, `"0001-01-01T00:00:00Z"`, "unknown"},
		{"stamp after the zero one", "stream-a", 9, 10, stamp(at(10)), "unknown"},
		{"interval after the zero stamp", "stream-a", 10, 11, stamp(at(11)), "true"},
		{"agent restart keeps the machine clock", "stream-b", 1, 12, stamp(at(12)), "true"},
		{"stamp steps back an hour", "stream-b", 2, 13, stamp(back(13)), "unknown"},
		{"first read on the stepped clock", "stream-b", 3, 14, stamp(back(14)), "unknown"},
		{"interval on the stepped clock", "stream-b", 4, 15, stamp(back(15)), "true"},
		{"equal stamp with the next sequence", "stream-b", 5, 16, stamp(back(15)), "unknown"},
		{"first read after the repeated stamp", "stream-b", 6, 17, stamp(back(17)), "unknown"},
		{"interval after the repeated stamp", "stream-b", 7, 18, stamp(back(18)), "true"},
	}
	for _, step := range steps {
		before := counterClockHistory(m, hostID)
		// Older-agent steps carry no tail so their Timestamp interval is exact.
		report := ioCounterWireReport(t, agent.hostReport(step.stream, step.seq, step.slot, at(step.slot)), step.stamp)
		host := applyCounterClockHost(t, m, report)
		switch step.want {
		case "unknown":
			wantHostRatesUnknown(t, step.name, host)
			if after := counterClockHistory(m, hostID); !reflect.DeepEqual(after, before) {
				t.Fatalf("%s wrote rate history without a comparable interval:\nbefore %v\nafter  %v", step.name, before, after)
			}
		case "true":
			wantHostTrueRates(t, step.name, host)
		}
	}

	// The host reboots: every counter restarts from zero. That keeps its
	// existing meaning, a measured zero that rebases, and the next read is
	// measured from the restarted counters.
	rebooted := agent.hostReport("stream-b", 8, 19, back(19))
	rebooted.Network[0].RXBytes, rebooted.Network[0].TXBytes = 0, 0
	rebooted.DiskIO[0].ReadBytes, rebooted.DiskIO[0].WriteBytes, rebooted.DiskIO[0].IOTime = 0, 0, 0
	host := applyCounterClockHost(t, m, ioCounterWireReport(t, rebooted, stamp(back(19))))
	points := m.metricsHistory.GetGuestMetrics("agent:"+hostID, "netin", 24*time.Hour)
	if host.NetInRate != 0 || len(points) == 0 || points[len(points)-1].Value != 0 {
		t.Fatalf("counter reset = rate %.1f, last history %+v, want the existing measured-zero rebase", host.NetInRate, points)
	}
	next := agent.hostReport("stream-b", 9, 20, back(20))
	slots := uint64(agent.interval / time.Second)
	next.Network[0].RXBytes, next.Network[0].TXBytes = counterClockNetInPerSecond*slots, counterClockNetOutPerSecond*slots
	next.DiskIO[0].ReadBytes, next.DiskIO[0].WriteBytes, next.DiskIO[0].IOTime = counterClockReadPerSecond*slots, counterClockWritePerSecond*slots, counterClockBusyMsPerSecond*slots
	wantHostTrueRates(t, "first interval after the reset", applyCounterClockHost(t, m, ioCounterWireReport(t, next, stamp(back(20)))))
}
