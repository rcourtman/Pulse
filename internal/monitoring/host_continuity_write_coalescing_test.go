package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newContinuityJournalMonitor(t *testing.T, dir string) *Monitor {
	t.Helper()
	m := newCounterClockHostMonitor(t)
	m.hostContinuityStore = config.NewHostContinuityStore(dir, nil)
	return m
}

func journalReportSequence(t *testing.T, dir, hostID string) uint64 {
	t.Helper()
	entry, ok := config.NewHostContinuityStore(dir, nil).Get(hostID)
	if !ok {
		t.Fatalf("journal has no entry for %s", hostID)
	}
	return entry.ReportSequence
}

// Every host report used to rewrite the whole continuity journal, which grows
// with the fleet: about 1.5 GiB/day of writes at 30 agents. A steady agent's
// reports now update the journal in memory, and Stop writes the exact
// watermark so the next process still rejects replays of accepted reports.
func TestSteadyHostReportsDeferJournalRewritesUntilStop(t *testing.T) {
	dir := t.TempDir()
	m := newContinuityJournalMonitor(t, dir)
	agent := newCounterClockAgent(time.Now().Add(-10 * time.Minute))

	first := applyCounterClockHost(t, m, agent.hostReport("stream-a", 1, 1, agent.collectedAt(1)))
	if got := journalReportSequence(t, dir, first.ID); got != 1 {
		t.Fatalf("first report watermark on disk = %d, want 1", got)
	}
	for slot := 2; slot <= 6; slot++ {
		applyCounterClockHost(t, m, agent.hostReport("stream-a", uint64(slot), slot, agent.collectedAt(slot)))
	}
	if got := journalReportSequence(t, dir, first.ID); got != 1 {
		t.Fatalf("steady reports rewrote the journal: on-disk watermark %d, want 1 until the flush", got)
	}

	m.Stop()
	if got := journalReportSequence(t, dir, first.ID); got != 6 {
		t.Fatalf("Stop left on-disk watermark %d, want 6", got)
	}

	restarted := newContinuityJournalMonitor(t, dir)
	replayed, err := restarted.ApplyHostReport(agent.hostReport("stream-a", 6, 6, agent.collectedAt(6)), nil)
	if err != nil {
		t.Fatalf("replay after restart: %v", err)
	}
	if replayed.UptimeSeconds == 6 {
		t.Fatal("restarted monitor accepted a replay of an already accepted report")
	}
	applyCounterClockHost(t, restarted, agent.hostReport("stream-a", 7, 7, agent.collectedAt(7)))
}

// A new report stream is the boundary that retires the old one, so it must be
// on disk before the report that opened it is acknowledged.
func TestNewHostReportStreamReachesJournalAtOnce(t *testing.T) {
	dir := t.TempDir()
	m := newContinuityJournalMonitor(t, dir)
	agent := newCounterClockAgent(time.Now().Add(-10 * time.Minute))

	host := applyCounterClockHost(t, m, agent.hostReport("stream-a", 1, 1, agent.collectedAt(1)))
	applyCounterClockHost(t, m, agent.hostReport("stream-a", 2, 2, agent.collectedAt(2)))
	applyCounterClockHost(t, m, agent.hostReport("stream-b", 1, 3, agent.collectedAt(3)))

	entry, ok := config.NewHostContinuityStore(dir, nil).Get(host.ID)
	if !ok || entry.ReportStreamID != "stream-b" || entry.ReportSequence != 1 {
		t.Fatalf("journal after a stream change = %+v, want stream-b at 1", entry)
	}
}
