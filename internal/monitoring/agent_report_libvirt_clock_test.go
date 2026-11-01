package monitoring

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

const (
	// One vCPU-second per second across four vCPUs: 25% of capacity.
	libvirtClockCPUNanosPerSecond = 1_000_000_000
	libvirtClockVCPUs             = 4
	libvirtClockNetInPerSecond    = 500_000
	libvirtClockNetOutPerSecond   = 125_000
	libvirtClockReadPerSecond     = 2_000_000
	libvirtClockWritePerSecond    = 1_000_000
	// The agent stamps CollectedAt before it reads domain stats and the report
	// Timestamp after every collector, so CollectedAt sits earlier.
	libvirtClockCollectedLead = 7 * time.Second
)

var libvirtClockTrueRates = map[string]float64{
	"cpu":       100.0 / libvirtClockVCPUs,
	"netin":     libvirtClockNetInPerSecond,
	"netout":    libvirtClockNetOutPerSecond,
	"diskread":  libvirtClockReadPerSecond,
	"diskwrite": libvirtClockWritePerSecond,
}

func TestApplyHostReportLibvirtCPUCounterRetainsUnsignedRange(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	for slot := 1; slot <= 3; slot++ {
		domain := agent.libvirtDomain(slot, 0)
		// A busy, long-running domain can accumulate more than MaxInt64
		// nanoseconds across its vCPUs. Clamping both samples loses the delta.
		domain.CPUTimeNanoseconds = uint64(math.MaxInt64) - 15_000_000_000 + uint64(slot)*30_000_000_000
		report := agent.libvirtHostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot), agent.libvirtCollectedAt(slot), domain)
		got := libvirtClockDomain(t, "unsigned CPU counter", applyCounterClockHost(t, m, report))
		if slot > 1 {
			wantLibvirtTrueRates(t, "unsigned CPU counter", got)
		}
	}
}

// libvirtCollectedAt is when the fake agent read slot's domain stats.
func (a counterClockAgent) libvirtCollectedAt(slot int) time.Time {
	return a.collectedAt(slot).Add(-libvirtClockCollectedLead)
}

// libvirtCounter is a domain counter at slot. A domain restarted at
// restartSlot counts again from zero; restartSlot 0 means it never restarted.
func (a counterClockAgent) libvirtCounter(perSecond uint64, slot, restartSlot int) uint64 {
	if restartSlot == 0 {
		return a.counter(perSecond, slot)
	}
	return perSecond * uint64(slot-restartSlot) * uint64(a.interval/time.Second)
}

func (a counterClockAgent) libvirtDomain(slot, restartSlot int) agentshost.LibvirtDomain {
	return agentshost.LibvirtDomain{
		Name:               "app",
		State:              "running",
		VCPUs:              libvirtClockVCPUs,
		CPUTimeNanoseconds: a.libvirtCounter(libvirtClockCPUNanosPerSecond, slot, restartSlot),
		MemoryCurrentBytes: 2 << 30,
		MemoryMaximumBytes: 4 << 30,
		NetworkRXBytes:     a.libvirtCounter(libvirtClockNetInPerSecond, slot, restartSlot),
		NetworkTXBytes:     a.libvirtCounter(libvirtClockNetOutPerSecond, slot, restartSlot),
		DiskReadBytes:      a.libvirtCounter(libvirtClockReadPerSecond, slot, restartSlot),
		DiskWriteBytes:     a.libvirtCounter(libvirtClockWritePerSecond, slot, restartSlot),
	}
}

// libvirtHostReport is hostReport carrying a readable libvirt inventory
// collected at collectedAt (zero: an inventory without CollectedAt), holding
// the "app" domain unless it is omitted.
func (a counterClockAgent) libvirtHostReport(stream string, seq uint64, slot int, authoredAt, collectedAt time.Time, domains ...agentshost.LibvirtDomain) agentshost.Report {
	report := a.hostReport(stream, seq, slot, authoredAt)
	report.Libvirt = &agentshost.LibvirtInventory{
		Domains:     append([]agentshost.LibvirtDomain{}, domains...),
		CollectedAt: collectedAt,
	}
	return report
}

func libvirtClockDomain(t *testing.T, label string, host models.Host) models.HostLibvirtDomain {
	t.Helper()
	if host.Libvirt == nil || len(host.Libvirt.Domains) != 1 {
		t.Fatalf("%s libvirt inventory = %+v, want the app domain", label, host.Libvirt)
	}
	return host.Libvirt.Domains[0]
}

