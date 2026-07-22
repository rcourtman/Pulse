package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// branchcov0722PMRecord builds a minimal alerts.Alert whose
// attentionAlertNewer comparison only depends on the two timestamps it reads.
func branchcov0722PMRecord(stateChanged, lastObserved time.Time) alerts.Alert {
	return alerts.Alert{
		OperationalRecord: &operationaltrust.OperationalRecord{
			ID:             "rec",
			StateChangedAt: stateChanged,
			LastObservedAt: lastObserved,
		},
	}
}

func TestBranchcov0722PMAttentionAlertNewer(t *testing.T) {
	base := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	later := base.Add(time.Hour)

	cases := []struct {
		name      string
		candidate alerts.Alert
		existing  alerts.Alert
		want      bool
	}{
		{
			name:      "candidate nil operational record returns false",
			candidate: alerts.Alert{},
			existing:  branchcov0722PMRecord(base, base),
			want:      false,
		},
		{
			name:      "existing nil operational record returns true",
			candidate: branchcov0722PMRecord(base, base),
			existing:  alerts.Alert{},
			want:      true,
		},
		{
			name:      "candidate state changed newer wins regardless of last observed",
			candidate: branchcov0722PMRecord(later, base),
			existing:  branchcov0722PMRecord(base, later),
			want:      true,
		},
		{
			name:      "existing state changed newer wins regardless of last observed",
			candidate: branchcov0722PMRecord(base, later),
			existing:  branchcov0722PMRecord(later, base),
			want:      false,
		},
		{
			name:      "state changed tie falls back to candidate last observed newer",
			candidate: branchcov0722PMRecord(base, later),
			existing:  branchcov0722PMRecord(base, base),
			want:      true,
		},
		{
			name:      "state changed tie falls back to existing last observed newer",
			candidate: branchcov0722PMRecord(base, base),
			existing:  branchcov0722PMRecord(base, later),
			want:      false,
		},
		{
			name:      "exact tie on both timestamps keeps existing",
			candidate: branchcov0722PMRecord(base, base),
			existing:  branchcov0722PMRecord(base, base),
			want:      false,
		},
		{
			name:      "both zero time keeps existing",
			candidate: branchcov0722PMRecord(time.Time{}, time.Time{}),
			existing:  branchcov0722PMRecord(time.Time{}, time.Time{}),
			want:      false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := attentionAlertNewer(tc.candidate, tc.existing)
			if got != tc.want {
				t.Fatalf("attentionAlertNewer = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMCountPatrolRunFact(t *testing.T) {
	cases := []struct {
		name  string
		value int
		label string
		want  string
	}{
		{name: "zero omits fact", value: 0, label: "nodes", want: ""},
		{name: "negative omits fact", value: -3, label: "nodes", want: ""},
		{name: "single emits one label verbatim", value: 1, label: "node", want: "1 node"},
		{name: "many emits count and label verbatim", value: 5, label: "nodes", want: "5 nodes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countPatrolRunFact(tc.value, tc.label)
			if got != tc.want {
				t.Fatalf("countPatrolRunFact(%d, %q) = %q, want %q",
					tc.value, tc.label, got, tc.want)
			}
		})
	}
}

func TestBranchcov0722PMPatrolLookupResourceMetrics(t *testing.T) {
	t.Run("node present returns cpu and memory metrics", func(t *testing.T) {
		snap := patrolRuntimeState{
			Nodes: []models.Node{
				{
					ID:     "node-1",
					Name:   "node-one",
					CPU:    0.42,
					Memory: models.Memory{Usage: 80.0},
				},
			},
		}
		metrics, ok := patrolLookupResourceMetrics(snap, "node-1")
		if !ok {
			t.Fatal("patrolLookupResourceMetrics(node-1): expected ok=true, got false")
		}
		if metrics["cpu"] != 42.0 {
			t.Fatalf("cpu metric = %v, want 42.0", metrics["cpu"])
		}
		if metrics["memory"] != 80.0 {
			t.Fatalf("memory metric = %v, want 80.0", metrics["memory"])
		}
		if _, leaked := metrics["disk"]; leaked {
			t.Fatalf("node metrics unexpectedly included disk: %v", metrics)
		}
	})

	t.Run("storage present returns usage metric", func(t *testing.T) {
		snap := patrolRuntimeState{
			Storage: []models.Storage{
				{ID: "storage-1", Name: "pool-one", Usage: 55.5},
			},
		}
		metrics, ok := patrolLookupResourceMetrics(snap, "storage-1")
		if !ok {
			t.Fatal("patrolLookupResourceMetrics(storage-1): expected ok=true, got false")
		}
		if len(metrics) != 1 {
			t.Fatalf("expected single usage key, got %v", metrics)
		}
		if metrics["usage"] != 55.5 {
			t.Fatalf("usage metric = %v, want 55.5", metrics["usage"])
		}
	})

	t.Run("resource absent returns nil and false", func(t *testing.T) {
		snap := patrolRuntimeState{
			Nodes: []models.Node{{ID: "node-1", Name: "node-one"}},
		}
		metrics, ok := patrolLookupResourceMetrics(snap, "missing")
		if ok {
			t.Fatal("expected ok=false for absent resource, got true")
		}
		if metrics != nil {
			t.Fatalf("expected nil metrics for absent resource, got %v", metrics)
		}
	})

	t.Run("empty snapshot returns nil and false", func(t *testing.T) {
		snap := patrolRuntimeState{}
		metrics, ok := patrolLookupResourceMetrics(snap, "anything")
		if ok {
			t.Fatal("expected ok=false for empty snapshot, got true")
		}
		if metrics != nil {
			t.Fatalf("expected nil metrics for empty snapshot, got %v", metrics)
		}
	})

	t.Run("present node with all zero metrics still found", func(t *testing.T) {
		// The node visitor always emits {"cpu": node.cpu} and only adds
		// "memory" when mem > 0. A zero-CPU, zero-memory node is still
		// "present" (found=true) but returns a minimal {"cpu": 0} map.
		// A truly empty metrics map is not reachable through this visitor
		// for an in-package snapshot literal.
		snap := patrolRuntimeState{
			Nodes: []models.Node{{ID: "node-zero", Name: "zero"}},
		}
		metrics, ok := patrolLookupResourceMetrics(snap, "node-zero")
		if !ok {
			t.Fatal("expected ok=true for present node with zero metrics")
		}
		if len(metrics) != 1 {
			t.Fatalf("expected exactly one key {cpu:0}, got %v", metrics)
		}
		if metrics["cpu"] != 0 {
			t.Fatalf("cpu metric = %v, want 0", metrics["cpu"])
		}
		if _, hasMem := metrics["memory"]; hasMem {
			t.Fatalf("did not expect memory key when mem==0, got %v", metrics)
		}
	})

	t.Run("identifier matched by name alias", func(t *testing.T) {
		snap := patrolRuntimeState{
			Nodes: []models.Node{
				{ID: "node-1", Name: "node-one", CPU: 0.10},
			},
		}
		metrics, ok := patrolLookupResourceMetrics(snap, "node-one")
		if !ok {
			t.Fatal("expected ok=true when matching by name alias")
		}
		if metrics["cpu"] != 10.0 {
			t.Fatalf("cpu metric = %v, want 10.0", metrics["cpu"])
		}
	})

	t.Run("mutating returned map does not corrupt snapshot", func(t *testing.T) {
		snap := patrolRuntimeState{
			Nodes: []models.Node{
				{
					ID:     "node-1",
					Name:   "node-one",
					CPU:    0.42,
					Memory: models.Memory{Usage: 80.0},
				},
			},
		}
		first, ok := patrolLookupResourceMetrics(snap, "node-1")
		if !ok {
			t.Fatal("expected ok=true on first lookup")
		}
		first["cpu"] = 999.0
		delete(first, "memory")
		first["bogus"] = -1

		second, ok := patrolLookupResourceMetrics(snap, "node-1")
		if !ok {
			t.Fatal("expected ok=true on second lookup")
		}
		if second["cpu"] != 42.0 {
			t.Fatalf("snapshot corrupted by map mutation: cpu = %v, want 42.0", second["cpu"])
		}
		if second["memory"] != 80.0 {
			t.Fatalf("snapshot corrupted by map mutation: memory = %v, want 80.0", second["memory"])
		}
		if _, leaked := second["bogus"]; leaked {
			t.Fatalf("snapshot corrupted: bogus key leaked into fresh lookup: %v", second)
		}
	})
}
