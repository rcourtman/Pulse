package monitoring

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// counterClockAgent is a fake Unified Agent. It collects once per interval on
// its own clock and its cumulative counters grow at constant rates, so every
// pair of its samples has one true rate. Pulse receipt time is whatever the
// test's back-to-back calls take: the reconnect burst of a drained FIFO.
type counterClockAgent struct {
	start    time.Time
	interval time.Duration
}

const (
	counterClockNetInPerSecond  = 1_000_000
	counterClockNetOutPerSecond = 250_000
	counterClockReadPerSecond   = 4_000_000
	counterClockWritePerSecond  = 2_000_000
	// Busy milliseconds per second: ten per cent of the interval.
	counterClockBusyMsPerSecond = 100
)

var counterClockTrueRates = map[string]float64{
	"agent netin":     counterClockNetInPerSecond,
	"agent netout":    counterClockNetOutPerSecond,
	"agent diskread":  counterClockReadPerSecond,
	"agent diskwrite": counterClockWritePerSecond,
	"sda diskread":    counterClockReadPerSecond,
	"sda diskwrite":   counterClockWritePerSecond,
	"sda disk":        counterClockBusyMsPerSecond / 10.0,
}

func newCounterClockAgent(start time.Time) counterClockAgent {
	return counterClockAgent{start: start.UTC().Truncate(time.Second), interval: 30 * time.Second}
}

func (a counterClockAgent) collectedAt(slot int) time.Time {
	return a.start.Add(time.Duration(slot) * a.interval)
}

func (a counterClockAgent) counter(perSecond uint64, slot int) uint64 {
	return 1_000_000_000_000 + perSecond*uint64(slot)*uint64(a.interval/time.Second)
}

// hostReport is the report holding slot's counters, authored at authoredAt
// and sequenced in stream. An empty stream is a legacy report, and a zero
// authoredAt a report without a timestamp. Host.UptimeSeconds marks the slot.
func (a counterClockAgent) hostReport(stream string, seq uint64, slot int, authoredAt time.Time) agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "counter-clock-agent", Version: "6.6.0", Type: "unified", IntervalSeconds: int(a.interval / time.Second)},
		Host: agentshost.HostInfo{
			ID:            "counter-clock-host",
			MachineID:     "counter-clock-machine",
			Hostname:      "counter-clock-host",
			Platform:      "linux",
			UptimeSeconds: int64(slot),
		},
		Network: []agentshost.NetworkInterface{{
			Name:    "eth0",
			RXBytes: a.counter(counterClockNetInPerSecond, slot),
			TXBytes: a.counter(counterClockNetOutPerSecond, slot),
		}},
		DiskIO: []agentshost.DiskIO{{
			Device:     "sda",
			ReadBytes:  a.counter(counterClockReadPerSecond, slot),
			WriteBytes: a.counter(counterClockWritePerSecond, slot),
			IOTime:     a.counter(counterClockBusyMsPerSecond, slot),
		}},
		Timestamp:  authoredAt,
		SequenceID: agentshost.FormatReportSequenceID(stream, seq),
	}
}

func newCounterClockHostMonitor(t *testing.T) *Monitor {
	t.Helper()
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	return &Monitor{
		state:               models.NewState(),
		resourceStore:       unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil)),
		hostContinuityStore: config.NewHostContinuityStore(t.TempDir(), nil),
		rateTracker:         NewRateTracker(),
		metricsHistory:      NewMetricsHistory(1000, 24*time.Hour),
	}
}

// applyCounterClockHost applies report and requires it to own host state.
func applyCounterClockHost(t *testing.T, m *Monitor, report agentshost.Report) models.Host {
	t.Helper()
	host, err := m.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport slot %d: %v", report.Host.UptimeSeconds, err)
	}
	if host.UptimeSeconds != report.Host.UptimeSeconds {
		t.Fatalf("slot %d report %q was not applied; host holds slot %d", report.Host.UptimeSeconds, report.SequenceID, host.UptimeSeconds)
	}
	return host
}

func wantTrueRate(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > want*1e-9 {
		t.Fatalf("%s = %.1f, want the agent's real rate %.1f", what, got, want)
	}
}

func wantHostTrueRates(t *testing.T, label string, host models.Host) {
	t.Helper()
	wantTrueRate(t, label+" NetInRate", host.NetInRate, counterClockNetInPerSecond)
	wantTrueRate(t, label+" NetOutRate", host.NetOutRate, counterClockNetOutPerSecond)
	wantTrueRate(t, label+" DiskReadRate", host.DiskReadRate, counterClockReadPerSecond)
	wantTrueRate(t, label+" DiskWriteRate", host.DiskWriteRate, counterClockWritePerSecond)
}