func wantLibvirtTrueRates(t *testing.T, label string, domain models.HostLibvirtDomain) {
	t.Helper()
	if !domain.CPUUsageValid || !domain.IORatesValid {
		t.Fatalf("%s reported no rate (cpu valid %v, io valid %v), want the domain's real rates", label, domain.CPUUsageValid, domain.IORatesValid)
	}
	wantTrueRate(t, label+" CPU usage", domain.CPUUsagePercent, libvirtClockTrueRates["cpu"])
	wantTrueRate(t, label+" NetworkInRate", domain.NetworkInRate, libvirtClockNetInPerSecond)
	wantTrueRate(t, label+" NetworkOutRate", domain.NetworkOutRate, libvirtClockNetOutPerSecond)
	wantTrueRate(t, label+" DiskReadRate", domain.DiskReadRate, libvirtClockReadPerSecond)
	wantTrueRate(t, label+" DiskWriteRate", domain.DiskWriteRate, libvirtClockWritePerSecond)
}

func wantLibvirtUnknown(t *testing.T, label string, domain models.HostLibvirtDomain) {
	t.Helper()
	if domain.CPUUsageValid || domain.IORatesValid {
		t.Fatalf("%s reported rates without a comparable interval: cpu %.1f (valid %v), in %.1f (valid %v)",
			label, domain.CPUUsagePercent, domain.CPUUsageValid, domain.NetworkInRate, domain.IORatesValid)
	}
}

// libvirtClockHistory returns every CPU and I/O history point recorded for
// the app domain, by metric.
func libvirtClockHistory(m *Monitor, hostID string) map[string][]float64 {
	metricID := libvirtDomainMetricID(hostID, agentLibvirtDomainID("app"))
	series := make(map[string][]float64)
	for metric := range libvirtClockTrueRates {
		for _, point := range m.metricsHistory.GetGuestMetrics(metricID, metric, 24*time.Hour) {
			series[metric] = append(series[metric], point.Value)
		}
	}
	return series
}

func wantLibvirtHistoryTrueRates(t *testing.T, m *Monitor, hostID string) {
	t.Helper()
	series := libvirtClockHistory(m, hostID)
	for metric, want := range libvirtClockTrueRates {
		if len(series[metric]) == 0 {
			t.Fatalf("libvirt %s history is empty", metric)
		}
		for i, got := range series[metric] {
			wantTrueRate(t, fmt.Sprintf("libvirt %s history point %d", metric, i), got, want)
		}
	}
}

// A drained FIFO delivers host reports collected 30s apart within
// milliseconds. The libvirt domain's CPU and I/O counters inside each report
// must be measured over the 30s between their CollectedAt stamps, not over the
// receipt gap, and not over the report Timestamp, which the agent stamps after
// a variable-length tail of other collectors.
func TestApplyHostReportDrainedFIFOLibvirtRatesUseCollectedAt(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-48 * time.Hour))
	authoredAt := func(slot int) time.Time {
		return agent.collectedAt(slot).Add(time.Duration(slot%3) * 4 * time.Second)
	}
	report := func(slot int) agentshost.Report {
		return agent.libvirtHostReport("stream-a", uint64(slot), slot, authoredAt(slot), agent.libvirtCollectedAt(slot), agent.libvirtDomain(slot, 0))
	}

	first := applyCounterClockHost(t, m, report(1))
	wantLibvirtUnknown(t, "first sample", libvirtClockDomain(t, "first sample", first))
	for slot := 2; slot <= 11; slot++ {
		host := applyCounterClockHost(t, m, report(slot))
		wantLibvirtTrueRates(t, fmt.Sprintf("drained slot %d", slot), libvirtClockDomain(t, "drained", host))
	}
	wantLibvirtHistoryTrueRates(t, m, first.ID)

	// Both domain series were received just now; cleanup keyed on their
	// two-day-old sample clock would drop the baselines.
	m.cleanupRateTracker(time.Now())
	next := applyCounterClockHost(t, m, report(12))
	wantLibvirtTrueRates(t, "report after rate-series cleanup", libvirtClockDomain(t, "after cleanup", next))
}

