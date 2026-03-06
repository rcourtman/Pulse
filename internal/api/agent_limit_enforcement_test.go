package api

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

func TestAgentCountNilMonitor(t *testing.T) {
	got := agentCount(nil)
	if got != 0 {
		t.Fatalf("expected 0 for nil monitor, got %d", got)
	}
}

func TestLegacyConnectionCountsFromReadState(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "pve-1",
			Resource: unifiedresources.Resource{
				ID:      "pve-1",
				Name:    "pve-1",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
			},
		},
		{
			SourceID: "pve-2",
			Resource: unifiedresources.Resource{
				ID:      "pve-2",
				Name:    "pve-2",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
				Agent:   &unifiedresources.AgentData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceDocker, []unifiedresources.IngestRecord{
		{
			SourceID: "docker-1",
			Resource: unifiedresources.Resource{
				ID:     "docker-1",
				Name:   "docker-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Status: unifiedresources.StatusOnline,
				Docker: &unifiedresources.DockerData{},
			},
		},
		{
			SourceID: "docker-2",
			Resource: unifiedresources.Resource{
				ID:     "docker-2",
				Name:   "docker-2",
				Type:   unifiedresources.ResourceTypeAgent,
				Status: unifiedresources.StatusOnline,
				Docker: &unifiedresources.DockerData{},
				Agent:  &unifiedresources.AgentData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceK8s, []unifiedresources.IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: unifiedresources.Resource{
				ID:         "k8s-1",
				Name:       "prod",
				Type:       unifiedresources.ResourceTypeK8sCluster,
				Status:     unifiedresources.StatusOnline,
				Kubernetes: &unifiedresources.K8sData{AgentID: "legacy-k8s-1"},
			},
		},
	})

	counts := legacyConnectionCountsFromReadState(unifiedresources.NewMonitorAdapter(registry))
	if counts.ProxmoxNodes != 1 {
		t.Fatalf("expected proxmox_nodes=1, got %d", counts.ProxmoxNodes)
	}
	if counts.DockerHosts != 1 {
		t.Fatalf("expected docker_hosts=1, got %d", counts.DockerHosts)
	}
	if counts.KubernetesClusters != 1 {
		t.Fatalf("expected kubernetes_clusters=1, got %d", counts.KubernetesClusters)
	}
}

func TestLegacyConnectionCountsUsesSnapshotFallback(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "pve-1",
			Resource: unifiedresources.Resource{
				ID:      "pve-1",
				Name:    "pve-1",
				Type:    unifiedresources.ResourceTypeAgent,
				Status:  unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceK8s, []unifiedresources.IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: unifiedresources.Resource{
				ID:         "k8s-1",
				Name:       "prod",
				Type:       unifiedresources.ResourceTypeK8sCluster,
				Status:     unifiedresources.StatusOnline,
				Kubernetes: &unifiedresources.K8sData{AgentID: "legacy-k8s-1"},
			},
		},
	})

	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))

	counts := legacyConnectionCounts(monitor)
	if counts.ProxmoxNodes != 1 || counts.KubernetesClusters != 1 {
		t.Fatalf("unexpected legacy counts from monitor: %+v", counts)
	}
}

func TestHostReportTargetsExistingHostBridge(t *testing.T) {
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{{ID: "host-1"}},
	}

	t.Run("matches_existing", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-1"},
		}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected match by host ID")
		}
	})

	t.Run("no_match_for_new_host", func(t *testing.T) {
		report := agentshost.Report{
			Host: agentshost.HostInfo{ID: "host-new", Hostname: "new-server"},
		}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, nil) {
			t.Fatal("expected no match for unknown host")
		}
	})

	t.Run("token_record_forwarded", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{Hostname: "srv-1", TokenID: "token-a"}},
		}
		report := agentshost.Report{
			Host: agentshost.HostInfo{Hostname: "srv-1"},
		}
		token := &config.APITokenRecord{ID: "token-a"}
		if !hostReportTargetsExistingHost(snapshot.Hosts, report, token) {
			t.Fatal("expected match with matching token")
		}
		wrongToken := &config.APITokenRecord{ID: "token-b"}
		if hostReportTargetsExistingHost(snapshot.Hosts, report, wrongToken) {
			t.Fatal("expected no match with different token")
		}
	})
}

func TestDeployReservedCount(t *testing.T) {
	// Nil counter returns 0.
	SetDeployReservationCounter(nil)
	if got := deployReservedCount(context.Background()); got != 0 {
		t.Fatalf("expected 0 with nil counter, got %d", got)
	}

	// Wired counter returns value.
	SetDeployReservationCounter(func(_ context.Context) int { return 5 })
	t.Cleanup(func() { SetDeployReservationCounter(nil) })

	if got := deployReservedCount(context.Background()); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}
