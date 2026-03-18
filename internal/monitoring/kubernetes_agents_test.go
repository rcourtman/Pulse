package monitoring

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func newKubernetesTestMonitor() *Monitor {
	return &Monitor{
		state:                     models.NewState(),
		config:                    &config.Config{},
		removedKubernetesClusters: make(map[string]time.Time),
		kubernetesTokenBindings:   make(map[string]string),
	}
}

func TestNormalizeKubernetesClusterIdentifier(t *testing.T) {
	report := agentsk8s.Report{
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1"},
		Agent:   agentsk8s.AgentInfo{ID: "agent-1"},
	}
	if got := normalizeKubernetesClusterIdentifier(report); got != "cluster-1" {
		t.Fatalf("unexpected identifier: %s", got)
	}

	report.Cluster.ID = ""
	if got := normalizeKubernetesClusterIdentifier(report); got != "agent-1" {
		t.Fatalf("unexpected identifier: %s", got)
	}

	report.Agent.ID = ""
	report.Cluster.Server = "https://server"
	report.Cluster.Context = "ctx"
	report.Cluster.Name = "name"
	stableKey := "https://server|ctx|name"
	sum := sha256.Sum256([]byte(stableKey))
	expected := hex.EncodeToString(sum[:])
	if got := normalizeKubernetesClusterIdentifier(report); got != expected {
		t.Fatalf("unexpected hashed identifier: %s", got)
	}

	report.Cluster.Server = ""
	report.Cluster.Context = ""
	report.Cluster.Name = ""
	if got := normalizeKubernetesClusterIdentifier(report); got != "" {
		t.Fatalf("expected empty identifier, got %s", got)
	}
}

func TestApplyKubernetesReport(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "agent-1", IntervalSeconds: 10},
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1", Name: "cluster"},
	}

	cluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster.ID != "cluster-1" || cluster.DisplayName != "cluster" {
		t.Fatalf("unexpected cluster: %+v", cluster)
	}
	if !monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"] {
		t.Fatal("expected connection health to be true")
	}

	monitor.removedKubernetesClusters["cluster-2"] = time.Now()
	report.Cluster.ID = "cluster-2"
	if _, err := monitor.ApplyKubernetesReport(report, nil); err == nil {
		t.Fatal("expected error for removed cluster")
	}

	token := &config.APITokenRecord{ID: "token-1", Name: "Token"}
	monitor.kubernetesTokenBindings["token-1"] = "other-agent"
	report.Cluster.ID = "cluster-3"
	report.Agent.ID = "agent-1"
	if _, err := monitor.ApplyKubernetesReport(report, token); err == nil {
		t.Fatal("expected error for token bound to different agent")
	}
}

func TestApplyKubernetesReport_PodNetworkAndEphemeralMetrics(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.rateTracker = NewRateTracker()
	monitor.metricsHistory = NewMetricsHistory(10, time.Hour)

	baseTime := time.Now().Add(-20 * time.Second).UTC()
	report := agentsk8s.Report{
		Agent:   agentsk8s.AgentInfo{ID: "agent-1", IntervalSeconds: 10},
		Cluster: agentsk8s.ClusterInfo{ID: "cluster-1", Name: "cluster"},
		Nodes: []agentsk8s.Node{
			{
				Name: "node-a",
				Capacity: agentsk8s.NodeResources{
					CPUCores:    4,
					MemoryBytes: 16 * 1024 * 1024 * 1024,
				},
				Allocatable: agentsk8s.NodeResources{
					CPUCores:    4,
					MemoryBytes: 16 * 1024 * 1024 * 1024,
				},
			},
		},
		Pods: []agentsk8s.Pod{
			{
				UID:       "pod-1",
				Name:      "api-0",
				Namespace: "default",
				NodeName:  "node-a",
				Phase:     "Running",
				Usage: &agentsk8s.PodUsage{
					CPUMilliCores:                 400,
					MemoryBytes:                   1024 * 1024 * 1024,
					NetworkRxBytes:                1000,
					NetworkTxBytes:                2000,
					EphemeralStorageUsedBytes:     500 * 1024 * 1024,
					EphemeralStorageCapacityBytes: 2 * 1024 * 1024 * 1024,
				},
			},
		},
		Timestamp: baseTime,
	}

	firstCluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport(first): %v", err)
	}
	if len(firstCluster.Pods) != 1 {
		t.Fatalf("expected one pod, got %d", len(firstCluster.Pods))
	}

	firstPod := firstCluster.Pods[0]
	if firstPod.DiskUsagePercent <= 0 {
		t.Fatalf("expected non-zero disk usage percent from ephemeral storage, got %f", firstPod.DiskUsagePercent)
	}
	if firstPod.NetInRate != 0 || firstPod.NetOutRate != 0 {
		t.Fatalf("expected first sample network rates to be zero, got in=%f out=%f", firstPod.NetInRate, firstPod.NetOutRate)
	}

	report.Timestamp = baseTime.Add(10 * time.Second)
	report.Pods[0].Usage.NetworkRxBytes = 61000
	report.Pods[0].Usage.NetworkTxBytes = 42000

	secondCluster, err := monitor.ApplyKubernetesReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyKubernetesReport(second): %v", err)
	}
	secondPod := secondCluster.Pods[0]
	if secondPod.NetInRate <= 0 || secondPod.NetOutRate <= 0 {
		t.Fatalf("expected positive network rates, got in=%f out=%f", secondPod.NetInRate, secondPod.NetOutRate)
	}

	metricID := kubernetesPodMetricID(secondCluster, secondPod)
	if metricID == "" {
		t.Fatal("expected kubernetes pod metric id")
	}
	netInPoints := monitor.metricsHistory.GetGuestMetrics(metricID, "netin", time.Hour)
	if len(netInPoints) == 0 {
		t.Fatal("expected netin points to be recorded")
	}
	diskPoints := monitor.metricsHistory.GetGuestMetrics(metricID, "disk", time.Hour)
	if len(diskPoints) == 0 {
		t.Fatal("expected disk points to be recorded")
	}
}

