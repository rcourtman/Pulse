package licensing

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func TestConfiguredNodeCount(t *testing.T) {
	got := ConfiguredNodeCount(2, 3, 4)
	if got != 9 {
		t.Fatalf("expected 9, got %d", got)
	}
}

func TestExceedsNodeLimit(t *testing.T) {
	tests := []struct {
		name      string
		current   int
		additions int
		limit     int
		want      bool
	}{
		{name: "unlimited", current: 10, additions: 1, limit: 0, want: false},
		{name: "no additions", current: 5, additions: 0, limit: 5, want: false},
		{name: "within limit", current: 4, additions: 1, limit: 5, want: false},
		{name: "at limit", current: 5, additions: 1, limit: 5, want: true},
		{name: "over limit", current: 6, additions: 1, limit: 5, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExceedsNodeLimit(tt.current, tt.additions, tt.limit); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

func TestNodeLimitExceededMessage(t *testing.T) {
	got := NodeLimitExceededMessage(6, 5)
	want := "Node limit reached (6/5). Remove a node or upgrade your license."
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRegisteredNodeSlotCount(t *testing.T) {
	snapshot := models.StateSnapshot{
		Hosts:              []models.Host{{ID: "h1"}},
		DockerHosts:        []models.DockerHost{{ID: "d1"}, {ID: "d2"}},
		KubernetesClusters: []models.KubernetesCluster{{ID: "k1"}},
	}
	got := RegisteredNodeSlotCount(5, snapshot)
	if got != 9 {
		t.Fatalf("expected 9, got %d", got)
	}
}

func TestCollectNonEmptyStrings(t *testing.T) {
	got := CollectNonEmptyStrings(" alpha ", "", "alpha", "beta", "beta ", " gamma ")
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("expected %d values, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %q at index %d, got %q", want[i], i, got[i])
		}
	}
}

func TestHostReportTargetsExistingHost(t *testing.T) {
	t.Run("matches_by_ids", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{ID: "host-1", MachineID: "machine-1"}},
		}
		report := agentshost.Report{
			Agent: agentshost.AgentInfo{ID: "agent-1"},
			Host:  agentshost.HostInfo{ID: "host-1", MachineID: "machine-x", Hostname: "srv-1"},
		}
		if !HostReportTargetsExistingHost(snapshot, report, "") {
			t.Fatalf("expected match by host ID")
		}
	})

	t.Run("hostname_requires_matching_token_when_token_provided", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{Hostname: "srv-1", TokenID: "token-a"}},
		}
		report := agentshost.Report{
			Host: agentshost.HostInfo{Hostname: "srv-1"},
		}
		if HostReportTargetsExistingHost(snapshot, report, "token-b") {
			t.Fatalf("expected no match for hostname with different token")
		}
		if !HostReportTargetsExistingHost(snapshot, report, "token-a") {
			t.Fatalf("expected match for hostname with same token")
		}
	})
}

func TestDockerReportTargetsExistingHost(t *testing.T) {
	t.Run("matches_by_agent_key", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			DockerHosts: []models.DockerHost{{AgentID: "agent-1"}},
		}
		report := agentsdocker.Report{
			Agent: agentsdocker.AgentInfo{ID: "agent-1"},
			Host:  agentsdocker.HostInfo{Hostname: "docker-1"},
		}
		if !DockerReportTargetsExistingHost(snapshot, report, "") {
			t.Fatalf("expected match by agent key")
		}
	})

	t.Run("hostname_requires_matching_token_when_token_provided", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			DockerHosts: []models.DockerHost{{Hostname: "docker-1", TokenID: "token-a"}},
		}
		report := agentsdocker.Report{
			Host: agentsdocker.HostInfo{Hostname: "docker-1"},
		}
		if DockerReportTargetsExistingHost(snapshot, report, "token-b") {
			t.Fatalf("expected no match for hostname with different token")
		}
		if !DockerReportTargetsExistingHost(snapshot, report, "token-a") {
			t.Fatalf("expected match for hostname with same token")
		}
	})
}

func TestKubernetesReportIdentifier(t *testing.T) {
	t.Run("prefers_cluster_id", func(t *testing.T) {
		report := agentsk8s.Report{
			Cluster: agentsk8s.ClusterInfo{ID: "cluster-1"},
			Agent:   agentsk8s.AgentInfo{ID: "agent-1"},
		}
		if got := KubernetesReportIdentifier(report); got != "cluster-1" {
			t.Fatalf("expected cluster ID, got %q", got)
		}
	})

	t.Run("falls_back_to_agent_id", func(t *testing.T) {
		report := agentsk8s.Report{
			Cluster: agentsk8s.ClusterInfo{},
			Agent:   agentsk8s.AgentInfo{ID: "agent-1"},
		}
		if got := KubernetesReportIdentifier(report); got != "agent-1" {
			t.Fatalf("expected agent ID, got %q", got)
		}
	})

	t.Run("hashes_stable_fields", func(t *testing.T) {
		report := agentsk8s.Report{
			Cluster: agentsk8s.ClusterInfo{
				Server:  "https://k8s.example",
				Context: "ctx",
				Name:    "prod",
			},
		}
		got := KubernetesReportIdentifier(report)
		if got == "" {
			t.Fatalf("expected hashed identifier")
		}
		if len(got) != 64 {
			t.Fatalf("expected sha256 hex length 64, got %d", len(got))
		}
	})
}

func TestKubernetesReportTargetsExistingCluster(t *testing.T) {
	t.Run("matches_by_identifier", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{{ID: "cluster-1"}},
		}
		report := agentsk8s.Report{
			Cluster: agentsk8s.ClusterInfo{ID: "cluster-1"},
		}
		if !KubernetesReportTargetsExistingCluster(snapshot, report, "") {
			t.Fatalf("expected match by cluster identifier")
		}
	})

	t.Run("matches_by_agent_id", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{{AgentID: "agent-1"}},
		}
		report := agentsk8s.Report{
			Agent: agentsk8s.AgentInfo{ID: "agent-1"},
		}
		if !KubernetesReportTargetsExistingCluster(snapshot, report, "") {
			t.Fatalf("expected match by agent ID")
		}
	})

	t.Run("matches_token_and_name_when_identifier_missing", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{{
				Name:    "prod",
				TokenID: "token-a",
			}},
		}
		report := agentsk8s.Report{
			Cluster: agentsk8s.ClusterInfo{Name: "prod"},
		}
		if !KubernetesReportTargetsExistingCluster(snapshot, report, "token-a") {
			t.Fatalf("expected match by token and cluster name")
		}
		if KubernetesReportTargetsExistingCluster(snapshot, report, "token-b") {
			t.Fatalf("expected no match with different token")
		}
	})
}
