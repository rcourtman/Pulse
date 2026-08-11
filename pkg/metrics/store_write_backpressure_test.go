package metrics

import (
	"testing"
	"time"
)

// Regression coverage for #1437: WriteBatchBounded sits on the monitoring
// pipeline (state broadcast, agent ingest, poll publish). When the ingestion
// worker cannot keep up with the disk, the call must return within its wait
// budget instead of stalling the monitor until polling stops. WriteBatchSync
// keeps its full commit wait — mock seeding and the write-path invariant
// tests read their writes straight back.

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

func requireBoundedReturn(t *testing.T, s *Store, label string) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		s.WriteBatchBounded([]WriteMetric{backpressureProbeMetric()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(syncWriteWaitTimeout + 3*time.Second):
		t.Fatalf("WriteBatchBounded stalled past its wait budget (%s)", label)
	}
}

func TestWriteBatchBoundedReturnsWhenQueueSaturated(t *testing.T) {
	s := newUnservicedStore(1)
	s.writeCh <- writeRequest{metrics: []bufferedMetric{{}}}

	requireBoundedReturn(t, s, "saturated queue")

	if got := len(s.writeCh); got != 1 {
		t.Fatalf("saturated queue depth = %d, want the pre-existing batch only", got)
	}
}

func TestWriteBatchBoundedReturnsWhenCommitLags(t *testing.T) {
	s := newUnservicedStore(4)

	requireBoundedReturn(t, s, "lagging commit")

	if got := len(s.writeCh); got != 1 {
		t.Fatalf("queued batches = %d, want 1 (batch must stay queued, not be dropped)", got)
	}
}

func TestWriteBatchBoundedReturnsOnClosedStore(t *testing.T) {
	s := newUnservicedStore(1)
	s.writeCh <- writeRequest{metrics: []bufferedMetric{{}}}
	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.WriteBatchBounded([]WriteMetric{backpressureProbeMetric()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WriteBatchBounded did not observe store shutdown")
	}
}

// WriteBatchSync must keep waiting for the commit well past the bounded
// budget — an early return would break read-your-writes for mock seeding and
// the write-path invariant tests on slow disks, which is exactly what a
// bounded WriteBatchSync did to CI (run 31494553977).
func TestWriteBatchSyncWaitsForCommitBeyondBoundedBudget(t *testing.T) {
	s := newUnservicedStore(4)

	returned := make(chan struct{})
	go func() {
		s.WriteBatchSync([]WriteMetric{backpressureProbeMetric()})
		close(returned)
	}()

	select {
	case <-returned:
		t.Fatal("WriteBatchSync returned before its batch committed")
	case <-time.After(syncWriteWaitTimeout + time.Second):
	}

	// Simulate the worker committing the queued batch: closing done must
	// release the waiting caller.
	req := <-s.writeCh
	if req.done == nil {
		t.Fatal("queued request carries no done channel")
	}
	close(req.done)

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("WriteBatchSync did not return after its commit completed")
	}
}