// wantHostRatesUnknown requires host to carry no rate. Every fake counter
// moves, so a measured rate would be positive.
func wantHostRatesUnknown(t *testing.T, label string, host models.Host) {
	t.Helper()
	if host.NetInRate != 0 || host.NetOutRate != 0 || host.DiskReadRate != 0 || host.DiskWriteRate != 0 {
		t.Fatalf("%s reported rates without a comparable interval: in %.1f out %.1f read %.1f write %.1f",
			label, host.NetInRate, host.NetOutRate, host.DiskReadRate, host.DiskWriteRate)
	}
}

// counterClockHistory returns every rate point recorded for the host and its
// sda disk, by series.
func counterClockHistory(m *Monitor, hostID string) map[string][]float64 {
	series := make(map[string][]float64)
	for _, metric := range []string{"netin", "netout", "diskread", "diskwrite"} {
		for _, point := range m.metricsHistory.GetGuestMetrics("agent:"+hostID, metric, 24*time.Hour) {
			series["agent "+metric] = append(series["agent "+metric], point.Value)
		}
	}
	diskID := hostID + ":" + normalizeHostDiskDevice("sda")
	for _, metric := range []string{"diskread", "diskwrite", "disk"} {
		for _, point := range m.metricsHistory.GetDiskMetrics(diskID, metric, 24*time.Hour) {
			series["sda "+metric] = append(series["sda "+metric], point.Value)
		}
	}
	return series
}

// wantHistoryTrueRates requires every rate series to hold points and every
// point to be the agent's real rate.
func wantHistoryTrueRates(t *testing.T, m *Monitor, hostID string) {
	t.Helper()
	series := counterClockHistory(m, hostID)
	for name, want := range counterClockTrueRates {
		if len(series[name]) == 0 {
			t.Fatalf("%s history is empty", name)
		}
		for i, got := range series[name] {
			wantTrueRate(t, fmt.Sprintf("%s history point %d", name, i), got, want)
		}
	}
}

// A FIFO drained on reconnect delivers reports collected 30s apart within
// milliseconds of each other. Each is newer than the last accepted report, so
// the stream watermark admits them all, and each must measure its counters
// over the 30s it really covers rather than over the receipt gap.
func TestApplyHostReportDrainedFIFORatesUseAuthoredCollectionInterval(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	// Collected two days ago and held through the outage, so rate-series
	// cleanup must judge freshness by receipt, not by the sample clock.
	agent := newCounterClockAgent(time.Now().Add(-48 * time.Hour))

	first := applyCounterClockHost(t, m, agent.hostReport("stream-a", 1, 1, agent.collectedAt(1)))
	wantHostRatesUnknown(t, "first sample", first)
	for slot := 2; slot <= 11; slot++ {
		host := applyCounterClockHost(t, m, agent.hostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot)))
		wantHostTrueRates(t, fmt.Sprintf("drained slot %d", slot), host)
	}
	wantHistoryTrueRates(t, m, first.ID)

	// Every series was received just now. Expiring it by its two-day-old
	// sample time would leave the next report without a baseline.
	m.cleanupRateTracker(time.Now())
	next := applyCounterClockHost(t, m, agent.hostReport("stream-a", 12, 12, agent.collectedAt(12)))
	wantHostTrueRates(t, "report after rate-series cleanup", next)
}

// An agent restart opens a new stream on the same machine clock. Its first
// report is measured from the old stream's last accepted report, and a
// delayed old-stream report is rejected without touching rates or history.
func TestApplyHostReportStreamChangeKeepsMachineClockAndRetiresOldStream(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	for slot := 1; slot <= 3; slot++ {
		applyCounterClockHost(t, m, agent.hostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot)))
	}

	restarted := applyCounterClockHost(t, m, agent.hostReport("stream-b", 1, 4, agent.collectedAt(4)))
	wantHostTrueRates(t, "first report of the new stream", restarted)

	before := counterClockHistory(m, restarted.ID)
	delayed, err := m.ApplyHostReport(agent.hostReport("stream-a", 4, 3, agent.collectedAt(3)), nil)
	if err != nil {
		t.Fatalf("ApplyHostReport delayed old-stream report: %v", err)
	}
	if delayed.UptimeSeconds != 4 {
		t.Fatalf("retired-stream report replaced state: host holds slot %d, want 4", delayed.UptimeSeconds)
	}
	wantHostTrueRates(t, "stored host returned for the retired stream", delayed)
	if after := counterClockHistory(m, restarted.ID); !reflect.DeepEqual(after, before) {
		t.Fatalf("retired-stream report wrote rate history:\nbefore %v\nafter  %v", before, after)
	}

	next := applyCounterClockHost(t, m, agent.hostReport("stream-b", 2, 5, agent.collectedAt(5)))
	wantHostTrueRates(t, "second report of the new stream", next)
	wantHistoryTrueRates(t, m, next.ID)
}

