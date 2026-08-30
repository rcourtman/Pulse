package config_test

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestWorkloadHistoryActivityTallyCountsOnlyClosedMilestones(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	for _, activity := range []string{
		config.WorkloadHistoryActivityPreview,
		config.WorkloadHistoryActivityScrub,
		config.WorkloadHistoryActivityRangeChange,
		config.WorkloadHistoryActivityDetailsSelected,
	} {
		if err := persistence.RecordWorkloadHistoryActivity(activity, now); err != nil {
			t.Fatalf("RecordWorkloadHistoryActivity(%q) = %v", activity, err)
		}
	}

	tally, err := persistence.LoadWorkloadHistoryActivityTally()
	if err != nil {
		t.Fatalf("LoadWorkloadHistoryActivityTally = %v", err)
	}
	since := now.AddDate(0, 0, -30)
	if got := tally.PreviewsSince(since); got != 1 {
		t.Fatalf("PreviewsSince = %d, want 1", got)
	}
	if got := tally.ScrubsSince(since); got != 1 {
		t.Fatalf("ScrubsSince = %d, want 1", got)
	}
	if got := tally.RangeChangesSince(since); got != 1 {
		t.Fatalf("RangeChangesSince = %d, want 1", got)
	}
	if got := tally.DetailsSelectedSince(since); got != 1 {
		t.Fatalf("DetailsSelectedSince = %d, want 1", got)
	}
}

func TestWorkloadHistoryActivityTallyRejectsUnknownActivity(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.RecordWorkloadHistoryActivity("guest-101", time.Now().UTC()); err == nil {
		t.Fatal("unknown activity was accepted")
	}
	tally, err := persistence.LoadWorkloadHistoryActivityTally()
	if err != nil {
		t.Fatalf("LoadWorkloadHistoryActivityTally = %v", err)
	}
	if got := tally.PreviewsSince(time.Time{}); got != 0 {
		t.Fatalf("unknown activity changed tally: previews = %d", got)
	}
}

func TestWorkloadHistoryActivityTallyPrunesOutsideRetention(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	if err := persistence.RecordWorkloadHistoryActivity(config.WorkloadHistoryActivityPreview, now.AddDate(0, 0, -40)); err != nil {
		t.Fatalf("record old activity = %v", err)
	}
	if err := persistence.RecordWorkloadHistoryActivity(config.WorkloadHistoryActivityPreview, now); err != nil {
		t.Fatalf("record current activity = %v", err)
	}
	tally, err := persistence.LoadWorkloadHistoryActivityTally()
	if err != nil {
		t.Fatalf("LoadWorkloadHistoryActivityTally = %v", err)
	}
	if got := tally.PreviewsSince(now.AddDate(0, 0, -30)); got != 1 {
		t.Fatalf("PreviewsSince = %d, want 1", got)
	}
}
