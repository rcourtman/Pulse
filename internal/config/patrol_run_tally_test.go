package config

import (
	"testing"
	"time"
)

// A patrol run history that has already saturated the operator-facing cap must
// still report the true run count for a thirty-day telemetry window.
func TestPatrolRunTallySurvivesHistoryCap(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -30)

	// Patrol every five minutes for three days, saving the newest-first
	// capped window after each run exactly as the run history store does.
	const cap = 100
	total := 0
	runs := make([]PatrolRunRecord, 0, cap)
	for at := now.AddDate(0, 0, -3); at.Before(now); at = at.Add(5 * time.Minute) {
		runs = append([]PatrolRunRecord{{ID: at.Format(time.RFC3339Nano), StartedAt: at, CompletedAt: at}}, runs...)
		if len(runs) > cap {
			runs = runs[:cap]
		}
		total++
		if err := p.SavePatrolRunHistory(runs); err != nil {
			t.Fatalf("SavePatrolRunHistory: %v", err)
		}
	}

	loaded, err := p.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory: %v", err)
	}
	if len(loaded.Runs) != cap {
		t.Fatalf("retained runs = %d, want the capped %d", len(loaded.Runs), cap)
	}
	if got := loaded.PatrolRunsSince(since); got != total {
		t.Fatalf("PatrolRunsSince = %d, want %d (the cap would have reported %d)", got, total, cap)
	}
}

// Repeated saves of an overlapping newest-first window must not double count.
func TestPatrolRunTallyDoesNotDoubleCount(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	runs := []PatrolRunRecord{
		{ID: "c", StartedAt: now.Add(-1 * time.Hour), CompletedAt: now.Add(-1 * time.Hour)},
		{ID: "b", StartedAt: now.Add(-2 * time.Hour), CompletedAt: now.Add(-2 * time.Hour)},
		{ID: "a", StartedAt: now.Add(-3 * time.Hour), CompletedAt: now.Add(-3 * time.Hour)},
	}
	for i := 0; i < 4; i++ {
		if err := p.SavePatrolRunHistory(runs); err != nil {
			t.Fatalf("SavePatrolRunHistory: %v", err)
		}
	}
	loaded, err := p.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory: %v", err)
	}
	if got := loaded.PatrolRunsSince(now.AddDate(0, 0, -30)); got != 3 {
		t.Fatalf("PatrolRunsSince = %d, want 3 after four saves of the same window", got)
	}
}

// An install upgrading into the tally has no tallied days yet, so the counter
// must fall back to the run list rather than reporting zero.
func TestPatrolRunsSinceFallsBackToHistory(t *testing.T) {
	now := time.Now().UTC()
	data := &PatrolRunHistoryData{Runs: []PatrolRunRecord{
		{ID: "a", StartedAt: now.Add(-2 * time.Hour), CompletedAt: now.Add(-2 * time.Hour)},
		{ID: "b", StartedAt: now.Add(-1 * time.Hour), CompletedAt: now.Add(-1 * time.Hour)},
		{ID: "old", StartedAt: now.AddDate(0, 0, -40), CompletedAt: now.AddDate(0, 0, -40)},
	}}
	if got := data.PatrolRunsSince(now.AddDate(0, 0, -30)); got != 2 {
		t.Fatalf("PatrolRunsSince = %d, want 2", got)
	}
}

// Days that fall out of the retention window must be pruned so the tally stays
// bounded no matter how long an install runs.
func TestPatrolRunTallyPrunesOldDays(t *testing.T) {
	now := time.Now().UTC()
	data := &PatrolRunHistoryData{DailyRuns: map[string]int{
		PatrolRunTallyDayKey(now.AddDate(0, 0, -400)): 12,
		PatrolRunTallyDayKey(now.AddDate(0, 0, -40)):  7,
		PatrolRunTallyDayKey(now.AddDate(0, 0, -1)):   3,
	}}
	advancePatrolRunTally(data, nil, now)
	if _, ok := data.DailyRuns[PatrolRunTallyDayKey(now.AddDate(0, 0, -400))]; ok {
		t.Fatal("400-day-old tally day was not pruned")
	}
	if _, ok := data.DailyRuns[PatrolRunTallyDayKey(now.AddDate(0, 0, -40))]; ok {
		t.Fatal("40-day-old tally day was not pruned")
	}
	if data.DailyRuns[PatrolRunTallyDayKey(now.AddDate(0, 0, -1))] != 3 {
		t.Fatal("yesterday's tally day was pruned")
	}
}