// Each clock transition either has a comparable interval on one clock or has
// none. Without one, no rate is reported and no rate history is written:
// neither an authored time subtracted from a receipt baseline, nor a huge or
// zero rate from an interval that went backwards or did not advance.
func TestApplyHostReportClockTransitionsReportUnknownWithoutComparableInterval(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-6 * time.Hour))
	steppedBack := func(slot int) time.Time { return agent.collectedAt(slot).Add(-time.Hour) }
	steppedForward := func(slot int) time.Time { return agent.collectedAt(slot).Add(time.Hour) }

	// A legacy report sets a receipt-time baseline.
	hostID := applyCounterClockHost(t, m, agent.hostReport("", 0, 1, agent.collectedAt(1))).ID

	steps := []struct {
		name       string
		seq        uint64
		slot       int
		authoredAt time.Time
		want       string
	}{
		{"upgrade to a sequenced stream after a receipt-time baseline", 1, 2, agent.collectedAt(2), "unknown"},
		{"second authored sample", 2, 3, agent.collectedAt(3), "true"},
		{"sequenced report without a timestamp", 3, 4, time.Time{}, "unknown"},
		{"timestamp returns", 4, 5, agent.collectedAt(5), "unknown"},
		{"authored interval again", 5, 6, agent.collectedAt(6), "true"},
		{"clock steps back an hour", 6, 7, steppedBack(7), "unknown"},
		{"first sample on the stepped clock", 7, 8, steppedBack(8), "unknown"},
		{"interval on the stepped clock", 8, 9, steppedBack(9), "true"},
		{"equal timestamp with the next sequence", 9, 10, steppedBack(9), "unknown"},
		{"first sample after the repeated timestamp", 10, 11, steppedBack(11), "unknown"},
		{"interval after the repeated timestamp", 11, 12, steppedBack(12), "true"},
		// A forward step is indistinguishable from a longer collection gap,
		// which rates average over: the sample may under-report, never spike.
		{"clock steps forward two hours", 12, 13, steppedForward(13), "no spike"},
		{"interval on the forward clock", 13, 14, steppedForward(14), "true"},
	}
	for _, step := range steps {
		before := counterClockHistory(m, hostID)
		host := applyCounterClockHost(t, m, agent.hostReport("stream-a", step.seq, step.slot, step.authoredAt))
		switch step.want {
		case "unknown":
			wantHostRatesUnknown(t, step.name, host)
			if after := counterClockHistory(m, hostID); !reflect.DeepEqual(after, before) {
				t.Fatalf("%s wrote rate history without a comparable interval:\nbefore %v\nafter  %v", step.name, before, after)
			}
		case "true":
			wantHostTrueRates(t, step.name, host)
		case "no spike":
			for _, rate := range []struct {
				what      string
				got, want float64
			}{
				{"NetInRate", host.NetInRate, counterClockNetInPerSecond},
				{"NetOutRate", host.NetOutRate, counterClockNetOutPerSecond},
				{"DiskReadRate", host.DiskReadRate, counterClockReadPerSecond},
				{"DiskWriteRate", host.DiskWriteRate, counterClockWritePerSecond},
			} {
				if rate.got <= 0 || rate.got > rate.want {
					t.Fatalf("%s %s = %.1f, want a positive rate no higher than the real %.1f", step.name, rate.what, rate.got, rate.want)
				}
			}
		}
	}
	for name, values := range counterClockHistory(m, hostID) {
		for i, got := range values {
			if got <= 0 || got > counterClockTrueRates[name]*(1+1e-9) {
				t.Fatalf("%s history point %d = %.1f, want a positive rate no higher than %.1f", name, i, got, counterClockTrueRates[name])
			}
		}
	}
}

