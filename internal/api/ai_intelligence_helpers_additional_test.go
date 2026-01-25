package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
)

func TestRemediationStatsFromRecords(t *testing.T) {
	records := []ai.RemediationRecord{
		{Outcome: ai.OutcomeResolved, Automatic: true},
		{Outcome: ai.OutcomeResolved, Automatic: false},
		{Outcome: ai.OutcomePartial, Automatic: true},
		{Outcome: ai.OutcomeFailed, Automatic: false},
		{Outcome: "unknown", Automatic: true},
	}

	stats := remediationStatsFromRecords(records)
	if stats["total"] != 5 {
		t.Fatalf("total = %d, want 5", stats["total"])
	}
	if stats["resolved"] != 2 {
		t.Fatalf("resolved = %d, want 2", stats["resolved"])
	}
	if stats["partial"] != 1 {
		t.Fatalf("partial = %d, want 1", stats["partial"])
	}
	if stats["failed"] != 1 {
		t.Fatalf("failed = %d, want 1", stats["failed"])
	}
	if stats["unknown"] != 1 {
		t.Fatalf("unknown = %d, want 1", stats["unknown"])
	}
	if stats["automatic"] != 3 {
		t.Fatalf("automatic = %d, want 3", stats["automatic"])
	}
	if stats["manual"] != 2 {
		t.Fatalf("manual = %d, want 2", stats["manual"])
	}
}
