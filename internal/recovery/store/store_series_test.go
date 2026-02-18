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
}
