package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newSharedAgentTokenLifecycleMonitor(tokenID string) *Monitor {
	return &Monitor{
		state:                     models.NewState(),
		config:                    &config.Config{APITokens: []config.APITokenRecord{{ID: tokenID, Name: "Unified agent"}}},
		hostTokenBindings:         make(map[string]string),
		dockerTokenBindings:       make(map[string]string),
		kubernetesTokenBindings:   make(map[string]string),
		removedKubernetesClusters: make(map[string]time.Time),
	}
}

func requireAgentTokenPresent(t *testing.T, monitor *Monitor, tokenID string) {
	t.Helper()
	if got := len(monitor.config.APITokens); got != 1 || monitor.config.APITokens[0].ID != tokenID {
		t.Fatalf("shared unified token %q was revoked while a sibling module was live: %+v", tokenID, monitor.config.APITokens)
	}
}

func TestRemoveKubernetesClusterKeepsUnifiedTokenUsedByHost(t *testing.T) {
	const tokenID = "unified-kubernetes-token"
	monitor := newSharedAgentTokenLifecycleMonitor(tokenID)
	monitor.state.UpsertHost(models.Host{ID: "host-1", Hostname: "worker-1", TokenID: tokenID})
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID: "cluster-1", Name: "cluster-1", TokenID: tokenID, TokenName: "Unified agent",
	})

	if _, err := monitor.RemoveKubernetesCluster("cluster-1"); err != nil {
		t.Fatalf("RemoveKubernetesCluster: %v", err)
	}
	requireAgentTokenPresent(t, monitor, tokenID)
}

func TestRemoveDockerHostKeepsUnifiedTokenUsedByKubernetes(t *testing.T) {
	const tokenID = "unified-docker-token"
	monitor := newSharedAgentTokenLifecycleMonitor(tokenID)
	monitor.state.UpsertDockerHost(models.DockerHost{
		ID: "docker-1", Hostname: "worker-1", TokenID: tokenID, TokenName: "Unified agent",
	})
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID: "cluster-1", Name: "cluster-1", TokenID: tokenID,
	})

	if _, err := monitor.RemoveDockerHost("docker-1"); err != nil {
		t.Fatalf("RemoveDockerHost: %v", err)
	}
	requireAgentTokenPresent(t, monitor, tokenID)
}

func TestRemoveHostAgentKeepsUnifiedTokenUsedByKubernetes(t *testing.T) {
	const tokenID = "unified-host-token"
	monitor := newSharedAgentTokenLifecycleMonitor(tokenID)
	monitor.state.UpsertHost(models.Host{ID: "host-1", Hostname: "worker-1", TokenID: tokenID})
	monitor.state.UpsertKubernetesCluster(models.KubernetesCluster{
		ID: "cluster-1", Name: "cluster-1", TokenID: tokenID,
	})

	if _, err := monitor.RemoveHostAgent("host-1"); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}
	requireAgentTokenPresent(t, monitor, tokenID)
}

func TestUninstallHostAgentKeepsCredentialUsedBySiblingHost(t *testing.T) {
	dataPath := t.TempDir()
	monitor := newHostRemovalLifecycleMonitor(t, dataPath)
	monitor.persistence = config.NewConfigPersistence(dataPath)

	now := time.Now().UTC()
	token := config.APITokenRecord{ID: "shared-host-token", Name: "Shared host token", CreatedAt: now.Add(-time.Hour)}
	monitor.config.APITokens = []config.APITokenRecord{token}
	if err := monitor.persistence.SaveAPITokens(monitor.config.APITokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	firstReport := hostRemovalLifecycleReport("machine-a", "machine-a", "agent-a", "site-a", "linux", now)
	secondReport := hostRemovalLifecycleReport("machine-b", "machine-b", "agent-b", "site-b", "linux", now)
	first, err := monitor.ApplyHostReport(firstReport, &token)
	if err != nil {
		t.Fatalf("ApplyHostReport(first): %v", err)
	}
	second, err := monitor.ApplyHostReport(secondReport, &token)
	if err != nil {
		t.Fatalf("ApplyHostReport(second): %v", err)
	}

	if _, err := monitor.UninstallHostAgent(first.ID, token.ID); err != nil {
		t.Fatalf("UninstallHostAgent(first): %v", err)
	}
	requireAgentTokenPresent(t, monitor, token.ID)
	live := monitor.GetLiveHostsSnapshot()
	if len(live) != 1 || live[0].ID != second.ID {
		t.Fatalf("sibling host %q did not survive first host uninstall: %+v", second.ID, live)
	}
	firstReport.Timestamp = now.Add(time.Minute)
	if _, err := monitor.ApplyHostReport(firstReport, &token); err == nil {
		t.Fatal("uninstalled host resumed reporting through the preserved shared credential")
	}
	persisted, err := monitor.persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != token.ID {
		t.Fatalf("shared credential was removed from durable inventory: %+v", persisted)
	}
}
