package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRecordNodeTestOutcomeCountsAttemptsAndFailures(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		if err := cp.RecordNodeTestOutcome(true, now); err != nil {
			t.Fatalf("RecordNodeTestOutcome(failed) = %v", err)
		}
	}
	if err := cp.RecordNodeTestOutcome(false, now); err != nil {
		t.Fatalf("RecordNodeTestOutcome(success) = %v", err)
	}

	tally, err := cp.LoadNodeTestTally()
	if err != nil {
		t.Fatalf("LoadNodeTestTally = %v", err)
	}
	since := now.AddDate(0, 0, -30)
	if got := tally.AttemptsSince(since); got != 4 {
		t.Fatalf("AttemptsSince = %d, want 4", got)
	}
	if got := tally.FailuresSince(since); got != 3 {
		t.Fatalf("FailuresSince = %d, want 3", got)
	}
}

// A success must never be counted as a failure, otherwise the failure share
// this counter exists to measure is meaningless.
func TestRecordNodeTestOutcomeSuccessLeavesFailuresZero(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()

	if err := cp.RecordNodeTestOutcome(false, now); err != nil {
		t.Fatalf("RecordNodeTestOutcome = %v", err)
	}

	tally, err := cp.LoadNodeTestTally()
	if err != nil {
		t.Fatalf("LoadNodeTestTally = %v", err)
	}
	if got := tally.FailuresSince(now.AddDate(0, 0, -30)); got != 0 {
		t.Fatalf("FailuresSince = %d, want 0", got)
	}
	if got := tally.AttemptsSince(now.AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("AttemptsSince = %d, want 1", got)
	}
}

// Counts outside the reporting window must not leak into it, and days outside
// the retention window must be pruned so the file stays bounded.
func TestNodeTestTallyPrunesAndWindowsCorrectly(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()

	if err := cp.RecordNodeTestOutcome(true, now.AddDate(0, 0, -40)); err != nil {
		t.Fatalf("RecordNodeTestOutcome(old) = %v", err)
	}
	if err := cp.RecordNodeTestOutcome(true, now.AddDate(0, 0, -10)); err != nil {
		t.Fatalf("RecordNodeTestOutcome(recent) = %v", err)
	}
	if err := cp.RecordNodeTestOutcome(true, now); err != nil {
		t.Fatalf("RecordNodeTestOutcome(now) = %v", err)
	}

	tally, err := cp.LoadNodeTestTally()
	if err != nil {
		t.Fatalf("LoadNodeTestTally = %v", err)
	}
	if got := tally.AttemptsSince(now.AddDate(0, 0, -30)); got != 2 {
		t.Fatalf("AttemptsSince(30d) = %d, want 2", got)
	}
	if _, ok := tally.DailyAttempts[config.NodeTestTallyDayKey(now.AddDate(0, 0, -40))]; ok {
		t.Fatalf("day outside retention window was not pruned")
	}
}

// The counters are advisory. A corrupt tally must start over rather than make
// a connection test fail.
func TestRecordNodeTestOutcomeRecoversFromCorruptTally(t *testing.T) {
	dir := t.TempDir()
	cp := config.NewConfigPersistence(dir)
	if err := os.WriteFile(filepath.Join(dir, "node_test_tally.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed corrupt tally: %v", err)
	}

	now := time.Now().UTC()
	if err := cp.RecordNodeTestOutcome(true, now); err != nil {
		t.Fatalf("RecordNodeTestOutcome = %v", err)
	}

	tally, err := cp.LoadNodeTestTally()
	if err != nil {
		t.Fatalf("LoadNodeTestTally = %v", err)
	}
	if got := tally.AttemptsSince(now.AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("AttemptsSince = %d, want 1", got)
	}
}
