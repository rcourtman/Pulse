package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestApplyPersistedMetadataProjectsHostWebInterfacesAcrossPlatforms(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.hostMetadataStore = config.NewHostMetadataStore(t.TempDir(), nil)
	if monitor.dockerMetadataStore == nil {
		monitor.dockerMetadataStore = config.NewDockerMetadataStore(t.TempDir(), nil)
	}
	seedHost := func(id, url string) {
		t.Helper()
		if err := monitor.hostMetadataStore.Set(id, &config.HostMetadata{CustomURL: url}); err != nil {
			t.Fatalf("seed host metadata %q: %v", id, err)
		}
	}

	seedHost("agent-stable", "https://agent.internal")
	seedHost("pve-source", "https://pve.internal")
	seedHost("pbs-stable", "https://pbs-override.internal")
	seedHost("pmg-stable", "https://pmg.internal")
	seedHost("cluster-stable", "https://kubernetes.internal")
	seedHost("cluster-stable:node:worker-renamed", "https://worker.internal")
	seedHost("retired-agent-id", "https://migrated-agent.internal")
	if err := monitor.dockerMetadataStore.SetHostMetadata(
		"docker-runtime-stable",
		&config.DockerHostMetadata{
			CustomDisplayName: "Renamed Podman host",
			CustomURL:         "https://podman.internal",
			Notes:             []string{"retain me"},
		},
	); err != nil {
		t.Fatalf("seed Docker host metadata: %v", err)
	}

	resources := []unifiedresources.Resource{
		{
			ID:   "agent-row",
			Type: unifiedresources.ResourceTypeAgent,
			Name: "agent-renamed",
			Agent: &unifiedresources.AgentData{
				AgentID: "agent-stable",
			},
		},
		{
			ID:   "pve-row",
			Type: unifiedresources.ResourceTypeAgent,
			Name: "pve-renamed",
			Proxmox: &unifiedresources.ProxmoxData{
				SourceID: "pve-source",
				NodeName: "pve-renamed",
			},
		},
		{
			ID:         "docker-row",
			Type:       unifiedresources.ResourceTypeAgent,
			Technology: "podman",
			Name:       "podman-renamed",
			Docker: &unifiedresources.DockerData{
				HostSourceID: "docker-runtime-stable",
				Hostname:     "podman-renamed",
			},
		},
		{
			ID:        "pbs-row",
			Type:      unifiedresources.ResourceTypePBS,
			Name:      "pbs-renamed",
			CustomURL: "https://pbs-configured.internal",
			PBS: &unifiedresources.PBSData{
				InstanceID: "pbs-stable",
			},
		},
		{
			ID:   "pmg-row",
			Type: unifiedresources.ResourceTypePMG,
			Name: "pmg-renamed",
			PMG: &unifiedresources.PMGData{
				InstanceID: "pmg-stable",
			},
		},
		{
			ID:   "cluster-row",
			Type: unifiedresources.ResourceTypeK8sCluster,
			Name: "cluster-renamed",
			Kubernetes: &unifiedresources.K8sData{
				ClusterID: "cluster-stable",
			},
		},
		{
			ID:   "node-row-after-recreation",
			Type: unifiedresources.ResourceTypeK8sNode,
			Name: "worker-renamed",
			Kubernetes: &unifiedresources.K8sData{
				ClusterID: "cluster-stable",
				NodeName:  "worker-renamed",
				NodeUID:   "new-node-uid",
			},
		},
		{
			ID:   "agent-row-after-migration",
			Type: unifiedresources.ResourceTypeAgent,
			Name: "migrated-agent",
			Canonical: &unifiedresources.CanonicalIdentity{
				PrimaryID:     "agent:current-agent-id",
				SupersededIDs: []string{"retired-agent-id"},
			},
		},
	}

	got := monitor.applyPersistedMetadataToUnifiedResources(resources)
	want := []string{
		"https://agent.internal",
		"https://pve.internal",
		"https://podman.internal",
		"https://pbs-override.internal",
		"https://pmg.internal",
		"https://kubernetes.internal",
		"https://worker.internal",
		"https://migrated-agent.internal",
	}
	for i := range want {
		if got[i].CustomURL != want[i] {
			t.Fatalf("resource %d (%s) CustomURL = %q, want %q", i, got[i].Name, got[i].CustomURL, want[i])
		}
	}

	if resources[0].CustomURL != "" {
		t.Fatalf("projection mutated input resources: %#v", resources[0])
	}
}

func TestApplyPersistedMetadataPreservesConfiguredHostURLWithoutManualOverride(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.hostMetadataStore = config.NewHostMetadataStore(t.TempDir(), nil)
	if err := monitor.hostMetadataStore.Set("pbs-stable", &config.HostMetadata{
		Description: "metadata without a URL",
	}); err != nil {
		t.Fatalf("seed non-URL host metadata: %v", err)
	}

	resource := unifiedresources.Resource{
		ID:        "pbs-row",
		Type:      unifiedresources.ResourceTypePBS,
		Name:      "pbs",
		CustomURL: "https://pbs-configured.internal",
		PBS:       &unifiedresources.PBSData{InstanceID: "pbs-stable"},
	}
	got := monitor.applyPersistedMetadataToUnifiedResources([]unifiedresources.Resource{resource})
	if got[0].CustomURL != "https://pbs-configured.internal" {
		t.Fatalf("configured CustomURL = %q", got[0].CustomURL)
	}
}
