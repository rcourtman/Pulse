package config

import (
	"errors"
	"os"
	"path/filepath"
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
