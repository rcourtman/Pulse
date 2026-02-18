package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestStore_UpsertAndList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	size := int64(1234)
	verified := true

	point := recovery.RecoveryPoint{
		ID:          "point-1",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &now,
		CompletedAt: &now,
		SizeBytes:   &size,
		Verified:    &verified,
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
			UID:       "pvc-uid",
		},
		Details: map[string]any{"foo": "bar"},
	}

	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	got, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Provider: recovery.ProviderKubernetes, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListPoints() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(got) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(got))
	}
	if got[0].ID != "point-1" {
		t.Fatalf("point id = %q, want point-1", got[0].ID)
	}
	if got[0].SubjectRef == nil || got[0].SubjectRef.Type != "k8s-pvc" {
		t.Fatalf("subjectRef = %#v, want k8s-pvc", got[0].SubjectRef)
	}
}
