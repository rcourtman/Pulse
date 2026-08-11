package metrics

import (
	"testing"
	"time"
)

// Regression coverage for #1437: WriteBatchSync sits on the monitoring
// pipeline (state broadcast, agent ingest, poll publish). When the ingestion
// worker cannot keep up with the disk, the call must return within its wait
// budget instead of stalling the monitor until polling stops.

// newUnservicedStore builds a bare store whose ingestion worker never runs,
// modelling a writer wedged behind slow SQLite maintenance.
func newUnservicedStore(queueCap int) *Store {
	return &Store{
		writeCh: make(chan writeRequest, queueCap),
		stopCh:  make(chan struct{}),
	}
}

func backpressureProbeMetric() WriteMetric {
	return WriteMetric{
		ResourceType: "vm",
		ResourceID:   "backpressure-probe",
		MetricType:   "cpu",
		Value:        1,
		Timestamp:    time.Unix(1_700_000_000, 0),
		Tier:         TierRaw,
	}
}

func requireReturnsWithinWaitBudget(t *testing.T, s *Store, label string) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		s.WriteBatchSync([]WriteMetric{backpressureProbeMetric()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(syncWriteWaitTimeout + 3*time.Second):
		t.Fatalf("WriteBatchSync stalled past its wait budget (%s)", label)
	}
}

func TestWriteBatchSyncReturnsWhenQueueSaturated(t *testing.T) {
	s := newUnservicedStore(1)
	s.writeCh <- writeRequest{metrics: []bufferedMetric{{}}}

	requireReturnsWithinWaitBudget(t, s, "saturated queue")

	if got := len(s.writeCh); got != 1 {
		t.Fatalf("saturated queue depth = %d, want the pre-existing batch only", got)
	}
}

func TestWriteBatchSyncReturnsWhenCommitLags(t *testing.T) {
	s := newUnservicedStore(4)

	requireReturnsWithinWaitBudget(t, s, "lagging commit")

	if got := len(s.writeCh); got != 1 {
		t.Fatalf("queued batches = %d, want 1 (batch must stay queued, not be dropped)", got)
	}
}

func TestWriteBatchSyncReturnsOnClosedStore(t *testing.T) {
	s := newUnservicedStore(1)
	s.writeCh <- writeRequest{metrics: []bufferedMetric{{}}}
	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.WriteBatchSync([]WriteMetric{backpressureProbeMetric()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WriteBatchSync did not observe store shutdown")
	}
}
