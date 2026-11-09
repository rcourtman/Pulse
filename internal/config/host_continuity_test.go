package config

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestHostContinuityStoreRoundTripAndMatch(t *testing.T) {
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.Upsert(HostContinuityEntry{
		HostID:               "host-1",
		ReportHostID:         "machine-1",
		AgentReportedID:      "agent-1",
		Hostname:             "host-1.local",
		MachineID:            "machine-1",
		TokenID:              "token-1",
		LastSeen:             now,
		IntervalSeconds:      30,
		ReportObservedAt:     now.Add(-time.Second),
		ReportLastReceivedAt: now.Add(time.Second),
		ReportStreamID:       "stream-current",
		ReportSequence:       14,
		RetiredReportStreamIDs: []string{
			"stream-old",
			"stream-old",
		},
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reloaded := NewHostContinuityStore(dir, nil)
	if entry, ok := reloaded.Get("host-1"); !ok {
		t.Fatal("expected reloaded entry")
	} else if entry.TokenID != "token-1" ||
		entry.Hostname != "host-1.local" ||
		entry.IntervalSeconds != 30 ||
		entry.ReportObservedAt != now.Add(-time.Second) ||
		entry.ReportLastReceivedAt != now.Add(time.Second) ||
		entry.ReportStreamID != "stream-current" ||
		entry.ReportSequence != 14 ||
		len(entry.RetiredReportStreamIDs) != 1 ||
		entry.RetiredReportStreamIDs[0] != "stream-old" {
		t.Fatalf("unexpected reloaded entry: %+v", entry)
	}

	if entry, ok := reloaded.Match("machine-1", "", "", "", "", now.Add(-time.Minute)); !ok {
		t.Fatal("expected match by report host ID")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
	}
	if entry, ok := reloaded.Match("MACHINE-1", "", "", "", "", now.Add(-time.Minute)); !ok {
		t.Fatal("expected machine identity aliases to match case-insensitively")
	} else if entry.HostID != "host-1" {
		t.Fatalf("case-insensitive alias matched host ID = %q, want %q", entry.HostID, "host-1")
	}

	if entry, ok := reloaded.Match("", "", "", "host-1.local", "token-1", now.Add(-time.Minute)); !ok {
		t.Fatal("expected match by hostname and token")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
	}

	if entry, ok := reloaded.Match("", "", "", "host-1", "token-1", now.Add(-time.Minute)); !ok {
		t.Fatal("expected match by equivalent short hostname and token")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
	}

	if _, ok := reloaded.Match("", "", "", "host-1.example", "token-1", now.Add(-time.Minute)); ok {
		t.Fatal("expected distinct fully-qualified hostnames not to match")
	}

	if _, ok := reloaded.Match("", "", "", "host-1.local", "token-2", now.Add(-time.Minute)); ok {
		t.Fatal("expected token mismatch not to match")
	}

	// #1753: hostname+token alone must not fold a different machine into this
	// entry. Two standalone sites reusing one short hostname and one shared
	// install token report distinct machine IDs.
	if _, ok := reloaded.Match("", "machine-other-site", "", "host-1.local", "token-1", now.Add(-time.Minute)); ok {
		t.Fatal("expected hostname+token match to be rejected for a conflicting machine ID")
	}
	if entry, ok := reloaded.Match("", "machine-1", "", "host-1.local", "token-1", now.Add(-time.Minute)); !ok {
		t.Fatal("expected hostname+token match to survive an agreeing machine ID")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
	}
}

func TestHostContinuityStoreRecentEntriesFiltersStaleState(t *testing.T) {
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)

	now := time.Now().UTC().Truncate(time.Second)
	for _, entry := range []HostContinuityEntry{
		{HostID: "host-new", Hostname: "new.local", LastSeen: now},
		{HostID: "host-old", Hostname: "old.local", LastSeen: now.Add(-96 * time.Hour)},
	} {
		if err := store.Upsert(entry); err != nil {
			t.Fatalf("Upsert(%s): %v", entry.HostID, err)
		}
	}

	recent := store.RecentEntries(now.Add(-72 * time.Hour))
	if len(recent) != 1 {
		t.Fatalf("RecentEntries count = %d, want 1", len(recent))
	}
	if recent[0].HostID != "host-new" {
		t.Fatalf("RecentEntries[0].HostID = %q, want %q", recent[0].HostID, "host-new")
	}
}

func TestHostContinuityStoreRemovalTombstoneRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)
	now := time.Now().UTC().Truncate(time.Second)

	entry := HostContinuityEntry{
		HostID:          "host-removed",
		ReportHostID:    "systemd-machine-id",
		AgentReportedID: "persisted-agent-id",
		Hostname:        "removed.local",
		MachineID:       "systemd-machine-id",
		TokenID:         "old-token",
		DeniedTokenIDs:  []string{"old-token"},
		LastSeen:        now.Add(-time.Minute),
		RemovedAt:       now,
	}
	if err := store.Upsert(entry); err != nil {
		t.Fatalf("Upsert tombstone: %v", err)
	}

	reloaded := NewHostContinuityStore(dir, nil)
	if err := reloaded.LoadError(); err != nil {
		t.Fatalf("LoadError: %v", err)
	}
	removed := reloaded.RemovedEntries()
	if len(removed) != 1 || removed[0].HostID != entry.HostID || !removed[0].RemovedAt.Equal(now) {
		t.Fatalf("RemovedEntries = %+v, want tombstone for %q at %v", removed, entry.HostID, now)
	}
	if got := reloaded.RecentEntries(time.Time{}); len(got) != 0 {
		t.Fatalf("RecentEntries included tombstone: %+v", got)
	}
	if _, ok := reloaded.Match(entry.ReportHostID, entry.MachineID, entry.AgentReportedID, entry.Hostname, entry.TokenID, time.Time{}); ok {
		t.Fatal("Match returned a removal tombstone as active continuity")
	}

	cleared, err := reloaded.ClearRemoval(entry.HostID, "fresh-token", false)
	if err != nil {
		t.Fatalf("ClearRemoval: %v", err)
	}
	if !cleared {
		t.Fatal("ClearRemoval reported no tombstone")
	}
	if got := NewHostContinuityStore(dir, nil).RemovedEntries(); len(got) != 0 {
		t.Fatalf("tombstone survived ClearRemoval: %+v", got)
	}
	if active := reloaded.RecentEntries(time.Time{}); len(active) != 1 || active[0].HostID != entry.HostID {
		t.Fatalf("active continuity after ClearRemoval = %+v", active)
	} else if active[0].TokenID != "fresh-token" ||
		len(active[0].DeniedTokenIDs) != 1 ||
		active[0].DeniedTokenIDs[0] != "old-token" {
		t.Fatalf("ClearRemoval lost token lineage: %+v", active[0])
	}

	if err := reloaded.Upsert(entry); err != nil {
		t.Fatalf("restore tombstone for manual allowance: %v", err)
	}
	if cleared, err := reloaded.ClearRemoval(entry.HostID, "", true); err != nil || !cleared {
		t.Fatalf("manual ClearRemoval = (%v, %v)", cleared, err)
	}
	if active := reloaded.RecentEntries(time.Time{}); len(active) != 1 || len(active[0].DeniedTokenIDs) != 0 {
		t.Fatalf("manual ClearRemoval retained denied token lineage: %+v", active)
	}
}

