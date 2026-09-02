package ai

import (
	"testing"
	"time"
)

// flapFinding builds a fresh warning finding for the flap tests.
func flapFinding() *Finding {
	return &Finding{
		ID:           "docker:h/web:exited",
		Key:          "container-exited",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		ResourceID:   "docker:h/web",
		ResourceName: "web",
		Title:        `Container "web" exited`,
	}
}

func lifecycleTypes(f *Finding) []string {
	types := make([]string, 0, len(f.Lifecycle))
	for _, event := range f.Lifecycle {
		types = append(types, event.Type)
	}
	return types
}

func TestFindingsStore_CollapsesFlappingIntoOneLifecycleRow(t *testing.T) {
	store := NewFindingsStore()
	store.SetStormThrottler(newFindingStormThrottler())

	store.Add(flapFinding())
	// Resolve and re-detect repeatedly. Each resolve and each regression is
	// one open/resolved transition.
	for i := 0; i < 6; i++ {
		if !store.ResolveWithReason("docker:h/web:exited", "container running again") {
			t.Fatalf("resolve %d failed", i)
		}
		if store.Add(flapFinding()) {
			t.Fatalf("re-detection %d was treated as a new finding", i)
		}
	}

	f := store.Get("docker:h/web:exited")
	if f == nil {
		t.Fatal("finding disappeared")
	}
	if f.Flapping == nil {
		t.Fatalf("twelve transitions in a minute were not labelled flapping: %v", lifecycleTypes(f))
	}
	if f.Flapping.TransitionCount != 12 {
		t.Fatalf("TransitionCount = %d, want 12", f.Flapping.TransitionCount)
	}
	if f.RegressionCount != 6 {
		t.Fatalf("RegressionCount = %d, want 6 (regressions still counted while collapsed)", f.RegressionCount)
	}

	// Transitions 1-3 were recorded individually; from the fourth onward the
	// store maintains a single flapping row instead of appending nine more.
	types := lifecycleTypes(f)
	flappingRows := 0
	individualTransitions := 0
	for _, typ := range types {
		switch typ {
		case FindingLifecycleFlapping:
			flappingRows++
		case "regressed", "auto_resolved", "resolved":
			individualTransitions++
		}
	}
	if flappingRows != 1 {
		t.Fatalf("flapping rows = %d, want exactly one collapsed row: %v", flappingRows, types)
	}
	if individualTransitions != findingFlapThreshold-1 {
		t.Fatalf("individual transition rows = %d, want %d pre-label rows: %v", individualTransitions, findingFlapThreshold-1, types)
	}
	last := f.Lifecycle[len(f.Lifecycle)-1]
	if last.Type != FindingLifecycleFlapping {
		t.Fatalf("last lifecycle row = %q, want the collapsed flapping row", last.Type)
	}
	if last.Metadata["transition_count"] != "12" {
		t.Fatalf("collapsed row transition_count = %q, want 12", last.Metadata["transition_count"])
	}
	if last.Metadata["last_transition"] != "regressed" {
		t.Fatalf("collapsed row last_transition = %q, want regressed", last.Metadata["last_transition"])
	}
}

func TestFindingsStore_WithoutThrottlerKeepsIndividualRows(t *testing.T) {
	store := NewFindingsStore()
	store.Add(flapFinding())
	for i := 0; i < 3; i++ {
		store.ResolveWithReason("docker:h/web:exited", "running again")
		store.Add(flapFinding())
	}
	f := store.Get("docker:h/web:exited")
	if f.Flapping != nil {
		t.Fatal("no throttler installed but the finding was labelled flapping")
	}
	for _, typ := range lifecycleTypes(f) {
		if typ == FindingLifecycleFlapping {
			t.Fatalf("collapsed row appeared without a throttler: %v", lifecycleTypes(f))
		}
	}
}

func TestFindingsStore_FlappingLabelClearsWhenChurnStops(t *testing.T) {
	store := NewFindingsStore()
	throttler := newFindingStormThrottler()
	store.SetStormThrottler(throttler)
	store.Add(flapFinding())
	for i := 0; i < 3; i++ {
		store.ResolveWithReason("docker:h/web:exited", "running again")
		store.Add(flapFinding())
	}
	if store.Get("docker:h/web:exited").Flapping == nil {
		t.Fatal("expected flapping after six transitions")
	}

	// Age the tracked window out, then one more transition: no longer flapping.
	throttler.mu.Lock()
	tracker := throttler.flaps["docker:h/web:exited"]
	for i := range tracker.transitions {
		tracker.transitions[i] = tracker.transitions[i].Add(-findingFlapWindow - time.Hour)
	}
	throttler.mu.Unlock()
	store.ResolveWithReason("docker:h/web:exited", "running again")

	f := store.Get("docker:h/web:exited")
	if f.Flapping != nil {
		t.Fatalf("label persisted after the window emptied: %+v", f.Flapping)
	}
	if last := f.Lifecycle[len(f.Lifecycle)-1]; last.Type != "auto_resolved" {
		t.Fatalf("post-flap transition row = %q, want an ordinary auto_resolved row", last.Type)
	}
}

func TestFindingJSONRoundTripsFlappingAndMirror(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	f := flapFinding()
	f.DetectedAt = now
	f.LastSeenAt = now
	f.Flapping = &FindingFlapping{TransitionCount: 7, WindowHours: 24, FirstTransitionAt: now.Add(-20 * time.Hour), LastTransitionAt: now}
	f.MirrorsAlertID = "docker-container-state-docker:h/web"
	f.MirrorsAlertType = "docker-container-state"

	data, err := f.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Finding
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if decoded.Flapping == nil || decoded.Flapping.TransitionCount != 7 || decoded.Flapping.WindowHours != 24 {
		t.Fatalf("flapping did not round-trip: %+v", decoded.Flapping)
	}
	if decoded.MirrorsAlertID != f.MirrorsAlertID || decoded.MirrorsAlertType != f.MirrorsAlertType {
		t.Fatalf("mirror fields did not round-trip: %q %q", decoded.MirrorsAlertID, decoded.MirrorsAlertType)
	}
}
