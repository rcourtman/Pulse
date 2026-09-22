package unifiedresources

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise the producer side of PBS History (#1723): monitor generation
// replacement, uncached/cached target lookup, and API-style unified reseeding.
// Display labels, datastore order and source freshness are not metric IDs.
func TestPBSTargetContinuityAcrossSnapshotGenerations(t *testing.T) {
	for _, shape := range []string{"pbs-only", "pbs-agent", "side-by-side"} {
		t.Run(shape, func(t *testing.T) {
			adapter := NewMonitorAdapter(NewRegistry(nil))
			var baseline map[string]MetricsTarget
			for generation := 0; generation < 8; generation++ {
				snapshot := pbsTargetSnapshot(shape)
				if generation%2 != 0 {
					ds := snapshot.PBSInstances[0].Datastores
					ds[0], ds[1] = ds[1], ds[0]
					if len(snapshot.Hosts) > 0 {
						snapshot.Hosts[0].DisplayName = "Backup host label"
					}
				}
				if generation%3 == 2 {
					snapshot.PBSInstances[0].Status = "offline"
					snapshot.PBSInstances[0].LastSeen = time.Now().Add(-time.Hour)
					for i := range snapshot.Hosts {
						snapshot.Hosts[i].LastSeen = time.Now().Add(-time.Hour)
					}
				}
				adapter.replaceRegistry(snapshot, nil)
				registry := adapter.currentRegistry()
				targets := pbsTargetCoordinates(t, registry)
				if baseline == nil {
					baseline = targets
				} else if !reflect.DeepEqual(targets, baseline) {
					t.Fatalf("generation %d changed IDs/targets: got %v, want %v", generation, targets, baseline)
				}
				// Typed views build the inverse source index used by subsequent
				// metrics-target lookups. It must agree with the uncached scan.
				registry.Hosts()
				if got := pbsTargetCoordinates(t, registry); !reflect.DeepEqual(got, targets) {
					t.Fatalf("cached targets differ: got %v, want %v", got, targets)
				}
				rehydrated := NewRegistry(nil)
				rehydrated.IngestResources(registry.List())
				if got := pbsTargetCoordinates(t, rehydrated); !reflect.DeepEqual(got, targets) {
					t.Fatalf("reseeded targets differ: got %v, want %v", got, targets)
				}
			}
			t.Logf("8 generations retain %d canonical IDs and exact targets", len(baseline))
		})
	}
}

// A missing or replaced agent must not keep advertising its old target. The
// independently observed PBS service retains its own history coordinates.
func TestPBSTargetContinuityAgentRemovalAndReplacement(t *testing.T) {
	adapter := NewMonitorAdapter(NewRegistry(nil))
	for _, agentID := range []string{"agent-uuid-a", "", "agent-uuid-b", "agent-uuid-a"} {
		snapshot := pbsTargetSnapshot("pbs-agent")
		if agentID == "" {
			snapshot.Hosts = nil
		} else {
			snapshot.Hosts[0].ID = agentID
		}
		adapter.replaceRegistry(snapshot, nil)
		registry := adapter.currentRegistry()
		pbsTargetCoordinates(t, registry)
		var agents int
		for _, resource := range registry.List() {
			if resource.Type == ResourceTypeAgent {
				agents++
				target := registry.MetricsTarget(resource.ID)
				if target == nil || target.ResourceID != agentID {
					t.Fatalf("agent %q advertises stale/missing target: %+v", agentID, target)
				}
			}
		}
		want := 1
		if agentID == "" {
			want = 0
		}
		if agents != want {
			t.Fatalf("agent %q: got %d host resources, want %d", agentID, agents, want)
		}
	}
}

func pbsTargetSnapshot(shape string) models.StateSnapshot {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{{
			ID: "pbs-backup", Name: "backup", Host: "https://backup.example.invalid:8007",
			Status: "online", LastSeen: now,
			Datastores: []models.PBSDatastore{{Name: "fast", Total: 1000, Used: 200}, {Name: "archive", Total: 2000, Used: 300}},
		}},
	}
	if shape != "pbs-only" {
		snapshot.Hosts = []models.Host{{ID: "agent-uuid-a", Hostname: "backup", MachineID: "machine-a", LastSeen: now}}
	}
	if shape == "side-by-side" {
		snapshot.Nodes = []models.Node{{ID: "pve/backup", Name: "backup", Instance: "pve", LinkedAgentID: "agent-uuid-a", LastSeen: now}}
		snapshot.Hosts[0].LinkedNodeID = "pve/backup"
	}
	return snapshot
}

func pbsTargetCoordinates(t *testing.T, registry *ResourceRegistry) map[string]MetricsTarget {
	t.Helper()
	targets := make(map[string]MetricsTarget)
	var services, datastores int
	for _, resource := range registry.List() {
		target := registry.MetricsTarget(resource.ID)
		if target == nil {
			t.Fatalf("resource %s (%s) lost its metrics target", resource.ID, resource.Type)
		}
		targets[resource.ID] = *target
		switch resource.Type {
		case ResourceTypePBS:
			services++
			if *target != (MetricsTarget{ResourceType: "agent", ResourceID: "pbs-backup"}) {
				t.Fatalf("PBS target = %+v", target)
			}
		case ResourceTypeStorage:
			datastores++
			want := MetricsTarget{ResourceType: "storage", ResourceID: fmt.Sprintf("pbs-backup/%s", resource.Name)}
			if *target != want {
				t.Fatalf("datastore target = %+v, want %+v", target, want)
			}
		case ResourceTypeAgent:
			if resource.Agent == nil || *target != (MetricsTarget{ResourceType: "agent", ResourceID: resource.Agent.AgentID}) {
				t.Fatalf("host target disagrees with agent identity: %+v", target)
			}
		}
	}
	if services != 1 || datastores != 2 {
		t.Fatalf("got %d PBS services, %d datastores", services, datastores)
	}
	return targets
}
