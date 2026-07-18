package ai

import "testing"

// The finding notification callback must fire exactly once per finding
// lifetime, on the genuinely-new path, and only for warning+ severities.
func TestRecordFinding_FindingNotifyCallback(t *testing.T) {
	newFinding := func(id string, severity FindingSeverity) *Finding {
		return &Finding{
			ID:           id,
			Severity:     severity,
			Category:     FindingCategoryReliability,
			ResourceID:   "vm-101",
			ResourceName: "web",
			ResourceType: "vm",
			Title:        "Test finding " + id,
		}
	}

	t.Run("fires once for a new warning finding", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		var notified []*Finding
		ps.SetFindingNotifyCallback(func(f *Finding) {
			notified = append(notified, f)
		})

		if !ps.recordFinding(newFinding("warn-1", FindingSeverityWarning)) {
			t.Fatal("expected finding to record as new")
		}
		if len(notified) != 1 {
			t.Fatalf("callback fired %d times, want 1", len(notified))
		}
		if notified[0].ID != "warn-1" {
			t.Fatalf("notified finding ID = %q", notified[0].ID)
		}

		// A re-detection of the same finding must not notify again.
		ps.recordFinding(newFinding("warn-1", FindingSeverityWarning))
		if len(notified) != 1 {
			t.Fatalf("re-detection fired the callback; %d calls, want 1", len(notified))
		}
	})

	t.Run("fires for critical findings", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		calls := 0
		ps.SetFindingNotifyCallback(func(*Finding) { calls++ })
		ps.recordFinding(newFinding("crit-1", FindingSeverityCritical))
		if calls != 1 {
			t.Fatalf("callback fired %d times, want 1", calls)
		}
	})

	t.Run("does not fire for info findings", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		calls := 0
		ps.SetFindingNotifyCallback(func(*Finding) { calls++ })
		ps.recordFinding(newFinding("info-1", FindingSeverityInfo))
		if calls != 0 {
			t.Fatalf("callback fired %d times for an info finding, want 0", calls)
		}
	})

	t.Run("nil callback is safe", func(t *testing.T) {
		ps := NewPatrolService(nil, nil)
		if !ps.recordFinding(newFinding("warn-2", FindingSeverityWarning)) {
			t.Fatal("expected finding to record as new")
		}
	})
}
