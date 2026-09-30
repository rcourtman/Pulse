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

func TestRefreshPlatformScopesAllSourceCombinations(t *testing.T) {
	// Include every membership combination, reverse input ordering, duplicates
	// and unknown sources: changing the membership representation must not
	// change page admission or canonical ordering.
	sources := []DataSource{SourceAgent, SourceTrueNAS, SourceProxmox, SourcePBS, SourcePMG, SourceDocker, SourceK8s, SourceVMware, SourceAvailability}
	for mask := 0; mask < 1<<len(sources); mask++ {
		resource := Resource{Type: ResourceTypeVM, Sources: []DataSource{"unknown"}, PlatformScopes: []string{"stale"}}
		var want []string
		for i, source := range sources {
			if mask&(1<<i) != 0 {
				want = append(want, platformScopeForSource(source))
			}
		}
		for i := len(sources) - 1; i >= 0; i-- {
			if mask&(1<<i) != 0 {
				resource.Sources = append(resource.Sources, sources[i], sources[i])
			}
		}
		RefreshPlatformScopes(&resource)
		assertStringSliceEqual(t, resource.PlatformScopes, want)
		if mask == 0 && resource.PlatformScopes != nil {
			t.Fatal("absent membership should remain nil")
		}
	}
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