// Each libvirt clock transition either has a comparable CollectedAt interval
// or reports no CPU or I/O rate and writes no rate history for that sample.
func TestApplyHostReportLibvirtClockTransitionsReportUnknownWithoutComparableInterval(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-6 * time.Hour))
	steppedBack := func(slot int) time.Time { return agent.libvirtCollectedAt(slot).Add(-time.Hour) }

	// A legacy report sets receipt-time baselines even though it carries
	// CollectedAt: without a stream it has no ordered source clock.
	legacy := agent.libvirtHostReport("", 0, 1, agent.collectedAt(1), agent.libvirtCollectedAt(1), agent.libvirtDomain(1, 0))
	hostID := applyCounterClockHost(t, m, legacy).ID

	steps := []struct {
		name        string
		seq         uint64
		slot        int
		authoredAt  time.Time
		collectedAt time.Time
		want        string
	}{
		{"upgrade to a sequenced stream after receipt-time baselines", 1, 2, agent.collectedAt(2), agent.libvirtCollectedAt(2), "unknown"},
		{"second CollectedAt sample", 2, 3, agent.collectedAt(3), agent.libvirtCollectedAt(3), "true"},
		{"report without a Timestamp keeps the inventory's own clock", 3, 4, time.Time{}, agent.libvirtCollectedAt(4), "true"},
		{"inventory without CollectedAt", 4, 5, agent.collectedAt(5), time.Time{}, "unknown"},
		{"CollectedAt returns", 5, 6, agent.collectedAt(6), agent.libvirtCollectedAt(6), "unknown"},
		{"CollectedAt interval again", 6, 7, agent.collectedAt(7), agent.libvirtCollectedAt(7), "true"},
		{"clock steps back an hour", 7, 8, agent.collectedAt(8).Add(-time.Hour), steppedBack(8), "unknown"},
		{"first sample on the stepped clock", 8, 9, agent.collectedAt(9).Add(-time.Hour), steppedBack(9), "unknown"},
		{"interval on the stepped clock", 9, 10, agent.collectedAt(10).Add(-time.Hour), steppedBack(10), "true"},
		{"equal CollectedAt with the next sequence", 10, 11, agent.collectedAt(11).Add(-time.Hour), steppedBack(10), "unknown"},
		{"first sample after the repeated CollectedAt", 11, 12, agent.collectedAt(12).Add(-time.Hour), steppedBack(12), "unknown"},
		{"interval after the repeated CollectedAt", 12, 13, agent.collectedAt(13).Add(-time.Hour), steppedBack(13), "true"},
	}
	for _, step := range steps {
		before := libvirtClockHistory(m, hostID)
		host := applyCounterClockHost(t, m, agent.libvirtHostReport("stream-a", step.seq, step.slot, step.authoredAt, step.collectedAt, agent.libvirtDomain(step.slot, 0)))
		domain := libvirtClockDomain(t, step.name, host)
		switch step.want {
		case "unknown":
			wantLibvirtUnknown(t, step.name, domain)
			if after := libvirtClockHistory(m, hostID); !reflect.DeepEqual(after, before) {
				t.Fatalf("%s wrote libvirt rate history without a comparable interval:\nbefore %v\nafter  %v", step.name, before, after)
			}
		case "true":
			wantLibvirtTrueRates(t, step.name, domain)
		}
	}
	wantLibvirtHistoryTrueRates(t, m, hostID)
}

// An agent restart keeps the machine clock, so the new stream's first libvirt
// sample is measured from the old stream's last, and a delayed old-stream
// report changes neither the domain nor its history.
func TestApplyHostReportLibvirtStreamChangeKeepsMachineClock(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	report := func(stream string, seq uint64, slot int) agentshost.Report {
		return agent.libvirtHostReport(stream, seq, slot, agent.collectedAt(slot), agent.libvirtCollectedAt(slot), agent.libvirtDomain(slot, 0))
	}
	for slot := 1; slot <= 3; slot++ {
		applyCounterClockHost(t, m, report("stream-a", uint64(slot), slot))
	}

	restarted := applyCounterClockHost(t, m, report("stream-b", 1, 4))
	wantLibvirtTrueRates(t, "first report of the new stream", libvirtClockDomain(t, "restarted", restarted))

	before := libvirtClockHistory(m, restarted.ID)
	delayed, err := m.ApplyHostReport(report("stream-a", 4, 3), nil)
	if err != nil {
		t.Fatalf("ApplyHostReport delayed old-stream report: %v", err)
	}
	if delayed.UptimeSeconds != 4 {
		t.Fatalf("retired-stream report replaced state: host holds slot %d, want 4", delayed.UptimeSeconds)
	}
	wantLibvirtTrueRates(t, "stored domain returned for the retired stream", libvirtClockDomain(t, "delayed", delayed))
	if after := libvirtClockHistory(m, restarted.ID); !reflect.DeepEqual(after, before) {
		t.Fatalf("retired-stream report wrote libvirt history:\nbefore %v\nafter  %v", before, after)
	}

	next := applyCounterClockHost(t, m, report("stream-b", 2, 5))
	wantLibvirtTrueRates(t, "second report of the new stream", libvirtClockDomain(t, "next", next))
}

