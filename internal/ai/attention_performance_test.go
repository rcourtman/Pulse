package ai

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

func TestAttentionProjectionLargeCardinalityKeepsResponseBounded(t *testing.T) {
	const cardinality = 10_000
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	active := make([]alerts.Alert, cardinality)
	for index := range active {
		active[index] = attentionTestAlert(
			fmt.Sprintf("record-%05d", index),
			operationaltrust.OperationalOpen,
			operationaltrust.SeverityWarning,
			now.Add(-time.Duration(index)*time.Second),
			now,
		)
	}

	startedAt := time.Now()
	projection := ProjectAttentionItems(active, nil, nil, now)
	elapsed := time.Since(startedAt)
	if projection.Summary.ActiveCount != cardinality {
		t.Fatalf(
			"ActiveCount = %d, want %d",
			projection.Summary.ActiveCount,
			cardinality,
		)
	}
	page, err := PaginateAttentionDetails(projection.Details, 1, MaxAttentionPageSize)
	if err != nil {
		t.Fatalf("PaginateAttentionDetails(): %v", err)
	}
	if len(page) != MaxAttentionPageSize {
		t.Fatalf("page size = %d, want %d", len(page), MaxAttentionPageSize)
	}
	// This is deliberately generous enough for shared CI while still catching
	// accidental quadratic projection or an external request per item.
	if elapsed > 5*time.Second {
		t.Fatalf("10k attention projection took %s, want <= 5s", elapsed)
	}
}
