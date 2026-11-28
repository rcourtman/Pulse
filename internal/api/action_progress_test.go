package api

import (
	"fmt"
	"testing"
	"time"
)

func TestActionProgressStoreIsolationDedupeAndBounds(t *testing.T) {
	now := time.Date(2026, 11, 28, 14, 22, 0, 0, time.UTC)
	store := newActionProgressStore()
	store.now = func() time.Time { return now }

	store.Record("org-a", "act-1", "server", "dispatch", "Dispatched\tto agent\n", now)
	store.Record("org-a", "act-1", "server", "dispatch", "Dispatched to agent", now.Add(time.Second))
	store.Record("org-a", "act-1", "agent", "pull", "Pulling image qmcgaw/gluetun:latest...", now.Add(2*time.Second))
	store.Record("org-b", "act-1", "agent", "pull", "Other tenant message", now.Add(2*time.Second))

	orgA := store.List("org-a", "act-1")
	if len(orgA) != 2 {
		t.Fatalf("expected 2 deduplicated entries for org-a, got %d: %+v", len(orgA), orgA)
	}
	if orgA[0].Message != "Dispatched to agent" || orgA[1].Message != "Pulling image qmcgaw/gluetun:latest..." {
		t.Fatalf("unexpected org-a entries: %+v", orgA)
	}

	orgB := store.List("org-b", "act-1")
	if len(orgB) != 1 || orgB[0].Message != "Other tenant message" {
		t.Fatalf("tenant isolation failed for org-b: %+v", orgB)
	}

	// Bound per-action entries while preserving the opening step and latest tail.
	for i := 0; i < maxActionProgressEntriesPerAction+10; i++ {
		store.Record("org-a", "act-bound", "agent", "", fmt.Sprintf("step %d", i), now.Add(time.Duration(i)*time.Second))
	}
	bounded := store.List("org-a", "act-bound")
	if len(bounded) != maxActionProgressEntriesPerAction {
		t.Fatalf("expected %d entries, got %d", maxActionProgressEntriesPerAction, len(bounded))
	}
	if bounded[0].Message != "step 0" {
		t.Fatalf("expected first step preserved, got %q", bounded[0].Message)
	}
	if bounded[len(bounded)-1].Message != fmt.Sprintf("step %d", maxActionProgressEntriesPerAction+9) {
		t.Fatalf("expected newest step preserved, got %q", bounded[len(bounded)-1].Message)
	}

	// Entries expire after retention TTL.
	now = now.Add(actionProgressRetentionTTL + time.Minute)
	if got := store.List("org-a", "act-1"); got != nil {
		t.Fatalf("expected expired progress entries to be nil, got %+v", got)
	}
}
