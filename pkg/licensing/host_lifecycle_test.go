package licensing

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// testClock is a controllable clock for deterministic tests.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func newTestClock(t time.Time) *testClock {
	return &testClock{now: t}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestTracker(clock *testClock) *HostLifecycleTracker {
	t := NewHostLifecycleTracker()
	t.nowFunc = clock.Now
	return t
}

// ---------------------------------------------------------------------------
// H2: Stabilization
// ---------------------------------------------------------------------------

func TestNewHostIsNotStable(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")

	if tracker.IsStable("host-1") {
		t.Fatal("new host should not be stable immediately")
	}
	// Should not appear in stable-active list.
	if got := tracker.StableActiveHosts(); len(got) != 0 {
		t.Fatalf("expected 0 stable-active hosts, got %d", len(got))
	}
}

func TestHostBecomesStableAfter10Min(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")

	// 9 minutes: still not stable.
	clock.Advance(9 * time.Minute)
	tracker.RecordHeartbeat("host-1")
	if tracker.IsStable("host-1") {
		t.Fatal("host should not be stable before 10 minutes")
	}

	// Exactly 10 minutes: becomes stable on heartbeat.
	clock.Advance(1 * time.Minute)
	tracker.RecordHeartbeat("host-1")
	if !tracker.IsStable("host-1") {
		t.Fatal("host should be stable after 10 minutes of heartbeats")
	}

	hosts := tracker.StableActiveHosts()
	if len(hosts) != 1 || hosts[0] != "host-1" {
		t.Fatalf("expected [host-1], got %v", hosts)
	}
}

func TestStabilityIsNotLostOnInactivity(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")
	clock.Advance(15 * time.Minute)
	tracker.RecordHeartbeat("host-1")
	if !tracker.IsStable("host-1") {
		t.Fatal("host should be stable")
	}

	// After 50h (still within 72h), host is still stable.
	clock.Advance(50 * time.Hour)
	if !tracker.IsStable("host-1") {
		t.Fatal("stability should not be cleared by passage of time alone")
	}
}

// ---------------------------------------------------------------------------
// H3: Inactivity
// ---------------------------------------------------------------------------

func TestHostBecomesInactiveAfter72h(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")
	clock.Advance(15 * time.Minute)
	tracker.RecordHeartbeat("host-1") // now stable

	if !tracker.IsActive("host-1") {
		t.Fatal("host should be active right after heartbeat")
	}

	// Just within the window.
	clock.Advance(72 * time.Hour)
	if !tracker.IsActive("host-1") {
		t.Fatal("host should still be active at exactly 72h")
	}

	// Past the window.
	clock.Advance(1 * time.Second)
	if tracker.IsActive("host-1") {
		t.Fatal("host should be inactive after 72h + 1s")
	}

	// Stable but inactive — should NOT be in stable-active list.
	if got := tracker.StableActiveHosts(); len(got) != 0 {
		t.Fatalf("expected 0 stable-active hosts after inactivity, got %d", len(got))
	}
}

func TestHeartbeatResetsInactivityTimer(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")
	clock.Advance(15 * time.Minute)
	tracker.RecordHeartbeat("host-1")

	// Advance close to 72h, then send another heartbeat.
	clock.Advance(71 * time.Hour)
	tracker.RecordHeartbeat("host-1")

	// Another 71h — total 142h since first seen, but only 71h since last heartbeat.
	clock.Advance(71 * time.Hour)
	if !tracker.IsActive("host-1") {
		t.Fatal("host should still be active — heartbeat reset the timer")
	}
}

// ---------------------------------------------------------------------------
// Prune
// ---------------------------------------------------------------------------

func TestPruneRemovesInactiveHosts(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("keep")
	tracker.RecordHeartbeat("remove")

	// Make "remove" inactive.
	clock.Advance(73 * time.Hour)
	tracker.RecordHeartbeat("keep") // keep stays alive

	tracker.Prune()

	if tracker.HostCount() != 1 {
		t.Fatalf("expected 1 host after prune, got %d", tracker.HostCount())
	}
	if tracker.IsActive("remove") {
		t.Fatal("pruned host should not exist")
	}
	if !tracker.IsActive("keep") {
		t.Fatal("kept host should still be active")
	}
}

func TestPruneNoOpWhenAllActive(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("a")
	tracker.RecordHeartbeat("b")
	tracker.Prune()

	if tracker.HostCount() != 2 {
		t.Fatalf("expected 2 hosts, got %d", tracker.HostCount())
	}
}

// ---------------------------------------------------------------------------
// FirstSeen
// ---------------------------------------------------------------------------

func TestFirstSeen(t *testing.T) {
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := newTestClock(start)
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")
	clock.Advance(5 * time.Minute)
	tracker.RecordHeartbeat("host-1") // should not change firstSeen

	got := tracker.FirstSeen("host-1")
	if !got.Equal(start) {
		t.Fatalf("FirstSeen should be %v, got %v", start, got)
	}

	// Unknown host returns zero.
	if fs := tracker.FirstSeen("unknown"); !fs.IsZero() {
		t.Fatalf("FirstSeen for unknown host should be zero, got %v", fs)
	}
}

// ---------------------------------------------------------------------------
// Unknown host defaults
// ---------------------------------------------------------------------------

func TestUnknownHostDefaults(t *testing.T) {
	tracker := NewHostLifecycleTracker()

	if tracker.IsStable("nope") {
		t.Fatal("unknown host should not be stable")
	}
	if tracker.IsActive("nope") {
		t.Fatal("unknown host should not be active")
	}
}

// ---------------------------------------------------------------------------
// Multiple hosts
// ---------------------------------------------------------------------------