func TestHostContinuityStoreLoadsLegacyEntryWithoutRemoval(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"legacy-host":{"hostId":"legacy-host","hostname":"legacy.local","lastSeen":"2026-07-01T12:00:00Z"}}`)
	if err := os.WriteFile(filepath.Join(dir, "host_continuity.json"), legacy, 0o600); err != nil {
		t.Fatalf("write legacy continuity: %v", err)
	}

	store := NewHostContinuityStore(dir, nil)
	if err := store.LoadError(); err != nil {
		t.Fatalf("LoadError: %v", err)
	}
	if got := store.RemovedEntries(); len(got) != 0 {
		t.Fatalf("legacy entry migrated as removed: %+v", got)
	}
	if got := store.RecentEntries(time.Time{}); len(got) != 1 || got[0].HostID != "legacy-host" {
		t.Fatalf("legacy active continuity = %+v", got)
	}
}

func TestHostContinuityStoreMutationsRollbackWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)
	original := HostContinuityEntry{
		HostID:   "host-rollback",
		Hostname: "rollback.local",
		LastSeen: time.Now().UTC().Truncate(time.Second),
	}
	if err := store.Upsert(original); err != nil {
		t.Fatalf("initial Upsert: %v", err)
	}

	store.fs = &mockFSError{
		FileSystem: defaultFileSystem{},
		writeError: errors.New("write failed"),
	}
	tombstone := original
	tombstone.RemovedAt = time.Now().UTC()
	if err := store.Upsert(tombstone); err == nil {
		t.Fatal("Upsert succeeded despite persistence failure")
	}
	if got, ok := store.Get(original.HostID); !ok || !got.RemovedAt.IsZero() {
		t.Fatalf("failed Upsert changed in-memory entry: %+v, ok=%v", got, ok)
	}

	if err := store.Delete(original.HostID); err == nil {
		t.Fatal("Delete succeeded despite persistence failure")
	}
	if _, ok := store.Get(original.HostID); !ok {
		t.Fatal("failed Delete removed in-memory entry")
	}

	store.fs = defaultFileSystem{}
	if err := store.Upsert(tombstone); err != nil {
		t.Fatalf("persist tombstone: %v", err)
	}
	store.fs = &mockFSError{
		FileSystem: defaultFileSystem{},
		writeError: errors.New("write failed"),
	}
	if cleared, err := store.ClearRemoval(original.HostID, "fresh-token", false); err == nil || cleared {
		t.Fatalf("ClearRemoval = (%v, %v), want persistence error", cleared, err)
	}
	if got, ok := store.Get(original.HostID); !ok || got.RemovedAt.IsZero() {
		t.Fatalf("failed ClearRemoval erased in-memory tombstone: %+v, ok=%v", got, ok)
	}
}

func TestHostContinuityStoreReportsLoadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "host_continuity.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write continuity fixture: %v", err)
	}
	store := NewHostContinuityStore(dir, &mockFSError{
		FileSystem: defaultFileSystem{},
		readError:  errors.New("read failed"),
	})
	if err := store.LoadError(); err == nil {
		t.Fatal("LoadError = nil, want continuity read failure")
	}
}

func TestHostContinuityNodeLinkIntentsPreserveJournal(t *testing.T) {
	dir := t.TempDir()
	s := NewHostContinuityStore(dir, nil)
	entry := HostContinuityEntry{HostID: "a", LinkedNodeID: "n", NodeLinkSource: "manual",
		TokenID: "synthetic-token", DeniedTokenIDs: []string{"synthetic-denied"},
		ReportStreamID: "stream", ReportSequence: 42, RemovedAt: time.Now().UTC()}
	if err := s.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	if err := s.SetNodeLinkIntents([]HostContinuityEntry{
		{HostID: "a", NodeLinkSource: "unlinked"},
		{HostID: "b", LinkedNodeID: "n", NodeLinkSource: "manual"},
	}); err != nil {
		t.Fatal(err)
	}
	reloaded := NewHostContinuityStore(dir, nil)
	a, _ := reloaded.Get("a")
	b, _ := reloaded.Get("b")
	if a.NodeLinkSource != "unlinked" || a.LinkedNodeID != "" || b.NodeLinkSource != "manual" || b.LinkedNodeID != "n" {
		t.Fatal("replacement intent lost across reload")
	}
	if a.TokenID != entry.TokenID || len(a.DeniedTokenIDs) != 1 || a.ReportSequence != 42 || !a.RemovedAt.Equal(entry.RemovedAt) {
		t.Fatal("link update changed lifecycle evidence")
	}
}

func TestHostContinuityNodeLinkReplacesDormantOwner(t *testing.T) {
	dir := t.TempDir()
	s := NewHostContinuityStore(dir, nil)
	if err := s.SetNodeLinkIntents([]HostContinuityEntry{{HostID: "a", LinkedNodeID: "n", NodeLinkSource: "manual"}}); err != nil {
		t.Fatal(err)
	}
	if !s.NodeLinkReservedByOther("b", "n") || s.NodeLinkReservedByOther("a", "n") {
		t.Fatal("reservation ownership incorrect")
	}
	if err := s.SetNodeLinkIntents([]HostContinuityEntry{{HostID: "b", LinkedNodeID: "n", NodeLinkSource: "manual"}}); err != nil {
		t.Fatal(err)
	}
	s = NewHostContinuityStore(dir, nil)
	a, _ := s.Get("a")
	if a.LinkedNodeID != "" || a.NodeLinkSource != "unlinked" {
		t.Fatal("dormant owner can reclaim replacement link")
	}
	if !s.NodeLinkReservedByOther("a", "n") || s.NodeLinkReservedByOther("b", "n") {
		t.Fatal("replacement reservation incorrect")
	}
}

// Replacing a dormant owner changes two entries even though the caller supplies
// only the new owner. A failed transaction must roll both back, including the
// reservation used by automatic matching before either host reconnects.
func TestHostContinuityNodeLinkReplacementRollback(t *testing.T) {
	dir := t.TempDir()
	s := NewHostContinuityStore(dir, nil)
	for _, entry := range []HostContinuityEntry{
		{HostID: "owner", LinkedNodeID: "node", NodeLinkSource: "manual", ReportStreamID: "owner-stream", ReportSequence: 42},
		{HostID: "replacement", NodeLinkSource: "unlinked", ReportStreamID: "replacement-stream", ReportSequence: 17},
	} {
		if err := s.Upsert(entry); err != nil {
			t.Fatal(err)
		}
	}
	assertOwner := func(store *HostContinuityStore, ownerID string) {
		t.Helper()
		for id, sequence := range map[string]uint64{"owner": 42, "replacement": 17} {
			entry, ok := store.Get(id)
			wantNode, wantSource := "", "unlinked"
			if id == ownerID {
				wantNode, wantSource = "node", "manual"
			}
			if !ok || entry.LinkedNodeID != wantNode || entry.NodeLinkSource != wantSource || entry.ReportSequence != sequence {
				t.Fatalf("%s ownership or report watermark changed: %+v", id, entry)
			}
			if got := store.NodeLinkReservedByOther(id, "node"); got != (id != ownerID) {
				t.Fatalf("reservation for %s = %v, owner %s", id, got, ownerID)
			}
		}
	}
	s.fs = &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("synthetic journal failure")}
	links := []HostContinuityEntry{{HostID: "replacement", LinkedNodeID: "node", NodeLinkSource: "manual"}}
	if err := s.SetNodeLinkIntents(links); err == nil {
		t.Fatal("replacement acknowledged despite failed persistence")
	}
	assertOwner(s, "owner")
	assertOwner(NewHostContinuityStore(dir, nil), "owner")
	// Retry on the same store, not a reload that could conceal memory corruption.
	s.fs = defaultFileSystem{}
	if err := s.SetNodeLinkIntents(links); err != nil {
		t.Fatal(err)
	}
	assertOwner(s, "replacement")
	assertOwner(NewHostContinuityStore(dir, nil), "replacement")
}

type countingContinuityFS struct {
	FileSystem
	mu     sync.Mutex
	writes int
}

func (c *countingContinuityFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	c.mu.Lock()
	c.writes++
	c.mu.Unlock()
	return c.FileSystem.WriteFile(name, data, perm)
}

func (c *countingContinuityFS) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes
}

func withContinuityFlushDelay(t *testing.T, delay time.Duration) {
	t.Helper()
	previous := hostContinuityLivenessFlushDelay
	hostContinuityLivenessFlushDelay = delay
	t.Cleanup(func() { hostContinuityLivenessFlushDelay = previous })
}

func observedContinuityEntry(sequence uint64, at time.Time) HostContinuityEntry {
	return HostContinuityEntry{
		HostID:               "host-observed",
		AgentReportedID:      "agent-observed",
		Hostname:             "observed.local",
		MachineID:            "machine-observed",
		TokenID:              "token-observed",
		AgentVersion:         "6.7.0",
		Platform:             "linux",
		LastSeen:             at,
		IntervalSeconds:      30,
		ReportObservedAt:     at.Add(-time.Second),
		ReportLastReceivedAt: at,
		ReportStreamID:       "stream-a",
		ReportSequence:       sequence,
	}
}

func reloadedContinuitySequence(t *testing.T, dir string) uint64 {
	t.Helper()
	entry, ok := NewHostContinuityStore(dir, nil).Get("host-observed")
	if !ok {
		t.Fatal("journal has no observed host")
	}
	return entry.ReportSequence
}

func TestHostContinuityObserveReportDefersLivenessOnlyChanges(t *testing.T) {
	withContinuityFlushDelay(t, time.Hour)
	dir := t.TempDir()
	fs := &countingContinuityFS{FileSystem: defaultFileSystem{}}
	store := NewHostContinuityStore(dir, fs)
	start := time.Now().UTC().Truncate(time.Second)

	if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
		t.Fatalf("first ObserveReport: %v", err)
	}
	if fs.count() != 1 {
		t.Fatalf("a new host must be written at once; writes = %d", fs.count())
	}
	for sequence := uint64(2); sequence <= 6; sequence++ {
		if err := store.ObserveReport(observedContinuityEntry(sequence, start.Add(time.Duration(sequence)*30*time.Second))); err != nil {
			t.Fatalf("ObserveReport %d: %v", sequence, err)
		}
	}
	if fs.count() != 1 {
		t.Fatalf("liveness-only reports rewrote the journal %d times", fs.count()-1)
	}
	if entry, _ := store.Get("host-observed"); entry.ReportSequence != 6 {
		t.Fatalf("in-memory watermark = %d, want 6", entry.ReportSequence)
	}
	if got := reloadedContinuitySequence(t, dir); got != 1 {
		t.Fatalf("journal watermark before flush = %d, want 1", got)
	}
	if err := store.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := reloadedContinuitySequence(t, dir); got != 6 {
		t.Fatalf("journal watermark after flush = %d, want 6", got)
	}
	if err := store.Flush(); err != nil || fs.count() != 2 {
		t.Fatalf("a clean Flush wrote again: err=%v writes=%d", err, fs.count())
	}
}

func TestHostContinuityObserveReportWritesBoundariesAtOnce(t *testing.T) {
	withContinuityFlushDelay(t, time.Hour)
	start := time.Now().UTC().Truncate(time.Second)
	for name, mutate := range map[string]func(*HostContinuityEntry){
		"new stream":    func(e *HostContinuityEntry) { e.ReportStreamID, e.ReportSequence = "stream-b", 1 },
		"retired":       func(e *HostContinuityEntry) { e.RetiredReportStreamIDs = []string{"stream-old"} },
		"token":         func(e *HostContinuityEntry) { e.TokenID = "token-rotated" },
		"denied token":  func(e *HostContinuityEntry) { e.DeniedTokenIDs = []string{"token-old"} },
		"hostname":      func(e *HostContinuityEntry) { e.Hostname = "renamed.local" },
		"machine":       func(e *HostContinuityEntry) { e.MachineID = "machine-other" },
		"node link":     func(e *HostContinuityEntry) { e.LinkedNodeID, e.NodeLinkSource = "node-1", "manual" },
		"agent version": func(e *HostContinuityEntry) { e.AgentVersion = "6.8.0" },
		"interval":      func(e *HostContinuityEntry) { e.IntervalSeconds = 60 },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			fs := &countingContinuityFS{FileSystem: defaultFileSystem{}}
			store := NewHostContinuityStore(dir, fs)
			if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
				t.Fatalf("seed: %v", err)
			}
			next := observedContinuityEntry(2, start.Add(30*time.Second))
			mutate(&next)
			if err := store.ObserveReport(next); err != nil {
				t.Fatalf("ObserveReport: %v", err)
			}
			if fs.count() != 2 {
				t.Fatalf("%s change was deferred; writes = %d", name, fs.count())
			}
			reloaded, _ := NewHostContinuityStore(dir, nil).Get("host-observed")
			if reloaded.ReportSequence != next.ReportSequence || reloaded.ReportStreamID != next.ReportStreamID {
				t.Fatalf("journal = %+v, want the %s change", reloaded, name)
			}
		})
	}
}

func TestHostContinuityObserveReportFlushesAfterDelay(t *testing.T) {
	withContinuityFlushDelay(t, 20*time.Millisecond)
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)
	start := time.Now().UTC().Truncate(time.Second)
	if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.ObserveReport(observedContinuityEntry(2, start.Add(30*time.Second))); err != nil {
		t.Fatalf("ObserveReport: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for reloadedContinuitySequence(t, dir) != 2 {
		if time.Now().After(deadline) {
			t.Fatal("deferred liveness change never reached the journal")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestHostContinuitySynchronousWriteCarriesPendingLiveness(t *testing.T) {
	withContinuityFlushDelay(t, time.Hour)
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)
	start := time.Now().UTC().Truncate(time.Second)
	if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.ObserveReport(observedContinuityEntry(9, start.Add(time.Minute))); err != nil {
		t.Fatalf("ObserveReport: %v", err)
	}
	if err := store.Upsert(HostContinuityEntry{HostID: "host-other", Hostname: "other.local", LastSeen: start}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got := reloadedContinuitySequence(t, dir); got != 9 {
		t.Fatalf("journal watermark = %d, want the pending 9 written with the other host", got)
	}
}

func TestHostContinuityCloseFlushesAndStopsDeferring(t *testing.T) {
	withContinuityFlushDelay(t, time.Hour)
	dir := t.TempDir()
	fs := &countingContinuityFS{FileSystem: defaultFileSystem{}}
	store := NewHostContinuityStore(dir, fs)
	start := time.Now().UTC().Truncate(time.Second)
	if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.ObserveReport(observedContinuityEntry(2, start.Add(30*time.Second))); err != nil {
		t.Fatalf("ObserveReport: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := reloadedContinuitySequence(t, dir); got != 2 {
		t.Fatalf("journal watermark after Close = %d, want 2", got)
	}
	before := fs.count()
	if err := store.ObserveReport(observedContinuityEntry(3, start.Add(time.Minute))); err != nil {
		t.Fatalf("ObserveReport after Close: %v", err)
	}
	if fs.count() != before+1 || reloadedContinuitySequence(t, dir) != 3 {
		t.Fatalf("observation after Close was deferred; writes %d -> %d", before, fs.count())
	}
}

func TestHostContinuityFlushReportsDeferredWriteFailure(t *testing.T) {
	withContinuityFlushDelay(t, time.Hour)
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)
	start := time.Now().UTC().Truncate(time.Second)
	if err := store.ObserveReport(observedContinuityEntry(1, start)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.ObserveReport(observedContinuityEntry(2, start.Add(30*time.Second))); err != nil {
		t.Fatalf("ObserveReport: %v", err)
	}
	store.fs = &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write failed")}
	if err := store.Flush(); err == nil {
		t.Fatal("Flush hid a persistence failure")
	}
	if entry, _ := store.Get("host-observed"); entry.ReportSequence != 2 {
		t.Fatalf("failed flush dropped the in-memory watermark: %d", entry.ReportSequence)
	}
	store.fs = defaultFileSystem{}
	if err := store.Flush(); err != nil {
		t.Fatalf("Flush after repair: %v", err)
	}
	if got := reloadedContinuitySequence(t, dir); got != 2 {
		t.Fatalf("journal watermark after repaired flush = %d, want 2", got)
	}
}
