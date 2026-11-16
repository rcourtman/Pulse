package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// Every store query that takes ListPointsOptions honours Repositories, so a
// caller never gets the whole inventory back from a filter it believed applied.
func TestStoreQueriesFilterByRepository(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	at := time.Now().UTC().Add(-time.Hour)
	point := func(id, subject, node string) recovery.RecoveryPoint {
		return recovery.RecoveryPoint{ID: id, Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindBackup,
			Mode: recovery.ModeLocal, Outcome: recovery.OutcomeSuccess, StartedAt: &at, CompletedAt: &at,
			SubjectResourceID: subject,
			RepositoryRef:     &recovery.ExternalRef{Type: "proxmox-storage", Namespace: "pve1", Name: "local", Class: node}}
	}
	if err := s.UpsertPoints(ctx, []recovery.RecoveryPoint{
		point("a", "guest-a", "node1"), point("b", "guest-b", "node1"), point("c", "guest-c", "node2"),
	}); err != nil {
		t.Fatal(err)
	}
	node1 := recovery.ListPointsOptions{Repositories: []recovery.RepositoryFilter{
		{Type: "proxmox-storage", Namespace: "pve1", Name: "local", Class: "node1"},
	}}

	points, total, err := s.ListPoints(ctx, node1)
	if err != nil || total != 2 || len(points) != 2 {
		t.Fatalf("ListPoints = %d points, total %d, err %v; want 2", len(points), total, err)
	}
	rollups, total, err := s.ListRollups(ctx, node1)
	if err != nil || total != 2 || len(rollups) != 2 {
		t.Fatalf("ListRollups = %d rollups, total %d, err %v; want 2", len(rollups), total, err)
	}
	series, err := s.ListPointsSeries(ctx, node1, 0)
	if err != nil {
		t.Fatal(err)
	}
	var counted int
	for _, bucket := range series {
		counted += bucket.Total
	}
	if counted != 2 {
		t.Fatalf("ListPointsSeries counted %d points, want 2", counted)
	}
	if _, err := s.ListPointsFacets(ctx, node1); err != nil {
		t.Fatal(err)
	}

	points, total, err = s.ListPoints(ctx, recovery.ListPointsOptions{Repositories: []recovery.RepositoryFilter{{Name: "local"}}})
	if err != nil || total != 0 || len(points) != 0 {
		t.Fatalf("a filter without a type listed %d points (total %d, err %v); want none", len(points), total, err)
	}
}
