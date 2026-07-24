package unifiedresources

import "testing"

// TestBranchcov0724pmContainerViewRuntimeStatus covers every branch of
// ContainerView.RuntimeStatus: the nil-receiver arm, the nil-Proxmox arm, and
// the populated arm (including whitespace trimming and an explicit empty
// status).
func TestBranchcov0724pmContainerViewRuntimeStatus(t *testing.T) {
	cases := []struct {
		name string
		view ContainerView
		want string
	}{
		{"nil receiver returns empty", ContainerView{}, ""},
		{"resource without proxmox payload returns empty", NewContainerView(&Resource{ID: "ct-1"}), ""},
		{"populated status is trimmed", NewContainerView(&Resource{
			ID:      "ct-1",
			Proxmox: &ProxmoxData{RuntimeStatus: "  running  "},
		}), "running"},
		{"empty status stays empty after trim", NewContainerView(&Resource{
			ID:      "ct-2",
			Proxmox: &ProxmoxData{RuntimeStatus: "   "},
		}), ""},
		{"stopped status surfaces verbatim", NewContainerView(&Resource{
			ID:      "ct-3",
			Proxmox: &ProxmoxData{RuntimeStatus: "stopped"},
		}), "stopped"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.view.RuntimeStatus(); got != tc.want {
				t.Errorf("RuntimeStatus() = %q want %q", got, tc.want)
			}
		})
	}
}

// TestBranchcov0724pmHostViewString covers the nil-receiver and populated arms
// of HostView.String, asserting the concrete formatted output.
func TestBranchcov0724pmHostViewString(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		if got, want := (HostView{}).String(), `HostView(, "")`; got != want {
			t.Errorf("nil HostView.String() = %q want %q", got, want)
		}
	})
	t.Run("populated", func(t *testing.T) {
		v := NewHostView(&Resource{ID: "host-1", Type: ResourceTypeAgent, Name: "node-a"})
		if got, want := v.String(), `HostView(host-1, "node-a")`; got != want {
			t.Errorf("HostView.String() = %q want %q", got, want)
		}
	})
}

// TestBranchcov0724pmDockerHostViewString covers the nil-receiver and populated
// arms of DockerHostView.String, asserting the concrete formatted output.
func TestBranchcov0724pmDockerHostViewString(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		if got, want := (DockerHostView{}).String(), `DockerHostView(, "")`; got != want {
			t.Errorf("nil DockerHostView.String() = %q want %q", got, want)
		}
	})
	t.Run("populated", func(t *testing.T) {
		v := NewDockerHostView(&Resource{ID: "docker-host-1", Type: ResourceTypeAgent, Name: "swarm-1"})
		if got, want := v.String(), `DockerHostView(docker-host-1, "swarm-1")`; got != want {
			t.Errorf("DockerHostView.String() = %q want %q", got, want)
		}
	})
}

// TestBranchcov0724pmK8sClusterViewString covers the nil-receiver and populated
// arms of K8sClusterView.String, asserting the concrete formatted output.
func TestBranchcov0724pmK8sClusterViewString(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		if got, want := (K8sClusterView{}).String(), `K8sClusterView(, "")`; got != want {
			t.Errorf("nil K8sClusterView.String() = %q want %q", got, want)
		}
	})
	t.Run("populated", func(t *testing.T) {
		v := NewK8sClusterView(&Resource{ID: "k8s-1", Type: ResourceTypeK8sCluster, Name: "prod-cluster"})
		if got, want := v.String(), `K8sClusterView(k8s-1, "prod-cluster")`; got != want {
			t.Errorf("K8sClusterView.String() = %q want %q", got, want)
		}
	})
}
