package ai

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
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

func TestPatrolMetrics_ObserveAttentionProjectionUsesCanonicalStates(t *testing.T) {
	m := GetPatrolMetrics()
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	m.ObserveAttentionProjection(AttentionProjection{
		Details: []AttentionItemDetail{{
			Item: AttentionItem{
				State:           operationaltrust.OperationalOpen,
				FirstObservedAt: now.Add(-time.Minute),
			},
		}},
		Summary: AttentionSummary{
			ActiveCount: 1,
			EvaluatedAt: now,
		},
	}, now)

	if got := testutil.ToFloat64(
		m.attentionItems.WithLabelValues(string(operationaltrust.OperationalOpen)),
	); got != 1 {
		t.Fatalf("open attention gauge = %v, want 1", got)
	}
	if got := testutil.ToFloat64(
		m.attentionItems.WithLabelValues(string(operationaltrust.OperationalResolved)),
	); got != 0 {
		t.Fatalf("resolved attention gauge = %v, want 0", got)
	}
}
