package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// swarmTwoManagerSnapshot builds a snapshot where two swarm managers report
// the same cluster-scoped objects (service, nodes, secret, config). The
// managers' service views differ slightly (manager-2 lags one running task),
// so an alternating dedupe winner would flip the service status between
// online and warning in addition to re-parenting the swarm node resources.
func swarmTwoManagerSnapshot(m1LastSeen, m2LastSeen time.Time, flipOrder bool) models.StateSnapshot {
	swarmInfo := func(nodeID string) *models.DockerSwarmInfo {
		return &models.DockerSwarmInfo{
			NodeID:           nodeID,
			NodeRole:         "manager",
			LocalState:       "active",
			ControlAvailable: true,
			ClusterID:        "swarm-cluster-1",
			ClusterName:      "prod-swarm",
		}
	}
	nodes := []models.DockerNode{
		{ID: "node-m1", Hostname: "manager-1", Role: "manager", State: "ready", ManagerReachability: "reachable", EngineVersion: "27.0.1"},
		{ID: "node-m2", Hostname: "manager-2", Role: "manager", State: "ready", ManagerReachability: "reachable", EngineVersion: "27.0.1"},
	}
	secrets := []models.DockerSecret{{ID: "secret-1", Name: "db-password"}}
	configs := []models.DockerConfig{{ID: "config-1", Name: "app-config"}}

	serviceSeenByM1 := models.DockerService{
		ID:           "svc-1",
		Name:         "web",
		Image:        "nginx:1.27",
		Mode:         "replicated",
		DesiredTasks: 3,
		RunningTasks: 3,
	}
	serviceSeenByM2 := serviceSeenByM1
	serviceSeenByM2.RunningTasks = 2

	m1 := models.DockerHost{
		ID:       "dockerhost-m1",
		AgentID:  "agent-m1",
		Hostname: "manager-1",
		Status:   "online",
		LastSeen: m1LastSeen,
		Swarm:    swarmInfo("node-m1"),
		Services: []models.DockerService{serviceSeenByM1},
		Nodes:    nodes,
		Secrets:  secrets,
		Configs:  configs,
	}
	m2 := models.DockerHost{
		ID:       "dockerhost-m2",
		AgentID:  "agent-m2",
		Hostname: "manager-2",
		Status:   "online",
		LastSeen: m2LastSeen,
		Swarm:    swarmInfo("node-m2"),
		Services: []models.DockerService{serviceSeenByM2},
		Nodes:    nodes,
		Secrets:  secrets,
		Configs:  configs,
	}

	hosts := []models.DockerHost{m1, m2}
	if flipOrder {
		hosts = []models.DockerHost{m2, m1}
	}
	return models.StateSnapshot{DockerHosts: hosts}
}

// Every state update rebuilds the registry from scratch, and in a
// multi-manager swarm the reporting hosts' LastSeen ordering flips between
// polls. The cluster-scoped dedupe winner must not track that jitter: an
// alternating winner re-parents the swarm node resources and flips the
// service status with the managers' slightly divergent views, writing
// phantom resource_changes rows on every poll.
func TestSwarmClusterScopedDedupe_NoChangeEmissionAcrossLastSeenJitter(t *testing.T) {
	base := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)

	build := func(m1LastSeen, m2LastSeen time.Time, flipOrder bool) []Resource {
		rr := NewRegistry(nil)
		rr.IngestSnapshot(swarmTwoManagerSnapshot(m1LastSeen, m2LastSeen, flipOrder))
		return rr.List()
	}

	previous := build(base, base.Add(2*time.Second), false)
	if len(previous) == 0 {
		t.Fatal("expected resources from swarm snapshot, got none")
	}

	for poll := 1; poll <= 6; poll++ {
		offset := time.Duration(poll) * 10 * time.Second
		m1Seen := base.Add(offset)
		m2Seen := base.Add(offset + 2*time.Second)
		flip := poll%2 == 1
		if flip {
			// Alternate which manager reported most recently, well inside
			// the docker stale threshold, and flip snapshot host order too.
			m1Seen, m2Seen = m2Seen, m1Seen
		}
		current := build(m1Seen, m2Seen, flip)

		beforeByID := make(map[string]Resource, len(previous))
		for _, resource := range previous {
			beforeByID[resource.ID] = resource
		}
		afterByID := make(map[string]Resource, len(current))
		for _, resource := range current {
			afterByID[resource.ID] = resource
		}
		union := make(map[string]struct{}, len(beforeByID)+len(afterByID))
		for id := range beforeByID {
			union[id] = struct{}{}
		}
		for id := range afterByID {
			union[id] = struct{}{}
		}

		for id := range union {
			before, beforeOK := beforeByID[id]
			after, afterOK := afterByID[id]
			change := buildResourceChange(before, beforeOK, after, afterOK, base.Add(offset), nil, SourcePulseDiff, "")
			if change != nil {
				t.Fatalf("poll %d: unexpected %s change for %s: from=%q to=%q reason=%q",
					poll, change.Kind, id, change.From, change.To, change.Reason)
			}
		}
		previous = current
	}
}

func TestPreferDockerSwarmHost(t *testing.T) {
	base := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	threshold := 120 * time.Second

	host := func(id string, lastSeen time.Time, controlAvailable bool) models.DockerHost {
		return models.DockerHost{
			ID:       id,
			LastSeen: lastSeen,
			Swarm:    &models.DockerSwarmInfo{ClusterID: "swarm-cluster-1", ControlAvailable: controlAvailable},
		}
	}

	manager := host("host-b", base, true)
	fresherWorker := host("host-a", base.Add(60*time.Second), false)
	if !preferDockerSwarmHost(manager, fresherWorker, threshold) {
		t.Error("control-available manager should beat a fresher worker")
	}
	if preferDockerSwarmHost(fresherWorker, manager, threshold) {
		t.Error("fresher worker should not beat a control-available manager")
	}

	// Freshness jitter inside the stale threshold must not decide: the
	// lowest host ID wins from either direction.
	a := host("host-a", base, true)
	bFresher := host("host-b", base.Add(30*time.Second), true)
	if !preferDockerSwarmHost(a, bFresher, threshold) {
		t.Error("lowest host ID should win when freshness jitter is inside the stale threshold")
	}
	if preferDockerSwarmHost(bFresher, a, threshold) {
		t.Error("higher host ID should lose when freshness jitter is inside the stale threshold")
	}

	// Once the gap exceeds the stale threshold the fresher host wins even
	// with the higher ID: the other manager has genuinely gone quiet.
	bWellFresher := host("host-b", base.Add(threshold+time.Second), true)
	if !preferDockerSwarmHost(bWellFresher, a, threshold) {
		t.Error("host beyond the stale threshold should lose to the fresher host")
	}
	if preferDockerSwarmHost(a, bWellFresher, threshold) {
		t.Error("stale host should not beat a host fresher by more than the stale threshold")
	}
}
