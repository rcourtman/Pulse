package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestStore_ListSeriesAndFacets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	t1 := time.Date(2026, 2, 13, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC)

	size := int64(1234)
	verified := true

	points := []recovery.RecoveryPoint{
		{
			ID:        "p1",
			Provider:  recovery.ProviderKubernetes,
			Kind:      recovery.KindSnapshot,
			Mode:      recovery.ModeSnapshot,
			Outcome:   recovery.OutcomeSuccess,
			StartedAt: &t1, CompletedAt: &t1,
			SubjectRef: &recovery.ExternalRef{
				Type:      "k8s-pvc",
				Namespace: "default",
				Name:      "postgres-pvc",
				UID:       "pvc-uid-1",
			},
			Details: map[string]any{
				"k8sClusterName": "dev-cluster",
				"snapshotName":   "snap-1",
			},
			SizeBytes: &size,
			Verified:  &verified,
		},
		{
			ID:        "p2",
			Provider:  recovery.ProviderTrueNAS,
			Kind:      recovery.KindBackup,
			Mode:      recovery.ModeRemote,
			Outcome:   recovery.OutcomeFailed,
			StartedAt: &t2, CompletedAt: &t2,
			SubjectRef: &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: "tank/apps/postgres",
				ID:   "tank/apps/postgres",
			},
			RepositoryRef: &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: "backup/postgres",
				ID:   "backup/postgres",
			},
			Details: map[string]any{
				"hostname":     "truenas-1",
				"taskName":     "replicate-postgres",
				"lastSnapshot": "auto-2026-02-14",
			},
		},
	}

	if err := store.UpsertPoints(context.Background(), points); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	from := time.Date(2026, 2, 13, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 14, 23, 59, 59, 0, time.UTC)
	opts := recovery.ListPointsOptions{From: &from, To: &to}

	series, err := store.ListPointsSeries(context.Background(), opts, 0)
	if err != nil {
		t.Fatalf("ListPointsSeries() error = %v", err)
	}
	if len(series) != 2 {
		t.Fatalf("len(series) = %d, want 2", len(series))
	}
	if series[0].Day != "2026-02-13" || series[0].Total != 1 || series[0].Snapshot != 1 {
		t.Fatalf("series[0] = %#v, want 2026-02-13 total=1 snapshot=1", series[0])
	}
	if series[1].Day != "2026-02-14" || series[1].Total != 1 || series[1].Remote != 1 {
		t.Fatalf("series[1] = %#v, want 2026-02-14 total=1 remote=1", series[1])
	}

	facets, err := store.ListPointsFacets(context.Background(), opts)
	if err != nil {
		t.Fatalf("ListPointsFacets() error = %v", err)
	}
	if !facets.HasSize {
		t.Fatalf("facets.HasSize = false, want true")
	}
	if !facets.HasVerification {
		t.Fatalf("facets.HasVerification = false, want true")
	}
	if len(facets.Clusters) == 0 || facets.Clusters[0] != "dev-cluster" {
		t.Fatalf("facets.Clusters = %#v, want dev-cluster", facets.Clusters)
	}
	if len(facets.Namespaces) == 0 || facets.Namespaces[0] != "default" {
		t.Fatalf("facets.Namespaces = %#v, want default", facets.Namespaces)
	}
	if len(facets.NodesHosts) == 0 || facets.NodesHosts[0] != "truenas-1" {
		t.Fatalf("facets.NodesHosts = %#v, want truenas-1", facets.NodesHosts)
	}
	if len(facets.ItemTypes) != 2 || facets.ItemTypes[0] != "dataset" || facets.ItemTypes[1] != "pvc" {
		t.Fatalf("facets.ItemTypes = %#v, want [dataset pvc]", facets.ItemTypes)
	}

	pvcOnly, err := store.ListPointsSeries(context.Background(), recovery.ListPointsOptions{
		From:     &from,
		To:       &to,
		ItemType: "pvc",
	}, 0)
	if err != nil {
		t.Fatalf("ListPointsSeries(itemType) error = %v", err)
	}
	if len(pvcOnly) != 2 || pvcOnly[0].Total != 1 || pvcOnly[1].Total != 0 {
		t.Fatalf("ListPointsSeries(itemType) = %#v, want only pvc activity on first day", pvcOnly)
	}

	// The series is dense, so the widest window a caller can name must still
	// be a bounded number of buckets: its newest MaxPointsSeriesDays days.
	widestFrom := time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC)
	widestTo := time.Date(9999, 12, 30, 0, 0, 0, 0, time.UTC)
	for _, window := range [][2]*time.Time{{&widestFrom, &widestTo}, {&widestTo, &widestFrom}} {
		widest, err := store.ListPointsSeries(context.Background(), recovery.ListPointsOptions{From: window[0], To: window[1]}, 0)
		if err != nil {
			t.Fatalf("ListPointsSeries(widest) error = %v", err)
		}
		if len(widest) != recovery.MaxPointsSeriesDays {
			t.Fatalf("len(widest series) = %d, want %d", len(widest), recovery.MaxPointsSeriesDays)
		}
		if last := widest[len(widest)-1].Day; last != "9999-12-30" {
			t.Fatalf("widest series ends on %q, want 9999-12-30", last)
		}
	}

	// A window wider than the bound keeps its newest days and their counts.
	wideFrom := to.AddDate(-20, 0, 0)
	wide, err := store.ListPointsSeries(context.Background(), recovery.ListPointsOptions{From: &wideFrom, To: &to}, 0)
	if err != nil {
		t.Fatalf("ListPointsSeries(wide) error = %v", err)
	}
	if len(wide) != recovery.MaxPointsSeriesDays {
		t.Fatalf("len(wide series) = %d, want %d", len(wide), recovery.MaxPointsSeriesDays)
	}
	if prev, last := wide[len(wide)-2], wide[len(wide)-1]; prev.Day != "2026-02-13" || prev.Total != 1 || last.Day != "2026-02-14" || last.Total != 1 {
		t.Fatalf("wide series tail = %#v, %#v, want both seeded days", prev, last)
	}
}