func TestRemoveAndReenrollKubernetesCluster(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.kubernetesTokenBindings["token-1"] = "agent-1"
	monitor.config.APITokens = []config.APITokenRecord{{ID: "token-1"}}
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:          "cluster-1",
		Name:        "cluster",
		DisplayName: "cluster",
		TokenID:     "token-1",
		TokenName:   "Token",
	})
	monitor.state.SetConnectionHealth(kubernetesConnectionPrefix+"cluster-1", true)

	_, err := monitor.RemoveKubernetesCluster("cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(monitor.state.GetKubernetesClusters()) != 0 {
		t.Fatal("expected cluster removed")
	}
	if _, exists := monitor.kubernetesTokenBindings["token-1"]; exists {
		t.Fatal("expected token binding removed")
	}
	if _, exists := monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"]; exists {
		t.Fatal("expected connection health removed")
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 1 {
		t.Fatal("expected removed cluster entry")
	}

	if err := monitor.AllowKubernetesClusterReenroll("cluster-1"); err != nil {
		t.Fatalf("unexpected reenroll error: %v", err)
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 0 {
		t.Fatal("expected removed entry cleared")
	}
}

func TestKubernetesClusterUpdates(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:              "cluster-1",
		Name:            "cluster",
		LastSeen:        time.Now().Add(-10 * time.Second),
		Status:          "online",
		IntervalSeconds: 5,
	})
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID:       "cluster-2",
		Name:     "cluster2",
		LastSeen: time.Now().Add(-10 * time.Hour),
		Status:   "online",
	})

	now := time.Now()
	monitor.evaluateKubernetesAgents(now)
	if monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-1"] != true {
		t.Fatal("expected cluster-1 healthy")
	}
	if monitor.state.ConnectionHealth[kubernetesConnectionPrefix+"cluster-2"] != false {
		t.Fatal("expected cluster-2 unhealthy")
	}

	_, err := monitor.UnhideKubernetesCluster("cluster-1")
	if err != nil {
		t.Fatalf("unexpected unhide error: %v", err)
	}
	if _, err := monitor.MarkKubernetesClusterPendingUninstall("cluster-1"); err != nil {
		t.Fatalf("unexpected pending uninstall error: %v", err)
	}
	if _, err := monitor.SetKubernetesClusterCustomDisplayName("cluster-1", "custom"); err != nil {
		t.Fatalf("unexpected set display name error: %v", err)
	}
}

func TestCleanupRemovedKubernetesClusters(t *testing.T) {
	monitor := newKubernetesTestMonitor()
	monitor.removedKubernetesClusters["cluster-1"] = time.Now().Add(-2 * removedKubernetesClustersTTL)
	monitor.state.AddRemovedKubernetesCluster(models.RemovedKubernetesCluster{
		ID:        "cluster-1",
		Name:      "cluster",
		RemovedAt: time.Now().Add(-2 * removedKubernetesClustersTTL),
	})

	monitor.cleanupRemovedKubernetesClusters(time.Now())
	if len(monitor.removedKubernetesClusters) != 0 {
		t.Fatal("expected removed clusters cleaned up")
	}
	if len(monitor.state.GetRemovedKubernetesClusters()) != 0 {
		t.Fatal("expected state cleanup")
	}
}
