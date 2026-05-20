package unifiedresources

import "testing"

func TestRefreshPlatformScopes_DerivesSourceMembership(t *testing.T) {
	resource := Resource{
		ID:      "node-1",
		Type:    ResourceTypeAgent,
		Name:    "pve-node",
		Sources: []DataSource{SourceAgent, SourceProxmox},
		Agent:   &AgentData{Hostname: "pve-node"},
		Proxmox: &ProxmoxData{NodeName: "pve-node"},
	}

	RefreshPlatformScopes(&resource)

	assertStringSliceEqual(t, resource.PlatformScopes, []string{"agent", "proxmox-pve"})
}

func TestRefreshPlatformScopes_ProxmoxLXCDockerRuntimeBelongsToRuntimeAndOwningPlatform(t *testing.T) {
	resource := Resource{
		ID:      "docker-container-frigate-141",
		Type:    ResourceTypeAppContainer,
		Name:    "frigate",
		Sources: []DataSource{SourceDocker},
		Docker: &DockerData{
			HostSourceID: "proxmox-lxc-docker:pve-a:node-a:141",
			ContainerID:  "frigate",
			Runtime:      "docker",
		},
	}

	RefreshPlatformScopes(&resource)

	assertStringSliceEqual(t, resource.PlatformScopes, []string{"proxmox-pve", "docker"})
}

func TestRefreshPlatformScopes_TrueNASAppContainerKeepsOwningPlatform(t *testing.T) {
	resource := Resource{
		ID:      "app-container:truenas-main:nextcloud",
		Type:    ResourceTypeAppContainer,
		Name:    "nextcloud",
		Sources: []DataSource{SourceTrueNAS},
		TrueNAS: &TrueNASData{Hostname: "truenas-main"},
		Docker: &DockerData{
			ContainerID: "nextcloud",
			Image:       "ix-nextcloud:latest",
			Runtime:     "docker",
		},
	}

	RefreshPlatformScopes(&resource)

	assertStringSliceEqual(t, resource.PlatformScopes, []string{"truenas"})
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}
