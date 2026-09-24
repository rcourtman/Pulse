package memory

import (
	"testing"
	"time"
)

func TestChangeDetector_GetChangesForResource(t *testing.T) {
	now := time.Now()
	d := &ChangeDetector{maxChanges: 100, changes: []Change{
		{ID: "c1", ResourceID: "vm-100", ChangeType: ChangeCreated, DetectedAt: now.Add(-3 * time.Minute)},
		{ID: "c2", ResourceID: "vm-200", ChangeType: ChangeCreated, DetectedAt: now.Add(-2 * time.Minute)},
		{ID: "c3", ResourceID: "vm-100", ChangeType: ChangeStatus, DetectedAt: now.Add(-time.Minute)},
	}}

	// Get changes for vm-100 only, most recent first
	changes := d.GetChangesForResource("vm-100", 10)
	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes for vm-100, got %d", len(changes))
	}
	if changes[0].ID != "c3" || changes[1].ID != "c1" {
		t.Errorf("Expected vm-100 changes newest first, got %s, %s", changes[0].ID, changes[1].ID)
	}
	if limited := d.GetChangesForResource("vm-100", 1); len(limited) != 1 || limited[0].ID != "c3" {
		t.Errorf("Expected the limit to keep only the newest change, got %v", limited)
	}
}

func TestRemediationLog_LogAndRetrieve(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		ResourceID: "vm-100",
		Problem:    "High memory usage",
		Action:     "systemctl restart nginx",
		Outcome:    OutcomeResolved,
	})

	records := r.GetForResource("vm-100", 10)
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
	if records[0].Action != "systemctl restart nginx" {
		t.Errorf("Wrong action: %s", records[0].Action)
	}
}

func TestRemediationLog_Stats(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{Problem: "p1", Action: "a1", Outcome: OutcomeResolved})
	_ = r.Log(RemediationRecord{Problem: "p2", Action: "a2", Outcome: OutcomeResolved})
	_ = r.Log(RemediationRecord{Problem: "p3", Action: "a3", Outcome: OutcomeFailed})

	stats := r.GetRemediationStats()
	if stats["total"] != 3 {
		t.Errorf("Expected 3 total, got %d", stats["total"])
	}
	if stats["resolved"] != 2 {
		t.Errorf("Expected 2 resolved, got %d", stats["resolved"])
	}
	if stats["failed"] != 1 {
		t.Errorf("Expected 1 failed, got %d", stats["failed"])
	}
}

func TestChangeDetector_GetRecentChanges(t *testing.T) {
	now := time.Now()
	d := &ChangeDetector{maxChanges: 100, changes: []Change{
		{ID: "old", ResourceID: "vm-100", DetectedAt: now.Add(-2 * time.Hour)},
		{ID: "new", ResourceID: "vm-100", DetectedAt: now.Add(-10 * time.Minute)},
	}}

	// Only changes inside the window are recent
	changes := d.GetRecentChanges(10, now.Add(-1*time.Hour))
	if len(changes) != 1 || changes[0].ID != "new" {
		t.Errorf("Expected only the change inside the window, got %v", changes)
	}
}

func TestRemediationLog_GetRecentRemediationStats(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	// Log some remediations with different outcomes
	now := time.Now()

	_ = r.Log(RemediationRecord{
		Timestamp: now.Add(-1 * time.Hour),
		Problem:   "p1",
		Action:    "a1",
		Outcome:   OutcomeResolved,
		Automatic: true,
	})
	_ = r.Log(RemediationRecord{
		Timestamp: now.Add(-2 * time.Hour),
		Problem:   "p2",
		Action:    "a2",
		Outcome:   OutcomePartial,
		Automatic: false,
	})
	_ = r.Log(RemediationRecord{
		Timestamp: now.Add(-30 * time.Minute),
		Problem:   "p3",
		Action:    "a3",
		Outcome:   OutcomeFailed,
		Automatic: true,
	})
	_ = r.Log(RemediationRecord{
		Timestamp: now.Add(-48 * time.Hour),
		Problem:   "old",
		Action:    "old",
		Outcome:   OutcomeResolved,
		Automatic: false,
	})

	// Get stats for last 24 hours
	since := now.Add(-24 * time.Hour)
	stats := r.GetRecentRemediationStats(since)

	if stats["total"] != 3 {
		t.Errorf("Expected 3 total (last 24h), got %d", stats["total"])
	}
	if stats["resolved"] != 1 {
		t.Errorf("Expected 1 resolved, got %d", stats["resolved"])
	}
	if stats["partial"] != 1 {
		t.Errorf("Expected 1 partial, got %d", stats["partial"])
	}
	if stats["failed"] != 1 {
		t.Errorf("Expected 1 failed, got %d", stats["failed"])
	}
	if stats["automatic"] != 2 {
		t.Errorf("Expected 2 automatic, got %d", stats["automatic"])
	}
	if stats["manual"] != 1 {
		t.Errorf("Expected 1 manual, got %d", stats["manual"])
	}
}

func TestRemediationLog_AutomaticVsManual(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		Problem:   "auto problem",
		Action:    "auto action",
		Outcome:   OutcomeResolved,
		Automatic: true,
	})
	_ = r.Log(RemediationRecord{
		Problem:   "manual problem",
		Action:    "manual action",
		Outcome:   OutcomeResolved,
		Automatic: false,
	})

	stats := r.GetRemediationStats()
	// Verify both are counted
	if stats["total"] != 2 {
		t.Errorf("Expected 2 total, got %d", stats["total"])
	}
}
