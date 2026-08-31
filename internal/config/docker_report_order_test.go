package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerReportOrderStorePersistsAndClonesEntries(t *testing.T) {
	dir := t.TempDir()
	store := NewDockerReportOrderStore(dir, nil)
	observedAt := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	want := DockerReportOrderEntry{
		HostID: "docker-host", ObservedAt: observedAt, LastReceivedAt: observedAt.Add(time.Second),
		StreamID: "current", Sequence: 42, RetiredStreamIDs: []string{"old"},
	}
	if err := store.Upsert(want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reloaded := NewDockerReportOrderStore(dir, nil)
	got, ok := reloaded.Get(want.HostID)
	if !ok || got.StreamID != want.StreamID || got.Sequence != want.Sequence ||
		!got.ObservedAt.Equal(want.ObservedAt) || !got.LastReceivedAt.Equal(want.LastReceivedAt) {
		t.Fatalf("reloaded entry = %+v, ok=%v", got, ok)
	}
	got.RetiredStreamIDs[0] = "mutated"
	cloned, _ := reloaded.Get(want.HostID)
	if cloned.RetiredStreamIDs[0] != "old" {
		t.Fatalf("Get returned mutable retired stream slice: %+v", cloned)
	}
}

func TestDockerReportOrderStoreSaveFailureRollsBackMemory(t *testing.T) {
	dir := t.TempDir()
	store := NewDockerReportOrderStore(dir, nil)
	original := DockerReportOrderEntry{HostID: "host", StreamID: "stable", Sequence: 1}
	if err := store.Upsert(original); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	store.fs = &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write failed")}
	if err := store.Upsert(DockerReportOrderEntry{HostID: "host", StreamID: "new", Sequence: 2}); err == nil {
		t.Fatal("Upsert succeeded despite persistence failure")
	}
	got, _ := store.Get("host")
	if got.StreamID != original.StreamID || got.Sequence != original.Sequence {
		t.Fatalf("failed Upsert changed in-memory entry: %+v", got)
	}
}

func TestDockerReportOrderStoreReportsLoadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, dockerReportOrderFileName), []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid journal: %v", err)
	}
	store := NewDockerReportOrderStore(dir, nil)
	if err := store.LoadError(); err == nil {
		t.Fatal("LoadError = nil, want malformed journal failure")
	}
}

func TestDockerReportOrderStoreRejectsUnsafeJournalFiles(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		dir := t.TempDir()
		payload := bytes.Repeat([]byte("x"), int(maxDockerReportOrderFileSizeBytes+1))
		if err := os.WriteFile(filepath.Join(dir, dockerReportOrderFileName), payload, 0o600); err != nil {
			t.Fatalf("write oversized journal: %v", err)
		}
		if err := NewDockerReportOrderStore(dir, nil).LoadError(); err == nil || !strings.Contains(err.Error(), "exceeds max size") {
			t.Fatalf("LoadError = %v, want size-limit failure", err)
		}
	})

	t.Run("non-regular", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, dockerReportOrderFileName), 0o700); err != nil {
			t.Fatalf("create directory journal: %v", err)
		}
		if err := NewDockerReportOrderStore(dir, nil).LoadError(); err == nil || !strings.Contains(err.Error(), "non-regular") {
			t.Fatalf("LoadError = %v, want non-regular-file failure", err)
		}
	})
}
