package config

import (
	"testing"
	"time"
)

func TestHostContinuityStoreRoundTripAndMatch(t *testing.T) {
	dir := t.TempDir()
	store := NewHostContinuityStore(dir, nil)

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.Upsert(HostContinuityEntry{
		HostID:          "host-1",
		ReportHostID:    "machine-1",
		AgentReportedID: "agent-1",
		Hostname:        "host-1.local",
		MachineID:       "machine-1",
		TokenID:         "token-1",
		LastSeen:        now,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reloaded := NewHostContinuityStore(dir, nil)
	if entry, ok := reloaded.Get("host-1"); !ok {
		t.Fatal("expected reloaded entry")
	} else if entry.TokenID != "token-1" || entry.Hostname != "host-1.local" {
		t.Fatalf("unexpected reloaded entry: %+v", entry)
	}

	if entry, ok := reloaded.Match("machine-1", "", "", "", "", now.Add(-time.Minute)); !ok {
		t.Fatal("expected match by report host ID")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
	}

	if entry, ok := reloaded.Match("", "", "", "host-1.local", "token-1", now.Add(-time.Minute)); !ok {
		t.Fatal("expected match by hostname and token")
	} else if entry.HostID != "host-1" {
		t.Fatalf("matched host ID = %q, want %q", entry.HostID, "host-1")
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