// Reports delivered at the same time race for the per-host lock. Whatever
// order they win in, the watermark admits an increasing subset, each accepted
// report is measured over the authored interval since the previously accepted
// one, a rejected report returns the stored host, and the newest report owns
// state.
func TestApplyHostReportConcurrentDeliveryMeasuresAcceptedReportsOnAuthoredClock(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	first := applyCounterClockHost(t, m, agent.hostReport("stream-a", 1, 1, agent.collectedAt(1)))

	const last = 16
	results := make([]models.Host, last+1)
	errs := make([]error, last+1)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for slot := 2; slot <= last; slot++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			<-release
			results[slot], errs[slot] = m.ApplyHostReport(agent.hostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot)), nil)
		}(slot)
	}
	close(release)
	wg.Wait()

	for slot := 2; slot <= last; slot++ {
		if errs[slot] != nil {
			t.Fatalf("ApplyHostReport slot %d: %v", slot, errs[slot])
		}
		wantHostTrueRates(t, fmt.Sprintf("concurrent slot %d", slot), results[slot])
	}
	stored, ok := m.state.GetHost(first.ID)
	if !ok || stored.UptimeSeconds != last {
		t.Fatalf("newest report must own state: found %v holding slot %d, want slot %d", ok, stored.UptimeSeconds, last)
	}
	wantHostTrueRates(t, "stored host", stored)
	wantHistoryTrueRates(t, m, first.ID)
}

// Legacy reports carry no stream, so their timestamps only guard the
// reconnect burst and never time a rate: rates stay on Pulse receipt time.
func TestApplyHostReportLegacyRatesStayOnReceiptTime(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	host := applyCounterClockHost(t, m, agent.hostReport("", 0, 1, agent.collectedAt(1)))

	// Unchanged counters are measured idle over the receipt interval.
	idle := agent.hostReport("", 0, 1, agent.collectedAt(2))
	idle.Host.UptimeSeconds = 2
	idleSentAt := time.Now()
	applyCounterClockHost(t, m, idle)
	points := m.metricsHistory.GetGuestMetrics("agent:"+host.ID, "netin", 24*time.Hour)
	if len(points) != 1 || points[0].Value != 0 {
		t.Fatalf("unchanged legacy counters must be measured idle: %+v", points)
	}

	// Two slots of counters authored one interval later: an authored-clock
	// rate would be twice the real rate, while the receipt interval is at
	// most the wall time these calls took.
	moved := applyCounterClockHost(t, m, agent.hostReport("", 0, 3, agent.collectedAt(3)))
	wall := time.Since(idleSentAt).Seconds()
	delta := float64(agent.counter(counterClockNetInPerSecond, 3) - agent.counter(counterClockNetInPerSecond, 1))
	if moved.NetInRate < delta/wall {
		t.Fatalf("legacy NetInRate = %.1f, want the receipt-interval rate of at least %.1f (authored interval would give %.1f)",
			moved.NetInRate, delta/wall, delta/agent.interval.Seconds())
	}

	// A backwards legacy timestamp inside the reconnect burst stays rejected.
	before := counterClockHistory(m, host.ID)
	late := agent.hostReport("", 0, 2, agent.collectedAt(2))
	stored, err := m.ApplyHostReport(late, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport late legacy report: %v", err)
	}
	if stored.UptimeSeconds != 3 {
		t.Fatalf("late legacy report replaced state: host holds slot %d, want 3", stored.UptimeSeconds)
	}
	if after := counterClockHistory(m, host.ID); !reflect.DeepEqual(after, before) {
		t.Fatalf("late legacy report wrote rate history:\nbefore %v\nafter  %v", before, after)
	}
}

