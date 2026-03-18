package memory

import (
	"testing"
	"time"
)

func TestChangeDetector_DetectNew(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// First detection - should see new resource
	snapshots := []ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node1"},
	}

	changes := d.DetectChanges(snapshots)
	if len(changes) != 1 {
		t.Errorf("Expected 1 change (creation), got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeCreated {
		t.Errorf("Expected ChangeCreated, got %s", changes[0].ChangeType)
	}
}

func TestChangeDetector_DetectStatusChange(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// Initial state
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node1"},
	})

	// Status change
	changes := d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "stopped", Node: "node1"},
	})

	if len(changes) != 1 {
		t.Errorf("Expected 1 status change, got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeStatus {
		t.Errorf("Expected ChangeStatus, got %s", changes[0].ChangeType)
	}
	if changes[0].Before != "running" || changes[0].After != "stopped" {
		t.Errorf("Expected running->stopped, got %v->%v", changes[0].Before, changes[0].After)
	}
}

func TestChangeDetector_DetectMigration(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// Initial state
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node1"},
	})

	// Migration
	changes := d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node2"},
	})

	if len(changes) != 1 {
		t.Errorf("Expected 1 migration change, got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeMigrated {
		t.Errorf("Expected ChangeMigrated, got %s", changes[0].ChangeType)
	}
}

func TestChangeDetector_DetectDeleted(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// Initial state
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node1"},
	})

	// Delete (empty snapshot)
	changes := d.DetectChanges([]ResourceSnapshot{})

	if len(changes) != 1 {
		t.Errorf("Expected 1 deletion change, got %d", len(changes))
	}
	if changes[0].ChangeType != ChangeDeleted {
		t.Errorf("Expected ChangeDeleted, got %s", changes[0].ChangeType)
	}
}

func TestChangeDetector_NoChanges(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	snapshot := []ResourceSnapshot{
		{ID: "vm-100", Name: "web-server", Type: "vm", Status: "running", Node: "node1"},
	}

	// First time - creates
	d.DetectChanges(snapshot)

	// Second time - no changes
	changes := d.DetectChanges(snapshot)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(changes))
	}
}

func TestChangeDetector_GetChangesForResource(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// Create and change a few times
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web", Type: "vm", Status: "stopped", Node: "node1"},
		{ID: "vm-200", Name: "db", Type: "vm", Status: "running", Node: "node1"},
	})
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web", Type: "vm", Status: "running", Node: "node1"},
		{ID: "vm-200", Name: "db", Type: "vm", Status: "running", Node: "node1"},
	})

	// Get changes for vm-100 only
	changes := d.GetChangesForResource("vm-100", 10)
	for _, c := range changes {
		if c.ResourceID != "vm-100" {
			t.Errorf("Got change for wrong resource: %s", c.ResourceID)
		}
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

func TestRemediationLog_GetSimilar(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	// Log some remediations
	_ = r.Log(RemediationRecord{
		ResourceID: "vm-100",
		Problem:    "High memory usage causing OOM",
		Action:     "Restart service",
		Outcome:    OutcomeResolved,
	})
	_ = r.Log(RemediationRecord{
		ResourceID: "vm-200",
		Problem:    "Memory leak detected",
		Action:     "Cleared cache",
		Outcome:    OutcomePartial,
	})
	_ = r.Log(RemediationRecord{
		ResourceID: "vm-300",
		Problem:    "CPU spike from backup",
		Action:     "Rescheduled backup",
		Outcome:    OutcomeResolved,
	})

	// Search for similar memory issues
	similar := r.GetSimilar("High memory usage causing slowdown", 5)
	if len(similar) < 1 {
		t.Errorf("Expected at least 1 similar record")
	}
}

func TestRemediationLog_GetSuccessfulRemediations(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		Problem: "Memory usage high",
		Action:  "Restart service",
		Outcome: OutcomeResolved,
	})
	_ = r.Log(RemediationRecord{
		Problem: "Memory usage high",
		Action:  "Kill process",
		Outcome: OutcomeFailed,
	})

	successful := r.GetSuccessfulRemediations("Memory usage issue", 5)
	for _, rec := range successful {
		if rec.Outcome != OutcomeResolved && rec.Outcome != OutcomePartial {
			t.Errorf("Got unsuccessful remediation in successful list: %s", rec.Outcome)
		}
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
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// Create some changes
	d.DetectChanges([]ResourceSnapshot{
		{ID: "vm-100", Name: "web", Type: "vm", Status: "running", Node: "node1"},
	})

	// Get recent changes
	since := time.Now().Add(-1 * time.Hour)
	changes := d.GetRecentChanges(10, since)
	if len(changes) == 0 {
		t.Error("Expected at least 1 recent change")
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

func TestChangeDetector_Limit(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 5})

	// Create many changes to exceed limit
	for i := 0; i < 10; i++ {
		d.DetectChanges([]ResourceSnapshot{
			{ID: "vm-100", Name: "web", Type: "vm", Status: "running", Node: "node1"},
		})
		// Alternate status to create changes
		d.DetectChanges([]ResourceSnapshot{
			{ID: "vm-100", Name: "web", Type: "vm", Status: "stopped", Node: "node1"},
		})
	}

	// Should have limited records
	allChanges := d.GetRecentChanges(100, time.Time{})
	if len(allChanges) > 5 {
		t.Errorf("Expected max 5 changes due to limit, got %d", len(allChanges))
	}
}
