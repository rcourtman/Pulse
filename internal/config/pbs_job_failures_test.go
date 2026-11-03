package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func failedPBSJob(id string) models.PBSJobHealthEvidence {
	ended := time.Date(2026, 11, 3, 6, 0, 0, 0, time.UTC)
	return models.PBSJobHealthEvidence{
		ID:             id,
		Family:         "sync",
		Store:          "offsite",
		Remote:         "pbs-main",
		Enabled:        true,
		LastRunState:   "connection error",
		LastRunEndtime: ended.Unix(),
		Posture:        "critical",
		PostureReason:  "last-run-failed",
		Freshness: models.PBSJobHealthFreshness{
			ObservedAt:     time.Now(),
			LastRunEndTime: ended,
			AgeSeconds:     42,
			State:          "observed",
		},
	}
}

func TestPBSJobFailureStoreKeepsFailuresAcrossARestart(t *testing.T) {
	dir := t.TempDir()
	store := NewPBSJobFailureStore(dir, nil)
	healthy := failedPBSJob("v-1")
	healthy.Family, healthy.Posture, healthy.PostureReason = "verify", "healthy", "last-run-success"
	if err := store.Set("pbs-main", "https://pbs-a:8007", []models.PBSJobHealthEvidence{failedPBSJob("s-1"), healthy}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	reopened := NewPBSJobFailureStore(dir, nil)
	if err := reopened.LoadError(); err != nil {
		t.Fatalf("LoadError: %v", err)
	}
	got := reopened.Get("pbs-main", "https://pbs-a:8007")
	if len(got) != 1 || got[0].ID != "s-1" || got[0].Posture != "critical" || got[0].Remote != "pbs-main" {
		t.Fatalf("reloaded failures = %+v, want only the failed sync", got)
	}
	// Poll times are not part of the record, so an unchanged failure never
	// rewrites the file.
	if !got[0].Freshness.ObservedAt.IsZero() || got[0].Freshness.AgeSeconds != 0 {
		t.Fatalf("poll-time fields were persisted: %+v", got[0].Freshness)
	}
	if got[0].Freshness.LastRunEndTime.IsZero() {
		t.Fatal("the failed run's end time must be kept")
	}
}

func TestPBSJobFailureStoreDoesNotRewriteAnUnchangedFailure(t *testing.T) {
	dir := t.TempDir()
	store := NewPBSJobFailureStore(dir, nil)
	if err := store.Set("pbs-main", "https://pbs-a:8007", []models.PBSJobHealthEvidence{failedPBSJob("s-1")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	path := filepath.Join(dir, pbsJobFailuresFileName)
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	next := failedPBSJob("s-1")
	next.Freshness.ObservedAt = time.Now().Add(time.Minute)
	next.Freshness.AgeSeconds = 102
	if err := store.Set("pbs-main", "https://pbs-a:8007", []models.PBSJobHealthEvidence{next}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("an unchanged failure rewrote the record at %v", info.ModTime())
	}
}

func TestPBSJobFailureStoreForgetsAnInstanceWithNoFailures(t *testing.T) {
	dir := t.TempDir()
	store := NewPBSJobFailureStore(dir, nil)
	if err := store.Set("pbs-main", "https://pbs-a:8007", []models.PBSJobHealthEvidence{failedPBSJob("s-1")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Set("pbs-main", "https://pbs-a:8007", nil); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := NewPBSJobFailureStore(dir, nil).Get("pbs-main", "https://pbs-a:8007"); len(got) != 0 {
		t.Fatalf("cleared instance reloaded %+v", got)
	}
	var missing *PBSJobFailureStore
	if missing.Get("pbs-main", "https://pbs-a:8007") != nil || missing.Set("pbs-main", "https://pbs-a:8007", nil) != nil || missing.Retain(nil) != nil {
		t.Fatal("a nil store must be inert")
	}
}

func TestPBSJobFailureStoreReportsAnUnreadableRecord(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, pbsJobFailuresFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store := NewPBSJobFailureStore(dir, nil)
	if store.LoadError() == nil {
		t.Fatal("expected a load error for a corrupt record")
	}
	if got := store.Get("pbs-main", "https://pbs-a:8007"); len(got) != 0 {
		t.Fatalf("corrupt record yielded %+v", got)
	}
}

func TestPBSJobFailureStoreWithoutADataDirectoryWritesNothing(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	store := NewPBSJobFailureStore("", nil)
	if err := store.Set("pbs-main", "https://pbs-a:8007", []models.PBSJobHealthEvidence{failedPBSJob("s-1")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := store.Get("pbs-main", "https://pbs-a:8007"); len(got) != 1 {
		t.Fatalf("in-memory record = %+v, want the failure", got)
	}
	if _, err := os.Stat(filepath.Join(cwd, pbsJobFailuresFileName)); !os.IsNotExist(err) {
		t.Fatalf("record written to the working directory: %v", err)
	}
}

func TestPBSJobFailureStoreBindsARecordToItsServer(t *testing.T) {
	store := NewPBSJobFailureStore(t.TempDir(), nil)
	if err := store.Set("pbs-main", "https://PBS-A:8007/", []models.PBSJobHealthEvidence{failedPBSJob("s-1")}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := store.Get("pbs-main", "https://pbs-a:8007"); len(got) != 1 {
		t.Fatalf("same server, other spelling = %+v", got)
	}
	if got := store.Get("pbs-main", "https://pbs-b:8007"); got != nil {
		t.Fatalf("another server under the same name got %+v", got)
	}
}

func TestPBSJobFailureStoreRetainDropsRemovedInstances(t *testing.T) {
	dir := t.TempDir()
	store := NewPBSJobFailureStore(dir, nil)
	for _, id := range []string{"pbs-main", "pbs-old"} {
		if err := store.Set(id, "https://pbs-a:8007", []models.PBSJobHealthEvidence{failedPBSJob("s-1")}); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	if err := store.Retain([]string{"pbs-main"}); err != nil {
		t.Fatalf("Retain: %v", err)
	}
	reopened := NewPBSJobFailureStore(dir, nil)
	if len(reopened.Get("pbs-main", "https://pbs-a:8007")) != 1 || len(reopened.Get("pbs-old", "https://pbs-a:8007")) != 0 {
		t.Fatal("Retain must keep configured instances and drop the rest")
	}
}

func TestPBSJobFailureStoreNeverWritesAnUnreadableRecord(t *testing.T) {
	dir := t.TempDir()
	store := NewPBSJobFailureStore(dir, nil)
	many := make([]models.PBSJobHealthEvidence, 0, 400)
	for i := 0; i < 400; i++ {
		job := failedPBSJob(fmt.Sprintf("s-%03d-%s", i, strings.Repeat("x", 200)))
		job.Remote = strings.Repeat("r", 400)
		many = append(many, job)
	}
	var lastErr error
	for i := 0; i < 20 && lastErr == nil; i++ {
		lastErr = store.Set(fmt.Sprintf("pbs-%02d", i), "https://pbs-a:8007", many)
	}
	if lastErr == nil {
		t.Fatal("expected the size bound to refuse a write")
	}
	reopened := NewPBSJobFailureStore(dir, nil)
	if err := reopened.LoadError(); err != nil {
		t.Fatalf("the record on disk must stay readable: %v", err)
	}
	if got := reopened.Get("pbs-00", "https://pbs-a:8007"); len(got) != maxPBSJobFailuresPerInstance {
		t.Fatalf("per-instance cap = %d, want %d", len(got), maxPBSJobFailuresPerInstance)
	}
}
