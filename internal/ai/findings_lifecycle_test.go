package ai

import "testing"

func TestFindingsStore_AddRecordsDetectedLifecycleEvent(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-1",
		ResourceID:   "node-1",
		ResourceName: "node-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		Title:        "High CPU usage trend",
		Description:  "CPU trend indicates sustained pressure",
	}

	if !store.Add(f) {
		t.Fatal("expected first add to create a finding")
	}

	got := store.Get("lf-1")
	if got == nil {
		t.Fatal("expected finding to exist")
	}
	if got.TimesRaised != 1 {
		t.Fatalf("expected timesRaised=1, got %d", got.TimesRaised)
	}
	if len(got.Lifecycle) == 0 {
		t.Fatal("expected lifecycle events to be recorded")
	}
	last := got.Lifecycle[len(got.Lifecycle)-1]
	if last.Type != "detected" {
		t.Fatalf("expected last lifecycle event type=detected, got %q", last.Type)
	}
}

func TestFindingsStore_RegressionIncrementsAndRecordsLifecycleEvent(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "lf-2",
		ResourceID:   "vm-101",
		ResourceName: "web",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts",
	}
	if !store.Add(f) {
		t.Fatal("expected first add to create finding")
	}
	if !store.Resolve("lf-2", false) {
		t.Fatal("expected resolve to succeed")
	}
	if store.Get("lf-2").RegressionCount != 0 {
		t.Fatal("expected no regressions before re-detection")
	}
	if store.Add(&Finding{
		ID:           "lf-2",
		ResourceID:   "vm-101",
		ResourceName: "web",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		Title:        "Restart loop",
		Description:  "Service repeatedly restarts again",
	}) {
		t.Fatal("expected second add to update existing finding")
	}

	got := store.Get("lf-2")
	if got == nil {
		t.Fatal("expected finding to exist")
	}
	if got.RegressionCount != 1 {
		t.Fatalf("expected regressionCount=1, got %d", got.RegressionCount)
	}
	if got.LastRegressionAt == nil {
		t.Fatal("expected lastRegressionAt to be set")
	}
	foundRegressed := false
	for _, e := range got.Lifecycle {
		if e.Type == "regressed" {
			foundRegressed = true
			break
		}
	}
	if !foundRegressed {
		t.Fatal("expected regressed lifecycle event")
	}
}

func TestFindingsStore_BlocksInvalidLoopStateTransition(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:                   "lf-3",
		ResourceID:           "ct-1",
		ResourceName:         "container-1",
		Severity:             FindingSeverityWarning,
		Category:             FindingCategoryPerformance,
		Title:                "CPU burst",
		Description:          "Unexpected sustained CPU burst",
		LoopState:            string(FindingLoopStateResolved),
		InvestigationOutcome: string(InvestigationOutcomeFixExecuted), // would derive to remediating
	}

	// Directly call lock-only helper to validate transition guard behavior.
	store.mu.Lock()
	store.syncLoopStateLocked(f)
	store.mu.Unlock()

	if f.LoopState != string(FindingLoopStateResolved) {
		t.Fatalf("expected loop state to remain resolved, got %q", f.LoopState)
	}
	if len(f.Lifecycle) == 0 {
		t.Fatal("expected lifecycle event for blocked transition")
	}
	last := f.Lifecycle[len(f.Lifecycle)-1]
	if last.Type != "loop_transition_violation" {
		t.Fatalf("expected loop_transition_violation, got %q", last.Type)
	}
}
