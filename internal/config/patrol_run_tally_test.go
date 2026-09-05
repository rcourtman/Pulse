package config

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
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

// A failed read of the existing history must not block the save — the tally is
// telemetry — but the tally reset it causes has to be visible in the log, not
// silent. (A genuinely missing file loads as empty data with no error, so it
// never reaches the warning path.)
func TestPatrolRunTallyReadErrorWarnsAndKeepsSaving(t *testing.T) {
	p := NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()

	// Seed history whose tally holds a run that has already fallen off the
	// capped run list: only a preserved tally would still count it.
	runA := PatrolRunRecord{ID: "a", NewFindings: 2, StartedAt: now.Add(-2 * time.Hour), CompletedAt: now.Add(-2 * time.Hour)}
	if err := p.SavePatrolRunHistory([]PatrolRunRecord{runA}); err != nil {
		t.Fatalf("SavePatrolRunHistory seed: %v", err)
	}

	logs := captureConfigLogs(t)
	mfs := &mockFSError{FileSystem: defaultFileSystem{}, readError: errors.New("transient read failure")}
	p.SetFileSystem(mfs)

	runB := PatrolRunRecord{ID: "b", NewFindings: 3, StartedAt: now.Add(-1 * time.Hour), CompletedAt: now.Add(-1 * time.Hour)}
	if err := p.SavePatrolRunHistory([]PatrolRunRecord{runB}); err != nil {
		t.Fatalf("SavePatrolRunHistory with failing read must still save: %v", err)
	}
	if !strings.Contains(logs.String(), "daily run and finding tallies restart") {
		t.Fatalf("no tally-restart warning was logged; logs: %s", logs.String())
	}

	mfs.readError = nil
	loaded, err := p.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("LoadPatrolRunHistory: %v", err)
	}
	if len(loaded.Runs) != 1 || loaded.Runs[0].ID != "b" {
		t.Fatalf("saved runs = %+v, want just run b", loaded.Runs)
	}
	// The tally restarted from the saved window: run a is gone, run b counted.
	if got := loaded.PatrolRunsSince(now.AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("PatrolRunsSince = %d, want 1 after the tally restart", got)
	}
	if got := loaded.PatrolNewFindingsSince(now.AddDate(0, 0, -30)); got != 3 {
		t.Fatalf("PatrolNewFindingsSince = %d, want 3 after the tally restart", got)
	}
}

// Upgrading a persisted run-only tally must backfill retained findings even
// when every retained run is already behind the run tally's cursor. Once
// saved, that finding volume must survive restart, trimming and repeated saves.
func TestPatrolNewFindingsTallyUpgradeAndRestart(t *testing.T) {
	dir := t.TempDir()
	p := NewConfigPersistence(dir)
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -30)
	at := now.Add(-time.Hour)
	legacy := PatrolRunHistoryData{
		Version: 1,
		Runs: []PatrolRunRecord{
			{ID: "b", CompletedAt: at, NewFindings: 3},
			{ID: "a", StartedAt: at.Add(-time.Hour), NewFindings: 2},
		},
		DailyRuns:       map[string]int{PatrolRunTallyDayKey(at): 150},
		RunTallyThrough: at,
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.aiPatrolRunsFile, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.LoadPatrolRunHistory()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.PatrolNewFindingsSince(since); got != 5 {
		t.Fatalf("legacy history fallback = %d, want 5", got)
	}
	if err := p.SavePatrolRunHistory(loaded.Runs); err != nil {
		t.Fatal(err)
	}

	// A new persistence instance must carry both tallies from disk after the
	// original source records are dropped. Zero-finding runs advance too.
	p = NewConfigPersistence(dir)
	next := []PatrolRunRecord{
		{ID: "d", CompletedAt: now, NewFindings: 0},
		{ID: "c", CompletedAt: now.Add(-time.Minute), NewFindings: 4},
	}
	for i := 0; i < 3; i++ {
		if err := p.SavePatrolRunHistory(next); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err = p.LoadPatrolRunHistory()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.PatrolRunsSince(since); got != 152 {
		t.Fatalf("run tally after upgrade = %d, want 152", got)
	}
	if got := loaded.PatrolNewFindingsSince(since); got != 9 {
		t.Fatalf("finding tally after restart and repeated saves = %d, want 9", got)
	}
	if !loaded.NewFindingsTallyThrough.Equal(now) {
		t.Fatalf("finding cursor = %s, want %s", loaded.NewFindingsTallyThrough, now)
	}
}

func TestPatrolNewFindingsTallyWindowAndRetention(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -30)
	runs := []PatrolRunRecord{
		{CompletedAt: since.Add(-time.Hour), NewFindings: 2},
		{StartedAt: since, NewFindings: 3},
		{CompletedAt: now.AddDate(0, 0, -32), NewFindings: 100},
		{CompletedAt: now, NewFindings: -1},
		{NewFindings: 100},
	}
	data := &PatrolRunHistoryData{Runs: runs}
	if got := data.PatrolNewFindingsSince(since); got != 3 {
		t.Fatalf("history fallback = %d, want 3", got)
	}
	advancePatrolRunTally(data, runs, now)
	data.Runs = nil
	if got := data.PatrolNewFindingsSince(since); got != 5 {
		t.Fatalf("UTC-day tally = %d, want 5 including cutoff day", got)
	}
	if len(data.DailyNewFindings) != 1 {
		t.Fatalf("tally contains expired or invalid findings: %v", data.DailyNewFindings)
	}
	var absent *PatrolRunHistoryData
	if got := absent.PatrolNewFindingsSince(since); got != 0 {
		t.Fatalf("nil history = %d, want 0", got)
	}
}
