package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestStore_ListRollups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	t1 := time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC)

	points := []recovery.RecoveryPoint{
		{
			ID:                "p1",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeSuccess,
			StartedAt:         &t1,
			CompletedAt:       &t1,
			SubjectResourceID: "vm-1",
		},
		{
			ID:                "p2",
			Provider:          recovery.ProviderKubernetes,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeFailed,
			StartedAt:         &t2,
			CompletedAt:       &t2,
			SubjectResourceID: "vm-1",
		},
		{
			ID:          "p3",
			Provider:    recovery.ProviderTrueNAS,
			Kind:        recovery.KindSnapshot,
			Mode:        recovery.ModeSnapshot,
			Outcome:     recovery.OutcomeSuccess,
			StartedAt:   &t3,
			CompletedAt: &t3,
			SubjectRef: &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: "tank/apps",
				ID:   "tank/apps",
			},
		},
	}

	if err := store.UpsertPoints(context.Background(), points); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	got, total, err := store.ListRollups(context.Background(), recovery.ListPointsOptions{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListRollups() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if len(got) != 2 {
		t.Fatalf("len(rollups) = %d, want 2", len(got))
	}

	// Newest attempt first: TrueNAS dataset at t3.
	if got[0].LastAttemptAt == nil || !got[0].LastAttemptAt.Equal(t3) {
		t.Fatalf("rollup[0].LastAttemptAt = %v, want %v", got[0].LastAttemptAt, t3)
	}
	if got[0].LastOutcome != recovery.OutcomeSuccess {
		t.Fatalf("rollup[0].LastOutcome = %q, want %q", got[0].LastOutcome, recovery.OutcomeSuccess)
	}
	if got[0].RollupID == "" || got[0].RollupID == "res:vm-1" {
		t.Fatalf("rollup[0].RollupID = %q, want external key", got[0].RollupID)
	}
	if got[0].SubjectResourceID != "" {
		t.Fatalf("rollup[0].SubjectResourceID = %q, want empty for external subject", got[0].SubjectResourceID)
	}
	if got[0].SubjectRef == nil || got[0].SubjectRef.Type != "truenas-dataset" || got[0].SubjectRef.Name != "tank/apps" {
		t.Fatalf("rollup[0].SubjectRef = %#v, want truenas dataset ref", got[0].SubjectRef)
	}

	// Second: vm-1 with latest failure at t2 and last success at t1.
	if got[1].RollupID != "res:vm-1" {
		t.Fatalf("rollup[1].RollupID = %q, want %q", got[1].RollupID, "res:vm-1")
	}
	if got[1].SubjectResourceID != "vm-1" {
		t.Fatalf("rollup[1].SubjectResourceID = %q, want %q", got[1].SubjectResourceID, "vm-1")
	}
	if got[1].SubjectRef != nil {
		t.Fatalf("rollup[1].SubjectRef = %#v, want nil", got[1].SubjectRef)
	}
	if got[1].LastAttemptAt == nil || !got[1].LastAttemptAt.Equal(t2) {
		t.Fatalf("rollup[1].LastAttemptAt = %v, want %v", got[1].LastAttemptAt, t2)
	}
	if got[1].LastSuccessAt == nil || !got[1].LastSuccessAt.Equal(t1) {
		t.Fatalf("rollup[1].LastSuccessAt = %v, want %v", got[1].LastSuccessAt, t1)
	}
	if got[1].LastOutcome != recovery.OutcomeFailed {
		t.Fatalf("rollup[1].LastOutcome = %q, want %q", got[1].LastOutcome, recovery.OutcomeFailed)
	}

	// Smoke: From filter excludes older entries.
	from := t3.Add(-1 * time.Minute)
	got2, total2, err := store.ListRollups(context.Background(), recovery.ListPointsOptions{From: &from, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListRollups(from) error = %v", err)
	}
	if total2 != 1 || len(got2) != 1 {
		t.Fatalf("ListRollups(from) total=%d len=%d, want 1/1", total2, len(got2))
	}
}