func TestMultipleHostsIndependent(t *testing.T) {
	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("a")
	clock.Advance(5 * time.Minute)
	tracker.RecordHeartbeat("b")
	clock.Advance(5 * time.Minute)
	// a: 10min elapsed, b: 5min elapsed
	tracker.RecordHeartbeat("a")
	tracker.RecordHeartbeat("b")

	if !tracker.IsStable("a") {
		t.Fatal("a should be stable (10min)")
	}
	if tracker.IsStable("b") {
		t.Fatal("b should not be stable yet (5min)")
	}

	clock.Advance(5 * time.Minute)
	tracker.RecordHeartbeat("b")
	if !tracker.IsStable("b") {
		t.Fatal("b should be stable now (10min)")
	}

	hosts := tracker.StableActiveHosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 stable-active hosts, got %d", len(hosts))
	}
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.json")

	clock := newTestClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	tracker := newTestTracker(clock)

	tracker.RecordHeartbeat("host-1")
	clock.Advance(15 * time.Minute)
	tracker.RecordHeartbeat("host-1") // stable

	tracker.RecordHeartbeat("host-2") // not stable yet

	if err := tracker.SaveState(path); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Load into a fresh tracker.
	tracker2 := newTestTracker(clock)
	if err := tracker2.LoadState(path); err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if !tracker2.IsStable("host-1") {
		t.Fatal("host-1 should be stable after load")
	}
	if tracker2.IsStable("host-2") {
		t.Fatal("host-2 should not be stable after load")
	}
	if tracker2.HostCount() != 2 {
		t.Fatalf("expected 2 hosts after load, got %d", tracker2.HostCount())
	}

	// FirstSeen preserved.
	expected := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if fs := tracker2.FirstSeen("host-1"); !fs.Equal(expected) {
		t.Fatalf("FirstSeen not preserved: want %v, got %v", expected, fs)
	}
}

func TestLoadStateMissingFile(t *testing.T) {
	tracker := NewHostLifecycleTracker()
	if err := tracker.LoadState("/nonexistent/path/lifecycle.json"); err != nil {
		t.Fatalf("LoadState on missing file should not error, got: %v", err)
	}
	if tracker.HostCount() != 0 {
		t.Fatal("tracker should be empty after loading missing file")
	}
}

func TestLoadStateEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.json")
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	// Pre-populate to verify empty file replaces state.
	tracker := NewHostLifecycleTracker()
	tracker.RecordHeartbeat("old")

	if err := tracker.LoadState(path); err != nil {
		t.Fatalf("LoadState on empty file should not error, got: %v", err)
	}
	if tracker.HostCount() != 0 {
		t.Fatalf("expected 0 hosts after loading empty file, got %d", tracker.HostCount())
	}
}

func TestLoadStateNullHostEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.json")
	// Simulate a corrupted file with a null entry.
	if err := os.WriteFile(path, []byte(`{"hosts":{"h1":null,"h2":{"first_seen":"2026-01-01T00:00:00Z","last_seen":"2026-01-01T00:15:00Z","stable_at":"2026-01-01T00:10:00Z"}}}`), 0600); err != nil {
		t.Fatal(err)
	}

	tracker := NewHostLifecycleTracker()
	if err := tracker.LoadState(path); err != nil {
		t.Fatalf("LoadState should handle null entries, got: %v", err)
	}
	// Null entry filtered out; valid entry kept.
	if tracker.HostCount() != 1 {
		t.Fatalf("expected 1 host (null filtered), got %d", tracker.HostCount())
	}
	if !tracker.IsStable("h2") {
		t.Fatal("h2 should be stable")
	}
}

func TestLoadStateNullHostsMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.json")
	if err := os.WriteFile(path, []byte(`{"hosts":null}`), 0600); err != nil {
		t.Fatal(err)
	}

	// Pre-populate tracker to verify LoadState replaces state.
	tracker := NewHostLifecycleTracker()
	tracker.RecordHeartbeat("old-host")

	if err := tracker.LoadState(path); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if tracker.HostCount() != 0 {
		t.Fatalf("expected 0 hosts after loading null map, got %d", tracker.HostCount())
	}
}

func TestLoadStateReplacesExistingState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lifecycle.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	tracker := NewHostLifecycleTracker()
	tracker.RecordHeartbeat("stale")

	if err := tracker.LoadState(path); err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	// Loading an empty object should replace existing state.
	if tracker.HostCount() != 0 {
		t.Fatalf("expected 0 hosts after loading empty state, got %d", tracker.HostCount())
	}
}

func TestEmptyHostIDIgnored(t *testing.T) {
	tracker := NewHostLifecycleTracker()
	tracker.RecordHeartbeat("")
	if tracker.HostCount() != 0 {
		t.Fatalf("empty hostID should be ignored, got %d hosts", tracker.HostCount())
	}
}

// ---------------------------------------------------------------------------
// Thread safety
// ---------------------------------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	tracker := NewHostLifecycleTracker()

	var wg sync.WaitGroup
	const goroutines = 50
	const iterations = 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			hostID := "host-" + string(rune('A'+id%26))
			for j := 0; j < iterations; j++ {
				tracker.RecordHeartbeat(hostID)
				tracker.IsStable(hostID)
				tracker.IsActive(hostID)
				tracker.FirstSeen(hostID)
				tracker.StableActiveHosts()
				if j%50 == 0 {
					tracker.Prune()
				}
			}
		}(i)
	}

	wg.Wait()

	// No panics or races = pass. Just verify basic sanity.
	if tracker.HostCount() == 0 {
		t.Fatal("expected some hosts after concurrent access")
	}
}