// The Docker module follows the same stream ordering. A drained FIFO measures
// container counters on the authored interval, and a helper-degraded status
// report between complete reports neither replaces the inventory nor moves a
// rate baseline, so the next complete report spans both intervals.
func TestApplyDockerReportDrainedFIFOContainerRatesUseAuthoredCollectionInterval(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	m := newTestMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))

	report := func(seq uint64, slot int, complete bool) agentsdocker.Report {
		inventoryComplete := complete
		r := agentsdocker.Report{
			Agent:             agentsdocker.AgentInfo{ID: "counter-clock-docker", Version: "6.6.0", Type: "unified", IntervalSeconds: 30},
			Host:              agentsdocker.HostInfo{Hostname: "counter-clock-docker-host"},
			InventoryComplete: &inventoryComplete,
			Timestamp:         agent.collectedAt(slot),
			SequenceID:        agentshost.FormatReportSequenceID("docker-stream", seq),
		}
		if complete {
			r.Containers = []agentsdocker.Container{{
				ID:             "counter-clock-container",
				Name:           "api",
				State:          "running",
				NetworkRXBytes: agent.counter(counterClockNetInPerSecond, slot),
				NetworkTXBytes: agent.counter(counterClockNetOutPerSecond, slot),
				BlockIO: &agentsdocker.ContainerBlockIO{
					ReadBytes:  agent.counter(counterClockReadPerSecond, slot),
					WriteBytes: agent.counter(counterClockWritePerSecond, slot),
				},
			}}
		}
		return r
	}
	apply := func(seq uint64, slot int, complete bool) models.DockerContainer {
		t.Helper()
		host, err := m.ApplyDockerReport(report(seq, slot, complete), nil)
		if err != nil {
			t.Fatalf("ApplyDockerReport slot %d: %v", slot, err)
		}
		if len(host.Containers) != 1 {
			t.Fatalf("slot %d containers = %+v, want the one running container", slot, host.Containers)
		}
		return host.Containers[0]
	}
	wantContainerTrueRates := func(label string, container models.DockerContainer) {
		t.Helper()
		wantTrueRate(t, label+" NetInRate", container.NetInRate, counterClockNetInPerSecond)
		wantTrueRate(t, label+" NetOutRate", container.NetOutRate, counterClockNetOutPerSecond)
		if container.BlockIO == nil || container.BlockIO.ReadRateBytesPerSecond == nil || container.BlockIO.WriteRateBytesPerSecond == nil {
			t.Fatalf("%s block I/O rates missing: %+v", label, container.BlockIO)
		}
		wantTrueRate(t, label+" read rate", *container.BlockIO.ReadRateBytesPerSecond, counterClockReadPerSecond)
		wantTrueRate(t, label+" write rate", *container.BlockIO.WriteRateBytesPerSecond, counterClockWritePerSecond)
	}
	containerHistory := func() map[string][]float64 {
		series := make(map[string][]float64)
		for _, metric := range []string{"netin", "netout", "diskread", "diskwrite"} {
			for _, point := range m.metricsHistory.GetGuestMetrics("docker:counter-clock-container", metric, 24*time.Hour) {
				series[metric] = append(series[metric], point.Value)
			}
		}
		return series
	}

	apply(1, 1, true)
	for metric, values := range containerHistory() {
		if len(values) != 0 {
			t.Fatalf("first sample wrote %s history without an interval: %v", metric, values)
		}
	}
	for slot := 2; slot <= 6; slot++ {
		wantContainerTrueRates(fmt.Sprintf("drained slot %d", slot), apply(uint64(slot), slot, true))
	}

	before := containerHistory()
	retained := apply(7, 7, false)
	wantContainerTrueRates("container retained through the status-only report", retained)
	if after := containerHistory(); !reflect.DeepEqual(after, before) {
		t.Fatalf("status-only report wrote container history:\nbefore %v\nafter  %v", before, after)
	}

	wantContainerTrueRates("complete report after the status-only report", apply(8, 8, true))
	before = containerHistory()
	fault := report(9, 9, true)
	fault.Timestamp = agent.collectedAt(8)
	if _, err := m.ApplyDockerReport(fault, nil); err != nil {
		t.Fatal(err)
	}
	if after := containerHistory(); !reflect.DeepEqual(after, before) {
		t.Fatalf("repeated source time wrote unknown rates as history: before %v after %v", before, after)
	}
	apply(10, 10, true) // First baseline after dropping the ambiguous sample.
	wantContainerTrueRates("interval after the clock fault", apply(11, 11, true))
	series := containerHistory()
	for metric, want := range map[string]float64{
		"netin": counterClockNetInPerSecond, "netout": counterClockNetOutPerSecond,
		"diskread": counterClockReadPerSecond, "diskwrite": counterClockWritePerSecond,
	} {
		values := series[metric]
		if len(values) == 0 {
			t.Fatalf("container %s history is empty", metric)
		}
		for i, got := range values {
			wantTrueRate(t, fmt.Sprintf("container %s history point %d", metric, i), got, want)
		}
	}
	idle := report(12, 12, true)
	idle.Containers = report(11, 11, true).Containers
	if _, err := m.ApplyDockerReport(idle, nil); err != nil {
		t.Fatal(err)
	}
	for metric, values := range containerHistory() {
		if len(values) == 0 || values[len(values)-1] != 0 {
			t.Fatalf("measured idle %s did not retain zero: %v", metric, values)
		}
	}
}
