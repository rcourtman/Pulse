package monitoring

import (
	"testing"
	"time"
)

const testIdentityFlapWindow = 15 * time.Minute

func TestIdentityFlapTrackerDetectsAlternatingHostnames(t *testing.T) {
	tracker := newIdentityFlapTracker(testIdentityFlapWindow)
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	interval := 30 * time.Second

	if conflict := tracker.observe("clone-a", "machine-1", base); conflict != nil {
		t.Fatalf("first report should not conflict, got %+v", conflict)
	}
	// First switch could be a legitimate rename, so it must not warn yet.
	if conflict := tracker.observe("clone-b", "machine-1", base.Add(interval)); conflict != nil {
		t.Fatalf("single hostname switch should not conflict, got %+v", conflict)
	}
	// The revisit of clone-a proves two machines are alternating.
	conflict := tracker.observe("clone-a", "machine-1", base.Add(2*interval))
	if conflict == nil {
		t.Fatal("expected conflict after hostname revisit")
	}
	if len(conflict.hostnames) != 2 || conflict.hostnames[0] != "clone-a" || conflict.hostnames[1] != "clone-b" {
		t.Fatalf("expected sorted hostnames [clone-a clone-b], got %v", conflict.hostnames)
	}
	if len(conflict.secondaries) != 0 {
		t.Fatalf("shared secondary value should not be listed as conflicting, got %v", conflict.secondaries)
	}
	if conflict.firstSeen.IsZero() || conflict.lastSeen.IsZero() {
		t.Fatalf("expected conflict timestamps, got %+v", conflict)
	}

	// Continued flapping keeps the conflict alive and refreshes lastSeen.
	later := tracker.observe("clone-b", "machine-1", base.Add(3*interval))
	if later == nil {
		t.Fatal("expected conflict to persist while flapping continues")
	}
	if !later.lastSeen.After(conflict.lastSeen) {
		t.Fatalf("expected lastSeen to advance, got %v then %v", conflict.lastSeen, later.lastSeen)
	}
	if !later.firstSeen.Equal(conflict.firstSeen) {
		t.Fatalf("expected firstSeen to be stable, got %v then %v", conflict.firstSeen, later.firstSeen)
	}
}

func TestIdentityFlapTrackerDetectsAlternatingSecondariesBehindOneHostname(t *testing.T) {
	// MSP template deployments reuse hostnames across sites (pve01 at two
	// customers), so the alternating secondary value (report IP) is the only
	// field that betrays the clone.
	tracker := newIdentityFlapTracker(testIdentityFlapWindow)
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	tracker.observe("pve01", "192.168.1.10", base)
	if conflict := tracker.observe("pve01", "10.0.0.10", base.Add(30*time.Second)); conflict != nil {
		t.Fatalf("single secondary switch should not conflict, got %+v", conflict)
	}
	conflict := tracker.observe("pve01", "192.168.1.10", base.Add(60*time.Second))
	if conflict == nil {
		t.Fatal("expected conflict after secondary revisit")
	}
	if len(conflict.hostnames) != 1 || conflict.hostnames[0] != "pve01" {
		t.Fatalf("expected shared hostname [pve01], got %v", conflict.hostnames)
	}
	if len(conflict.secondaries) != 2 || conflict.secondaries[0] != "10.0.0.10" || conflict.secondaries[1] != "192.168.1.10" {
		t.Fatalf("expected sorted secondaries [10.0.0.10 192.168.1.10], got %v", conflict.secondaries)
	}
}

func TestIdentityFlapTrackerIgnoresSingleRename(t *testing.T) {
	tracker := newIdentityFlapTracker(testIdentityFlapWindow)
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	if conflict := tracker.observe("old-name", "machine-1", base); conflict != nil {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
	for i := 1; i <= 5; i++ {
		if conflict := tracker.observe("new-name", "machine-1", base.Add(time.Duration(i)*30*time.Second)); conflict != nil {
			t.Fatalf("rename should never conflict, got %+v on report %d", conflict, i)
		}
	}
}

func TestIdentityFlapTrackerHandlesAbsentSecondary(t *testing.T) {
	tracker := newIdentityFlapTracker(testIdentityFlapWindow)
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	tracker.observe("clone-a", "", base)
	tracker.observe("clone-b", "", base.Add(30*time.Second))
	conflict := tracker.observe("clone-a", "", base.Add(60*time.Second))
	if conflict == nil {
		t.Fatal("expected conflict after hostname revisit")
	}
	if len(conflict.secondaries) != 0 {
		t.Fatalf("absent secondaries should not be listed as conflicting, got %v", conflict.secondaries)
	}
}

func TestIdentityFlapTrackerConflictExpiresAfterWindow(t *testing.T) {
	tracker := newIdentityFlapTracker(testIdentityFlapWindow)
	base := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	tracker.observe("clone-a", "machine-1", base)
	tracker.observe("clone-b", "machine-1", base.Add(30*time.Second))
	if conflict := tracker.observe("clone-a", "machine-1", base.Add(60*time.Second)); conflict == nil {
		t.Fatal("expected conflict after revisit")
	}

	// One clone goes away; steady reports from the survivor eventually clear it.
	steady := base.Add(60 * time.Second).Add(testIdentityFlapWindow + time.Minute)
	if conflict := tracker.observe("clone-a", "machine-1", steady); conflict != nil {
		t.Fatalf("expected conflict to expire after quiet window, got %+v", conflict)
	}
}