// Omissions keep their existing meaning: a readable inventory without the
// domain retires it, so CPU usage restarts on its return, while its
// cumulative I/O counters measure the whole span since their last sample; an
// inventory the agent failed to read preserves the last one, and the next
// read measures CPU from that last read rather than from the host report in
// between; a domain whose counters restart has no CPU usage for that sample.
func TestApplyHostReportLibvirtOmittedDomainInventoryAndRestart(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	report := func(seq uint64, slot int, domains ...agentshost.LibvirtDomain) agentshost.Report {
		return agent.libvirtHostReport("stream-a", seq, slot, agent.collectedAt(slot), agent.libvirtCollectedAt(slot), domains...)
	}
	for slot := 1; slot <= 2; slot++ {
		applyCounterClockHost(t, m, report(uint64(slot), slot, agent.libvirtDomain(slot, 0)))
	}

	omitted := applyCounterClockHost(t, m, report(3, 3))
	if omitted.Libvirt == nil || len(omitted.Libvirt.Domains) != 0 {
		t.Fatalf("readable inventory without the domain = %+v, want no domains", omitted.Libvirt)
	}

	returned := libvirtClockDomain(t, "returned", applyCounterClockHost(t, m, report(4, 4, agent.libvirtDomain(4, 0))))
	if returned.CPUUsageValid {
		t.Fatalf("domain absent from the previous report reported CPU usage %.1f", returned.CPUUsagePercent)
	}
	if !returned.IORatesValid {
		t.Fatal("returned domain lost its I/O rates")
	}
	wantTrueRate(t, "returned NetworkInRate", returned.NetworkInRate, libvirtClockNetInPerSecond)
	wantTrueRate(t, "returned DiskWriteRate", returned.DiskWriteRate, libvirtClockWritePerSecond)

	wantLibvirtTrueRates(t, "domain after its return", libvirtClockDomain(t, "slot 5", applyCounterClockHost(t, m, report(5, 5, agent.libvirtDomain(5, 0)))))

	unread := agent.hostReport("stream-a", 6, 6, agent.collectedAt(6))
	wantLibvirtTrueRates(t, "inventory preserved through a failed read", libvirtClockDomain(t, "slot 6", applyCounterClockHost(t, m, unread)))
	wantLibvirtTrueRates(t, "read after the failed one", libvirtClockDomain(t, "slot 7", applyCounterClockHost(t, m, report(7, 7, agent.libvirtDomain(7, 0)))))

	restarted := libvirtClockDomain(t, "restarted", applyCounterClockHost(t, m, report(8, 8, agent.libvirtDomain(8, 8))))
	if restarted.CPUUsageValid {
		t.Fatalf("restarted domain reported CPU usage %.1f across its counter reset", restarted.CPUUsagePercent)
	}
	if !restarted.IORatesValid || restarted.NetworkInRate != 0 || restarted.DiskReadRate != 0 {
		t.Fatalf("restarted domain I/O = %+v, want the counter-reset rebase of zero", restarted)
	}
	wantLibvirtTrueRates(t, "first interval after the restart", libvirtClockDomain(t, "slot 9", applyCounterClockHost(t, m, report(9, 9, agent.libvirtDomain(9, 8)))))
}

// Legacy reports have no ordered source clock, so their libvirt counters keep
// receipt time even when the inventory carries CollectedAt.
func TestApplyHostReportLegacyLibvirtRatesStayOnReceiptTime(t *testing.T) {
	m := newCounterClockHostMonitor(t)
	agent := newCounterClockAgent(time.Now().Add(-time.Hour))
	report := func(slot int) agentshost.Report {
		return agent.libvirtHostReport("", 0, slot, agent.collectedAt(slot), agent.libvirtCollectedAt(slot), agent.libvirtDomain(slot, 0))
	}
	// Start the clock before the first report so wall spans both receipt
	// stamps; starting after it let a slow first apply exceed the second's.
	sentAt := time.Now()
	applyCounterClockHost(t, m, report(1))
	domain := libvirtClockDomain(t, "legacy", applyCounterClockHost(t, m, report(2)))
	wall := time.Since(sentAt).Seconds()
	if !domain.CPUUsageValid || !domain.IORatesValid {
		t.Fatalf("legacy domain lost its receipt-time rates: %+v", domain)
	}
	delta := float64(agent.libvirtCounter(libvirtClockNetInPerSecond, 2, 0) - agent.libvirtCounter(libvirtClockNetInPerSecond, 1, 0))
	if domain.NetworkInRate < delta/wall {
		t.Fatalf("legacy NetworkInRate = %.1f, want the receipt-interval rate of at least %.1f (CollectedAt would give %.1f)",
			domain.NetworkInRate, delta/wall, delta/agent.interval.Seconds())
	}
}
