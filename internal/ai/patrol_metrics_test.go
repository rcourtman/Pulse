package ai

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPatrolMetrics_StreamRecorders(t *testing.T) {
	m := GetPatrolMetrics()
	m.RecordStreamReplay(3)
	m.RecordStreamSnapshot("buffer_rotated")
	m.RecordStreamMiss()

	if got := testutil.CollectAndCount(m.streamReplayBatch); got != 1 {
		t.Fatalf("expected stream replay batch histogram to be registered once, got %d", got)
	}
}
